// Copyright (C) 2017 ScyllaDB

package command

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/mermaid/mermaidtest"
	"github.com/scylladb/mermaid/uuid"
)

func TestServerReadConfig(t *testing.T) {
	t.Parallel()

	s := ServerCommand{}
	c, err := s.readConfig("testdata/scylla-mgmt.yml")
	if err != nil {
		t.Fatal(err)
	}

	var u uuid.UUID
	u.UnmarshalText([]byte("a3f1b32b-ed5b-438d-81a7-c82eb7bde800"))

	e := &serverConfig{
		HTTP:        "127.0.0.1:80",
		HTTPS:       "127.0.0.1:443",
		TLSCertFile: "tls.cert",
		TLSKeyFile:  "tls.key",
		Database: dbConfig{
			Hosts:                         []string{"172.16.1.10", "172.16.1.20"},
			User:                          "user",
			Keyspace:                      "scylla_management",
			KeyspaceTplFile:               "/etc/scylla-mgmt/create_keyspace.cql.tpl",
			Password:                      "password",
			MigrateDir:                    "/etc/scylla-mgmt/cql",
			MigrateTimeout:                30 * time.Second,
			MigrateMaxWaitSchemaAgreement: 5 * time.Minute,
			Consistency:                   "ONE",
		},
		Clusters: []*clusterConfig{
			{
				UUID:                            u,
				Hosts:                           []string{"172.16.1.10", "172.16.1.20"},
				ShardCount:                      16,
				Murmur3PartitionerIgnoreMsbBits: 12,
			},
		},
	}

	if diff := cmp.Diff(c, e, mermaidtest.UUIDComparer()); diff != "" {
		t.Fatal(diff)
	}
}

func TestReadKeyspaceTplFile(t *testing.T) {
	s := ServerCommand{}
	stmt, err := s.readKeyspaceTplFile(&serverConfig{
		Database: dbConfig{
			Keyspace:        "keyspace",
			KeyspaceTplFile: "../dist/etc/create_keyspace.cql.tpl",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stmt, "CREATE KEYSPACE IF NOT EXISTS keyspace WITH replication") {
		t.Fatal(stmt)
	}
}
