package e2e_migration

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"testing"
	"time"

	"github.com/docker/compose-on-kubernetes/api/constants"
	"github.com/docker/compose-on-kubernetes/install"
	"github.com/docker/compose-on-kubernetes/internal/e2e/cluster"
	e2ewait "github.com/docker/compose-on-kubernetes/internal/e2e/wait"
	homedir "github.com/mitchellh/go-homedir"
	. "github.com/onsi/ginkgo" // Import gomega to simplify test code
	ginkgocfg "github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	appsv1beta2types "k8s.io/api/apps/v1beta2"
	apiv1 "k8s.io/api/core/v1"
	rbacv1types "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/typed/core/v1"
	rbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd" // Import ginkgo to simplify test code
)

const (
	crdControllerVersion = "v0.2.15-ucp"
)

var (
	kubeconfig   = flag.String("kubeconfig", "", "Path to kube config file")
	outputDir    = flag.String("outputDir", os.Getenv("PWD"), "Test result output directory")
	fryNamespace = flag.String("namespace", "e2emigration", "Namespace to use for the test")
	tag          = flag.String("tag", "latest", "Image tag to use for the test")
)

func TestE2E(t *testing.T) {
	flag.Parse()
	junitReporter := reporters.NewJUnitReporter(fmt.Sprintf(path.Join(*outputDir, "junit_%d.xml"), ginkgocfg.GinkgoConfig.ParallelNode))
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Compose E2E Migration Test Suite", []Reporter{junitReporter})
}

var config *rest.Config

var _ = Describe("Compose migration", func() {
	It("Should migrate properly", func() {
		installOptCrd := []install.InstallerOption{
			install.WithUnsafe(install.UnsafeOptions{
				OptionsCommon: install.OptionsCommon{
					Namespace:              *fryNamespace,
					Tag:                    crdControllerVersion,
					ReconciliationInterval: constants.DefaultFullSyncInterval,
				},
			}),
			install.WithControllerImage("dockereng/kube-compose:" + crdControllerVersion),
			install.WithControllerOnly(),
			install.WithObjectFilter(func(o runtime.Object) (bool, error) {
				switch v := o.(type) {
				case *appsv1beta2types.Deployment:
					v.Spec.Template.Spec.ImagePullSecrets = []apiv1.LocalObjectReference{
						{Name: "migration-pull-secret"},
					}
				}
				return true, nil
			}),
		}
		installOptAPIAggregation := []install.InstallerOption{install.WithUnsafe(install.UnsafeOptions{
			OptionsCommon: install.OptionsCommon{
				Namespace:              *fryNamespace,
				Tag:                    *tag,
				ReconciliationInterval: constants.DefaultFullSyncInterval,
			}}),
			install.WithObjectFilter(func(o runtime.Object) (bool, error) {
				switch v := o.(type) {
				case *appsv1beta2types.Deployment:
					// change from pull always to pull never (image is already loaded, and not yet on hub)
					// only apply to 1st container in POD (2nd container for API is etcd, and we might need to pull it)
					v.Spec.Template.Spec.Containers[0].ImagePullPolicy = apiv1.PullNever
				}
				return true, nil
			}),
		}
		By("Installing compose CRD")
		err := install.CRDCRD(config)
		Expect(err).NotTo(HaveOccurred())
		// The roles setup by installer do no suit the CRD version of the controller
		rbacClient, err := rbacv1.NewForConfig(config)
		Expect(err).NotTo(HaveOccurred())
		_, err = rbacClient.ClusterRoleBindings().Get("crdcompose", metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			_, err = rbacClient.ClusterRoleBindings().Create(&rbacv1types.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "crdcompose",
					Labels: map[string]string{"com.docker.fry": "compose"},
				},
				RoleRef: rbacv1types.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     "cluster-admin",
				},
				Subjects: []rbacv1types.Subject{
					{
						Kind:      "ServiceAccount",
						Name:      "compose",
						Namespace: *fryNamespace,
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
		}
		err = install.Do(context.Background(), config, installOptCrd...)
		Expect(err).NotTo(HaveOccurred())

		By("Deploying some stacks")
		nsnginx, nsnginxcleanup, err := cluster.CreateNamespace(config, config, "nginx")
		Expect(err).NotTo(HaveOccurred())
		nsnginx.CreateStack(cluster.StackOperationV1beta1, "app", `version: '3.2'
services:
  back:
    image: nginx:1.12.1-alpine`)
		waitUntil(nsnginx.ContainsNPods(1))

		nsnginx.CreateStack(cluster.StackOperationV1beta1, "broken",
			`this is not
 a valid compose file
 `)
		nsnginx.CreateStack(cluster.StackOperationV1beta1, "invalid",
			`version: '3.2'
services:
  notsog@@d:
    image: nginx:1.12.1-alpine`)

		errs, err := install.DryRun(config)
		Expect(err).NotTo(HaveOccurred())
		Expect(errs).To(HaveLen(2))

		By("Backing up stacks")
		err = install.Backup(config, install.BackupPreviousErase)
		Expect(err).NotTo(HaveOccurred())

		By("Removing CRD")
		err = install.UninstallComposeCRD(config, *fryNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("Checking that our stacks are still live")
		waitUntil(nsnginx.ContainsNPods(1))

		By("Installing API server")
		err = install.Do(context.Background(), config, append(installOptAPIAggregation, install.WithoutController())...)
		Expect(err).NotTo(HaveOccurred())
		err = e2ewait.For(30, func() (bool, error) {
			return install.IsRunning(config)
		})
		Expect(err).NotTo(HaveOccurred())

		By("Restoring stacks")
		errs, err = install.Restore(config, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(errs).To(HaveLen(0)) // now that we have skip validation option, should have no error here

		By("Engaging controller")
		err = install.Do(context.Background(), config, append(installOptAPIAggregation, install.WithControllerOnly())...)
		Expect(err).NotTo(HaveOccurred())

		By("Checking our stacks are still good")
		waitUntil(nsnginx.ContainsNPods(1))

		By("Adding more stacks")
		nsnginx.CreateStack(cluster.StackOperationV1beta1, "app2", `version: '3.2'
services:
  backtwo:
    image: nginx:1.12.1-alpine`)
		waitUntil(nsnginx.ContainsNPods(2))

		By("Rolling back to CRD")
		err = install.Backup(config, install.BackupPreviousErase)
		Expect(err).NotTo(HaveOccurred())

		By("Uninstalling API server")
		err = install.UninstallComposeAPIServer(config, *fryNamespace)
		Expect(err).NotTo(HaveOccurred())
		// Our stacks should still be up
		waitUntil(nsnginx.ContainsNPods(2))

		By("Installing CRD")
		err = install.CRDCRD(config)
		Expect(err).NotTo(HaveOccurred())

		waitUntil(func() (bool, error) {
			_, err := nsnginx.StacksV1beta1().List(metav1.ListOptions{})
			return err == nil, nil
		})

		By("Restoring stacks")
		errs, err = install.Restore(config, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(errs).To(HaveLen(0))

		By("Starting controller")
		err = install.Do(context.Background(), config, installOptCrd...)
		Expect(err).NotTo(HaveOccurred())
		waitUntil(nsnginx.ContainsNPods(2))

		By("Cleaning up")
		nsnginxcleanup()
	})
})

func waitUntil(condition wait.ConditionFunc) {
	ExpectWithOffset(1, wait.PollImmediate(1*time.Second, 5*time.Minute, condition)).NotTo(HaveOccurred())
}

var _ = SynchronizedBeforeSuite(func() []byte { return nil },
	func(_ []byte) {
		kcfg := *kubeconfig
		if kcfg == "" {
			kcfg = os.Getenv("KUBECONFIG")
		}
		kcfg, _ = homedir.Expand(kcfg)
		var err error
		config, err = clientcmd.BuildConfigFromFlags("", kcfg)
		Expect(err).NotTo(HaveOccurred())
	})

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
				Expect(cont.RestartCount).To(Equal(int32(0)), "container %s/%s was restarted", pod.Name, cont.Name)
			}
		}
	})
