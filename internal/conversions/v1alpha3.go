package conversions

import (
	"regexp"
	"strconv"
	"strings"

	composetypes "github.com/docker/cli/cli/compose/types"
	"github.com/docker/compose-on-kubernetes/api/compose/v1alpha3"
	"github.com/docker/compose-on-kubernetes/internal/internalversion"
	"github.com/docker/compose-on-kubernetes/internal/parsing"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
)

// RegisterV1alpha3Conversions adds conversion functions to the given scheme.
// Public to allow building arbitrary schemes.
func RegisterV1alpha3Conversions(scheme *runtime.Scheme) error {
	return scheme.AddConversionFuncs(
		ownerToInternalV1alpha3,
		ownerFromInternalV1alpha3,
		stackToInternalV1alpha3,
		stackFromInternalV1alpha3,
		stackListToInternalV1alpha3,
		stackListFromInternalV1alpha3,
	)
}

func ownerToInternalV1alpha3(in *v1alpha3.Owner, out *internalversion.Owner, _ conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.Owner = in.Owner
	return nil
}

func ownerFromInternalV1alpha3(in *internalversion.Owner, out *v1alpha3.Owner, _ conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.Owner = in.Owner
	return nil
}

func stackToInternalV1alpha3(in *v1alpha3.Stack, out *internalversion.Stack, _ conversion.Scope) error {
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

func stackFromInternalV1alpha3(in *internalversion.Stack, out *v1alpha3.Stack, _ conversion.Scope) error {
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
		out.Status = &v1alpha3.StackStatus{
			Message: in.Status.Message,
			Phase:   v1alpha3.StackPhase(in.Status.Phase),
		}
	} else {
		out.Status = nil
	}
	return nil
}

// FromComposeConfig converts a cli/compose/types.Config to a v1alpha3 Config struct.
// Use it only for backward compat between v1beta1 and later.
func FromComposeConfig(c *composetypes.Config) *v1alpha3.StackSpec {
	if c == nil {
		return nil
	}
	serviceConfigs := make([]v1alpha3.ServiceConfig, len(c.Services))
	for i, s := range c.Services {
		serviceConfigs[i] = fromComposeServiceConfig(s)
	}
	return &v1alpha3.StackSpec{
		Services: serviceConfigs,
		Secrets:  fromComposeSecrets(c.Secrets),
		Configs:  fromComposeConfigs(c.Configs),
	}
}

func fromComposeSecrets(s map[string]composetypes.SecretConfig) map[string]v1alpha3.SecretConfig {
	if s == nil {
		return nil
	}
	m := map[string]v1alpha3.SecretConfig{}
	for key, value := range s {
		m[key] = v1alpha3.SecretConfig{
			Name: value.Name,
			File: value.File,
			External: v1alpha3.External{
				Name:     value.External.Name,
				External: value.External.External,
			},
			Labels: value.Labels,
		}
	}
	return m
}

func fromComposeConfigs(s map[string]composetypes.ConfigObjConfig) map[string]v1alpha3.ConfigObjConfig {
	if s == nil {
		return nil
	}
	m := map[string]v1alpha3.ConfigObjConfig{}
	for key, value := range s {
		m[key] = v1alpha3.ConfigObjConfig{
			Name: value.Name,
			File: value.File,
			External: v1alpha3.External{
				Name:     value.External.Name,
				External: value.External.External,
			},
			Labels: value.Labels,
		}
	}
	return m
}

func fromComposeServiceConfig(s composetypes.ServiceConfig) v1alpha3.ServiceConfig {
	var userID *int64
	if s.User != "" {
		numerical, err := strconv.Atoi(s.User)
		if err == nil {
			unixUserID := int64(numerical)
			userID = &unixUserID
		}
	}
	return v1alpha3.ServiceConfig{
		Name:    s.Name,
		CapAdd:  s.CapAdd,
		CapDrop: s.CapDrop,
		Command: s.Command,
		Configs: fromComposeServiceConfigs(s.Configs),
		Deploy: v1alpha3.DeployConfig{
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
		StopGracePeriod: composetypes.ConvertDurationPtr(s.StopGracePeriod),
		Tmpfs:           s.Tmpfs,
		Tty:             s.Tty,
		User:            userID,
		Volumes:         fromComposeServiceVolumeConfig(s.Volumes),
		WorkingDir:      s.WorkingDir,
	}
}

func fromComposePorts(ports []composetypes.ServicePortConfig) []v1alpha3.ServicePortConfig {
	if ports == nil {
		return nil
	}
	p := make([]v1alpha3.ServicePortConfig, len(ports))
	for i, port := range ports {
		p[i] = v1alpha3.ServicePortConfig{
			Mode:      port.Mode,
			Target:    port.Target,
			Published: port.Published,
			Protocol:  port.Protocol,
		}
	}
	return p
}

func fromComposeServiceSecrets(secrets []composetypes.ServiceSecretConfig) []v1alpha3.ServiceSecretConfig {
	if secrets == nil {
		return nil
	}
	c := make([]v1alpha3.ServiceSecretConfig, len(secrets))
	for i, secret := range secrets {
		c[i] = v1alpha3.ServiceSecretConfig{
			Source: secret.Source,
			Target: secret.Target,
			UID:    secret.UID,
			Mode:   secret.Mode,
		}
	}
	return c
}

func fromComposeServiceConfigs(configs []composetypes.ServiceConfigObjConfig) []v1alpha3.ServiceConfigObjConfig {
	if configs == nil {
		return nil
	}
	c := make([]v1alpha3.ServiceConfigObjConfig, len(configs))
	for i, config := range configs {
		c[i] = v1alpha3.ServiceConfigObjConfig{
			Source: config.Source,
			Target: config.Target,
			UID:    config.UID,
			Mode:   config.Mode,
		}
	}
	return c
}

func fromComposeHealthcheck(h *composetypes.HealthCheckConfig) *v1alpha3.HealthCheckConfig {
	if h == nil {
		return nil
	}
	return &v1alpha3.HealthCheckConfig{
		Test:     h.Test,
		Timeout:  composetypes.ConvertDurationPtr(h.Timeout),
		Interval: composetypes.ConvertDurationPtr(h.Interval),
		Retries:  h.Retries,
	}
}

func fromComposePlacement(p composetypes.Placement) v1alpha3.Placement {
	return v1alpha3.Placement{
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

func fromComposeConstraints(s []string) *v1alpha3.Constraints {
	if len(s) == 0 {
		return nil
	}
	constraints := &v1alpha3.Constraints{}
	for _, constraint := range s {
		matches := constraintEquals.FindStringSubmatch(constraint)
		if len(matches) == 4 {
			key := matches[1]
			operator := matches[2]
			value := matches[3]
			constraint := &v1alpha3.Constraint{
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
					constraints.MatchLabels = map[string]v1alpha3.Constraint{}
				}
				constraints.MatchLabels[strings.TrimPrefix(key, swarmLabelPrefix)] = *constraint
			}
		}
	}
	return constraints
}

func fromComposeResources(r composetypes.Resources) v1alpha3.Resources {
	return v1alpha3.Resources{
		Limits:       fromComposeResourcesResource(r.Limits),
		Reservations: fromComposeResourcesResource(r.Reservations),
	}
}

func fromComposeResourcesResource(r *composetypes.Resource) *v1alpha3.Resource {
	if r == nil {
		return nil
	}
	return &v1alpha3.Resource{
		MemoryBytes: int64(r.MemoryBytes),
		NanoCPUs:    r.NanoCPUs,
	}
}

func fromComposeUpdateConfig(u *composetypes.UpdateConfig) *v1alpha3.UpdateConfig {
	if u == nil {
		return nil
	}
	return &v1alpha3.UpdateConfig{
		Parallelism: u.Parallelism,
	}
}

func fromComposeRestartPolicy(r *composetypes.RestartPolicy) *v1alpha3.RestartPolicy {
	if r == nil {
		return nil
	}
	return &v1alpha3.RestartPolicy{
		Condition: r.Condition,
	}
}

func fromComposeServiceVolumeConfig(vs []composetypes.ServiceVolumeConfig) []v1alpha3.ServiceVolumeConfig {
	if vs == nil {
		return nil
	}
	volumes := []v1alpha3.ServiceVolumeConfig{}
	for _, v := range vs {
		volumes = append(volumes, v1alpha3.ServiceVolumeConfig{
			Type:     v.Type,
			Source:   v.Source,
			Target:   v.Target,
			ReadOnly: v.ReadOnly,
		})
	}
	return volumes
}

// StackFromInternalV1alpha3 converts an internal stack to v1alpha3 flavor
func StackFromInternalV1alpha3(in *internalversion.Stack) (*v1alpha3.Stack, error) {
	if in == nil {
		return nil, nil
	}
	out := new(v1alpha3.Stack)
	err := stackFromInternalV1alpha3(in, out, nil)
	return out, err
}

func stackListToInternalV1alpha3(in *v1alpha3.StackList, out *internalversion.StackList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		inSlice, outSlice := &in.Items, &out.Items
		*outSlice = make([]internalversion.Stack, len(*inSlice))
		for i := range *inSlice {
			if err := stackToInternalV1alpha3(&(*inSlice)[i], &(*outSlice)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func stackListFromInternalV1alpha3(in *internalversion.StackList, out *v1alpha3.StackList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		inSlice, outSlice := &in.Items, &out.Items
		*outSlice = make([]v1alpha3.Stack, len(*inSlice))
		for i := range *inSlice {
			if err := stackFromInternalV1alpha3(&(*inSlice)[i], &(*outSlice)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = make([]v1alpha3.Stack, 0)
	}
	return nil
}
