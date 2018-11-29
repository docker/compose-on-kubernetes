package main

import (
	"github.com/docker/compose-on-kubernetes/install"
	"github.com/spf13/cobra"
)

type updateOptions struct {
	namespace          string
	skipPreflightCheck bool
}

func newUpdateCommand(cli *cli) *cobra.Command {
	opts := &updateOptions{}
	cmd := &cobra.Command{
		Use:   "update <tag>",
		Short: "Update compose component preserving existing stacks.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tag := args[0]
			errs, err := install.Update(cli.kubeconfig, opts.namespace, tag, !opts.skipPreflightCheck)
			printErrs(errs)
			return err
		},
	}
	cmd.Flags().BoolVarP(&opts.skipPreflightCheck, "skip-preflight-check", "s", false, "Skip preliminary stack validation step")
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "kube-system", "Kubernetes namespace in which to deploy components")
	return cmd
}
