package registry

import (
	"context"

	iv "github.com/docker/compose-on-kubernetes/internal/internalversion"
	"github.com/docker/compose-on-kubernetes/internal/requestaddons"
	log "github.com/sirupsen/logrus"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// APIVersion describes an API level (has impact on canonicalization logic)
type APIVersion string

const (
	// APIV1beta1 represents v1beta1 API level
	APIV1beta1 = APIVersion("v1beta1")
	// APIV1beta2 represents v1beta2 API level or later
	APIV1beta2 = APIVersion("v1beta2")
)

type stackStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
	coreClient corev1.CoreV1Interface
	appsClient appsv1.AppsV1Interface
	version    APIVersion
}

func newStackStrategy(apiVersion APIVersion, typer runtime.ObjectTyper, coreClient corev1.CoreV1Interface, appsClient appsv1.AppsV1Interface) *stackStrategy {
	return &stackStrategy{ObjectTyper: typer, NameGenerator: names.SimpleNameGenerator, coreClient: coreClient, appsClient: appsClient, version: apiVersion}
}

func (s *stackStrategy) NamespaceScoped() bool {
	return true
}

type prepareStep func(ctx context.Context, oldStack *iv.Stack, newStack *iv.Stack) error
type validateStep func(ctx context.Context, stack *iv.Stack) field.ErrorList

func (s *stackStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	stack, ok := obj.(*iv.Stack)
	if !ok {
		log.Error("Unable to cast object to Stack")
		return
	}

	skipValidation := requestaddons.SkipValidationFrom(ctx)

	steps := []prepareStep{
		prepareStackOwnership(),
		prepareStackFromComposefile(skipValidation),
	}

	for _, step := range steps {
		if err := step(ctx, nil, stack); err != nil {
			stack.Status = &iv.StackStatus{
				Phase:   iv.StackFailure,
				Message: err.Error(),
			}
			log.Errorf("PrepareForCreate error (stack: %s/%s): %s", stack.Namespace, stack.Name, err)
			return
		}
	}
	stack.Status = &iv.StackStatus{
		Phase:   iv.StackReconciliationPending,
		Message: "Stack is waiting for reconciliation",
	}
}

func (s *stackStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	stack := obj.(*iv.Stack)

	skipValidation := requestaddons.SkipValidationFrom(ctx)
	if skipValidation {
		return nil
	}

	steps := []validateStep{
		validateCreationStatus(),
		validateStackNotNil(),
		validateObjectNames(),
		validateDryRun(),
		validateCollisions(s.coreClient, s.appsClient),
	}

	for _, step := range steps {
		if lst := step(ctx, stack); len(lst) > 0 {
			log.Errorf("Validate error (stack: %s/%s): %s", stack.Namespace, stack.Name, lst.ToAggregate())
			return lst
		}
	}
	return nil
}

func (s *stackStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newStack, ok := obj.(*iv.Stack)
	if !ok {
		log.Error("Unable to cast object to Stack")
		return
	}

	oldStack, ok := old.(*iv.Stack)
	if !ok {
		log.Error("Unable to cast object to Stack")
		return
	}
	skipValidation := requestaddons.SkipValidationFrom(ctx)

	steps := []prepareStep{
		prepareStackOwnership(),
		prepareFieldsForUpdate(s.version),
		prepareStackFromComposefile(skipValidation),
	}

	for _, step := range steps {
		if err := step(ctx, oldStack, newStack); err != nil {
			newStack.Status = &iv.StackStatus{
				Phase:   iv.StackFailure,
				Message: err.Error(),
			}
			log.Errorf("PrepareForUpdate error (stack: %s/%s): %s", newStack.Namespace, newStack.Name, err)
			return
		}
	}
	if !apiequality.Semantic.DeepEqual(oldStack.Spec, newStack.Spec) {
		log.Infof("stack %s/%s spec has changed, marking as waiting for reconciliation", newStack.Namespace, newStack.Name)
		newStack.Status = &iv.StackStatus{
			Phase:   iv.StackReconciliationPending,
			Message: "Stack is waiting for reconciliation",
		}
	}
}

func (s *stackStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	stack := obj.(*iv.Stack)

	skipValidation := requestaddons.SkipValidationFrom(ctx)
	if skipValidation {
		return nil
	}

	steps := []validateStep{
		validateStackNotNil(),
		validateObjectNames(),
		validateDryRun(),
		validateCollisions(s.coreClient, s.appsClient),
	}

	for _, step := range steps {
		if lst := step(ctx, stack); len(lst) > 0 {
			log.Errorf("ValidateUpdate error (stack: %s/%s): %s", stack.Namespace, stack.Name, lst.ToAggregate())
			return lst
		}
	}
	return nil
}

func (s *stackStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (s *stackStrategy) AllowUnconditionalUpdate() bool {
	return true
}

func (s *stackStrategy) Canonicalize(obj runtime.Object) {
}
