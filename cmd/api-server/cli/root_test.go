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

// TestGenerateBundleAndMerge simulate 2 consecutive runs of the API server for the same POD (e.g.: simulating node crash)
// It checks 2 aspects:
// - the generated CA and Cert have correct info encoded in them
// - when a new CA is generated for the same POD as another one, the "merge" operation discard the old CA from the bundle (but keeps other CAS valid for other PODs)
func TestGenerateBundleAndMerge(t *testing.T) {
	hostname, err := os.Hostname()
	assert.NoError(t, err)
	// simulate the generation of the first CA+Cert bundle
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

	// load old CA from the generated file, and checks its characteristics
	oldCA, err := certutil.CertsFromFile(oldOpts.caBundleFile)
	assert.NoError(t, err)
	assert.Len(t, oldCA, 1)
	assert.Equal(t, "compose-api-ca-"+strings.ToLower(hostname), oldCA[0].Subject.CommonName)

	// load old cert from the generated file and checks its characteristics
	oldCert, err := certutil.CertsFromFile(oldOpts.RecommendedOptions.SecureServing.ServerCert.CertKey.CertFile)
	assert.NoError(t, err)
	assert.Len(t, oldCert, 1)
	assert.Equal(t, "old-service-name.old-namespace.svc", oldCert[0].Subject.CommonName)
	assert.Contains(t, oldCert[0].DNSNames, "old-service-name.old-namespace.svc")
	assert.Contains(t, oldCert[0].DNSNames, "localhost")
	assert.Len(t, oldCert[0].IPAddresses, 1)
	assert.True(t, oldCert[0].IPAddresses[0].Equal(loopbackIP))

	// simulate the generation of the new CA for the same POD
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

	// create an other CA that should not be touched by the merge
	otherCa, err := keys.NewSelfSignedCA("other-ca", nil)
	assert.NoError(t, err)

	// old ca bundle contains both the old CA and the "other" CA
	oldCABundle := append(keys.EncodeCertPEM(oldCA[0]), keys.EncodeCertPEM(otherCa.Cert())...)

	// after merging, the bundle should contain the new CA and the "other" CA (but not the old CA)
	expectedNewCABundle := append(keys.EncodeCertPEM(otherCa.Cert()), keys.EncodeCertPEM(newCA[0])...)
	newBundle, err := mergeCABundle(oldCABundle, keys.EncodeCertPEM(newCA[0]))
	assert.NoError(t, err)
	assert.EqualValues(t, newBundle, expectedNewCABundle)
}
