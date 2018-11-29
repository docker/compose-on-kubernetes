package main

import (
	"github.com/docker/compose-on-kubernetes/install"
	"github.com/spf13/cobra"
)

func newUninstallAPICommand(cli *cli) *cobra.Command {
	opts := &namespaceOption{}
	cmd := &cobra.Command{
		Use:   "uninstall-api [--namespace=<namespace>]",
		Short: "uninstall api",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return install.UninstallComposeAPIServer(cli.kubeconfig, opts.namespace)
		},
	}
	opts.addFlags(cmd.Flags())
	return cmd
}
