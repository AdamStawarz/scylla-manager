// Copyright (C) 2017 ScyllaDB

package healthcheck

import (
	"crypto/tls"
	"time"
)

// Health check defaults.
var (
	DefaultCQLPort        = "9042"
	DefaultAlternatorPort = "8000"
	DefaultTLSConfig      = &tls.Config{
		InsecureSkipVerify: true,
	}
)

// Config specifies the healthcheck service configuration.
type Config struct {
	// Timeout specifies CQL ping timeout.
	Timeout time.Duration `yaml:"timeout"`
	// SSLTimeout specifies encrypted CQL ping timeout.
	SSLTimeout time.Duration `yaml:"ssl_timeout"`
	// DynamicTimeout specifies dynamic timeout configuration.
	DynamicTimeout DynamicTimeoutConfig `yaml:"dynamic_timeout"`
	// NodeInfoTTL specifies how long node info should be cached.
	NodeInfoTTL time.Duration `yaml:"node_info_ttl"`
}

// DynamicTimeoutConfig specifies healthcheck dynamic timeouts.
// Dynamic timeout takes recent probes RTTs, calculates mean (m) and
// standard deviation (stddev) and returns timeout of next probe
// equal to m + max(stddev_multiplier * stddev, 2ms).
type DynamicTimeoutConfig struct {
	// Enabled controls whether dynamic timeout is enabled or not.
	Enabled bool `yaml:"enabled"`
	// Probes specifies how many recent probes are kept in memory and are
	// part of calculations.
	Probes int `yaml:"probes"`
	// MaxTimeout specifies maximum value of calculated timeout.
	// Zero MaxTimeout means no limit on learned dynamic timeout.
	MaxTimeout time.Duration `yaml:"max_timeout"`
	// StdDeviationMultiplier controls how many standard deviations should be added to the next timeout.
	// For stable connections it's recommended to set this value high.
	StdDevMultiplier int `yaml:"stddev_multiplier"`
}

// DefaultConfig returns a Config initialized with default values.
func DefaultConfig() Config {
	return Config{
		Timeout:    250 * time.Millisecond,
		SSLTimeout: 750 * time.Millisecond,
		DynamicTimeout: DynamicTimeoutConfig{
			Enabled:          true,
			Probes:           200,
			StdDevMultiplier: 4,
			MaxTimeout:       30 * time.Second,
		},
		NodeInfoTTL: 5 * time.Minute,
	}
}
