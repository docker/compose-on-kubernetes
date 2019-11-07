package e2e

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strings"
	"time"

	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta1"
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta2"
	"github.com/docker/compose-on-kubernetes/api/labels"
	"github.com/docker/compose-on-kubernetes/internal/e2e/cluster"
	. "github.com/onsi/ginkgo" // Import ginkgo to simplify test code
	. "github.com/onsi/gomega" // Import gomega to simplify test code
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	portMin                  = 32768
	portMax                  = 35535
	privateImagePullUsername = "composeonk8simagepull"
	privateImagePullPassword = "XHWl8mJ6IH5o"
	privateImagePullImage    = "composeonkubernetes/nginx:1.12.1-alpine"
)

var usedPorts = map[int]struct{}{}

func getRandomPort() int {
	candidate := rand.Intn(portMax-portMin) + portMin
	if _, ok := usedPorts[candidate]; ok {
		return getRandomPort()
	}
	usedPorts[candidate] = struct{}{}
	return candidate
}

const deployNamespace = "e2e-tests"

const defaultStrategy = cluster.StackOperationV1beta2Compose

func scaleStack(s *latest.Stack, service string, replicas int) (*latest.Stack, error) {
	stack := s.Clone()
	for i, svc := range stack.Spec.Services {
		if svc.Name != service {
			continue
		}
		r := uint64(replicas)
		stack.Spec.Services[i].Deploy.Replicas = &r
		return stack, nil
	}
	return nil, errors.New(service + " not found")
}

func testUpdate(ns *cluster.Namespace, create, update cluster.StackOperationStrategy) {
	By("Creating a stack")
	port := getRandomPort()
	_, err := ns.CreateStack(create, "app", fmt.Sprintf(`version: '3.2'
services:
  front:
    image: nginx:1.12.1-alpine
    ports:
    - %d:80`, port))
	expectNoError(err)
	waitUntil(ns.IsStackAvailable("app"))
	waitUntil(ns.IsServiceResponding(fmt.Sprintf("front-published:%d-tcp", port), "proxy/index.html", "Welcome to nginx!"))

	By("Updating the stack")
	_, err = ns.UpdateStack(update, "app", fmt.Sprintf(`version: '3.2'
services:
  front:
    image: httpd:2.4.27-alpine
    ports:
    - %d:80`, port))
	expectNoError(err)
	By("Verifying the stack has been updated")
	waitUntil(ns.IsServiceResponding(fmt.Sprintf("front-published:%d-tcp", port), "proxy/index.html", "It works!"))
}

func skipIfNoStorageClass(ns *cluster.Namespace) {
	h, err := ns.HasStorageClass()
	expectNoError(err)
	if !h {
		Skip("Cluster does not have any storage class")
	}
}

var _ = Describe("Compose fry", func() {

	var (
		ns      *cluster.Namespace
		cleanup func()
	)

	BeforeEach(func() {
		ns, cleanup = createNamespace()
	})

	AfterEach(func() {
		cleanup()
	})

	It("Should contain zero stack", func() {
		waitUntil(ns.ContainsZeroStack())
	})

	It("Should deploy a stack", func() {
		composeFile := `version: '3.2'
services:
  back:
    image: nginx:1.12.1-alpine`
		By("Creating a stack")
		_, err := ns.CreateStack(defaultStrategy, "app", composeFile)

		expectNoError(err)

		By("Verifying the stack Spec")
		items, err := ns.ListStacks()

		expectNoError(err)
		Expect(items).To(HaveLen(1))
		Expect(items[0].Name).To(Equal("app"))
		var cf latest.ComposeFile
		err = ns.RESTClientV1alpha3().Get().Namespace(ns.Name()).Name("app").Resource("stacks").SubResource("composefile").Do().Into(&cf)
		expectNoError(err)
		Expect(cf.ComposeFile).To(Equal(composeFile))

		waitUntil(ns.ContainsNPods(1))

		pods, err := ns.ListAllPods()

		expectNoError(err)
		Expect(pods).To(HaveLen(1))
		imgName := pods[0].Spec.Containers[0].Image
		Expect(imgName).To(ContainSubstring("nginx:1.12.1-alpine"))

		deployments, err := ns.ListDeployments("")

		By("Verifying the Deployment ownerRef")
		expectNoError(err)
		Expect(deployments).To(HaveLen(1))
		Expect(deployments[0].OwnerReferences).To(HaveLen(1))

		ownerRef := deployments[0].OwnerReferences[0]

		Expect(ownerRef.Kind).To(Equal("Stack"))
		Expect(ownerRef.Name).To(Equal(items[0].Name))
		Expect(ownerRef.UID).To(Equal(items[0].UID))
	})

	It("Should update a stack (yaml yaml)", func() { testUpdate(ns, cluster.StackOperationV1beta2Compose, cluster.StackOperationV1beta2Compose) })
	It("Should update a stack (yaml stack)", func() { testUpdate(ns, cluster.StackOperationV1beta2Compose, cluster.StackOperationV1beta2Stack) })
	It("Should update a stack (stack yaml)", func() { testUpdate(ns, cluster.StackOperationV1beta2Stack, cluster.StackOperationV1beta2Compose) })
	It("Should update a stack (stack stack)", func() { testUpdate(ns, cluster.StackOperationV1beta2Stack, cluster.StackOperationV1beta2Stack) })
	It("Should update a stack (v1alpha3 v1alpha3)", func() { testUpdate(ns, cluster.StackOperationV1alpha3, cluster.StackOperationV1alpha3) })
	It("Should update a stack (v1beta2 v1alpha3)", func() { testUpdate(ns, cluster.StackOperationV1beta2Stack, cluster.StackOperationV1alpha3) })
	It("Should update a stack (v1beta1 v1alpha3)", func() { testUpdate(ns, cluster.StackOperationV1beta1, cluster.StackOperationV1alpha3) })

	It("Should update a stack with orphans", func() {
		_, err := ns.CreateStack(defaultStrategy, "app", `version: '3.2'
services:
  front:
    image: httpd:2.4.27-alpine
  orphan:
    image: nginx`)
		expectNoError(err)
		waitUntil(ns.ContainsNPodsMatchingSelector(1, stackServiceLabel("orphan")))

		_, err = ns.UpdateStack(defaultStrategy, "app", `version: '3.2'
services:
  front:
    image: nginx`)
		expectNoError(err)
		waitUntil(ns.ContainsNPodsMatchingSelector(0, stackServiceLabel("orphan")))
		waitUntil(ns.IsServiceNotPresent(stackServiceLabel("orphan")))
	})

	It("Should update Deployement to StatefulSet", func() {
		_, err := ns.CreateStack(defaultStrategy, "app", `version: '3.2'
services:
  front:
    image: nginx`)
		expectNoError(err)

		waitUntil(ns.ContainsNPodsMatchingSelector(1, stackServiceLabel("front")))

		pods, err := ns.ListPods(stackServiceLabel("front"))
		expectNoError(err)

		references := pods[0].ObjectMeta.OwnerReferences
		Expect(references).To(HaveLen(1))
		Expect(references[0].Kind).To(Equal("ReplicaSet"))

		_, err = ns.UpdateStack(defaultStrategy, "app", `version: '3.2'
services:
  front:
    image: nginx
    volumes:
    - data:/tmp
volumes:
  data:`)
		expectNoError(err)

		waitUntil(ns.ContainsNPodsWithPredicate(1, stackServiceLabel("front"), func(pod corev1.Pod) (bool, string) {
			references := pod.ObjectMeta.OwnerReferences
			if len(references) != 1 {
				return false, "Owner reference not updated"
			}
			if references[0].Kind != "StatefulSet" {
				return false, "Wrong owner reference: " + references[0].Kind
			}
			return true, ""
		}))
	})

	It("Should remove a stack", func() {
		_, err := ns.CreateStack(defaultStrategy, "app", `version: '3.2'
services:
  back:
    image: nginx:1.12.1-alpine`)
		expectNoError(err)

		waitUntil(ns.ContainsNPods(1))

		err = ns.DeleteStack("app")
		expectNoError(err)

		waitUntil(ns.ContainsZeroPod())
	})

	It("Should fail when removing not existing stack", func() {
		err := ns.DeleteStack("unknown")
		Expect(err).To(HaveOccurred())
	})

	It("Should scale as expected", func() {
		spec := `version: '3.2'
services:
  back:
    image: nginx:1.12.1-alpine`

		stack, err := ns.CreateStack(cluster.StackOperationV1beta2Stack, "app", spec)
		expectNoError(err)

		waitUntil(ns.ContainsNPods(1))

		stack, err = scaleStack(stack, "back", 3)
		expectNoError(err)

		_, err = ns.UpdateStackFromSpec("app", stack)
		expectNoError(err)

		waitUntil(ns.ContainsNPods(3))

		stack, err = scaleStack(stack, "back", 1)
		expectNoError(err)

		_, err = ns.UpdateStackFromSpec("app", stack)
		expectNoError(err)

		waitUntil(ns.ContainsNPods(1))
	})

	It("Should scale using the scale subresource", func() {
		spec := `version: '3.2'
services:
  back:
    image: nginx:1.12.1-alpine`

		_, err := ns.CreateStack(defaultStrategy, "app", spec)
		expectNoError(err)

		waitUntil(ns.ContainsNPods(1))
		scalerV1beta2 := v1beta2.Scale{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app",
			},
			Spec: map[string]int{"back": 2},
		}
		err = ns.RESTClientV1beta2().Put().Namespace(ns.Name()).Name("app").Resource("stacks").SubResource("scale").Body(&scalerV1beta2).Do().Error()
		expectNoError(err)
		waitUntil(ns.ContainsNPods(2))

		scalerV1alpha3 := latest.Scale{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app",
			},
			Spec: map[string]int{"back": 1},
		}
		err = ns.RESTClientV1alpha3().Put().Namespace(ns.Name()).Name("app").Resource("stacks").SubResource("scale").Body(&scalerV1alpha3).Do().Error()
		expectNoError(err)
		waitUntil(ns.ContainsNPods(1))

		scalerV1alpha3.Spec["nope"] = 1
		err = ns.RESTClientV1alpha3().Put().Namespace(ns.Name()).Name("app").Resource("stacks").SubResource("scale").Body(&scalerV1alpha3).Do().Error()
		Expect(err).To(HaveOccurred())
	})

	It("Should place pods on the expected node", func() {
		nodes, err := ns.ListNodes()
		expectNoError(err)
		Expect(len(nodes)).To(BeNumerically(">=", 1))
		hostname := nodes[0].GetLabels()["kubernetes.io/hostname"]

		spec := `version: '3.2'
services:
  back:
    image: nginx:1.12.1-alpine
    deploy:
      replicas: 6
      placement:
        constraints:
         - node.hostname == ` + hostname

		_, err = ns.CreateStack(defaultStrategy, "app", spec)
		expectNoError(err)

		waitUntil(ns.ContainsNPods(6))
		pods, err := ns.ListPods("app")
		expectNoError(err)
		for _, pod := range pods {
			Expect(pod.Spec.Hostname).To(Equal(hostname))
		}
	})

	It("Should deploy Docker Pets", func() {
		port0 := getRandomPort()
		port1 := getRandomPort()
		port2 := getRandomPort()
		spec := fmt.Sprintf(`version: '3.1'
services:
    web:
        #Use following image to download from Docker Hub
        #image: chrch/docker-pets:latest
        image: chrch/docker-pets:1.0
        deploy:
            mode: replicated
            replicas: 2
        healthcheck:
            interval: 10s
            timeout: 5s
            retries: 3
        ports:
            - %d:5000
            - %d:7000
        environment:
            DB: 'db'
            THREADED: 'True'
        networks:
            - backend
    db:
        image: consul:0.7.2
        command: agent -server -ui -client=0.0.0.0 -bootstrap-expect=3 -retry-join=db -retry-join=db -retry-join=db -retry-interval 5s
        deploy:
            replicas: 3
        ports:
            - %d:8500
        environment:
            CONSUL_BIND_INTERFACE: 'eth2'
            CONSUL_LOCAL_CONFIG: '{"skip_leave_on_interrupt": true}'
        networks:
            - backend
networks:
    backend:
`, port0, port1, port2)
		_, err := ns.CreateStack(defaultStrategy, "app", spec)
		expectNoError(err)
		waitUntil(ns.ContainsNPods(5))
	})

	It("should keep pod accessible on non-exposed port", func() {
		port0 := getRandomPort()
		port1 := getRandomPort()
		spec := fmt.Sprintf(`version: '3.3'

services:
  web:
    build: web
    image: dockerdemos/lab-web
    ports:
     - "%d:80"

  words:
    build: words
    image: dockerdemos/lab-words
    ports:
     - "%d:8080"
    deploy:
      replicas: 5
      endpoint_mode: dnsrr
      resources:
        limits:
          memory: 64M
        reservations:
          memory: 64M

  db:
    build: db
    image: dockerdemos/lab-db`, port0, port1)

		_, err := ns.CreateStack(defaultStrategy, "app", spec)
		expectNoError(err)

		waitUntil(ns.IsServicePresent(stackServiceLabel("web")))

		services, err := ns.ListServices(stackServiceLabel("web"))
		expectNoError(err)
		Expect(len(services)).To(BeNumerically(">=", 1))

		waitUntil(ns.IsServiceResponding(fmt.Sprintf("web-published:%d-tcp", port0), "/proxy/words/verb", "\"word\""))
	})

	It("Should propagate deploy.labels to pod templates, selectors and services", func() {
		_, err := ns.CreateStack(cluster.StackOperationV1beta2Stack, "app", `version: "3"
services:
  simple:
    image: busybox:latest
    command: top
    labels:
      should-be-annotation: annotation
    deploy:
      labels:
        test-label: test-value`)
		expectNoError(err)

		waitUntil(ns.IsServicePresent(stackServiceLabel("simple")))
		services, err := ns.ListServices(stackServiceLabel("simple"))
		expectNoError(err)
		deps, err := ns.ListDeployments(stackServiceLabel("simple"))
		expectNoError(err)

		// check annotation on pod template
		Expect(deps[0].Spec.Template.Annotations["should-be-annotation"]).To(Equal("annotation"))
		// check service labels
		Expect(services[0].Labels["test-label"]).To(Equal("test-value"))
		// check pod template labels
		Expect(deps[0].Spec.Template.Labels["test-label"]).To(Equal("test-value"))
	})

	It("Should support deploy.labels updates", func() {
		_, err := ns.CreateStack(cluster.StackOperationV1beta2Stack, "app", `version: "3"
services:
  simple:
    image: busybox:latest
    command: top
    labels:
      should-be-annotation: annotation
    deploy:
      labels:
        test-label: test-value`)
		expectNoError(err)
		waitUntil(ns.IsStackAvailable("app"))
		_, err = ns.UpdateStack(cluster.StackOperationV1beta2Stack, "app", `version: "3"
services:
  simple:
    image: nginx:1.13-alpine
    labels:
      should-be-annotation: annotation
    deploy:
      labels:
        test-label: test-value-updated
        second-test: test-value`)
		expectNoError(err)

		waitUntil(ns.ContainsNPodsWithPredicate(1, stackServiceLabel("simple"), func(pod corev1.Pod) (bool, string) {
			return pod.Spec.Containers[0].Image == "nginx:1.13-alpine", ""
		}))

		s, err := ns.GetStack("app")
		expectNoError(err)
		Expect(s.Status.Phase).NotTo(Equal(latest.StackFailure))
		waitUntil(ns.IsStackAvailable("app"))
	})

	It("Should transform ports with the correct rules and keep inter pod communication working", func() {
		port0 := getRandomPort()
		port1 := getRandomPort()
		spec := fmt.Sprintf(`version: '3.3'
services:
  web:
    image: nginx:1.13-alpine
    ports:
     - "%d:80"
     - "%d:81"
     - "82"
     - "83"`, port0, port1)
		_, err := ns.CreateStack(defaultStrategy, "app", spec)
		expectNoError(err)
		waitUntil(ns.ServiceCount(stackServiceLabel("web"), 3))
		services, err := ns.ListServices(stackServiceLabel("web"))
		expectNoError(err)
		Expect(len(services)).To(Equal(3)) // 1 headless service for inter pod, 1 loadbalancer for 80:80 and 81:81, 1 node-port for 82 and 83
		var headless, pub, np *corev1.Service
		for _, svc := range services {
			localSvc := svc
			switch {
			case strings.HasSuffix(localSvc.Name, "-published"):
				pub = &localSvc
			case strings.HasSuffix(localSvc.Name, "-random-ports"):
				np = &localSvc
			default:
				headless = &localSvc
			}
		}
		Expect(headless).NotTo(BeNil(), "Service web has no headless service!")
		Expect(pub).NotTo(BeNil(), "Service web has no published service!")
		Expect(np).NotTo(BeNil(), "Service web has no random ports service!")
		Expect(len(pub.Spec.Ports)).To(Equal(2))
		Expect(pub.Spec.Type).To(Equal(corev1.ServiceType(*publishedServiceType)))
		if corev1.ServiceType(*publishedServiceType) == corev1.ServiceTypeLoadBalancer {
			Expect(pub.Spec.Ports[0].Port).To(Equal(int32(port0)))
			Expect(pub.Spec.Ports[1].Port).To(Equal(int32(port1)))

		} else {
			Expect(pub.Spec.Ports[0].NodePort == int32(port0)).To(BeTrue())
			Expect(pub.Spec.Ports[1].NodePort == int32(port1)).To(BeTrue())
		}

		Expect(pub.Spec.Ports[0].TargetPort).To(Equal(intstr.FromInt(80)))
		Expect(pub.Spec.Ports[1].TargetPort).To(Equal(intstr.FromInt(81)))
		Expect(len(np.Spec.Ports)).To(Equal(2))
		Expect(np.Spec.Ports[0].TargetPort).To(Equal(intstr.FromInt(82)))
		Expect(np.Spec.Ports[1].TargetPort).To(Equal(intstr.FromInt(83)))

		// check cleanup
		expectNoError(ns.DeleteStack("app"))
		waitUntil(ns.ServiceCount(stackServiceLabel("web"), 0))
	})

	It("Should support raw stack creation", func() {
		kubeClient, err := kubernetes.NewForConfig(config)
		expectNoError(err)
		stackData := fmt.Sprintf(`{
"apiVersion": "compose.docker.com/v1beta1",
"kind": "Stack",
"metadata": {"name": "app", "namespace": "%s"},
"spec": {
  "composeFile": "version: '3.2'\nservices:\n  back:\n    image: nginx:1.12.1-alpine"
}
}`, ns.Name())
		res := kubeClient.CoreV1().RESTClient().Verb("POST").RequestURI(fmt.Sprintf("/apis/compose.docker.com/v1beta1/namespaces/%s/stacks", ns.Name())).Body([]byte(stackData)).Do()
		expectNoError(res.Error())
		waitUntil(ns.ContainsNPods(1))
		waitUntil(ns.IsStackAvailable("app"))
	})

	It("Should create and update in v1beta1", func() {
		port := getRandomPort()
		_, err := ns.CreateStack(cluster.StackOperationV1beta1, "app", fmt.Sprintf(`version: '3.2'
services:
  front:
    image: nginx:1.12.1-alpine
    ports:
    - %d:80`, port))
		expectNoError(err)
		waitUntil(ns.IsStackAvailable("app"))
		waitUntil(ns.IsServiceResponding(fmt.Sprintf("front-published:%d-tcp", port), "proxy/index.html", "Welcome to nginx!"))
		_, err = ns.UpdateStack(cluster.StackOperationV1beta1, "app", fmt.Sprintf(`version: '3.2'
services:
  front:
    image: httpd:2.4.27-alpine
    ports:
    - %d:80`, port))
		expectNoError(err)
		waitUntil(ns.IsServiceResponding(fmt.Sprintf("front-published:%d-tcp", port), "proxy/index.html", "It works!"))
	})

	It("Should handle broken compose files", func() {
		badspec := `version: '3.2'
services:
  back:
    image: nginx:1.12.1-alpine
      bla: bli
   blo: blo`

		_, err := ns.CreateStack(defaultStrategy, "app", badspec)
		Expect(err).To(HaveOccurred())
	})

	It("Should detect stack conflicts", func() {
		_, err := ns.CreateStack(cluster.StackOperationV1beta1, "app", `version: '3.2'
services:
  front:
    image: nginx:1.12.1-alpine`)
		expectNoError(err)
		waitUntil(ns.IsStackAvailable("app"))

		// create conflict
		_, err = ns.CreateStack(cluster.StackOperationV1beta1, "app2", `version: '3.2'
services:
  front:
    image: nginx:1.12.1-alpine`)
		Expect(err).To(HaveOccurred())

		_, err = ns.CreateStack(cluster.StackOperationV1beta1, "app2", `version: '3.2'
services:
  front2:
    image: nginx:1.12.1-alpine`)
		expectNoError(err)

		waitUntil(ns.IsStackAvailable("app2"))
		// update conflict
		_, err = ns.UpdateStack(cluster.StackOperationV1beta1, "app2", `version: '3.2'
services:
  front:
    image: nginx:1.12.1-alpine`)
		Expect(err).To(HaveOccurred())

		_, err = ns.Deployments().Create(&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "front3",
				Namespace: ns.Name(),
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "front3"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "front3"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "front3-nginx",
								Image: "nginx:1.12.1-alpine",
							},
						},
					},
				},
			},
		})
		expectNoError(err)
		_, err = ns.UpdateStack(cluster.StackOperationV1beta1, "app2", `version: '3.2'
services:
  front3:
    image: nginx:1.12.1-alpine`)
		Expect(err).To(HaveOccurred())
	})

	It("Should read logs correctly v1beta2", func() {
		port := getRandomPort()
		_, err := ns.CreateStack(cluster.StackOperationV1beta1, "app", fmt.Sprintf(`version: '3.2'
services:
  front:
    image: nginx:1.12.1-alpine
    ports:
    - %d:80`, port))
		expectNoError(err)
		waitUntil(ns.IsStackAvailable("app"))
		waitUntil(ns.IsServiceResponding(fmt.Sprintf("front-published:%d-tcp", port), "proxy/index.html", "Welcome to nginx!"))
		ns.IsServiceResponding(fmt.Sprintf("front-published:%d-tcp", port), "proxy/nope.html", "nope")
		s, err := ns.RESTClientV1beta2().Get().Namespace(ns.Name()).Name("app").Resource("stacks").SubResource("log").Stream()
		expectNoError(err)
		data, err := ioutil.ReadAll(s)
		expectNoError(err)
		s.Close()
		sdata := string(data)
		Expect(len(strings.Split(sdata, "\n"))).To(BeNumerically(">=", 2))
		Expect(strings.Contains(sdata, "GET")).To(BeTrue())
		// try with filter
		s, err = ns.RESTClientV1beta2().Get().Namespace(ns.Name()).Name("app").Resource("stacks").SubResource("log").Param("filter", "404").Stream()
		expectNoError(err)
		data, err = ioutil.ReadAll(s)
		expectNoError(err)
		s.Close()
		sdata = string(data)
		Expect(len(strings.Split(sdata, "\n"))).To(Equal(1))
	})

	It("Should read logs correctly v1alpha3", func() {
		port := getRandomPort()
		_, err := ns.CreateStack(cluster.StackOperationV1beta1, "app", fmt.Sprintf(`version: '3.2'
services:
  front:
    image: nginx:1.12.1-alpine
    ports:
    - %d:80`, port))
		expectNoError(err)
		waitUntil(ns.IsStackAvailable("app"))
		waitUntil(ns.IsServiceResponding(fmt.Sprintf("front-published:%d-tcp", port), "proxy/index.html", "Welcome to nginx!"))
		ns.IsServiceResponding(fmt.Sprintf("front-published:%d-tcp", port), "proxy/nope.html", "nope")
		s, err := ns.RESTClientV1alpha3().Get().Namespace(ns.Name()).Name("app").Resource("stacks").SubResource("log").Stream()
		expectNoError(err)
		data, err := ioutil.ReadAll(s)
		expectNoError(err)
		s.Close()
		sdata := string(data)
		Expect(len(strings.Split(sdata, "\n"))).To(BeNumerically(">=", 2))
		Expect(strings.Contains(sdata, "GET")).To(BeTrue())
		// try with filter
		s, err = ns.RESTClientV1alpha3().Get().Namespace(ns.Name()).Name("app").Resource("stacks").SubResource("log").Param("filter", "404").Stream()
		expectNoError(err)
		data, err = ioutil.ReadAll(s)
		expectNoError(err)
		s.Close()
		sdata = string(data)
		Expect(len(strings.Split(sdata, "\n"))).To(Equal(1))
	})

	It("Should follow logs correctly v1beta2", func() {
		port := getRandomPort()
		_, err := ns.CreateStack(cluster.StackOperationV1beta1, "app", fmt.Sprintf(`version: '3.2'
services:
  front:
    image: nginx:1.12.1-alpine
    ports:
    - %d:80`, port))
		expectNoError(err)
		waitUntil(ns.IsStackAvailable("app"))
		waitUntil(ns.IsServiceResponding(fmt.Sprintf("front-published:%d-tcp", port), "proxy/index.html", "Welcome to nginx!"))

		s, err := ns.RESTClientV1beta2().Get().Namespace(ns.Name()).Name("app").Resource("stacks").SubResource("log").Param("follow", "true").Stream()
		expectNoError(err)
		reader := bufio.NewReader(s)
		lineStream := make(chan string, 100)
		go func() {
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					lineStream <- "EOF"
					break
				}
				lineStream <- line
			}
		}()
		ok, _ := ns.IsServiceResponding(fmt.Sprintf("front-published:%d-tcp", port), "proxy/notexisting.html", "nope")()
		Expect(ok).To(BeFalse()) // 404
		ok = drainUntil(lineStream, "notexisting.html")
		Expect(ok).To(BeTrue())

		// shoot the pod
		pods, err := ns.ListAllPods()
		expectNoError(err)
		Expect(len(pods)).To(Equal(1))
		originalPodName := pods[0].Name
		ns.Pods().Delete(originalPodName, &metav1.DeleteOptions{})
		waitUntil(ns.PodIsActuallyRemoved(originalPodName))

		// check we get logs from the new pod
		waitUntil(ns.IsServiceResponding(fmt.Sprintf("front-published:%d-tcp", port), "proxy/index.html", "Welcome to nginx!"))
		ok, _ = ns.IsServiceResponding(fmt.Sprintf("front-published:%d-tcp", port), "proxy/notexisting2.html", "nope")()
		Expect(ok).To(BeFalse()) // 404
		ok = drainUntil(lineStream, "notexisting2.html")
		Expect(ok).To(BeTrue())
		s.Close()
	})

	It("Should follow logs correctly v1alpha3", func() {
		port := getRandomPort()
		_, err := ns.CreateStack(cluster.StackOperationV1beta1, "app", fmt.Sprintf(`version: '3.2'
services:
  front:
    image: nginx:1.12.1-alpine
    ports:
    - %d:80`, port))
		expectNoError(err)
		waitUntil(ns.IsStackAvailable("app"))
		waitUntil(ns.IsServiceResponding(fmt.Sprintf("front-published:%d-tcp", port), "proxy/index.html", "Welcome to nginx!"))

		s, err := ns.RESTClientV1alpha3().Get().Namespace(ns.Name()).Name("app").Resource("stacks").SubResource("log").Param("follow", "true").Stream()
		expectNoError(err)
		reader := bufio.NewReader(s)
		lineStream := make(chan string, 100)
		go func() {
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					lineStream <- "EOF"
					break
				}
				lineStream <- line
			}
		}()
		ok, _ := ns.IsServiceResponding(fmt.Sprintf("front-published:%d-tcp", port), "proxy/notexisting.html", "nope")()
		Expect(ok).To(BeFalse()) // 404
		ok = drainUntil(lineStream, "notexisting.html")
		Expect(ok).To(BeTrue())

		// shoot the pod
		pods, err := ns.ListAllPods()
		expectNoError(err)
		Expect(len(pods)).To(Equal(1))
		originalPodName := pods[0].Name
		ns.Pods().Delete(originalPodName, &metav1.DeleteOptions{})
		waitUntil(ns.PodIsActuallyRemoved(originalPodName))

		// check we get logs from the new pod
		waitUntil(ns.IsServiceResponding(fmt.Sprintf("front-published:%d-tcp", port), "proxy/index.html", "Welcome to nginx!"))
		ok, _ = ns.IsServiceResponding(fmt.Sprintf("front-published:%d-tcp", port), "proxy/notexisting2.html", "nope")()
		Expect(ok).To(BeFalse()) // 404
		ok = drainUntil(lineStream, "notexisting2.html")
		Expect(ok).To(BeTrue())
		s.Close()
	})

	It("Should fail fast when service name is invalid", func() {
		_, err := ns.CreateStack(cluster.StackOperationV1beta1, "app", `version: '3.2'
services:
  front_invalid:
    image: nginx:1.12.1-alpine`)
		Expect(err).To(HaveOccurred())
	})

	It("Should fail fast when volume name is invalid", func() {
		_, err := ns.CreateStack(cluster.StackOperationV1beta1, "app", `version: '3.2'
services:
  front:
    image: nginx:1.12.1-alpine
    volumes:
    - data_invalid:/tmp`)
		Expect(err).To(HaveOccurred())
	})

	It("Should fail fast when secret name is invalid", func() {
		_, err := ns.CreateStack(cluster.StackOperationV1beta1, "app", `version: '3.2'
services:
  front:
    image: nginx:1.12.1-alpine
    secrets:
    - data_invalid
secrets:
  data_invalid:
    external: true`)
		Expect(err).To(HaveOccurred())
	})

	It("Should fail on compose v2", func() {
		_, err := ns.CreateStack(cluster.StackOperationV1beta1, "app", `version: "2"
services:
  simple:
    image: busybox:latest
    command: top
  another:
    image: busybox:latest
    command: top`)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unsupported Compose file version: 2"))
	})

	It("Should fail on unallowed field", func() {
		_, err := ns.CreateStack(cluster.StackOperationV1beta1, "app", `version: "3"
services:
  simple:
    image: busybox:latest
    command: top
  another:
    image: busybox:latest
    command: top
not_allowed_field: hello`)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Additional property not_allowed_field is not allowed"))
	})

	It("Should fail on malformed yaml", func() {
		_, err := ns.CreateStack(cluster.StackOperationV1beta1, "app", `version: "3"
services
  simple:
    image: busybox:latest
    command: top
  another:
    image: busybox:latest
    command: top`)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("line 3: could not find expected ':'"))
	})

	It("Should support bind volumes", func() {
		_, err := ns.CreateStack(defaultStrategy, "app", `version: "3.4"
services:
  test:
    image: busybox:latest
    command:
    - /bin/sh
    - -c
    - "ls /tmp/hostetc/ ; sleep 3600"
    volumes:
      - type: bind
        source: /etc
        target: /tmp/hostetc`)
		expectNoError(err)
		waitUntil(ns.IsStackAvailable("app"))
		s, err := ns.RESTClientV1alpha3().Get().Namespace(ns.Name()).Name("app").Resource("stacks").SubResource("log").Stream()
		expectNoError(err)
		defer s.Close()
		data, err := ioutil.ReadAll(s)
		expectNoError(err)
		sdata := string(data)
		Expect(strings.Contains(sdata, "hosts")).To(BeTrue())
	})

	It("Should support volumes", func() {
		skipIfNoStorageClass(ns)
		_, err := ns.CreateStack(defaultStrategy, "app", `version: "3.4"
services:
  test:
    image: busybox:latest
    command:
    - /bin/sh
    - -c
    - "touch /tmp/mountvolume/somefile && echo success ; sleep 3600"
    volumes:
      - myvolume:/tmp/mountvolume
volumes:
  myvolume:`)
		expectNoError(err)
		waitUntil(ns.IsStackAvailable("app"))
		s, err := ns.RESTClientV1alpha3().Get().Namespace(ns.Name()).Name("app").Resource("stacks").SubResource("log").Stream()
		expectNoError(err)
		defer s.Close()
		data, err := ioutil.ReadAll(s)
		expectNoError(err)
		sdata := string(data)
		Expect(strings.Contains(sdata, "success")).To(BeTrue())
	})

	It("Should support anonymous volumes", func() {
		skipIfNoStorageClass(ns)
		_, err := ns.CreateStack(defaultStrategy, "app", `version: "3.4"
services:
  test:
    image: busybox:latest
    command:
    - /bin/sh
    - -c
    - "touch /tmp/mountvolume/somefile && echo success ; sleep 3600"
    volumes:
      - /tmp/mountvolume`)
		expectNoError(err)
		waitUntil(ns.IsStackAvailable("app"))
		s, err := ns.RESTClientV1alpha3().Get().Namespace(ns.Name()).Name("app").Resource("stacks").SubResource("log").Stream()
		expectNoError(err)
		defer s.Close()
		data, err := ioutil.ReadAll(s)
		expectNoError(err)
		sdata := string(data)
		Expect(strings.Contains(sdata, "success")).To(BeTrue())
	})

	It("Should survive a yaml bomb", func() {
		spec := `
version: "3"
services: &services ["lol","lol","lol","lol","lol","lol","lol","lol","lol"]
b: &b [*services,*services,*services,*services,*services,*services,*services,*services,*services]
c: &c [*b,*b,*b,*b,*b,*b,*b,*b,*b]
d: &d [*c,*c,*c,*c,*c,*c,*c,*c,*c]
e: &e [*d,*d,*d,*d,*d,*d,*d,*d,*d]
f: &f [*e,*e,*e,*e,*e,*e,*e,*e,*e]
g: &g [*f,*f,*f,*f,*f,*f,*f,*f,*f]
h: &h [*g,*g,*g,*g,*g,*g,*g,*g,*g]
i: &i [*h,*h,*h,*h,*h,*h,*h,*h,*h]`
		_, err := ns.CreateStack(cluster.StackOperationV1beta1, "app", spec)
		Expect(err).To(HaveOccurred())
		_, err = ns.CreateStack(cluster.StackOperationV1beta1, "app", `version: '3.2'
services:
  front:
    image: nginx:1.12.1-alpine`)
		Expect(err).NotTo(HaveOccurred())
		_, err = ns.UpdateStack(cluster.StackOperationV1beta1, "app", spec)
		Expect(err).To(HaveOccurred())
		_, err = ns.UpdateStack(cluster.StackOperationV1beta1, "app", `version: '3.2'
services:
  front:
    image: nginx:1.12.1-alpine`)
		Expect(err).NotTo(HaveOccurred())
	})
	It("Should update status of invalid stacks", func() {
		kcli := ns.StacksV1beta1()
		stack := &v1beta1.Stack{
			ObjectMeta: metav1.ObjectMeta{
				Name: "invalid-mount",
			},
			Spec: v1beta1.StackSpec{
				ComposeFile: `version: "3.3"
services:
   mounts:
      image: nginx
      volumes:
      - "./web/static:/static"`,
			},
		}
		_, err := kcli.WithSkipValidation().Create(stack)
		Expect(err).ToNot(HaveOccurred())
		waitUntil(ns.IsStackFailed("invalid-mount", "web/static: only absolute paths can be specified in mount source"))
	})
	It("Should update status stacks with invalid yaml", func() {
		kcli := ns.StacksV1beta1()
		stack := &v1beta1.Stack{
			ObjectMeta: metav1.ObjectMeta{
				Name: "invalid-yaml",
			},
			Spec: v1beta1.StackSpec{
				ComposeFile: `version: "3.3"
invalid-yaml"`,
			},
		}
		_, err := kcli.WithSkipValidation().Create(stack)
		Expect(err).ToNot(HaveOccurred())
		waitUntil(ns.IsStackFailed("invalid-yaml", "parsing error"))
	})

	It("Should remove owned configs and secrets", func() {
		_, err := ns.CreateStack(cluster.StackOperationV1beta2Stack, "app", `version: '3.3'
services:
  front:
    image: nginx:1.12.1-alpine
secrets:
  test-secret:
    file: ./secret-data
configs:
  test-config:
    file: ./config-data`)
		Expect(err).ToNot(HaveOccurred())
		_, err = ns.ConfigMaps().Create(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-config", Labels: map[string]string{labels.ForStackName: "app"}}})
		Expect(err).ToNot(HaveOccurred())
		_, err = ns.Secrets().Create(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "test-secret", Labels: map[string]string{labels.ForStackName: "app"}}})
		Expect(err).ToNot(HaveOccurred())
		waitUntil(ns.IsStackAvailable("app"))
		Expect(ns.DeleteStack("app")).ToNot(HaveOccurred())
		waitUntil(func() (done bool, err error) {
			_, err = ns.ConfigMaps().Get("test-config", metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		})
		waitUntil(func() (done bool, err error) {
			_, err = ns.Secrets().Get("test-secret", metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		})
	})

	It("Should deploy stacks with private images", func() {
		err := ns.CreatePullSecret("test-pull-secret", "https://index.docker.io/v1/", privateImagePullUsername, privateImagePullPassword)
		expectNoError(err)
		s := &latest.Stack{
			ObjectMeta: metav1.ObjectMeta{
				Name: "with-private-images",
			},
			Spec: &latest.StackSpec{
				Services: []latest.ServiceConfig{
					{
						Name:       "test-service",
						Image:      privateImagePullImage,
						PullSecret: "test-pull-secret",
					},
				},
			},
		}
		s, err = ns.StacksV1alpha3().Create(s)
		expectNoError(err)
		waitUntil(ns.IsStackAvailable(s.Name))
	})
	It("Should delete stacks with propagation=Foreground", func() {
		_, err := ns.CreateStack(cluster.StackOperationV1alpha3, "app", `version: '3.2'
services:
  front:
    image: nginx:1.12.1-alpine`)
		Expect(err).NotTo(HaveOccurred())
		waitUntil(ns.IsStackAvailable("app"))
		err = ns.DeleteStackWithPropagation("app", metav1.DeletePropagationForeground)
		Expect(err).NotTo(HaveOccurred())
		waitUntil(ns.ContainsZeroStack())
	})

	It("Should leverage cluster IP if InternalPorts are specified", func() {
		stack, err := ns.CreateStack(cluster.StackOperationV1alpha3, "app", `version: '3.2'
services:
  front:
    image: nginx:1.12.1-alpine`)
		Expect(err).NotTo(HaveOccurred())
		waitUntil(ns.IsStackAvailable("app"))
		svc, err := ns.Services().Get("front", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(svc.Spec.ClusterIP).To(Equal(corev1.ClusterIPNone))
		stack.Spec.Services[0].InternalPorts = []latest.InternalPort{
			{
				Port:     80,
				Protocol: corev1.ProtocolTCP,
			},
		}
		stack, err = ns.UpdateStackFromSpec("app", stack)
		Expect(err).NotTo(HaveOccurred())
		waitUntil(ns.IsStackAvailable("app"))
		svc, err = ns.Services().Get("front", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(svc.Spec.ClusterIP).NotTo(Equal(corev1.ClusterIPNone))
		stack.Spec.Services[0].InternalPorts = nil
		stack, err = ns.UpdateStackFromSpec("app", stack)
		Expect(err).NotTo(HaveOccurred())
		waitUntil(ns.IsStackAvailable("app"))
		svc, err = ns.Services().Get("front", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(svc.Spec.ClusterIP).To(Equal(corev1.ClusterIPNone))
	})

})

func drainUntil(stream chan string, match string) bool {
	to := time.After(30 * time.Second)
	for {
		select {
		case line := <-stream:
			if strings.Contains(line, match) {
				return true
			}
		case <-to:
			return false
		}
	}
}

// helpers

func createNamespace() (*cluster.Namespace, func()) {
	namespaceName := strings.ToLower(fmt.Sprintf("%s-%s-%d", deployNamespace, "compose", rand.Int63()))

	ns, cleanup, err := cluster.CreateNamespace(config, config, namespaceName)
	if apierrors.IsAlreadyExists(err) {
		// retry with another "random" namespace name
		return createNamespace()
	}
	expectNoError(err)

	return ns, cleanup
}

func expectNoError(err error) {
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}

func waitUntil(condition wait.ConditionFunc) {
	ExpectWithOffset(1, wait.PollImmediate(1*time.Second, 10*time.Minute, condition)).NotTo(HaveOccurred())
}

func stackServiceLabel(name string) string {
	return "com.docker.service.name=" + name
}
