package diff

import (
	"reflect"
	"strings"

	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	log "github.com/sirupsen/logrus"
	appstypes "k8s.io/api/apps/v1"
	coretypes "k8s.io/api/core/v1"
)

const (
	ingnoredTolerationPrefix = "com.docker.ucp."
)

// StackStateDiff is a diff between a current and desired state
type StackStateDiff struct {
	DeploymentsToAdd     []appstypes.Deployment
	DeploymentsToDelete  []appstypes.Deployment
	DeploymentsToUpdate  []appstypes.Deployment
	StatefulsetsToAdd    []appstypes.StatefulSet
	StatefulsetsToDelete []appstypes.StatefulSet
	StatefulsetsToUpdate []appstypes.StatefulSet
	DaemonsetsToAdd      []appstypes.DaemonSet
	DaemonsetsToDelete   []appstypes.DaemonSet
	DaemonsetsToUpdate   []appstypes.DaemonSet
	ServicesToAdd        []coretypes.Service
	ServicesToDelete     []coretypes.Service
	ServicesToUpdate     []coretypes.Service
}

//Empty returns true if the diff is empty
func (d *StackStateDiff) Empty() bool {
	return len(d.DeploymentsToAdd) == 0 &&
		len(d.DeploymentsToDelete) == 0 &&
		len(d.DeploymentsToUpdate) == 0 &&
		len(d.StatefulsetsToAdd) == 0 &&
		len(d.StatefulsetsToDelete) == 0 &&
		len(d.StatefulsetsToUpdate) == 0 &&
		len(d.DaemonsetsToAdd) == 0 &&
		len(d.DaemonsetsToDelete) == 0 &&
		len(d.DaemonsetsToUpdate) == 0 &&
		len(d.ServicesToAdd) == 0 &&
		len(d.ServicesToDelete) == 0 &&
		len(d.ServicesToUpdate) == 0
}

func normalizeTolerationsForEquality(spec *coretypes.PodSpec) {
	var tolerations []coretypes.Toleration
	for _, t := range spec.Tolerations {
		if !strings.HasPrefix(t.Key, ingnoredTolerationPrefix) {
			tolerations = append(tolerations, t)
		}
	}
	spec.Tolerations = tolerations
}

func normalizePodSpecForEquality(current, desired *coretypes.PodSpec) {
	// normalize tolerations
	normalizeTolerationsForEquality(current)
	normalizeTolerationsForEquality(desired)

	// normalize potentially DCT patched images
	if len(desired.InitContainers) == len(current.InitContainers) &&
		len(desired.Containers) == len(current.Containers) {
		for ix := range desired.InitContainers {
			currentImage := current.InitContainers[ix].Image
			desiredImage := desired.InitContainers[ix].Image
			if strings.HasPrefix(currentImage, desiredImage+"@") {
				desired.InitContainers[ix].Image = currentImage
			}
		}
		for ix := range desired.Containers {
			currentImage := current.Containers[ix].Image
			desiredImage := desired.Containers[ix].Image
			if strings.HasPrefix(currentImage, desiredImage+"@") {
				desired.Containers[ix].Image = currentImage
			}
		}
	}
}

func normalizeEqualDeployment(current, desired *appstypes.Deployment) bool {
	current = current.DeepCopy()
	desired = desired.DeepCopy()
	normalizePodSpecForEquality(&current.Spec.Template.Spec, &desired.Spec.Template.Spec)
	return reflect.DeepEqual(current.Spec, desired.Spec) && reflect.DeepEqual(current.Labels, desired.Labels)
}

func normalizeEqualStatefulset(current, desired *appstypes.StatefulSet) bool {
	current = current.DeepCopy()
	desired = desired.DeepCopy()
	normalizePodSpecForEquality(&current.Spec.Template.Spec, &desired.Spec.Template.Spec)
	return reflect.DeepEqual(current.Spec, desired.Spec) && reflect.DeepEqual(current.Labels, desired.Labels)
}

func normalizeEqualDaemonset(current, desired *appstypes.DaemonSet) bool {
	current = current.DeepCopy()
	desired = desired.DeepCopy()
	normalizePodSpecForEquality(&current.Spec.Template.Spec, &desired.Spec.Template.Spec)
	return reflect.DeepEqual(current.Spec, desired.Spec) && reflect.DeepEqual(current.Labels, desired.Labels)
}

func normalizeEqualService(current, desired *coretypes.Service) bool {
	current = current.DeepCopy()
	desired = desired.DeepCopy()
	return reflect.DeepEqual(current.Spec, desired.Spec) && reflect.DeepEqual(current.Labels, desired.Labels)
}

func serviceRequiresReCreate(current, desired *coretypes.Service) bool {
	if current.Spec.Type != coretypes.ServiceTypeExternalName &&
		desired.Spec.Type != coretypes.ServiceTypeExternalName &&
		current.Spec.ClusterIP != "" {
		// once a cluster IP is assigned to a service, it cannot be changed (except if changing type from/to external name)
		return current.Spec.ClusterIP != desired.Spec.ClusterIP
	}
	return false
}

func computeDeploymentsDiff(current, desired *stackresources.StackState, result *StackStateDiff) {
	for k, desiredVersion := range desired.Deployments {
		if currentVersion, ok := current.Deployments[k]; ok {
			if !normalizeEqualDeployment(&currentVersion, &desiredVersion) {
				desiredVersion.ResourceVersion = currentVersion.ResourceVersion
				result.DeploymentsToUpdate = append(result.DeploymentsToUpdate, desiredVersion)
			}
		} else {
			result.DeploymentsToAdd = append(result.DeploymentsToAdd, desiredVersion)
		}
	}
	for k, v := range current.Deployments {
		if _, ok := desired.Deployments[k]; !ok {
			result.DeploymentsToDelete = append(result.DeploymentsToDelete, v)
		}
	}
}

func computeStatefulsetsDiff(current, desired *stackresources.StackState, result *StackStateDiff) {
	for k, desiredVersion := range desired.Statefulsets {
		if currentVersion, ok := current.Statefulsets[k]; ok {
			if !normalizeEqualStatefulset(&currentVersion, &desiredVersion) {
				desiredVersion.ResourceVersion = currentVersion.ResourceVersion
				result.StatefulsetsToUpdate = append(result.StatefulsetsToUpdate, desiredVersion)
			}
		} else {
			result.StatefulsetsToAdd = append(result.StatefulsetsToAdd, desiredVersion)
		}
	}
	for k, v := range current.Statefulsets {
		if _, ok := desired.Statefulsets[k]; !ok {
			result.StatefulsetsToDelete = append(result.StatefulsetsToDelete, v)
		}
	}
}

func computeDaemonsetsDiff(current, desired *stackresources.StackState, result *StackStateDiff) {
	for k, desiredVersion := range desired.Daemonsets {
		if currentVersion, ok := current.Daemonsets[k]; ok {
			if !normalizeEqualDaemonset(&currentVersion, &desiredVersion) {
				desiredVersion.ResourceVersion = currentVersion.ResourceVersion
				result.DaemonsetsToUpdate = append(result.DaemonsetsToUpdate, desiredVersion)
			}
		} else {
			result.DaemonsetsToAdd = append(result.DaemonsetsToAdd, desiredVersion)
		}
	}
	for k, v := range current.Daemonsets {
		if _, ok := desired.Daemonsets[k]; !ok {
			result.DaemonsetsToDelete = append(result.DaemonsetsToDelete, v)
		}
	}
}

func computeServicesDiff(current, desired *stackresources.StackState, result *StackStateDiff) {
	for k, desiredVersion := range desired.Services {
		if currentVersion, ok := current.Services[k]; ok {
			if !normalizeEqualService(&currentVersion, &desiredVersion) {
				if serviceRequiresReCreate(&currentVersion, &desiredVersion) {
					result.ServicesToDelete = append(result.ServicesToDelete, currentVersion)
					result.ServicesToAdd = append(result.ServicesToAdd, desiredVersion)
				} else {
					desiredVersion.ResourceVersion = currentVersion.ResourceVersion
					result.ServicesToUpdate = append(result.ServicesToUpdate, desiredVersion)
				}
			}
		} else {
			result.ServicesToAdd = append(result.ServicesToAdd, desiredVersion)
		}
	}
	for k, v := range current.Services {
		if _, ok := desired.Services[k]; !ok {
			result.ServicesToDelete = append(result.ServicesToDelete, v)
		}
	}
}

// ComputeDiff computes a diff between a current and a desired stack state
func ComputeDiff(current, desired *stackresources.StackState) *StackStateDiff {
	result := &StackStateDiff{}
	computeDeploymentsDiff(current, desired, result)
	computeStatefulsetsDiff(current, desired, result)
	computeDaemonsetsDiff(current, desired, result)
	computeServicesDiff(current, desired, result)

	log.Debugf("produced stack state diff %#v", result)
	return result
}
