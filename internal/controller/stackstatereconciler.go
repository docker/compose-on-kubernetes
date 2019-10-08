package controller

import (
	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	"github.com/docker/compose-on-kubernetes/api/labels"
	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	appstypes "k8s.io/api/apps/v1"
	coretypes "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
)

// NameAndNamespace is a name/namespace pair
type NameAndNamespace struct {
	Namespace string
	Name      string
}

func (n *NameAndNamespace) objKey() string {
	return stackresources.ObjKey(n.Namespace, n.Name)
}

func extractStackNameAndNamespace(obj interface{}) (NameAndNamespace, error) {
	if stack, ok := obj.(*latest.Stack); ok {
		return NameAndNamespace{Name: stack.Name, Namespace: stack.Namespace}, nil
	}
	m, err := meta.Accessor(obj)
	if err != nil {
		return NameAndNamespace{}, err
	}
	lbls := m.GetLabels()
	if lbls == nil {
		return NameAndNamespace{}, errors.New("resource is not owned by a stack")
	}
	stackName, ok := lbls[labels.ForStackName]
	if !ok {
		return NameAndNamespace{}, errors.New("resource is not owned by a stack")
	}
	namespace := m.GetNamespace()
	return NameAndNamespace{Name: stackName, Namespace: namespace}, nil
}

func generateStatus(stack *latest.Stack, resources []interface{}) latest.StackStatus {
	log.Debugf("Generating status for stack %s/%s", stack.Namespace, stack.Name)
	remainingNames := make(map[string]struct{})
	for _, svc := range stack.Spec.Services {
		remainingNames[svc.Name] = struct{}{}
	}
	for _, r := range resources {
		switch v := r.(type) {
		case *appstypes.Deployment:
			desired := int32(1)
			if v.Spec.Replicas != nil {
				desired = *v.Spec.Replicas
			}
			if desired == v.Status.ReadyReplicas {
				delete(remainingNames, v.Name)
			}
			log.Debugf("Deployment %s has %d/%d", v.Name, v.Status.ReadyReplicas, desired)
		case *appstypes.StatefulSet:
			desired := int32(1)
			if v.Spec.Replicas != nil {
				desired = *v.Spec.Replicas
			}
			if desired == v.Status.ReadyReplicas {
				delete(remainingNames, v.Name)
			}
			log.Debugf("Statefulset %s has %d/%d", v.Name, v.Status.ReadyReplicas, desired)
		case *appstypes.DaemonSet:
			if v.Status.NumberUnavailable == 0 {
				delete(remainingNames, v.Name)
			}
			log.Debugf("Daemonset %s has %d unavailable", v.Name, v.Status.NumberUnavailable)
		case *coretypes.Service:
			// ignore
		default:
			log.Warnf("Unexpected type %T", v)
		}
	}
	if len(remainingNames) != 0 {
		log.Debugf("Services %v have not been seen", remainingNames)
		return statusProgressing()
	}
	log.Debugf("Stack is available")
	return statusAvailable()
}

func statusAvailable() latest.StackStatus {
	return latest.StackStatus{
		Phase:   latest.StackAvailable,
		Message: "Stack is started",
	}
}

func byStackIndexer(obj interface{}) ([]string, error) {
	m, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	lbls := m.GetLabels()
	if lbls == nil {
		return []string{}, nil
	}
	stackName, ok := lbls[labels.ForStackName]
	if !ok {
		return []string{}, nil
	}
	namespace := m.GetNamespace()
	if namespace == "" {
		return []string{stackName}, nil
	}
	return []string{stackresources.ObjKey(namespace, stackName)}, nil
}
