package apiserver

import (
	"net/http"
	"reflect"
	"unsafe"

	"github.com/docker/compose-on-kubernetes/api/compose/v1alpha3"
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta1"
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta2"
	"github.com/docker/compose-on-kubernetes/internal/conversions"
	"github.com/docker/compose-on-kubernetes/internal/internalversion"
	composeregistry "github.com/docker/compose-on-kubernetes/internal/registry"
	"github.com/docker/compose-on-kubernetes/internal/requestaddons"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	"k8s.io/client-go/tools/clientcmd"
)

// Internal variables
var (
	Scheme = runtime.NewScheme()
	Codecs = serializer.NewCodecFactory(Scheme)
)

func init() {
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})
	unversioned := schema.GroupVersion{Group: "", Version: "v1"}
	Scheme.AddUnversionedTypes(unversioned,
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)
	v1beta1.AddToScheme(Scheme)
	v1beta2.AddToScheme(Scheme)
	v1alpha3.AddToScheme(Scheme)
	Scheme.SetVersionPriority(v1alpha3.SchemeGroupVersion, v1beta2.SchemeGroupVersion, v1beta1.SchemeGroupVersion)
	internalversion.AddStorageToScheme(Scheme)
	internalversion.AddInternalToScheme(Scheme)
	if err := conversions.RegisterV1beta1Conversions(Scheme); err != nil {
		panic(err)
	}
	if err := conversions.RegisterV1alpha3Conversions(Scheme); err != nil {
		panic(err)
	}
	if err := conversions.RegisterV1beta2Conversions(Scheme); err != nil {
		panic(err)
	}
	// We do not support protobuf serialization as the `Stack` struct has
	// fields with unsupported types (e.g.: map[string]*string). This causes
	// issues like https://github.com/docker/compose-on-kubernetes/issues/150.
	// The workaround is to remove protobuf from the advertised supported codecs.
	removeProtobufMediaType(&Codecs)
}

// removeProtobufMediaType removes protobuf from the list of accepted media
// types for the given CodecFactory.
func removeProtobufMediaType(c *serializer.CodecFactory) {
	codecsPtr := reflect.Indirect(reflect.ValueOf(c))
	accepts := codecsPtr.FieldByName("accepts")
	acceptsPtr := (*[]runtime.SerializerInfo)(unsafe.Pointer(accepts.UnsafeAddr()))
	for i, v := range *acceptsPtr {
		if v.MediaType == runtime.ContentTypeProtobuf {
			*acceptsPtr = append((*acceptsPtr)[0:i], (*acceptsPtr)[i+1:]...)
			break
		}
	}
}

// Config is the API server config
type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
}

// CompletedConfig is the complete API server config
type CompletedConfig struct {
	// Embed a private pointer that cannot be instantiated outside of this package.
	*completedConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (cfg *Config) Complete() CompletedConfig {
	cfg.GenericConfig.BuildHandlerChainFunc = func(apiHandler http.Handler, c *genericapiserver.Config) (secure http.Handler) {
		handler := requestaddons.WithSkipValidationHandler(apiHandler)
		return genericapiserver.DefaultBuildHandlerChain(handler, c)
	}
	c := completedConfig{
		GenericConfig: cfg.GenericConfig.Complete(),
	}

	c.GenericConfig.Version = &version.Info{
		Major: "1",
		Minor: "0",
	}

	c.GenericConfig.LongRunningFunc = genericfilters.BasicLongRunningRequestCheck(sets.NewString("watch"), sets.NewString("log"))

	return CompletedConfig{&c}
}

// ComposeServer is the compose api server
type ComposeServer struct {
	*genericapiserver.GenericAPIServer
}

// New returns a new instance of ComposeServer from the given config.
func (c completedConfig) New(kubeconfig string) (*ComposeServer, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	genericServer, err := c.GenericConfig.New("compose-apiserver", genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	s := &ComposeServer{
		GenericAPIServer: genericServer,
	}
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(v1beta2.GroupName, Scheme, metav1.ParameterCodec, Codecs)

	stackRegistry1, err := composeregistry.NewStackREST(composeregistry.APIV1beta1, Scheme, c.GenericConfig.RESTOptionsGetter, config)
	if err != nil {
		return nil, err
	}

	stackRegistry2, err := composeregistry.NewStackREST(composeregistry.APIV1beta2, Scheme, c.GenericConfig.RESTOptionsGetter, config)
	if err != nil {
		return nil, err
	}
	stackOwnerRegistry1 := composeregistry.NewStackOwnerRest(stackRegistry1, composeregistry.APIV1beta1)
	v1beta1storage := map[string]rest.Storage{}
	v1beta1storage["stacks"] = stackRegistry1
	v1beta1storage["stacks/owner"] = stackOwnerRegistry1
	apiGroupInfo.VersionedResourcesStorageMap["v1beta1"] = v1beta1storage

	stackOwnerRegistry2 := composeregistry.NewStackOwnerRest(stackRegistry2, composeregistry.APIV1beta2)
	stackComposeFileRegistry := composeregistry.NewStackComposeFileRest(stackRegistry1)
	stackScaleRegistry := composeregistry.NewStackScaleRest(stackRegistry2, config)
	stackLogRegistry := composeregistry.NewStackLogRest(config)
	v1beta2storage := map[string]rest.Storage{}
	v1beta2storage["stacks"] = stackRegistry2
	v1beta2storage["stacks/owner"] = stackOwnerRegistry2
	v1beta2storage["stacks/composeFile"] = stackComposeFileRegistry
	v1beta2storage["stacks/scale"] = stackScaleRegistry
	v1beta2storage["stacks/log"] = stackLogRegistry
	apiGroupInfo.VersionedResourcesStorageMap["v1beta2"] = v1beta2storage

	// v1alpha3 has exactly the same implementation as v1beta2
	apiGroupInfo.VersionedResourcesStorageMap["v1alpha3"] = v1beta2storage

	if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return s, nil
}
