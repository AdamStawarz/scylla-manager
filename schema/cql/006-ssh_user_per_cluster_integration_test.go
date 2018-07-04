// Copyright (C) 2017 ScyllaDB

// +build all integration

package cql

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/gocql/gocql"
	"github.com/scylladb/gocqlx/migrate"
	"github.com/scylladb/gocqlx/qb"
	log "github.com/scylladb/golog"
	. "github.com/scylladb/mermaid/mermaidtest"
	"github.com/scylladb/mermaid/uuid"
)

func TestCopySSHInfoToClusterAfter006Integration(t *testing.T) {
	session := CreateSessionWithoutMigration(t)

	Print("Given: config files")
	dir, err := ioutil.TempDir("", "mermaid.schema.cql.TestCopySSHInfoToClusterAfter006Integration")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Remove(dir)
	}()

	pemFile := filepath.Join(dir, "scylla_manager.pem")
	if err := ioutil.WriteFile(pemFile, []byte("pem"), 0400); err != nil {
		t.Fatal(err)
	}

	oldConfig := `# Communication with Scylla nodes.
ssh:
  # SSH user name, user must exist on Scylla nodes.
  user: user
  # PEM encoded SSH private key for user.
  identity_file: ` + pemFile

	oldConfigFile := filepath.Join(dir, "scylla-manager.yaml.rpmsave")
	if err := ioutil.WriteFile(oldConfigFile, []byte(oldConfig), 0400); err != nil {
		t.Fatal(err)
	}

	Print("And: clusters")
	h := copySSHInfoToCluster006{
		oldConfigFile: oldConfigFile,
		dir:           dir,
	}
	registerMigrationCallback("006-ssh_user_per_cluster.cql", migrate.AfterMigration, func(ctx context.Context, session *gocql.Session, logger log.Logger) error {
		const insertClusterCql = `INSERT INTO cluster (id) VALUES (uuid())`
		ExecStmt(t, session, insertClusterCql)
		return h.After(ctx, session, logger)
	})

	Print("When: migrate")
	if err := migrate.Migrate(context.Background(), session, "."); err != nil {
		t.Fatal("migrate:", err)
	}

	Print("Then: SSH user is added")
	stmt, _ := qb.Select("cluster").Columns("id", "ssh_user").ToCql()
	q := session.Query(stmt)
	var (
		id      uuid.UUID
		sshUser string
	)
	if err := q.Scan(&id, &sshUser); err != nil {
		t.Fatal(err)
	}
	q.Release()
	if sshUser != "user" {
		t.Fatal(sshUser)
	}

	Print("And: file exists")
	if _, err := os.Stat(filepath.Join(dir, id.String())); err != nil {
		t.Fatal(err)
	}
}
