// Copyright (C) 2017 ScyllaDB

package mermaidclient

import (
	"net"

	"github.com/scylladb/mermaid/mermaidclient/internal/models"
)

// RepairProgressRow contains shard progress info.
type RepairProgressRow struct {
	Host     net.IP
	Shard    int
	Progress int
	Error    int
}

// RepairUnit is repair.Unit representation.
type RepairUnit = models.RepairUnit
