// Copyright (C) 2017 ScyllaDB

package backup

import (
	"context"
	"encoding/json"

	"github.com/gocql/gocql"
	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/gocqlx"
	"github.com/scylladb/gocqlx/qb"
	"github.com/scylladb/mermaid"
	"github.com/scylladb/mermaid/internal/httputil/middleware"
	"github.com/scylladb/mermaid/internal/inexlist/dcfilter"
	"github.com/scylladb/mermaid/internal/inexlist/ksfilter"
	"github.com/scylladb/mermaid/internal/timeutc"
	"github.com/scylladb/mermaid/schema"
	"github.com/scylladb/mermaid/scyllaclient"
	"github.com/scylladb/mermaid/uuid"
	"go.uber.org/multierr"
)

// ClusterNameFunc returns name for a given ID.
type ClusterNameFunc func(ctx context.Context, clusterID uuid.UUID) (string, error)

// Service orchestrates clusterName backups.
type Service struct {
	session *gocql.Session
	config  Config

	clusterName  ClusterNameFunc
	scyllaClient scyllaclient.ProviderFunc
	logger       log.Logger
}

func NewService(session *gocql.Session, config Config, clusterName ClusterNameFunc, scyllaClient scyllaclient.ProviderFunc, logger log.Logger) (*Service, error) {
	if session == nil || session.Closed() {
		return nil, errors.New("invalid session")
	}

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
		session:      session,
		config:       config,
		clusterName:  clusterName,
		scyllaClient: scyllaClient,
		logger:       logger,
	}, nil
}

// Runner creates a Runner that handles repairs.
func (s *Service) Runner() Runner {
	return Runner{service: s}
}

// GetTarget converts runner properties into repair Target.
// It also ensures configuration for the backup providers is registered on the
// targeted hosts.
func (s *Service) GetTarget(ctx context.Context, clusterID uuid.UUID, properties json.RawMessage, force bool) (Target, error) {
	p := defaultTaskProperties()
	t := Target{}

	if err := json.Unmarshal(properties, &p); err != nil {
		return t, mermaid.ErrValidate(errors.Wrapf(err, "failed to parse runner properties: %s", properties))
	}

	// Copy simple properties
	t.Retention = p.Retention
	t.Continue = p.Continue

	if p.Location == nil {
		return t, errors.Errorf("missing location")
	}

	client, err := s.scyllaClient(ctx, clusterID)
	if err != nil {
		return t, errors.Wrapf(err, "failed to get client")
	}

	// Get hosts in DCs
	dcMap, err := client.Datacenters(ctx)
	if err != nil {
		return t, errors.Wrap(err, "failed to read datacenters")
	}

	// Filter DCs
	if t.DC, err = dcfilter.Apply(dcMap, p.DC); err != nil {
		return t, err
	}

	// Validate location DCs
	if err := checkDCs(func(i int) (string, string) { return p.Location[i].DC, p.Location[i].String() }, len(p.Location), dcMap); err != nil {
		return t, errors.Wrap(err, "invalid location")
	}

	// Filter out definitions with missing dc.
	t.Location = filterDCLocations(p.Location, t.DC)
	if t.Location == nil {
		return t, errors.Errorf("failed to match locations to dcs")
	}
	t.RateLimit = filterDCLimits(p.RateLimit, t.DC)
	t.SnapshotParallel = filterDCLimits(p.SnapshotParallel, t.DC)
	t.UploadParallel = filterDCLimits(p.UploadParallel, t.DC)

	if err := checkAllDCsCovered(func(i int) string { return t.Location[i].DC }, len(t.Location), t.DC); err != nil {
		return t, errors.Wrap(err, "invalid location")
	}

	// Register providers with hosts
	if err := s.registerProviders(ctx, client, t.DC, dcMap); err != nil {
		return t, errors.Wrap(err, "failed to register backup providers")
	}

	// Validate that locations are accessible from the nodes
	if err := s.checkRemoteLocations(ctx, client, t.Location, t.DC, dcMap); err != nil {
		return t, errors.Wrap(err, "location not accessible")
	}

	// Validate rate limit DCs
	if err := checkDCs(dcLimitDCAtPos(p.RateLimit), len(p.RateLimit), dcMap); err != nil {
		return t, errors.Wrap(err, "invalid rate-limit")
	}

	// Validate upload parallel DCs
	if err := checkDCs(dcLimitDCAtPos(p.SnapshotParallel), len(p.SnapshotParallel), dcMap); err != nil {
		return t, errors.Wrap(err, "invalid snapshot-parallel")
	}

	// Validate snapshot parallel DCs
	if err := checkDCs(dcLimitDCAtPos(p.UploadParallel), len(p.UploadParallel), dcMap); err != nil {
		return t, errors.Wrap(err, "invalid upload-parallel")
	}

	// Filter keyspaces
	f, err := ksfilter.NewFilter(p.Keyspace)
	if err != nil {
		return t, err
	}

	keyspaces, err := client.Keyspaces(ctx)
	if err != nil {
		return t, errors.Wrapf(err, "failed to read keyspaces")
	}
	for _, keyspace := range keyspaces {
		tables, err := client.Tables(ctx, keyspace)
		if err != nil {
			return t, errors.Wrapf(err, "keyspace %s: failed to get tables", keyspace)
		}

		// Get the ring description and skip local data
		ring, err := client.DescribeRing(ctx, keyspace)
		if err != nil {
			return t, errors.Wrapf(err, "keyspace %s: failed to get ring description", keyspace)
		}
		if ring.Replication == scyllaclient.LocalStrategy {
			continue
		}

		// Add to the filter
		f.Add(keyspace, tables)
	}

	// Get the filtered units
	v, err := f.Apply(force)
	if err != nil {
		return t, err
	}

	// Copy units
	for _, u := range v {
		uu := Unit{
			Keyspace:  u.Keyspace,
			Tables:    u.Tables,
			AllTables: u.AllTables,
		}
		t.Units = append(t.Units, uu)
	}

	return t, nil
}

type diskSize struct {
	Size  int64
	Error error
}

// GetTargetSize calculates total size of the backup for the provided target.
func (s *Service) GetTargetSize(ctx context.Context, clusterID uuid.UUID, target Target) (int64, error) {
	s.logger.Info(ctx, "Calculating target size")
	client, err := s.scyllaClient(ctx, clusterID)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get client")
	}
	// Get hosts in all DCs
	dcMap, err := client.Datacenters(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to read datacenters")
	}
	// Get hosts in the given DCs
	hosts := dcHosts(dcMap, target.DC)
	if len(hosts) == 0 {
		return 0, errors.New("no matching hosts found")
	}

	results := make(chan diskSize)

	for _, h := range hosts {
		go func(h string) {
			for _, u := range target.Units {
				for _, t := range u.Tables {
					size, err := client.TableDiskSize(ctx, h, u.Keyspace, t)
					if err != nil {
						err = errors.Wrapf(err, "failed to get size of %s.%s on %s", u.Keyspace, t, h)
					}
					results <- diskSize{size, err}
				}
			}
		}(h)
	}

	var (
		total int64
		errs error
	)

	for range hosts {
		for _, u := range target.Units {
			for range u.Tables {
				result := <-results
				if result.Error != nil {
					errs = multierr.Append(errs, err)
				}
				total += result.Size
			}
		}
	}
	if errs != nil {
		total = 0
	}

	return total, errs
}

// checkRemoteLocations checks if each node has access to the remote location.
func (s *Service) checkRemoteLocations(ctx context.Context, client *scyllaclient.Client, locations []Location, dcs []string, dcMap map[string][]string) error {
	s.logger.Info(ctx, "Checking accessibility of remote locations")
	// Get hosts of the target DCs
	hosts := dcHosts(dcMap, dcs)
	if len(hosts) == 0 {
		return errors.New("no matching hosts found")
	}

	var (
		errs error
		res  = make(chan error)
	)

	for _, l := range locations {
		if l.DC != "" && !sliceContains(l.DC, dcs) {
			continue
		}
		lh := hosts
		if l.DC != "" {
			lh = dcHosts(dcMap, []string{l.DC})
			if len(lh) == 0 {
				s.logger.Error(ctx, "No matching hosts found for %s", l)
				continue
			}
		}

		for _, h := range lh {
			go func(h string, l Location) {
				res <- s.checkHostAccessibility(ctx, client, h, l)
			}(h, l)
		}

		for range lh {
			errs = multierr.Append(errs, <-res)
		}
	}

	return mermaid.ErrValidate(errs)
}

func (s *Service) checkHostAccessibility(ctx context.Context, client *scyllaclient.Client, h string, l Location) error {
	_, err := client.RcloneListDir(middleware.DontRetry(ctx), h, l.RemotePath(""), false)
	if err != nil {
		s.logger.Error(ctx, "Host location check FAILED", "host", h, "location", l, "error", err)
		var e error
		if scyllaclient.StatusCodeOf(err) == 404 {
			e = errors.Errorf("%s: failed to access %s make sure that it's correct and credentials are set", h, l)
		} else {
			e = errors.Wrapf(err, "%s: failed to access %s", h, l)
		}
		return e
	}
	s.logger.Info(ctx, "Host location check OK", "host", h, "location", l)
	return nil
}

// registerProviders ensures that configuration for supported backup providers
// is set on the provided hosts.
func (s *Service) registerProviders(ctx context.Context, client *scyllaclient.Client, dcs []string, dcMap map[string][]string) error {
	// Get hosts of the target DCs
	hosts := dcHosts(dcMap, dcs)
	if len(hosts) == 0 {
		return errors.New("no matching hosts found")
	}

	var errs error

	for _, pr := range SupportedProviders {
		for _, h := range hosts {
			s.logger.Info(ctx, "Registering backup provider", "host", h, "provider", pr)
			if err := registerProvider(ctx, client, pr, h, s.config); err != nil {
				errs = multierr.Append(errs, errors.Wrap(err, h))
			}
		}
	}

	return mermaid.ErrValidate(errs)
}

// Backup executes a backup on a given target.
func (s *Service) Backup(ctx context.Context, clusterID uuid.UUID, taskID uuid.UUID, runID uuid.UUID, target Target) error {
	s.logger.Debug(ctx, "Backup",
		"cluster_id", clusterID,
		"task_id", taskID,
		"run_id", runID,
		"target", target,
	)

	run := &Run{
		ClusterID: clusterID,
		TaskID:    taskID,
		ID:        runID,
		Units:     target.Units,
		DC:        target.DC,
		Location:  target.Location,
		StartTime: timeutc.Now().UTC(),
	}

	s.logger.Info(ctx, "Initializing backup",
		"cluster_id", run.ClusterID,
		"task_id", run.TaskID,
		"run_id", run.ID,
		"target", target,
	)

	// Get the cluster client
	client, err := s.scyllaClient(ctx, run.ClusterID)
	if err != nil {
		return errors.Wrap(err, "failed to get client proxy")
	}

	// Get hosts in all DCs
	dcMap, err := client.Datacenters(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to read datacenters")
	}

	// Get hosts in the given DCs
	hosts := dcHosts(dcMap, run.DC)
	if len(hosts) == 0 {
		return errors.New("no matching hosts found")
	}

	// Get host IDs
	hostIDs, err := client.HostIDs(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get host IDs")
	}

	// Create hostInfo for hosts
	hi, err := hostInfoFromHosts(hosts, dcMap, hostIDs, target.Location, target.RateLimit)
	if err != nil {
		return err
	}

	if target.Continue {
		if err := s.decorateWithPrevRun(ctx, run); err != nil {
			return err
		}
		// Update run with previous progress.
		if run.PrevID != uuid.Nil {
			s.putRunLogError(ctx, run)
		}
	}

	// Generate snapshot tag
	if run.SnapshotTag == "" {
		run.SnapshotTag = newSnapshotTag()
	}

	// Register the run
	if err := s.putRun(run); err != nil {
		return errors.Wrap(err, "failed to register the run")
	}

	// Create a worker
	w := worker{
		ClusterID:     clusterID,
		TaskID:        taskID,
		RunID:         runID,
		SnapshotTag:   run.SnapshotTag,
		Config:        s.config,
		Units:         run.Units,
		Client:        client,
		Logger:        s.logger.Named("worker"),
		OnRunProgress: s.putRunProgressLogError,
	}

	if run.PrevID == uuid.Nil {
		if err := w.Snapshot(ctx, hi, target.SnapshotParallel); err != nil {
			return err
		}
	}

	return w.Upload(ctx, hi, target.UploadParallel, target.Retention)
}

// decorateWithPrevRun gets task previous run and if it can be continued
// sets PrevID on the given run.
func (s *Service) decorateWithPrevRun(ctx context.Context, run *Run) error {
	prev, err := s.GetLastResumableRun(ctx, run.ClusterID, run.TaskID)
	if err == mermaid.ErrNotFound {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "failed to get previous run")
	}

	// Check if can continue from prev
	s.logger.Info(ctx, "Found previous run", "run_id", prev.ID, "prev_run_id", prev.PrevID)
	if timeutc.Since(prev.StartTime) > s.config.AgeMax {
		s.logger.Info(ctx, "Starting from scratch: previous run is too old")
		return nil
	}

	run.PrevID = prev.ID
	run.SnapshotTag = prev.SnapshotTag
	run.Units = prev.Units
	run.DC = prev.DC

	return nil
}

// GetLastResumableRun returns the the most recent started but not done run of
// the task, if there is a recent run that is completely done ErrNotFound is
// reported.
func (s *Service) GetLastResumableRun(ctx context.Context, clusterID, taskID uuid.UUID) (*Run, error) {
	s.logger.Debug(ctx, "GetLastResumableRun",
		"cluster_id", clusterID,
		"task_id", taskID,
	)

	stmt, names := qb.Select(schema.BackupRun.Name()).Where(
		qb.Eq("cluster_id"),
		qb.Eq("task_id"),
	).Limit(20).ToCql()

	q := gocqlx.Query(s.session.Query(stmt), names).BindMap(qb.M{
		"cluster_id": clusterID,
		"task_id":    taskID,
	})

	var runs []*Run
	if err := q.SelectRelease(&runs); err != nil {
		return nil, err
	}

	for _, r := range runs {
		prog, err := s.getProgress(r)
		if err != nil {
			return nil, err
		}
		size, uploaded := runProgress(r, prog)
		if size > 0 {
			if size == uploaded {
				break
			}
			return r, nil
		}
	}

	return nil, mermaid.ErrNotFound
}

// runProgress returns total size and uploaded bytes for all files belonging to
// the run.
func runProgress(run *Run, prog []*RunProgress) (int64, int64) {
	var size, uploaded int64
	if len(run.Units) == 0 {
		return size, uploaded
	}

	for i := range prog {
		size += prog[i].Size
		uploaded += prog[i].Uploaded
	}

	return size, uploaded
}

func (s *Service) getProgress(run *Run) ([]*RunProgress, error) {
	stmt, names := schema.BackupRunProgress.Select()

	q := gocqlx.Query(s.session.Query(stmt), names).BindMap(qb.M{
		"cluster_id": run.ClusterID,
		"task_id":    run.TaskID,
		"run_id":     run.ID,
	})

	var p []*RunProgress
	return p, q.SelectRelease(&p)
}

// putRun upserts a backup run.
func (s *Service) putRun(r *Run) error {
	stmt, names := schema.BackupRun.Insert()
	q := gocqlx.Query(s.session.Query(stmt), names).BindStruct(r)
	return q.ExecRelease()
}

// putRunLogError executes putRun and consumes the error.
func (s *Service) putRunLogError(ctx context.Context, r *Run) {
	if err := s.putRun(r); err != nil {
		s.logger.Error(ctx, "failed to update the run",
			"run", r,
			"error", err,
		)
	}
}

// putRunProgress upserts a backup run progress.
func (s *Service) putRunProgress(ctx context.Context, p *RunProgress) error {
	s.logger.Debug(ctx, "PutRunProgress", "run_progress", p)

	stmt, names := schema.BackupRunProgress.Insert()
	q := gocqlx.Query(s.session.Query(stmt), names).BindStruct(p)

	return q.ExecRelease()
}

// putRunProgressLogError executes putRunProgress and consumes the error.
func (s *Service) putRunProgressLogError(ctx context.Context, p *RunProgress) {
	if err := s.putRunProgress(ctx, p); err != nil {
		s.logger.Error(ctx, "Failed to update file progress",
			"progress", p,
			"error", err,
		)
	}
}

// GetRun returns a run based on ID. If nothing was found mermaid.ErrNotFound
// is returned.
func (s *Service) GetRun(ctx context.Context, clusterID, taskID, runID uuid.UUID) (*Run, error) {
	s.logger.Debug(ctx, "GetRun",
		"cluster_id", clusterID,
		"task_id", taskID,
		"run_id", runID,
	)

	stmt, names := schema.BackupRun.Get()

	q := gocqlx.Query(s.session.Query(stmt), names).BindMap(qb.M{
		"cluster_id": clusterID,
		"task_id":    taskID,
		"id":         runID,
	})

	var r Run
	return &r, q.GetRelease(&r)
}

// GetProgress aggregates progress for the run of the task and breaks it down
// by keyspace and table.json
// If nothing was found mermaid.ErrNotFound is returned.
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
	prog, err := s.getProgress(run)
	if err != nil {
		return Progress{}, err
	}

	p := aggregateProgress(run, prog)

	return p, nil
}
