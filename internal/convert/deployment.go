package convert

import (
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta2"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// toDeployment converts a Compose Service to a Kube Deployment if its replica mode is NOT `global`.
func toDeployment(s v1beta2.ServiceConfig, objectMeta metav1.ObjectMeta, podTemplate apiv1.PodTemplateSpec, labelSelector map[string]string, original appsv1beta2.Deployment) *appsv1beta2.Deployment {
	revisionHistoryLimit := int32(3)
	dep := original.DeepCopy()
	dep.ObjectMeta = objectMeta
	dep.Spec.Replicas = toReplicas(s.Deploy.Replicas)
	dep.Spec.RevisionHistoryLimit = &revisionHistoryLimit
	dep.Spec.Template = forceRestartPolicy(podTemplate, apiv1.RestartPolicyAlways)
	dep.Spec.Strategy = toDeploymentStrategy(s, original.Spec.Strategy)
	dep.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: labelSelector,
	}
	return dep
}

func isGlobal(srv v1beta2.ServiceConfig) bool {
	return srv.Deploy.Mode == "global"
}

func toDeploymentStrategy(s v1beta2.ServiceConfig, original appsv1beta2.DeploymentStrategy) appsv1beta2.DeploymentStrategy {
	config := s.Deploy.UpdateConfig
	if config == nil {
		return original
	}

	if config.Parallelism == nil {
		return original
	}

	return appsv1beta2.DeploymentStrategy{
		Type: appsv1beta2.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &appsv1beta2.RollingUpdateDeployment{
			MaxUnavailable: &intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: int32(*config.Parallelism),
			},
			MaxSurge: nil,
		},
	}
}

func toReplicas(replicas *uint64) *int32 {
	v := int32(1)

	if replicas != nil {
		v = int32(*replicas)
	}

	return &v
}
