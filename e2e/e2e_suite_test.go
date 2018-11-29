package e2e

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"testing"
	"time"

	"github.com/docker/compose-on-kubernetes/internal/e2e/compose"
	homedir "github.com/mitchellh/go-homedir"
	// Import ginkgo to simplify test code
	. "github.com/onsi/ginkgo"
	ginkgocfg "github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	// Import gomega to simplify test code
	. "github.com/onsi/gomega"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/typed/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var config *rest.Config

var (
	kubeconfig           = envString{defaultValue: "~/.kube/config", envVarName: "KUBECONFIG"}
	outputDir            = flag.String("outputDir", os.Getenv("PWD"), "Test result output directory")
	fryNamespace         = flag.String("namespace", "e2e", "Namespace to use for the test")
	tag                  = flag.String("tag", "latest", "Image tag to use for the test")
	pullSecret           = flag.String("pull-secret", "", "Docker Hub secret for pulling image")
	skipProvisioning     = flag.Bool("skip-provisioning", false, "Skip deployment/cleanup of compose fry and tiller service")
	publishedServiceType = flag.String("published-service-type", "LoadBalancer", "Service type for published ports (LoadBalancer|NodePort)")
)

func TestE2E(t *testing.T) {
	flag.Var(&kubeconfig, "kubeconfig", "Path to a kube config. Only required if out-of-cluster. (default is ~/.kube/config, and can be overridden with ${KUBECONFIG})")
	flag.Parse()
	junitReporter := reporters.NewJUnitReporter(fmt.Sprintf(path.Join(*outputDir, "junit_%d.xml"), ginkgocfg.GinkgoConfig.ParallelNode))
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Compose E2E Test Suite", []Reporter{junitReporter})
}

var (
	cleanup       func()
	tillerCleanup func()
)

var _ = SynchronizedBeforeSuite(func() []byte {
	if !*skipProvisioning {
		setupConfig()
		client, err := v1.NewForConfig(config)
		Expect(err).NotTo(HaveOccurred())
		if _, err := client.Namespaces().Get(*fryNamespace, metav1.GetOptions{}); err == nil {
			Expect(err).To(HaveOccurred())
		}

		cleanup, err = compose.Install(config, *fryNamespace, *tag, *pullSecret)
		Expect(err).NotTo(HaveOccurred())
	}
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(1 * time.Second)
	return nil
}, func(_ []byte) {
	setupConfig()
	rand.Seed(time.Now().UTC().UnixNano())
})

func setupConfig() {
	configFile, err := homedir.Expand(kubeconfig.String())
	Expect(err).NotTo(HaveOccurred())
	config, err = clientcmd.BuildConfigFromFlags("", configFile)
	Expect(err).NotTo(HaveOccurred())
}

var _ = SynchronizedAfterSuite(func() {},
	func() {
		// check for restarts
		fmt.Printf("Checking for restarts in %s\n", *fryNamespace)
		client, err := v1.NewForConfig(config)
		Expect(err).NotTo(HaveOccurred())
		pods, err := client.Pods(*fryNamespace).List(metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, pod := range pods.Items {
			for _, cont := range pod.Status.ContainerStatuses {
				if cont.RestartCount != 0 {
					// dump previous container logs
					fmt.Fprintf(os.Stderr, "\nPrevious logs for %s/%s\n", pod.Name, cont.Name)
					data, err := client.Pods(*fryNamespace).GetLogs(pod.Name, &apiv1.PodLogOptions{Container: cont.Name, Previous: true}).Stream()
					Expect(err).NotTo(HaveOccurred())
					io.Copy(os.Stderr, data)
				}
				fmt.Fprintf(os.Stderr, "\nCurrent logs for %s/%s\n", pod.Name, cont.Name)
				data, err := client.Pods(*fryNamespace).GetLogs(pod.Name, &apiv1.PodLogOptions{Container: cont.Name}).Stream()
				Expect(err).NotTo(HaveOccurred())
				io.Copy(os.Stderr, data)
			}
		}
		fmt.Fprintln(os.Stderr, "\nPods details:")
		podsJSON, err := json.MarshalIndent(pods, "", "  ")
		Expect(err).NotTo(HaveOccurred())
		os.Stderr.Write(podsJSON)
		if cleanup != nil {
			cleanup()
		}
		if tillerCleanup != nil {
			tillerCleanup()
		}
	})

type envString struct {
	isSet         bool
	defaultValue  string
	explicitValue string
	envVarName    string
}

func (v *envString) String() string {
	if v.isSet {
		return v.explicitValue
	}
	if envValue, ok := os.LookupEnv(v.envVarName); ok {
		return envValue
	}
	return v.defaultValue
}

func (v *envString) Set(value string) error {
	v.isSet = true
	v.explicitValue = value
	return nil
}
