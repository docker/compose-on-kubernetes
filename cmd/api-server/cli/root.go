package cli

import (
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/compose-on-kubernetes/api/compose/v1alpha3"
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta1"
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta2"
	"github.com/docker/compose-on-kubernetes/api/openapi"
	"github.com/docker/compose-on-kubernetes/internal/apiserver"
	"github.com/docker/compose-on-kubernetes/internal/internalversion"
	"github.com/docker/compose-on-kubernetes/internal/keys"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	genericopenapi "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/healthz"
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
	healthzCheckPort   int
}

// NewCommandStartComposeServer provides a CLI handler for 'start master' command
func NewCommandStartComposeServer(stopCh <-chan struct{}) *cobra.Command {
	codec := apiserver.Codecs.LegacyCodec(internalversion.StorageSchemeGroupVersion)

	o := &apiServerOptions{
		RecommendedOptions: genericoptions.NewRecommendedOptions(defaultEtcdPathPrefix, codec, nil),
	}

	cmd := &cobra.Command{
		Short: "Launch a compose API server",
		Long:  "Launch a compose API server",
		RunE: func(c *cobra.Command, args []string) error {
			o.RecommendedOptions.ProcessInfo = genericoptions.NewProcessInfo("compose-on-kubernetes", o.serviceNamespace)
			errors := []error{}
			errors = append(errors, o.RecommendedOptions.Validate()...)
			if err := utilerrors.NewAggregate(errors); err != nil {
				return err
			}
			nextBackoff := time.Second
			var err error
			for attempt := 0; attempt < 8; attempt++ {
				err = runComposeServer(o, stopCh)
				// If the compose-api starts before the API server is listening then we can get a transient connection refused
				// while looking up missing authentication information in the cluster (see #120).
				if err != nil && strings.Contains(err.Error(), "connection refused") {
					fmt.Fprintf(os.Stderr, "unable to start compose server: %s. Will retry in %s\n", err, nextBackoff)
					time.Sleep(nextBackoff)
					nextBackoff = nextBackoff * 2
					continue
				}
				return err
			}
			fmt.Fprintf(os.Stderr, "giving up trying to start compose server")
			return err
		},
	}

	flags := cmd.Flags()
	o.RecommendedOptions.AddFlags(flags)
	flags.StringVar(&o.serviceNamespace, "service-namespace", "", "defines the namespace of the service exposing the aggregated API")
	flags.StringVar(&o.serviceName, "service-name", "", "defines the name of the service exposing the aggregated API")
	flags.StringVar(&o.caBundleFile, "ca-bundle-file", "", "defines the path to the CA bundle file")
	flags.IntVar(&o.healthzCheckPort, "healthz-check-port", 8080, "defines the port used by healthz check server (0 to disable it)")
	return cmd
}

func generateCertificateIfRequired(o *apiServerOptions) error {
	if o.RecommendedOptions.SecureServing.ServerCert.CertKey.CertFile != "" && o.RecommendedOptions.SecureServing.ServerCert.CertKey.KeyFile != "" {
		return nil
	}
	// generate tls bundle
	hostName, err := os.Hostname()
	if err != nil {
		return err
	}
	ca, err := keys.NewSelfSignedCA("compose-api-ca-"+strings.ToLower(hostName), nil)
	if err != nil {
		return err
	}
	key, err := keys.NewRSASigner()
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

	cert, err := ca.NewSignedCert(cfg, key.Public())
	if err != nil {
		return err
	}

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
	if err = ioutil.WriteFile(keyPath, key.PEM(), 0600); err != nil {
		return err
	}
	if err = ioutil.WriteFile(certPath, keys.EncodeCertPEM(cert), 0600); err != nil {
		return err
	}
	if err = ioutil.WriteFile(caPath, keys.EncodeCertPEM(ca.Cert()), 0600); err != nil {
		return err
	}

	go func() {
		// We don't want to actually reach tls timeout
		// so before it is reached, we exit with non-zero result code in order to let Kubernetes
		// restart the POD which will re-create a new TLS bundle
		expiry := ca.Cert().NotAfter
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

func registerAggregatedAPIs(aggregatorClient kubeaggreagatorv1beta1.APIServiceInterface, caBundle []byte, serviceNamespace, serviceName string, apiVersions ...string) error {
	for ix, v := range apiVersions {
		if err := registerAggregatedAPI(aggregatorClient, caBundle, serviceNamespace, serviceName, v, int32(ix+15)); err != nil {
			return err
		}
	}
	return nil
}

func registerAggregatedAPI(aggregatorClient kubeaggreagatorv1beta1.APIServiceInterface,
	caBundle []byte, serviceNamespace, serviceName, apiVersion string, versionPriority int32) error {
	apiService := &apiregistrationv1beta1types.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: apiVersion + ".compose.docker.com",
			Labels: map[string]string{
				"com.docker.fry": "compose.api",
			},
		},
		Spec: apiregistrationv1beta1types.APIServiceSpec{
			CABundle:             caBundle,
			Group:                v1beta1.GroupName,
			GroupPriorityMinimum: 1000,
			VersionPriority:      versionPriority,
			Version:              apiVersion,
			Service: &apiregistrationv1beta1types.ServiceReference{
				Namespace: serviceNamespace,
				Name:      serviceName,
			},
		},
	}

	existing, err := aggregatorClient.Get(apiVersion+".compose.docker.com", metav1.GetOptions{})
	if err == nil {
		bundle, err := mergeCABundle(existing.Spec.CABundle, caBundle)
		if err != nil {
			return err
		}
		apiService.ObjectMeta.ResourceVersion = existing.ObjectMeta.ResourceVersion
		apiService.Spec.CABundle = bundle
		if _, err := aggregatorClient.Update(apiService); err != nil {
			return err
		}
	} else {
		if _, err := aggregatorClient.Create(apiService); err != nil {
			return err
		}
	}
	return nil
}

func runComposeServer(o *apiServerOptions, stopCh <-chan struct{}) error {
	if err := generateCertificateIfRequired(o); err != nil {
		return err
	}
	caBundle, err := ioutil.ReadFile(o.caBundleFile)
	if err != nil {
		return err
	}
	serverConfig := genericapiserver.NewRecommendedConfig(apiserver.Codecs)
	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
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

	if o.healthzCheckPort > 0 {
		err = server.GenericAPIServer.AddPostStartHook("start-compose-server-healthz-endpoint", func(context genericapiserver.PostStartHookContext) error {
			m := http.NewServeMux()
			healthz.InstallHandler(m, server.GenericAPIServer.HealthzChecks()...)
			srv := &http.Server{
				Addr:    fmt.Sprintf(":%d", o.healthzCheckPort),
				Handler: m,
			}
			go srv.ListenAndServe()
			go func() {
				<-context.StopCh
				srv.Close()
			}()
			return nil
		})
		if err != nil {
			return err
		}
	}
	err = server.GenericAPIServer.AddPostStartHook("start-compose-server-informers", func(context genericapiserver.PostStartHookContext) error {
		config.GenericConfig.SharedInformerFactory.Start(context.StopCh)
		return registerAggregatedAPIs(
			aggregatorClient.APIServices(),
			caBundle,
			o.serviceNamespace,
			o.serviceName,
			v1beta1.SchemeGroupVersion.Version, v1beta2.SchemeGroupVersion.Version, v1alpha3.SchemeGroupVersion.Version,
		)
	})
	if err != nil {
		return err
	}

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
			result = append(result, keys.EncodeCertPEM(existingCert)...)
		}
	}
	result = append(result, newBundlePEM...)
	return result, nil
}
