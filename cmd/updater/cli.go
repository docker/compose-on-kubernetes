package main

import (
	customflags "github.com/docker/compose-on-kubernetes/internal/flags"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type cliOptions struct {
	kubeconfigPath *pflag.Flag
}

func (o *cliOptions) addFlags(flags *pflag.FlagSet) {
	o.kubeconfigPath = customflags.EnvStringCobra(flags, "kubeconfig", "~/.kube/config", "KUBECONFIG", "path to a kubeconfig file (set to \"\" to use incluster config)")
}

type cli struct {
	kubeconfig *rest.Config
}

func (c *cli) initialize(opts *cliOptions) error {
	kubeconfigPath, err := homedir.Expand(opts.kubeconfigPath.Value.String())
	if err != nil {
		return err
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return err
	}
	c.kubeconfig = config
	return nil
}
