package main

import (
	"fmt"
	"sync"
	"time"

	clientset "github.com/docker/compose-on-kubernetes/api/client/clientset/typed/compose/v1beta2"
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta2"
	coretypes "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	k8sclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type stateUpdater func(*workerState)

func benchmarkCreateStacks(stacksclient clientset.StacksGetter, workerID string, stackCount int) error {
	for ix := 0; ix < stackCount; ix++ {
		stack := &v1beta2.Stack{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("stack-%d", ix),
			},
			Spec: &v1beta2.StackSpec{
				Services: []v1beta2.ServiceConfig{
					{
						Name:  fmt.Sprintf("service-%d", ix),
						Image: "nginx:1.15.3-alpine",
					},
				},
			},
		}
		if _, err := stacksclient.Stacks(workerID).Create(stack); err != nil {
			return err
		}
	}
	return nil
}

func benchmarkWaitReconciliation(stacksclient clientset.StacksGetter, workerID string, stackCount int, updateState func(stateUpdater)) error {
	var mut sync.Mutex
	statuses := map[string]v1beta2.StackPhase{}
	stopCh := make(chan struct{})
	informer := cache.NewSharedInformer(&stackListerWatcher{stacksclient.Stacks(workerID)}, &v1beta2.Stack{}, 12*time.Hour)
	informer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			s := obj.(*v1beta2.Stack)
			if s.Status == nil {
				return
			}
			mut.Lock()
			defer mut.Unlock()
			statuses[s.Name] = s.Status.Phase
			if getStatusCounts(statuses).hasReconciliationEnded(stackCount) {
				close(stopCh)
			}
		},
		DeleteFunc: func(obj interface{}) {

		},
		UpdateFunc: func(_, obj interface{}) {
			s := obj.(*v1beta2.Stack)
			if s.Status == nil {
				return
			}
			mut.Lock()
			defer mut.Unlock()
			statuses[s.Name] = s.Status.Phase
			if getStatusCounts(statuses).hasReconciliationEnded(stackCount) {
				close(stopCh)
			}
		},
	})

	go informer.Run(stopCh)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return nil
		case <-ticker.C:
			func() {
				mut.Lock()
				defer mut.Unlock()

				updateState(func(s *workerState) {
					s.CurrentMessage = getStatusCounts(statuses).String()
				})
			}()
		}
	}
}

type stackListerWatcher struct {
	clientset.StackInterface
}

var _ cache.ListerWatcher = &stackListerWatcher{}

func (w *stackListerWatcher) List(options metav1.ListOptions) (runtime.Object, error) {
	return w.StackInterface.List(options)
}

type statusCounts struct {
	progressing int
	failed      int
	available   int
}

func (s statusCounts) hasReconciliationEnded(stackCount int) bool {
	return s.failed+s.available == stackCount
}

func (s statusCounts) String() string {
	return fmt.Sprintf("{available: %d, progressing: %d, failed: %d}", s.available, s.progressing, s.failed)
}

func getStatusCounts(src map[string]v1beta2.StackPhase) statusCounts {
	revert := map[v1beta2.StackPhase]int{}
	for _, v := range src {
		revert[v]++
	}
	return statusCounts{
		progressing: revert[v1beta2.StackProgressing],
		failed:      revert[v1beta2.StackFailure],
		available:   revert[v1beta2.StackAvailable],
	}
}

func benchmarkDelete(stacksclient clientset.StacksGetter, k8sclient k8sclientset.Interface, workerID string, updateState func(stateUpdater)) error {
	var mut sync.Mutex
	closed := false
	stopCh := make(chan struct{})
	deploymentsInformer := appsinformers.NewDeploymentInformer(k8sclient, workerID, 12*time.Hour, cache.Indexers{})
	servicesInformer := coreinformers.NewServiceInformer(k8sclient, workerID, 12*time.Hour, cache.Indexers{})
	onDelete := func(_ interface{}) {
		mut.Lock()
		defer mut.Unlock()
		if len(deploymentsInformer.GetStore().List()) == 0 &&
			len(servicesInformer.GetStore().List()) == 0 &&
			!closed {
			closed = true
			close(stopCh)
		}
	}
	deploymentsInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ interface{}) {},
		UpdateFunc: func(_, _ interface{}) {},
		DeleteFunc: onDelete,
	})
	servicesInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ interface{}) {},
		UpdateFunc: func(_, _ interface{}) {},
		DeleteFunc: onDelete,
	})
	stopCh = make(chan struct{})
	go deploymentsInformer.Run(stopCh)
	go servicesInformer.Run(stopCh)
	cache.WaitForCacheSync(stopCh, deploymentsInformer.HasSynced, servicesInformer.HasSynced)

	if err := stacksclient.Stacks(workerID).DeleteCollection(nil, metav1.ListOptions{}); err != nil {
		return err
	}

	updateState(func(s *workerState) {
		s.CurrentMessage = "waiting for deployments and services to be removed"
	})

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return nil
		case <-ticker.C:
			func() {
				mut.Lock()
				defer mut.Unlock()
				updateState(func(s *workerState) {
					s.CurrentMessage = fmt.Sprintf("remaining services: %d, deployments: %d", len(servicesInformer.GetStore().List()), len(deploymentsInformer.GetStore().List()))
				})
			}()
		}
	}
}

func benchmarkRun(cfg *rest.Config, workerID string, stackCount int, updateState func(stateUpdater)) error {
	k8sclient, err := k8sclientset.NewForConfig(cfg)
	if err != nil {
		return err
	}
	// create namespace
	_, err = k8sclient.CoreV1().Namespaces().Create(&coretypes.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: workerID,
		},
	})
	if err != nil {
		return err
	}
	defer k8sclient.CoreV1().Namespaces().Delete(workerID, nil)
	stacksclient, err := clientset.NewForConfig(cfg)
	if err != nil {
		return err
	}

	updateState(func(s *workerState) {
		s.CurrentPhase = "stacks creation"
		s.CurrentMessage = fmt.Sprintf("creating %d stacks", stackCount)
	})
	if err := benchmarkCreateStacks(stacksclient, workerID, stackCount); err != nil {
		return err
	}
	updateState(func(s *workerState) {
		s.PreviousPhases = append(s.PreviousPhases, phaseState{
			DoneTime: time.Now(),
			Name:     "stacks creation",
		})
		s.CurrentPhase = "reconciliation"
		s.CurrentMessage = "waiting for reconciliation..."
	})
	if err := benchmarkWaitReconciliation(stacksclient, workerID, stackCount, updateState); err != nil {
		return err
	}
	updateState(func(s *workerState) {
		s.PreviousPhases = append(s.PreviousPhases, phaseState{
			DoneTime: time.Now(),
			Name:     "reconciliation",
		})
		s.CurrentPhase = "delete"
		s.CurrentMessage = "deleting stacks..."
	})
	if err := benchmarkDelete(stacksclient, k8sclient, workerID, updateState); err != nil {
		return err
	}
	updateState(func(s *workerState) {
		s.PreviousPhases = append(s.PreviousPhases, phaseState{
			DoneTime: time.Now(),
			Name:     "delete",
		})
		s.CurrentPhase = "done"
		s.CurrentMessage = ""
	})
	return nil
}
