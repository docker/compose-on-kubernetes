package controller

import (
	"math/rand"
	"time"

	"github.com/docker/compose-on-kubernetes/api/labels"
	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	k8sclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// ChildrenListener listens to changes from resources created for a stack
type ChildrenListener struct {
	deployments    cache.Indexer
	statefulsets   cache.Indexer
	daemonsets     cache.Indexer
	services       cache.Indexer
	reconcileQueue chan<- string
	runFunc        func(stop chan struct{})
	hasSynced      func() bool
}

func (s *ChildrenListener) onEvent(obj interface{}, methodName string) {
	if !s.hasSynced() {
		return
	}
	n, err := extractStackNameAndNamespace(obj)
	if err != nil {
		log.Warnf("ChildrenListener: %s: %s. obj was %#v", methodName, err, obj)
		return
	}
	objKey := n.objKey()
	log.Debugf("Sending stack reconciliation request: %s", objKey)
	s.reconcileQueue <- objKey
}

func (s *ChildrenListener) onAdd(obj interface{}) {
	s.onEvent(obj, "onAdd")
}

func (s *ChildrenListener) onUpdate(_, newObj interface{}) {
	// first argument is to satisfy cache.ResourceEventHandlerFuncs
	s.onEvent(newObj, "onUpdate")
}

func (s *ChildrenListener) onDelete(obj interface{}) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	s.onEvent(obj, "onDelete")
}

func (s *ChildrenListener) getCurrentStackState(objKey string) (*stackresources.StackState, error) {
	deployments, err := s.deployments.ByIndex("by-stack", objKey)
	if err != nil {
		return nil, err
	}
	statefulsets, err := s.statefulsets.ByIndex("by-stack", objKey)
	if err != nil {
		return nil, err
	}
	daemonsets, err := s.daemonsets.ByIndex("by-stack", objKey)
	if err != nil {
		return nil, err
	}
	services, err := s.services.ByIndex("by-stack", objKey)
	if err != nil {
		return nil, err
	}
	return stackresources.NewStackState(append(deployments, append(statefulsets, append(daemonsets, services...)...)...)...)
}

// StartAndWaitForFullSync starts the underlying informers and wait for a full sync to occur (makes sure everything is in cache)
func (s *ChildrenListener) StartAndWaitForFullSync(stop chan struct{}) bool {
	s.runFunc(stop)
	return cache.WaitForCacheSync(stop, s.hasSynced)
}

func randomDuration(baseDuration time.Duration) time.Duration {
	var baseSeconds = baseDuration.Seconds()
	factor := rand.Float64() + 1 // number between 1 on 2
	finalSeconds := baseSeconds * factor
	return time.Duration(float64(time.Second) * finalSeconds)
}

// NewChildrenListener creates a ChildrenListener
func NewChildrenListener(clientSet k8sclientset.Interface, reconciliationInterval time.Duration, reconcileQueue chan<- string) (*ChildrenListener, error) {
	sharedInformersOption := func(o *metav1.ListOptions) {
		o.LabelSelector = labels.ForStackName
	}

	indexers := cache.Indexers{
		"by-stack": byStackIndexer,
	}
	deploymentsInformer := appsinformers.NewFilteredDeploymentInformer(clientSet, metav1.NamespaceAll, randomDuration(reconciliationInterval), indexers, sharedInformersOption)
	statefulsetInformer := appsinformers.NewFilteredStatefulSetInformer(clientSet, metav1.NamespaceAll, randomDuration(reconciliationInterval), indexers, sharedInformersOption)
	daemonsetInformer := appsinformers.NewFilteredDaemonSetInformer(clientSet, metav1.NamespaceAll, randomDuration(reconciliationInterval), indexers, sharedInformersOption)
	servicesInformer := coreinformers.NewFilteredServiceInformer(clientSet, metav1.NamespaceAll, randomDuration(reconciliationInterval), indexers, sharedInformersOption)
	result := &ChildrenListener{
		deployments:    deploymentsInformer.GetIndexer(),
		statefulsets:   statefulsetInformer.GetIndexer(),
		daemonsets:     daemonsetInformer.GetIndexer(),
		services:       servicesInformer.GetIndexer(),
		reconcileQueue: reconcileQueue,
		runFunc: func(stop chan struct{}) {
			go deploymentsInformer.Run(stop)
			go statefulsetInformer.Run(stop)
			go daemonsetInformer.Run(stop)
			go servicesInformer.Run(stop)
		},
		hasSynced: func() bool {
			return deploymentsInformer.HasSynced() &&
				statefulsetInformer.HasSynced() &&
				daemonsetInformer.HasSynced() &&
				servicesInformer.HasSynced()
		},
	}
	prepareResourceInformers(cache.ResourceEventHandlerFuncs{
		AddFunc:    result.onAdd,
		DeleteFunc: result.onDelete,
		UpdateFunc: result.onUpdate,
	}, deploymentsInformer, statefulsetInformer, daemonsetInformer, servicesInformer)

	return result, nil
}

func prepareResourceInformers(resourceHandler cache.ResourceEventHandler, informers ...cache.SharedIndexInformer) {
	for _, i := range informers {
		i.AddEventHandler(resourceHandler)
	}
}
