package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"

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

func parseOptions(uninstall *bool, options *install.SafeOptions) *rest.Config {
	var etcdCA, etcdCert, etcdKey, tlsCA, tlsCert, tlsKey string
	var logLevel string
	var pullPolicy string
	flag.BoolVar(&options.Network.ShouldUseHost, "network-host", false, "Use network host")
	flag.StringVar(&options.OptionsCommon.Namespace, "namespace", "docker", "Namespace in which to deploy")
	flag.DurationVar(&options.OptionsCommon.ReconciliationInterval, "reconciliation-interval", constants.DefaultFullSyncInterval, "Interval of reconciliation loop")
	flag.StringVar(&options.OptionsCommon.Tag, "tag", "latest", "Image tag")
	flag.StringVar(&pullPolicy, "pull-policy", string(corev1types.PullAlways), fmt.Sprintf("Image pull policy (%q|%q|%q)", corev1types.PullAlways, corev1types.PullNever, corev1types.PullIfNotPresent))
	flag.StringVar(&options.Etcd.Servers, "etcd-servers", "", "etcd server addresses")
	flag.BoolVar(&options.SkipLivenessProbes, "skip-liveness-probes", false, "Disable liveness probe on Controller and API server deployments. Use this when HTTPS liveness probe fails.")

	flag.StringVar(&etcdCA, "etcd-ca-file", "", "CA of etcd TLS certificate")
	flag.StringVar(&etcdCert, "etcd-cert-file", "", "TLS client certificate for accessing etcd")
	flag.StringVar(&etcdKey, "etcd-key-file", "", "TLS client private key for accessing etcd")

	flag.StringVar(&tlsCA, "tls-ca-file", "", "CA used to generate the TLS certificate")
	flag.StringVar(&tlsCert, "tls-cert-file", "", "Server TLS certificate (must be valid for name compose-api.<namespace>.svc)")
	flag.StringVar(&tlsKey, "tls-key-file", "", "Server TLS private key")
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

	pp, err := parsePullPolicy(pullPolicy)
	if err != nil {
		panic(err)
	}
	options.OptionsCommon.PullPolicy = pp

	if etcdCA != "" || etcdCert != "" || etcdKey != "" {
		if etcdCA == "" || etcdCert == "" || etcdKey == "" {
			panic("etcd-ca-file, etcd-cert-file and etcd-key-file must be set altogether or not at all")
		}
	}
	if tlsCA != "" || tlsCert != "" || tlsKey != "" {
		if tlsCA == "" || tlsCert == "" || tlsKey == "" {
			panic("tls-ca-file, tls-cert-file and tls-key-file must be set altogether or not at all")
		}
	}

	options.Etcd.ClientTLSBundle = loadTLSBundle(etcdCA, etcdCert, etcdKey)
	options.Network.CustomTLSBundle = loadTLSBundle(tlsCA, tlsCert, tlsKey)

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

func parsePullPolicy(pullPolicy string) (corev1types.PullPolicy, error) {
	switch pp := corev1types.PullPolicy(pullPolicy); pp {
	case corev1types.PullAlways, corev1types.PullNever, corev1types.PullIfNotPresent:
		return pp, nil
	default:
		return "", fmt.Errorf("invalid pull policy: %q", pullPolicy)
	}
}

func loadTLSBundle(caFile, certFile, keyFile string) *install.TLSBundle {
	var caBytes, certBytes, keyBytes []byte
	loadFile(caFile, &caBytes)
	loadFile(certFile, &certBytes)
	loadFile(keyFile, &keyBytes)
	if caBytes == nil {
		return nil
	}
	bundle, err := install.NewTLSBundle(caBytes, certBytes, keyBytes)
	if err != nil {
		panic(err)
	}
	return bundle
}

func main() {
	var uninstall bool
	var options install.SafeOptions

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
	err := install.Safe(context.Background(), config, options)
	if err != nil {
		panic(err)
	}
}

func loadFile(filePath string, tofill *[]byte) {
	if filePath == "" {
		return
	}
	res, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(err)
	}
	*tofill = res
}
