package main

import (
	"github.com/docker/compose-on-kubernetes/install"
	"github.com/spf13/cobra"
)

func newDryrunCommand(cli *cli) *cobra.Command {
	return &cobra.Command{
		Use:   "dry-run",
		Short: "validates existing stacks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			failedStacks, err := install.DryRun(cli.kubeconfig)
			printErrs(failedStacks)
			return err
		},
	}
}
