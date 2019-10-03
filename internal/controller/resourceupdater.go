package controller

import (
	"github.com/docker/compose-on-kubernetes/api/client/clientset"
	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	"github.com/docker/compose-on-kubernetes/api/labels"
	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	"github.com/docker/compose-on-kubernetes/internal/stackresources/diff"
	"github.com/pkg/errors"
	appstypes "k8s.io/api/apps/v1"
	coretypes "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type impersonatingResourceUpdaterProvider struct {
	config     rest.Config
	ownerCache StackOwnerCacher
}

func (p *impersonatingResourceUpdaterProvider) getUpdater(stack *latest.Stack, isDirty bool) (resourceUpdater, error) {
	ic, err := p.ownerCache.getWithRetries(stack, !isDirty)
	if err != nil {
		return nil, err
	}
	localConfig := p.config
	localConfig.Impersonate = ic
	result := &k8sResourceUpdater{
		originalStack: stack,
	}
	if result.stackClient, err = clientset.NewForConfig(&localConfig); err != nil {
		return nil, err
	}
	if result.k8sclient, err = k8sclientset.NewForConfig(&localConfig); err != nil {
		return nil, err
	}
	return result, nil
}

// NewImpersonatingResourceUpdaterProvider creates a ResourceUpdaterProvider that impersonate api calls
func NewImpersonatingResourceUpdaterProvider(config rest.Config, ownerCache StackOwnerCacher) ResourceUpdaterProvider {
	return &impersonatingResourceUpdaterProvider{config: config, ownerCache: ownerCache}
}

var deletePolicy = metav1.DeletePropagationForeground
var deleteOptions = metav1.DeleteOptions{
	PropagationPolicy: &deletePolicy,
}

type k8sResourceUpdater struct {
	stackClient   clientset.Interface
	k8sclient     k8sclientset.Interface
	originalStack *latest.Stack
}

func (u *k8sResourceUpdater) applyDaemonsets(toAdd, toUpdate, toDelete []appstypes.DaemonSet) error {
	for _, r := range toDelete {
		if err := u.k8sclient.AppsV1().DaemonSets(u.originalStack.Namespace).Delete(r.Name, &deleteOptions); err != nil && !kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "k8sResourceUpdater: error while deleting daemonset %s in stack %s", r.Name, stackresources.ObjKey(u.originalStack.Namespace, u.originalStack.Name))
		}
	}
	for _, r := range toAdd {
		if _, err := u.k8sclient.AppsV1().DaemonSets(u.originalStack.Namespace).Create(&r); err != nil {
			return errors.Wrapf(err, "k8sResourceUpdater: error while creating daemonset %s in stack %s", r.Name, stackresources.ObjKey(u.originalStack.Namespace, u.originalStack.Name))
		}
	}
	for _, r := range toUpdate {
		if _, err := u.k8sclient.AppsV1().DaemonSets(u.originalStack.Namespace).Update(&r); err != nil {
			return errors.Wrapf(err, "k8sResourceUpdater: error while patching daemonset %s in stack %s", r.Name, stackresources.ObjKey(u.originalStack.Namespace, u.originalStack.Name))
		}
	}
	return nil
}

func (u *k8sResourceUpdater) applyDeployments(toAdd, toUpdate, toDelete []appstypes.Deployment) error {
	for _, r := range toDelete {
		if err := u.k8sclient.AppsV1().Deployments(u.originalStack.Namespace).Delete(r.Name, &deleteOptions); err != nil && !kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "k8sResourceUpdater: error while deleting deployment %s in stack %s", r.Name, stackresources.ObjKey(u.originalStack.Namespace, u.originalStack.Name))
		}
	}
	for _, r := range toAdd {
		if _, err := u.k8sclient.AppsV1().Deployments(u.originalStack.Namespace).Create(&r); err != nil {
			return errors.Wrapf(err, "k8sResourceUpdater: error while creating deployment %s in stack %s", r.Name, stackresources.ObjKey(u.originalStack.Namespace, u.originalStack.Name))
		}
	}
	for _, r := range toUpdate {
		if _, err := u.k8sclient.AppsV1().Deployments(u.originalStack.Namespace).Update(&r); err != nil {
			return errors.Wrapf(err, "k8sResourceUpdater: error while patching deployment %s in stack %s", r.Name, stackresources.ObjKey(u.originalStack.Namespace, u.originalStack.Name))
		}
	}
	return nil
}

func (u *k8sResourceUpdater) applyStatefulsets(toAdd, toUpdate, toDelete []appstypes.StatefulSet) error {
	for _, r := range toDelete {
		if err := u.k8sclient.AppsV1().StatefulSets(u.originalStack.Namespace).Delete(r.Name, &deleteOptions); err != nil && !kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "k8sResourceUpdater: error while deleting statefulset %s in stack %s", r.Name, stackresources.ObjKey(u.originalStack.Namespace, u.originalStack.Name))
		}
	}
	for _, r := range toAdd {
		if _, err := u.k8sclient.AppsV1().StatefulSets(u.originalStack.Namespace).Create(&r); err != nil {
			return errors.Wrapf(err, "k8sResourceUpdater: error while creating statefulset %s in stack %s", r.Name, stackresources.ObjKey(u.originalStack.Namespace, u.originalStack.Name))
		}
	}
	for _, r := range toUpdate {
		if _, err := u.k8sclient.AppsV1().StatefulSets(u.originalStack.Namespace).Update(&r); err != nil {
			return errors.Wrapf(err, "k8sResourceUpdater: error while patching statefulset %s in stack %s", r.Name, stackresources.ObjKey(u.originalStack.Namespace, u.originalStack.Name))
		}
	}
	return nil
}

func (u *k8sResourceUpdater) applyServices(toAdd, toUpdate, toDelete []coretypes.Service) error {
	for _, r := range toDelete {
		if err := u.k8sclient.CoreV1().Services(u.originalStack.Namespace).Delete(r.Name, &deleteOptions); err != nil && !kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "k8sResourceUpdater: error while deleting service %s in stack %s", r.Name, stackresources.ObjKey(u.originalStack.Namespace, u.originalStack.Name))
		}
	}
	for _, r := range toAdd {
		if _, err := u.k8sclient.CoreV1().Services(u.originalStack.Namespace).Create(&r); err != nil {
			return errors.Wrapf(err, "k8sResourceUpdater: error while creating service %s in stack %s", r.Name, stackresources.ObjKey(u.originalStack.Namespace, u.originalStack.Name))
		}
	}
	for _, r := range toUpdate {
		if _, err := u.k8sclient.CoreV1().Services(u.originalStack.Namespace).Update(&r); err != nil {
			return errors.Wrapf(err, "k8sResourceUpdater: error while patching service %s in stack %s", r.Name, stackresources.ObjKey(u.originalStack.Namespace, u.originalStack.Name))
		}
	}
	return nil
}

func (u *k8sResourceUpdater) applyStackDiff(d *diff.StackStateDiff) error {
	if err := u.applyDaemonsets(d.DaemonsetsToAdd, d.DaemonsetsToUpdate, d.DaemonsetsToDelete); err != nil {
		return err
	}
	if err := u.applyDeployments(d.DeploymentsToAdd, d.DeploymentsToUpdate, d.DeploymentsToDelete); err != nil {
		return err
	}
	if err := u.applyStatefulsets(d.StatefulsetsToAdd, d.StatefulsetsToUpdate, d.StatefulsetsToDelete); err != nil {
		return err
	}
	if err := u.applyServices(d.ServicesToAdd, d.ServicesToUpdate, d.ServicesToDelete); err != nil {
		return err
	}
	return nil
}

func (u *k8sResourceUpdater) updateStackStatus(status latest.StackStatus) (*latest.Stack, error) {
	if u.originalStack.Status != nil && *u.originalStack.Status == status {
		return u.originalStack, nil
	}
	newStack := u.originalStack.Clone()
	newStack.Status = &status
	updated, err := u.stackClient.ComposeLatest().Stacks(u.originalStack.Namespace).WithSkipValidation().Update(newStack)
	if err != nil {
		return nil, errors.Wrapf(err, "k8sResourceUpdater: error while patching stack %s", stackresources.ObjKey(u.originalStack.Namespace, u.originalStack.Name))
	}
	return updated, nil
}

func (u *k8sResourceUpdater) deleteSecretsNoCollection() error {
	if u.originalStack.Spec == nil {
		return nil
	}
	for name, s := range u.originalStack.Spec.Secrets {
		if s.External.External {
			continue
		}
		if err := u.k8sclient.CoreV1().Secrets(u.originalStack.Namespace).Delete(name, nil); err != nil && !kerrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (u *k8sResourceUpdater) deleteConfigMapsNoCollection() error {
	if u.originalStack.Spec == nil {
		return nil
	}
	for name, s := range u.originalStack.Spec.Configs {
		if s.External.External {
			continue
		}
		if err := u.k8sclient.CoreV1().ConfigMaps(u.originalStack.Namespace).Delete(name, nil); err != nil && !kerrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}
func (u *k8sResourceUpdater) deleteSecretsAndConfigMaps() error {
	listOptions := metav1.ListOptions{
		LabelSelector: labels.SelectorForStack(u.originalStack.Name),
	}
	if err := u.k8sclient.CoreV1().Secrets(u.originalStack.Namespace).DeleteCollection(nil, listOptions); err != nil {
		if kerrors.IsForbidden(err) {
			if err := u.deleteSecretsNoCollection(); err != nil {
				return err
			}
		}
	}
	if err := u.k8sclient.CoreV1().ConfigMaps(u.originalStack.Namespace).DeleteCollection(nil, listOptions); err != nil {
		if kerrors.IsForbidden(err) {
			if err := u.deleteConfigMapsNoCollection(); err != nil {
				return err
			}
		}
	}
	return nil
}
