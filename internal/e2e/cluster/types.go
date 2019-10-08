package cluster

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	composev1alpha3 "github.com/docker/compose-on-kubernetes/api/client/clientset/typed/compose/v1alpha3"
	composev1beta1 "github.com/docker/compose-on-kubernetes/api/client/clientset/typed/compose/v1beta1"
	composev1beta2 "github.com/docker/compose-on-kubernetes/api/client/clientset/typed/compose/v1beta2"
	"github.com/docker/compose-on-kubernetes/api/compose/v1alpha3"
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta1"
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta2"
	"github.com/docker/compose-on-kubernetes/internal/conversions"
	"github.com/docker/compose-on-kubernetes/internal/parsing"
	"github.com/docker/compose-on-kubernetes/internal/patch"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	storagetypes "k8s.io/api/storage/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	typesappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	storagev1 "k8s.io/client-go/kubernetes/typed/storage/v1"
	"k8s.io/client-go/rest"
)

// Namespace is a test dedicated namespace
type Namespace struct {
	name              string
	stackRESTv1beta2  rest.Interface
	stackRESTv1alpha3 rest.Interface
	stacks            composev1beta2.StackInterface
	stacksv1alpha3    composev1alpha3.StackInterface
	stacks1           composev1beta1.StackInterface
	pods              corev1.PodInterface
	deployments       typesappsv1.DeploymentInterface
	services          corev1.ServiceInterface
	nodes             corev1.NodeInterface
	servicesSupplier  func() *rest.Request
	storageClasses    storagev1.StorageClassInterface
	configMaps        corev1.ConfigMapInterface
	secrets           corev1.SecretInterface
	config            *rest.Config
}

// StackOperationStrategy is the strategy for a stack create/update
type StackOperationStrategy int

const (
	//StackOperationV1beta1 will use v1beta1 API
	StackOperationV1beta1 StackOperationStrategy = iota
	//StackOperationV1beta2Compose will use v1beta2 composefile subresource
	StackOperationV1beta2Compose
	//StackOperationV1beta2Stack will use v1beta2 structured stack
	StackOperationV1beta2Stack
	//StackOperationV1alpha3 will use a v1alpha3 structured stack
	StackOperationV1alpha3
)

// PodPredicate returns true when a predicate is verified on a pod and an optional message indicating why the predicate is false
type PodPredicate func(pod apiv1.Pod) (bool, string)

func newNamespace(config *rest.Config, namespace string) (*Namespace, error) {
	composeClientSet, err := composev1beta2.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	composeClientSet1, err := composev1beta1.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	composeClientSetv1alpha3, err := composev1alpha3.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	coreClientSet, err := corev1.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	storageClientSet, err := storagev1.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	appsClientSet, err := typesappsv1.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Namespace{
		name:              namespace,
		stackRESTv1beta2:  composeClientSet.RESTClient(),
		stackRESTv1alpha3: composeClientSetv1alpha3.RESTClient(),
		stacks:            composeClientSet.Stacks(namespace),
		stacks1:           composeClientSet1.Stacks(namespace),
		stacksv1alpha3:    composeClientSetv1alpha3.Stacks(namespace),
		pods:              coreClientSet.Pods(namespace),
		deployments:       appsClientSet.Deployments(namespace),
		services:          coreClientSet.Services(namespace),
		nodes:             coreClientSet.Nodes(),
		storageClasses:    storageClientSet.StorageClasses(),
		servicesSupplier: func() *rest.Request {
			return coreClientSet.RESTClient().Get().Resource("services").Namespace(namespace)
		},
		secrets:    coreClientSet.Secrets(namespace),
		configMaps: coreClientSet.ConfigMaps(namespace),
		config:     config,
	}, nil
}

// HasStorageClass returns true if cluster has at least one StorageClass defined
func (ns *Namespace) HasStorageClass() (bool, error) {
	storageClasses, err := ns.storageClasses.List(metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	sc := defaultStorageClass(storageClasses.Items)
	if sc == nil || sc.Provisioner == "kubernetes.io/host-path" {
		return false, nil
	}
	return true, nil
}

func defaultStorageClass(classes []storagetypes.StorageClass) *storagetypes.StorageClass {
	for _, c := range classes {
		if c.Annotations != nil && c.Annotations["storageclass.beta.kubernetes.io/is-default-class"] == "true" {
			return &c
		}
	}
	return nil
}

// RESTClientV1beta2 returns a RESTClient for the stacks
func (ns *Namespace) RESTClientV1beta2() rest.Interface {
	return ns.stackRESTv1beta2
}

// RESTClientV1alpha3 returns a RESTClient for the stacks
func (ns *Namespace) RESTClientV1alpha3() rest.Interface {
	return ns.stackRESTv1alpha3
}

// Name returns the name of the namespace.
func (ns *Namespace) Name() string {
	return ns.name
}

// Deployments returns a DeploymentInterface
func (ns *Namespace) Deployments() typesappsv1.DeploymentInterface {
	return ns.deployments
}

// Pods returns a PodInterface
func (ns *Namespace) Pods() corev1.PodInterface {
	return ns.pods
}

// StacksV1beta1 returns a v1beta1 client
func (ns *Namespace) StacksV1beta1() composev1beta1.StackInterface {
	return ns.stacks1
}

// StacksV1alpha3 returns a v1alpha3 client
func (ns *Namespace) StacksV1alpha3() composev1alpha3.StackInterface {
	return ns.stacksv1alpha3
}

// CreatePullSecret creates a pull secret
func (ns *Namespace) CreatePullSecret(name, server, username, password string) error {
	data, err := generatePullSecretData(server, username, password)
	if err != nil {
		return err
	}
	s := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns.name,
			Name:      name,
		},
		Type: apiv1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			apiv1.DockerConfigJsonKey: data,
		},
	}
	_, err = ns.Secrets().Create(s)
	return err
}

func generatePullSecretData(server, username, password string) ([]byte, error) {
	e := dockerConfigEntry{
		Username: username,
		Password: password,
		Auth:     base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password))),
	}
	cfg := dockerConfigJSON{
		Auths: map[string]dockerConfigEntry{
			server: e,
		},
	}
	return json.Marshal(&cfg)
}

type dockerConfigJSON struct {
	Auths map[string]dockerConfigEntry `json:"auths"`
}
type dockerConfigEntry struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Email    string `json:"email,omitempty"`
	Auth     string `json:"auth,omitempty"`
}

// CreateStack creates a stack.
func (ns *Namespace) CreateStack(strategy StackOperationStrategy, name, composeFile string) (*v1alpha3.Stack, error) {
	switch strategy {
	case StackOperationV1beta2Compose:
		compose := &v1beta2.ComposeFile{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			ComposeFile: composeFile,
		}
		return nil, ns.stackRESTv1beta2.Post().Namespace(ns.name).Name(name).Resource("stacks").SubResource("composefile").Body(compose).Do().Error()
	case StackOperationV1beta2Stack:
		var stack *v1beta2.Stack
		var err error
		config, err := parsing.LoadStackData([]byte(composeFile), map[string]string{})
		if err != nil {
			return nil, err
		}
		stack = &v1beta2.Stack{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: &v1beta2.StackSpec{},
		}
		spec := conversions.FromComposeConfig(config)
		if err := v1alpha3.Convert_v1alpha3_StackSpec_To_v1beta2_StackSpec(spec, stack.Spec, nil); err != nil {
			return nil, err
		}
		res, err := ns.stacks.Create(stack)
		if err != nil {
			return nil, err
		}
		var aslatest v1alpha3.Stack
		err = v1alpha3.Convert_v1beta2_Stack_To_v1alpha3_Stack(res, &aslatest, nil)
		return &aslatest, err
	case StackOperationV1alpha3:
		var stack *v1alpha3.Stack
		var err error
		config, err := parsing.LoadStackData([]byte(composeFile), map[string]string{})
		if err != nil {
			return nil, err
		}
		stack = &v1alpha3.Stack{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: conversions.FromComposeConfig(config),
		}
		return ns.stacksv1alpha3.Create(stack)
	case StackOperationV1beta1:
		stack := &v1beta1.Stack{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: v1beta1.StackSpec{
				ComposeFile: composeFile,
			},
		}
		_, err := ns.stacks1.Create(stack)
		return nil, err
	}
	return nil, nil
}

// DeleteStacks deletes all stacks.
func (ns *Namespace) DeleteStacks() error {
	return ns.stacks.DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{})
}

// DeleteStack deletes a stack.
func (ns *Namespace) DeleteStack(name string) error {
	return ns.stacks.Delete(name, &metav1.DeleteOptions{})
}

// DeleteStackWithPropagation deletes a stack using the specified propagation.
func (ns *Namespace) DeleteStackWithPropagation(name string, propagation metav1.DeletionPropagation) error {
	return ns.stacks.Delete(name, &metav1.DeleteOptions{PropagationPolicy: &propagation})
}

// DeleteStacksv1 deletes all stacks.
func (ns *Namespace) DeleteStacksv1() error {
	return ns.stacks1.DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{})
}

// DeleteStackv1 deletes a stack.
func (ns *Namespace) DeleteStackv1(name string) error {
	return ns.stacks1.Delete(name, &metav1.DeleteOptions{})
}

// UpdateStack updates a stack.
func (ns *Namespace) UpdateStack(strategy StackOperationStrategy, name, composeFile string) (*v1alpha3.Stack, error) {
	switch strategy {
	case StackOperationV1beta2Compose:
		compose := &v1beta2.ComposeFile{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			ComposeFile: composeFile,
		}
		return nil, ns.stackRESTv1beta2.Put().Namespace(ns.name).Name(name).Resource("stacks").SubResource("composefile").Body(compose).Do().Error()
	case StackOperationV1alpha3:
		p := patch.New()
		config, err := parsing.LoadStackData([]byte(composeFile), map[string]string{})
		if err != nil {
			return nil, err
		}
		newStack := &v1alpha3.Stack{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: conversions.FromComposeConfig(config),
		}
		if err != nil {
			return nil, err
		}
		p = p.Replace("/spec", newStack.Spec)

		buf, err := p.ToJSON()
		if err != nil {
			return nil, err
		}
		return ns.stacksv1alpha3.Patch(name, apitypes.JSONPatchType, buf)
	case StackOperationV1beta2Stack:
		p := patch.New()
		config, err := parsing.LoadStackData([]byte(composeFile), map[string]string{})
		if err != nil {
			return nil, err
		}
		newStack := &v1beta2.Stack{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: &v1beta2.StackSpec{},
		}
		spec := conversions.FromComposeConfig(config)
		if err := v1alpha3.Convert_v1alpha3_StackSpec_To_v1beta2_StackSpec(spec, newStack.Spec, nil); err != nil {
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		p = p.Replace("/spec", newStack.Spec)

		buf, err := p.ToJSON()
		if err != nil {
			return nil, err
		}
		res, err := ns.stacks.Patch(name, apitypes.JSONPatchType, buf)
		if err != nil {
			return nil, err
		}
		var aslatest v1alpha3.Stack
		err = v1alpha3.Convert_v1beta2_Stack_To_v1alpha3_Stack(res, &aslatest, nil)
		return &aslatest, err
	case StackOperationV1beta1:
		p := patch.New()
		p = p.Replace("/spec/composeFile", composeFile)
		buf, err := p.ToJSON()
		if err != nil {
			return nil, err
		}
		_, err = ns.stacks1.Patch(name, apitypes.JSONPatchType, buf)
		return nil, err
	}
	return nil, nil
}

// UpdateStackFromSpec updates a stack from a Spec.
func (ns *Namespace) UpdateStackFromSpec(name string, newStack *v1alpha3.Stack) (*v1alpha3.Stack, error) {
	filtered := &v1alpha3.Stack{
		Spec: newStack.Spec,
	}
	buf, err := json.Marshal(filtered)
	if err != nil {
		return nil, errors.Wrap(err, "stack marshaling error")
	}
	return ns.stacksv1alpha3.Patch(name, apitypes.MergePatchType, buf)
}

// GetStack gets a stack.
func (ns *Namespace) GetStack(name string) (*v1alpha3.Stack, error) {
	return ns.stacksv1alpha3.Get(name, metav1.GetOptions{})
}

// ListStacks lists the stacks.
func (ns *Namespace) ListStacks() ([]v1alpha3.Stack, error) {
	stacks, err := ns.stacksv1alpha3.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return stacks.Items, nil
}

// ContainsZeroStack is a poller that checks that no stack is created.
func (ns *Namespace) ContainsZeroStack() wait.ConditionFunc {
	return ns.ContainsNStacks(0)
}

// ContainsNStacks is a poller that checks how many stacks are created.
func (ns *Namespace) ContainsNStacks(count int) wait.ConditionFunc {
	return func() (bool, error) {
		stacks, err := ns.ListStacks()
		if err != nil {
			return false, err
		}

		if len(stacks) != count {
			return false, nil
		}

		return true, nil
	}
}

// ContainsZeroPod is a poller that checks that no pod is created.
func (ns *Namespace) ContainsZeroPod() wait.ConditionFunc {
	return ns.ContainsNPods(0)
}

// ContainsNPods is a poller that checks how many pods are created.
func (ns *Namespace) ContainsNPods(count int) wait.ConditionFunc {
	return ns.ContainsNPodsMatchingSelector(count, "")
}

// PodIsActuallyRemoved is a poller that checks that a pod has been terminated
func (ns *Namespace) PodIsActuallyRemoved(name string) wait.ConditionFunc {
	return func() (bool, error) {
		_, err := ns.pods.Get(name, metav1.GetOptions{})
		if kerrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}
}

// ContainsNPodsMatchingSelector is a poller that checks how many pods are created for given label selector.
func (ns *Namespace) ContainsNPodsMatchingSelector(count int, labelSelector string) wait.ConditionFunc {
	return func() (bool, error) {
		pods, err := ns.ListPods(labelSelector)
		if err != nil {
			return false, err
		}

		if len(pods) != count {
			return false, nil
		}

		return true, nil
	}
}

// ContainsNPodsWithPredicate is a poller that checks how many pods matching the predicate are created.
func (ns *Namespace) ContainsNPodsWithPredicate(count int, labelSelector string, predicate PodPredicate) wait.ConditionFunc {
	return func() (bool, error) {
		pods, err := ns.ListPods(labelSelector)
		if err != nil {
			return false, err
		}

		if len(pods) != count {
			return false, nil
		}

		for _, pod := range pods {
			if ok, _ := predicate(pod); !ok {
				return false, nil
			}
		}

		return true, nil
	}
}

// IsStackAvailable is a poller that checks is a given stack is available.
func (ns *Namespace) IsStackAvailable(name string) wait.ConditionFunc {
	return func() (bool, error) {
		stack, err := ns.GetStack(name)
		if err != nil {
			return false, err
		}

		if stack.Status == nil || stack.Status.Phase != v1alpha3.StackAvailable {
			return false, nil
		}

		return true, nil
	}
}

// IsStackFailed is a poller that checks if a given stack has failed with the correct error.
func (ns *Namespace) IsStackFailed(name string, errorSubstr string) wait.ConditionFunc {
	return func() (bool, error) {
		stack, err := ns.GetStack(name)
		if err != nil {
			return false, err
		}

		if stack.Status == nil || stack.Status.Phase != v1alpha3.StackFailure {
			return false, nil
		}

		if !strings.Contains(stack.Status.Message, errorSubstr) {
			return false, fmt.Errorf("status message is %q. expected to contain %q", stack.Status.Message, errorSubstr)
		}

		return true, nil
	}
}

// IsServicePresent is a poller that checks if a service is present.
func (ns *Namespace) IsServicePresent(labelSelector string) wait.ConditionFunc {
	return func() (bool, error) {
		services, err := ns.services.List(metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return false, err
		}

		if len(services.Items) == 0 {
			return false, nil
		}

		return true, nil
	}
}

// ServiceCount is a poller that checks a number of services to be present.
func (ns *Namespace) ServiceCount(labelSelector string, count int) wait.ConditionFunc {
	return func() (bool, error) {
		services, err := ns.services.List(metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return false, err
		}

		if len(services.Items) != count {
			return false, nil
		}

		return true, nil
	}
}

// IsServiceNotPresent is a poller that checks if a service is not present.
func (ns *Namespace) IsServiceNotPresent(labelSelector string) wait.ConditionFunc {
	return func() (bool, error) {
		services, err := ns.services.List(metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return false, err
		}

		if len(services.Items) > 0 {
			return false, nil
		}

		return true, nil
	}
}

// IsServiceResponding is a poller that checks is responding with the expected
// content text.
func (ns *Namespace) IsServiceResponding(service string, url string, expectedText string) wait.ConditionFunc {
	return func() (bool, error) {
		resp, err := ns.servicesSupplier().
			Name(service).
			SubResource(strings.Split(url, "/")...).
			DoRaw()
		if err != nil {
			return false, nil
		}

		if !strings.Contains(string(resp), expectedText) {
			return false, nil
		}

		return true, nil
	}
}

// ListPods lists the pods that match a given selector.
func (ns *Namespace) ListPods(labelSelector string) ([]apiv1.Pod, error) {
	pods, err := ns.pods.List(metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	return pods.Items, nil
}

// ListAllPods lists all pods in the namespace.
func (ns *Namespace) ListAllPods() ([]apiv1.Pod, error) {
	return ns.ListPods("")
}

// ListDeployments lists the deployments that match a given selector.
func (ns *Namespace) ListDeployments(labelSelector string) ([]appsv1.Deployment, error) {
	deployments, err := ns.deployments.List(metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	return deployments.Items, nil
}

// ListServices lists the services that match a given selector.
func (ns *Namespace) ListServices(labelSelector string) ([]apiv1.Service, error) {
	services, err := ns.services.List(metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	return services.Items, nil
}

// ListNodes lists the nodes available in the cluster.
func (ns *Namespace) ListNodes() ([]apiv1.Node, error) {
	nodes, err := ns.nodes.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return nodes.Items, nil
}

// ConfigMaps returns a ConfigMaps client for the namespace
func (ns *Namespace) ConfigMaps() corev1.ConfigMapInterface {
	return ns.configMaps
}

// Secrets returns a Secrets client for the namespace
func (ns *Namespace) Secrets() corev1.SecretInterface {
	return ns.secrets
}

// Services returns a Services client for the namespace
func (ns *Namespace) Services() corev1.ServiceInterface {
	return ns.services
}

// As returns the same namespace with an impersonated config
func (ns *Namespace) As(user rest.ImpersonationConfig) (*Namespace, error) {
	cfg := *ns.config
	cfg.Impersonate = user
	return newNamespace(&cfg, ns.name)
}
