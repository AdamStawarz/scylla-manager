// Copyright (C) 2017 ScyllaDB

package migrate

import (
	"context"

	"github.com/gocql/gocql"
	"github.com/scylladb/go-log"
	"github.com/scylladb/gocqlx"
	"github.com/scylladb/gocqlx/migrate"
	"github.com/scylladb/gocqlx/qb"
	"github.com/scylladb/mermaid/pkg/util/timeutc"
	"github.com/scylladb/mermaid/pkg/util/uuid"
)

func init() {
	registerCallback("008-drop_repair_unit.cql", migrate.AfterMigration, createDefaultRepairTaskForClusterAfter008)
}

func createDefaultRepairTaskForClusterAfter008(ctx context.Context, session *gocql.Session, logger log.Logger) error {
	stmt, names := qb.Select("cluster").Columns("id").ToCql()
	q := gocqlx.Query(session.Query(stmt), names)
	var ids []uuid.UUID
	if err := q.SelectRelease(&ids); err != nil {
		return err
	}

	const insertTaskCql = `INSERT INTO scheduler_task(cluster_id, type, id, enabled, sched, properties) 
VALUES (?, 'repair', uuid(), true, {start_date: ?, interval_days: ?, num_retries: ?}, ?)`
	iq := session.Query(insertTaskCql)
	defer iq.Release()

	for _, id := range ids {
		iq.Bind(id, timeutc.TodayMidnight(), 7, 3, []byte{'{', '}'})
		if err := iq.Exec(); err != nil {
			logger.Error(ctx, "Failed to add repair task", "cluster_id", id, "error", err.Error())
		}
	}

	return nil
}
