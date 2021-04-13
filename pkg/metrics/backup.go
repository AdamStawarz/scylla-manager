// Copyright (C) 2017 ScyllaDB

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/scylladb/scylla-manager/pkg/util/uuid"
)

type BackupMetrics struct {
	snapshot           *prometheus.GaugeVec
	filesSizeBytes     *prometheus.GaugeVec
	filesUploadedBytes *prometheus.GaugeVec
	filesSkippedBytes  *prometheus.GaugeVec
	filesFailedBytes   *prometheus.GaugeVec
	purgeFiles         *prometheus.GaugeVec
	purgeDeletedFiles  *prometheus.GaugeVec
}

func NewBackupMetrics() BackupMetrics {
	g := gaugeVecCreator("backup")

	return BackupMetrics{
		snapshot: g("Indicates if snapshot was taken.",
			"snapshot", "cluster", "keyspace", "host"),
		filesSizeBytes: g("Total size of backup files in bytes.",
			"files_size_bytes", "cluster", "keyspace", "table", "host"),
		filesUploadedBytes: g("Number of bytes uploaded to backup location.",
			"files_uploaded_bytes", "cluster", "keyspace", "table", "host"),
		filesSkippedBytes: g("Number of deduplicated bytes already uploaded to backup location.",
			"files_skipped_bytes", "cluster", "keyspace", "table", "host"),
		filesFailedBytes: g("Number of bytes failed to upload to backup location.",
			"files_failed_bytes", "cluster", "keyspace", "table", "host"),
		purgeFiles: g("Number of files that need to be deleted due to retention policy.",
			"purge_files", "cluster", "host"),
		purgeDeletedFiles: g("Number of files that were deleted.",
			"purge_deleted_files", "cluster", "host"),
	}
}

func (m BackupMetrics) all() []prometheus.Collector {
	return []prometheus.Collector{
		m.snapshot,
		m.filesSizeBytes,
		m.filesUploadedBytes,
		m.filesSkippedBytes,
		m.filesFailedBytes,
		m.purgeFiles,
		m.purgeDeletedFiles,
	}
}

// MustRegister shall be called to make the metrics visible by prometheus client.
func (m BackupMetrics) MustRegister() BackupMetrics {
	prometheus.MustRegister(m.all()...)
	return m
}

// DeleteClusterMetrics removes all metrics labeled with the cluster.
func (m BackupMetrics) DeleteClusterMetrics(clusterID uuid.UUID) {
	for _, c := range m.all() {
		DeleteMatching(c.(CollectorDeleter), clusterMatcher(clusterID))
	}
}

// SetSnapshot updates "snapshot" metric.
func (m BackupMetrics) SetSnapshot(clusterID uuid.UUID, keyspace, host string, taken bool) {
	l := prometheus.Labels{
		"cluster":  clusterID.String(),
		"keyspace": keyspace,
		"host":     host,
	}
	v := 0.
	if taken {
		v = 1
	}
	m.snapshot.With(l).Set(v)
}

// SetFilesProgress updates "files_{uploaded,skipped,failed}_bytes" metrics.
func (m BackupMetrics) SetFilesProgress(clusterID uuid.UUID, keyspace, table, host string, size, uploaded, skipped, failed int64) {
	l := prometheus.Labels{
		"cluster":  clusterID.String(),
		"keyspace": keyspace,
		"table":    table,
		"host":     host,
	}
	m.filesSizeBytes.With(l).Set(float64(size))
	m.filesUploadedBytes.With(l).Set(float64(uploaded))
	m.filesSkippedBytes.With(l).Set(float64(skipped))
	m.filesFailedBytes.With(l).Set(float64(failed))
}

// SetPurgeFiles updates "purge_files" metric.
func (m BackupMetrics) SetPurgeFiles(clusterID uuid.UUID, host string, files int) {
	m.purgeFiles.WithLabelValues(clusterID.String(), host).Set(float64(files))
}

// IncPurgeDeletedFiles adds 1 to "purge_deleted_files" metric.
func (m BackupMetrics) IncPurgeDeletedFiles(clusterID uuid.UUID, host string) {
	m.purgeDeletedFiles.WithLabelValues(clusterID.String(), host).Add(1)
}
