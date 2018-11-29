package main

import (
	"context"

	"github.com/docker/compose-on-kubernetes/api/constants"
	"github.com/docker/compose-on-kubernetes/install"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type namespaceOption struct {
	namespace string
}

func (o *namespaceOption) addFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&o.namespace, "namespace", "n", "kube-system", "Kubernetes namespace in which to deploy components")
}

func newInstallCrdControllerCommand(cli *cli) *cobra.Command {
	opts := &namespaceOption{}
	cmd := &cobra.Command{
		Use:   "install-crd-controller <tag> [--namespace=<namespace>]",
		Short: "Install CRD controller",
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
				}), install.WithControllerOnly())
		},
	}
	opts.addFlags(cmd.Flags())
	return cmd
}
