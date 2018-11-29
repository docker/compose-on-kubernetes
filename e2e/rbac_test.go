package e2e

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"

	apiv1beta2 "github.com/docker/compose-on-kubernetes/api/compose/v1beta2"
	"github.com/docker/compose-on-kubernetes/internal/e2e/cluster"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedrbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"

	// Import ginkgo to simplify test code
	. "github.com/onsi/ginkgo"
)

var _ = Describe("Compose fry with permission", func() {

	var (
		ns      *cluster.Namespace
		cleanup func()
	)

	BeforeEach(func() {
		ns, cleanup = createNamespaceWithImpersonation()
	})

	AfterEach(func() {
		cleanup()
	})

	It("Should allow user to create stack", func() {
		rbac, err := typedrbacv1.NewForConfig(config)
		expectNoError(err)
		// allow employee stack and kube access on namespace
		_, err = rbac.Roles(ns.Name()).Create(&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "office-role",
				Namespace: ns.Name(),
			},
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{"", "extensions", "apps", "compose.docker.com"},
				Resources: []string{"deployments", "replicasets", "pods", "stacks", "stacks/composefile"},
				Verbs:     []string{"*"},
			}},
		})
		expectNoError(err)

		_, err = rbac.RoleBindings(ns.Name()).Create(&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "office-role-binding",
				Namespace: ns.Name(),
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "Role",
				Name: "office-role",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "User",
					Name: "employee",
				},
			},
		})
		expectNoError(err)

		spec := `version: '3.2'
services:
  back:
    image: nginx:1.12.1-alpine`

		// rbac rules can take time to apply
		waitUntil(func() (bool, error) {
			_, err = ns.CreateStack(cluster.StackOperationV1beta2Compose, "app", spec)
			if err != nil {
				GinkgoWriter.Write([]byte(fmt.Sprintf("error occurred, waiting: %s\n", err)))
			}
			return err == nil, nil
		})

		waitUntil(ns.ContainsNPods(1))
	})

	It("Should deny user to create stack", func() {
		rbac, err := typedrbacv1.NewForConfig(config)
		expectNoError(err)
		// allow employee stack access on denied namespace
		_, err = rbac.Roles(ns.Name()).Create(&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "denied-role",
				Namespace: ns.Name(),
			},
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{"compose.docker.com"},
				Resources: []string{"stacks", "stacks/composefile"},
				Verbs:     []string{"*"},
			}},
		})
		expectNoError(err)

		_, err = rbac.RoleBindings(ns.Name()).Create(&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "denied-role-binding",
				Namespace: ns.Name(),
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "Role",
				Name: "denied-role",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "User",
					Name: "employee",
				},
			},
		})
		expectNoError(err)

		spec := `version: '3.2'
services:
  back:
    image: nginx:1.12.1-alpine`
		// rbac rules can take time to apply
		waitUntil(func() (bool, error) {
			_, err = ns.CreateStack(cluster.StackOperationV1beta2Compose, "app", spec)
			if err != nil {
				GinkgoWriter.Write([]byte(fmt.Sprintf("error occurred, waiting: %s\n", err)))
			}
			return err == nil, nil
		})
		waitUntil(func() (bool, error) {
			stack, err := ns.GetStack("app")
			if err != nil {
				return false, err
			}
			if stack.Status == nil {
				return false, nil
			}
			if stack.Status.Phase == apiv1beta2.StackFailure {
				return true, nil
			}
			if stack.Status.Phase == apiv1beta2.StackAvailable {
				return false, errors.New("Stack available when it should not")
			}
			return false, nil
		})
	})

})

// helpers

func createNamespaceWithImpersonation() (*cluster.Namespace, func()) {
	cfg := *config
	cfg.Impersonate.UserName = "employee"
	cfg.Impersonate.Groups = []string{"test-group"}
	cfg.Impersonate.Extra = map[string][]string{
		"test-extra": {"test-value"},
	}
	namespaceName := strings.ToLower(fmt.Sprintf("%s-office-%d", deployNamespace, rand.Int63()))

	ns, cleanup, err := cluster.CreateNamespace(config, &cfg, namespaceName)
	if apierrors.IsAlreadyExists(err) {
		// retry with another "random" namespace name
		return createNamespaceWithImpersonation()
	}
	expectNoError(err)

	return ns, cleanup
}
