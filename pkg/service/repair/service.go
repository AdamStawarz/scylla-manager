// Copyright (C) 2017 ScyllaDB

package repair

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/go-set/strset"
	"github.com/scylladb/gocqlx/v2"
	"github.com/scylladb/gocqlx/v2/qb"
	"github.com/scylladb/mermaid/pkg/dht"
	"github.com/scylladb/mermaid/pkg/schema/table"
	"github.com/scylladb/mermaid/pkg/scyllaclient"
	"github.com/scylladb/mermaid/pkg/service"
	"github.com/scylladb/mermaid/pkg/util/inexlist/dcfilter"
	"github.com/scylladb/mermaid/pkg/util/inexlist/ksfilter"
	"github.com/scylladb/mermaid/pkg/util/parallel"
	"github.com/scylladb/mermaid/pkg/util/timeutc"
	"github.com/scylladb/mermaid/pkg/util/uuid"
	"go.uber.org/atomic"
	"golang.org/x/sync/errgroup"
)

// ClusterNameFunc returns name for a given ID.
type ClusterNameFunc func(ctx context.Context, clusterID uuid.UUID) (string, error)

type metricsWatcher interface {
	RegisterCallback(func()) func()
}

// Service orchestrates clusterName repairs.
type Service struct {
	session gocqlx.Session
	config  Config

	clusterName  ClusterNameFunc
	scyllaClient scyllaclient.ProviderFunc
	logger       log.Logger
	mw           metricsWatcher

	intensityHandlers map[uuid.UUID]*intensityHandler
	mu                sync.Mutex
}

func NewService(session gocqlx.Session, config Config, clusterName ClusterNameFunc,
	scyllaClient scyllaclient.ProviderFunc, logger log.Logger, mw metricsWatcher) (*Service, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid config")
	}

	if clusterName == nil {
		return nil, errors.New("invalid cluster name provider")
	}

	if scyllaClient == nil {
		return nil, errors.New("invalid scylla provider")
	}

	return &Service{
		session:           session,
		config:            config,
		clusterName:       clusterName,
		scyllaClient:      scyllaClient,
		logger:            logger,
		mw:                mw,
		intensityHandlers: make(map[uuid.UUID]*intensityHandler),
	}, nil
}

// Runner creates a Runner that handles repairs.
func (s *Service) Runner() Runner {
	return Runner{service: s}
}

// GetTarget converts runner properties into repair Target.
func (s *Service) GetTarget(ctx context.Context, clusterID uuid.UUID, properties json.RawMessage) (Target, error) {
	p := defaultTaskProperties()

	// Parse task properties
	if err := json.Unmarshal(properties, &p); err != nil {
		return Target{}, service.ErrValidate(errors.Wrapf(err, "parse runner properties: %s", properties))
	}

	// Copy basic properties
	t := Target{
		FailFast:            p.FailFast,
		Continue:            p.Continue,
		Intensity:           p.Intensity,
		Parallel:            p.Parallel,
		SmallTableThreshold: p.SmallTableThreshold,
	}

	client, err := s.scyllaClient(ctx, clusterID)
	if err != nil {
		return t, errors.Wrapf(err, "get client")
	}

	// Get hosts in DCs
	dcMap, err := client.Datacenters(ctx)
	if err != nil {
		return t, errors.Wrap(err, "read datacenters")
	}

	// Filter DCs
	if t.DC, err = dcfilter.Apply(dcMap, p.DC); err != nil {
		return t, err
	}

	// Filter keyspaces
	f, err := ksfilter.NewFilter(p.Keyspace)
	if err != nil {
		return t, err
	}

	keyspaces, err := client.Keyspaces(ctx)
	if err != nil {
		return t, errors.Wrapf(err, "read keyspaces")
	}

	dcs := strset.New(t.DC...)
	var skippedKeyspaces []string
	for _, keyspace := range keyspaces {
		tables, err := client.Tables(ctx, keyspace)
		if err != nil {
			return t, errors.Wrapf(err, "keyspace %s: get tables", keyspace)
		}

		// Get the ring description and skip local data
		ring, err := client.DescribeRing(ctx, keyspace)
		if err != nil {
			return t, errors.Wrapf(err, "keyspace %s: get ring description", keyspace)
		}

		// Ignore keyspaces not replicated in desired DCs
		if !dcs.HasAny(ring.Datacenters()...) {
			continue
		}

		if !s.singleNodeCluster(dcMap) {
			// Ignore not replicated keyspaces
			if ring.Replication == scyllaclient.LocalStrategy {
				continue
			}

			notEnoughReplicas := false
			for _, tr := range ring.Tokens {
				replicas := 0
				for _, r := range tr.Replicas {
					if dcs.Has(ring.HostDC[r]) {
						replicas++
					}
				}
				if replicas <= 1 {
					notEnoughReplicas = true
					break
				}
			}
			if notEnoughReplicas {
				skippedKeyspaces = append(skippedKeyspaces, keyspace)
				continue
			}
		}

		// Add to the filter
		f.Add(keyspace, tables)
	}

	if len(skippedKeyspaces) > 0 {
		s.logger.Info(ctx,
			"Repair of the following keyspaces will be skipped because not all the tokens are present in the specified DCs",
			"keyspaces", strings.Join(skippedKeyspaces, ", "),
		)
	}

	// Get the filtered units
	t.Units, err = f.Apply(false)
	if err != nil {
		return t, err
	}

	return t, nil
}

func (s *Service) singleNodeCluster(dcMap map[string][]string) bool {
	if len(dcMap) == 1 {
		for _, dc := range dcMap {
			if len(dc) <= 1 {
				return true
			}
		}
	}
	return false
}

// Repair performs the repair process on the Target.
func (s *Service) Repair(ctx context.Context, clusterID, taskID, runID uuid.UUID, target Target) error {
	s.logger.Debug(ctx, "Repair",
		"cluster_id", clusterID,
		"task_id", taskID,
		"run_id", runID,
		"target", target,
	)

	run := &Run{
		ClusterID: clusterID,
		TaskID:    taskID,
		ID:        runID,
		DC:        target.DC,
		StartTime: timeutc.Now().UTC(),
	}
	if err := s.putRun(run); err != nil {
		return errors.Wrapf(err, "put run")
	}

	// Get cluster name
	clusterName, err := s.clusterName(ctx, run.ClusterID)
	if err != nil {
		return errors.Wrap(err, "invalid cluster")
	}
	run.clusterName = clusterName

	s.logger.Info(ctx, "Initializing repair",
		"cluster_id", run.ClusterID,
		"task_id", run.TaskID,
		"run_id", run.ID,
		"target", target,
	)

	// Get the cluster client
	client, err := s.scyllaClient(ctx, run.ClusterID)
	if err != nil {
		return errors.Wrap(err, "get client proxy")
	}

	if target.Continue {
		if err := s.decorateWithPrevRun(ctx, run); err != nil {
			return err
		}
		if run.PrevID != uuid.Nil {
			s.putRunLogError(ctx, run)
		}
	}

	// Create generator
	var (
		manager = newProgressManager(run, s.session)
		g       = newGenerator(s.config.GracefulStopTimeout, manager, target.FailFast, s.logger)
		wc      int
	)
	for _, u := range target.Units {
		// Get ring
		ring, err := client.DescribeRing(ctx, u.Keyspace)
		if err != nil {
			return errors.Wrapf(err, "keyspace %s: get ring description", u.Keyspace)
		}

		// Transform ring to tableTokenRanges and init generator
		g.Add(newTableTokenRangeBuilder(target, ring.HostDC).Add(ring.Tokens).Build(u))

		// Estimate worker count
		if c := workerCount(ring.Tokens); c > wc {
			wc = c
		}
	}

	repairHosts := g.Hosts()

	hostRangesLimits, err := s.hostRangeLimits(ctx, client, repairHosts.List())
	if err != nil {
		return errors.Wrap(err, "fetch host range limits")
	}
	g.SetHostRangeLimits(hostRangesLimits)

	// Dynamic Intensity
	ih, cleanup := s.newIntensityHandler(ctx, clusterID, wc, target.Intensity, target.Parallel)
	defer cleanup()

	// Init Generator
	if err := g.Init(ctx, ih); err != nil {
		return err
	}

	// Check if no other repairs are running
	if active, err := client.ActiveRepairs(ctx, repairHosts.List()); err != nil {
		s.logger.Error(ctx, "Active repair check failed", "error", err)
	} else if len(active) > 0 {
		return errors.Errorf("ensure no active repair on hosts, %s are repairing", strings.Join(active, ", "))
	}

	// Get hosts in all DCs
	status, err := client.Status(ctx)
	if err != nil {
		return errors.Wrap(err, "status")
	}

	// Validate that all hosts to repair are up
	if down := status.DownHosts(); repairHosts.HasAny(down...) {
		return errors.Errorf("ensure nodes are up, down nodes: %s", strings.Join(down, ","))
	}

	if err := s.optimizeSmallTables(ctx, client, target, g); err != nil {
		return errors.Wrap(err, "optimize small tables")
	}

	hp := make(hostPriority)
	// In a multi-dc repair look for a local datacenter
	if len(target.DC) > 1 {
		dcMap, err := client.Datacenters(ctx)
		if err != nil {
			return errors.Wrap(err, "read datacenters")
		}

		targetDCs := strset.New(target.DC...)
		for dc := range dcMap {
			if !targetDCs.Has(dc) {
				delete(dcMap, dc)
			}
		}
		closest, err := client.ClosestDC(ctx, dcMap)
		if err != nil {
			return errors.Wrap(err, "calculate datacenter latency measurement")
		}

		for p, dc := range closest {
			for _, h := range dcMap[dc] {
				if repairHosts.Has(h) {
					hp[h] = p
				}
			}
		}
	}
	g.SetHostPriority(hp)

	hostPartitioner, err := s.hostPartitioner(ctx, repairHosts.List(), client)
	if err != nil {
		return errors.Wrap(err, "initialize host partitioner")
	}
	// Create worker
	w := newWorker(run, g.Next(), g.Result(), client, s.logger, manager, s.config.PollInterval, hostPartitioner)

	// Worker context doesn't derive from ctx, generator will handle graceful
	// shutdown. Generator must receive ctx.
	workerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start updating progress metrics.
	stop := s.watchProgressMetrics(ctx, run.ClusterID, run.TaskID, run.ID)
	defer stop()

	// Run Workers and Generator
	var eg errgroup.Group
	for i := 0; i < wc; i++ {
		wctx := log.WithFields(workerCtx, "worker", i)
		eg.Go(func() error {
			return w.Run(wctx)
		})
	}
	eg.Go(func() error {
		return g.Run(ctx)
	})

	if err := eg.Wait(); err != nil {
		if err == context.Canceled || target.FailFast {
			// Send kill repair request to all hosts.
			s.killAllRepairs(ctx, client, repairHosts.List())
		}
		return err
	}

	return nil
}

func (s *Service) killAllRepairs(ctx context.Context, client *scyllaclient.Client, hosts []string) {
	killCtx := log.CopyTraceID(context.Background(), ctx)
	killCtx = scyllaclient.Interactive(killCtx)
	if err := client.KillAllRepairs(killCtx, hosts...); err != nil {
		s.logger.Error(killCtx, "Failed to kill repairs", "hosts", hosts, "error", err)
	}
}

func (s *Service) optimizeSmallTables(ctx context.Context, client *scyllaclient.Client, target Target, g *generator) error {
	repairHosts := g.Hosts()

	// Get report for Host, Keyspace, Table tuples
	var hkts []scyllaclient.HostKeyspaceTable
	for _, u := range target.Units {
		for _, t := range u.Tables {
			for _, h := range repairHosts.List() {
				hkts = append(hkts, scyllaclient.HostKeyspaceTable{h, u.Keyspace, t})
			}
		}
	}
	report, err := client.TableDiskSizeReport(ctx, hkts)
	if err != nil {
		return errors.Wrap(err, "fetch table disk size report")
	}

	// Calculate total table size across hosts
	totalSize := make(map[string]int64)
	for i, size := range report {
		key := hkts[i].Keyspace + "." + hkts[i].Table
		totalSize[key] += size
	}

	// Log and mark small tables
	var smallTables []string
	for _, u := range target.Units {
		for _, t := range u.Tables {
			key := u.Keyspace + "." + t
			total := totalSize[key]

			if total <= target.SmallTableThreshold {
				s.logger.Debug(ctx, "Detected small table", "keyspace", u.Keyspace, "table", t, "size", total, "threshold", target.SmallTableThreshold)
				g.markSmallTable(u.Keyspace, t)
				smallTables = append(smallTables, key)
			}
		}
	}
	if len(smallTables) > 0 {
		s.logger.Info(ctx, "Detected small tables", "tables", smallTables, "threshold", target.SmallTableThreshold)
	}

	return nil
}

type rangesLimit struct {
	Default int
	Max     int
}

type hostRangesLimit map[string]rangesLimit

func (s *Service) hostRangeLimits(ctx context.Context, client *scyllaclient.Client, hosts []string) (hostRangesLimit, error) {
	var (
		out = make(hostRangesLimit, len(hosts))
		mu  sync.Mutex
	)

	err := parallel.Run(len(hosts), parallel.NoLimit, func(i int) error {
		h := hosts[i]

		totalMemory, err := client.TotalMemory(ctx, h)
		if err != nil {
			return errors.Wrapf(err, "%s: get total memory", h)
		}

		shards, err := client.ShardCount(ctx, h)
		if err != nil {
			return errors.Wrapf(err, "%s: get shard count", h)
		}

		v := rangesLimit{
			Default: int(shards),
			Max:     s.maxRepairRangesInParallel(totalMemory),
		}
		s.logger.Info(ctx, "Host repair intensity limit", "host", h, "limit", v)

		mu.Lock()
		out[h] = v
		mu.Unlock()

		return nil
	})

	return out, err
}

func (s *Service) maxRepairRangesInParallel(totalMemory int64) int {
	return int(float64(totalMemory) * 0.1 / (32 * 1024 * 1024))
}

func (s *Service) newIntensityHandler(ctx context.Context, clusterID uuid.UUID, wc int, intensity float64, parallel int) (ih *intensityHandler, cleanup func()) {
	ih = &intensityHandler{
		logger:      s.logger.Named("control"),
		intensity:   atomic.NewFloat64(intensity),
		parallel:    atomic.NewInt64(int64(parallel)),
		maxParallel: wc,
	}

	s.mu.Lock()
	if _, ok := s.intensityHandlers[clusterID]; ok {
		s.logger.Error(ctx, "Overriding intensity handler", "cluster_id", clusterID, "intensity", intensity, "parallel", parallel)
	}
	s.intensityHandlers[clusterID] = ih
	s.mu.Unlock()

	return ih, func() {
		s.mu.Lock()
		delete(s.intensityHandlers, clusterID)
		s.mu.Unlock()
	}
}

// decorateWithPrevRun looks for previous run and if it can be continued sets
// PrevID on the given run.
func (s *Service) decorateWithPrevRun(ctx context.Context, run *Run) error {
	prev, err := s.GetLastResumableRun(ctx, run.ClusterID, run.TaskID)
	if err == service.ErrNotFound {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "get previous run")
	}

	// Check if can continue from prev
	s.logger.Info(ctx, "Found previous run", "prev_id", prev.ID)
	if s.config.AgeMax > 0 && timeutc.Since(prev.StartTime) > s.config.AgeMax {
		s.logger.Info(ctx, "Starting from scratch: previous run is too old")
		return nil
	}

	// Decorate run with previous run id.
	// Progress manager will use this as indication to restore state on
	// generator init.
	run.PrevID = prev.ID

	return nil
}

// putRun upserts a repair run.
func (s *Service) putRun(r *Run) error {
	return table.RepairRun.InsertQuery(s.session).BindStruct(r).ExecRelease()
}

// putRunLogError executes putRun and consumes the error.
func (s *Service) putRunLogError(ctx context.Context, r *Run) {
	if err := s.putRun(r); err != nil {
		s.logger.Error(ctx, "Cannot update the run",
			"run", r,
			"error", err,
		)
	}
}

func (s *Service) hostPartitioner(ctx context.Context, hosts []string, client *scyllaclient.Client) (map[string]*dht.Murmur3Partitioner, error) {
	out := make(map[string]*dht.Murmur3Partitioner)
	// Check the cluster partitioner
	p, err := client.Partitioner(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get client partitioner name")
	}

	// If partitioner is not supported or row-level repair is forced
	// return nil partitioner which will  signal that task should continue
	// as row-level repair.
	if p != scyllaclient.Murmur3Partitioner || s.config.ForceRepairType == TypeRowLevel {
		for _, h := range hosts {
			out[h] = nil
		}
		return out, nil
	}

	if s.config.ForceRepairType == TypeLegacy {
		for _, h := range hosts {
			p, err := s.partitioner(ctx, h, client)
			if err != nil {
				return nil, err
			}
			out[h] = p
		}

		return out, nil
	}

	sf, err := client.ScyllaFeatures(ctx, hosts)
	if err != nil {
		s.logger.Error(ctx, "Checking scylla features failed", "error", err)
		return nil, errors.Wrap(err, "scylla features")
	}

	for _, h := range hosts {
		if sf[h].RowLevelRepair {
			out[h] = nil
		} else {
			s.logger.Info(ctx, "Row-level repair not supported", "host", h)
			p, err := s.partitioner(ctx, h, client)
			if err != nil {
				return nil, err
			}
			out[h] = p
		}
	}

	return out, nil
}

func (s *Service) partitioner(ctx context.Context, host string, client *scyllaclient.Client) (*dht.Murmur3Partitioner, error) {
	shardCount, err := client.ShardCount(ctx, host)
	if err != nil {
		return nil, errors.Wrap(err, "get shard count")
	}
	return dht.NewMurmur3Partitioner(shardCount, uint(s.config.Murmur3PartitionerIgnoreMSBBits)), nil
}

func (s *Service) watchProgressMetrics(ctx context.Context, clusterID, taskID, runID uuid.UUID) func() {
	if s.mw == nil {
		return func() {}
	}

	update := func() {
		run, err := s.GetRun(ctx, clusterID, taskID, runID)
		if err != nil {
			s.logger.Error(ctx, "Failed to get run in metrics update",
				"cluster_id", clusterID,
				"task_id", taskID,
				"run_id", runID,
				"error", err,
			)
			return
		}

		p, err := aggregateProgress(s.hostIntensityFunc(clusterID), NewProgressVisitor(run, s.session))
		if err != nil {
			s.logger.Error(ctx, "Failed to aggregate progress in metrics update",
				"cluster_id", clusterID,
				"task_id", taskID,
				"run_id", runID,
				"error", err,
			)
			return
		}
		updateMetrics(run, p)
	}
	update()

	return s.mw.RegisterCallback(update)
}

// GetLastResumableRun returns the the most recent started but not done run of
// the task, if there is a recent run that is completely done ErrNotFound is
// reported.
func (s *Service) GetLastResumableRun(ctx context.Context, clusterID, taskID uuid.UUID) (*Run, error) {
	s.logger.Debug(ctx, "GetLastResumableRun",
		"cluster_id", clusterID,
		"task_id", taskID,
	)

	stmt, names := qb.Select(table.RepairRun.Name()).Where(
		qb.Eq("cluster_id"),
		qb.Eq("task_id"),
	).Limit(20).ToCql()

	q := s.session.Query(stmt, names).BindMap(qb.M{
		"cluster_id": clusterID,
		"task_id":    taskID,
	})

	var runs []*Run
	if err := q.SelectRelease(&runs); err != nil {
		return nil, err
	}

	for _, r := range runs {
		p, err := aggregateProgress(s.hostIntensityFunc(clusterID), NewProgressVisitor(r, s.session))
		if err != nil {
			return nil, err
		}
		if p.TokenRanges > 0 {
			if p.Success == p.TokenRanges {
				break
			}
			return r, nil
		}
	}

	return nil, service.ErrNotFound
}

// GetRun returns a run based on ID. If nothing was found mermaid.ErrNotFound
// is returned.
func (s *Service) GetRun(ctx context.Context, clusterID, taskID, runID uuid.UUID) (*Run, error) {
	s.logger.Debug(ctx, "GetRun",
		"cluster_id", clusterID,
		"task_id", taskID,
		"run_id", runID,
	)

	var r Run
	return &r, table.RepairRun.GetQuery(s.session).BindMap(qb.M{
		"cluster_id": clusterID,
		"task_id":    taskID,
		"id":         runID,
	}).GetRelease(&r)
}

// GetProgress returns run progress for all shards on all the hosts. If nothing
// was found mermaid.ErrNotFound is returned.
func (s *Service) GetProgress(ctx context.Context, clusterID, taskID, runID uuid.UUID) (Progress, error) {
	s.logger.Debug(ctx, "GetProgress",
		"cluster_id", clusterID,
		"task_id", taskID,
		"run_id", runID,
	)

	run, err := s.GetRun(ctx, clusterID, taskID, runID)
	if err != nil {
		return Progress{}, err
	}

	p, err := aggregateProgress(s.hostIntensityFunc(clusterID), NewProgressVisitor(run, s.session))
	if err != nil {
		return Progress{}, err
	}
	p.DC = run.DC

	return p, nil
}

func (s *Service) hostIntensityFunc(clusterID uuid.UUID) func() (float64, int) {
	// When repair is running, intensity is dynamic.
	// Otherwise always return 0, 0.
	intensityFunc := func() (float64, int) {
		return 0, 0
	}

	s.mu.Lock()
	if ih, ok := s.intensityHandlers[clusterID]; ok {
		intensityFunc = func() (float64, int) {
			return ih.Intensity(), ih.Parallel()
		}
	}
	s.mu.Unlock()

	return intensityFunc
}

// SetIntensity changes intensity of an ongoing repair.
func (s *Service) SetIntensity(ctx context.Context, clusterID uuid.UUID, intensity float64) error {
	s.mu.Lock()
	ih, ok := s.intensityHandlers[clusterID]
	s.mu.Unlock()

	if !ok {
		return errors.Wrap(service.ErrNotFound, "repair task")
	}

	if err := ih.SetIntensity(ctx, intensity); err != nil {
		return errors.Wrap(err, "set intensity")
	}

	return nil
}

// SetParallel changes parallelism of an ongoing repair.
func (s *Service) SetParallel(ctx context.Context, clusterID uuid.UUID, parallel int) error {
	s.mu.Lock()
	ih, ok := s.intensityHandlers[clusterID]
	s.mu.Unlock()

	if !ok {
		return errors.Wrap(service.ErrNotFound, "repair task")
	}

	if err := ih.SetParallel(ctx, parallel); err != nil {
		return errors.Wrap(err, "set parallel")
	}

	return nil
}

type intensityHandler struct {
	logger      log.Logger
	intensity   *atomic.Float64
	parallel    *atomic.Int64
	maxParallel int
}

const (
	maxIntensity    = 0
	defaultParallel = 0
)

// Sets repair intensity value.
func (i *intensityHandler) SetIntensity(ctx context.Context, intensity float64) error {
	if intensity < maxIntensity {
		return service.ErrValidate(errors.Errorf("setting invalid intensity value %.2f", intensity))
	}
	i.logger.Info(ctx, "Setting repair intensity", "value", intensity, "previous", i.intensity.Load())
	i.intensity.Store(intensity)

	return nil
}

// Sets repair parallel value.
func (i *intensityHandler) SetParallel(ctx context.Context, parallel int) error {
	if parallel < defaultParallel {
		return service.ErrValidate(errors.Errorf("setting invalid parallel value %d", parallel))
	}

	i.logger.Info(ctx, "Setting repair parallel", "value", parallel, "previous", i.parallel.Load())
	i.parallel.Store(int64(parallel))

	if parallel > i.maxParallel {
		i.logger.Info(ctx, "Requested parallel value will be capped to maximum possible", "requested", parallel, "maximum", i.maxParallel)
	}

	return nil
}

// Intensity returns stored value for intensity.
func (i *intensityHandler) Intensity() float64 {
	return i.intensity.Load()
}

// Parallel returns stored value for parallel.
func (i *intensityHandler) Parallel() int {
	return int(i.parallel.Load())
}

// MaxParallel returns maximum value of the parallel setting.
func (i *intensityHandler) MaxParallel() int {
	return i.maxParallel
}
