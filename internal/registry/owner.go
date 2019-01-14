package registry

import (
	"context"

	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta1"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
)

type stackOwnerRest struct {
	store   stackRESTGet
	version APIVersion
}

var _ rest.Storage = &stackOwnerRest{}
var _ rest.Getter = &stackOwnerRest{}

// NewStackOwnerRest returns a rest storage
func NewStackOwnerRest(store stackRESTGet, version APIVersion) rest.Storage {
	return &stackOwnerRest{store: store, version: version}
}

func (r *stackOwnerRest) New() runtime.Object {
	if r.version == "v1beta1" {
		return &v1beta1.Owner{}
	}
	return &latest.Owner{}
}

func (r *stackOwnerRest) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	stack, err := r.store.GetStack(ctx, name, options)
	if err != nil {
		return nil, err
	}
	var res latest.Owner
	res.Owner = stack.Spec.Owner
	log.Debugf("Answering owner request on %s: %s", name, res.Owner.UserName)
	return &res, nil
}
