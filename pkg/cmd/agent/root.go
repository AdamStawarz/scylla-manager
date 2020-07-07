// Copyright (C) 2017 ScyllaDB

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	"github.com/scylladb/mermaid/pkg"
	"github.com/spf13/cobra"
)

var rootArgs = struct {
	configFiles []string
	version     bool
}{}

var rootCmd = &cobra.Command{
	Use:           "scylla-manager",
	Short:         "Scylla Manager server",
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,

	RunE: func(cmd *cobra.Command, args []string) (runError error) {
		// Print version and return
		if rootArgs.version {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", pkg.Version())
			return nil
		}

		config, err := parseAndValidateConfigFile(rootArgs.configFiles)
		if err != nil {
			return err
		}

		// Get a base context with tracing id
		ctx := log.WithNewTraceID(context.Background())

		logger, err := logger(config)
		if err != nil {
			return errors.Wrapf(err, "logger")
		}
		defer func() {
			if runError != nil {
				logger.Error(ctx, "Bye", "error", runError)
			} else {
				logger.Info(ctx, "Bye")
			}
			logger.Sync() // nolint
		}()

		// Start server
		server := newServer(config, logger)
		if err := server.init(ctx); err != nil {
			return errors.Wrapf(err, "server init")
		}
		server.makeHTTPServers()
		server.startServers(ctx)
		defer server.shutdownServers(ctx, 30*time.Second)

		// Wait signal
		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
		select {
		case err := <-server.errCh:
			if err != nil {
				return err
			}
		case sig := <-signalCh:
			logger.Info(ctx, "Received signal", "signal", sig)
		}

		return nil
	},
}

func logger(c config) (log.Logger, error) {
	if c.Logger.Development {
		return log.NewDevelopmentWithLevel(c.Logger.Level), nil
	}

	return log.NewProduction(log.Config{
		Mode:  c.Logger.Mode,
		Level: c.Logger.Level,
	})
}

func init() {
	f := rootCmd.Flags()
	f.StringSliceVarP(&rootArgs.configFiles, "config-file", "c",
		[]string{"/etc/scylla-manager-agent/scylla-manager-agent.yaml"},
		"repeatable argument to supply one or more configuration file `paths`")
	f.BoolVar(&rootArgs.version, "version", false, "print product version and exit")
}
