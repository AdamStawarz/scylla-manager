// Copyright (C) 2017 ScyllaDB

package schema

import "github.com/scylladb/gocqlx/qb"

// Table represents a table in a database and it's metadata.
type Table struct {
	Name    string
	Columns []string
	PartKey []string
	SortKey []string

	primaryKey []qb.Cmp
	delete     cql
	get        cql
	insert     cql
	sel        cql
}

type cql struct {
	stmt  string
	names []string
}

// Get returns select by primary key statement.
func (t *Table) Get(columns ...string) (stmt string, names []string) {
	if len(columns) == 0 {
		return t.get.stmt, t.get.names
	}

	return qb.Select(t.Name).
		Columns(columns...).
		Where(t.primaryKey...).
		ToCql()
}

// Select returns select by partition key statement.
func (t *Table) Select(columns ...string) (stmt string, names []string) {
	if len(columns) == 0 {
		return t.sel.stmt, t.sel.names
	}

	return qb.Select(t.Name).
		Columns(columns...).
		Where(t.primaryKey[0:len(t.PartKey)]...).
		ToCql()
}

// SelectBuilder returns a builder initialised to select by partition key
// statement.
func (t *Table) SelectBuilder(columns ...string) *qb.SelectBuilder {
	return qb.Select(t.Name).
		Columns(columns...).
		Where(t.primaryKey[0:len(t.PartKey)]...)
}

// Insert returns insert all columns statement.
func (t *Table) Insert() (stmt string, names []string) {
	return t.insert.stmt, t.insert.names
}

// Update returns update columns statement.
func (t *Table) Update(columns ...string) (stmt string, names []string) {
	return qb.Update(t.Name).
		Set(columns...).
		Where(t.primaryKey...).
		ToCql()
}

// Delete returns delete by primary key statement.
func (t *Table) Delete() (stmt string, names []string) {
	return t.delete.stmt, t.delete.names
}

func (t Table) init() Table {
	// primary key comparator
	t.primaryKey = make([]qb.Cmp, len(t.PartKey)+len(t.SortKey))
	for i, c := range append(t.PartKey, t.SortKey...) {
		t.primaryKey[i] = qb.Eq(c)
	}

	// delete
	{
		t.delete.stmt, t.delete.names = qb.Delete(t.Name).Where(t.primaryKey...).ToCql()
	}

	// get
	{
		t.get.stmt, t.get.names = qb.Select(t.Name).Where(t.primaryKey...).ToCql()
	}

	// insert
	{
		t.insert.stmt, t.insert.names = qb.Insert(t.Name).Columns(t.Columns...).ToCql()
	}

	// select
	{
		t.sel.stmt, t.sel.names = qb.Select(t.Name).Where(t.primaryKey[0:len(t.PartKey)]...).ToCql()
	}

	return t
}

// Tables listing
var (
	Cluster = Table{
		Name:    "cluster",
		Columns: []string{"id", "name", "hosts", "ssh_user"},
		PartKey: []string{"id"},
	}.init()

	RepairRun = Table{
		Name:    "repair_run",
		Columns: []string{"cluster_id", "task_id", "id", "prev_id", "topology_hash", "units", "status", "cause", "start_time", "end_time"},
		PartKey: []string{"cluster_id", "task_id"},
		SortKey: []string{"id"},
	}.init()

	RepairRunProgress = Table{
		Name:    "repair_run_progress",
		Columns: []string{"cluster_id", "task_id", "run_id", "unit", "host", "shard", "segment_count", "segment_success", "segment_error", "segment_error_start_tokens", "last_start_token", "last_start_time", "last_command_id"},
		PartKey: []string{"cluster_id", "task_id", "run_id"},
		SortKey: []string{"unit", "host", "shard"},
	}.init()

	SchedTask = Table{
		Name:    "scheduler_task",
		Columns: []string{"cluster_id", "type", "id", "name", "tags", "enabled", "sched", "properties"},
		PartKey: []string{"cluster_id"},
		SortKey: []string{"type", "id"},
	}.init()

	SchedRun = Table{
		Name:    "scheduler_task_run",
		Columns: []string{"cluster_id", "type", "task_id", "id", "status", "cause", "owner", "start_time", "end_time"},
		PartKey: []string{"cluster_id", "type", "task_id"},
		SortKey: []string{"id"},
	}.init()
)
