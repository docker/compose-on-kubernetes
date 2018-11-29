package main

import (
	"github.com/docker/compose-on-kubernetes/install"
	"github.com/spf13/cobra"
)

func newBackupCommand(cli *cli) *cobra.Command {
	return &cobra.Command{
		Use:   "backup",
		Short: "backup stacks to a backup CRD",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return install.Backup(cli.kubeconfig, install.BackupPreviousErase)
		},
	}
}
