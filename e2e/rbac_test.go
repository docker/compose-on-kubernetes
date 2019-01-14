package e2e

import (
	"errors"
	"fmt"

	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	"github.com/docker/compose-on-kubernetes/internal/e2e/cluster"
	. "github.com/onsi/ginkgo" // Import ginkgo to simplify test code
	. "github.com/onsi/gomega" // Import gomega to simplify test code
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedrbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/rest"
)

var _ = Describe("Compose fry with permission", func() {

	var (
		originNS *cluster.Namespace
		cleanup  func()
	)

	BeforeEach(func() {
		originNS, cleanup = createNamespace()
	})

	AfterEach(func() {
		cleanup()
	})

	It("Should allow user to create stack", func() {
		ns, err := originNS.As(rest.ImpersonationConfig{
			UserName: "employee",
			Groups:   []string{"test-group"},
			Extra: map[string][]string{
				"test-extra": {"test-value"},
			},
		})
		expectNoError(err)
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
		ns, err := originNS.As(rest.ImpersonationConfig{
			UserName: "employee",
			Groups:   []string{"test-group"},
			Extra: map[string][]string{
				"test-extra": {"test-value"},
			},
		})
		expectNoError(err)
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
			if stack.Status.Phase == latest.StackFailure {
				return true, nil
			}
			if stack.Status.Phase == latest.StackAvailable {
				return false, errors.New("Stack available when it should not")
			}
			return false, nil
		})
	})

	It("Should respect view/edit/admin cluster roles", func() {
		viewer, err := originNS.As(rest.ImpersonationConfig{
			UserName: "viewer",
		})
		expectNoError(err)
		editor, err := originNS.As(rest.ImpersonationConfig{
			UserName: "editor",
		})
		expectNoError(err)
		admin, err := originNS.As(rest.ImpersonationConfig{
			UserName: "admin",
		})
		expectNoError(err)
		rbac, err := typedrbacv1.NewForConfig(config)
		expectNoError(err)
		_, err = rbac.RoleBindings(originNS.Name()).Create(&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "viewer-role-binding",
				Namespace: originNS.Name(),
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "ClusterRole",
				Name: "view",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "User",
					Name: "viewer",
				},
			},
		})
		expectNoError(err)
		_, err = rbac.RoleBindings(originNS.Name()).Create(&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "editor-role-binding",
				Namespace: originNS.Name(),
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "ClusterRole",
				Name: "edit",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "User",
					Name: "editor",
				},
			},
		})
		expectNoError(err)
		_, err = rbac.RoleBindings(originNS.Name()).Create(&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "admin-role-binding",
				Namespace: originNS.Name(),
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "ClusterRole",
				Name: "admin",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "User",
					Name: "admin",
				},
			},
		})
		expectNoError(err)
		_, err = originNS.CreateStack(cluster.StackOperationV1beta2Stack, "by-cluster-admin", `version: '3.2'
services:
  back-cluster-admin:
    image: nginx:1.12.1-alpine`)
		expectNoError(err)
		_, err = editor.CreateStack(cluster.StackOperationV1beta2Stack, "by-editor", `version: '3.2'
services:
  back-editor:
    image: nginx:1.12.1-alpine`)
		expectNoError(err)
		_, err = admin.CreateStack(cluster.StackOperationV1beta2Stack, "by-admin", `version: '3.2'
services:
  back-admin:
    image: nginx:1.12.1-alpine`)
		expectNoError(err)
		_, err = viewer.CreateStack(cluster.StackOperationV1beta2Stack, "by-viewer", `version: '3.2'
services:
  back-viewer:
    image: nginx:1.12.1-alpine`)
		Expect(err).To(HaveOccurred())
		stacks, err := viewer.ListStacks()
		expectNoError(err)
		Expect(stacks).To(HaveLen(3))
		stacks, err = editor.ListStacks()
		expectNoError(err)
		Expect(stacks).To(HaveLen(3))
		stacks, err = admin.ListStacks()
		expectNoError(err)
		Expect(stacks).To(HaveLen(3))
		waitUntil(originNS.IsStackAvailable("by-cluster-admin"))
		waitUntil(originNS.IsStackAvailable("by-editor"))
		waitUntil(originNS.IsStackAvailable("by-admin"))
	})
})
