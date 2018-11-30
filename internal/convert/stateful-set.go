package convert

import (
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta2"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// toStatefulSet converts a Compose Service to a Kube StatefulSet if its replica mode is NOT `global
// and it has persistent volumes.
func toStatefulSet(s v1beta2.ServiceConfig, objectMeta metav1.ObjectMeta, podTemplate apiv1.PodTemplateSpec,
	labelSelector map[string]string, original appsv1beta2.StatefulSet) *appsv1beta2.StatefulSet {
	revisionHistoryLimit := int32(3)
	res := original.DeepCopy()
	res.ObjectMeta = objectMeta
	res.Spec.Replicas = toReplicas(s.Deploy.Replicas)
	res.Spec.RevisionHistoryLimit = &revisionHistoryLimit
	res.Spec.Template = forceRestartPolicy(podTemplate, apiv1.RestartPolicyAlways)
	res.Spec.UpdateStrategy = toStatefulSetUpdateStrategy(s, res.Spec.UpdateStrategy)
	res.Spec.VolumeClaimTemplates = toPersistentVolumeClaims(s, res.Spec.VolumeClaimTemplates)
	res.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: labelSelector,
	}
	return res
}

func toStatefulSetUpdateStrategy(s v1beta2.ServiceConfig, original appsv1beta2.StatefulSetUpdateStrategy) appsv1beta2.StatefulSetUpdateStrategy {
	config := s.Deploy.UpdateConfig
	if config == nil {
		return original
	}

	if config.Parallelism == nil {
		return original
	}

	return appsv1beta2.StatefulSetUpdateStrategy{
		Type: appsv1beta2.RollingUpdateStatefulSetStrategyType,
		RollingUpdate: &appsv1beta2.RollingUpdateStatefulSetStrategy{
			Partition: int32Ptr(config.Parallelism),
		},
	}
}

func int32Ptr(value *uint64) *int32 {
	if value == nil {
		return nil
	}

	result := int32(*value)
	return &result
}
