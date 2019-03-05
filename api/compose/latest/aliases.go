package latest

import (
	ref "github.com/docker/compose-on-kubernetes/api/compose/v1alpha3"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Constraint defines a constraint and it's operator (== or !=)
type Constraint = ref.Constraint

// RestartPolicy is the service restart policy
type RestartPolicy = ref.RestartPolicy

// Stack is v1alpha3's representation of a Stack
type Stack = ref.Stack

// Scale contains the current/desired replica count for services in a stack.
type Scale = ref.Scale

// SecretConfig for a secret
type SecretConfig = ref.SecretConfig

// ServiceConfig is the configuration of one service
type ServiceConfig = ref.ServiceConfig

// ComposeFile is the content of a stack's compose file if any
type ComposeFile = ref.ComposeFile

// External identifies a Volume or Network as a reference to a resource that is
// not managed, and should already exist.
// External.name is deprecated and replaced by Volume.name
type External = ref.External

// UpdateConfig is the service update configuration
type UpdateConfig = ref.UpdateConfig

// ServicePortConfig is the port configuration for a service
type ServicePortConfig = ref.ServicePortConfig

// StackList is a list of stacks
type StackList = ref.StackList

// Placement constraints for the service
type Placement = ref.Placement

// ServiceConfigObjConfig is the config obj configuration for a service
type ServiceConfigObjConfig = ref.ServiceConfigObjConfig

// ServiceSecretConfig is the secret configuration for a service
type ServiceSecretConfig = ref.ServiceSecretConfig

// StackSpec defines the desired state of Stack
type StackSpec = ref.StackSpec

// StackPhase is the deployment phase of a stack
type StackPhase = ref.StackPhase

// FileObjectConfig is a config type for a file used by a service
type FileObjectConfig = ref.FileObjectConfig

// HealthCheckConfig the healthcheck configuration for a service
type HealthCheckConfig = ref.HealthCheckConfig

// StackStatus defines the observed state of Stack
type StackStatus = ref.StackStatus

// ConfigObjConfig is the config for the swarm "Config" object
type ConfigObjConfig = ref.ConfigObjConfig

// Constraints lists constraints that can be set on the service
type Constraints = ref.Constraints

// DeployConfig is the deployment configuration for a service
type DeployConfig = ref.DeployConfig

// Resource is a resource to be limited or reserved
type Resource = ref.Resource

// ServiceVolumeConfig are references to a volume used by a service
type ServiceVolumeConfig = ref.ServiceVolumeConfig

// Owner describes the user who created the stack
type Owner = ref.Owner

// Resources the resource limits and reservations
type Resources = ref.Resources

// FileReferenceConfig for a reference to a swarm file object
type FileReferenceConfig = ref.FileReferenceConfig

// InternalPort describes a Port exposed internally to other services
// in the stack
type InternalPort = ref.InternalPort

// InternalServiceType defines the strategy for defining the Service Type to use for inter-service networking
type InternalServiceType = ref.InternalServiceType

// These are valid conditions of a stack.
const (
	// StackAvailable means the stack is available.
	StackAvailable StackPhase = "Available"
	// StackProgressing means the deployment is progressing.
	StackProgressing StackPhase = "Progressing"
	// StackFailure is added in a stack when one of its members fails to be created
	// or deleted.
	StackFailure StackPhase = "Failure"
	// StackReconciliationPending means the stack has not yet been reconciled
	StackReconciliationPending StackPhase = "ReconciliationPending"

	// InternalServiceTypeAuto behavior is the same as InternalServiceTypeHeadless if InternalPorts is empty, InternalServiceTypeClusterIP otherwise
	InternalServiceTypeAuto = InternalServiceType("")
	// InternalServiceTypeHeadless always create a Headless service
	InternalServiceTypeHeadless = InternalServiceType("Headless")
	// InternalServiceTypeClusterIP always create a ClusterIP service
	InternalServiceTypeClusterIP = InternalServiceType("ClusterIP")
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = ref.SchemeGroupVersion
	// SchemeBuilder is the scheme builder
	SchemeBuilder = ref.SchemeBuilder
	// AddToScheme adds to scheme
	AddToScheme = ref.AddToScheme
)

// GroupResource takes an unqualified resource and returns a Group qualified GroupResource
func GroupResource(resource string) schema.GroupResource {
	return ref.GroupResource(resource)
}
