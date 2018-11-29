package controller

import (
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta2"
	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func statusFailure(err error) v1beta2.StackStatus {
	// on reconciliation failure, we do not set the observed generation
	// so it will always try to recreate-resources
	return v1beta2.StackStatus{
		Phase:   v1beta2.StackFailure,
		Message: err.Error(),
	}
}

func statusProgressing() v1beta2.StackStatus {
	return v1beta2.StackStatus{
		Phase:   v1beta2.StackProgressing,
		Message: "Stack is starting",
	}
}

func setStackOwnership(stackState *stackresources.StackState, stack *v1beta2.Stack) {
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

func ownerReference(stack *v1beta2.Stack) metav1.OwnerReference {
	blockOwnerDeletion := true
	isController := true

	return metav1.OwnerReference{
		APIVersion:         v1beta2.SchemeGroupVersion.String(),
		Kind:               "Stack",
		Name:               stack.Name,
		UID:                stack.UID,
		BlockOwnerDeletion: &blockOwnerDeletion,
		Controller:         &isController,
	}
}
