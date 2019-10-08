package convert

import (
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// toDaemonSet converts a Compose Service to a Kube DaemonSet if its replica mode is `global`.
func toDaemonSet(objectMeta metav1.ObjectMeta, podTemplate apiv1.PodTemplateSpec, labelSelector map[string]string, original appsv1.DaemonSet) *appsv1.DaemonSet {
	ds := original.DeepCopy()
	ds.ObjectMeta = objectMeta
	ds.Spec.Template = podTemplate
	ds.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: labelSelector,
	}
	return ds
}
