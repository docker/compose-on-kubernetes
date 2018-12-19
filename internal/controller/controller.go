package controller

import (
	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func statusFailure(err error) latest.StackStatus {
	// on reconciliation failure, we do not set the observed generation
	// so it will always try to recreate-resources
	return latest.StackStatus{
		Phase:   latest.StackFailure,
		Message: err.Error(),
	}
}

func statusProgressing() latest.StackStatus {
	return latest.StackStatus{
		Phase:   latest.StackProgressing,
		Message: "Stack is starting",
	}
}

func setStackOwnership(stackState *stackresources.StackState, stack *latest.Stack) {
	ownerRef := []metav1.OwnerReference{ownerReference(stack)}
	for k, v := range stackState.Daemonsets {
		v.OwnerReferences = ownerRef
		stackState.Daemonsets[k] = v
	}
	for k, v := range stackState.Deployments {
		v.OwnerReferences = ownerRef
		stackState.Deployments[k] = v
	}
	for k, v := range stackState.Services {
		v.OwnerReferences = ownerRef
		stackState.Services[k] = v
	}
	for k, v := range stackState.Statefulsets {
		v.OwnerReferences = ownerRef
		stackState.Statefulsets[k] = v
	}
}

func ownerReference(stack *latest.Stack) metav1.OwnerReference {
	blockOwnerDeletion := true
	isController := true

	return metav1.OwnerReference{
		APIVersion:         latest.SchemeGroupVersion.String(),
		Kind:               "Stack",
		Name:               stack.Name,
		UID:                stack.UID,
		BlockOwnerDeletion: &blockOwnerDeletion,
		Controller:         &isController,
	}
}
