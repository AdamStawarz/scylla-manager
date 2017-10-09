// Copyright (C) 2017 ScyllaDB

package command

import (
	"context"
	"fmt"

	"github.com/scylladb/mermaid/restapiclient/client/operations"
)

// RepairUnitList lists repair units.
type RepairUnitList struct {
	BaseClientCommand
	clusterID string
}

// Synopsis implements cli.Command.
func (cmd *RepairUnitList) Synopsis() string {
	return "Shows repair units within a cluster"
}

// InitFlags sets the command flags.
func (cmd *RepairUnitList) InitFlags() {
	f := cmd.BaseCommand.NewFlagSet(cmd)
	f.StringVar(&cmd.clusterID, "cluster", "", "ID or name of a cluster.")
}

// Run implements cli.Command.
func (cmd *RepairUnitList) Run(args []string) int {
	// parse command line arguments
	if err := cmd.Parse(args); err != nil {
		cmd.UI.Error(fmt.Sprintf("Command line error: %s", err))
		return 1
	}

	// get client
	m := cmd.client()

	resp, err := m.GetClusterClusterIDRepairUnits(&operations.GetClusterClusterIDRepairUnitsParams{
		Context:   context.Background(),
		ClusterID: cmd.clusterID,
	})
	if err != nil {
		cmd.UI.Error(fmt.Sprintf("Host %s: %s", cmd.APIHost, err))
		return 1
	}

	t := newTable("unit id", "keyspace", "tables")
	for _, p := range resp.Payload {
		t.append(p.ID, p.Keyspace, p.Tables)
	}
	cmd.UI.Info(t.String())

	return 0
}
