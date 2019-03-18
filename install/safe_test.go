package install

import (
	"testing"
	"time"

	"k8s.io/client-go/rest"
)

func getSafeOptions() SafeOptions {
	safeOptions := SafeOptions{
		OptionsCommon: OptionsCommon{
			Namespace:              "compose",
			Tag:                    "latest",
			PullSecret:             "",
			ReconciliationInterval: time.Hour * 12,
			DefaultServiceType:     "com.docker.default-service-type",
			APIServerAffinity:      nil,
			ControllerAffinity:     nil,
			SkipLivenessProbes:     true,
			PullPolicy:             "Always",
		},
		Etcd: EtcdOptions{
			Servers:         "http://compose-etcd-client:2379",
			ClientTLSBundle: nil,
		},
		Network: NetworkOptions{
			ShouldUseHost:   false,
			CustomTLSBundle: nil,
			Port:            0,
		},
	}

	return safeOptions
}

func TestInstallerWithSafeOptions(t *testing.T) {
	options := getSafeOptions()
	var restConfig *rest.Config

}
