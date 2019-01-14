package registry

import (
	"context"
	"net/http"

	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	restclient "k8s.io/client-go/rest"
)

type stackLogRest struct {
	config *restclient.Config
}

var _ rest.Storage = &stackLogRest{}
var _ rest.Connecter = &stackLogRest{}

// NewStackLogRest returns a rest storage for log subresource
func NewStackLogRest(config *restclient.Config) rest.Storage {
	return &stackLogRest{config: config}
}

func (s *stackLogRest) New() runtime.Object {
	return &latest.Stack{} // Not used here, but needs to be a valid and registered type.
}

// ProducesMIMETypes returns a list of the MIME types the specified HTTP verb (GET, POST, DELETE,
// PATCH) can respond with.
func (s *stackLogRest) ProducesMIMETypes(verb string) []string {
	return []string{"application/octet-stream"}
}

// ProducesObject returns an object the specified HTTP verb respond with. It will overwrite storage object if
// it is not nil. Only the type of the return object matters, the value will be ignored.
func (s *stackLogRest) ProducesObject(verb string) interface{} {
	return nil
}

func (s *stackLogRest) ConnectMethods() []string {
	return []string{"GET"}
}

func (s *stackLogRest) NewConnectOptions() (runtime.Object, bool, string) {
	return nil, false, ""
}

func (s *stackLogRest) Connect(ctx context.Context, name string, options runtime.Object, r rest.Responder) (http.Handler, error) {
	namespace, _ := genericapirequest.NamespaceFrom(ctx)
	log.Infof("log connect %s/%s", namespace, name)
	return &logStreamer{s.config, namespace, name}, nil
}
