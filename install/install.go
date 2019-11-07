package install

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	kubeapiclientset "github.com/docker/compose-on-kubernetes/api/client/clientset"
	apiv1beta2 "github.com/docker/compose-on-kubernetes/api/compose/v1beta2"
	"github.com/docker/compose-on-kubernetes/internal/e2e/wait"
	log "github.com/sirupsen/logrus"
	appsv1types "k8s.io/api/apps/v1"
	corev1types "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1types "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

const (
	// TimeoutDefault is the default install timeout.
	TimeoutDefault = 30 * time.Second

	// installWaitNumMaxPolls is the maximum number of API operations to be
	// performed in sequence while waiting for the component to be installed.
	installWaitNumMaxPolls = 60

	fryKey                      = "com.docker.fry"
	imageTagKey                 = "com.docker.image-tag"
	namespaceKey                = "com.docker.deploy-namespace"
	defaultServiceTypeKey       = "com.docker.default-service-type"
	customTLSHashAnnotationName = "com.docker.custom-tls-hash"
	composeFry                  = "compose"
	composeAPIServerFry         = "compose.api"
	composeGroupName            = "compose.docker.com"

	controllerDebugPort = 40000
	apiServerDebugPort  = 40001
)

var (
	imageRepoPrefix = "docker/kube-compose-"
	imagePrefix     = func() string {
		if ir := os.Getenv("IMAGE_REPO_PREFIX"); ir != "" {
			return ir
		}
		return imageRepoPrefix
	}()
	everythingSelector = fmt.Sprintf("%s in (%s, %s)", fryKey, composeFry, composeAPIServerFry)
)

var linuxAmd64NodeAffinity = &corev1types.Affinity{
	NodeAffinity: &corev1types.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1types.NodeSelector{
			NodeSelectorTerms: []corev1types.NodeSelectorTerm{
				{
					MatchExpressions: []corev1types.NodeSelectorRequirement{
						{
							Key:      "beta.kubernetes.io/os",
							Operator: corev1types.NodeSelectorOpIn,
							Values:   []string{"linux"},
						},
						{
							Key:      "beta.kubernetes.io/arch",
							Operator: corev1types.NodeSelectorOpIn,
							Values:   []string{"amd64"},
						},
					},
				},
			},
		},
	},
}

// GetInstallStatus retrives the current installation status
func GetInstallStatus(config *rest.Config) (Status, error) {
	installer, err := newInstaller(config)
	if err != nil {
		return Status{}, err
	}
	return installer.isInstalled()
}

// Unsafe installs the Compose features without High availability, and with insecure ETCD.
func Unsafe(ctx context.Context, config *rest.Config, options UnsafeOptions) error {
	return Do(ctx, config, WithUnsafe(options))
}

// WaitNPods waits for n pods to be up
func WaitNPods(config *rest.Config, namespace string, count int, timeout time.Duration) error {
	log.Infof("Wait for %d pod(s) to be up with timeout %s", count, timeout)
	client, err := corev1.NewForConfig(config)
	if err != nil {
		return err
	}

	period := 2 * time.Second
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(period) {
		log.Debugf("Check pod(s) are running...")
		pods, err := client.Pods(namespace).List(metav1.ListOptions{
			LabelSelector: everythingSelector,
		})
		if err != nil {
			return err
		}

		if len(pods.Items) != count {
			log.Debugf("Pod(s) not yet created, waiting %s", period)
			continue
		}

		running, err := allRunning(pods.Items)
		if err != nil {
			return err
		}

		if running {
			return nil
		}
		log.Debugf("Pod(s) not running, waiting %s", period)
	}

	return errors.New("installation timed out")
}

func checkPodContainers(pod corev1types.Pod) error {
	for _, status := range pod.Status.ContainerStatuses {
		waiting := status.State.Waiting
		if waiting != nil {
			if IsErrImagePull(waiting.Reason) {
				return errors.New(waiting.Message)
			}
		}
	}
	return nil
}

func allRunning(pods []corev1types.Pod) (bool, error) {
	for _, pod := range pods {
		switch pod.Status.Phase {
		case corev1types.PodRunning:
		case corev1types.PodPending:
			return false, checkPodContainers(pod)
		case corev1types.PodFailed:
			return false, errors.New("unable to start controller: " + pod.Status.Message)
		default:
			return false, nil
		}
	}
	return true, nil
}

// IsRunning checks if the compose api server is available
func IsRunning(config *rest.Config) (bool, error) {
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return false, err
	}

	groups, err := client.Discovery().ServerGroups()
	if err != nil {
		return false, err
	}

	for _, group := range groups.Groups {
		if group.Name == apiv1beta2.SchemeGroupVersion.Group {
			stackClient, err := kubeapiclientset.NewForConfig(config)
			if err != nil {
				return false, err
			}
			err = wait.For(installWaitNumMaxPolls, func() (bool, error) {
				_, err := stackClient.ComposeV1beta2().Stacks("e2e").List(metav1.ListOptions{})
				if err != nil {
					return false, nil
				}
				_, err = stackClient.ComposeV1beta1().Stacks("e2e").List(metav1.ListOptions{})
				if err != nil {
					return false, nil
				}
				_, err = stackClient.ComposeV1alpha3().Stacks("e2e").List(metav1.ListOptions{})
				return err == nil, nil
			})
			return err == nil, err
		}
	}
	return false, nil
}

func (c *installer) createNamespace(*installerContext) error {
	log.Debugf("Create namespace: %s", c.commonOptions.Namespace)

	if _, err := c.coreClient.Namespaces().Get(c.commonOptions.Namespace, metav1.GetOptions{}); err == nil {
		return nil
	}
	ns := &corev1types.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.commonOptions.Namespace,
		},
	}
	shouldDo, err := c.objectFilter.filter(ns)
	if err != nil {
		return err
	}
	if shouldDo {
		_, err := c.coreClient.Namespaces().Create(ns)
		return err
	}
	return nil
}

func (c *installer) createPullSecretIfRequired(ctx *installerContext) error {
	if c.commonOptions.PullSecret == "" {
		return nil
	}
	log.Debug("Create pull secret")
	secret, err := c.coreClient.Secrets(c.commonOptions.Namespace).Get("compose", metav1.GetOptions{})
	if err == nil {
		ctx.pullSecret = secret
		return nil
	}

	bin, err := base64.StdEncoding.DecodeString(c.commonOptions.PullSecret)
	if err != nil {
		return err
	}

	secret = &corev1types.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "compose",
			Namespace: c.commonOptions.Namespace,
			Labels:    c.labels,
		},
		Data: map[string][]byte{
			".dockercfg": bin,
		},
		Type: corev1types.SecretTypeDockercfg,
	}
	shouldDo, err := c.objectFilter.filter(secret)
	if err != nil {
		return err
	}
	if shouldDo {
		secret, err = c.coreClient.Secrets(c.commonOptions.Namespace).Create(secret)
	}
	ctx.pullSecret = secret
	return err
}

func (c *installer) createServiceAccount(ctx *installerContext) error {
	log.Debug("Create ServiceAccount")
	sa, err := c.coreClient.ServiceAccounts(c.commonOptions.Namespace).Get("compose", metav1.GetOptions{})
	if err == nil {
		ctx.serviceAccount = sa
		return nil
	}
	sa = &corev1types.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "compose",
			Namespace: c.commonOptions.Namespace,
			Labels:    c.labels,
		},
	}
	shouldDo, err := c.objectFilter.filter(sa)
	if err != nil {
		return err
	}
	if shouldDo {
		sa, err = c.coreClient.ServiceAccounts(c.commonOptions.Namespace).Create(sa)
	}
	ctx.serviceAccount = sa
	return err
}

var composeRoleRules = []rbacv1types.PolicyRule{
	{
		APIGroups: []string{""},
		Resources: []string{"users", "groups", "serviceaccounts"},
		Verbs:     []string{"impersonate"},
	},
	{
		APIGroups: []string{"authentication.k8s.io"},
		Resources: []string{"*"},
		Verbs:     []string{"impersonate"},
	},
	{
		APIGroups: []string{"", "apps"},
		Resources: []string{"services", "deployments", "statefulsets", "daemonsets"},
		Verbs:     []string{"get"},
	},
	{
		APIGroups: []string{""},
		Resources: []string{"pods", "pods/log"},
		Verbs:     []string{"get", "watch", "list"},
	},
	{
		APIGroups: []string{composeGroupName},
		Resources: []string{"stacks"},
		Verbs:     []string{"*"},
	},
	{
		APIGroups: []string{composeGroupName},
		Resources: []string{"stacks/owner"},
		Verbs:     []string{"get"},
	},
	{
		APIGroups: []string{"admissionregistration.k8s.io"},
		Resources: []string{"validatingwebhookconfigurations", "mutatingwebhookconfigurations"},
		Verbs:     []string{"get", "watch", "list"},
	},
	{
		APIGroups:     []string{"apiregistration.k8s.io"},
		Resources:     []string{"apiservices"},
		ResourceNames: []string{"v1beta1.compose.docker.com", "v1beta2.compose.docker.com", "v1alpha3.compose.docker.com"},
		Verbs:         []string{"*"},
	},
	{
		APIGroups: []string{"apiregistration.k8s.io"},
		Resources: []string{"apiservices"},
		Verbs:     []string{"create"},
	},
}

func viewStackRole() *rbacv1types.ClusterRole {
	return &rbacv1types.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "compose-stack-view",
			Labels: map[string]string{
				"rbac.authorization.k8s.io/aggregate-to-view": "true",
			},
		},
		Rules: []rbacv1types.PolicyRule{
			{
				APIGroups: []string{composeGroupName},
				Resources: []string{"stacks", "stacks/scale", "stacks/log", "stacks/composeFile"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
}

func editStackRole() *rbacv1types.ClusterRole {
	return &rbacv1types.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "compose-stack-edit",
			Labels: map[string]string{
				"rbac.authorization.k8s.io/aggregate-to-edit": "true",
			},
		},
		Rules: []rbacv1types.PolicyRule{
			{
				APIGroups: []string{composeGroupName},
				Resources: []string{"stacks", "stacks/scale", "stacks/log", "stacks/composeFile"},
				Verbs: []string{
					"create",
					"delete",
					"deletecollection",
					"get",
					"list",
					"patch",
					"update",
					"watch",
				},
			},
		},
	}
}

func adminStackRole() *rbacv1types.ClusterRole {
	return &rbacv1types.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "compose-stack-admin",
			Labels: map[string]string{
				"rbac.authorization.k8s.io/aggregate-to-admin": "true",
			},
		},
		Rules: []rbacv1types.PolicyRule{
			{
				APIGroups: []string{composeGroupName},
				Resources: []string{"stacks", "stacks/scale", "stacks/log", "stacks/composeFile"},
				Verbs: []string{
					"create",
					"delete",
					"deletecollection",
					"get",
					"list",
					"patch",
					"update",
					"watch",
				},
			},
			{
				APIGroups: []string{composeGroupName},
				Resources: []string{"stacks/owner"},
				Verbs:     []string{"get"},
			},
		},
	}
}

func (c *installer) createDefaultClusterRoles(_ *installerContext) error {
	var shouldDo bool
	roles := []*rbacv1types.ClusterRole{viewStackRole(), editStackRole(), adminStackRole()}
	for _, r := range roles {
		existing, err := c.rbacClient.ClusterRoles().Get(r.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			if shouldDo, err = c.objectFilter.filter(r); err != nil {
				return err
			}
			if shouldDo {
				if _, err := c.rbacClient.ClusterRoles().Create(r); err != nil {
					return err
				}
			}
		} else if err != nil {
			return err
		} else {
			r.ResourceVersion = existing.ResourceVersion
			if shouldDo, err = c.objectFilter.filter(r); err != nil {
				return err
			}
			if shouldDo {
				if _, err := c.rbacClient.ClusterRoles().Update(r); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *installer) createSAClusterRole() error {
	role, err := c.rbacClient.ClusterRoles().Get("compose-service", metav1.GetOptions{})
	var shouldDo bool
	if apierrors.IsNotFound(err) {
		role = &rbacv1types.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "compose-service",
				Labels: c.labels,
			},
			Rules: composeRoleRules,
		}
		if shouldDo, err = c.objectFilter.filter(role); err != nil {
			return err
		}
		if shouldDo {
			role, err = c.rbacClient.ClusterRoles().Create(role)
		}
	} else if err == nil {
		role.Rules = composeRoleRules
		if shouldDo, err = c.objectFilter.filter(role); err != nil {
			return err
		}
		if shouldDo {
			role, err = c.rbacClient.ClusterRoles().Update(role)
		}
	}
	return err
}

type roleBindingRequirement struct {
	name      string
	namespace string
	roleRef   rbacv1types.RoleRef
}

var requiredRoleBindings = []roleBindingRequirement{
	{
		name:      "compose-auth-reader",
		namespace: "kube-system",
		roleRef: rbacv1types.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "extension-apiserver-authentication-reader",
		},
	},
}
var requiredClusteRoleBindings = []roleBindingRequirement{
	{
		name: "compose-auth-delegator",
		roleRef: rbacv1types.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "system:auth-delegator",
		},
	},
	{
		name: "compose-auth-view",
		roleRef: rbacv1types.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "view",
		},
	},
	{
		name: "compose",
		roleRef: rbacv1types.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "compose-service",
		},
	},
}

func (c *installer) createSARoleBindings(ctx *installerContext) error {
	subjects := []rbacv1types.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      ctx.serviceAccount.Name,
			Namespace: ctx.serviceAccount.Namespace,
		},
	}
	var shouldDo bool
	for _, req := range requiredRoleBindings {
		shouldCreate := false
		rb, err := c.rbacClient.RoleBindings(req.namespace).Get(req.name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			shouldCreate = true
			rb = &rbacv1types.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      req.name,
					Labels:    c.labels,
					Namespace: req.namespace,
				},
				RoleRef:  req.roleRef,
				Subjects: subjects,
			}
		} else if err == nil {
			rb.RoleRef = req.roleRef
			rb.Subjects = subjects
		}
		if shouldDo, err = c.objectFilter.filter(rb); err != nil {
			return err
		}
		if shouldDo {
			if shouldCreate {
				_, err = c.rbacClient.RoleBindings(req.namespace).Create(rb)
			} else {
				_, err = c.rbacClient.RoleBindings(req.namespace).Update(rb)
			}
		}
		if err != nil {
			return err
		}
	}
	for _, req := range requiredClusteRoleBindings {
		shouldCreate := false
		crb, err := c.rbacClient.ClusterRoleBindings().Get(req.name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			shouldCreate = true
			crb = &rbacv1types.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:   req.name,
					Labels: c.labels,
				},
				RoleRef:  req.roleRef,
				Subjects: subjects,
			}
		} else if err == nil {
			crb.RoleRef = req.roleRef
			crb.Subjects = subjects
		}
		if shouldDo, err = c.objectFilter.filter(crb); err != nil {
			return err
		}
		if shouldDo {
			if shouldCreate {
				_, err = c.rbacClient.ClusterRoleBindings().Create(crb)
			} else {
				_, err = c.rbacClient.ClusterRoleBindings().Update(crb)
			}
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *installer) createClusterRoleBindings(ctx *installerContext) error {
	log.Debug("Create stack cluster role bindings")
	if err := c.createSAClusterRole(); err != nil {
		return err
	}

	log.Debug("Create auth RoleBindings")

	return c.createSARoleBindings(ctx)
}

func applyCustomTLSHash(hash string, deploy *appsv1types.Deployment) {
	if hash == "" {
		return
	}
	if deploy.Annotations == nil {
		deploy.Annotations = make(map[string]string)
	}
	if deploy.Spec.Template.Annotations == nil {
		deploy.Spec.Template.Annotations = make(map[string]string)
	}
	deploy.Annotations[customTLSHashAnnotationName] = hash
	deploy.Spec.Template.Annotations[customTLSHashAnnotationName] = hash
}

func (c *installer) configureAPIServerImage() (string, []string, []corev1types.EnvVar, corev1types.PullPolicy) {
	if c.enableCoverage {
		return imagePrefix + "api-server-coverage" + ":" + c.commonOptions.Tag,
			[]string{},
			[]corev1types.EnvVar{{Name: "TEST_API_SERVER", Value: "1"}},
			corev1types.PullNever
	}
	args := []string{
		"--kubeconfig", "",
		"--authentication-kubeconfig=",
		"--authorization-kubeconfig=",
		"--service-name=compose-api",
		"--service-namespace", c.commonOptions.Namespace,
		"--healthz-check-port", strconv.Itoa(c.commonOptions.HealthzCheckPort),
	}
	if c.debugImages {
		return imagePrefix + "api-server-debug:latest",
			args,
			[]corev1types.EnvVar{},
			corev1types.PullNever
	}
	return imagePrefix + "api-server" + ":" + c.commonOptions.Tag,
		args,
		[]corev1types.EnvVar{},
		c.commonOptions.PullPolicy
}

func (c *installer) validateOptions() error {
	if c.etcdOptions == nil && c.commonOptions.APIServerReplicas != nil && *c.commonOptions.APIServerReplicas != 1 {
		// etcdOptions == nil makes the installer run etcd as a sidecar container of the APIServer
		// thus, the user cannot scale it
		return errors.New("can't specify the API server replicas without referencing an external etcd instance")
	}
	return nil
}

func (c *installer) createAPIServer(ctx *installerContext) error {
	log.Debugf("Create API server deployment and service in namespace %q", c.commonOptions.Namespace)
	image, args, env, pullPolicy := c.configureAPIServerImage()
	if c.apiServerImageOverride != "" {
		image = c.apiServerImageOverride
	}

	affinity := c.commonOptions.APIServerAffinity
	if affinity == nil {
		affinity = linuxAmd64NodeAffinity
	}

	log.Infof("Api server: image: %q, pullPolicy: %q", image, pullPolicy)

	deploy := &appsv1types.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "compose-api",
			Namespace: c.commonOptions.Namespace,
			Labels:    c.apiLabels,
		},
		Spec: appsv1types.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: c.apiLabels,
			},
			Template: corev1types.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: c.apiLabels,
				},
				Spec: corev1types.PodSpec{
					ServiceAccountName: ctx.serviceAccount.Name,
					ImagePullSecrets:   pullSecrets(ctx.pullSecret),
					Containers: []corev1types.Container{
						{
							Name:            "compose",
							Image:           image,
							ImagePullPolicy: pullPolicy,
							Args:            args,
							Env:             env,
							LivenessProbe: &corev1types.Probe{
								Handler: corev1types.Handler{
									HTTPGet: &corev1types.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromInt(c.commonOptions.HealthzCheckPort),
										Scheme: corev1types.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 15,
								TimeoutSeconds:      15,
								FailureThreshold:    8,
							},
						},
					},
					Affinity: affinity,
				},
			},
			Replicas: c.commonOptions.APIServerReplicas,
		},
	}
	if c.commonOptions.HealthzCheckPort == 0 {
		deploy.Spec.Template.Spec.Containers[0].LivenessProbe = nil
	}

	applyEtcdOptions(&deploy.Spec.Template.Spec, c.etcdOptions)
	applyNetworkOptions(&deploy.Spec.Template.Spec, c.networkOptions)
	port := 9443
	if c.networkOptions != nil && c.networkOptions.Port != 0 {
		port = int(c.networkOptions.Port)
	}

	applyCustomTLSHash(c.customTLSHash, deploy)

	shouldDo, err := c.objectFilter.filter(deploy)
	if err != nil {
		return err
	}
	if shouldDo {
		if c.debugImages {
			trueval := true
			for ix := range deploy.Spec.Template.Spec.Containers {
				deploy.Spec.Template.Spec.Containers[ix].SecurityContext = &corev1types.SecurityContext{
					Privileged: &trueval,
				}
				deploy.Spec.Template.Spec.Containers[ix].LivenessProbe = nil
			}
		}
		d, err := c.appsClient.Deployments(c.commonOptions.Namespace).Get("compose-api", metav1.GetOptions{})
		if err == nil {
			deploy.ObjectMeta.ResourceVersion = d.ObjectMeta.ResourceVersion
			_, err = c.appsClient.Deployments(c.commonOptions.Namespace).Update(deploy)
		} else {
			_, err = c.appsClient.Deployments(c.commonOptions.Namespace).Create(deploy)
		}
		if err != nil {
			return err
		}
	}

	if err = c.createAPIServerService(port); err != nil {
		return err
	}
	if c.debugImages {
		// create a load balanced service for exposing remote debug endpoint
		return c.createDebugService("compose-api-server-remote-debug", apiServerDebugPort, c.apiLabels)
	}
	return nil
}

func (c *installer) createAPIServerService(port int) error {
	svc := &corev1types.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "compose-api",
			Namespace: c.commonOptions.Namespace,
			Labels:    c.apiLabels,
		},
		Spec: corev1types.ServiceSpec{
			Ports: []corev1types.ServicePort{
				{
					Name:       "api",
					Port:       443,
					TargetPort: intstr.FromInt(port),
				},
			},
			Selector: c.apiLabels,
		},
	}
	shouldDo, err := c.objectFilter.filter(svc)
	if err != nil {
		return err
	}
	if shouldDo {
		s, err := c.coreClient.Services(c.commonOptions.Namespace).Get("compose-api", metav1.GetOptions{})
		if err == nil {
			svc.Spec.ClusterIP = s.Spec.ClusterIP
			svc.ObjectMeta.ResourceVersion = s.ObjectMeta.ResourceVersion
			_, err = c.coreClient.Services(c.commonOptions.Namespace).Update(svc)
		} else {
			_, err = c.coreClient.Services(c.commonOptions.Namespace).Create(svc)
		}
		return err
	}
	return nil
}

func pullSecrets(secret *corev1types.Secret) []corev1types.LocalObjectReference {
	if secret == nil {
		return nil
	}
	return []corev1types.LocalObjectReference{{Name: secret.Name}}
}

func (c *installer) configureControllerImage() (string, []string, v1.PullPolicy) {
	if c.enableCoverage {
		return imagePrefix + "controller-coverage" + ":" + c.commonOptions.Tag, []string{}, corev1types.PullNever
	}
	args := []string{
		"--kubeconfig", "",
		"--reconciliation-interval", c.commonOptions.ReconciliationInterval.String(),
		"--healthz-check-port", strconv.Itoa(c.commonOptions.HealthzCheckPort),
	}
	if c.debugImages {
		return imagePrefix + "controller-debug:latest", args, corev1types.PullNever
	}
	return imagePrefix + "controller:" + c.commonOptions.Tag, args, c.commonOptions.PullPolicy
}

func (c *installer) createController(ctx *installerContext) error {
	log.Debugf("Create deployment with tag %q in namespace %q, reconciliation interval %s", c.commonOptions.Tag, c.commonOptions.Namespace, c.commonOptions.ReconciliationInterval)

	image, args, pullPolicy := c.configureControllerImage()

	if c.commonOptions.DefaultServiceType != "" {
		args = append(args, "--default-service-type="+c.commonOptions.DefaultServiceType)
	}

	if c.controllerImageOverride != "" {
		image = c.controllerImageOverride
	}
	affinity := c.commonOptions.ControllerAffinity
	if affinity == nil {
		affinity = linuxAmd64NodeAffinity
	}
	log.Infof("Controller: image: %q, pullPolicy: %q", image, pullPolicy)
	deploy := &appsv1types.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "compose",
			Namespace: c.commonOptions.Namespace,
			Labels:    c.labels,
		},
		Spec: appsv1types.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: c.labels,
			},
			Template: corev1types.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: c.labels,
				},
				Spec: corev1types.PodSpec{
					ServiceAccountName: ctx.serviceAccount.Name,
					ImagePullSecrets:   pullSecrets(ctx.pullSecret),
					Containers: []corev1types.Container{
						{
							Name:            "compose",
							Image:           image,
							ImagePullPolicy: pullPolicy,
							Args:            args,
							LivenessProbe: &corev1types.Probe{
								Handler: corev1types.Handler{
									HTTPGet: &corev1types.HTTPGetAction{
										Path:   "/healthz",
										Scheme: corev1types.URISchemeHTTP,
										Port:   intstr.FromInt(c.commonOptions.HealthzCheckPort),
									},
								},
								InitialDelaySeconds: 15,
								TimeoutSeconds:      15,
								FailureThreshold:    8,
							},
						},
					},
					Affinity: affinity,
				},
			},
		},
	}
	if c.enableCoverage {
		envList := []corev1types.EnvVar{{Name: "TEST_COMPOSE_CONTROLLER", Value: "1"}}
		if c.commonOptions.HealthzCheckPort > 0 {
			envList = append(envList, corev1types.EnvVar{Name: "TEST_COMPOSE_HEALTHZ_PORT", Value: strconv.Itoa(c.commonOptions.HealthzCheckPort)})
		}
		deploy.Spec.Template.Spec.Containers[0].Env = envList
	}

	if c.commonOptions.HealthzCheckPort == 0 {
		deploy.Spec.Template.Spec.Containers[0].LivenessProbe = nil
	}

	shouldDo, err := c.objectFilter.filter(deploy)
	if err != nil {
		return err
	}
	if shouldDo {
		if c.debugImages {
			trueval := true
			for ix := range deploy.Spec.Template.Spec.Containers {
				deploy.Spec.Template.Spec.Containers[ix].SecurityContext = &corev1types.SecurityContext{
					Privileged: &trueval,
				}
				deploy.Spec.Template.Spec.Containers[ix].LivenessProbe = nil
			}
		}
		d, err := c.appsClient.Deployments(c.commonOptions.Namespace).Get("compose", metav1.GetOptions{})
		if err == nil {
			deploy.ObjectMeta.ResourceVersion = d.ObjectMeta.ResourceVersion
			if _, err = c.appsClient.Deployments(c.commonOptions.Namespace).Update(deploy); err != nil {
				return err
			}
		} else if _, err = c.appsClient.Deployments(c.commonOptions.Namespace).Create(deploy); err != nil {
			return err
		}
	}
	if c.debugImages {
		// create a load balanced service for exposing remote debug endpoint
		return c.createDebugService("compose-controller-remote-debug", controllerDebugPort, c.labels)
	}
	return nil
}

func (c *installer) createDebugService(name string, port int32, labels map[string]string) error {
	svc, err := c.coreClient.Services(c.commonOptions.Namespace).Get(name, metav1.GetOptions{})
	if err == nil {
		svc.Spec.Type = corev1types.ServiceTypeLoadBalancer
		svc.Spec.Ports = []corev1types.ServicePort{
			{Name: "delve", Port: port, TargetPort: intstr.FromInt(40000)},
		}
		svc.Spec.Selector = labels
		_, err = c.coreClient.Services(c.commonOptions.Namespace).Update(svc)
		return err
	}
	_, err = c.coreClient.Services(c.commonOptions.Namespace).Create(&corev1types.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: corev1types.ServiceSpec{
			Type:     corev1types.ServiceTypeLoadBalancer,
			Selector: labels,
			Ports: []corev1types.ServicePort{
				{Name: "delve", Port: port, TargetPort: intstr.FromInt(40000)},
			},
		},
	})
	return err
}
