package conversions

import (
	"regexp"
	"strconv"
	"strings"

	composetypes "github.com/docker/cli/cli/compose/types"
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta2"
	"github.com/docker/compose-on-kubernetes/internal/internalversion"
	"github.com/docker/compose-on-kubernetes/internal/parsing"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
)

// RegisterV1beta2Conversions adds conversion functions to the given scheme.
// Public to allow building arbitrary schemes.
func RegisterV1beta2Conversions(scheme *runtime.Scheme) error {
	return scheme.AddGeneratedConversionFuncs(
		ownerToInternalV1beta2,
		ownerFromInternalV1beta2,
		stackToInternalV1beta2,
		stackFromInternalV1beta2,
		stackListToInternalV1beta2,
		stackListFromInternalV1beta2,
	)
}

func ownerToInternalV1beta2(in *v1beta2.Owner, out *internalversion.Owner, _ conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.Owner = in.Owner
	return nil
}

func ownerFromInternalV1beta2(in *internalversion.Owner, out *v1beta2.Owner, _ conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.Owner = in.Owner
	return nil
}

func stackToInternalV1beta2(in *v1beta2.Stack, out *internalversion.Stack, _ conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.Spec.Stack = in.Spec
	if in.Status != nil {
		out.Status = &internalversion.StackStatus{
			Message: in.Status.Message,
			Phase:   internalversion.StackPhase(in.Status.Phase),
		}
	} else {
		out.Status = nil
	}
	return nil
}

func stackFromInternalV1beta2(in *internalversion.Stack, out *v1beta2.Stack, _ conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.Spec = in.Spec.Stack
	if in.Spec.Stack == nil && in.Spec.ComposeFile != "" {
		composeConfig, err := parsing.LoadStackData([]byte(in.Spec.ComposeFile), map[string]string{})
		if err == nil {
			return err
		}
		// This should only occur if old v1beta1-era objects are present in storage
		// In that case we need to provide a Spec
		config := FromComposeConfig(composeConfig)
		out.Spec = config
	}
	if in.Status != nil {
		out.Status = &v1beta2.StackStatus{
			Message: in.Status.Message,
			Phase:   v1beta2.StackPhase(in.Status.Phase),
		}
	} else {
		out.Status = nil
	}
	return nil
}

// FromComposeConfig converts a cli/compose/types.Config to a v1beta2 Config struct.
// Use it only for backward compat between v1beta1 and later.
func FromComposeConfig(c *composetypes.Config) *v1beta2.StackSpec {
	if c == nil {
		return nil
	}
	serviceConfigs := make([]v1beta2.ServiceConfig, len(c.Services))
	for i, s := range c.Services {
		serviceConfigs[i] = fromComposeServiceConfig(s)
	}
	return &v1beta2.StackSpec{
		Services: serviceConfigs,
		Secrets:  fromComposeSecrets(c.Secrets),
		Configs:  fromComposeConfigs(c.Configs),
	}
}

func fromComposeSecrets(s map[string]composetypes.SecretConfig) map[string]v1beta2.SecretConfig {
	if s == nil {
		return nil
	}
	m := map[string]v1beta2.SecretConfig{}
	for key, value := range s {
		m[key] = v1beta2.SecretConfig{
			Name: value.Name,
			File: value.File,
			External: v1beta2.External{
				Name:     value.External.Name,
				External: value.External.External,
			},
			Labels: value.Labels,
		}
	}
	return m
}

func fromComposeConfigs(s map[string]composetypes.ConfigObjConfig) map[string]v1beta2.ConfigObjConfig {
	if s == nil {
		return nil
	}
	m := map[string]v1beta2.ConfigObjConfig{}
	for key, value := range s {
		m[key] = v1beta2.ConfigObjConfig{
			Name: value.Name,
			File: value.File,
			External: v1beta2.External{
				Name:     value.External.Name,
				External: value.External.External,
			},
			Labels: value.Labels,
		}
	}
	return m
}

func fromComposeServiceConfig(s composetypes.ServiceConfig) v1beta2.ServiceConfig {
	var userID *int64
	if s.User != "" {
		numerical, err := strconv.Atoi(s.User)
		if err == nil {
			unixUserID := int64(numerical)
			userID = &unixUserID
		}
	}
	return v1beta2.ServiceConfig{
		Name:    s.Name,
		CapAdd:  s.CapAdd,
		CapDrop: s.CapDrop,
		Command: s.Command,
		Configs: fromComposeServiceConfigs(s.Configs),
		Deploy: v1beta2.DeployConfig{
			Mode:          s.Deploy.Mode,
			Replicas:      s.Deploy.Replicas,
			Labels:        s.Deploy.Labels,
			UpdateConfig:  fromComposeUpdateConfig(s.Deploy.UpdateConfig),
			Resources:     fromComposeResources(s.Deploy.Resources),
			RestartPolicy: fromComposeRestartPolicy(s.Deploy.RestartPolicy),
			Placement:     fromComposePlacement(s.Deploy.Placement),
		},
		Entrypoint:      s.Entrypoint,
		Environment:     s.Environment,
		ExtraHosts:      s.ExtraHosts,
		Hostname:        s.Hostname,
		HealthCheck:     fromComposeHealthcheck(s.HealthCheck),
		Image:           s.Image,
		Ipc:             s.Ipc,
		Labels:          s.Labels,
		Pid:             s.Pid,
		Ports:           fromComposePorts(s.Ports),
		Privileged:      s.Privileged,
		ReadOnly:        s.ReadOnly,
		Secrets:         fromComposeServiceSecrets(s.Secrets),
		StdinOpen:       s.StdinOpen,
		StopGracePeriod: s.StopGracePeriod,
		Tmpfs:           s.Tmpfs,
		Tty:             s.Tty,
		User:            userID,
		Volumes:         fromComposeServiceVolumeConfig(s.Volumes),
		WorkingDir:      s.WorkingDir,
	}
}

func fromComposePorts(ports []composetypes.ServicePortConfig) []v1beta2.ServicePortConfig {
	if ports == nil {
		return nil
	}
	p := make([]v1beta2.ServicePortConfig, len(ports))
	for i, port := range ports {
		p[i] = v1beta2.ServicePortConfig{
			Mode:      port.Mode,
			Target:    port.Target,
			Published: port.Published,
			Protocol:  port.Protocol,
		}
	}
	return p
}

func fromComposeServiceSecrets(secrets []composetypes.ServiceSecretConfig) []v1beta2.ServiceSecretConfig {
	if secrets == nil {
		return nil
	}
	c := make([]v1beta2.ServiceSecretConfig, len(secrets))
	for i, secret := range secrets {
		c[i] = v1beta2.ServiceSecretConfig{
			Source: secret.Source,
			Target: secret.Target,
			UID:    secret.UID,
			Mode:   secret.Mode,
		}
	}
	return c
}

func fromComposeServiceConfigs(configs []composetypes.ServiceConfigObjConfig) []v1beta2.ServiceConfigObjConfig {
	if configs == nil {
		return nil
	}
	c := make([]v1beta2.ServiceConfigObjConfig, len(configs))
	for i, config := range configs {
		c[i] = v1beta2.ServiceConfigObjConfig{
			Source: config.Source,
			Target: config.Target,
			UID:    config.UID,
			Mode:   config.Mode,
		}
	}
	return c
}

func fromComposeHealthcheck(h *composetypes.HealthCheckConfig) *v1beta2.HealthCheckConfig {
	if h == nil {
		return nil
	}
	return &v1beta2.HealthCheckConfig{
		Test:     h.Test,
		Timeout:  h.Timeout,
		Interval: h.Interval,
		Retries:  h.Retries,
	}
}

func fromComposePlacement(p composetypes.Placement) v1beta2.Placement {
	return v1beta2.Placement{
		Constraints: fromComposeConstraints(p.Constraints),
	}
}

var constraintEquals = regexp.MustCompile(`([\w\.]*)\W*(==|!=)\W*([\w\.]*)`)

const (
	swarmOs          = "node.platform.os"
	swarmArch        = "node.platform.arch"
	swarmHostname    = "node.hostname"
	swarmLabelPrefix = "node.labels."
)

func fromComposeConstraints(s []string) *v1beta2.Constraints {
	if len(s) == 0 {
		return nil
	}
	constraints := &v1beta2.Constraints{}
	for _, constraint := range s {
		matches := constraintEquals.FindStringSubmatch(constraint)
		if len(matches) == 4 {
			key := matches[1]
			operator := matches[2]
			value := matches[3]
			constraint := &v1beta2.Constraint{
				Operator: operator,
				Value:    value,
			}
			switch {
			case key == swarmOs:
				constraints.OperatingSystem = constraint
			case key == swarmArch:
				constraints.Architecture = constraint
			case key == swarmHostname:
				constraints.Hostname = constraint
			case strings.HasPrefix(key, swarmLabelPrefix):
				if constraints.MatchLabels == nil {
					constraints.MatchLabels = map[string]v1beta2.Constraint{}
				}
				constraints.MatchLabels[strings.TrimPrefix(key, swarmLabelPrefix)] = *constraint
			}
		}
	}
	return constraints
}

func fromComposeResources(r composetypes.Resources) v1beta2.Resources {
	return v1beta2.Resources{
		Limits:       fromComposeResourcesResource(r.Limits),
		Reservations: fromComposeResourcesResource(r.Reservations),
	}
}

func fromComposeResourcesResource(r *composetypes.Resource) *v1beta2.Resource {
	if r == nil {
		return nil
	}
	return &v1beta2.Resource{
		MemoryBytes: int64(r.MemoryBytes),
		NanoCPUs:    r.NanoCPUs,
	}
}

func fromComposeUpdateConfig(u *composetypes.UpdateConfig) *v1beta2.UpdateConfig {
	if u == nil {
		return nil
	}
	return &v1beta2.UpdateConfig{
		Parallelism: u.Parallelism,
	}
}

func fromComposeRestartPolicy(r *composetypes.RestartPolicy) *v1beta2.RestartPolicy {
	if r == nil {
		return nil
	}
	return &v1beta2.RestartPolicy{
		Condition: r.Condition,
	}
}

func fromComposeServiceVolumeConfig(vs []composetypes.ServiceVolumeConfig) []v1beta2.ServiceVolumeConfig {
	if vs == nil {
		return nil
	}
	volumes := []v1beta2.ServiceVolumeConfig{}
	for _, v := range vs {
		volumes = append(volumes, v1beta2.ServiceVolumeConfig{
			Type:     v.Type,
			Source:   v.Source,
			Target:   v.Target,
			ReadOnly: v.ReadOnly,
		})
	}
	return volumes
}

// StackFromInternalV1beta2 converts an internal stack to v1beta2 flavor
func StackFromInternalV1beta2(in *internalversion.Stack) (*v1beta2.Stack, error) {
	if in == nil {
		return nil, nil
	}
	out := new(v1beta2.Stack)
	err := stackFromInternalV1beta2(in, out, nil)
	return out, err
}

func stackListToInternalV1beta2(in *v1beta2.StackList, out *internalversion.StackList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		inSlice, outSlice := &in.Items, &out.Items
		*outSlice = make([]internalversion.Stack, len(*inSlice))
		for i := range *inSlice {
			if err := stackToInternalV1beta2(&(*inSlice)[i], &(*outSlice)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func stackListFromInternalV1beta2(in *internalversion.StackList, out *v1beta2.StackList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		inSlice, outSlice := &in.Items, &out.Items
		*outSlice = make([]v1beta2.Stack, len(*inSlice))
		for i := range *inSlice {
			if err := stackFromInternalV1beta2(&(*inSlice)[i], &(*outSlice)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = make([]v1beta2.Stack, 0)
	}
	return nil
}
