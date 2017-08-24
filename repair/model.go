package repair

import (
	"errors"
	"time"

	"github.com/scylladb/mermaid"
)

// ConfigType specifies a type of configuration. Configuration object is built
// by merging configurations of different types. If configuration option is not
// found for UnitConfig then it falls back to KeyspaceConfig, ClusterConfig and
// TenantConfig.
type ConfigType string

// ConfigType enumeration.
const (
	UnitConfig     ConfigType = "unit"
	KeyspaceConfig            = "keyspace"
	ClusterConfig             = "cluster"
	tenantConfig              = "tenant"
)

// Config specifies how a Unit is repaired.
type Config struct {
	// Enabled specifies if repair should take place at all.
	Enabled *bool
	// SegmentsPerShard specifies in how many steps a shard will be repaired,
	// increasing this value decreases singe node repair command time and
	// increases number of node repair commands.
	SegmentsPerShard *int
	// RetryLimit specifies how many times a failed segment should be retried
	// before reporting an error.
	RetryLimit *int
	// RetryBackoffSeconds specifies minimal time in seconds to wait before
	// retrying a failed segment.
	RetryBackoffSeconds *int
	// ParallelNodeLimit specifies how many nodes can be repaired in parallel.
	// Set to 0 for unlimited.
	ParallelNodeLimit *int
	// ParallelShardPercent specifies how many shards on a node can be repaired
	// in parallel as a percent of total shards. ParallelShardPercent takes
	// values from 0 to 1.
	ParallelShardPercent *float32
}

// Validate checks if all the fields are properly set.
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("nil config")
	}

	var (
		i int
		f float32
	)

	if c.SegmentsPerShard != nil {
		i = *c.SegmentsPerShard
		if i < 1 {
			return errors.New("invalid value for SegmentsPerShard, valid values are greater or equal 1")
		}
	}
	if c.RetryLimit != nil {
		i = *c.RetryLimit
		if i < 0 {
			return errors.New("invalid value for RetryLimit, valid values are greater or equal 0")
		}
	}
	if c.RetryBackoffSeconds != nil {
		i = *c.RetryBackoffSeconds
		if i < 0 {
			return errors.New("invalid value for RetryBackoffSeconds, valid values are greater or equal 0")
		}
	}
	if c.ParallelNodeLimit != nil {
		i = *c.ParallelNodeLimit
		if i < -1 {
			return errors.New("invalid value for ParallelNodeLimit, valid values are greater or equal -1")
		}
	}
	if c.ParallelShardPercent != nil {
		f = *c.ParallelShardPercent
		if f < 0 || f > 1 {
			return errors.New("invalid value for ParallelShardPercent, valid values are between 0 and 1")
		}
	}

	return nil
}

// Unit is a set of tables in a keyspace that are repaired together.
type Unit struct {
	ID        mermaid.UUID
	ClusterID mermaid.UUID
	Keyspace  string
	Tables    []string
}

// Status specifies the status of a Run.
type Status string

// Status enumeration.
const (
	StatusRunning Status = "running"
	StatusSuccess Status = "success"
	StatusError   Status = "error"
	StatusPaused  Status = "paused"
	StatusAborted Status = "aborted"
)

// Run tracks repair progress, shares ID with sched.Run that initiated it.
type Run struct {
	ID           mermaid.UUID
	UnitID       mermaid.UUID
	ClusterID    mermaid.UUID
	Status       Status
	Cause        string
	RestartCount int
	StartTime    time.Time
	EndTime      time.Time
	PauseTime    time.Time
}
