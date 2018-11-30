package convert

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/docker/compose-on-kubernetes/api/compose/v1beta2"
	"github.com/docker/docker/api/types/swarm"
	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func toPodTemplate(serviceConfig v1beta2.ServiceConfig, labels map[string]string, configuration *v1beta2.StackSpec, original apiv1.PodTemplateSpec) (apiv1.PodTemplateSpec, error) {
	tpl := *original.DeepCopy()
	nodeAffinity, err := toNodeAffinity(serviceConfig.Deploy.Placement.Constraints)
	if err != nil {
		return apiv1.PodTemplateSpec{}, err
	}
	hostAliases, err := toHostAliases(serviceConfig.ExtraHosts)
	if err != nil {
		return apiv1.PodTemplateSpec{}, err
	}
	env, err := toEnv(serviceConfig.Environment)
	if err != nil {
		return apiv1.PodTemplateSpec{}, err
	}
	restartPolicy, err := toRestartPolicy(serviceConfig, tpl.Spec.RestartPolicy)
	if err != nil {
		return apiv1.PodTemplateSpec{}, err
	}
	limits, err := toResource(serviceConfig.Deploy.Resources.Limits)
	if err != nil {
		return apiv1.PodTemplateSpec{}, err
	}
	requests, err := toResource(serviceConfig.Deploy.Resources.Reservations)
	if err != nil {
		return apiv1.PodTemplateSpec{}, err
	}
	volumes, err := toVolumes(serviceConfig, configuration)
	if err != nil {
		return apiv1.PodTemplateSpec{}, err
	}
	volumeMounts, err := toVolumeMounts(serviceConfig, configuration)
	if err != nil {
		return apiv1.PodTemplateSpec{}, err
	}
	tpl.ObjectMeta = metav1.ObjectMeta{
		Labels:      labels,
		Annotations: serviceConfig.Labels,
	}
	tpl.Spec.RestartPolicy = restartPolicy
	tpl.Spec.Volumes = volumes
	tpl.Spec.HostPID = toHostPID(serviceConfig.Pid)
	tpl.Spec.HostIPC = toHostIPC(serviceConfig.Ipc)
	tpl.Spec.Hostname = serviceConfig.Hostname
	tpl.Spec.TerminationGracePeriodSeconds = toTerminationGracePeriodSeconds(serviceConfig.StopGracePeriod, tpl.Spec.TerminationGracePeriodSeconds)
	tpl.Spec.HostAliases = hostAliases
	tpl.Spec.Affinity = nodeAffinity
	// we dont want to remove all containers and recreate them because:
	// an admission plugin can add sidecar containers
	// we for sure want to keep the main container to be additive
	if len(tpl.Spec.Containers) == 0 {
		tpl.Spec.Containers = []apiv1.Container{{}}
	}
	containerIX := 0
	for ix, c := range tpl.Spec.Containers {
		if c.Name == serviceConfig.Name {
			containerIX = ix
			break
		}
	}
	tpl.Spec.Containers[containerIX].Name = serviceConfig.Name
	tpl.Spec.Containers[containerIX].Image = serviceConfig.Image
	tpl.Spec.Containers[containerIX].ImagePullPolicy = toImagePullPolicy(serviceConfig.Image)
	tpl.Spec.Containers[containerIX].Command = serviceConfig.Entrypoint
	tpl.Spec.Containers[containerIX].Args = serviceConfig.Command
	tpl.Spec.Containers[containerIX].WorkingDir = serviceConfig.WorkingDir
	tpl.Spec.Containers[containerIX].TTY = serviceConfig.Tty
	tpl.Spec.Containers[containerIX].Stdin = serviceConfig.StdinOpen
	tpl.Spec.Containers[containerIX].Ports = toPorts(serviceConfig.Ports)
	tpl.Spec.Containers[containerIX].LivenessProbe = toLivenessProbe(serviceConfig.HealthCheck)
	tpl.Spec.Containers[containerIX].Env = env
	tpl.Spec.Containers[containerIX].VolumeMounts = volumeMounts
	tpl.Spec.Containers[containerIX].SecurityContext = toSecurityContext(serviceConfig)
	tpl.Spec.Containers[containerIX].Resources = apiv1.ResourceRequirements{
		Limits:   limits,
		Requests: requests,
	}
	return tpl, nil
}

func toImagePullPolicy(image string) apiv1.PullPolicy {
	if strings.HasSuffix(image, ":latest") {
		return apiv1.PullAlways
	}
	return apiv1.PullIfNotPresent
}

func toHostAliases(extraHosts []string) ([]apiv1.HostAlias, error) {
	if extraHosts == nil {
		return nil, nil
	}

	byHostnames := map[string]string{}
	for _, host := range extraHosts {
		split := strings.SplitN(host, ":", 2)
		if len(split) != 2 {
			return nil, errors.Errorf("malformed host %s", host)
		}
		byHostnames[split[0]] = split[1]
	}

	byIPs := map[string][]string{}
	for k, v := range byHostnames {
		byIPs[v] = append(byIPs[v], k)
	}

	aliases := make([]apiv1.HostAlias, len(byIPs))
	i := 0
	for key, hosts := range byIPs {
		sort.Strings(hosts)
		aliases[i] = apiv1.HostAlias{
			IP:        key,
			Hostnames: hosts,
		}
		i++
	}
	sort.Slice(aliases, func(i, j int) bool { return aliases[i].IP < aliases[j].IP })
	return aliases, nil
}

func toHostPID(pid string) bool {
	return "host" == pid
}

func toHostIPC(ipc string) bool {
	return "host" == ipc
}

func toTerminationGracePeriodSeconds(duration *time.Duration, original *int64) *int64 {
	if duration == nil {
		return original
	}
	gracePeriod := int64(duration.Seconds())
	return &gracePeriod
}

func toLivenessProbe(hc *v1beta2.HealthCheckConfig) *apiv1.Probe {
	if hc == nil || len(hc.Test) < 1 || hc.Test[0] == "NONE" {
		return nil
	}

	command := hc.Test[1:]
	if hc.Test[0] == "CMD-SHELL" {
		command = append([]string{"sh", "-c"}, command...)
	}

	return &apiv1.Probe{
		TimeoutSeconds:   toSecondsOrDefault(hc.Timeout, 1),
		PeriodSeconds:    toSecondsOrDefault(hc.Interval, 1),
		FailureThreshold: int32(defaultUint64(hc.Retries, 3)),
		Handler: apiv1.Handler{
			Exec: &apiv1.ExecAction{
				Command: command,
			},
		},
	}
}

func toEnv(env map[string]*string) ([]apiv1.EnvVar, error) {
	var envVars []apiv1.EnvVar

	for k, v := range env {
		if v == nil {
			return nil, errors.Errorf("%s has no value, unsetting an environment variable is not supported", k)
		}
		envVars = append(envVars, toEnvVar(k, *v))
	}
	sort.Slice(envVars, func(i, j int) bool { return envVars[i].Name < envVars[j].Name })
	return envVars, nil
}

func toEnvVar(key, value string) apiv1.EnvVar {
	return apiv1.EnvVar{
		Name:  key,
		Value: value,
	}
}

func toPorts(list []v1beta2.ServicePortConfig) []apiv1.ContainerPort {
	var ports []apiv1.ContainerPort

	for _, v := range list {
		ports = append(ports, apiv1.ContainerPort{
			ContainerPort: int32(v.Target),
			Protocol:      toProtocol(v.Protocol),
		})
	}

	return ports
}

func toProtocol(value string) apiv1.Protocol {
	if value == "udp" {
		return apiv1.ProtocolUDP
	}
	return apiv1.ProtocolTCP
}

func toRestartPolicy(s v1beta2.ServiceConfig, original apiv1.RestartPolicy) (apiv1.RestartPolicy, error) {
	policy := s.Deploy.RestartPolicy
	if policy == nil {
		return original, nil
	}

	switch policy.Condition {
	case string(swarm.RestartPolicyConditionAny):
		return apiv1.RestartPolicyAlways, nil
	case string(swarm.RestartPolicyConditionNone):
		return apiv1.RestartPolicyNever, nil
	case string(swarm.RestartPolicyConditionOnFailure):
		return apiv1.RestartPolicyOnFailure, nil
	default:
		return "", errors.Errorf("unsupported restart policy %s", policy.Condition)
	}
}

func toResource(res *v1beta2.Resource) (apiv1.ResourceList, error) {
	if res == nil {
		return nil, nil
	}
	list := make(apiv1.ResourceList)
	if res.NanoCPUs != "" {
		cpus, err := resource.ParseQuantity(res.NanoCPUs)
		if err != nil {
			return nil, err
		}
		list[apiv1.ResourceCPU] = cpus
	}
	if res.MemoryBytes != 0 {
		memory, err := resource.ParseQuantity(fmt.Sprintf("%v", res.MemoryBytes))
		if err != nil {
			return nil, err
		}
		list[apiv1.ResourceMemory] = memory
	}
	return list, nil
}

func toSecurityContext(s v1beta2.ServiceConfig) *apiv1.SecurityContext {
	isPrivileged := toBoolPointer(s.Privileged)
	isReadOnly := toBoolPointer(s.ReadOnly)

	var capabilities *apiv1.Capabilities
	if s.CapAdd != nil || s.CapDrop != nil {
		capabilities = &apiv1.Capabilities{
			Add:  toCapabilities(s.CapAdd),
			Drop: toCapabilities(s.CapDrop),
		}
	}

	if isPrivileged == nil && isReadOnly == nil && capabilities == nil && s.User == nil {
		return nil
	}

	return &apiv1.SecurityContext{
		RunAsUser:              s.User,
		Privileged:             isPrivileged,
		ReadOnlyRootFilesystem: isReadOnly,
		Capabilities:           capabilities,
	}
}

func toBoolPointer(value bool) *bool {
	if value {
		return &value
	}

	return nil
}

func defaultUint64(v *uint64, defaultValue uint64) uint64 { //nolint: unparam
	if v == nil {
		return defaultValue
	}

	return *v
}

func toCapabilities(list []string) (capabilities []apiv1.Capability) {
	for _, c := range list {
		capabilities = append(capabilities, apiv1.Capability(c))
	}
	return
}

//nolint: unparam
func forceRestartPolicy(podTemplate apiv1.PodTemplateSpec, forcedRestartPolicy apiv1.RestartPolicy) apiv1.PodTemplateSpec {
	if podTemplate.Spec.RestartPolicy != "" {
		podTemplate.Spec.RestartPolicy = forcedRestartPolicy
	}

	return podTemplate
}
