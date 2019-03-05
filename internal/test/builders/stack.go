package builders

import (
	"time"

	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Stack is a builder to create an Stack
func Stack(name string, builders ...func(*latest.Stack)) *latest.Stack {
	stack := &latest.Stack{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: &latest.StackSpec{},
	}

	for _, builder := range builders {
		builder(stack)
	}

	return stack
}

// WithNamespace set the namespace of the stack
func WithNamespace(namespace string) func(*latest.Stack) {
	return func(s *latest.Stack) {
		s.Namespace = namespace
	}
}

// WithSecretConfig add a SecretConfig to to stack
func WithSecretConfig(name string, builders ...func(*latest.SecretConfig)) func(*latest.Stack) {
	return func(s *latest.Stack) {
		secret := &latest.SecretConfig{
			Name: name,
		}

		for _, builder := range builders {
			builder(secret)
		}

		if s.Spec.Secrets == nil {
			s.Spec.Secrets = map[string]latest.SecretConfig{}
		}
		s.Spec.Secrets[name] = *secret
	}
}

// SecretFile specifies the path of a secret
func SecretFile(path string) func(*latest.SecretConfig) {
	return func(s *latest.SecretConfig) {
		s.File = path
	}
}

// WithConfigObjConfig add a ConfigConfig to to stack
func WithConfigObjConfig(name string, builders ...func(*latest.ConfigObjConfig)) func(*latest.Stack) {
	return func(s *latest.Stack) {
		secret := &latest.ConfigObjConfig{
			Name: name,
		}

		for _, builder := range builders {
			builder(secret)
		}

		if s.Spec.Configs == nil {
			s.Spec.Configs = map[string]latest.ConfigObjConfig{}
		}
		s.Spec.Configs[name] = *secret
	}
}

// ConfigFile specifies the path of a config
func ConfigFile(path string) func(*latest.ConfigObjConfig) {
	return func(c *latest.ConfigObjConfig) {
		c.File = path
	}
}

// ConfigExternal specifies that the config is external
func ConfigExternal(c *latest.ConfigObjConfig) {
	c.External = latest.External{
		External: true,
	}
}

// WithService adds a ServiceConifg to the stack
func WithService(name string, builders ...func(*latest.ServiceConfig)) func(*latest.Stack) {
	return func(s *latest.Stack) {
		service := &latest.ServiceConfig{
			Name:  name,
			Image: "busybox:latest",
		}

		for _, builder := range builders {
			builder(service)
		}

		if s.Spec.Services == nil {
			s.Spec.Services = []latest.ServiceConfig{}
		}
		s.Spec.Services = append(s.Spec.Services, *service)
	}
}

// Image specfies the image of the service
func Image(reference string) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		c.Image = reference
	}
}

// PullSecret specifies the name of the pull secret used for this service
func PullSecret(name string) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		c.PullSecret = name
	}
}

// PullPolicy specifies the pull policy used for this service
func PullPolicy(policy string) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		c.PullPolicy = policy
	}
}

// StopGracePeriod specifies the stop-grace-period duration of a service
func StopGracePeriod(duration time.Duration) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		c.StopGracePeriod = &duration
	}
}

// User specifies the user of a service
func User(user int64) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		c.User = &user
	}
}

// WithTmpFS adds a path to the tmpfs of a service
func WithTmpFS(path string) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		if c.Tmpfs == nil {
			c.Tmpfs = []string{}
		}
		c.Tmpfs = append(c.Tmpfs, path)
	}
}

// WithLabel adds a label to a service
func WithLabel(key, value string) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		if c.Labels == nil {
			c.Labels = map[string]string{}
		}
		c.Labels[key] = value
	}
}

// IPC sets the ipc mode of the service
func IPC(mode string) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		c.Ipc = mode
	}
}

// PID sets the pid mode of the service
func PID(mode string) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		c.Pid = mode
	}
}

// Hostname sets the hostname of the service
func Hostname(hostname string) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		c.Hostname = hostname
	}
}

// WithExtraHost adds an extra host to the service
func WithExtraHost(host string) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		if c.ExtraHosts == nil {
			c.ExtraHosts = []string{}
		}
		c.ExtraHosts = append(c.ExtraHosts, host)
	}
}

// WithVolume adds a volume to the service
func WithVolume(builders ...func(*latest.ServiceVolumeConfig)) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		if c.Volumes == nil {
			c.Volumes = []latest.ServiceVolumeConfig{}
		}

		volume := &latest.ServiceVolumeConfig{}

		for _, builder := range builders {
			builder(volume)
		}

		c.Volumes = append(c.Volumes, *volume)
	}
}

// Source sets the volume source
func Source(source string) func(*latest.ServiceVolumeConfig) {
	return func(v *latest.ServiceVolumeConfig) {
		v.Source = source
	}
}

// Target sets the volume target
func Target(target string) func(*latest.ServiceVolumeConfig) {
	return func(v *latest.ServiceVolumeConfig) {
		v.Target = target
	}
}

// Volume sets the volume type to volume
func Volume(v *latest.ServiceVolumeConfig) {
	v.Type = "volume"
}

// Mount sets the volume type to mount
func Mount(v *latest.ServiceVolumeConfig) {
	v.Type = "mount"
}

// VolumeReadOnly sets the volume to readonly
func VolumeReadOnly(v *latest.ServiceVolumeConfig) {
	v.ReadOnly = true
}

// Healthcheck sets the healthcheck config of the service
func Healthcheck(builders ...func(*latest.HealthCheckConfig)) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		healthcheck := &latest.HealthCheckConfig{}

		for _, builder := range builders {
			builder(healthcheck)
		}

		c.HealthCheck = healthcheck
	}
}

// Test sets the test commands of the healthcheck
func Test(cmd ...string) func(*latest.HealthCheckConfig) {
	return func(h *latest.HealthCheckConfig) {
		h.Test = cmd
	}
}

// Interval sets the interval duration of the healthcheck
func Interval(duration time.Duration) func(*latest.HealthCheckConfig) {
	return func(h *latest.HealthCheckConfig) {
		h.Interval = &duration
	}
}

// Timeout sets the timeout duration of the healthcheck
func Timeout(duration time.Duration) func(*latest.HealthCheckConfig) {
	return func(h *latest.HealthCheckConfig) {
		h.Timeout = &duration
	}
}

// Retries sets the number of retries of the healthcheck
func Retries(retries uint64) func(*latest.HealthCheckConfig) {
	return func(h *latest.HealthCheckConfig) {
		h.Retries = &retries
	}
}

// WithSecret adds a secret to the service
func WithSecret(builders ...func(*latest.ServiceSecretConfig)) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		if c.Secrets == nil {
			c.Secrets = []latest.ServiceSecretConfig{}
		}
		secret := &latest.ServiceSecretConfig{}
		for _, builder := range builders {
			builder(secret)
		}
		c.Secrets = append(c.Secrets, *secret)
	}
}

// SecretSource sets the source of the secret
func SecretSource(source string) func(*latest.ServiceSecretConfig) {
	return func(s *latest.ServiceSecretConfig) {
		s.Source = source
	}
}

// SecretTarget sets the target of the secret
func SecretTarget(target string) func(*latest.ServiceSecretConfig) {
	return func(s *latest.ServiceSecretConfig) {
		s.Target = target
	}
}

// WithConfig adds a config to the service
func WithConfig(builders ...func(*latest.ServiceConfigObjConfig)) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		if c.Configs == nil {
			c.Configs = []latest.ServiceConfigObjConfig{}
		}
		config := &latest.ServiceConfigObjConfig{}
		for _, builder := range builders {
			builder(config)
		}
		c.Configs = append(c.Configs, *config)
	}
}

// ConfigSource sets the source of the config
func ConfigSource(source string) func(*latest.ServiceConfigObjConfig) {
	return func(c *latest.ServiceConfigObjConfig) {
		c.Source = source
	}
}

// ConfigTarget sets the target of the config
func ConfigTarget(target string) func(*latest.ServiceConfigObjConfig) {
	return func(c *latest.ServiceConfigObjConfig) {
		c.Target = target
	}
}

// ConfigUID sets the uid of the config
func ConfigUID(uid string) func(*latest.ServiceConfigObjConfig) {
	return func(c *latest.ServiceConfigObjConfig) {
		c.UID = uid
	}
}

// ConfigGID sets the gid of the config
func ConfigGID(gid string) func(*latest.ServiceConfigObjConfig) {
	return func(c *latest.ServiceConfigObjConfig) {
		c.GID = gid
	}
}

// ConfigMode sets the mode of the config
func ConfigMode(mode uint32) func(*latest.ServiceConfigObjConfig) {
	return func(c *latest.ServiceConfigObjConfig) {
		c.Mode = &mode
	}
}

// Deploy sets the deploy config of the service
func Deploy(builders ...func(*latest.DeployConfig)) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		deploy := &latest.DeployConfig{}

		for _, builder := range builders {
			builder(deploy)
		}

		c.Deploy = *deploy
	}
}

// Resources sets the resources of the deploy config
func Resources(builders ...func(*latest.Resources)) func(*latest.DeployConfig) {
	return func(d *latest.DeployConfig) {
		resources := &latest.Resources{}

		for _, builder := range builders {
			builder(resources)
		}

		d.Resources = *resources
	}
}

// Limits sets the limits of the resources
func Limits(builders ...func(*latest.Resource)) func(*latest.Resources) {
	return func(r *latest.Resources) {
		limits := &latest.Resource{}

		for _, builder := range builders {
			builder(limits)
		}

		r.Limits = limits
	}
}

// Reservations sets the reservations of the resources
func Reservations(builders ...func(*latest.Resource)) func(*latest.Resources) {
	return func(r *latest.Resources) {
		reservations := &latest.Resource{}

		for _, builder := range builders {
			builder(reservations)
		}

		r.Reservations = reservations
	}
}

// CPUs sets the cup of the resource
func CPUs(cpus string) func(*latest.Resource) {
	return func(r *latest.Resource) {
		r.NanoCPUs = cpus
	}
}

// Memory sets the memory of the resource
func Memory(memory int64) func(*latest.Resource) {
	return func(r *latest.Resource) {
		r.MemoryBytes = memory
	}
}

// Update sets the update config of a deploy config
func Update(builders ...func(*latest.UpdateConfig)) func(*latest.DeployConfig) {
	return func(d *latest.DeployConfig) {
		update := &latest.UpdateConfig{}

		for _, builder := range builders {
			builder(update)
		}

		d.UpdateConfig = update
	}
}

// Parallelism sets the parallelism of the update config
func Parallelism(parallelism uint64) func(*latest.UpdateConfig) {
	return func(u *latest.UpdateConfig) {
		u.Parallelism = &parallelism
	}
}

// ModeGlobal sets the deploy mode to global
func ModeGlobal(d *latest.DeployConfig) {
	d.Mode = "global"
}

// Replicas sets the number of replicas of a deploy config
func Replicas(replicas uint64) func(*latest.DeployConfig) {
	return func(d *latest.DeployConfig) {
		d.Replicas = &replicas
	}
}

// WithDeployLabel adds a label to the deploy config
func WithDeployLabel(key, value string) func(*latest.DeployConfig) {
	return func(d *latest.DeployConfig) {
		if d.Labels == nil {
			d.Labels = map[string]string{}
		}
		d.Labels[key] = value
	}
}

// RestartPolicy sets the restart policy of the deploy config
func RestartPolicy(builders ...func(*latest.RestartPolicy)) func(*latest.DeployConfig) {
	return func(d *latest.DeployConfig) {
		rp := &latest.RestartPolicy{}

		for _, builder := range builders {
			builder(rp)
		}

		d.RestartPolicy = rp
	}
}

// OnFailure sets the restart policy to on-failure
func OnFailure(r *latest.RestartPolicy) {
	r.Condition = "on-failure"
}

// Placement sets the placement of the deploy config
func Placement(builders ...func(*latest.Placement)) func(*latest.DeployConfig) {
	return func(d *latest.DeployConfig) {
		placement := &latest.Placement{}

		for _, builder := range builders {
			builder(placement)
		}

		d.Placement = *placement
	}
}

// Constraints sets the  constraints to the placement
func Constraints(builders ...func(*latest.Constraints)) func(*latest.Placement) {
	return func(p *latest.Placement) {
		constraints := &latest.Constraints{}
		for _, builder := range builders {
			builder(constraints)
		}
		p.Constraints = constraints
	}
}

// OperatingSystem set the operating system constraint
func OperatingSystem(value, operator string) func(*latest.Constraints) {
	return func(c *latest.Constraints) {
		c.OperatingSystem = &latest.Constraint{
			Operator: operator,
			Value:    value,
		}
	}
}

// Architecture set the operating system constraint
func Architecture(value, operator string) func(*latest.Constraints) {
	return func(c *latest.Constraints) {
		c.Architecture = &latest.Constraint{
			Operator: operator,
			Value:    value,
		}
	}
}

// ConstraintHostname set the operating system constraint
func ConstraintHostname(value, operator string) func(*latest.Constraints) {
	return func(c *latest.Constraints) {
		c.Hostname = &latest.Constraint{
			Operator: operator,
			Value:    value,
		}
	}
}

// WithMatchLabel adds the labels constraint to the constraint
func WithMatchLabel(key, value, operator string) func(*latest.Constraints) {
	return func(c *latest.Constraints) {
		if c.MatchLabels == nil {
			c.MatchLabels = map[string]latest.Constraint{}
		}
		c.MatchLabels[key] = latest.Constraint{
			Operator: operator,
			Value:    value,
		}
	}
}

// WithCapAdd add a cap add to the service
func WithCapAdd(caps ...string) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		if c.CapAdd == nil {
			c.CapAdd = []string{}
		}
		c.CapAdd = append(c.CapAdd, caps...)
	}
}

// WithCapDrop adds a cap drop to the service
func WithCapDrop(caps ...string) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		if c.CapDrop == nil {
			c.CapDrop = []string{}
		}
		c.CapDrop = append(c.CapDrop, caps...)
	}
}

// WithEnvironment adds an environment variable to the service
func WithEnvironment(key string, value *string) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		if c.Environment == nil {
			c.Environment = map[string]*string{}
		}
		c.Environment[key] = value
	}
}

// ReadOnly sets the service to read only
func ReadOnly(c *latest.ServiceConfig) {
	c.ReadOnly = true
}

// Privileged sets the service to privileged
func Privileged(c *latest.ServiceConfig) {
	c.Privileged = true
}

// Entrypoint sets the entrypoint of the service
func Entrypoint(s ...string) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		c.Entrypoint = s
	}
}

// Command sets the command of the service
func Command(s ...string) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		c.Command = s
	}
}

// WorkingDir sets the service's working folder
func WorkingDir(w string) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		c.WorkingDir = w
	}
}

// WithPort adds a port config to the service
func WithPort(target uint32, builders ...func(*latest.ServicePortConfig)) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		if c.Ports == nil {
			c.Ports = []latest.ServicePortConfig{}
		}
		port := &latest.ServicePortConfig{
			Target:   target,
			Protocol: "tcp",
		}

		for _, builder := range builders {
			builder(port)
		}

		c.Ports = append(c.Ports, *port)
	}
}

// WithInternalPort adds an internal port declaration
func WithInternalPort(port int32, protocol corev1.Protocol) func(*latest.ServiceConfig) {
	return func(c *latest.ServiceConfig) {
		c.InternalPorts = append(c.InternalPorts, latest.InternalPort{
			Port:     port,
			Protocol: protocol,
		})
	}
}

// Published sets the published port
func Published(published uint32) func(*latest.ServicePortConfig) {
	return func(c *latest.ServicePortConfig) {
		c.Published = published
	}
}

// ProtocolUDP set's the port's protocol
func ProtocolUDP(c *latest.ServicePortConfig) {
	c.Protocol = "udp"
}

// Tty sets the service's tty to true
func Tty(s *latest.ServiceConfig) {
	s.Tty = true
}

// StdinOpen sets the service's stdin opne to true
func StdinOpen(s *latest.ServiceConfig) {
	s.StdinOpen = true
}
