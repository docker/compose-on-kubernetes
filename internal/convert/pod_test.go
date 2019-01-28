package convert

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	. "github.com/docker/compose-on-kubernetes/internal/test/builders"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func podTemplate(t *testing.T, stack *latest.Stack) apiv1.PodTemplateSpec {
	res, err := podTemplateWithError(stack)
	assert.NoError(t, err)
	return res
}

func podTemplateWithError(stack *latest.Stack) (apiv1.PodTemplateSpec, error) {
	s, err := StackToStack(*stack, loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	if err != nil {
		return apiv1.PodTemplateSpec{}, err
	}
	for _, r := range s.Deployments {
		return r.Spec.Template, nil
	}
	for _, r := range s.Daemonsets {
		return r.Spec.Template, nil
	}
	for _, r := range s.Statefulsets {
		return r.Spec.Template, nil
	}
	return apiv1.PodTemplateSpec{}, nil
}

func TestToPodWithDockerSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("on windows, source path validation is broken (and actually, source validation for windows workload is broken too). Skip it for now, as we don't support it yet")
		return
	}
	podTemplate := podTemplate(t, Stack("demo",
		WithService("redis",
			Image("redis:alpine"),
			WithVolume(
				Source("/var/run/docker.sock"),
				Target("/var/run/docker.sock"),
				Mount,
			),
		),
	))

	expectedVolume := apiv1.Volume{
		Name: "mount-0",
		VolumeSource: apiv1.VolumeSource{
			HostPath: &apiv1.HostPathVolumeSource{
				Path: "/var/run",
			},
		},
	}

	expectedMount := apiv1.VolumeMount{
		Name:      "mount-0",
		MountPath: "/var/run/docker.sock",
		SubPath:   "docker.sock",
	}

	assert.Len(t, podTemplate.Spec.Volumes, 1)
	assert.Len(t, podTemplate.Spec.Containers[0].VolumeMounts, 1)
	assert.Equal(t, expectedVolume, podTemplate.Spec.Volumes[0])
	assert.Equal(t, expectedMount, podTemplate.Spec.Containers[0].VolumeMounts[0])
}

func TestToPodWithFunkyCommand(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("redis",
			Image("basi/node-exporter"),
			Command("-collector.procfs", "/host/proc", "-collector.sysfs", "/host/sys"),
		),
	))

	expectedArgs := []string{
		`-collector.procfs`,
		`/host/proc`, // ?
		`-collector.sysfs`,
		`/host/sys`, // ?
	}
	assert.Equal(t, expectedArgs, podTemplate.Spec.Containers[0].Args)
}

func TestToPodWithGlobalVolume(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("db",
			Image("postgres:9.4"),
			WithVolume(
				Source("dbdata"),
				Target("/var/lib/postgresql/data"),
				Volume,
			),
		),
	))

	expectedMount := apiv1.VolumeMount{
		Name:      "dbdata",
		MountPath: "/var/lib/postgresql/data",
	}
	assert.Len(t, podTemplate.Spec.Volumes, 0)
	assert.Len(t, podTemplate.Spec.Containers[0].VolumeMounts, 1)
	assert.Equal(t, expectedMount, podTemplate.Spec.Containers[0].VolumeMounts[0])
}

func TestToPodWithResources(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("db",
			Image("postgres:9.4"),
			Deploy(Resources(
				Limits(CPUs("0.001"), Memory(50*1024*1024)),
				Reservations(CPUs("0.0001"), Memory(20*1024*1024)),
			)),
		),
	))

	expectedResourceRequirements := apiv1.ResourceRequirements{
		Limits: map[apiv1.ResourceName]resource.Quantity{
			apiv1.ResourceCPU:    resource.MustParse("0.001"),
			apiv1.ResourceMemory: resource.MustParse(fmt.Sprintf("%d", 50*1024*1024)),
		},
		Requests: map[apiv1.ResourceName]resource.Quantity{
			apiv1.ResourceCPU:    resource.MustParse("0.0001"),
			apiv1.ResourceMemory: resource.MustParse(fmt.Sprintf("%d", 20*1024*1024)),
		},
	}
	assert.Equal(t, expectedResourceRequirements, podTemplate.Spec.Containers[0].Resources)
}

func TestToPodWithCapabilities(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("redis",
			Image("redis:alpine"),
			WithCapAdd("ALL"),
			WithCapDrop("NET_ADMIN", "SYS_ADMIN"),
		),
	))

	expectedSecurityContext := &apiv1.SecurityContext{
		Capabilities: &apiv1.Capabilities{
			Add:  []apiv1.Capability{"ALL"},
			Drop: []apiv1.Capability{"NET_ADMIN", "SYS_ADMIN"},
		},
	}

	assert.Equal(t, expectedSecurityContext, podTemplate.Spec.Containers[0].SecurityContext)
}

func TestToPodWithReadOnly(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("redis",
			Image("redis:alpine"),
			ReadOnly,
		),
	))

	yes := true
	expectedSecurityContext := &apiv1.SecurityContext{
		ReadOnlyRootFilesystem: &yes,
	}
	assert.Equal(t, expectedSecurityContext, podTemplate.Spec.Containers[0].SecurityContext)
}

func TestToPodWithPrivileged(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("redis",
			Image("redis:alpine"),
			Privileged,
		),
	))

	yes := true
	expectedSecurityContext := &apiv1.SecurityContext{
		Privileged: &yes,
	}
	assert.Equal(t, expectedSecurityContext, podTemplate.Spec.Containers[0].SecurityContext)
}

func strptr(s string) *string {
	return &s
}

func TestToPodWithEnvNilShouldErrorOut(t *testing.T) {
	stack := Stack("demo",
		WithService("redis",
			Image("redis:alpine"),
			WithEnvironment("SESSION_SECRET", nil),
		),
	)
	_, err := StackToStack(*stack, loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.Error(t, err)
}

func TestToPodWithEnv(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("redis",
			Image("redis:alpine"),
			WithEnvironment("RACK_ENV", strptr("development")),
			WithEnvironment("SHOW", strptr("true")),
		),
	))

	expectedEnv := []apiv1.EnvVar{
		{
			Name:  "RACK_ENV",
			Value: "development",
		},
		{
			Name:  "SHOW",
			Value: "true",
		},
	}

	assert.Equal(t, expectedEnv, podTemplate.Spec.Containers[0].Env)
}

func TestToPodWithVolume(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("on windows, source path validation is broken (and actually, source validation for windows workload is broken too). Skip it for now, as we don't support it yet")
		return
	}
	podTemplate := podTemplate(t, Stack("demo",
		WithService("nginx",
			Image("nginx"),
			WithVolume(Source("/ignore"), Target("/ignore"), Mount),
			WithVolume(Source("/opt/data"), Target("/var/lib/mysql"), VolumeReadOnly, Mount),
		),
	))

	assert.Len(t, podTemplate.Spec.Volumes, 2)
	assert.Len(t, podTemplate.Spec.Containers[0].VolumeMounts, 2)
}

func TestToPodWithRelativeVolumes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("on windows, source path validation is broken (and actually, source validation for windows workload is broken too). Skip it for now, as we don't support it yet")
		return
	}
	stack := Stack("demo",
		WithService("nginx",
			Image("nginx"),
			WithVolume(Source("./fail"), Target("/ignore"), Mount),
		))
	_, err := StackToStack(*stack, loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.Error(t, err)
}

func TestToPodWithHealthCheck(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("nginx",
			Image("nginx"),
			Healthcheck(
				Test("CMD", "curl", "-f", "http://localhost"),
				Interval(90*time.Second),
				Timeout(10*time.Second),
				Retries(3),
			),
		),
	))
	expectedLivenessProbe := &apiv1.Probe{
		TimeoutSeconds:   10,
		PeriodSeconds:    90,
		FailureThreshold: 3,
		Handler: apiv1.Handler{
			Exec: &apiv1.ExecAction{
				Command: []string{"curl", "-f", "http://localhost"},
			},
		},
	}

	assert.Equal(t, expectedLivenessProbe, podTemplate.Spec.Containers[0].LivenessProbe)
}

func TestToPodWithShellHealthCheck(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("nginx",
			Image("nginx"),
			Healthcheck(
				Test("CMD-SHELL", "curl -f http://localhost"),
			),
		),
	))

	expectedLivenessProbe := &apiv1.Probe{
		TimeoutSeconds:   1,
		PeriodSeconds:    1,
		FailureThreshold: 3,
		Handler: apiv1.Handler{
			Exec: &apiv1.ExecAction{
				Command: []string{"sh", "-c", "curl -f http://localhost"},
			},
		},
	}

	assert.Equal(t, expectedLivenessProbe, podTemplate.Spec.Containers[0].LivenessProbe)
}

func TestToPodWithHealthCheckEmptyCommand(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("nginx",
			Image("nginx"),
			Healthcheck(Test()),
		),
	))

	assert.Nil(t, podTemplate.Spec.Containers[0].LivenessProbe)
}

func TestToPodWithTargetlessExternalSecret(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("nginx",
			Image("nginx"),
			WithSecret(
				SecretSource("my_secret"),
			),
		),
	))

	expectedVolume := apiv1.Volume{
		Name: "secret-0",
		VolumeSource: apiv1.VolumeSource{
			Secret: &apiv1.SecretVolumeSource{
				SecretName: "my_secret",
				Items: []apiv1.KeyToPath{
					{
						Key:  "file", // TODO: This is the key we assume external secrets use
						Path: "secret-0",
					},
				},
			},
		},
	}

	expectedMount := apiv1.VolumeMount{
		Name:      "secret-0",
		ReadOnly:  true,
		MountPath: "/run/secrets/my_secret",
		SubPath:   "secret-0",
	}

	assert.Len(t, podTemplate.Spec.Volumes, 1)
	assert.Len(t, podTemplate.Spec.Containers[0].VolumeMounts, 1)
	assert.Equal(t, expectedVolume, podTemplate.Spec.Volumes[0])
	assert.Equal(t, expectedMount, podTemplate.Spec.Containers[0].VolumeMounts[0])
}

func TestToPodWithExternalSecret(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("nginx",
			Image("nginx"),
			WithSecret(
				SecretSource("my_secret"),
				SecretTarget("nginx_secret"),
			),
		),
	))

	expectedVolume := apiv1.Volume{
		Name: "secret-0",
		VolumeSource: apiv1.VolumeSource{
			Secret: &apiv1.SecretVolumeSource{
				SecretName: "my_secret",
				Items: []apiv1.KeyToPath{
					{
						Key:  "file", // TODO: This is the key we assume external secrets use
						Path: "secret-0",
					},
				},
			},
		},
	}

	expectedMount := apiv1.VolumeMount{
		Name:      "secret-0",
		ReadOnly:  true,
		MountPath: "/run/secrets/nginx_secret",
		SubPath:   "secret-0",
	}

	assert.Len(t, podTemplate.Spec.Volumes, 1)
	assert.Len(t, podTemplate.Spec.Containers[0].VolumeMounts, 1)
	assert.Equal(t, expectedVolume, podTemplate.Spec.Volumes[0])
	assert.Equal(t, expectedMount, podTemplate.Spec.Containers[0].VolumeMounts[0])
}

func TestToPodWithFileBasedSecret(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("nginx",
			Image("nginx"),
			WithSecret(
				SecretSource("my_secret"),
			),
		),
		WithSecretConfig("my_secret", SecretFile("./secret.txt")),
	))

	expectedVolume := apiv1.Volume{
		Name: "secret-0",
		VolumeSource: apiv1.VolumeSource{
			Secret: &apiv1.SecretVolumeSource{
				SecretName: "my_secret",
				Items: []apiv1.KeyToPath{
					{
						Key:  "secret.txt",
						Path: "secret-0",
					},
				},
			},
		},
	}

	expectedMount := apiv1.VolumeMount{
		Name:      "secret-0",
		ReadOnly:  true,
		MountPath: "/run/secrets/my_secret",
		SubPath:   "secret-0",
	}

	assert.Len(t, podTemplate.Spec.Volumes, 1)
	assert.Len(t, podTemplate.Spec.Containers[0].VolumeMounts, 1)
	assert.Equal(t, expectedVolume, podTemplate.Spec.Volumes[0])
	assert.Equal(t, expectedMount, podTemplate.Spec.Containers[0].VolumeMounts[0])
}

func TestToPodWithTwoFileBasedSecrets(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("nginx",
			Image("nginx"),
			WithSecret(
				SecretSource("my_secret1"),
			),
			WithSecret(
				SecretSource("my_secret2"),
				SecretTarget("secret2"),
			),
		),
		WithSecretConfig("my_secret1", SecretFile("./secret1.txt")),
		WithSecretConfig("my_secret2", SecretFile("./secret2.txt")),
	))

	expectedVolumes := []apiv1.Volume{
		{
			Name: "secret-0",
			VolumeSource: apiv1.VolumeSource{
				Secret: &apiv1.SecretVolumeSource{
					SecretName: "my_secret1",
					Items: []apiv1.KeyToPath{
						{
							Key:  "secret1.txt",
							Path: "secret-0",
						},
					},
				},
			},
		},
		{
			Name: "secret-1",
			VolumeSource: apiv1.VolumeSource{
				Secret: &apiv1.SecretVolumeSource{
					SecretName: "my_secret2",
					Items: []apiv1.KeyToPath{
						{
							Key:  "secret2.txt",
							Path: "secret-1",
						},
					},
				},
			},
		},
	}

	expectedMounts := []apiv1.VolumeMount{
		{
			Name:      "secret-0",
			ReadOnly:  true,
			MountPath: "/run/secrets/my_secret1",
			SubPath:   "secret-0",
		},
		{
			Name:      "secret-1",
			ReadOnly:  true,
			MountPath: "/run/secrets/secret2",
			SubPath:   "secret-1",
		},
	}

	assert.Equal(t, expectedVolumes, podTemplate.Spec.Volumes)
	assert.Equal(t, expectedMounts, podTemplate.Spec.Containers[0].VolumeMounts)
}

func TestToPodWithTerminationGracePeriod(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("redis",
			Image("redis:alpine"),
			StopGracePeriod(100*time.Second),
		),
	))

	expected := int64(100)
	assert.Equal(t, &expected, podTemplate.Spec.TerminationGracePeriodSeconds)
}

func TestToPodWithTmpfs(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("redis",
			Image("redis:alpine"),
			WithTmpFS("/tmp"),
		),
	))

	expectedVolume := apiv1.Volume{
		Name: "tmp-0",
		VolumeSource: apiv1.VolumeSource{
			EmptyDir: &apiv1.EmptyDirVolumeSource{
				Medium: "Memory",
			},
		},
	}

	expectedMount := apiv1.VolumeMount{
		Name:      "tmp-0",
		MountPath: "/tmp",
	}

	assert.Len(t, podTemplate.Spec.Volumes, 1)
	assert.Len(t, podTemplate.Spec.Containers[0].VolumeMounts, 1)
	assert.Equal(t, expectedVolume, podTemplate.Spec.Volumes[0])
	assert.Equal(t, expectedMount, podTemplate.Spec.Containers[0].VolumeMounts[0])
}

func TestToPodWithNumericalUser(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("redis",
			Image("redis:alpine"),
			User(1000),
		),
	))

	userID := int64(1000)

	expectedSecurityContext := &apiv1.SecurityContext{
		RunAsUser: &userID,
	}

	assert.Equal(t, expectedSecurityContext, podTemplate.Spec.Containers[0].SecurityContext)
}

func TestToPodWithGitVolume(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("redis",
			Image("redis:alpine"),
			WithVolume(
				Source("git@github.com:moby/moby.git"),
				Target("/sources"),
				Mount,
			),
		),
	))

	expectedVolume := apiv1.Volume{
		Name: "mount-0",
		VolumeSource: apiv1.VolumeSource{
			GitRepo: &apiv1.GitRepoVolumeSource{
				Repository: "git@github.com:moby/moby.git",
			},
		},
	}

	expectedMount := apiv1.VolumeMount{
		Name:      "mount-0",
		ReadOnly:  false,
		MountPath: "/sources",
	}

	assert.Len(t, podTemplate.Spec.Volumes, 1)
	assert.Len(t, podTemplate.Spec.Containers[0].VolumeMounts, 1)
	assert.Equal(t, expectedVolume, podTemplate.Spec.Volumes[0])
	assert.Equal(t, expectedMount, podTemplate.Spec.Containers[0].VolumeMounts[0])
}

func TestToPodWithFileBasedConfig(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("nginx",
			Image("nginx"),
			WithConfig(
				ConfigSource("my_config"),
				ConfigTarget("/usr/share/nginx/html/index.html"),
				ConfigUID("103"),
				ConfigGID("103"),
				ConfigMode(0440),
			),
		),
		WithConfigObjConfig("my_config", ConfigFile("./file.html")),
	))

	mode := int32(0440)

	expectedVolume := apiv1.Volume{
		Name: "config-0",
		VolumeSource: apiv1.VolumeSource{
			ConfigMap: &apiv1.ConfigMapVolumeSource{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: "my_config",
				},
				Items: []apiv1.KeyToPath{
					{
						Key:  "file.html",
						Path: "config-0",
						Mode: &mode,
					},
				},
			},
		},
	}

	expectedMount := apiv1.VolumeMount{
		Name:      "config-0",
		ReadOnly:  true,
		MountPath: "/usr/share/nginx/html/index.html",
		SubPath:   "config-0",
	}

	assert.Len(t, podTemplate.Spec.Volumes, 1)
	assert.Len(t, podTemplate.Spec.Containers[0].VolumeMounts, 1)
	assert.Equal(t, expectedVolume, podTemplate.Spec.Volumes[0])
	assert.Equal(t, expectedMount, podTemplate.Spec.Containers[0].VolumeMounts[0])
}

func TestToPodWithTargetlessFileBasedConfig(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("nginx",
			Image("nginx"),
			WithConfig(
				ConfigSource("myconfig"),
			),
		),
		WithConfigObjConfig("myconfig", ConfigFile("./file.html")),
	))

	expectedVolume := apiv1.Volume{
		Name: "config-0",
		VolumeSource: apiv1.VolumeSource{
			ConfigMap: &apiv1.ConfigMapVolumeSource{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: "myconfig",
				},
				Items: []apiv1.KeyToPath{
					{
						Key:  "file.html",
						Path: "config-0",
					},
				},
			},
		},
	}

	expectedMount := apiv1.VolumeMount{
		Name:      "config-0",
		ReadOnly:  true,
		MountPath: "/myconfig",
		SubPath:   "config-0",
	}

	assert.Len(t, podTemplate.Spec.Volumes, 1)
	assert.Len(t, podTemplate.Spec.Containers[0].VolumeMounts, 1)
	assert.Equal(t, expectedVolume, podTemplate.Spec.Volumes[0])
	assert.Equal(t, expectedMount, podTemplate.Spec.Containers[0].VolumeMounts[0])
}

func TestToPodWithExternalConfig(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("nginx",
			Image("nginx"),
			WithConfig(
				ConfigSource("my_config"),
				ConfigTarget("/usr/share/nginx/html/index.html"),
				ConfigUID("103"),
				ConfigGID("103"),
				ConfigMode(0440),
			),
		),
		WithConfigObjConfig("my_config", ConfigExternal),
	))

	mode := int32(0440)

	expectedVolume := apiv1.Volume{
		Name: "config-0",
		VolumeSource: apiv1.VolumeSource{
			ConfigMap: &apiv1.ConfigMapVolumeSource{
				LocalObjectReference: apiv1.LocalObjectReference{
					Name: "my_config",
				},
				Items: []apiv1.KeyToPath{
					{
						Key:  "file", // TODO: This is the key we assume external config use
						Path: "config-0",
						Mode: &mode,
					},
				},
			},
		},
	}

	expectedMount := apiv1.VolumeMount{
		Name:      "config-0",
		ReadOnly:  true,
		MountPath: "/usr/share/nginx/html/index.html",
		SubPath:   "config-0",
	}

	assert.Len(t, podTemplate.Spec.Volumes, 1)
	assert.Len(t, podTemplate.Spec.Containers[0].VolumeMounts, 1)
	assert.Equal(t, expectedVolume, podTemplate.Spec.Volumes[0])
	assert.Equal(t, expectedMount, podTemplate.Spec.Containers[0].VolumeMounts[0])
}

func TestToPodWithTwoConfigsSameMountPoint(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("nginx",
			Image("nginx"),
			WithConfig(
				ConfigSource("first"),
				ConfigTarget("/data/first.json"),
				ConfigMode(0440),
			),
			WithConfig(
				ConfigSource("second"),
				ConfigTarget("/data/second.json"),
				ConfigMode(0550),
			),
		),
		WithConfigObjConfig("first", ConfigFile("./file1")),
		WithConfigObjConfig("second", ConfigFile("./file2")),
	))

	mode0440 := int32(0440)
	mode0550 := int32(0550)

	expectedVolumes := []apiv1.Volume{
		{
			Name: "config-0",
			VolumeSource: apiv1.VolumeSource{
				ConfigMap: &apiv1.ConfigMapVolumeSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: "first",
					},
					Items: []apiv1.KeyToPath{
						{
							Key:  "file1",
							Path: "config-0",
							Mode: &mode0440,
						},
					},
				},
			},
		},
		{
			Name: "config-1",
			VolumeSource: apiv1.VolumeSource{
				ConfigMap: &apiv1.ConfigMapVolumeSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: "second",
					},
					Items: []apiv1.KeyToPath{
						{
							Key:  "file2",
							Path: "config-1",
							Mode: &mode0550,
						},
					},
				},
			},
		},
	}

	expectedMounts := []apiv1.VolumeMount{
		{
			Name:      "config-0",
			ReadOnly:  true,
			MountPath: "/data/first.json",
			SubPath:   "config-0",
		},
		{
			Name:      "config-1",
			ReadOnly:  true,
			MountPath: "/data/second.json",
			SubPath:   "config-1",
		},
	}

	assert.Equal(t, expectedVolumes, podTemplate.Spec.Volumes)
	assert.Equal(t, expectedMounts, podTemplate.Spec.Containers[0].VolumeMounts)
}

func TestToPodWithTwoExternalConfigsSameMountPoint(t *testing.T) {
	podTemplate := podTemplate(t, Stack("demo",
		WithService("nginx",
			Image("nginx"),
			WithConfig(
				ConfigSource("first"),
				ConfigTarget("/data/first.json"),
			),
			WithConfig(
				ConfigSource("second"),
				ConfigTarget("/data/second.json"),
			),
		),
		WithConfigObjConfig("first", ConfigExternal),
		WithConfigObjConfig("second", ConfigExternal),
	))

	expectedVolumes := []apiv1.Volume{
		{
			Name: "config-0",
			VolumeSource: apiv1.VolumeSource{
				ConfigMap: &apiv1.ConfigMapVolumeSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: "first",
					},
					Items: []apiv1.KeyToPath{
						{
							Key:  "file",
							Path: "config-0",
						},
					},
				},
			},
		},
		{
			Name: "config-1",
			VolumeSource: apiv1.VolumeSource{
				ConfigMap: &apiv1.ConfigMapVolumeSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: "second",
					},
					Items: []apiv1.KeyToPath{
						{
							Key:  "file",
							Path: "config-1",
						},
					},
				},
			},
		},
	}

	expectedMounts := []apiv1.VolumeMount{
		{
			Name:      "config-0",
			ReadOnly:  true,
			MountPath: "/data/first.json",
			SubPath:   "config-0",
		},
		{
			Name:      "config-1",
			ReadOnly:  true,
			MountPath: "/data/second.json",
			SubPath:   "config-1",
		},
	}

	assert.Equal(t, expectedVolumes, podTemplate.Spec.Volumes)
	assert.Equal(t, expectedMounts, podTemplate.Spec.Containers[0].VolumeMounts)
}

func TestToPodWithPullSecret(t *testing.T) {
	podTemplateWithSecret := podTemplate(t, Stack("demo",
		WithService("nginx",
			Image("nginx"),
			PullSecret("test-pull-secret"),
		)))
	assert.Equal(t, 1, len(podTemplateWithSecret.Spec.ImagePullSecrets))
	assert.Equal(t, "test-pull-secret", podTemplateWithSecret.Spec.ImagePullSecrets[0].Name)
	podTemplateNoSecret := podTemplate(t, Stack("demo",
		WithService("nginx",
			Image("nginx"),
		)))
	assert.Nil(t, podTemplateNoSecret.Spec.ImagePullSecrets)
}

func TestToPodWithPullPolicy(t *testing.T) {
	cases := []struct {
		name           string
		stack          *latest.Stack
		expectedPolicy apiv1.PullPolicy
		expectedError  string
	}{
		{
			name: "specific tag",
			stack: Stack("demo",
				WithService("nginx",
					Image("nginx:specific"),
				)),
			expectedPolicy: apiv1.PullIfNotPresent,
		},
		{
			name: "latest tag",
			stack: Stack("demo",
				WithService("nginx",
					Image("nginx:latest"),
				)),
			expectedPolicy: apiv1.PullAlways,
		},
		{
			name: "explicit policy",
			stack: Stack("demo",
				WithService("nginx",
					Image("nginx:latest"),
					PullPolicy("Never"),
				)),
			expectedPolicy: apiv1.PullNever,
		},
		{
			name: "invalid policy",
			stack: Stack("demo",
				WithService("nginx",
					Image("nginx:latest"),
					PullPolicy("Invalid"),
				)),
			expectedError: `invalid pull policy "Invalid", must be "Always", "IfNotPresent" or "Never"`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pod, err := podTemplateWithError(c.stack)
			if c.expectedError != "" {
				assert.EqualError(t, err, c.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, pod.Spec.Containers[0].ImagePullPolicy, c.expectedPolicy)
			}
		})
	}
}
