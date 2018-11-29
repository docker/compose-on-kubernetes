package main

import (
	"github.com/docker/compose-on-kubernetes/install"
	"github.com/spf13/cobra"
)

func newInstallCrdCrdCommand(cli *cli) *cobra.Command {
	return &cobra.Command{
		Use:   "install-crd-crd",
		Short: "Install CRD component of CRD mode",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return install.CRDCRD(cli.kubeconfig)
		},
	}
}
