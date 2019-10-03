package registry

import (
	"context"
	"fmt"

	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	"github.com/docker/compose-on-kubernetes/internal/conversions"
	"github.com/docker/compose-on-kubernetes/internal/convert"
	iv "github.com/docker/compose-on-kubernetes/internal/internalversion"
	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	log "github.com/sirupsen/logrus"
	coretypes "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	restclient "k8s.io/client-go/rest"
)

type stackScaleRest struct {
	storage stackRESTStore
	config  *restclient.Config
}

var _ rest.Storage = &stackScaleRest{}
var _ rest.Getter = &stackScaleRest{}
var _ rest.Updater = &stackScaleRest{}

// NewStackScaleRest returns a rest storage for scale subresource
func NewStackScaleRest(store stackRESTStore, config *restclient.Config) rest.Storage {
	return &stackScaleRest{storage: store, config: config}
}

func (r *stackScaleRest) New() runtime.Object {
	return &latest.Scale{}
}

func (r *stackScaleRest) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	stack, err := r.storage.GetStack(ctx, name, options)
	if err != nil {
		return nil, err
	}
	res := latest.Scale{
		Spec:   make(map[string]int),
		Status: make(map[string]int),
	}

	for _, s := range stack.Spec.Stack.Services {
		count := 1
		switch {
		case s.Deploy.Mode == "global":
			count = -1
		case s.Deploy.Replicas != nil:
			count = int(*s.Deploy.Replicas)
		}
		res.Spec[s.Name] = count
	}

	apps, err := appsv1.NewForConfig(r.config)
	if err != nil {
		log.Errorf("Failed to get apps: %s", err)
		return nil, err
	}
	stackLatest, err := conversions.StackFromInternalV1alpha3(stack)
	if err != nil {
		log.Errorf("Failed to convert to StackDefinition: %s", err)
		return nil, err
	}
	strategy, err := convert.ServiceStrategyFor(coretypes.ServiceTypeLoadBalancer) // in that case, service strategy does not really matter
	if err != nil {
		log.Errorf("Failed to convert to StackDefinition: %s", err)
		return nil, err
	}
	stackDef, err := convert.StackToStack(*stackLatest, strategy, stackresources.EmptyStackState)
	if err != nil {
		log.Errorf("Failed to convert to StackDefinition: %s", err)
		return nil, err
	}
	for _, v := range stackDef.Deployments {
		dep, err := apps.Deployments(stack.Namespace).Get(v.Name, metav1.GetOptions{})
		if err != nil {
			log.Errorf("Failed to get Deployment for %s: %s", v.Name, err)
		} else {
			res.Status[v.Name] = int(dep.Status.AvailableReplicas)
		}
	}
	for _, v := range stackDef.Statefulsets {
		ss, err := apps.StatefulSets(stack.Namespace).Get(v.Name, metav1.GetOptions{})
		if err != nil {
			log.Errorf("Failed to get StatefulSet for %s: %s", v.Name, err)
		} else {
			res.Status[v.Name] = int(ss.Status.ReadyReplicas)
		}
	}
	for _, v := range stackDef.Daemonsets {
		ds, err := apps.DaemonSets(stack.Namespace).Get(v.Name, metav1.GetOptions{})
		if err != nil {
			log.Errorf("Failed to get StatefulSet for %s: %s", v.Name, err)
		} else {
			res.Status[v.Name] = int(ds.Status.NumberAvailable)
		}
	}
	return &res, nil
}

func (r *stackScaleRest) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo,
	createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	scale, err := r.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}
	newScalero, err := objInfo.UpdatedObject(ctx, scale)
	if err != nil {
		return nil, false, err
	}
	newScale := newScalero.(*latest.Scale)
	log.Infof("Scale update %s: %v", name, newScale.Spec)

	return r.storage.UpdateStack(ctx, name, func(ctx context.Context, newObj *iv.Stack, oldObj *iv.Stack) (transformedNewObj *iv.Stack, err error) {
		for target, count := range newScale.Spec {
			r := uint64(count)
			hit := false
			for i, srv := range newObj.Spec.Stack.Services {
				if srv.Name == target {
					newObj.Spec.Stack.Services[i].Deploy.Replicas = &r
					hit = true
					break
				}
			}
			if !hit {
				return nil, fmt.Errorf("service %q not found in stack", target)
			}
		}
		if !apiequality.Semantic.DeepEqual(oldObj.Spec, newObj.Spec) {
			newObj.Generation = oldObj.Generation + 1
		}
		return newObj, nil
	}, createValidation, updateValidation, forceAllowCreate, options)
}
