package convert

import (
	"runtime"
	"testing"

	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	. "github.com/docker/compose-on-kubernetes/internal/test/builders"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestToDeployment(t *testing.T) {
	s := Stack("demo",
		WithService("nginx",
			Image("nginx:latest"),
		),
	)
	stack, err := StackToStack(*s, loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	deployment := stack.Deployments["nginx"]

	replicas := int32(1)
	revisionHistoryLimit := int32(3)

	expectedLabels := map[string]string{
		"com.docker.stack.namespace": "demo",
		"com.docker.service.name":    "nginx",
		"com.docker.service.id":      "demo-nginx",
	}

	expectedDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "nginx",
			Labels:      expectedLabels,
			Annotations: expectedAnnotationsOnCreate,
		},
		Spec: appsv1.DeploymentSpec{
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
							Image:           "nginx:latest",
							ImagePullPolicy: apiv1.PullAlways,
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

	assert.Equal(t, expectedDeployment, deployment)
}

func TestToDeploymentWithPorts(t *testing.T) {
	s := Stack("demo",
		WithService("redis",
			Image("redis:alpine"),
			Entrypoint("sh", "-c"),
			Command("echo", "hello"),
			WorkingDir("/code"),
			WithPort(443),
			WithPort(8080, Published(80)),
			Tty, StdinOpen,
		),
	)
	stack, err := StackToStack(*s, loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	deployment := stack.Deployments["redis"]

	expectedContainers := []apiv1.Container{
		{
			Name:            "redis",
			Image:           "redis:alpine",
			ImagePullPolicy: apiv1.PullIfNotPresent,
			Command:         []string{"sh", "-c"},
			Args:            []string{"echo", "hello"},
			WorkingDir:      "/code",
			TTY:             true,
			Stdin:           true,
			Ports: []apiv1.ContainerPort{
				{
					ContainerPort: 443,
					Protocol:      apiv1.ProtocolTCP,
				},
				{
					ContainerPort: 8080,
					Protocol:      apiv1.ProtocolTCP,
				},
			},
		},
	}

	assert.Equal(t, expectedContainers, deployment.Spec.Template.Spec.Containers)
}

func TestToDeploymentWithLongPort(t *testing.T) {
	s := Stack("demo",
		WithService("redis",
			Image("redis:alpine"),
			WithPort(443, Published(4443), ProtocolUDP),
		),
	)
	stack, err := StackToStack(*s, loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	deployment := stack.Deployments["redis"]

	expectedPorts := []apiv1.ContainerPort{
		{
			ContainerPort: 443,
			Protocol:      apiv1.ProtocolUDP,
		},
	}
	assert.Equal(t, expectedPorts, deployment.Spec.Template.Spec.Containers[0].Ports)
}

func TestToDeploymentWithRestartPolicy(t *testing.T) {
	s := Stack("demo",
		WithService("redis",
			Image("redis:alpine"),
			Deploy(RestartPolicy(OnFailure)),
		),
	)
	stack, err := StackToStack(*s, loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	deployment := stack.Deployments["redis"]
	// For a deployment, the restart policy is ignored
	assert.Equal(t, apiv1.RestartPolicyAlways, deployment.Spec.Template.Spec.RestartPolicy)
}

func TestToDeploymentWithReplicas(t *testing.T) {
	s := Stack("demo",
		WithService("redis",
			Image("redis:alpine"),
			Deploy(Replicas(6)),
		),
	)
	stack, err := StackToStack(*s, loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	deployment := stack.Deployments["redis"]

	replicas := int32(6)
	revisionHistoryLimit := int32(3)

	expectedLabels := map[string]string{
		"com.docker.stack.namespace": "demo",
		"com.docker.service.name":    "redis",
		"com.docker.service.id":      "demo-redis",
	}

	expectedDeploymentSpec := appsv1.DeploymentSpec{
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
					},
				},
				Affinity: makeExpectedAffinity(
					kv(kubernetesOs, "linux"),
					kv(kubernetesArch, "amd64"),
				),
			},
		},
	}

	assert.Equal(t, expectedDeploymentSpec, deployment.Spec)
}

func TestToDeploymentWithLabels(t *testing.T) {
	s := Stack("demo",
		WithService("nginx",
			Image("nginx:latest"),
			Deploy(
				WithDeployLabel("prod", "true"),
				WithDeployLabel("mode", "quick"),
			),
			WithLabel("com.example.description", "Database volume"),
			WithLabel("com.example.department", "IT/Ops"),
			WithLabel("com.example.label-with-empty-value", ""),
		),
	)
	stack, err := StackToStack(*s, loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	deployment := stack.Deployments["nginx"]

	expectedDeploymentLabels := map[string]string{
		"com.docker.stack.namespace": "demo",
		"com.docker.service.name":    "nginx",
		"com.docker.service.id":      "demo-nginx",
		"prod":                       "true",
		"mode":                       "quick",
	}

	expectedPodLabels := map[string]string{
		"com.docker.stack.namespace": "demo",
		"com.docker.service.name":    "nginx",
		"com.docker.service.id":      "demo-nginx",
		"prod":                       "true",
		"mode":                       "quick",
	}

	expectedPodAnnotations := map[string]string{
		"com.example.description":            "Database volume",
		"com.example.department":             "IT/Ops",
		"com.example.label-with-empty-value": "",
	}

	assert.Equal(t, expectedDeploymentLabels, deployment.ObjectMeta.Labels)
	assert.Equal(t, expectedPodLabels, deployment.Spec.Template.ObjectMeta.Labels)
	assert.Equal(t, expectedPodAnnotations, deployment.Spec.Template.ObjectMeta.Annotations)
}

func TestToDeploymentWithHostIPC(t *testing.T) {
	s := Stack("demo",
		WithService("redis",
			Image("redis:alpine"),
			IPC("host"),
		),
	)
	stack, err := StackToStack(*s, loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	deployment := stack.Deployments["redis"]

	assert.True(t, deployment.Spec.Template.Spec.HostIPC)
}

func TestToDeploymentWithHostPID(t *testing.T) {
	s := Stack("demo",
		WithService("redis",
			Image("redis:alpine"),
			PID("host"),
		),
	)
	stack, err := StackToStack(*s, loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	deployment := stack.Deployments["redis"]

	assert.True(t, deployment.Spec.Template.Spec.HostPID)
}

func TestToDeploymentWithHostname(t *testing.T) {
	s := Stack("demo",
		WithService("redis",
			Image("redis:alpine"),
			Hostname("foo"),
		),
	)
	stack, err := StackToStack(*s, loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	deployment := stack.Deployments["redis"]

	assert.Equal(t, "foo", deployment.Spec.Template.Spec.Hostname)
}

func TestToDeploymentWithExtraHosts(t *testing.T) {
	s := Stack("demo",
		WithService("redis",
			Image("redis:alpine"),
			WithExtraHost("somehost:162.242.195.82"),
			WithExtraHost("somehost2:162.242.195.82"),
			WithExtraHost("otherhost:50.31.209.229"),
		),
	)
	stack, err := StackToStack(*s, loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	deployment := stack.Deployments["redis"]

	expectedHostAliases := []apiv1.HostAlias{
		{
			IP:        "162.242.195.82",
			Hostnames: []string{"somehost", "somehost2"},
		},
		{
			IP:        "50.31.209.229",
			Hostnames: []string{"otherhost"},
		},
	}

	assert.Equal(t, expectedHostAliases, deployment.Spec.Template.Spec.HostAliases)
}

func TestToDeploymentWithUpdateConfig(t *testing.T) {
	s := Stack("demo",
		WithService("nginx",
			Image("nginx"),
			Deploy(Update(Parallelism(2))),
		),
	)
	stack, err := StackToStack(*s, loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	deployment := stack.Deployments["nginx"]

	assert.Equal(t, "RollingUpdate", string(deployment.Spec.Strategy.Type))
	assert.Equal(t, int32(2), deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.IntVal)
}

func TestToDeploymentWithBind(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("on windows, source path validation is broken (and actually, source validation for windows workload is broken too). Skip it for now, as we don't support it yet")
		return
	}
	s := Stack("demo",
		WithService("nginx",
			Image("nginx"),
			WithVolume(
				Source("/var/run/postgres/postgres.sock"),
				Target("/var/run/postgres/postgres.sock"),
				Mount,
			),
		),
	)
	stack, err := StackToStack(*s, loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	assert.Contains(t, stack.Deployments, "nginx")
}

func TestToDeploymentWithVolume(t *testing.T) {
	s := Stack("demo",
		WithService("nginx",
			Image("nginx"),
			WithVolume(
				Source("dbdata"),
				Target("/var/lib/postgresql/data"),
				Volume,
			),
		),
	)
	stack, err := StackToStack(*s, loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	assert.NotContains(t, stack.Deployments, "nginx")
}
