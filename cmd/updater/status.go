package main

import (
	"encoding/json"
	"os"

	"github.com/docker/compose-on-kubernetes/install"
	"github.com/spf13/cobra"
)

func newStatusCommand(cli *cli) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "reports the current installation status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := install.GetInstallStatus(cli.kubeconfig)
			if err != nil {
				return err
			}
			bytes, err := json.MarshalIndent(&status, "", "  ")
			if err != nil {
				return err
			}
			_, err = os.Stdout.Write(bytes)
			return err
		},
	}
}
