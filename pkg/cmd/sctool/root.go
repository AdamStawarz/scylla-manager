// Copyright (C) 2017 ScyllaDB

package main

import (
	"crypto/tls"
	"net/http"
	"os"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-manager/pkg"
	"github.com/scylladb/scylla-manager/pkg/config"
	"github.com/scylladb/scylla-manager/pkg/managerclient"
	"github.com/spf13/cobra"
)

var (
	defaultURL = "http://127.0.0.1:5080/api/v1"

	cfgAPIURL      string
	cfgAPICertFile string
	cfgAPIKeyFile  string
	cfgCluster     string

	client managerclient.Client
)

var rootCmd = &cobra.Command{
	Use:   "sctool",
	Short: "Scylla Manager " + pkg.Version(),
	Long: "Scylla Manager " + pkg.Version() + `

Documentation is available online at ` + docsBaseURL,

	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.IsAdditionalHelpTopicCommand() || cmd.Hidden {
			return nil
		}

		// Init client
		tlsConfig := managerclient.DefaultTLSConfig()

		// Load TLS certificate if provided
		if cfgAPICertFile != "" && cfgAPIKeyFile == "" {
			return errors.New("missing flag \"api-key-file\"")
		}
		if cfgAPIKeyFile != "" && cfgAPICertFile == "" {
			return errors.New("missing flag \"api-cert-file\"")
		}
		if cfgAPICertFile != "" {
			cert, err := tls.LoadX509KeyPair(cfgAPICertFile, cfgAPIKeyFile)
			if err != nil {
				return errors.Wrap(err, "load client certificate")
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		c, err := managerclient.NewClient(cfgAPIURL, &http.Transport{TLSClientConfig: tlsConfig})
		if err != nil {
			return err
		}
		client = c

		// RequireFlags cluster
		if needsCluster(cmd) {
			if os.Getenv("SCYLLA_MANAGER_CLUSTER") == "" {
				if err := cmd.Root().MarkFlagRequired("cluster"); err != nil {
					return err
				}
			}
		}

		return nil
	},
}

func needsCluster(cmd *cobra.Command) bool {
	switch cmd {
	case clusterAddCmd, clusterListCmd, statusCmd, taskListCmd, versionCmd:
		return false
	}
	return true
}

func init() {
	f := rootCmd.PersistentFlags()

	apiURL := os.Getenv("SCYLLA_MANAGER_API_URL")
	// Attempt to read local Scylla Manager configuration only if default URL
	// is not set by the environment variable.
	if apiURL == "" {
		cfg, err := config.ParseServerConfigFiles([]string{"/etc/scylla-manager/scylla-manager.yaml"})
		if err == nil {
			apiURL = urlFromConfig(cfg)
		}
	}
	if apiURL == "" {
		apiURL = defaultURL
	}
	f.StringVar(&cfgAPIURL, "api-url", apiURL, "`URL` of Scylla Manager server")
	f.StringVar(&cfgAPICertFile, "api-cert-file", os.Getenv("SCYLLA_MANAGER_API_CERT_FILE"), "`path` to HTTPS client certificate to access Scylla Manager server")
	f.StringVar(&cfgAPIKeyFile, "api-key-file", os.Getenv("SCYLLA_MANAGER_API_KEY_FILE"), "`path` to HTTPS client key to access Scylla Manager server")

	f.StringVarP(&cfgCluster, "cluster", "c", os.Getenv("SCYLLA_MANAGER_CLUSTER"), "Specifies the target cluster `name` or ID")
}
