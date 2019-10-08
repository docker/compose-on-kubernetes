package stackresources

import (
	"fmt"

	appstypes "k8s.io/api/apps/v1"
	coretypes "k8s.io/api/core/v1"
)

// ObjKey returns the key of a k8s object
func ObjKey(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return fmt.Sprintf("%s/%s", namespace, name)
}

// StackState holds resources created for a stack
type StackState struct {
	Deployments  map[string]appstypes.Deployment
	Statefulsets map[string]appstypes.StatefulSet
	Daemonsets   map[string]appstypes.DaemonSet
	Services     map[string]coretypes.Service
}

// FlattenResources returns all resources in an untyped slice
func (s *StackState) FlattenResources() []interface{} {
	var result []interface{}
	for ix := range s.Deployments {
		v := s.Deployments[ix]
		result = append(result, &v)
	}
	for ix := range s.Statefulsets {
		v := s.Statefulsets[ix]
		result = append(result, &v)
	}
	for ix := range s.Daemonsets {
		v := s.Daemonsets[ix]
		result = append(result, &v)
	}
	for ix := range s.Services {
		v := s.Services[ix]
		result = append(result, &v)
	}
	return result
}

// EmptyStackState is an empty StackState
var EmptyStackState = &StackState{
	Deployments:  make(map[string]appstypes.Deployment),
	Statefulsets: make(map[string]appstypes.StatefulSet),
	Daemonsets:   make(map[string]appstypes.DaemonSet),
	Services:     make(map[string]coretypes.Service),
}

// NewStackState creates a StackState from a list of existing resources
func NewStackState(objects ...interface{}) (*StackState, error) {
	result := &StackState{
		Deployments:  make(map[string]appstypes.Deployment),
		Statefulsets: make(map[string]appstypes.StatefulSet),
		Daemonsets:   make(map[string]appstypes.DaemonSet),
		Services:     make(map[string]coretypes.Service),
	}
	for _, o := range objects {
		switch r := o.(type) {
		case *appstypes.Deployment:
			result.Deployments[ObjKey(r.Namespace, r.Name)] = *r
		case *appstypes.StatefulSet:
			result.Statefulsets[ObjKey(r.Namespace, r.Name)] = *r
		case *appstypes.DaemonSet:
			result.Daemonsets[ObjKey(r.Namespace, r.Name)] = *r
		case *coretypes.Service:
			result.Services[ObjKey(r.Namespace, r.Name)] = *r
		default:
			return nil, fmt.Errorf("unexpected object type: %T", o)
		}
	}
	return result, nil
}
