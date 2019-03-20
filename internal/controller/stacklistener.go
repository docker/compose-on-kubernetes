package controller

import (
	"time"

	"github.com/docker/compose-on-kubernetes/api/client/clientset"
	"github.com/docker/compose-on-kubernetes/api/client/informers/compose/v1alpha3"
	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

// StackListener listen for changes in stacks from the API
type StackListener struct {
	stacks                 stackIndexer
	reconcileQueue         chan<- string
	reconcileDeletionQueue chan<- *latest.Stack
	ownerCache             StackOwnerCacher
}

type stackIndexer interface {
	GetStore() cache.Store
	Run(<-chan struct{})
}

func (s *StackListener) onAdd(obj interface{}) {
	n, err := extractStackNameAndNamespace(obj)
	if err != nil {
		log.Warnf("StackListener: onAdd: %s", err)
		return
	}
	objKey := n.objKey()
	s.ownerCache.setDirty(objKey)
	log.Debugf("Sending stack reconciliation request: %s", objKey)
	s.reconcileQueue <- objKey
}

func (s *StackListener) onUpdate(_, newObj interface{}) {
	n, err := extractStackNameAndNamespace(newObj)
	if err != nil {
		log.Warnf("StackListener: onUpdate: %s", err)
		return
	}
	objKey := n.objKey()
	s.ownerCache.setDirty(objKey)
	log.Debugf("Sending stack reconciliation request: %s", objKey)
	s.reconcileQueue <- objKey
}

func (s *StackListener) onDelete(obj interface{}) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	stack, ok := obj.(*latest.Stack)
	if !ok {
		log.Warnf("StackListener: onDelete: unable to retrive deleted stack")
		return
	}
	log.Debugf("Sending stack deletion request: %s/%s", stack.Namespace, stack.Name)
	s.reconcileDeletionQueue <- stack
}

func (s *StackListener) get(key string) (*latest.Stack, error) {
	res, exists, err := s.stacks.GetStore().GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.Errorf("not found: %s", key)
	}
	stack, ok := res.(*latest.Stack)
	if !ok {
		return nil, errors.Errorf("object with key %s is not a stack: %T", key, res)
	}
	return stack, nil
}

// Start starts the underlying informer
func (s *StackListener) Start(stop chan struct{}) {
	go s.stacks.Run(stop)
}

// NewStackListener creates a StackListener
func NewStackListener(clientSet clientset.Interface,
	reconciliationInterval time.Duration,
	reconcileQueue chan<- string,
	reconcileDeletionQueue chan<- *latest.Stack,
	ownerCache StackOwnerCacher) *StackListener {
	stacksInformer := v1alpha3.NewFilteredStackInformer(clientSet, reconciliationInterval, func(o *metav1.ListOptions) {})
	result := &StackListener{
		stacks:                 stacksInformer,
		reconcileQueue:         reconcileQueue,
		reconcileDeletionQueue: reconcileDeletionQueue,
		ownerCache:             ownerCache,
	}
	stacksInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    result.onAdd,
		UpdateFunc: result.onUpdate,
		DeleteFunc: result.onDelete,
	})
	return result
}
