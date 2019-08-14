// Copyright (C) 2017 ScyllaDB

package restapi

import (
	"context"
	"encoding/json"

	"github.com/scylladb/mermaid/service/backup"
	"github.com/scylladb/mermaid/service/cluster"
	"github.com/scylladb/mermaid/service/healthcheck"
	"github.com/scylladb/mermaid/service/repair"
	"github.com/scylladb/mermaid/service/scheduler"
	"github.com/scylladb/mermaid/uuid"
)

// Services contains REST API services.
type Services struct {
	Backup      BackupService
	Cluster     ClusterService
	HealthCheck HealthCheckService
	Repair      RepairService
	Scheduler   SchedService
}

// BackupService service interface for the REST API handlers.
type BackupService interface {
	GetTarget(ctx context.Context, clusterID uuid.UUID, properties json.RawMessage, force bool) (backup.Target, error)
}

// ClusterService service interface for the REST API handlers.
type ClusterService interface {
	ListClusters(ctx context.Context, f *cluster.Filter) ([]*cluster.Cluster, error)
	GetCluster(ctx context.Context, idOrName string) (*cluster.Cluster, error)
	PutCluster(ctx context.Context, c *cluster.Cluster) error
	DeleteCluster(ctx context.Context, id uuid.UUID) error
	ListNodes(ctx context.Context, id uuid.UUID) ([]cluster.Node, error)
}

// HealthCheckService service interface for the REST API handlers.
type HealthCheckService interface {
	GetStatus(ctx context.Context, clusterID uuid.UUID) ([]healthcheck.Status, error)
}

// RepairService service interface for the REST API handlers.
type RepairService interface {
	GetRun(ctx context.Context, clusterID, taskID, runID uuid.UUID) (*repair.Run, error)
	GetProgress(ctx context.Context, clusterID, taskID, runID uuid.UUID) (repair.Progress, error)
	GetTarget(ctx context.Context, clusterID uuid.UUID, properties json.RawMessage, force bool) (repair.Target, error)
}

// SchedService service interface for the REST API handlers.
type SchedService interface {
	GetTask(ctx context.Context, clusterID uuid.UUID, tp scheduler.TaskType, idOrName string) (*scheduler.Task, error)
	PutTask(ctx context.Context, t *scheduler.Task) error
	PutTaskOnce(ctx context.Context, t *scheduler.Task) error
	DeleteTask(ctx context.Context, t *scheduler.Task) error
	ListTasks(ctx context.Context, clusterID uuid.UUID, tp scheduler.TaskType) ([]*scheduler.Task, error)
	StartTask(ctx context.Context, t *scheduler.Task, opts ...scheduler.Opt) error
	StopTask(ctx context.Context, t *scheduler.Task) error
	GetRun(ctx context.Context, t *scheduler.Task, runID uuid.UUID) (*scheduler.Run, error)
	GetLastRun(ctx context.Context, t *scheduler.Task, n int) ([]*scheduler.Run, error)
}
