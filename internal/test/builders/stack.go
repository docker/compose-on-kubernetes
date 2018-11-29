package builders

import (
	"time"

	"github.com/docker/compose-on-kubernetes/api/compose/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Stack is a builder to create an Stack
func Stack(name string, builders ...func(*v1beta2.Stack)) *v1beta2.Stack {
	stack := &v1beta2.Stack{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: &v1beta2.StackSpec{},
	}

	for _, builder := range builders {
		builder(stack)
	}

	return stack
}

// WithNamespace set the namespace of the stack
func WithNamespace(namespace string) func(*v1beta2.Stack) {
	return func(s *v1beta2.Stack) {
		s.Namespace = namespace
	}
}

// WithSecretConfig add a SecretConfig to to stack
func WithSecretConfig(name string, builders ...func(*v1beta2.SecretConfig)) func(*v1beta2.Stack) {
	return func(s *v1beta2.Stack) {
		secret := &v1beta2.SecretConfig{
			Name: name,
		}

		for _, builder := range builders {
			builder(secret)
		}

		if s.Spec.Secrets == nil {
			s.Spec.Secrets = map[string]v1beta2.SecretConfig{}
		}
		s.Spec.Secrets[name] = *secret
	}
}

// SecretFile specifies the path of a secret
func SecretFile(path string) func(*v1beta2.SecretConfig) {
	return func(s *v1beta2.SecretConfig) {
		s.File = path
	}
}

// WithConfigObjConfig add a ConfigConfig to to stack
func WithConfigObjConfig(name string, builders ...func(*v1beta2.ConfigObjConfig)) func(*v1beta2.Stack) {
	return func(s *v1beta2.Stack) {
		secret := &v1beta2.ConfigObjConfig{
			Name: name,
		}

		for _, builder := range builders {
			builder(secret)
		}

		if s.Spec.Configs == nil {
			s.Spec.Configs = map[string]v1beta2.ConfigObjConfig{}
		}
		s.Spec.Configs[name] = *secret
	}
}

// ConfigFile specifies the path of a config
func ConfigFile(path string) func(*v1beta2.ConfigObjConfig) {
	return func(c *v1beta2.ConfigObjConfig) {
		c.File = path
	}
}

// ConfigExternal specifies that the config is external
func ConfigExternal(c *v1beta2.ConfigObjConfig) {
	c.External = v1beta2.External{
		External: true,
	}
}

// WithService adds a ServiceConifg to the stack
func WithService(name string, builders ...func(*v1beta2.ServiceConfig)) func(*v1beta2.Stack) {
	return func(s *v1beta2.Stack) {
		service := &v1beta2.ServiceConfig{
			Name:  name,
			Image: "busybox:latest",
		}

		for _, builder := range builders {
			builder(service)
		}

		if s.Spec.Services == nil {
			s.Spec.Services = []v1beta2.ServiceConfig{}
		}
		s.Spec.Services = append(s.Spec.Services, *service)
	}
}

// Image specfies the image of the service
func Image(reference string) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		c.Image = reference
	}
}

// StopGracePeriod specifies the stop-grace-period duration of a service
func StopGracePeriod(duration time.Duration) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		c.StopGracePeriod = &duration
	}
}

// User specifies the user of a service
func User(user int64) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		c.User = &user
	}
}

// WithTmpFS adds a path to the tmpfs of a service
func WithTmpFS(path string) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		if c.Tmpfs == nil {
			c.Tmpfs = []string{}
		}
		c.Tmpfs = append(c.Tmpfs, path)
	}
}

// WithLabel adds a label to a service
func WithLabel(key, value string) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		if c.Labels == nil {
			c.Labels = map[string]string{}
		}
		c.Labels[key] = value
	}
}

// IPC sets the ipc mode of the service
func IPC(mode string) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		c.Ipc = mode
	}
}

// PID sets the pid mode of the service
func PID(mode string) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		c.Pid = mode
	}
}

// Hostname sets the hostname of the service
func Hostname(hostname string) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		c.Hostname = hostname
	}
}

// WithExtraHost adds an extra host to the service
func WithExtraHost(host string) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		if c.ExtraHosts == nil {
			c.ExtraHosts = []string{}
		}
		c.ExtraHosts = append(c.ExtraHosts, host)
	}
}

// WithVolume adds a volume to the service
func WithVolume(builders ...func(*v1beta2.ServiceVolumeConfig)) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		if c.Volumes == nil {
			c.Volumes = []v1beta2.ServiceVolumeConfig{}
		}

		volume := &v1beta2.ServiceVolumeConfig{}

		for _, builder := range builders {
			builder(volume)
		}

		c.Volumes = append(c.Volumes, *volume)
	}
}

// Source sets the volume source
func Source(source string) func(*v1beta2.ServiceVolumeConfig) {
	return func(v *v1beta2.ServiceVolumeConfig) {
		v.Source = source
	}
}

// Target sets the volume target
func Target(target string) func(*v1beta2.ServiceVolumeConfig) {
	return func(v *v1beta2.ServiceVolumeConfig) {
		v.Target = target
	}
}

// Volume sets the volume type to volume
func Volume(v *v1beta2.ServiceVolumeConfig) {
	v.Type = "volume"
}

// Mount sets the volume type to mount
func Mount(v *v1beta2.ServiceVolumeConfig) {
	v.Type = "mount"
}

// VolumeReadOnly sets the volume to readonly
func VolumeReadOnly(v *v1beta2.ServiceVolumeConfig) {
	v.ReadOnly = true
}

// Healthcheck sets the healthcheck config of the service
func Healthcheck(builders ...func(*v1beta2.HealthCheckConfig)) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		healthcheck := &v1beta2.HealthCheckConfig{}

		for _, builder := range builders {
			builder(healthcheck)
		}

		c.HealthCheck = healthcheck
	}
}

// Test sets the test commands of the healthcheck
func Test(cmd ...string) func(*v1beta2.HealthCheckConfig) {
	return func(h *v1beta2.HealthCheckConfig) {
		h.Test = cmd
	}
}

// Interval sets the interval duration of the healthcheck
func Interval(duration time.Duration) func(*v1beta2.HealthCheckConfig) {
	return func(h *v1beta2.HealthCheckConfig) {
		h.Interval = &duration
	}
}

// Timeout sets the timeout duration of the healthcheck
func Timeout(duration time.Duration) func(*v1beta2.HealthCheckConfig) {
	return func(h *v1beta2.HealthCheckConfig) {
		h.Timeout = &duration
	}
}

// Retries sets the number of retries of the healthcheck
func Retries(retries uint64) func(*v1beta2.HealthCheckConfig) {
	return func(h *v1beta2.HealthCheckConfig) {
		h.Retries = &retries
	}
}

// WithSecret adds a secret to the service
func WithSecret(builders ...func(*v1beta2.ServiceSecretConfig)) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		if c.Secrets == nil {
			c.Secrets = []v1beta2.ServiceSecretConfig{}
		}
		secret := &v1beta2.ServiceSecretConfig{}
		for _, builder := range builders {
			builder(secret)
		}
		c.Secrets = append(c.Secrets, *secret)
	}
}

// SecretSource sets the source of the secret
func SecretSource(source string) func(*v1beta2.ServiceSecretConfig) {
	return func(s *v1beta2.ServiceSecretConfig) {
		s.Source = source
	}
}

// SecretTarget sets the target of the secret
func SecretTarget(target string) func(*v1beta2.ServiceSecretConfig) {
	return func(s *v1beta2.ServiceSecretConfig) {
		s.Target = target
	}
}

// WithConfig adds a config to the service
func WithConfig(builders ...func(*v1beta2.ServiceConfigObjConfig)) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		if c.Configs == nil {
			c.Configs = []v1beta2.ServiceConfigObjConfig{}
		}
		config := &v1beta2.ServiceConfigObjConfig{}
		for _, builder := range builders {
			builder(config)
		}
		c.Configs = append(c.Configs, *config)
	}
}

// ConfigSource sets the source of the config
func ConfigSource(source string) func(*v1beta2.ServiceConfigObjConfig) {
	return func(c *v1beta2.ServiceConfigObjConfig) {
		c.Source = source
	}
}

// ConfigTarget sets the target of the config
func ConfigTarget(target string) func(*v1beta2.ServiceConfigObjConfig) {
	return func(c *v1beta2.ServiceConfigObjConfig) {
		c.Target = target
	}
}

// ConfigUID sets the uid of the config
func ConfigUID(uid string) func(*v1beta2.ServiceConfigObjConfig) {
	return func(c *v1beta2.ServiceConfigObjConfig) {
		c.UID = uid
	}
}

// ConfigGID sets the gid of the config
func ConfigGID(gid string) func(*v1beta2.ServiceConfigObjConfig) {
	return func(c *v1beta2.ServiceConfigObjConfig) {
		c.GID = gid
	}
}

// ConfigMode sets the mode of the config
func ConfigMode(mode uint32) func(*v1beta2.ServiceConfigObjConfig) {
	return func(c *v1beta2.ServiceConfigObjConfig) {
		c.Mode = &mode
	}
}

// Deploy sets the deploy config of the service
func Deploy(builders ...func(*v1beta2.DeployConfig)) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		deploy := &v1beta2.DeployConfig{}

		for _, builder := range builders {
			builder(deploy)
		}

		c.Deploy = *deploy
	}
}

// Resources sets the resources of the deploy config
func Resources(builders ...func(*v1beta2.Resources)) func(*v1beta2.DeployConfig) {
	return func(d *v1beta2.DeployConfig) {
		resources := &v1beta2.Resources{}

		for _, builder := range builders {
			builder(resources)
		}

		d.Resources = *resources
	}
}

// Limits sets the limits of the resources
func Limits(builders ...func(*v1beta2.Resource)) func(*v1beta2.Resources) {
	return func(r *v1beta2.Resources) {
		limits := &v1beta2.Resource{}

		for _, builder := range builders {
			builder(limits)
		}

		r.Limits = limits
	}
}

// Reservations sets the reservations of the resources
func Reservations(builders ...func(*v1beta2.Resource)) func(*v1beta2.Resources) {
	return func(r *v1beta2.Resources) {
		reservations := &v1beta2.Resource{}

		for _, builder := range builders {
			builder(reservations)
		}

		r.Reservations = reservations
	}
}

// CPUs sets the cup of the resource
func CPUs(cpus string) func(*v1beta2.Resource) {
	return func(r *v1beta2.Resource) {
		r.NanoCPUs = cpus
	}
}

// Memory sets the memory of the resource
func Memory(memory int64) func(*v1beta2.Resource) {
	return func(r *v1beta2.Resource) {
		r.MemoryBytes = memory
	}
}

// Update sets the update config of a deploy config
func Update(builders ...func(*v1beta2.UpdateConfig)) func(*v1beta2.DeployConfig) {
	return func(d *v1beta2.DeployConfig) {
		update := &v1beta2.UpdateConfig{}

		for _, builder := range builders {
			builder(update)
		}

		d.UpdateConfig = update
	}
}

// Parallelism sets the parallelism of the update config
func Parallelism(parallelism uint64) func(*v1beta2.UpdateConfig) {
	return func(u *v1beta2.UpdateConfig) {
		u.Parallelism = &parallelism
	}
}

// ModeGlobal sets the deploy mode to global
func ModeGlobal(d *v1beta2.DeployConfig) {
	d.Mode = "global"
}

// Replicas sets the number of replicas of a deploy config
func Replicas(replicas uint64) func(*v1beta2.DeployConfig) {
	return func(d *v1beta2.DeployConfig) {
		d.Replicas = &replicas
	}
}

// WithDeployLabel adds a label to the deploy config
func WithDeployLabel(key, value string) func(*v1beta2.DeployConfig) {
	return func(d *v1beta2.DeployConfig) {
		if d.Labels == nil {
			d.Labels = map[string]string{}
		}
		d.Labels[key] = value
	}
}

// RestartPolicy sets the restart policy of the deploy config
func RestartPolicy(builders ...func(*v1beta2.RestartPolicy)) func(*v1beta2.DeployConfig) {
	return func(d *v1beta2.DeployConfig) {
		rp := &v1beta2.RestartPolicy{}

		for _, builder := range builders {
			builder(rp)
		}

		d.RestartPolicy = rp
	}
}

// OnFailure sets the restart policy to on-failure
func OnFailure(r *v1beta2.RestartPolicy) {
	r.Condition = "on-failure"
}

// Placement sets the placement of the deploy config
func Placement(builders ...func(*v1beta2.Placement)) func(*v1beta2.DeployConfig) {
	return func(d *v1beta2.DeployConfig) {
		placement := &v1beta2.Placement{}

		for _, builder := range builders {
			builder(placement)
		}

		d.Placement = *placement
	}
}

// Constraints sets the  constraints to the placement
func Constraints(builders ...func(*v1beta2.Constraints)) func(*v1beta2.Placement) {
	return func(p *v1beta2.Placement) {
		constraints := &v1beta2.Constraints{}
		for _, builder := range builders {
			builder(constraints)
		}
		p.Constraints = constraints
	}
}

// OperatingSystem set the operating system constraint
func OperatingSystem(value, operator string) func(*v1beta2.Constraints) {
	return func(c *v1beta2.Constraints) {
		c.OperatingSystem = &v1beta2.Constraint{
			Operator: operator,
			Value:    value,
		}
	}
}

// Architecture set the operating system constraint
func Architecture(value, operator string) func(*v1beta2.Constraints) {
	return func(c *v1beta2.Constraints) {
		c.Architecture = &v1beta2.Constraint{
			Operator: operator,
			Value:    value,
		}
	}
}

// ConstraintHostname set the operating system constraint
func ConstraintHostname(value, operator string) func(*v1beta2.Constraints) {
	return func(c *v1beta2.Constraints) {
		c.Hostname = &v1beta2.Constraint{
			Operator: operator,
			Value:    value,
		}
	}
}

// WithMatchLabel adds the labels constraint to the constraint
func WithMatchLabel(key, value, operator string) func(*v1beta2.Constraints) {
	return func(c *v1beta2.Constraints) {
		if c.MatchLabels == nil {
			c.MatchLabels = map[string]v1beta2.Constraint{}
		}
		c.MatchLabels[key] = v1beta2.Constraint{
			Operator: operator,
			Value:    value,
		}
	}
}

// WithCapAdd add a cap add to the service
func WithCapAdd(caps ...string) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		if c.CapAdd == nil {
			c.CapAdd = []string{}
		}
		c.CapAdd = append(c.CapAdd, caps...)
	}
}

// WithCapDrop adds a cap drop to the service
func WithCapDrop(caps ...string) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		if c.CapDrop == nil {
			c.CapDrop = []string{}
		}
		c.CapDrop = append(c.CapDrop, caps...)
	}
}

// WithEnvironment adds an environment variable to the service
func WithEnvironment(key string, value *string) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		if c.Environment == nil {
			c.Environment = map[string]*string{}
		}
		c.Environment[key] = value
	}
}

// ReadOnly sets the service to read only
func ReadOnly(c *v1beta2.ServiceConfig) {
	c.ReadOnly = true
}

// Privileged sets the service to privileged
func Privileged(c *v1beta2.ServiceConfig) {
	c.Privileged = true
}

// Entrypoint sets the entrypoint of the service
func Entrypoint(s ...string) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		c.Entrypoint = s
	}
}

// Command sets the command of the service
func Command(s ...string) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		c.Command = s
	}
}

// WorkingDir sets the service's working folder
func WorkingDir(w string) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		c.WorkingDir = w
	}
}

// WithPort adds a port config to the service
func WithPort(target uint32, builders ...func(*v1beta2.ServicePortConfig)) func(*v1beta2.ServiceConfig) {
	return func(c *v1beta2.ServiceConfig) {
		if c.Ports == nil {
			c.Ports = []v1beta2.ServicePortConfig{}
		}
		port := &v1beta2.ServicePortConfig{
			Target:   target,
			Protocol: "tcp",
		}

		for _, builder := range builders {
			builder(port)
		}

		c.Ports = append(c.Ports, *port)
	}
}

// Published sets the published port
func Published(published uint32) func(*v1beta2.ServicePortConfig) {
	return func(c *v1beta2.ServicePortConfig) {
		c.Published = published
	}
}

// ProtocolUDP set's the port's protocol
func ProtocolUDP(c *v1beta2.ServicePortConfig) {
	c.Protocol = "udp"
}

// Tty sets the service's tty to true
func Tty(s *v1beta2.ServiceConfig) {
	s.Tty = true
}

// StdinOpen sets the service's stdin opne to true
func StdinOpen(s *v1beta2.ServiceConfig) {
	s.StdinOpen = true
}
