package install

import (
	"context"
	"time"

	stacksclient "github.com/docker/compose-on-kubernetes/api/client/clientset/typed/compose/v1beta1"
	log "github.com/sirupsen/logrus"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/rest"
	typedkubeaggreagatorv1beta1 "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1beta1"
)

var (
	deletePolicy           = metav1.DeletePropagationForeground
	deleteBackgroundPolicy = metav1.DeletePropagationBackground
	deleteOrphanPolicy     = metav1.DeletePropagationOrphan
	deleteOptions          = metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}
	deleteBackgroundOptions = metav1.DeleteOptions{
		PropagationPolicy: &deleteBackgroundPolicy,
	}
	listOptions = metav1.ListOptions{
		LabelSelector: everythingSelector,
	}
)

func uninstallErrorFilter(err error) error {
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func uninstallCore(config *rest.Config, namespace string) error {
	client, err := corev1.NewForConfig(config)
	if err != nil {
		return err
	}
	if err = uninstallErrorFilter(client.Secrets(namespace).DeleteCollection(&deleteOptions, listOptions)); err != nil {
		return err
	}
	svcs, err := client.Services(namespace).List(listOptions)
	if uninstallErrorFilter(err) != nil {
		return err
	}
	for _, svc := range svcs.Items {
		if err = uninstallErrorFilter(client.Services(namespace).Delete(svc.Name, &deleteBackgroundOptions)); err != nil {
			return err
		}
	}
	return uninstallErrorFilter(client.ServiceAccounts(namespace).DeleteCollection(&deleteOptions, listOptions))
}

func uninstallApps(config *rest.Config, namespace string) error {
	apps, err := appsv1.NewForConfig(config)
	if uninstallErrorFilter(err) != nil {
		return err
	}
	return uninstallErrorFilter(apps.Deployments(namespace).DeleteCollection(&deleteOptions, listOptions))
}

func uninstallRbac(config *rest.Config) error {
	rbac, err := rbacv1.NewForConfig(config)
	if err != nil {
		return err
	}
	if err = uninstallErrorFilter(rbac.ClusterRoleBindings().DeleteCollection(&deleteOptions, listOptions)); err != nil {
		return err
	}
	if err = uninstallErrorFilter(rbac.ClusterRoles().DeleteCollection(&deleteOptions, listOptions)); err != nil {
		return err
	}
	return uninstallErrorFilter(rbac.RoleBindings("kube-system").DeleteCollection(&deleteOptions, listOptions))
}

func orphanAllStacks(stacks stacksclient.ComposeV1beta1Interface) error {

	type namespaceAndName struct {
		namespace string
		name      string
	}
	toDelete := []namespaceAndName{}
	listOpts := metav1.ListOptions{}
	for {
		stackList, err := stacks.Stacks(metav1.NamespaceAll).List(listOpts)
		if err != nil {
			return err
		}
		for _, stack := range stackList.Items {
			toDelete = append(toDelete, namespaceAndName{namespace: stack.Namespace, name: stack.Name})
		}
		if stackList.Continue == "" {
			break
		}
		listOpts.Continue = stackList.Continue
	}

	log.Infof("Orphaning %d stack(s)", len(toDelete))
	for _, stack := range toDelete {
		if err := stacks.Stacks(stack.namespace).Delete(stack.name, &metav1.DeleteOptions{
			PropagationPolicy: &deleteOrphanPolicy,
		}); err != nil {
			return err
		}
	}
	// wait for all stacks to be effectively deleted
	log.Info("Waiting for stack to be removed")
	for {
		list, err := stacks.Stacks(metav1.NamespaceAll).List(metav1.ListOptions{})
		if err != nil {
			return err
		}
		if len(list.Items) == 0 {
			break
		}
		log.Infof("%d stack(s) waiting to be removed", len(list.Items))
		time.Sleep(time.Second)
	}
	return nil
}

// UninstallCRD uninstalls the CustomResourceDefinition and preserves running stacks
func UninstallCRD(config *rest.Config) error {
	crds, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		return err
	}
	_, err = crds.ApiextensionsV1beta1().CustomResourceDefinitions().Get("stacks.compose.docker.com", metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	// deleteOrphanPolicy is not propagated if we straight out delete the CRD,
	// so let's go and delete the stacks manually before
	stacks, err := stacksclient.NewForConfig(config)
	if err != nil {
		return err
	}
	if err = orphanAllStacks(stacks); err != nil {
		return err
	}

	log.Info("Removing CRD")
	if err = uninstallErrorFilter(crds.ApiextensionsV1beta1().CustomResourceDefinitions().Delete("stacks.compose.docker.com",
		&metav1.DeleteOptions{})); err != nil {
		return err
	}
	// wait for crd removal
	for {
		_, err = crds.ApiextensionsV1beta1().CustomResourceDefinitions().Get("stacks.compose.docker.com", metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		time.Sleep(time.Second)
	}
}

// Uninstall uninstalls the Compose feature.
func Uninstall(config *rest.Config, namespace string, keepCRD bool) error {
	log.Debugf("Uninstall from namespace %q", namespace)

	crds, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		return err
	}

	err = uninstallApps(config, namespace)
	if err != nil {
		return err
	}

	if !keepCRD {
		err = crds.ApiextensionsV1beta1().CustomResourceDefinitions().Delete("stacks.compose.docker.com", &deleteOptions)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	err = uninstallRbac(config)
	if err != nil {
		return err
	}

	aggregator, err := typedkubeaggreagatorv1beta1.NewForConfig(config)
	if err != nil {
		return err
	}

	apisvcs, err := aggregator.APIServices().List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, apisvc := range apisvcs.Items {
		if apisvc.Labels[fryKey] == composeAPIServerFry {
			err = aggregator.APIServices().Delete(apisvc.Name, &deleteBackgroundOptions)
			if err != nil {
				return err
			}
		}
	}
	return uninstallCore(config, namespace)
}
func isDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func waitForNoCrd(ctx context.Context, config *rest.Config, namespace string) error {
	crds, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		return err
	}
	for {
		if isDone(ctx) {
			return context.DeadlineExceeded
		}
		lst, err := crds.ApiextensionsV1beta1().CustomResourceDefinitions().List(listOptions)
		if err != nil {
			return err
		}
		if len(lst.Items) == 0 {
			return nil
		}
		time.Sleep(time.Second)
	}
}

func waitForNoApps(ctx context.Context, config *rest.Config, namespace string) error {
	apps, err := appsv1.NewForConfig(config)
	if err != nil {
		return err
	}
	for {
		if isDone(ctx) {
			return context.DeadlineExceeded
		}
		lst, err := apps.Deployments(namespace).List(listOptions)
		if err != nil {
			return err
		}
		if len(lst.Items) == 0 {
			return nil
		}
		time.Sleep(time.Second)
	}
}

func waitForNoRbac(ctx context.Context, config *rest.Config, namespace string) error {
	rbac, err := rbacv1.NewForConfig(config)
	if err != nil {
		return err
	}

	for {
		if isDone(ctx) {
			return context.DeadlineExceeded
		}
		lst1, err := rbac.ClusterRoleBindings().List(listOptions)
		// if rbac is not enabled, List will fail with status 404 instead of returning an empty list
		// in that case we can just leave early
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if len(lst1.Items) > 0 {
			time.Sleep(time.Second)
			continue
		}
		lst2, err := rbac.ClusterRoles().List(listOptions)
		if err != nil {
			return err
		}
		if len(lst2.Items) > 0 {
			time.Sleep(time.Second)
			continue
		}
		lst3, err := rbac.RoleBindings("kube-system").List(listOptions)
		if err != nil {
			return err
		}
		if len(lst3.Items) > 0 {
			time.Sleep(time.Second)
			continue
		}
		return nil
	}
}

func waitForNoAPIAggregation(ctx context.Context, config *rest.Config, namespace string) error {
	aggregator, err := typedkubeaggreagatorv1beta1.NewForConfig(config)
	if err != nil {
		return err
	}

	for {
		if isDone(ctx) {
			return context.DeadlineExceeded
		}
		apisvcs, err := aggregator.APIServices().List(metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, apisvc := range apisvcs.Items {
			if apisvc.Labels[fryKey] == composeAPIServerFry {
				time.Sleep(time.Second)
				continue
			}
		}
		return nil
	}
}
func waitForNoCoreComponents(ctx context.Context, config *rest.Config, namespace string) error {
	client, err := corev1.NewForConfig(config)
	if err != nil {
		return err
	}
	for {
		if isDone(ctx) {
			return context.DeadlineExceeded
		}
		secrets, err := client.Secrets(namespace).List(listOptions)
		if err != nil {
			return err
		}
		if len(secrets.Items) > 0 {
			time.Sleep(time.Second)
			continue
		}

		svcs, err := client.Services(namespace).List(listOptions)
		if err != nil {
			return err
		}
		if len(svcs.Items) > 0 {
			time.Sleep(time.Second)
			continue
		}

		sas, err := client.ServiceAccounts(namespace).List(listOptions)
		if err != nil {
			return err
		}
		if len(sas.Items) > 0 {
			time.Sleep(time.Second)
			continue
		}
		return nil
	}
}

type uninstallWaiter func(context.Context, *rest.Config, string) error

// WaitForUninstallCompletion waits for an unistall operation to complete
func WaitForUninstallCompletion(ctx context.Context, config *rest.Config, namespace string, skipCRD bool) error {
	waiters := []uninstallWaiter{
		waitForNoApps,
		waitForNoRbac,
		waitForNoAPIAggregation,
		waitForNoCoreComponents,
	}
	if !skipCRD {
		waiters = append(waiters, waitForNoCrd)
	}
	for _, w := range waiters {
		if err := w(ctx, config, namespace); err != nil {
			return err
		}
	}
	return nil
}
