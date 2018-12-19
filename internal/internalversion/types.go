package internalversion

import (
	"github.com/docker/compose-on-kubernetes/api/compose/impersonation"
	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Stack is the internal representation of a compose stack
type Stack struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Spec   StackSpec    `json:"spec,omitempty"`
	Status *StackStatus `json:"status,omitempty"`
}

// DeepCopyObject clones the stack
func (s *Stack) DeepCopyObject() runtime.Object {
	return s.clone()
}

func (s *Stack) clone() *Stack {
	if s == nil {
		return nil
	}
	result := new(Stack)
	result.TypeMeta = s.TypeMeta
	result.ObjectMeta = s.ObjectMeta
	result.Spec = *s.Spec.clone()
	result.Status = s.Status.clone()
	return result
}

// StackStatus is the current status of a stack
type StackStatus struct {
	Phase   StackPhase
	Message string
}

func (s *StackStatus) clone() *StackStatus {
	if s == nil {
		return nil
	}
	result := *s
	return &result
}

// StackSpec is the Spec field of a Stack
type StackSpec struct {
	ComposeFile string               `json:"composeFile,omitempty"`
	Stack       *latest.StackSpec    `json:"stack,omitempty"`
	Owner       impersonation.Config `json:"owner,omitempty"`
}

func (s *StackSpec) clone() *StackSpec {
	if s == nil {
		return nil
	}
	result := new(StackSpec)
	result.ComposeFile = s.ComposeFile
	// consider that deserialized composefile is never modified after deserialization of the composefile
	result.Stack = s.Stack.DeepCopy()
	result.Owner = *s.Owner.Clone()
	return result
}

// StackPhase is the current status phase.
type StackPhase string

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
)

// StackList is a list of stacks
type StackList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []Stack
}

// DeepCopyObject clones the stack list
func (s *StackList) DeepCopyObject() runtime.Object {
	if s == nil {
		return nil
	}
	result := new(StackList)
	result.TypeMeta = s.TypeMeta
	result.ListMeta = s.ListMeta
	if s.Items == nil {
		return result
	}
	result.Items = make([]Stack, len(s.Items))
	for ix, s := range s.Items {
		result.Items[ix] = *s.clone()
	}
	return result
}

// Owner is the user who created the stack
type Owner struct {
	metav1.TypeMeta
	metav1.ObjectMeta
	Owner impersonation.Config
}
