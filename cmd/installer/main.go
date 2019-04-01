package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/docker/compose-on-kubernetes/api/constants"
	"github.com/docker/compose-on-kubernetes/install"
	customflags "github.com/docker/compose-on-kubernetes/internal/flags"
	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	corev1types "k8s.io/api/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // For Google Kubernetes Engine authentication
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	if err := do(); err != nil {
		fmt.Fprintf(os.Stderr, "Installation failed: %s", err)
		os.Exit(1)
	}
}

func do() error {
	installerConfig, err := parseOptions()
	if err != nil {
		return err
	}
	if installerConfig.uninstall {
		err := install.Uninstall(installerConfig.restConfig, installerConfig.options.Namespace, false)
		if err != nil {
			return err
		}
		return install.WaitForUninstallCompletion(context.Background(), installerConfig.restConfig, installerConfig.options.Namespace, false)
	}
	return install.Safe(context.Background(), installerConfig.restConfig, installerConfig.options)
}

type installerConfig struct {
	uninstall  bool
	options    install.SafeOptions
	restConfig *rest.Config
}

func parseOptions() (installerConfig, error) {
	var flags additionalFlags
	var options install.SafeOptions
	parseFlags(&flags, &options)
	loggerLevel, err := log.ParseLevel(flags.logLevel)
	if err != nil {
		return installerConfig{}, err
	}
	log.SetLevel(loggerLevel)

	pp, err := parsePullPolicy(flags.pullPolicy)
	if err != nil {
		return installerConfig{}, err
	}
	options.OptionsCommon.PullPolicy = pp

	options.Etcd.ClientTLSBundle, err = flags.etcdBundle.load()
	if err != nil {
		return installerConfig{}, err
	}
	options.Network.CustomTLSBundle, err = flags.tlsBundle.load()
	if err != nil {
		return installerConfig{}, err
	}
	configFile, err := homedir.Expand(flags.kubeconfig)
	if err != nil {
		return installerConfig{}, err
	}
	config, err := clientcmd.BuildConfigFromFlags("", configFile)
	if err != nil {
		return installerConfig{}, err
	}
	if flags.skipLivenessProbes {
		options.HealthzCheckPort = 0
	}
	if flags.apiServerReplicas != 1 {
		replicas := int32(flags.apiServerReplicas)
		options.APIServerReplicas = &replicas
	}
	return installerConfig{
		options:    options,
		restConfig: config,
		uninstall:  flags.uninstall,
	}, nil
}

func parsePullPolicy(pullPolicy string) (corev1types.PullPolicy, error) {
	switch pp := corev1types.PullPolicy(pullPolicy); pp {
	case corev1types.PullAlways, corev1types.PullNever, corev1types.PullIfNotPresent:
		return pp, nil
	default:
		return "", fmt.Errorf("invalid pull policy: %q", pullPolicy)
	}
}

type additionalFlags struct {
	etcdBundle         certBundleSource
	tlsBundle          certBundleSource
	logLevel           string
	pullPolicy         string
	kubeconfig         string
	uninstall          bool
	skipLivenessProbes bool
	apiServerReplicas  int
}

func parseFlags(customFlags *additionalFlags, options *install.SafeOptions) {
	flag.BoolVar(&options.Network.ShouldUseHost, "network-host", false, "Use network host")
	flag.StringVar(&options.OptionsCommon.Namespace, "namespace", "docker", "Namespace in which to deploy")
	flag.DurationVar(&options.OptionsCommon.ReconciliationInterval, "reconciliation-interval", constants.DefaultFullSyncInterval, "Interval of reconciliation loop")
	flag.StringVar(&options.OptionsCommon.Tag, "tag", "latest", "Image tag")
	pullPolicyDescription := fmt.Sprintf("Image pull policy (%q|%q|%q)", corev1types.PullAlways, corev1types.PullNever, corev1types.PullIfNotPresent)
	flag.StringVar(&customFlags.pullPolicy, "pull-policy", string(corev1types.PullAlways), pullPolicyDescription)
	flag.StringVar(&options.Etcd.Servers, "etcd-servers", "", "etcd server addresses")
	flag.BoolVar(&customFlags.skipLivenessProbes, "skip-liveness-probes", false, "Disable liveness probe on Controller and API server deployments. Use this when HTTPS liveness probe fails.")
	flag.IntVar(&options.HealthzCheckPort, "healthz-check-port", 8080, "Defines the port used by healthz check server for api-server and controller (0 to disable it)")

	flag.StringVar(&customFlags.etcdBundle.ca, "etcd-ca-file", "", "CA of etcd TLS certificate")
	flag.StringVar(&customFlags.etcdBundle.cert, "etcd-cert-file", "", "TLS client certificate for accessing etcd")
	flag.StringVar(&customFlags.etcdBundle.key, "etcd-key-file", "", "TLS client private key for accessing etcd")

	flag.StringVar(&customFlags.tlsBundle.ca, "tls-ca-file", "", "CA used to generate the TLS certificate")
	flag.StringVar(&customFlags.tlsBundle.cert, "tls-cert-file", "", "Server TLS certificate (must be valid for name compose-api.<namespace>.svc)")
	flag.StringVar(&customFlags.tlsBundle.key, "tls-key-file", "", "Server TLS private key")
	flag.BoolVar(&customFlags.uninstall, "uninstall", false, "Uninstall")
	flag.StringVar(&customFlags.logLevel, "log-level", "info", `Set the log level ("debug"|"info"|"warn"|"error"|"fatal")`)
	flag.IntVar(&customFlags.apiServerReplicas, "apiserver-replicas", 1, "Number of replicas for the API Server")
	kubeconfigFlag := customflags.EnvString("kubeconfig", "~/.kube/config", "KUBECONFIG", "Path to a kubeconfig file (set to \"\" to use incluster config)")
	flag.Parse()
	customFlags.kubeconfig = kubeconfigFlag.String()
}

type certBundleSource struct {
	ca, cert, key string
}

func (s certBundleSource) empty() bool {
	return s.ca == "" && s.cert == "" && s.key == ""
}

func (s certBundleSource) load() (*install.TLSBundle, error) {
	if s.empty() {
		return nil, nil
	}
	if s.ca == "" || s.cert == "" || s.key == "" {
		return nil, errors.New("ca-file, cert-file and key-file must be set altogether or not at all")
	}
	caBytes, err := ioutil.ReadFile(s.ca)
	if err != nil {
		return nil, err
	}
	certBytes, err := ioutil.ReadFile(s.cert)
	if err != nil {
		return nil, err
	}
	keyBytes, err := ioutil.ReadFile(s.key)
	if err != nil {
		return nil, err
	}
	bundle, err := install.NewTLSBundle(caBytes, certBytes, keyBytes)
	if err != nil {
		return nil, err
	}
	return bundle, nil
}
