package convert

import (
	"runtime"
	"testing"

	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	. "github.com/docker/compose-on-kubernetes/internal/test/builders"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStatefulSet(t *testing.T) {
	s, err := StackToStack(*Stack("demo",
		WithService("nginx",
			Image("nginx"),
			WithVolume(Source("mydata"), Target("/data"), Volume),
		),
	), loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)

	replicas := int32(1)
	revisionHistoryLimit := int32(3)

	expectedLabels := map[string]string{
		"com.docker.stack.namespace": "demo",
		"com.docker.service.name":    "nginx",
		"com.docker.service.id":      "demo-nginx",
	}

	statefulSet := s.Statefulsets["nginx"]

	expectedStatefulSet := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "nginx",
			Labels:      expectedLabels,
			Annotations: expectedAnnotationsOnCreate,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: expectedLabels,
			},
			Replicas:             &replicas,
			RevisionHistoryLimit: &revisionHistoryLimit,
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: expectedLabels,
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:            "nginx",
							Image:           "nginx",
							ImagePullPolicy: apiv1.PullIfNotPresent,
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      "mydata",
									MountPath: "/data",
								},
							},
						},
					},
					Affinity: makeExpectedAffinity(
						kv(kubernetesOs, "linux"),
						kv(kubernetesArch, "amd64"),
					),
				},
			},
			VolumeClaimTemplates: []apiv1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "mydata",
					},
					Spec: apiv1.PersistentVolumeClaimSpec{
						AccessModes: []apiv1.PersistentVolumeAccessMode{
							apiv1.ReadWriteOnce,
						},
						Resources: apiv1.ResourceRequirements{
							Requests: apiv1.ResourceList{
								apiv1.ResourceStorage: resource.MustParse("100Mi"),
							},
						},
					},
				},
			},
		},
	}

	assert.Equal(t, expectedStatefulSet, statefulSet)
}

func TestStatefulSetWithBind(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("on windows, source path validation is broken (and actually, source validation for windows workload is broken too). Skip it for now, as we don't support it yet")
		return
	}
	stack := Stack("demo",
		WithService("nginx",
			Image("nginx"),
			WithVolume(
				Source("/var/run/postgres/postgres.sock"),
				Target("/var/run/postgres/postgres.sock"),
				Mount,
			),
		),
	)
	s, err := StackToStack(*stack, loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	d := s.Deployments["nginx"]
	assert.NotNil(t, d)
	assert.Equal(t, 1, len(d.Spec.Template.Spec.Volumes))
	assert.Equal(t, "/var/run/postgres", d.Spec.Template.Spec.Volumes[0].HostPath.Path)
	assert.Equal(t, "postgres.sock", d.Spec.Template.Spec.Containers[0].VolumeMounts[0].SubPath)
	assert.Equal(t, "/var/run/postgres/postgres.sock", d.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath)
}

func TestStatefulSetWithVolume(t *testing.T) {
	s, err := StackToStack(*Stack("demo",
		WithService("nginx",
			Image("nginx"),
			WithVolume(
				Source("dbdata"),
				Target("/var/lib/postgresql/data"),
				Volume,
			),
		),
	), loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	assert.Contains(t, s.Statefulsets, "nginx")
}

func TestStatefulSetWithReplicas(t *testing.T) {
	s, err := StackToStack(*Stack("demo",
		WithService("redis",
			Image("redis:alpine"),
			WithVolume(Source("data"), Volume),
			Deploy(Replicas(6)),
		),
	), loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	assert.Contains(t, s.Statefulsets, "redis")
	replicas := int32(6)
	revisionHistoryLimit := int32(3)

	expectedLabels := map[string]string{
		"com.docker.stack.namespace": "demo",
		"com.docker.service.name":    "redis",
		"com.docker.service.id":      "demo-redis",
	}

	expectedStatefulSetSpec := appsv1.StatefulSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: expectedLabels,
		},
		Replicas:             &replicas,
		RevisionHistoryLimit: &revisionHistoryLimit,
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
						VolumeMounts: []apiv1.VolumeMount{
							{
								Name: "data",
							},
						},
					},
				},
				Affinity: makeExpectedAffinity(
					kv(kubernetesOs, "linux"),
					kv(kubernetesArch, "amd64"),
				),
			},
		},
		VolumeClaimTemplates: []apiv1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "data",
				},
				Spec: apiv1.PersistentVolumeClaimSpec{
					AccessModes: []apiv1.PersistentVolumeAccessMode{
						apiv1.ReadWriteOnce,
					},
					Resources: apiv1.ResourceRequirements{
						Requests: apiv1.ResourceList{
							apiv1.ResourceStorage: resource.MustParse("100Mi"),
						},
					},
				},
			},
		},
	}

	assert.Equal(t, expectedStatefulSetSpec, s.Statefulsets["redis"].Spec)
}

func TestStatefulSetWithUpdateConfig(t *testing.T) {
	s, err := StackToStack(*Stack("demo",
		WithService("nginx",
			Image("nginx"),
			WithVolume(Source("data"), Volume),
			Deploy(Update(Parallelism(2))),
		),
	), loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	assert.Contains(t, s.Statefulsets, "nginx")

	assert.Equal(t, "RollingUpdate", string(s.Statefulsets["nginx"].Spec.UpdateStrategy.Type))
	assert.Equal(t, int32(2), *s.Statefulsets["nginx"].Spec.UpdateStrategy.RollingUpdate.Partition)
}
