package internalversion

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const groupName = "compose.docker.com"

var (
	// StorageSchemeGroupVersion is the group version for storage
	StorageSchemeGroupVersion = schema.GroupVersion{Group: groupName, Version: "storage"}

	// InternalSchemeGroupVersion is group version used to register these objects
	InternalSchemeGroupVersion = schema.GroupVersion{Group: groupName, Version: runtime.APIVersionInternal}
)

// AddStorageToScheme adds the list of known types to api.Scheme.
func AddStorageToScheme(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(StorageSchemeGroupVersion,
		&Stack{},
		&StackList{},
	)
	return nil
}

// AddInternalToScheme adds the list of known types to api.Scheme.
func AddInternalToScheme(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(InternalSchemeGroupVersion,
		&Stack{},
		&StackList{},
	)
	return nil
}
