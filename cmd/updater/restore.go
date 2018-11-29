package main

import (
	"fmt"

	"github.com/docker/compose-on-kubernetes/install"
	"github.com/spf13/cobra"
)

func printErrs(errs map[string]error) {
	if len(errs) == 0 {
		return
	}
	fmt.Println("Some errors where encountered:")
	for k, v := range errs {
		fmt.Printf("  %s: %v\n", k, v)
	}
}

type restoreOptions struct {
	impersonate bool
}

func newRestoreCommand(cli *cli) *cobra.Command {
	opts := &restoreOptions{}
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "restore backup",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			errs, err := install.Restore(cli.kubeconfig, opts.impersonate)
			printErrs(errs)
			return err
		},
	}
	cmd.Flags().BoolVarP(&opts.impersonate, "impersonate", "i", false, "Converts UCP owner annotations to impersonated REST API calls")
	return cmd
}
