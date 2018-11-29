package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func main() {
	var flags *pflag.FlagSet
	cli := &cli{}
	cliOpts := &cliOptions{}
	root := &cobra.Command{
		Use:              fmt.Sprintf("%s [OPTIONS] COMMAND [ARG...]", os.Args[0]),
		TraverseChildren: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return cli.initialize(cliOpts)
		},
	}
	flags = root.PersistentFlags()
	cliOpts.addFlags(flags)
	root.AddCommand(
		newUpdateCommand(cli),
		newBackupCommand(cli),
		newRestoreCommand(cli),
		newDryrunCommand(cli),
		newDeleteBackupCommand(cli),
		newInstallCrdCrdCommand(cli),
		newInstallCrdControllerCommand(cli),
		newInstallAPIAPICommand(cli),
		newInstallAPIControllerCommand(cli),
		newUninstallCrdCommand(cli),
		newUninstallAPICommand(cli),
		newStatusCommand(cli),
	)
	root.Execute()
}
