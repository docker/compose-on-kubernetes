package cli

import (
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/compose-on-kubernetes/api/compose/v1beta1"
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta2"
	"github.com/docker/compose-on-kubernetes/api/openapi"
	"github.com/docker/compose-on-kubernetes/internal/apiserver"
	"github.com/docker/compose-on-kubernetes/internal/internalversion"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	genericopenapi "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	certutil "k8s.io/client-go/util/cert"
	apiregistrationv1beta1types "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
	kubeaggreagatorv1beta1 "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1beta1"
)

const defaultEtcdPathPrefix = "/registry/docker.com/stacks"

type apiServerOptions struct {
	RecommendedOptions *genericoptions.RecommendedOptions
	serviceNamespace   string
	serviceName        string
	caBundleFile       string
}

// NewCommandStartComposeServer provides a CLI handler for 'start master' command
func NewCommandStartComposeServer(stopCh <-chan struct{}) *cobra.Command {
	codec := apiserver.Codecs.LegacyCodec(internalversion.StorageSchemeGroupVersion)

	o := &apiServerOptions{
		RecommendedOptions: genericoptions.NewRecommendedOptions(defaultEtcdPathPrefix, codec),
	}

	cmd := &cobra.Command{
		Short: "Launch a compose API server",
		Long:  "Launch a compose API server",
		RunE: func(c *cobra.Command, args []string) error {
			errors := []error{}
			errors = append(errors, o.RecommendedOptions.Validate()...)
			if err := utilerrors.NewAggregate(errors); err != nil {
				return err
			}
			return runComposeServer(o, stopCh)
		},
	}

	flags := cmd.Flags()
	o.RecommendedOptions.AddFlags(flags)
	flags.StringVar(&o.serviceNamespace, "service-namespace", "", "defines the namespace of the service exposing the aggregated API")
	flags.StringVar(&o.serviceName, "service-name", "", "defines the name of the service exposing the aggregated API")
	flags.StringVar(&o.caBundleFile, "ca-bundle-file", "", "defines the path to the CA bundle file")
	return cmd
}

func generateCertificateIfRequired(o *apiServerOptions) error {
	if o.RecommendedOptions.SecureServing.ServerCert.CertKey.CertFile != "" && o.RecommendedOptions.SecureServing.ServerCert.CertKey.KeyFile != "" {
		return nil
	}
	// generate tls bundle
	caKey, err := certutil.NewPrivateKey()
	if err != nil {
		return err
	}
	hostName, err := os.Hostname()
	if err != nil {
		return err
	}
	caCert, err := certutil.NewSelfSignedCACert(certutil.Config{
		CommonName: "compose-api-ca-" + strings.ToLower(hostName),
	}, caKey)
	if err != nil {
		return err
	}
	key, err := certutil.NewPrivateKey()
	if err != nil {
		return err
	}
	cfg := certutil.Config{
		CommonName: fmt.Sprintf("%s.%s.svc", o.serviceName, o.serviceNamespace),
		AltNames: certutil.AltNames{
			DNSNames: []string{"localhost", fmt.Sprintf("%s.%s.svc", o.serviceName, o.serviceNamespace)},
			IPs:      []net.IP{loopbackIP},
		},
		Usages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	cert, err := certutil.NewSignedCert(cfg, key, caCert, caKey)

	if err != nil {
		return err
	}

	keyPEM := certutil.EncodePrivateKeyPEM(key)
	certPEM := certutil.EncodeCertPEM(cert)
	caPEM := certutil.EncodeCertPEM(caCert)

	dir, err := ioutil.TempDir("", "compose-tls-generated")
	if err != nil {
		return err
	}

	keyPath := filepath.Join(dir, "server.key")
	certPath := filepath.Join(dir, "server.crt")
	caPath := filepath.Join(dir, "ca.crt")

	o.caBundleFile = caPath
	o.RecommendedOptions.SecureServing.ServerCert = genericoptions.GeneratableKeyCert{
		CertKey: genericoptions.CertKey{
			CertFile: certPath,
			KeyFile:  keyPath,
		},
	}
	if err = ioutil.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return err
	}
	if err = ioutil.WriteFile(certPath, certPEM, 0600); err != nil {
		return err
	}
	if err = ioutil.WriteFile(caPath, caPEM, 0600); err != nil {
		return err
	}

	go func() {
		// We don't want to actually reach tls timeout
		// so before it is reached, we exit with non-zero result code in order to let Kubernetes
		// restart the POD which will re-create a new TLS bundle
		expiry := caCert.NotAfter
		if cert.NotAfter.Before(expiry) {
			expiry = cert.NotAfter
		}
		timeout := getTimeout(time.Until(expiry))
		time.Sleep(timeout)
		fmt.Fprint(os.Stderr, "certificate bundle needs regeneration")
		os.Exit(1)

	}()
	return nil
}

func getTimeout(tlsExpireTimeout time.Duration) time.Duration {
	rand.Seed(time.Now().UnixNano())
	factor := (rand.Float64() / 2) + 0.49
	timeoutSeconds := factor * tlsExpireTimeout.Seconds()
	return time.Duration(float64(time.Second) * timeoutSeconds)
}

var loopbackIP = net.ParseIP("127.0.0.1")

func runComposeServer(o *apiServerOptions, stopCh <-chan struct{}) error {
	if err := generateCertificateIfRequired(o); err != nil {
		return err
	}
	caBundle, err := ioutil.ReadFile(o.caBundleFile)
	if err != nil {
		return err
	}
	serverConfig := genericapiserver.NewRecommendedConfig(apiserver.Codecs)
	if err := o.RecommendedOptions.ApplyTo(serverConfig, apiserver.Scheme); err != nil {
		return err
	}
	serverConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(openapi.GetOpenAPIDefinitions, genericopenapi.NewDefinitionNamer(apiserver.Scheme))
	serverConfig.OpenAPIConfig.Info.Title = "Kube-compose API"
	serverConfig.OpenAPIConfig.Info.Version = "v1beta2"

	config := &apiserver.Config{
		GenericConfig: serverConfig,
	}

	server, err := config.Complete().New(o.RecommendedOptions.CoreAPI.CoreAPIKubeconfigPath)
	if err != nil {
		return err
	}

	aggregatorClient, err := kubeaggreagatorv1beta1.NewForConfig(serverConfig.ClientConfig)
	if err != nil {
		return err
	}
	server.GenericAPIServer.AddPostStartHook("start-compose-server-informers", func(context genericapiserver.PostStartHookContext) error {
		config.GenericConfig.SharedInformerFactory.Start(context.StopCh)
		apiServiceV1beta1 := &apiregistrationv1beta1types.APIService{
			ObjectMeta: metav1.ObjectMeta{
				Name: "v1beta1.compose.docker.com",
				Labels: map[string]string{
					"com.docker.fry": "compose.api",
				},
			},
			Spec: apiregistrationv1beta1types.APIServiceSpec{
				CABundle:             caBundle,
				Group:                v1beta1.SchemeGroupVersion.Group,
				GroupPriorityMinimum: 1000,
				VersionPriority:      15,
				Version:              v1beta1.SchemeGroupVersion.Version,
				Service: &apiregistrationv1beta1types.ServiceReference{
					Namespace: o.serviceNamespace,
					Name:      o.serviceName,
				},
			},
		}

		existing, err := aggregatorClient.APIServices().Get("v1beta1.compose.docker.com", metav1.GetOptions{})
		if err == nil {
			bundle, err := mergeCABundle(existing.Spec.CABundle, caBundle)
			if err != nil {
				return err
			}
			apiServiceV1beta1.ObjectMeta.ResourceVersion = existing.ObjectMeta.ResourceVersion
			apiServiceV1beta1.Spec.CABundle = bundle
			if _, err := aggregatorClient.APIServices().Update(apiServiceV1beta1); err != nil {
				return err
			}
		} else {
			if _, err := aggregatorClient.APIServices().Create(apiServiceV1beta1); err != nil {
				return err
			}
		}

		apiServiceV1beta2 := &apiregistrationv1beta1types.APIService{
			ObjectMeta: metav1.ObjectMeta{
				Name: "v1beta2.compose.docker.com",
				Labels: map[string]string{
					"com.docker.fry": "compose.api",
				},
			},
			Spec: apiregistrationv1beta1types.APIServiceSpec{
				CABundle:             caBundle,
				Group:                v1beta2.SchemeGroupVersion.Group,
				GroupPriorityMinimum: 1000,
				VersionPriority:      16,
				Version:              v1beta2.SchemeGroupVersion.Version,
				Service: &apiregistrationv1beta1types.ServiceReference{
					Namespace: o.serviceNamespace,
					Name:      o.serviceName,
				},
			},
		}
		existing, err = aggregatorClient.APIServices().Get("v1beta2.compose.docker.com", metav1.GetOptions{})
		if err == nil {
			bundle, err := mergeCABundle(existing.Spec.CABundle, caBundle)
			if err != nil {
				return err
			}
			apiServiceV1beta2.ObjectMeta.ResourceVersion = existing.ObjectMeta.ResourceVersion
			apiServiceV1beta2.Spec.CABundle = bundle
			if _, err := aggregatorClient.APIServices().Update(apiServiceV1beta2); err != nil {
				return err
			}
		} else {
			if _, err := aggregatorClient.APIServices().Create(apiServiceV1beta2); err != nil {
				return err
			}
		}
		return nil
	})

	return server.GenericAPIServer.PrepareRun().Run(stopCh)
}

func mergeCABundle(existingPEM, newBundlePEM []byte) ([]byte, error) {
	if len(existingPEM) == 0 {
		return newBundlePEM, nil
	}
	existing, err := certutil.ParseCertsPEM(existingPEM)
	if err != nil {
		// existing bundle is unparsable. override it
		return newBundlePEM, nil
	}
	newBundle, err := certutil.ParseCertsPEM(newBundlePEM)
	if err != nil {
		return nil, err
	}
	if len(newBundle) != 1 {
		return nil, errors.New("bundle has an unexpected number of certificates in it")
	}
	commonName := newBundle[0].Subject.CommonName
	var result []byte
	for _, existingCert := range existing {
		if existingCert.Subject.CommonName != commonName {
			result = append(result, certutil.EncodeCertPEM(existingCert)...)
		}
	}
	result = append(result, newBundlePEM...)
	return result, nil
}
