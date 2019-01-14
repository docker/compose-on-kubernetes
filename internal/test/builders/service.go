package builders

import (
	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	"github.com/docker/compose-on-kubernetes/api/labels"
	coretypes "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Service creates a core Service as if owned by a stack
func Service(owningStack *latest.Stack, name string, builders ...func(*coretypes.Service)) *coretypes.Service {
	svc := &coretypes.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: owningStack.Namespace,
			Labels:    labels.ForService(owningStack.Name, name),
		},
	}
	for _, b := range builders {
		b(svc)
	}
	return svc
}
