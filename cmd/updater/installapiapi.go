package main

import (
	"context"

	"github.com/docker/compose-on-kubernetes/api/constants"
	"github.com/docker/compose-on-kubernetes/install"
	"github.com/spf13/cobra"
)

func newInstallAPIAPICommand(cli *cli) *cobra.Command {
	opts := &namespaceOption{}
	cmd := &cobra.Command{
		Use:   "install-api-api <tag> [--namespace=<namespace>]",
		Short: "Install API server component",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tag := args[0]
			installOptions := install.OptionsCommon{
				Namespace:              opts.namespace,
				Tag:                    tag,
				ReconciliationInterval: constants.DefaultFullSyncInterval,
			}
			return install.Do(context.Background(), cli.kubeconfig, install.WithUnsafe(
				install.UnsafeOptions{
					OptionsCommon: installOptions,
				}), install.WithoutController())
		},
	}
	opts.addFlags(cmd.Flags())
	return cmd
}
