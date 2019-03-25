package registry

import (
	"context"

	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	iv "github.com/docker/compose-on-kubernetes/internal/internalversion"
	log "github.com/sirupsen/logrus"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

type stackComposeFileRest struct {
	storage stackRESTStore
}

var _ rest.Storage = &stackComposeFileRest{}
var _ rest.Getter = &stackComposeFileRest{}
var _ rest.CreaterUpdater = &stackComposeFileRest{}

// NewStackComposeFileRest returns a rest storage for composefile subresource
func NewStackComposeFileRest(storev1beta1 stackRESTStore) rest.Storage {
	return &stackComposeFileRest{storage: storev1beta1}
}

func (r *stackComposeFileRest) New() runtime.Object {
	return &latest.ComposeFile{}
}

func (r *stackComposeFileRest) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	stack, err := r.storage.GetStack(ctx, name, options)
	if err != nil {
		return nil, err
	}
	var res latest.ComposeFile
	res.ComposeFile = stack.Spec.ComposeFile
	return &res, nil
}

func (r *stackComposeFileRest) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	compose := obj.(*latest.ComposeFile)
	n, _ := genericapirequest.NamespaceFrom(ctx)
	log.Infof("Compose create from compose file %s/%s", n, compose.Name)
	var stack iv.Stack
	stack.Name = compose.Name
	stack.Namespace = n
	stack.Spec.ComposeFile = compose.ComposeFile
	stack.Generation = 1
	return r.storage.CreateStack(ctx, &stack, createValidation, options)
}

func (r *stackComposeFileRest) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo,
	createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	n, _ := genericapirequest.NamespaceFrom(ctx)
	log.Infof("Compose update from compose file %s/%s", n, name)
	return r.storage.UpdateStack(ctx, name, func(ctx context.Context, newObj *iv.Stack, oldObj *iv.Stack) (transformedNewObj *iv.Stack, err error) {
		composefile := latest.ComposeFile{
			ComposeFile: oldObj.Spec.ComposeFile,
		}
		newCompose, err := objInfo.UpdatedObject(ctx, &composefile)
		if err != nil {
			return nil, err
		}
		newObj.Spec.ComposeFile = newCompose.(*latest.ComposeFile).ComposeFile
		newObj.Spec.Stack = nil
		if !apiequality.Semantic.DeepEqual(oldObj.Spec, newObj.Spec) {
			newObj.Generation = oldObj.Generation + 1
		}
		return newObj, nil
	}, createValidation, updateValidation, forceAllowCreate, options)
}
