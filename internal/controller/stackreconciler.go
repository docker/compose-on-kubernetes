package controller

import (
	"time"

	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	"github.com/docker/compose-on-kubernetes/internal/convert"
	"github.com/docker/compose-on-kubernetes/internal/deduplication"
	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	"github.com/docker/compose-on-kubernetes/internal/stackresources/diff"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	coretypes "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

// childrenStore provides access to children resource indexed cache
type childrenStore interface {
	getCurrentStackState(objKey string) (*stackresources.StackState, error)
}

// stackStore provides access to the stack cache
type stackStore interface {
	get(key string) (*latest.Stack, error)
}

type resourceUpdater interface {
	applyStackDiff(d *diff.StackStateDiff) error
	updateStackStatus(status latest.StackStatus) (*latest.Stack, error)
	deleteSecretsAndConfigMaps() error
}

// ResourceUpdaterProvider is a factory providing resource updaters for a given stack (default implementation generates an impersonating clientset)
type ResourceUpdaterProvider interface {
	getUpdater(stack *latest.Stack, isDirty bool) (resourceUpdater, error)
}

// StackReconciler reconciles stack into children objects
type StackReconciler struct {
	children            childrenStore
	stacks              stackStore
	serviceStrategy     convert.ServiceStrategy
	resourceUpdater     ResourceUpdaterProvider
	ownerCache          StackOwnerCacher
	reconcileRetryQueue *deduplication.StringChan
}

// NewStackReconciler creates a StackReconciler
func NewStackReconciler(stackStore stackStore,
	childrenStore childrenStore,
	defaultServiceType coretypes.ServiceType,
	resourceUpdater ResourceUpdaterProvider,
	ownerCache StackOwnerCacher) (*StackReconciler, error) {
	strategy, err := convert.ServiceStrategyFor(defaultServiceType)
	if err != nil {
		return nil, err
	}
	return &StackReconciler{
		children:            childrenStore,
		stacks:              stackStore,
		serviceStrategy:     strategy,
		resourceUpdater:     resourceUpdater,
		ownerCache:          ownerCache,
		reconcileRetryQueue: deduplication.NewStringChan(20),
	}, nil
}

// Start starts the reconciliation loop
func (r *StackReconciler) Start(reconcileQueue <-chan string, deletionQueue <-chan *latest.Stack, stop <-chan struct{}) {
	go func() {
		for {
			select {
			case <-stop:
				return
			case key := <-reconcileQueue:
				r.reconcileStack(key)
			case key := <-r.reconcileRetryQueue.Out():
				r.reconcileStack(key)
			case stack := <-deletionQueue:
				r.deleteStackChildren(stack)
			}
		}
	}()
}

func (r *StackReconciler) reconcileStack(key string) {
	stack, err := r.stacks.get(key)
	if err != nil {
		log.Errorf("Cannot reconcile stack %s: %s", key, err)
		return
	}
	if stack.DeletionTimestamp != nil {
		// pending deletion
		r.deleteStackChildren(stack)
		return
	}
	updater, err := r.resourceUpdater.getUpdater(stack, convert.IsStackDirty(stack))
	if err != nil {
		log.Errorf("Updater resolution failed: %s", err)
		return
	}
	err = r.reconcileStatus(stack, r.reconcileChildren(stack, updater), updater)
	if err != nil {
		log.Errorf("Status reconciliation failed: %s", err)
	}
}

func (r *StackReconciler) deleteStackChildren(stack *latest.Stack) {
	current, err := r.children.getCurrentStackState(stackresources.ObjKey(stack.Namespace, stack.Name))
	if err != nil {
		log.Errorf("Failed to resolve current state for %s/%s: %s", stack.Namespace, stack.Name, err)
		return
	}
	updater, err := r.resourceUpdater.getUpdater(stack, false)
	if err != nil {
		log.Errorf("Updater resolution failed: %s", err)
		return
	}
	diff := diff.ComputeDiff(current, stackresources.EmptyStackState)
	if err := updater.applyStackDiff(diff); err != nil {
		log.Errorf("Failed to remove stack children for %s/%s: %s", stack.Namespace, stack.Name, err)
		return
	}

	// handle secrets and config maps
	if err := updater.deleteSecretsAndConfigMaps(); err != nil {
		log.Errorf("Failed to remove stack secrets and config maps for %s/%s: %s", stack.Namespace, stack.Name, err)
		return
	}

	// remove from impersonation cache
	r.ownerCache.remove(stackresources.ObjKey(stack.Namespace, stack.Name))
}

func (r *StackReconciler) reconcileChildren(stack *latest.Stack, resourceUpdater resourceUpdater) error {
	objKey := stackresources.ObjKey(stack.Namespace, stack.Name)
	current, err := r.children.getCurrentStackState(objKey)
	if err != nil {
		log.Errorf("Failed to resolve current state for %s", objKey)
		return err
	}
	desired, err := convert.StackToStack(*stack, r.serviceStrategy, current)
	if err != nil {
		log.Warnf("Failed to compute desired state for %s: %s", objKey, err)
		return err
	}
	setStackOwnership(desired, stack)
	diff := diff.ComputeDiff(current, desired)
	err = resourceUpdater.applyStackDiff(diff)
	if kerrors.IsConflict(errors.Cause(err)) {
		// some resources where not in sync on reconciliation. Backoff for 1sec and retry
		log.Warnf("Conflict when updating %s or its children, retrying in 1 sec", objKey)
		go func() {
			time.Sleep(time.Second)
			r.reconcileRetryQueue.In() <- objKey
		}()
	}
	return err
}

func (r *StackReconciler) reconcileStatus(stack *latest.Stack, reconcileError error, resourceUpdater resourceUpdater) error {
	objKey := stackresources.ObjKey(stack.Namespace, stack.Name)
	current, err := r.children.getCurrentStackState(objKey)
	if err != nil {
		return err
	}
	var status latest.StackStatus
	if reconcileError != nil {
		status = statusFailure(reconcileError)
	} else {
		status = generateStatus(stack, current.FlattenResources())
	}
	_, err = resourceUpdater.updateStackStatus(status)
	if kerrors.IsConflict(errors.Cause(err)) {
		// some resources where not in sync on reconciliation. Backoff for 1sec and retry
		log.Warnf("Conflict when updating %s or its children, retrying in 1 sec", objKey)
		go func() {
			time.Sleep(time.Second)
			r.reconcileRetryQueue.In() <- objKey
		}()
	}
	return err
}
