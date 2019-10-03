package convert

import (
	"testing"

	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	. "github.com/docker/compose-on-kubernetes/internal/test/builders"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestToDaemonSet(t *testing.T) {
	s := Stack("demo", WithNamespace("test"),
		WithService("redis",
			Image("redis:alpine"),
			Deploy(ModeGlobal),
		),
	)
	stack, err := StackToStack(*s, loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	daemonSet := stack.Daemonsets["test/redis"]

	expectedLabels := map[string]string{
		"com.docker.stack.namespace": "demo",
		"com.docker.service.name":    "redis",
		"com.docker.service.id":      "demo-redis",
	}

	expectedDaemonSet := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "redis",
			Labels:      expectedLabels,
			Namespace:   "test",
			Annotations: expectedAnnotationsOnCreate,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: expectedLabels,
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: expectedLabels,
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:            "redis",
							Image:           "redis:alpine",
							ImagePullPolicy: apiv1.PullIfNotPresent,
						},
					},
					Affinity: makeExpectedAffinity(
						kv(kubernetesOs, "linux"),
						kv(kubernetesArch, "amd64"),
					),
				},
			},
		},
	}

	assert.Equal(t, expectedDaemonSet, daemonSet)
}
