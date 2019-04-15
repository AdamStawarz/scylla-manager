// Copyright (C) 2017 ScyllaDB

package main

import (
	"time"

	"github.com/scylladb/mermaid/internal/duration"
	"github.com/scylladb/mermaid/internal/timeutc"
	"github.com/scylladb/mermaid/service/scheduler"
	"github.com/scylladb/mermaid/uuid"
)

var emptyProperties = []byte{'{', '}'}

func makeAutoHealthCheckTask(clusterID uuid.UUID) *scheduler.Task {
	return &scheduler.Task{
		ClusterID: clusterID,
		Type:      scheduler.HealthCheckTask,
		Enabled:   true,
		Sched: scheduler.Schedule{
			Interval:   duration.Duration(15 * time.Second),
			StartDate:  timeutc.Now().Add(30 * time.Second),
			NumRetries: 0,
		},
		Properties: emptyProperties,
	}
}

func makeAutoHealthCheckRESTTask(clusterID uuid.UUID) *scheduler.Task {
	return &scheduler.Task{
		ClusterID: clusterID,
		Type:      scheduler.HealthCheckRESTTask,
		Enabled:   true,
		Sched: scheduler.Schedule{
			Interval:   duration.Duration(1 * time.Hour),
			StartDate:  timeutc.Now().Add(1 * time.Minute),
			NumRetries: 0,
		},
		Properties: emptyProperties,
	}
}

func makeAutoRepairTask(clusterID uuid.UUID) *scheduler.Task {
	return &scheduler.Task{
		ClusterID: clusterID,
		Type:      scheduler.RepairTask,
		Enabled:   true,
		Sched: scheduler.Schedule{
			Interval:   duration.Duration(7 * 24 * time.Hour),
			StartDate:  timeutc.TodayMidnight(),
			NumRetries: 3,
		},
		Properties: emptyProperties,
	}
}
