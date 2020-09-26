package apis

import (
	"k8s.io/client-go/tools/clientcmd"
)

// NewKubernetesConfig creates a ClientConfig from the specified Kubernetes configuration file,
// or if empty, the KUBECONFIG environment variable per Kubernetes default CLI processing.
func NewKubernetesConfig(configPath string) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// Ignored if an empty string.
	loadingRules.ExplicitPath = configPath
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, &clientcmd.ConfigOverrides{})
}
