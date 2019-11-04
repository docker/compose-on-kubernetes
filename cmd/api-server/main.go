package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/docker/compose-on-kubernetes/cmd/api-server/cli"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/component-base/logs"
)

func main() {
	if err := start(func(*cobra.Command) error { return nil }); err != nil {
		panic(err)
	}
}

func start(init func(*cobra.Command) error) error {
	logs.InitLogs()
	defer logs.FlushLogs()
	stop := make(chan struct{})
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Infof("Received signal: %v", sig)
		close(stop)
	}()
	cmd := cli.NewCommandStartComposeServer(stop)
	cmd.Flags().AddFlagSet(pflag.CommandLine)
	if err := init(cmd); err != nil {
		return err
	}
	return cmd.Execute()
}
