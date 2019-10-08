package registry

import (
	"context"

	iv "github.com/docker/compose-on-kubernetes/internal/internalversion"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
)

const composeOutOfDate = "# This compose file is outdated: the stack was updated by other means\n"

// StackREST is a storage for stack resource
type StackREST struct {
	genericregistry.Store
}

type stackRESTGet interface {
	GetStack(ctx context.Context, name string, options *metav1.GetOptions) (*iv.Stack, error)
}

type stackRESTStore interface {
	stackRESTGet
	CreateStack(ctx context.Context, newStack *iv.Stack, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (*iv.Stack, error)
	UpdateStack(ctx context.Context, name string, transform StackTransform, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc,
		forceAllowCreate bool, options *metav1.UpdateOptions) (*iv.Stack, bool, error)
}

// GetStack wraps the Get method in a more strictly typed way
func (s *StackREST) GetStack(ctx context.Context, name string, options *metav1.GetOptions) (*iv.Stack, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	stack, ok := obj.(*iv.Stack)
	if !ok {
		return nil, errors.New("Object is not a stack")
	}
	return stack, nil
}

// CreateStack wraps the Create method in a more strictly typed way
func (s *StackREST) CreateStack(ctx context.Context, newStack *iv.Stack, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (*iv.Stack, error) {
	obj, err := s.Create(ctx, newStack, createValidation, options)
	if err != nil {
		return nil, err
	}
	stack, ok := obj.(*iv.Stack)
	if !ok {
		return nil, errors.New("Object is not a stack")
	}
	return stack, nil
}

// StackTransform is a transformation used in UpdateStack
type StackTransform func(ctx context.Context, newObj *iv.Stack, oldObj *iv.Stack) (transformedNewObj *iv.Stack, err error)

// UpdateStack wraps the Update method in a more strictly typed way
func (s *StackREST) UpdateStack(ctx context.Context, name string, transform StackTransform,
	createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (*iv.Stack, bool, error) {
	updateObjectInfo := rest.DefaultUpdatedObjectInfo(nil,
		func(ctx context.Context, newObj runtime.Object, oldObj runtime.Object) (transformedNewObj runtime.Object, err error) {
			if newObj == nil {
				newObj = oldObj.DeepCopyObject()
			}
			oldStack, ok := oldObj.(*iv.Stack)
			if !ok {
				return nil, errors.New("oldObj is not a stack")
			}
			newStack, ok := newObj.(*iv.Stack)
			if !ok {
				return nil, errors.New("newObj is not a stack")
			}
			return transform(ctx, newStack, oldStack)
		})
	obj, created, err := s.Update(ctx, name, updateObjectInfo, createValidation, updateValidation, forceAllowCreate, options)
	if err != nil {
		return nil, false, err
	}
	stack, ok := obj.(*iv.Stack)
	if !ok {
		return nil, false, errors.New("result is not a stack")
	}
	return stack, created, err
}

// NewStackREST return a rest store
func NewStackREST(version APIVersion, scheme rest.RESTDeleteStrategy, optsGetter generic.RESTOptionsGetter, config *restclient.Config) (*StackREST, error) {
	coreClient, err := corev1.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	appsClient, err := appsv1.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	strategy := newStackStrategy(version, scheme, coreClient, appsClient)

	store := &StackREST{
		genericregistry.Store{
			NewFunc:                  func() runtime.Object { return &iv.Stack{} },
			NewListFunc:              func() runtime.Object { return &iv.StackList{} },
			PredicateFunc:            matchStack,
			DefaultQualifiedResource: iv.InternalSchemeGroupVersion.WithResource("stacks").GroupResource(),

			CreateStrategy: strategy,
			UpdateStrategy: strategy,
			DeleteStrategy: strategy,
			TableConvertor: stackTableConvertor{},
		},
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: getStackAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}
	return store, nil
}

func getStackAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	stack, ok := obj.(*iv.Stack)
	if !ok {
		return nil, nil, errors.New("given object is not a Stack")
	}
	return labels.Set(stack.ObjectMeta.Labels), stackToSelectableFields(stack), nil
}

func matchStack(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: getStackAttrs,
	}
}

func stackToSelectableFields(obj *iv.Stack) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, true)
}
