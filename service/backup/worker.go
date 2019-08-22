// Copyright (C) 2017 ScyllaDB

package backup

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/mermaid/internal/timeutc"
	"github.com/scylladb/mermaid/scyllaclient"
	"github.com/scylladb/mermaid/uuid"
)

type worker struct {
	clusterID uuid.UUID
	taskID    uuid.UUID
	runID     uuid.UUID

	config Config
	units  []Unit
	client *scyllaclient.Client
	logger log.Logger

	OnRunProgress func(ctx context.Context, p *RunProgress)
}

type hostInfo struct {
	DC        string
	IP        string
	ID        string
	Location  Location
	RateLimit DCLimit
}

func (w *worker) Snapshot(ctx context.Context, hosts []hostInfo, limits []DCLimit) (err error) {
	w.logger.Info(ctx, "Starting snapshot procedure")
	defer func() {
		if err != nil {
			w.logger.Error(ctx, "Snapshot procedure completed with error(s) see exact errors above")
		} else {
			w.logger.Info(ctx, "Snapshot procedure completed")
		}
	}()

	return inParallelWithLimits(hosts, limits, func(h hostInfo) error {
		w.logger.Info(ctx, "Executing snapshot procedure on host", "host", h.IP)
		err := w.snapshotHost(ctx, h)
		if err != nil {
			w.logger.Error(ctx, "Snapshot procedure failed on host", "host", h.IP, "error", err)
		} else {
			w.logger.Info(ctx, "Done executing snapshot procedure on host", "host", h.IP)
		}
		return err
	})
}

func (w *worker) snapshotHost(ctx context.Context, h hostInfo) error {
	if err := w.checkAvailableDiskSpace(ctx, h); err != nil {
		return errors.Wrap(err, "disk space check")
	}
	if err := w.takeSnapshot(ctx, h); err != nil {
		return errors.Wrap(err, "failed to take snapshot")
	}
	if err := w.deleteOldSnapshots(ctx, h); err != nil {
		// Not a fatal error we can continue, just log the error
		w.logger.Error(ctx, "Failed to delete old snapshots", "error", err)
	}
	dirs, err := w.findSnapshotDirs(ctx, h)
	if err != nil {
		return errors.Wrap(err, "failed to list snapshot dirs")
	}
	if err := w.initProgress(ctx, dirs); err != nil {
		return errors.Wrap(err, "failed to initialize progress")
	}
	return nil
}

func (w *worker) Upload(ctx context.Context, hosts []hostInfo, limits []DCLimit, progress []*RunProgress) (err error) {
	w.logger.Info(ctx, "Starting upload procedure")
	defer func() {
		if err != nil {
			w.logger.Error(ctx, "Upload procedure completed with error(s) see exact errors above")
		} else {
			w.logger.Info(ctx, "Upload procedure completed")
		}
	}()

	return inParallelWithLimits(hosts, limits, func(h hostInfo) error {
		w.logger.Info(ctx, "Executing upload procedure on host", "host", h.IP)
		err := w.uploadHost(ctx, h, progress)
		if err != nil {
			w.logger.Error(ctx, "Upload procedure failed on host", "host", h.IP, "error", err)
		} else {
			w.logger.Info(ctx, "Done executing upload procedure on host", "host", h.IP)
		}
		return err
	})
}

func (w *worker) uploadHost(ctx context.Context, h hostInfo, progress []*RunProgress) error {
	if err := w.register(ctx, h); err != nil {
		return errors.Wrap(err, "failed to register remote")
	}
	if err := w.setRateLimit(ctx, h); err != nil {
		return errors.Wrap(err, "failed to set rate limit")
	}

	dirs, err := w.findSnapshotDirs(ctx, h)
	if err != nil {
		return errors.Wrap(err, "failed to list snapshot dirs")
	}
	for _, d := range dirs {
		// Check if we should attach to a previous job and wait for it to complete.
		if err := w.attachToJob(ctx, h, d, progress); err != nil {
			return errors.Wrap(err, "failed to attach to the agent job")
		}
		// Start new upload with new job.
		if err := w.uploadSnapshotDir(ctx, h, d, progress); err != nil {
			return errors.Wrap(err, "failed to upload snapshot")
		}
	}
	return nil
}

func (w *worker) attachToJob(ctx context.Context, h hostInfo, d snapshotDir, progress []*RunProgress) error {
	if jobID := w.snapshotJobID(ctx, d, progress); jobID != uuid.Nil {
		w.logger.Info(ctx, "Attaching to the previous agent job",
			"host", h.IP,
			"keyspace", d.Keyspace,
			"tag", snapshotTag(w.runID),
			"jobid", jobID.String(),
		)
		if err := w.waitJob(ctx, jobID, d, progress); err != nil {
			return err
		}
	}
	return nil
}

// snapshotJobID returns the id of the job that was last responsible for
// uploading the snapshot directory.
// If it's not available it will return uuid.Nil
func (w *worker) snapshotJobID(ctx context.Context, d snapshotDir, progress []*RunProgress) uuid.UUID {
	for _, p := range w.getSnapshotDirProgress(d, progress) {
		if p.AgentJobID == uuid.Nil || p.Size == p.Uploaded {
			continue
		}
		status, _ := w.getJobStatus(ctx, p.AgentJobID, d) //nolint:errcheck
		switch status {
		case jobError:
			return uuid.Nil
		case jobNotFound:
			return uuid.Nil
		case jobSuccess:
			return p.AgentJobID
		case jobRunning:
			return p.AgentJobID
		}
	}
	return uuid.Nil
}

func (w *worker) checkAvailableDiskSpace(ctx context.Context, h hostInfo) error {
	freePercent, err := w.diskFreePercent(ctx, h)
	if err != nil {
		return err
	}
	w.logger.Info(ctx, "Available disk space", "host", h.IP, "percent", freePercent)
	if freePercent < w.config.DiskSpaceFreeMinPercent {
		return errors.New("not enough disk space")
	}
	return nil
}

func (w *worker) diskFreePercent(ctx context.Context, h hostInfo) (int, error) {
	du, err := w.client.RcloneDiskUsage(ctx, h.IP, dataDir)
	if err != nil {
		return 0, err
	}
	return int(100 * (float64(du.Free) / float64(du.Total))), nil
}

func (w *worker) takeSnapshot(ctx context.Context, h hostInfo) error {
	for _, u := range w.units {
		w.logger.Info(ctx, "Taking snapshot", "host", h.IP, "keyspace", u.Keyspace, "tag", snapshotTag(w.runID))
		if err := w.client.TakeSnapshot(ctx, h.IP, snapshotTag(w.runID), u.Keyspace, u.Tables...); err != nil {
			return errors.Wrapf(err, "keyspace %s: snapshot failed", u.Keyspace)
		}
	}
	return nil
}

func (w *worker) deleteOldSnapshots(ctx context.Context, h hostInfo) error {
	tags, err := w.client.Snapshots(ctx, h.IP)
	if err != nil {
		return err
	}

	for _, t := range tags {
		if claimTag(t) && t != snapshotTag(w.runID) {
			w.logger.Info(ctx, "Deleting old snapshot", "host", h.IP, "tag", t)
			if err := w.client.DeleteSnapshot(ctx, h.IP, t); err != nil {
				return err
			}
		}
	}

	return nil
}

// snapshotDir represents a remote directory containing a table snapshot.
type snapshotDir struct {
	Host     string
	Unit     int64
	Path     string
	Keyspace string
	Table    string
	Version  string
	Size     int64
}

func (w *worker) findSnapshotDirs(ctx context.Context, h hostInfo) ([]snapshotDir, error) {
	var dirs []snapshotDir

	r := regexp.MustCompile("^([A-Za-z0-9_]+)-([a-f0-9]{32})/snapshots/" + snapshotTag(w.runID) + "$")

	for i, u := range w.units {
		w.logger.Debug(ctx, "Inspecting snapshot",
			"host", h.IP,
			"keyspace", u.Keyspace,
			"tag", snapshotTag(w.runID),
			"dir", keyspaceDir(u.Keyspace),
		)

		list, err := w.client.RcloneListDir(ctx, h.IP, keyspaceDir(u.Keyspace), true)
		if err != nil {
			return nil, err
		}
		curPath := "/"
		for _, f := range list {
			// Accumulate size of all files in a snapshot directory
			if !f.IsDir && strings.HasPrefix(f.Path, curPath) {
				dirs[len(dirs)-1].Size += f.Size
				continue
			}
			// Match snapshot directories
			m := r.FindStringSubmatch(f.Path)
			if m == nil {
				continue
			}
			dirs = append(dirs, snapshotDir{
				Host:     h.IP,
				Unit:     int64(i),
				Path:     f.Path,
				Keyspace: u.Keyspace,
				Table:    m[1],
				Version:  m[2],
			})
			curPath = f.Path
		}
	}

	return dirs, nil
}

func (w *worker) initProgress(ctx context.Context, dirs []snapshotDir) error {
	for _, d := range dirs {
		dirPath := path.Join(keyspaceDir(d.Keyspace), d.Path)
		w.logger.Debug(ctx, "Initializing progress",
			"host", d.Host,
			"keyspace", d.Keyspace,
			"tag", snapshotTag(w.runID),
			"dir", dirPath,
		)
		list, err := w.client.RcloneListDir(ctx, d.Host, dirPath, false)
		if err != nil {
			return err
		}
		for _, l := range list {
			if l.IsDir {
				continue
			}
			p := &RunProgress{
				ClusterID: w.clusterID,
				TaskID:    w.taskID,
				RunID:     w.runID,
				Host:      d.Host,
				Unit:      d.Unit,
				TableName: d.Table,
				FileName:  l.Name,
				Size:      l.Size,
			}
			w.onRunProgress(ctx, p)
		}
	}
	return nil
}

func (w *worker) register(ctx context.Context, h hostInfo) error {
	w.logger.Info(ctx, "Registering remote", "host", h.IP, "location", h.Location)

	if h.Location.Provider != S3 {
		return errors.Errorf("unsupported provider %s", h.Location.Provider)
	}

	p := scyllaclient.S3Params{
		EnvAuth:         true,
		DisableChecksum: true,

		Endpoint: w.config.TestS3Endpoint,
	}
	return w.client.RcloneRegisterS3Remote(ctx, h.IP, h.Location.RemoteName(), p)
}

func (w *worker) setRateLimit(ctx context.Context, h hostInfo) error {
	w.logger.Info(ctx, "Setting rate limit", "host", h.IP, "limit", h.RateLimit.Limit)
	return w.client.RcloneSetBandwidthLimit(ctx, h.IP, h.RateLimit.Limit)
}

const manifestFile = "manifest.json"

func (w *worker) uploadSnapshotDir(ctx context.Context, h hostInfo, d snapshotDir, progress []*RunProgress) error {
	w.logger.Info(ctx, "Uploading table snapshot",
		"host", h.IP,
		"keyspace", d.Keyspace,
		"table", d.Table,
		"location", h.Location,
	)

	// Upload manifest
	var (
		manifestDst = path.Join(h.Location.RemotePath(w.remoteMetaDir(h, d)), manifestFile)
		manifestSrc = path.Join(dataDir, d.Keyspace, d.Path, manifestFile)
	)
	if err := w.uploadFile(ctx, manifestDst, manifestSrc, d, progress); err != nil {
		return errors.Wrapf(err, "host %s: failed to copy %s to %s", h, manifestSrc, manifestDst)
	}

	// Upload sstables
	var (
		dataDst = h.Location.RemotePath(w.remoteSSTableDir(h, d))
		dataSrc = path.Join(dataDir, d.Keyspace, d.Path)
	)
	if err := w.uploadDir(ctx, dataDst, dataSrc, d, progress); err != nil {
		return errors.Wrapf(err, "host %s: failed to copy %s to %s", h, dataSrc, dataDst)
	}

	return nil
}

func (w *worker) uploadFile(ctx context.Context, dst, src string, d snapshotDir, progress []*RunProgress) error {
	w.logger.Debug(ctx, "Uploading file", "host", d.Host, "from", src, "to", dst)
	id, err := w.client.RcloneCopyFile(ctx, d.Host, dst, src)
	if err != nil {
		return err
	}
	p := w.getManifestProgress(d, progress)
	if p != nil {
		p.AgentJobID = id
		w.onRunProgress(ctx, p)
	}
	return w.waitJob(ctx, id, d, progress)
}

func (w *worker) getManifestProgress(d snapshotDir, progress []*RunProgress) *RunProgress {
	for _, p := range progress {
		if p.FileName == manifestFile && p.Unit == d.Unit && p.TableName == d.Table && p.Host == d.Host {
			return p
		}
	}
	return nil
}

func (w *worker) uploadDir(ctx context.Context, dst, src string, d snapshotDir, progress []*RunProgress) error {
	w.logger.Debug(ctx, "Uploading dir", "host", d.Host, "from", src, "to", dst)
	id, err := w.client.RcloneCopyDir(ctx, d.Host, dst, src, manifestFile)
	if err != nil {
		return err
	}

	for _, p := range w.getSnapshotDirProgress(d, progress) {
		p.AgentJobID = id
		w.onRunProgress(ctx, p)
	}
	return w.waitJob(ctx, id, d, progress)
}

func (w *worker) getSnapshotDirProgress(d snapshotDir, progress []*RunProgress) []*RunProgress {
	var out []*RunProgress
	for _, p := range progress {
		if p.Unit == d.Unit && p.TableName == d.Table && p.Host == d.Host {
			out = append(out, p)
		}
	}
	return out
}

func (w *worker) waitJob(ctx context.Context, id uuid.UUID, d snapshotDir, progress []*RunProgress) error {
	t := time.NewTicker(w.config.PollInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			err := w.client.RcloneJobStop(context.Background(), d.Host, id)
			if err != nil {
				w.logger.Error(ctx, "Failed to stop rclone job",
					"error", err,
					"host", d.Host,
					"unit", d.Unit,
					"jobid", id,
					"table", d.Table,
				)
			}
			w.updateProgress(ctx, id, d, progress)
			return ctx.Err()
		case <-t.C:
			status, err := w.getJobStatus(ctx, id, d)
			switch status {
			case jobError:
				return err
			case jobNotFound:
				return errors.Errorf("job not found (%s)", id)
			case jobSuccess:
				w.updateProgress(ctx, id, d, progress)
				return nil
			case jobRunning:
				w.updateProgress(ctx, id, d, progress)
			}
		}
	}
}

func (w *worker) getJobStatus(ctx context.Context, jobID uuid.UUID, d snapshotDir) (jobStatus, error) {
	s, err := w.client.RcloneJobStatus(ctx, d.Host, jobID)
	if err != nil {
		w.logger.Error(ctx, "Failed to fetch job status",
			"error", err,
			"host", d.Host,
			"unit", d.Unit,
			"jobid", jobID.String(),
			"table", d.Table,
		)
		if strings.Contains(err.Error(), "job not found") {
			// If job is no longer available fail.
			return jobNotFound, nil
		}
		return jobError, err
	}
	if s.Finished {
		if s.Success {
			return jobSuccess, nil
		}
		return jobError, errors.New(s.Error)
	}
	return jobRunning, nil
}

func (w *worker) updateProgress(ctx context.Context, jobID uuid.UUID, d snapshotDir, progress []*RunProgress) {
	transferred, err := w.client.RcloneTransferred(ctx, d.Host, jobID.String())
	if err != nil {
		w.logger.Error(ctx, "Failed to get transferred files",
			"error", err,
			"host", d.Host,
			"jobid", jobID,
		)
		return
	}
	stats, err := w.client.RcloneStats(ctx, d.Host, jobID.String())
	if err != nil {
		w.logger.Error(ctx, "Failed to get transfer stats",
			"error", err,
			"host", d.Host,
			"jobid", jobID,
		)
		return
	}

	for _, p := range w.getSnapshotDirProgress(d, progress) {
		if p.AgentJobID != jobID || p.Size == p.Uploaded {
			continue
		}
		trs := scyllaclient.TransferredByFilename(p.FileName, transferred)
		switch len(trs) {
		case 0: // Nothing in transferred so inspect transfers in progress.
			for _, tr := range stats.Transferring {
				if tr.Name == p.FileName {
					p.Uploaded = tr.Bytes
					w.onRunProgress(ctx, p)
					break
				}
			}
		case 1: // Only one transfer or one check.
			w.setProgressDates(ctx, p, d, jobID, trs[0].StartedAt, trs[0].CompletedAt)
			if trs[0].Error != "" {
				p.Error = trs[0].Error
				p.Failed = trs[0].Bytes
			}
			if trs[0].Checked {
				// File is already uploaded we just checked.
				p.Skipped = trs[0].Size
			} else {
				p.Uploaded = trs[0].Bytes
			}
			w.onRunProgress(ctx, p)
		case 2: // File is found and updated on remote (check plus transfer).
			// Order Check > Transfer is expected.
			// Taking start time from the check.
			w.setProgressDates(ctx, p, d, jobID, trs[0].StartedAt, trs[1].CompletedAt)
			if trs[0].Error != "" {
				p.Error = trs[0].Error
			}
			if trs[1].Error != "" {
				p.Error = fmt.Sprintf("%s %s", p.Error, trs[1].Error)
			}
			if p.Error != "" {
				p.Failed = trs[1].Bytes
			}
			p.Uploaded = trs[1].Bytes
			w.onRunProgress(ctx, p)
		}
	}
}

func (w *worker) onRunProgress(ctx context.Context, p *RunProgress) {
	if w.OnRunProgress != nil {
		w.OnRunProgress(ctx, p)
	}
}

func (w *worker) setProgressDates(ctx context.Context, p *RunProgress, d snapshotDir, jobID uuid.UUID, start, end string) {
	startedAt, err := timeutc.Parse(time.RFC3339, start)
	if err != nil {
		w.logger.Error(ctx, "Failed to parse start time",
			"error", err,
			"host", d.Host,
			"jobid", jobID,
			"value", start,
		)
	}
	if !startedAt.IsZero() {
		p.StartedAt = &startedAt
	}
	completedAt, err := timeutc.Parse(time.RFC3339, end)
	if err != nil {
		w.logger.Error(ctx, "Failed to parse complete time",
			"error", err,
			"host", d.Host,
			"jobid", jobID,
			"value", end,
		)
	}
	if !completedAt.IsZero() {
		p.CompletedAt = &completedAt
	}
}

func (w *worker) remoteMetaDir(h hostInfo, d snapshotDir) string {
	return path.Join(
		"backup",
		"cluster",
		w.clusterID.String(),
		"node",
		h.ID,
		"keyspace",
		d.Keyspace,
		"task",
		w.taskID.String(),
		"run",
		w.runID.String(),
		"table",
		d.Table,
		d.Version,
	)
}

func (w *worker) remoteSSTableDir(h hostInfo, d snapshotDir) string {
	return path.Join(
		remoteSstDir(w.clusterID.String(), h.ID, d.Keyspace),
		d.Table,
		d.Version,
	)
}

const dataDir = "/var/lib/scylla/data"

func keyspaceDir(keyspace string) string {
	return path.Join(dataDir, keyspace)
}

func remoteSstDir(clusterID, nodeID, keyspace string) string {
	return path.Join(
		"backup",
		"cluster",
		clusterID,
		"node",
		nodeID,
		"keyspace",
		keyspace,
		"sst",
	)
}

func snapshotTag(id uuid.UUID) string {
	return "sm_" + id.String()
}

func claimTag(tag string) bool {
	return strings.HasPrefix(tag, "sm_")
}
