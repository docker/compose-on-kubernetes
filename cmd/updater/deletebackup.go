package main

import (
	"github.com/docker/compose-on-kubernetes/install"
	"github.com/spf13/cobra"
)

func newDeleteBackupCommand(cli *cli) *cobra.Command {
	return &cobra.Command{
		Use:   "delete-backup",
		Short: "delete backup CRD",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return install.DeleteBackup(cli.kubeconfig)
		},
	}
}
