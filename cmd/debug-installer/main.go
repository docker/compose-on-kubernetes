package main

import (
	"context"
	"flag"

	"github.com/docker/compose-on-kubernetes/api/constants"
	"github.com/docker/compose-on-kubernetes/install"
	customflags "github.com/docker/compose-on-kubernetes/internal/flags"
	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus" // For Google Kubernetes Engine authentication
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func parseOptions(uninstall *bool, options *install.UnsafeOptions) *rest.Config {
	var logLevel string
	flag.StringVar(&options.OptionsCommon.Namespace, "namespace", "docker", "Namespace in which to deploy")
	flag.DurationVar(&options.OptionsCommon.ReconciliationInterval, "reconciliation-interval", constants.DefaultFullSyncInterval, "Interval of reconciliation loop")
	flag.StringVar(&options.OptionsCommon.Tag, "tag", "latest", "Image tag")
	flag.BoolVar(uninstall, "uninstall", false, "Uninstall")
	flag.StringVar(&logLevel, "log-level", "info", `Set the log level ("debug"|"info"|"warn"|"error"|"fatal")`)
	kubeconfigFlag := customflags.EnvString("kubeconfig", "~/.kube/config", "KUBECONFIG", "Path to a kubeconfig file (set to \"\" to use incluster config)")
	flag.Parse()
	kubeconfig := kubeconfigFlag.String()

	loggerLevel, err := log.ParseLevel(logLevel)
	if err != nil {
		panic(err)
	}
	log.SetLevel(loggerLevel)

	configFile, err := homedir.Expand(kubeconfig)
	if err != nil {
		panic(err)
	}
	config, err := clientcmd.BuildConfigFromFlags("", configFile)
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
