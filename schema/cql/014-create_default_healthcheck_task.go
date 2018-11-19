// Copyright (C) 2017 ScyllaDB

package cql

import (
	"context"
	"time"

	"github.com/gocql/gocql"
	"github.com/scylladb/go-log"
	"github.com/scylladb/gocqlx"
	"github.com/scylladb/gocqlx/migrate"
	"github.com/scylladb/gocqlx/qb"
	"github.com/scylladb/mermaid/internal/timeutc"
	"github.com/scylladb/mermaid/uuid"
)

func init() {
	registerMigrationCallback("013-update_ttl.cql", migrate.AfterMigration, createDefaultHealthCheckTaskForClusterAfter013)
}

func createDefaultHealthCheckTaskForClusterAfter013(ctx context.Context, session *gocql.Session, logger log.Logger) error {
	stmt, names := qb.Select("cluster").Columns("id").ToCql()
	q := gocqlx.Query(session.Query(stmt).WithContext(ctx), names)
	var ids []uuid.UUID
	if err := q.SelectRelease(&ids); err != nil {
		return err
	}

	const insertTaskCql = `INSERT INTO scheduler_task(cluster_id, type, id, enabled, sched, properties) 
VALUES (?, 'healthcheck', uuid(), true, {start_date: ?, interval_seconds: ?, num_retries: ?}, ?)`
	iq := session.Query(insertTaskCql).WithContext(ctx)
	defer iq.Release()

	for _, id := range ids {
		iq.Bind(id, timeutc.Now().Add(30*time.Second), 15, 0, []byte{'{', '}'})
		if err := iq.Exec(); err != nil {
			logger.Error(ctx, "failed to add healthcheck task", "cluster_id", id, "error", err.Error())
		}
	}

	return nil
}
