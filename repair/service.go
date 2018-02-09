// Copyright (C) 2017 ScyllaDB

package repair

import (
	"context"
	"sort"
	"sync"

	"github.com/cespare/xxhash"
	"github.com/fatih/set"
	"github.com/gocql/gocql"
	"github.com/pkg/errors"
	"github.com/scylladb/gocqlx"
	"github.com/scylladb/gocqlx/qb"
	"github.com/scylladb/mermaid"
	"github.com/scylladb/mermaid/log"
	"github.com/scylladb/mermaid/schema"
	"github.com/scylladb/mermaid/scyllaclient"
	"github.com/scylladb/mermaid/timeutc"
	"github.com/scylladb/mermaid/uuid"
)

// globalClusterID is a special value used as a cluster ID for a global
// configuration.
var globalClusterID = uuid.NewFromUint64(0, 0)

// Service orchestrates cluster repairs.
type Service struct {
	session      *gocql.Session
	client       scyllaclient.ProviderFunc
	active       map[uuid.UUID]uuid.UUID // maps cluster ID to active run ID
	activeMu     sync.Mutex
	workerCtx    context.Context
	workerCancel context.CancelFunc
	wg           sync.WaitGroup
	logger       log.Logger
}

// NewService creates a new service instance.
func NewService(session *gocql.Session, p scyllaclient.ProviderFunc, l log.Logger) (*Service, error) {
	if session == nil || session.Closed() {
		return nil, errors.New("invalid session")
	}

	if p == nil {
		return nil, errors.New("invalid scylla provider")
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Service{
		session:      session,
		client:       p,
		active:       make(map[uuid.UUID]uuid.UUID),
		workerCtx:    ctx,
		workerCancel: cancel,
		logger:       l,
	}, nil
}

// FixRunStatus shall be called when the service starts to assure proper
// functioning. It iterates over all the repair units and marks running and
// stopping runs as stopped.
func (s *Service) FixRunStatus(ctx context.Context) error {
	s.logger.Debug(ctx, "FixRunStatus")

	stmt, _ := qb.Select(schema.RepairUnit.Name).ToCql()
	q := s.session.Query(stmt).WithContext(ctx)
	defer q.Release()

	iter := gocqlx.Iter(q)
	defer iter.Close()

	var u Unit
	for iter.StructScan(&u) {
		last, err := s.GetLastRun(ctx, &u)
		if err == mermaid.ErrNotFound {
			continue
		}
		if err != nil {
			return errors.Wrap(err, "failed to get last run of a unit")
		}

		switch last.Status {
		case StatusRunning, StatusStopping:
			last.Status = StatusStopped
			if err := s.putRun(ctx, last); err != nil {
				return errors.Wrap(err, "failed to update a run")
			}
			s.logger.Info(ctx, "Marked run as stopped", "unit", u, "run_id", last.ID)
		}
	}

	return iter.Close()
}

// Repair starts an asynchronous repair process.
func (s *Service) Repair(ctx context.Context, u *Unit, runID uuid.UUID) error {
	s.logger.Info(ctx, "Repair", "unit", u, "run_id", runID)

	r := Run{
		ClusterID: u.ClusterID,
		UnitID:    u.ID,
		ID:        runID,
		Keyspace:  u.Keyspace,
		Tables:    u.Tables,
		Status:    StatusRunning,
		StartTime: timeutc.Now(),
	}

	// fail updates a run and passes the error
	fail := func(err error) error {
		r.Status = StatusError
		r.Cause = err.Error()
		r.EndTime = timeutc.Now()
		s.putRunLogError(ctx, &r)
		return err
	}

	// validate the unit
	if err := u.Validate(); err != nil {
		return fail(mermaid.ParamError{Cause: errors.Wrap(err, "invalid unit")})
	}

	// get the unit configuration
	c, err := s.GetMergedUnitConfig(ctx, u)
	if err != nil {
		return fail(errors.Wrap(err, "failed to get a unit configuration"))
	}
	s.logger.Info(ctx, "Using config", "config", &c.Config)

	// if repair is disabled return an error
	if !*c.Config.Enabled {
		s.logger.Info(ctx, "Disabled")
		return fail(mermaid.ParamError{Cause: ErrDisabled})
	}

	// make sure no other repairs are being run on that cluster
	if err := s.tryLockCluster(&r); err != nil {
		s.logger.Debug(ctx, "Lock error", "error", err)
		return fail(mermaid.ParamError{Cause: ErrActiveRepair})
	}
	defer func() {
		if r.Status != StatusRunning {
			if err := s.unlockCluster(&r); err != nil {
				s.logger.Error(ctx, "Unlock error", "error", err)
			}
		}
	}()

	// get last started run of the unit
	prev, err := s.GetLastStartedRun(ctx, u)
	if err != nil && err != mermaid.ErrNotFound {
		return fail(errors.Wrap(err, "failed to get previous run"))
	}
	if prev != nil {
		s.logger.Info(ctx, "Found previous run", "prev", prev)
		switch {
		case prev.Status == StatusDone:
			s.logger.Info(ctx, "Starting from scratch: nothing too continue from")
			prev = nil
		case timeutc.Since(prev.StartTime) > DefaultRepairMaxAge:
			s.logger.Info(ctx, "Starting from scratch: previous run is too old")
			prev = nil
		}
	}
	if prev != nil {
		r.PrevID = prev.ID
	}

	// register the run
	if err := s.putRun(ctx, &r); err != nil {
		return fail(errors.Wrap(err, "failed to register the run"))
	}

	// get the cluster client
	cluster, err := s.client(ctx, u.ClusterID)
	if err != nil {
		return fail(errors.Wrap(err, "failed to get the cluster proxy"))
	}

	// get the cluster topology hash
	r.TopologyHash, err = s.topologyHash(ctx, cluster)
	if err != nil {
		return fail(errors.Wrap(err, "failed to get topology hash"))
	}

	// ensure topology did not change, if changed start from scratch
	if prev != nil {
		if r.TopologyHash != prev.TopologyHash {
			s.logger.Info(ctx, "Starting from scratch: topology changed",
				"run_id", r.ID,
				"prev_run_id", prev.ID,
			)
			prev = nil
			r.PrevID = uuid.Nil
		}
	}
	if err := s.putRun(ctx, &r); err != nil {
		return fail(errors.Wrap(err, "failed to update the run"))
	}

	// check keyspace and tables
	all, err := cluster.Tables(ctx, r.Keyspace)
	if err != nil {
		return fail(errors.Wrap(err, "failed to get the cluster table names for keyspace"))
	}
	if len(all) == 0 {
		return fail(errors.Errorf("missing or empty keyspace %q", r.Keyspace))
	}
	if err := validateTables(r.Tables, all); err != nil {
		return fail(errors.Wrapf(err, "keyspace %q", r.Keyspace))
	}

	// check the cluster partitioner
	p, err := cluster.Partitioner(ctx)
	if err != nil {
		return fail(errors.Wrap(err, "failed to get the cluster partitioner name"))
	}
	if p != scyllaclient.Murmur3Partitioner {
		return fail(errors.Errorf("unsupported partitioner %q, the only supported partitioner is %q", p, scyllaclient.Murmur3Partitioner))
	}

	// get the ring description
	_, ring, err := cluster.DescribeRing(ctx, u.Keyspace)
	if err != nil {
		return fail(errors.Wrap(err, "failed to get the ring description"))
	}

	// get local datacenter name
	dc, err := cluster.Datacenter(ctx)
	if err != nil {
		return fail(errors.Wrap(err, "failed to get the local datacenter name"))
	}
	s.logger.Debug(ctx, "Using DC", "dc", dc)

	// split token range into coordination hosts
	hostSegments, err := groupSegmentsByHost(dc, ring)
	if err != nil {
		return fail(errors.Wrap(err, "segmentation failed"))
	}

	// init empty progress
	for host := range hostSegments {
		p := RunProgress{
			ClusterID: r.ClusterID,
			UnitID:    r.UnitID,
			RunID:     r.ID,
			Host:      host,
		}
		if err := s.putRunProgress(ctx, &p); err != nil {
			return fail(errors.Wrapf(err, "failed to initialise the run progress %s", &p))
		}
	}

	// update progress from the previous run
	if prev != nil {
		prog, err := s.GetProgress(ctx, u, prev.ID)
		if err != nil {
			return fail(errors.Wrap(err, "failed to get the last run progress"))
		}

		// check if host did not change
		prevHosts := set.NewNonTS()
		for _, p := range prog {
			prevHosts.Add(p.Host)
		}
		hosts := set.NewNonTS()
		for host := range hostSegments {
			hosts.Add(host)
		}

		if diff := set.SymmetricDifference(prevHosts, hosts); !diff.IsEmpty() {
			s.logger.Info(ctx, "Starting from scratch: hosts changed check that all API hosts belong to the same DC",
				"run_id", r.ID,
				"prev_run_id", prev.ID,
				"old", prevHosts,
				"new", hosts,
				"diff", diff,
			)

			prev = nil
			r.PrevID = uuid.Nil
			if err := s.putRun(ctx, &r); err != nil {
				return fail(errors.Wrap(err, "failed to update the run"))
			}
		} else {
			for _, p := range prog {
				if p.started() {
					p.RunID = r.ID
					if err := s.putRunProgress(ctx, p); err != nil {
						return fail(errors.Wrapf(err, "failed to initialise the run progress %s", &p))
					}
				}
			}
		}
	}

	// spawn async repair
	wctx := log.WithTraceID(s.workerCtx)
	s.logger.Info(ctx, "Starting repair",
		"unit", u,
		"run_id", runID,
		"prev_run_id", r.PrevID,
		"worker_trace_id", log.TraceID(wctx),
	)
	s.wg.Add(1)
	go func() {
		defer func() {
			s.wg.Done()
			if v := recover(); v != nil {
				s.logger.Error(wctx, "Panic", "panic", v)
				fail(errors.Errorf("%s", v))
			}
			if err := s.unlockCluster(&r); err != nil {
				s.logger.Error(wctx, "Unlock error", "error", err)
			}
		}()
		if err := s.repair(wctx, u, &r, &c.Config, cluster, hostSegments); err != nil {
			fail(err)
		}
	}()

	return nil
}

func (s *Service) tryLockCluster(r *Run) error {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()

	owner := s.active[r.ClusterID]
	if owner != uuid.Nil {
		return errors.Errorf("cluster owned by another run: %s", owner)
	}

	s.active[r.ClusterID] = r.ID
	return nil
}

func (s *Service) unlockCluster(r *Run) error {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()

	owner := s.active[r.ClusterID]
	if owner == uuid.Nil {
		return errors.Errorf("not locked")
	}
	if owner != r.ID {
		return errors.Errorf("cluster owned by another run: %s", owner)
	}

	delete(s.active, r.ClusterID)
	return nil
}

func (s *Service) repair(ctx context.Context, u *Unit, r *Run, c *Config, cluster *scyllaclient.Client, hostSegments map[string][]*Segment) error {
	// shuffle hosts
	hosts := make([]string, 0, len(hostSegments))
	for host := range hostSegments {
		hosts = append(hosts, host)
	}
	sort.Slice(hosts, func(i, j int) bool {
		return xxhash.Sum64String(hosts[i]) < xxhash.Sum64String(hosts[j])
	})

	for _, host := range hosts {
		// ensure topology did not change
		th, err := s.topologyHash(ctx, cluster)
		if err != nil {
			s.logger.Info(ctx, "Topology check error", "error", err)
		} else if r.TopologyHash != th {
			return errors.Errorf("topology changed old hash: %s new hash: %s", r.TopologyHash, th)
		}

		// ping host
		if _, err := cluster.Ping(ctx, host); err != nil {
			return errors.Wrapf(err, "host %s not available", host)
		}

		w := worker{
			Unit:     u,
			Run:      r,
			Config:   c,
			Service:  s,
			Cluster:  cluster,
			Host:     host,
			Segments: hostSegments[host],

			segmentsPerRepair: DefaultSegmentsPerRepair,
			maxFailedSegments: DefaultMaxFailedSegments,
			pollInterval:      DefaultPollInterval,
			backoff:           DefaultBackoff,

			logger: s.logger.Named("worker").With("run_id", r.ID, "host", host),
		}
		if err := w.exec(ctx); err != nil {
			w.logger.Error(ctx, "Repair error", "error", err)
			return errors.Wrapf(err, "repair error")
		}

		if ctx.Err() != nil {
			s.logger.Info(ctx, "Aborted", "run_id", r.ID)
			return nil
		}

		stopped, err := s.isStopped(ctx, u, r.ID)
		if err != nil {
			w.logger.Error(ctx, "Service error", "error", err)
		}

		if stopped {
			r.Status = StatusStopped
			r.EndTime = timeutc.Now()
			s.putRunLogError(ctx, r)

			s.logger.Info(ctx, "Stopped", "unit", u, "run_id", r.ID)
			return nil
		}
	}

	r.Status = StatusDone
	r.EndTime = timeutc.Now()
	s.putRunLogError(ctx, r)

	s.logger.Info(ctx, "Done", "run_id", r.ID)

	return nil
}

func (s *Service) topologyHash(ctx context.Context, cluster *scyllaclient.Client) (uuid.UUID, error) {
	tokens, err := cluster.Tokens(ctx)
	if err != nil {
		return uuid.Nil, errors.Wrap(err, "failed to get the cluster tokens")
	}

	return topologyHash(tokens), nil
}

// GetLastRun returns the the most recent run of the unit.
func (s *Service) GetLastRun(ctx context.Context, u *Unit) (*Run, error) {
	s.logger.Debug(ctx, "GetLastRun", "unit", u)

	// validate the unit
	if err := u.Validate(); err != nil {
		return nil, mermaid.ParamError{Cause: errors.Wrap(err, "invalid unit")}
	}

	stmt, names := qb.Select(schema.RepairRun.Name).
		Where(
			qb.Eq("cluster_id"),
			qb.Eq("unit_id"),
		).Limit(1).
		ToCql()

	q := gocqlx.Query(s.session.Query(stmt).WithContext(ctx), names).BindMap(qb.M{
		"cluster_id": u.ClusterID,
		"unit_id":    u.ID,
	})
	defer q.Release()

	if q.Err() != nil {
		return nil, q.Err()
	}

	var r Run
	if err := gocqlx.Get(&r, q.Query); err != nil {
		return nil, err
	}

	return &r, nil
}

// GetLastStartedRun returns the the most recent run of the unit that started
// the repair.
func (s *Service) GetLastStartedRun(ctx context.Context, u *Unit) (*Run, error) {
	s.logger.Debug(ctx, "GetLastStartedRun", "unit", u)

	// validate the unit
	if err := u.Validate(); err != nil {
		return nil, mermaid.ParamError{Cause: errors.Wrap(err, "invalid unit")}
	}

	stmt, names := qb.Select(schema.RepairRun.Name).
		Where(
			qb.Eq("cluster_id"),
			qb.Eq("unit_id"),
		).Limit(100).ToCql()

	q := gocqlx.Query(s.session.Query(stmt).WithContext(ctx), names).BindMap(qb.M{
		"cluster_id": u.ClusterID,
		"unit_id":    u.ID,
	})
	defer q.Release()

	if q.Err() != nil {
		return nil, q.Err()
	}

	var runs []*Run
	if err := gocqlx.Select(&runs, q.Query); err != nil {
		return nil, err
	}

	for _, r := range runs {
		if r.Status != StatusError {
			return r, nil
		}

		// check if repair started
		p, err := s.getAllHostsProgress(ctx, u, r.ID)
		if err != nil {
			return nil, err
		}
		if len(p) > 0 {
			return r, nil
		}
	}

	return nil, mermaid.ErrNotFound
}

// GetRun returns a run based on ID. If nothing was found mermaid.ErrNotFound
// is returned.
func (s *Service) GetRun(ctx context.Context, u *Unit, runID uuid.UUID) (*Run, error) {
	s.logger.Debug(ctx, "GetRun", "unit", u, "run_id", runID)

	// validate the unit
	if err := u.Validate(); err != nil {
		return nil, mermaid.ParamError{Cause: errors.Wrap(err, "invalid unit")}
	}

	stmt, names := schema.RepairRun.Get()

	q := gocqlx.Query(s.session.Query(stmt).WithContext(ctx), names).BindMap(qb.M{
		"cluster_id": u.ClusterID,
		"unit_id":    u.ID,
		"id":         runID,
	})
	defer q.Release()

	if q.Err() != nil {
		return nil, q.Err()
	}

	var r Run
	if err := gocqlx.Get(&r, q.Query); err != nil {
		return nil, err
	}

	return &r, nil
}

// putRun upserts a repair run.
func (s *Service) putRun(ctx context.Context, r *Run) error {
	s.logger.Debug(ctx, "PutRun", "run", r)

	stmt, names := schema.RepairRun.Insert()
	q := gocqlx.Query(s.session.Query(stmt).WithContext(ctx), names).BindStruct(r)

	return q.ExecRelease()
}

// putRunLogError executes putRun and consumes the error.
func (s *Service) putRunLogError(ctx context.Context, r *Run) {
	if err := s.putRun(ctx, r); err != nil {
		s.logger.Error(ctx, "Cannot update the run",
			"run", &r,
			"error", err,
		)
	}
}

// StopRun marks a running repair as stopping.
func (s *Service) StopRun(ctx context.Context, u *Unit, runID uuid.UUID) error {
	s.logger.Debug(ctx, "StopRun", "unit", u, "run_id", runID)

	// validate the unit
	if err := u.Validate(); err != nil {
		return mermaid.ParamError{Cause: errors.Wrap(err, "invalid unit")}
	}

	r, err := s.GetRun(ctx, u, runID)
	if err != nil {
		return err
	}

	if r.Status != StatusRunning {
		return errors.New("not running")
	}

	s.logger.Info(ctx, "Stopping repair", "unit", u, "run_id", runID)
	r.Status = StatusStopping

	return s.putRun(ctx, r)
}

// isStopped checks if repair is in StatusStopping or StatusStopped.
func (s *Service) isStopped(ctx context.Context, u *Unit, runID uuid.UUID) (bool, error) {
	s.logger.Debug(ctx, "isStopped", "unit", u, "run_id", runID)

	stmt, names := schema.RepairRun.Get("status")
	q := gocqlx.Query(s.session.Query(stmt).WithContext(ctx), names).BindMap(qb.M{
		"cluster_id": u.ClusterID,
		"unit_id":    u.ID,
		"id":         runID,
	})
	if q.Err() != nil {
		return false, q.Err()
	}

	var v Status
	if err := q.Query.Scan(&v); err != nil {
		return false, err
	}

	return v == StatusStopping || v == StatusStopped, nil
}

// GetProgress returns run progress. If nothing was found mermaid.ErrNotFound
// is returned.
func (s *Service) GetProgress(ctx context.Context, u *Unit, runID uuid.UUID, hosts ...string) ([]*RunProgress, error) {
	s.logger.Debug(ctx, "GetProgress", "unit", u, "run_id", runID)

	// validate the unit
	if err := u.Validate(); err != nil {
		return nil, mermaid.ParamError{Cause: errors.Wrap(err, "invalid unit")}
	}

	if len(hosts) == 0 {
		return s.getAllHostsProgress(ctx, u, runID)
	}

	return s.getHostProgress(ctx, u, runID, hosts...)
}

func (s *Service) getAllHostsProgress(ctx context.Context, u *Unit, runID uuid.UUID) ([]*RunProgress, error) {
	stmt, names := schema.RepairRunProgress.Select()
	q := gocqlx.Query(s.session.Query(stmt).WithContext(ctx), names)
	defer q.Release()

	q.BindMap(qb.M{
		"cluster_id": u.ClusterID,
		"unit_id":    u.ID,
		"run_id":     runID,
	})
	if q.Err() != nil {
		return nil, q.Err()
	}

	var p []*RunProgress
	return p, gocqlx.Select(&p, q.Query)
}

func (s *Service) getHostProgress(ctx context.Context, u *Unit, runID uuid.UUID, hosts ...string) ([]*RunProgress, error) {
	stmt, names := schema.RepairRunProgress.SelectBuilder().Where(qb.Eq("host")).ToCql()
	q := gocqlx.Query(s.session.Query(stmt).WithContext(ctx), names)
	defer q.Release()

	var p []*RunProgress

	m := qb.M{
		"cluster_id": u.ClusterID,
		"unit_id":    u.ID,
		"run_id":     runID,
	}

	for _, h := range hosts {
		m["host"] = h

		q.BindMap(m)
		if q.Err() != nil {
			return nil, q.Err()
		}

		var v []*RunProgress
		if err := gocqlx.Select(&v, q.Query); err != nil {
			return nil, err
		}

		p = append(p, v...)
	}

	return p, nil
}

// putRunProgress upserts a repair run.
func (s *Service) putRunProgress(ctx context.Context, p *RunProgress) error {
	s.logger.Debug(ctx, "PutRunProgress", "run_progress", p)

	stmt, names := schema.RepairRunProgress.Insert()
	q := gocqlx.Query(s.session.Query(stmt).WithContext(ctx), names).BindStruct(p)

	return q.ExecRelease()
}

// GetMergedUnitConfig returns a merged configuration for a unit.
// The configuration has no nil values. If any of the source configurations are
// disabled the resulting configuration is disabled. For other fields first
// matching configuration is used.
func (s *Service) GetMergedUnitConfig(ctx context.Context, u *Unit) (*ConfigInfo, error) {
	s.logger.Debug(ctx, "GetMergedUnitConfig", "unit", u)

	// validate the unit
	if err := u.Validate(); err != nil {
		return nil, mermaid.ParamError{Cause: errors.Wrap(err, "invalid unit")}
	}

	order := []ConfigSource{
		{
			ClusterID:  u.ClusterID,
			Type:       UnitConfig,
			ExternalID: u.ID.String(),
		},
		{
			ClusterID:  u.ClusterID,
			Type:       KeyspaceConfig,
			ExternalID: u.Keyspace,
		},
		{
			ClusterID: u.ClusterID,
			Type:      ClusterConfig,
		},
		{
			ClusterID: globalClusterID,
			Type:      tenantConfig,
		},
	}

	all := make([]*Config, 0, len(order))
	src := order[:]

	for _, o := range order {
		c, err := s.GetConfig(ctx, o)
		// no entry
		if err == mermaid.ErrNotFound {
			continue
		}
		if err != nil {
			return nil, err
		}

		// add result
		all = append(all, c)
		src = append(src, o)
	}

	return mergeConfigs(all, src)
}

// GetConfig returns repair configuration for a given object. If nothing was
// found mermaid.ErrNotFound is returned.
func (s *Service) GetConfig(ctx context.Context, src ConfigSource) (*Config, error) {
	s.logger.Debug(ctx, "GetConfig", "source", src)

	stmt, names := schema.RepairConfig.Get()

	q := gocqlx.Query(s.session.Query(stmt).WithContext(ctx), names).BindStruct(src)
	if q.Err() != nil {
		return nil, q.Err()
	}

	var c Config
	if err := gocqlx.Iter(q.Query).Unsafe().Get(&c); err != nil {
		return nil, err
	}

	return &c, nil
}

// PutConfig upserts repair configuration for a given object.
func (s *Service) PutConfig(ctx context.Context, src ConfigSource, c *Config) error {
	s.logger.Debug(ctx, "PutConfig", "source", src, "config", c)

	if err := c.Validate(); err != nil {
		return mermaid.ParamError{Cause: errors.Wrap(err, "invalid config")}
	}

	stmt, names := schema.RepairConfig.Insert()

	q := gocqlx.Query(s.session.Query(stmt).WithContext(ctx), names).BindStructMap(c, qb.M{
		"cluster_id":  src.ClusterID,
		"type":        src.Type,
		"external_id": src.ExternalID,
	})

	return q.ExecRelease()
}

// DeleteConfig removes repair configuration for a given object.
func (s *Service) DeleteConfig(ctx context.Context, src ConfigSource) error {
	s.logger.Debug(ctx, "DeleteConfig", "source", src)

	stmt, names := schema.RepairConfig.Delete()
	q := gocqlx.Query(s.session.Query(stmt).WithContext(ctx), names).BindStruct(src)

	return q.ExecRelease()
}

// SyncUnits ensures that for every keyspace there is a Unit. If there is no
// unit it will be created
func (s *Service) SyncUnits(ctx context.Context, clusterID uuid.UUID) error {
	s.logger.Debug(ctx, "SyncUnits", "cluster_id", clusterID)

	// get the cluster client
	cluster, err := s.client(ctx, clusterID)
	if err != nil {
		return errors.Wrap(err, "failed to get the cluster proxy")
	}

	// cluster keyspaces
	keyspaces, err := cluster.Keyspaces(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list keyspaces")
	}

	ck := set.NewNonTS()
	for _, k := range keyspaces {
		ck.Add(k)
	}

	// database keyspaces
	units, err := s.ListUnits(ctx, clusterID, &UnitFilter{})
	if err != nil {
		return errors.Wrap(err, "failed to list units")
	}

	dbk := set.NewNonTS()
	for _, u := range units {
		dbk.Add(u.Keyspace)
	}

	names := set.NewNonTS()
	for _, u := range units {
		names.Add(u.Name)
	}

	var dbErr error

	// add missing keyspaces
	set.Difference(ck, dbk).Each(func(i interface{}) bool {
		u := &Unit{ClusterID: clusterID, Keyspace: i.(string)}

		if !names.Has(i) {
			u.Name = u.Keyspace
		}

		dbErr = s.PutUnit(ctx, u)
		return dbErr == nil
	})
	if dbErr != nil {
		return dbErr
	}

	// delete dropped keyspaces
	set.Difference(dbk, ck).Each(func(i interface{}) bool {
		k := i.(string)
		for _, u := range units {
			if u.Keyspace == k {
				dbErr = s.DeleteUnit(ctx, clusterID, u.ID)
				if dbErr != nil {
					return false
				}
			}
		}
		return true
	})

	return dbErr
}

// ListUnits returns all the units in the cluster.
func (s *Service) ListUnits(ctx context.Context, clusterID uuid.UUID, f *UnitFilter) ([]*Unit, error) {
	s.logger.Debug(ctx, "ListUnits", "cluster_id", clusterID, "filter", f)

	// validate the filter
	if err := f.Validate(); err != nil {
		return nil, mermaid.ParamError{Cause: errors.Wrap(err, "invalid filter")}
	}

	stmt, names := schema.RepairUnit.Select()

	q := gocqlx.Query(s.session.Query(stmt).WithContext(ctx), names).BindMap(qb.M{
		"cluster_id": clusterID,
	})
	defer q.Release()

	if q.Err() != nil {
		return nil, q.Err()
	}

	var units []*Unit
	if err := gocqlx.Select(&units, q.Query); err != nil {
		return nil, err
	}

	// nothing to filter
	if f.Name == "" {
		return units, nil
	}

	filtered := units[:0]
	for _, u := range units {
		if u.Name == f.Name {
			filtered = append(filtered, u)
		}
	}
	for i := len(filtered); i < len(units); i++ {
		units[i] = nil
	}

	return filtered, nil
}

// GetUnit returns repair unit based on ID or name. If nothing was found
// mermaid.ErrNotFound is returned.
func (s *Service) GetUnit(ctx context.Context, clusterID uuid.UUID, idOrName string) (*Unit, error) {
	if id, err := uuid.Parse(idOrName); err == nil {
		return s.GetUnitByID(ctx, clusterID, id)
	}

	return s.GetUnitByName(ctx, clusterID, idOrName)
}

// GetUnitByID returns repair unit based on ID. If nothing was found
// mermaid.ErrNotFound is returned.
func (s *Service) GetUnitByID(ctx context.Context, clusterID, id uuid.UUID) (*Unit, error) {
	s.logger.Debug(ctx, "GetUnitByID", "cluster_id", clusterID, "id", id)

	stmt, names := schema.RepairUnit.Get()

	q := gocqlx.Query(s.session.Query(stmt).WithContext(ctx), names).BindMap(qb.M{
		"cluster_id": clusterID,
		"id":         id,
	})
	defer q.Release()

	if q.Err() != nil {
		return nil, q.Err()
	}

	var u Unit
	if err := gocqlx.Get(&u, q.Query); err != nil {
		return nil, err
	}

	return &u, nil
}

// GetUnitByName returns repair unit based on name. If nothing was found
// mermaid.ErrNotFound is returned.
func (s *Service) GetUnitByName(ctx context.Context, clusterID uuid.UUID, name string) (*Unit, error) {
	s.logger.Debug(ctx, "GetUnitByName", "cluster_id", clusterID, "name", name)

	units, err := s.ListUnits(ctx, clusterID, &UnitFilter{Name: name})
	if err != nil {
		return nil, err
	}

	switch len(units) {
	case 0:
		return nil, mermaid.ErrNotFound
	case 1:
		return units[0], nil
	default:
		return nil, errors.Errorf("multiple units share the same name %q", name)
	}
}

// PutUnit upserts a repair unit, unit instance must pass Validate() checks.
// If u.ID == uuid.Nil a new one is generated.
func (s *Service) PutUnit(ctx context.Context, u *Unit) error {
	s.logger.Debug(ctx, "PutUnit", "unit", u)
	if u == nil {
		return errors.New("nil unit")
	}

	if u.ID == uuid.Nil {
		var err error
		if u.ID, err = uuid.NewRandom(); err != nil {
			return errors.Wrap(err, "couldn't generate random UUID for Unit")
		}
	}

	// validate the unit
	if err := u.Validate(); err != nil {
		return mermaid.ParamError{Cause: errors.Wrap(err, "invalid unit")}
	}

	// check for conflicting names
	if u.Name != "" {
		conflict, err := s.GetUnitByName(ctx, u.ClusterID, u.Name)
		if err != mermaid.ErrNotFound {
			if err != nil {
				return err
			}
			if conflict.ID != u.ID {
				return mermaid.ParamError{Cause: errors.Errorf("name conflict on %q", u.Name)}
			}
		}
	}

	stmt, names := schema.RepairUnit.Insert()
	q := gocqlx.Query(s.session.Query(stmt).WithContext(ctx), names).BindStruct(u)

	return q.ExecRelease()
}

// DeleteUnit removes repair based on ID.
func (s *Service) DeleteUnit(ctx context.Context, clusterID, id uuid.UUID) error {
	s.logger.Debug(ctx, "DeleteUnit", "cluster_id", clusterID, "id", id)

	stmt, names := schema.RepairUnit.Delete()

	q := gocqlx.Query(s.session.Query(stmt).WithContext(ctx), names).BindMap(qb.M{
		"cluster_id": clusterID,
		"id":         id,
	})

	return q.ExecRelease()
}

// Close terminates all the worker routines.
func (s *Service) Close() {
	s.workerCancel()
	s.wg.Wait()
}
