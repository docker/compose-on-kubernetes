package install

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	stacksscheme "github.com/docker/compose-on-kubernetes/api/client/clientset/scheme"
	stacksclient "github.com/docker/compose-on-kubernetes/api/client/clientset/typed/compose/v1beta1"
	stacks "github.com/docker/compose-on-kubernetes/api/compose/v1beta1"
	"github.com/docker/compose-on-kubernetes/api/constants"
	"github.com/docker/compose-on-kubernetes/internal/conversions"
	"github.com/docker/compose-on-kubernetes/internal/internalversion"
	"github.com/docker/compose-on-kubernetes/internal/parsing"
	"github.com/docker/compose-on-kubernetes/internal/registry"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/kubernetes"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

const (
	// BackupPreviousErase erases previous backup
	BackupPreviousErase = iota
	// BackupPreviousMerge adds/merges new data to previous backup
	BackupPreviousMerge
	// BackupPreviousFail fails if a previous backup exists
	BackupPreviousFail

	backupAPIGroup    = "composebackup.docker.com"
	userAnnotationKey = "com.docker.compose.user"
)

func createBackupCrd(crds apiextensionsclient.CustomResourceDefinitionInterface) error {
	log.Info("Creating backup CRD")
	_, err := crds.Create(&apiextensions.CustomResourceDefinition{
		ObjectMeta: v1.ObjectMeta{
			Name: "stackbackups." + backupAPIGroup,
		},
		Spec: apiextensions.CustomResourceDefinitionSpec{
			Group:   backupAPIGroup,
			Version: "v1beta1",
			Names: apiextensions.CustomResourceDefinitionNames{
				Plural:   "stackbackups",
				Singular: "stackbackup",
				Kind:     "StackBackup",
				ListKind: "StackBackupList",
			},
			Scope: apiextensions.NamespaceScoped,
		},
	})
	return err
}

func copyStacksToBackupCrd(source stacks.StackList, kubeClient kubernetes.Interface) error {
	for _, stack := range source.Items {
		stack.APIVersion = fmt.Sprintf("%s/v1beta1", backupAPIGroup)
		stack.ResourceVersion = ""
		stack.Kind = "StackBackup"
		jstack, err := json.Marshal(stack)
		if err != nil {
			return errors.Wrap(err, "failed to marshal stack to JSON")
		}
		res := kubeClient.CoreV1().RESTClient().Verb("POST").
			RequestURI(fmt.Sprintf("/apis/%s/v1beta1/namespaces/%s/stackbackups", backupAPIGroup, stack.Namespace)).
			Body(jstack).Do()
		if res.Error() != nil {
			if apierrors.IsAlreadyExists(res.Error()) {
				// stack already exists, try updating it
				updateRes := kubeClient.CoreV1().RESTClient().Verb("PUT").
					RequestURI(fmt.Sprintf("/apis/%s/v1beta1/namespaces/%s/stackbackups", backupAPIGroup, stack.Namespace)).
					Body(jstack).Do()
				if updateRes.Error() == nil {
					continue
				} else {
					return errors.Wrap(updateRes.Error(), fmt.Sprintf("failed to write then update stack %s/%s", stack.Namespace, stack.Name))
				}
			}
			return errors.Wrap(res.Error(), fmt.Sprintf("failed to write stack %s/%s", stack.Namespace, stack.Name))
		}
	}
	return nil
}

// Backup saves all stacks to a new temporary CRD
func Backup(config *rest.Config, mode int) error {
	log.Info("Starting backup process")
	extClient, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		return err
	}
	crds := extClient.CustomResourceDefinitions()
	_, err = crds.Get("stackbackups."+backupAPIGroup, v1.GetOptions{})
	needsCreate := err != nil
	if err == nil {
		switch mode {
		case BackupPreviousFail:
			return errors.New("a previous backup already exists")
		case BackupPreviousErase:
			log.Info("Erasing previous backup")
			err = crds.Delete("stackbackups."+backupAPIGroup, &v1.DeleteOptions{})
			if err != nil {
				return err
			}
			for i := 0; i < 60; i++ {
				_, err = crds.Get("stackbackups."+backupAPIGroup, v1.GetOptions{})
				if err != nil {
					break
				}
				time.Sleep(1 * time.Second)
			}
			needsCreate = true
		case BackupPreviousMerge:
			log.Info("Merging with previous backup")
		}
	}
	if needsCreate {
		if err := createBackupCrd(crds); err != nil {
			return err
		}
	}
	log.Info("Copying stacks to backup CRD")
	// The stacks client will work both with apiserver and crd backed resource
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	var source stacks.StackList
	listOpts := metav1.ListOptions{}
	for {
		err = kubeClient.CoreV1().RESTClient().Verb("GET").
			RequestURI("/apis/compose.docker.com/v1beta1/stacks").
			VersionedParams(&listOpts, stacksscheme.ParameterCodec).
			Do().
			Into(&source)
		if err != nil {
			return err
		}
		if err = copyStacksToBackupCrd(source, kubeClient); err != nil {
			return err
		}
		if source.Continue == "" {
			break
		}
		listOpts.Continue = source.Continue
	}
	return nil
}

// Restore copies stacks from backup to v1beta1 stacks.compose.docker.com
func Restore(baseConfig *rest.Config, impersonate bool) (map[string]error, error) {
	log.Info("Restoring stacks from backup")
	kubeClient, err := kubernetes.NewForConfig(baseConfig)
	if err != nil {
		return nil, err
	}
	var (
		source   stacks.StackList
		listOpts metav1.ListOptions
	)
	stackErrs := make(map[string]error)
	config := *baseConfig
	client, err := stacksclient.NewForConfig(&config)
	if err != nil {
		return nil, err
	}

	for {
		err = kubeClient.CoreV1().RESTClient().Verb("GET").
			RequestURI(fmt.Sprintf("/apis/%s/v1beta1/stackbackups", backupAPIGroup)).
			VersionedParams(&listOpts, stacksscheme.ParameterCodec).
			Do().
			Into(&source)
		if err != nil {
			return nil, err
		}

		for _, stack := range source.Items {
			stack.APIVersion = "compose.docker.com/v1beta1"
			stack.Kind = "Stack"
			stack.ResourceVersion = ""
			if impersonate {
				username := ""
				if stack.Annotations != nil {
					username = stack.Annotations[userAnnotationKey]
					delete(stack.Annotations, userAnnotationKey)
				}
				if config.Impersonate.UserName != username {
					config.Impersonate.UserName = username
					log.Infof("Impersonating user %q", username)
					if client, err = stacksclient.NewForConfig(&config); err != nil {
						return nil, err
					}
				}
			}
			_, err = client.Stacks(stack.Namespace).WithSkipValidation().Create(&stack)
			if err != nil {
				stackErrs[fmt.Sprintf("%s/%s", stack.Namespace, stack.Name)] = err
				if !apierrors.IsAlreadyExists(err) {
					return stackErrs, errors.Wrap(err, "unable to restore stacks")
				}
			}
		}
		if source.Continue == "" {
			break
		}
		listOpts.Continue = source.Continue
	}
	return stackErrs, nil
}

func dryRunStacks(source stacks.StackList, res map[string]error, coreClient corev1.ServicesGetter, appsClient appsv1.AppsV1Interface) error {
	for _, stack := range source.Items {
		fullname := fmt.Sprintf("%s/%s", stack.Namespace, stack.Name)
		composeConfig, err := parsing.LoadStackData([]byte(stack.Spec.ComposeFile), nil)
		if err != nil {
			res[fullname] = err
			continue
		}
		spec := conversions.FromComposeConfig(composeConfig)
		internalStack := &internalversion.Stack{
			ObjectMeta: v1.ObjectMeta{
				Name:      stack.Name,
				Namespace: stack.Namespace,
			},
			Spec: internalversion.StackSpec{
				Stack: spec,
			},
		}
		errs := field.ErrorList{}
		errs = append(errs, registry.ValidateObjectNames(internalStack)...)
		errs = append(errs, registry.ValidateDryRun(internalStack)...)
		errs = append(errs, registry.ValidateCollisions(coreClient, appsClient, internalStack)...)
		aggregate := errs.ToAggregate()
		if aggregate != nil {
			res[fullname] = aggregate
		}
	}
	return nil
}

// DryRun checks existing stacks for conversion errors or conflicts
func DryRun(config *rest.Config) (map[string]error, error) {
	res := make(map[string]error)
	log.Info("Performing dry-run")
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	var source stacks.StackList
	listOpts := metav1.ListOptions{}
	for {
		err = kubeClient.CoreV1().RESTClient().Verb("GET").
			RequestURI("/apis/compose.docker.com/v1beta1/stacks").
			VersionedParams(&listOpts, stacksscheme.ParameterCodec).
			Do().
			Into(&source)
		if err != nil {
			return nil, err
		}
		if err = dryRunStacks(source, res, kubeClient.CoreV1(), kubeClient.AppsV1()); err != nil {
			return nil, err
		}
		if source.Continue == "" {
			break
		}
		listOpts.Continue = source.Continue
	}
	return res, nil
}

// DeleteBackup deletes the backup CRD
func DeleteBackup(config *rest.Config) error {
	extClient, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		return err
	}
	crds := extClient.CustomResourceDefinitions()
	_, err = crds.Get("stackbackups."+backupAPIGroup, v1.GetOptions{})
	if err == nil {
		return crds.Delete("stackbackups."+backupAPIGroup, &v1.DeleteOptions{})
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	for {
		_, err = crds.Get("stackbackups."+backupAPIGroup, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		time.Sleep(time.Second)
	}
}

// HasBackupCRD indicates if the backup crd is there
func HasBackupCRD(config *rest.Config) (bool, error) {
	extClient, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		return false, err
	}
	crds := extClient.CustomResourceDefinitions()
	_, err = crds.Get("stackbackups."+backupAPIGroup, v1.GetOptions{})
	if err == nil {
		return true, nil
	}
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	return false, err
}

// CRDCRD installs the CRD component of CRD install
func CRDCRD(config *rest.Config) error {
	extClient, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		return err
	}
	crds := extClient.CustomResourceDefinitions()
	_, err = crds.Create(&apiextensions.CustomResourceDefinition{
		ObjectMeta: v1.ObjectMeta{
			Name: "stacks.compose.docker.com",
		},
		Spec: apiextensions.CustomResourceDefinitionSpec{
			Group:   "compose.docker.com",
			Version: "v1beta1",
			Names: apiextensions.CustomResourceDefinitionNames{
				Plural:   "stacks",
				Singular: "stack",
				Kind:     "Stack",
				ListKind: "StackList",
			},
			Scope: apiextensions.NamespaceScoped,
		},
	})
	return err
}

// UninstallComposeCRD uninstalls compose in CRD mode, preserving running stacks
func UninstallComposeCRD(config *rest.Config, namespace string) error {
	if err := Uninstall(config, namespace, true); err != nil {
		return err
	}
	WaitForUninstallCompletion(context.Background(), config, namespace, true)
	if err := UninstallCRD(config); err != nil {
		return err
	}
	WaitForUninstallCompletion(context.Background(), config, namespace, false)
	return nil
}

// UninstallComposeAPIServer uninstalls compose in API server mode, preserving running stacks
func UninstallComposeAPIServer(config *rest.Config, namespace string) error {
	// First, shoot the controller
	log.Info("Removing controller")
	apps, err := appsv1.NewForConfig(config)
	if err != nil {
		return err
	}
	err = apps.Deployments(namespace).Delete("compose", &metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	log.Info("Unlinking stacks")
	if err := UninstallCRD(config); err != nil {
		return err
	}
	log.Info("Uninstalling all components")
	if err := Uninstall(config, namespace, false); err != nil {
		return err
	}
	log.Info("Waiting for uninstallation to complete")
	WaitForUninstallCompletion(context.Background(), config, namespace, false)
	return nil
}

// Update perform a full update operation, restoring the stacks
func Update(config *rest.Config, namespace, tag string, abortOnError bool) (map[string]error, error) {
	if abortOnError {
		errs, err := DryRun(config)
		if err != nil {
			return errs, err
		}
		if len(errs) != 0 {
			return errs, errors.New("dry-run returned errors")
		}
	}
	err := Backup(config, BackupPreviousErase)
	if err != nil {
		return nil, err
	}
	err = UninstallComposeCRD(config, namespace)
	if err != nil {
		return nil, err
	}
	installOptAPIAggregation := WithUnsafe(UnsafeOptions{
		OptionsCommon: OptionsCommon{
			Namespace:              namespace,
			Tag:                    tag,
			ReconciliationInterval: constants.DefaultFullSyncInterval,
		},
	})
	err = Do(context.Background(), config, installOptAPIAggregation, WithoutController())
	if err != nil {
		return nil, err
	}
	ready := false
	for i := 0; i < 30; i++ {
		running, err := IsRunning(config)
		if err != nil {
			return nil, err
		}
		if running {
			ready = true
			break
		}
		time.Sleep(time.Second)
	}
	if !ready {
		return nil, errors.New("compose did not start properly")
	}
	errs, err := Restore(config, true)
	err2 := Do(context.Background(), config, installOptAPIAggregation, WithControllerOnly())
	if err2 != nil {
		return errs, err2
	}
	return errs, err
}
