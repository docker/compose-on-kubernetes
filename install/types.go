package install

import (
	"errors"
	"time"

	corev1types "k8s.io/api/core/v1"
)

// OptionsCommon holds install options for the api extension
type OptionsCommon struct {
	Namespace              string
	Tag                    string
	PullSecret             string
	ReconciliationInterval time.Duration
	DefaultServiceType     string
	APIServerAffinity      *corev1types.Affinity
	ControllerAffinity     *corev1types.Affinity
	HealthzCheckPort       int
	PullPolicy             corev1types.PullPolicy
	APIServerReplicas      *int32
}

// UnsafeOptions holds install options for the api extension
type UnsafeOptions struct {
	OptionsCommon
	Coverage bool
	Debug    bool
}

// EtcdOptions holds install options related to ETCD
type EtcdOptions struct {
	Servers         string
	ClientTLSBundle *TLSBundle
}

// NetworkOptions holds install options related to networking
type NetworkOptions struct {
	ShouldUseHost   bool
	CustomTLSBundle *TLSBundle
	Port            int32
}

// SafeOptions holds install options for the api extension
type SafeOptions struct {
	OptionsCommon
	Etcd    EtcdOptions
	Network NetworkOptions
}

// TLSBundle is a bundle containing a CA, a public cert and private key, PEM encoded
type TLSBundle struct {
	ca   []byte
	cert []byte
	key  []byte
}

// NewTLSBundle creates a TLS bundle
func NewTLSBundle(ca, cert, key []byte) (*TLSBundle, error) {
	if ca == nil || cert == nil || key == nil {
		return nil, errors.New("ca, cert or key is missing")
	}
	return &TLSBundle{
		ca:   ca,
		cert: cert,
		key:  key,
	}, nil
}
