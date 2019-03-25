package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/compose-on-kubernetes/internal/keys"

	"github.com/stretchr/testify/assert"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	certutil "k8s.io/client-go/util/cert"
)

func TestGenerateBundleAndMerge(t *testing.T) {
	hostname, err := os.Hostname()
	assert.NoError(t, err)
	oldOpts := &apiServerOptions{
		RecommendedOptions: &genericoptions.RecommendedOptions{
			SecureServing: &genericoptions.SecureServingOptionsWithLoopback{
				SecureServingOptions: &genericoptions.SecureServingOptions{},
			},
		},
		serviceName:      "old-service-name",
		serviceNamespace: "old-namespace",
	}
	err = generateCertificateIfRequired(oldOpts)
	if oldOpts.caBundleFile != "" {
		defer os.RemoveAll(filepath.Dir(oldOpts.caBundleFile))
	}
	assert.NoError(t, err)
	oldCA, err := certutil.CertsFromFile(oldOpts.caBundleFile)
	assert.NoError(t, err)
	assert.Len(t, oldCA, 1)
	assert.Equal(t, "compose-api-ca-"+strings.ToLower(hostname), oldCA[0].Subject.CommonName)
	oldCert, err := certutil.CertsFromFile(oldOpts.RecommendedOptions.SecureServing.ServerCert.CertKey.CertFile)
	assert.NoError(t, err)
	assert.Len(t, oldCert, 1)
	assert.Equal(t, "old-service-name.old-namespace.svc", oldCert[0].Subject.CommonName)
	assert.Contains(t, oldCert[0].DNSNames, "old-service-name.old-namespace.svc")
	assert.Contains(t, oldCert[0].DNSNames, "localhost")
	assert.Len(t, oldCert[0].IPAddresses, 1)
	assert.True(t, oldCert[0].IPAddresses[0].Equal(loopbackIP))

	newOpts := &apiServerOptions{
		RecommendedOptions: &genericoptions.RecommendedOptions{
			SecureServing: &genericoptions.SecureServingOptionsWithLoopback{
				SecureServingOptions: &genericoptions.SecureServingOptions{},
			},
		},
		serviceName:      "new-service-name",
		serviceNamespace: "new-namespace",
	}
	err = generateCertificateIfRequired(newOpts)
	if newOpts.caBundleFile != "" {
		defer os.RemoveAll(filepath.Dir(newOpts.caBundleFile))
	}
	assert.NoError(t, err)
	newCA, err := certutil.CertsFromFile(newOpts.caBundleFile)
	assert.NoError(t, err)

	otherCa, err := keys.NewSelfSignedCA("other-ca", nil)
	assert.NoError(t, err)
	oldCABundle := append(keys.EncodeCertPEM(oldCA[0]), keys.EncodeCertPEM(otherCa.Cert())...)
	expectedNewCABundle := append(keys.EncodeCertPEM(otherCa.Cert()), keys.EncodeCertPEM(newCA[0])...)
	newBundle, err := mergeCABundle(oldCABundle, keys.EncodeCertPEM(newCA[0]))
	assert.NoError(t, err)
	assert.EqualValues(t, newBundle, expectedNewCABundle)
}
