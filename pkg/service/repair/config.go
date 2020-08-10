// Copyright (C) 2017 ScyllaDB

package repair

import (
	"time"

	"github.com/pkg/errors"
	"github.com/scylladb/mermaid/pkg/service"
	"go.uber.org/multierr"
)

// Type represents type of the repair algorithm.
type Type string

const (
	// TypeAuto auto detects repair algo.
	TypeAuto = "auto"
	// TypeRowLevel row level repair.
	TypeRowLevel = "row_level"
	// TypeLegacy legacy repair type.
	TypeLegacy = "legacy"
)

// Config specifies the repair service configuration.
type Config struct {
	PollInterval                    time.Duration `yaml:"poll_interval"`
	AgeMax                          time.Duration `yaml:"age_max"`
	GracefulShutdownTimeout         time.Duration `yaml:"graceful_shutdown_timeout"`
	ForceRepairType                 string        `yaml:"force_repair_type"`
	Murmur3PartitionerIgnoreMSBBits int           `yaml:"murmur3_partitioner_ignore_msb_bits"`
}

// DefaultConfig returns a Config initialized with default values.
func DefaultConfig() Config {
	return Config{
		PollInterval:                    50 * time.Millisecond,
		GracefulShutdownTimeout:         30 * time.Second,
		ForceRepairType:                 TypeAuto,
		Murmur3PartitionerIgnoreMSBBits: 12,
	}
}

// Validate checks if all the fields are properly set.
func (c *Config) Validate() error {
	if c == nil {
		return service.ErrNilPtr
	}

	var err error
	if c.PollInterval <= 0 {
		err = multierr.Append(err, errors.New("invalid poll_interval, must be > 0"))
	}
	if c.AgeMax < 0 {
		err = multierr.Append(err, errors.New("invalid age_max, must be >= 0"))
	}
	if c.GracefulShutdownTimeout <= 0 {
		err = multierr.Append(err, errors.New("invalid graceful_shutdown_timeout, must be > 0"))
	}
	switch c.ForceRepairType {
	case TypeAuto, TypeRowLevel, TypeLegacy:
	default:
		err = multierr.Append(err, errors.Errorf("invalid force_repair_type value %s", c.ForceRepairType))
	}
	if c.Murmur3PartitionerIgnoreMSBBits < 0 {
		err = multierr.Append(err, errors.New("invalid murmur3_partitioner_ignore_msb_bits, must be >= 0"))
	}

	return err
}
