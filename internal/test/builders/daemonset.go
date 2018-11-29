package builders

import (
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta2"
	"github.com/docker/compose-on-kubernetes/api/labels"
	appstypes "k8s.io/api/apps/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Daemonset creates a core Deployment as if owned by a stack
func Daemonset(owningStack *v1beta2.Stack, name string, builders ...func(*appstypes.DaemonSet)) *appstypes.DaemonSet {
	r := &appstypes.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: owningStack.Namespace,
			Labels:    labels.ForService(owningStack.Name, name),
		},
	}
	for _, b := range builders {
		b(r)
	}
	return r
}
