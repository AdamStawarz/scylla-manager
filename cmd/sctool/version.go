// Copyright (C) 2017 ScyllaDB

package main

import (
	"context"
	"fmt"

	"github.com/scylladb/mermaid"
	"github.com/spf13/cobra"
)

var versionCmd = withoutArgs(&cobra.Command{
	Use:   "version",
	Short: "Show version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), fmt.Sprintf("Client version: %v", mermaid.Version()))

		sv, err := client.Version(context.Background())
		if err != nil {
			return printableError{err}
		}

		fmt.Fprintln(cmd.OutOrStdout(), fmt.Sprintf("Server version: %v", sv.Version))

		return nil
	},
})

func init() {
	subcommand(versionCmd, rootCmd)
}
