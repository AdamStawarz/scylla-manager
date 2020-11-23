// Copyright (C) 2017 ScyllaDB

package main

import (
	"github.com/scylladb/scylla-manager/pkg/managerclient"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Shows cluster status",

	RunE: func(cmd *cobra.Command, args []string) error {
		var clusters []*managerclient.Cluster
		if cfgCluster == "" {
			var err error
			if clusters, err = client.ListClusters(ctx); err != nil {
				return err
			}
		} else {
			clusters = []*managerclient.Cluster{{ID: cfgCluster}}
		}

		w := cmd.OutOrStdout()
		for _, c := range clusters {
			if cfgCluster == "" {
				managerclient.FormatClusterName(w, c)
			}
			status, err := client.ClusterStatus(ctx, c.ID)
			if err != nil {
				return err
			}

			if err := render(w, status); err != nil {
				return err
			}
		}

		return nil
	},
}

func init() {
	cmd := statusCmd
	register(cmd, rootCmd)
}
