package cluster

import (
	"fmt"

	"github.com/docker/compose-on-kubernetes/internal/e2e/wait"
	"github.com/onsi/ginkgo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

// By is an alias to ginkgo.By
var By = ginkgo.By

// CreateNamespace creates a namespace
func CreateNamespace(createConfig, config *rest.Config, name string) (*Namespace, func(), error) {
	coreV1Set, err := corev1.NewForConfig(createConfig)
	if err != nil {
		return nil, nil, err
	}

	namespaces := coreV1Set.Namespaces()

	_, err = namespaces.Get(name, metav1.GetOptions{})
	if err == nil {
		err := DeleteNamespace(namespaces, name, true)
		if err != nil {
			return nil, nil, err
		}
	}

	By(fmt.Sprintf("Creating namespace %q for this suite.", name))
	_, err = namespaces.Create(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	})
	if err != nil {
		return nil, nil, err
	}

	namespace, err := newNamespace(config, name)
	if err != nil {
		return nil, nil, err
	}

	return namespace, func() {
		namespace.DeleteStacks()
		DeleteNamespace(namespaces, name, true) // Ignore err
	}, nil
}

// DeleteNamespace deletes a namespace
func DeleteNamespace(namespaces corev1.NamespaceInterface, ns string, waitForDeletion bool) error {
	By(fmt.Sprintf("Destroying namespace %q for this suite.", ns))
	err := namespaces.Delete(ns, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	if !waitForDeletion {
		return nil
	}

	return wait.For(30, func() (bool, error) {
		return isDeleted(namespaces, ns)
	})
}

func isDeleted(namespaces corev1.NamespaceInterface, ns string) (bool, error) {
	_, err := namespaces.Get(ns, metav1.GetOptions{})
	return err != nil, nil
}
