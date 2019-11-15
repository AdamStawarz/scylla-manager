// Copyright (C) 2017 ScyllaDB

package main

import (
	"context"
	"io/ioutil"
	"net"
	"os"

	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/mermaid/scyllaclient"
	"go.uber.org/multierr"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v2"
)

const (
	defaultHTTPSPort = "10001"
	noCPU            = -1
)

type logConfig struct {
	Mode        log.Mode      `yaml:"mode"`
	Level       zapcore.Level `yaml:"level"`
	Development bool          `yaml:"development"`
}

// scyllaConfig contains selected elements of Scylla configuration.
type scyllaConfig struct {
	APIAddress string `yaml:"api_address"`
	APIPort    string `yaml:"api_port"`

	ListenAddress     string
	PrometheusAddress string
	PrometheusPort    string
}

// config specifies the agent and scylla configuration.
type config struct {
	AuthToken   string       `yaml:"auth_token"`
	HTTPS       string       `yaml:"https"`
	TLSCertFile string       `yaml:"tls_cert_file"`
	TLSKeyFile  string       `yaml:"tls_key_file"`
	Debug       string       `yaml:"debug"`
	CPU         int          `yaml:"cpu"`
	Logger      logConfig    `yaml:"logger"`
	Scylla      scyllaConfig `yaml:"scylla"`
}

func defaultConfig() config {
	return config{
		TLSCertFile: "/var/lib/scylla-manager/scylla_manager.crt",
		TLSKeyFile:  "/var/lib/scylla-manager/scylla_manager.key",
		CPU:         noCPU,
		Scylla: scyllaConfig{
			APIAddress: "0.0.0.0",
			APIPort:    "10000",
		},
		Logger: logConfig{
			Mode:        log.StderrMode,
			Level:       zapcore.InfoLevel,
			Development: false,
		},
	}
}

func parseConfigFile(file string) (config, error) {
	c := defaultConfig()

	b, err := ioutil.ReadFile(file)
	if err != nil {
		return c, errors.Wrapf(err, "read config file %s", file)
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return c, errors.Wrapf(err, "parse config file %s", file)
	}
	if err := c.validate(); err != nil {
		return c, errors.Wrapf(err, "invalid config file %s", file)
	}

	return c, nil
}

func (c *config) updateWithScyllaConfig(external scyllaConfig) {
	c.Scylla.ListenAddress = external.ListenAddress
	c.Scylla.PrometheusAddress = external.PrometheusAddress
	c.Scylla.PrometheusPort = external.PrometheusPort

	if c.HTTPS != "" {
		return
	}
	c.HTTPS = net.JoinHostPort(c.Scylla.ListenAddress, defaultHTTPSPort)
}

func (c config) validate() (errs error) {
	// Validate TLS config
	if c.TLSCertFile == "" {
		errs = multierr.Append(errs, errors.New("missing tls_cert_file"))
	} else if _, err := os.Stat(c.TLSCertFile); err != nil {
		errs = multierr.Append(errs, errors.Wrapf(err, "invalid tls_cert_file %s", c.TLSCertFile))
	}
	if c.TLSKeyFile == "" {
		errs = multierr.Append(errs, errors.New("missing tls_key_file"))
	} else if _, err := os.Stat(c.TLSKeyFile); err != nil {
		errs = multierr.Append(errs, errors.Wrapf(err, "invalid tls_key_file %s", c.TLSKeyFile))
	}
	// Validate Scylla config
	errs = multierr.Append(errs, errors.Wrap(c.Scylla.validate(), "scylla"))

	return
}

func (c scyllaConfig) validate() (errs error) {
	if c.APIAddress == "" {
		errs = multierr.Append(errs, errors.New("missing api_address"))
	}
	if c.APIPort == "" {
		errs = multierr.Append(errs, errors.New("missing api_port"))
	}
	return
}

// enrichConfigFromAPI fetches address info from the node and updates the
// configuration.
func (c *config) enrichConfigFromAPI(ctx context.Context, addr string) error {
	external, err := fetchScyllaConfig(ctx, addr)
	if err != nil {
		return err
	}

	c.updateWithScyllaConfig(external)
	return nil
}

func fetchScyllaConfig(ctx context.Context, addr string) (c scyllaConfig, err error) {
	client := scyllaclient.NewConfigClient(addr)

	if c.ListenAddress, err = client.ListenAddress(ctx); err != nil {
		return
	}
	if c.PrometheusAddress, err = client.PrometheusAddress(ctx); err != nil {
		return
	}
	if c.PrometheusPort, err = client.PrometheusPort(ctx); err != nil {
		return
	}
	return
}
