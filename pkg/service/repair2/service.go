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
	"github.com/scylladb/mermaid/pkg/scyllaclient"
	"github.com/scylladb/mermaid/pkg/service"
	"github.com/scylladb/mermaid/pkg/util/inexlist/dcfilter"
	"github.com/scylladb/mermaid/pkg/util/inexlist/ksfilter"
	"github.com/scylladb/mermaid/pkg/util/timeutc"
	"github.com/scylladb/mermaid/pkg/util/uuid"
	"golang.org/x/sync/errgroup"
)

// ClusterNameFunc returns name for a given ID.
type ClusterNameFunc func(ctx context.Context, clusterID uuid.UUID) (string, error)

// Service orchestrates clusterName repairs.
type Service struct {
	session gocqlx.Session
	config  Config

	clusterName  ClusterNameFunc
	scyllaClient scyllaclient.ProviderFunc
	logger       log.Logger

	intensityChannels map[uuid.UUID]chan float64
	mu                sync.Mutex
}

func NewService(session gocqlx.Session, config Config, clusterName ClusterNameFunc, scyllaClient scyllaclient.ProviderFunc, logger log.Logger) (*Service, error) {
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
		intensityChannels: make(map[uuid.UUID]chan float64),
	}, nil
}

// Runner creates a Runner that handles repairs.
func (s *Service) Runner() Runner {
	return Runner{service: s}
}

// GetTarget converts runner properties into repair Target.
func (s *Service) GetTarget(ctx context.Context, clusterID uuid.UUID, properties json.RawMessage, force bool) (Target, error) {
	p := defaultTaskProperties()

	// Parse task properties
	if err := json.Unmarshal(properties, &p); err != nil {
		return Target{}, service.ErrValidate(errors.Wrapf(err, "parse runner properties: %s", properties))
	}

	// Copy basic properties
	t := Target{
		FailFast:  p.FailFast,
		Continue:  p.Continue,
		Intensity: p.Intensity,
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

		// Ignore not replicated keyspaces
		if ring.Replication == scyllaclient.LocalStrategy {
			continue
		}

		// Ignore keyspaces not replicated in desired DCs
		if !dcs.HasAny(ring.Datacenters()...) {
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
			s.logger.Info(ctx, "Keyspace skipped because there're no enough replicas in target", "keyspace", keyspace)
			continue
		}

		// Add to the filter
		f.Add(keyspace, tables)
	}

	// Get the filtered units
	t.Units, err = f.Apply(force)
	if err != nil {
		return t, err
	}

	return t, nil
}

// Repair performs the repair process on the Target.
func (s *Service) Repair(ctx context.Context, clusterID, taskID, runID uuid.UUID, target Target) error {
	s.logger.Debug(ctx, "Repair",
		"cluster_id", clusterID,
		"task_id", taskID,
		"run_id", runID,
		"target", target,
	)

	// TODO add target validation
	// * DCs must be set
	// * If host is set token ranges must be set too
	// * token ranges and with hosts can only be set if host is set

	run := &Run{
		ClusterID: clusterID,
		TaskID:    taskID,
		ID:        runID,
		StartTime: timeutc.Now().UTC(),
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

	// Check the cluster partitioner
	p, err := client.Partitioner(ctx)
	if err != nil {
		return errors.Wrap(err, "get client partitioner name")
	}
	if p != scyllaclient.Murmur3Partitioner {
		return errors.Errorf("unsupported partitioner %s, the only supported partitioner is %s", p, scyllaclient.Murmur3Partitioner)
	}

	// Dynamic Intensity
	intensityCh, cleanup := s.newIntensityChannel(clusterID, target.Intensity)
	defer cleanup()

	// Create and init generator
	var (
		g  = newGenerator(intensityCh, s.config.GracefulShutdownTimeout, s.logger)
		wc int
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

	// Get hosts in all DCs
	status, err := client.Status(ctx)
	if err != nil {
		return errors.Wrap(err, "status")
	}

	// Validate that there are no hosts to repair down
	if down := status.DownHosts(); repairHosts.HasAny(down...) {
		return errors.Errorf("nodes are down: %s", strings.Join(down, ","))
	}

	hostRangesLimits, err := s.hostRangeLimits(ctx, client, repairHosts.List())
	if err != nil {
		return errors.Wrap(err, "host range limits")
	}
	g.SetHostRangeLimits(hostRangesLimits)

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
			return errors.Wrap(err, "datacenter latency measurement")
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

	// Init Generator
	g.Init(wc)

	// Create worker
	w := newWorker(g.Next(), g.Result(), client, s.logger)

	// Worker context doesn't derive from ctx, generator will handle graceful
	// shutdown. Generator must receive ctx.
	workerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	return eg.Wait()
}

func (s *Service) hostRangeLimits(ctx context.Context, client *scyllaclient.Client, hosts []string) (hostRangesLimit, error) {
	hrl := make(hostRangesLimit)

	for _, h := range hosts {
		totalMemory, err := client.TotalMemory(ctx, h)
		if err != nil {
			return nil, err
		}

		hrl[h] = s.maxRepairRangesInParallel(totalMemory)
		s.logger.Debug(ctx, "Setting host ranges in parallel", "limit", hrl[h], "host", h)
	}
	return hrl, nil
}

func (s *Service) maxRepairRangesInParallel(totalMemory int64) int {
	return int(float64(totalMemory) * 0.1 / (32 * 1024 * 1024))
}

func (s *Service) newIntensityChannel(clusterID uuid.UUID, intensity float64) (ch <-chan float64, cleanup func()) {
	intensityCh := make(chan float64, 1)
	intensityCh <- intensity

	s.mu.Lock()
	if _, ok := s.intensityChannels[clusterID]; ok {
		panic("two repairs for the same cluster are running")
	}
	s.intensityChannels[clusterID] = intensityCh
	s.mu.Unlock()

	return intensityCh, func() {
		s.mu.Lock()
		close(s.intensityChannels[clusterID])
		delete(s.intensityChannels, clusterID)
		s.mu.Unlock()
	}
}

// GetRun returns a run based on ID. If nothing was found mermaid.ErrNotFound
// is returned.
func (s *Service) GetRun(ctx context.Context, clusterID, taskID, runID uuid.UUID) (*Run, error) {
	return nil, errors.New("not implemented")
}

// GetProgress TODO implement
func (s *Service) GetProgress(ctx context.Context, clusterID, taskID, runID uuid.UUID) (Progress, error) {
	return Progress{}, errors.New("not implemented")
}

// SetIntensity changes intensity of ongoing repair.
func (s *Service) SetIntensity(ctx context.Context, clusterID uuid.UUID, intensity float64) error {
	s.mu.Lock()
	ch, ok := s.intensityChannels[clusterID]
	s.mu.Unlock()

	if !ok {
		return errors.Wrap(service.ErrNotFound, "repair task")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case ch <- intensity:
	default:
		// ch is full or already closed, generator hasn't applied previous change yet or just finished.
		return errors.New("intensity change was not applied")
	}

	return nil
}
