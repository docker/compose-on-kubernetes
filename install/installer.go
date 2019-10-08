package install

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	corev1types "k8s.io/api/core/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/rest"
	kubeaggreagatorv1beta1 "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1beta1"
)

type installer struct {
	coreClient              corev1.CoreV1Interface
	rbacClient              rbacv1.RbacV1Interface
	appsClient              appsv1.AppsV1Interface
	aggregatorClient        kubeaggreagatorv1beta1.ApiregistrationV1beta1Interface
	commonOptions           OptionsCommon
	etcdOptions             *EtcdOptions
	networkOptions          *NetworkOptions
	enableCoverage          bool
	config                  *rest.Config
	labels                  map[string]string
	apiLabels               map[string]string
	disableController       bool
	controllerOnly          bool
	controllerImageOverride string
	apiServerImageOverride  string
	objectFilter            runtimeObjectFilters
	customMatch             func(Status) bool
	expiresOffset           time.Duration
	customTLSHash           string
	debugImages             bool
}

// RuntimeObjectFilter allows to modify or bypass completely a k8s object
type RuntimeObjectFilter func(runtime.Object) (bool, error)

type runtimeObjectFilters []RuntimeObjectFilter

func (fs runtimeObjectFilters) filter(obj runtime.Object) (bool, error) {
	for _, f := range fs {
		if res, err := f(obj); err != nil || !res {
			return res, err
		}
	}
	return true, nil
}

type installerContext struct {
	pullSecret     *corev1types.Secret
	serviceAccount *corev1types.ServiceAccount
}

// Do proceeds with installing
func Do(ctx context.Context, config *rest.Config, options ...InstallerOption) error {
	installer, err := newInstaller(config, options...)
	if err != nil {
		return err
	}
	return installer.install(ctx)
}

// InstallerOption defines modifies the installer
type InstallerOption func(*installer)

// WithObjectFilter applies a RuntimeObjectFilter
func WithObjectFilter(filter RuntimeObjectFilter) InstallerOption {
	return func(i *installer) {
		i.objectFilter = append(i.objectFilter, filter)
	}
}

// WithExpiresOffset specifies the duration offset to apply when checking if generated tls bundle has expired
func WithExpiresOffset(d time.Duration) InstallerOption {
	return func(i *installer) {
		i.expiresOffset = d
	}
}

// WithoutController install components without the controller
func WithoutController() InstallerOption {
	return func(i *installer) {
		i.disableController = true
	}
}

// WithControllerImage overrides controller image selection
func WithControllerImage(image string) InstallerOption {
	return func(i *installer) {
		i.controllerImageOverride = image
	}
}

// WithAPIServerImage overrides API server image selection
func WithAPIServerImage(image string) InstallerOption {
	return func(i *installer) {
		i.apiServerImageOverride = image
	}
}

// WithControllerOnly installs only the controller
func WithControllerOnly() InstallerOption {
	return func(i *installer) {
		i.controllerOnly = true
	}
}

// WithUnsafe initializes the installer with unsafe options
func WithUnsafe(o UnsafeOptions) InstallerOption {
	return func(i *installer) {
		i.commonOptions = o.OptionsCommon
		i.enableCoverage = o.Coverage
		i.debugImages = o.Debug
	}
}

// WithSafe initializes the installer with Safe options
func WithSafe(o SafeOptions) InstallerOption {
	return func(i *installer) {
		i.commonOptions = o.OptionsCommon
		i.etcdOptions = &o.Etcd
		i.networkOptions = &o.Network
	}
}

// WithCustomStatusMatch allows to provide additional predicates to
// check if the current install status matches the desired state
func WithCustomStatusMatch(match func(Status) bool) InstallerOption {
	return func(i *installer) {
		i.customMatch = match
	}
}

func tagForCustomImages(controllerImage, apiServerImage string) string {
	bytes := sha1.Sum([]byte(fmt.Sprintf("%s %s", controllerImage, apiServerImage)))
	return hex.EncodeToString(bytes[:])
}

func newInstaller(config *rest.Config, options ...InstallerOption) (*installer, error) {
	i := &installer{
		config: config,
	}
	// default expires offset is 30 days
	i.expiresOffset = 30 * 24 * time.Hour
	for _, o := range options {
		o(i)
	}
	if i.controllerImageOverride != "" && i.apiServerImageOverride != "" {
		// compute a tag for these images
		i.commonOptions.Tag = tagForCustomImages(i.controllerImageOverride, i.apiServerImageOverride)
	}
	if i.debugImages {
		i.commonOptions.Tag = "debug"
	}
	i.labels = map[string]string{
		fryKey:                composeFry,
		imageTagKey:           i.commonOptions.Tag,
		namespaceKey:          i.commonOptions.Namespace,
		defaultServiceTypeKey: i.commonOptions.DefaultServiceType,
	}
	i.apiLabels = map[string]string{
		fryKey:       composeAPIServerFry,
		imageTagKey:  i.commonOptions.Tag,
		namespaceKey: i.commonOptions.Namespace,
	}
	if i.networkOptions != nil &&
		i.networkOptions.CustomTLSBundle != nil {
		// hash tls data so that we can use that to dertermine if we need to re-deploy
		dataToHash := append(append(i.networkOptions.CustomTLSBundle.ca, i.networkOptions.CustomTLSBundle.cert...), i.networkOptions.CustomTLSBundle.key...)
		hash := sha1.Sum(dataToHash)
		i.customTLSHash = hex.EncodeToString(hash[:])
	}
	coreClient, err := corev1.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	i.coreClient = coreClient

	rbacClient, err := rbacv1.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	i.rbacClient = rbacClient

	appsClient, err := appsv1.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	i.appsClient = appsClient

	aggregatorClient, err := kubeaggreagatorv1beta1.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	i.aggregatorClient = aggregatorClient

	client, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	kubeVersion, err := client.ServerVersion()
	if err != nil {
		return nil, err
	}

	return i, checkVersion(kubeVersion, minimumServerVersion)
}

// Status reports current installation status details
type Status struct {
	// True if there is a deployment with compose labels in the cluster
	IsInstalled bool
	// Tag of the installed components
	Tag string
	// Indicates if there is a legacy compose CRD in the system
	IsCrdPresent bool
	// Namespace in which components are deployed
	Namespace string
	// Image of the controller
	ControllerImage string
	// Image of the API service
	APIServiceImage string
	// Default service type for published services
	DefaultServiceType string
	// ControllerLabels contains all labels from Controller deployment
	ControllerLabels map[string]string
	// APIServiceLabels contains all labels from API service deployment
	APIServiceLabels map[string]string
	// APIServiceAnnotations contains annotations from the API service deployment
	APIServiceAnnotations map[string]string
}

func (c *installer) isInstalled() (Status, error) {
	crds, err := apiextensionsclient.NewForConfig(c.config)
	if err != nil {
		return Status{}, err
	}
	isCrdPresent := true
	_, err = crds.CustomResourceDefinitions().Get("stacks.compose.docker.com", metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			isCrdPresent = false
		} else {
			return Status{}, err
		}
	}
	apps, err := c.appsClient.Deployments(metav1.NamespaceAll).List(metav1.ListOptions{
		LabelSelector: everythingSelector,
	})
	if err != nil {
		return Status{}, err
	}
	if len(apps.Items) == 0 {
		return Status{
			IsInstalled:  false,
			IsCrdPresent: isCrdPresent,
		}, nil
	}
	tag := ""
	if apps.Items[0].Labels != nil {
		tag = apps.Items[0].Labels[imageTagKey]
	}

	var apiServiceImage, controllerImage, defaultServiceType string
	var controllerLabels, apiServiceLabels, apiServiceAnnotations map[string]string

	for _, deploy := range apps.Items {
		if deploy.Labels == nil {
			continue
		}
		switch deploy.Labels[fryKey] {
		case composeFry:
			controllerImage = deploy.Spec.Template.Spec.Containers[0].Image
			controllerLabels = deploy.Labels
		case composeAPIServerFry:
			apiServiceImage = deploy.Spec.Template.Spec.Containers[0].Image
			apiServiceLabels = deploy.Labels
			apiServiceAnnotations = deploy.Annotations
		}
		if svcType, ok := deploy.Labels[defaultServiceTypeKey]; ok {
			defaultServiceType = svcType
		}
	}

	return Status{
		IsInstalled:           true,
		Tag:                   tag,
		IsCrdPresent:          isCrdPresent,
		Namespace:             apps.Items[0].Namespace,
		ControllerImage:       controllerImage,
		APIServiceImage:       apiServiceImage,
		DefaultServiceType:    defaultServiceType,
		ControllerLabels:      controllerLabels,
		APIServiceLabels:      apiServiceLabels,
		APIServiceAnnotations: apiServiceAnnotations,
	}, nil
}

func (s Status) match(c *installer) (bool, string) {
	// make sure debug tags are never a match
	if c.commonOptions.Tag == "debug" || s.Tag == "debug" {
		return false, "force redeploy if desired or current state is in debug mode"
	}
	if s.Tag == c.commonOptions.Tag && s.DefaultServiceType == c.commonOptions.DefaultServiceType {
		// check customTLSHash

		if s.APIServiceAnnotations == nil && c.customTLSHash != "" {
			return false, "Custom TLS hash mismatch"
		}

		if s.APIServiceAnnotations != nil {
			actualValue := s.APIServiceAnnotations[customTLSHashAnnotationName]
			if actualValue != c.customTLSHash {
				return false, "Custom TLS hash mismatch"
			}
		}

		// check custom matches
		if c.customMatch == nil || c.customMatch(s) {
			return true, fmt.Sprintf("Compose version %s is already installed in namespace %q with the same settings", c.commonOptions.Tag, s.Namespace)
		}
	}
	return false, fmt.Sprintf("An older version is installed in namespace %q. Uninstalling...", s.Namespace)
}

func (c *installer) install(ctx context.Context) error {
	log.Info("Checking installation state")
	installStatus, err := c.isInstalled()
	if err != nil {
		return err
	}
	if err := c.validateOptions(); err != nil {
		return err
	}
	if installStatus.IsInstalled && !c.controllerOnly {
		match, message := installStatus.match(c)
		log.Info(message)
		if match {
			return nil
		}

		if err = Uninstall(c.config, installStatus.Namespace, false); err != nil {
			return err
		}
		if err = WaitForUninstallCompletion(ctx, c.config, installStatus.Namespace, false); err != nil {
			return err
		}
	}
	log.Infof("Install image with tag %q in namespace %q", c.commonOptions.Tag, c.commonOptions.Namespace)
	ictx := &installerContext{}
	var steps []func(*installerContext) error
	if c.controllerOnly {
		steps = []func(*installerContext) error{
			c.createNamespace,
			c.createPullSecretIfRequired,
			c.createServiceAccount,
			c.createClusterRoleBindings,
			c.createController,
		}
	} else {
		steps = []func(*installerContext) error{
			c.createNamespace,
			c.createPullSecretIfRequired,
			c.createServiceAccount,
			c.createClusterRoleBindings,
			c.createEtcdSecret,
			c.createNetworkSecret,
			c.createAPIServer,
		}
		if !c.disableController {
			steps = append(steps, c.createController)
		}
	}
	steps = append(steps, c.createDefaultClusterRoles)
	for _, step := range steps {
		if err := step(ictx); err != nil {
			return err
		}
	}
	return nil
}
