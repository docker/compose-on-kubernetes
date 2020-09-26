package main

import (
	"context"
	"flag"

	kubernetes "github.com/docker/compose-on-kubernetes/api"
	"github.com/docker/compose-on-kubernetes/api/constants"
	"github.com/docker/compose-on-kubernetes/install"
	log "github.com/sirupsen/logrus" // For Google Kubernetes Engine authentication
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
)

func parseOptions(uninstall *bool, options *install.UnsafeOptions) *rest.Config {
	var logLevel string
	var kubeconfig string
	flag.StringVar(&options.OptionsCommon.Namespace, "namespace", "docker", "Namespace in which to deploy")
	flag.DurationVar(&options.OptionsCommon.ReconciliationInterval, "reconciliation-interval", constants.DefaultFullSyncInterval, "Interval of reconciliation loop")
	flag.StringVar(&options.OptionsCommon.Tag, "tag", "latest", "Image tag")
	flag.BoolVar(uninstall, "uninstall", false, "Uninstall")
	flag.StringVar(&logLevel, "log-level", "info", `Set the log level ("debug"|"info"|"warn"|"error"|"fatal")`)
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig file")
	flag.Parse()

	loggerLevel, err := log.ParseLevel(logLevel)
	if err != nil {
		panic(err)
	}
	log.SetLevel(loggerLevel)

	config, err := kubernetes.NewKubernetesConfig(kubeconfig).ClientConfig()
	if err != nil {
		panic(err)
	}
	return config
}

func main() {
	var uninstall bool
	var options install.UnsafeOptions
	options.Debug = true

	config := parseOptions(&uninstall, &options)

	if uninstall {
		err := install.Uninstall(config, options.Namespace, false)
		if err != nil {
			panic(err)
		}
		err = install.WaitForUninstallCompletion(context.Background(), config, options.Namespace, false)
		if err != nil {
			panic(err)
		}
		return
	}
	err := install.Unsafe(context.Background(), config, options)
	if err != nil {
		panic(err)
	}
}
