package main

import (
	"github.com/docker/compose-on-kubernetes/install"
	"github.com/spf13/cobra"
)

func newUninstallCrdCommand(cli *cli) *cobra.Command {
	opts := &namespaceOption{}
	cmd := &cobra.Command{
		Use:   "uninstall-crd [--namespace=<namespace>]",
		Short: "uninstall CRD",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return install.UninstallComposeCRD(cli.kubeconfig, opts.namespace)
		},
	}
	opts.addFlags(cmd.Flags())
	return cmd
}
