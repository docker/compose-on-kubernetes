package diff

import (
	"testing"

	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	"github.com/stretchr/testify/assert"
	appstypes "k8s.io/api/apps/v1"
	coretypes "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type resourceOpt func(interface{})

func testDeployment(spec string, labels map[string]string, opts ...resourceOpt) *appstypes.Deployment {
	v := &appstypes.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns",
			Name:      "test",
			Labels:    labels,
		},
		Spec: appstypes.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"test": spec}},
		},
	}
	for _, o := range opts {
		o(v)
	}
	return v
}
func testStatefulset(spec string, labels map[string]string, opts ...resourceOpt) *appstypes.StatefulSet {
	v := &appstypes.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns",
			Name:      "test",
			Labels:    labels,
		},
		Spec: appstypes.StatefulSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"test": spec}},
		},
	}
	for _, o := range opts {
		o(v)
	}
	return v
}
func testDaemonset(spec string, labels map[string]string, opts ...resourceOpt) *appstypes.DaemonSet {
	v := &appstypes.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns",
			Name:      "test",
			Labels:    labels,
		},
		Spec: appstypes.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"test": spec}},
		},
	}
	for _, o := range opts {
		o(v)
	}
	return v
}
func testService(spec string, labels map[string]string, opts ...resourceOpt) *coretypes.Service {
	v := &coretypes.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns",
			Name:      "test",
			Labels:    labels,
		},
		Spec: coretypes.ServiceSpec{
			Selector: map[string]string{"test": spec},
		},
	}
	for _, o := range opts {
		o(v)
	}
	return v
}

func testServiceType(t coretypes.ServiceType) resourceOpt {
	return func(v interface{}) {
		svc := v.(*coretypes.Service)
		svc.Spec.Type = t
	}
}

func testClusterIP(ip string) resourceOpt {
	return func(v interface{}) {
		svc := v.(*coretypes.Service)
		svc.Spec.ClusterIP = ip
	}
}

func testPodSpecImage(spec *coretypes.PodTemplateSpec, img string) {
	spec.Spec.InitContainers = []coretypes.Container{
		{
			Image: img,
		},
	}
	spec.Spec.Containers = []coretypes.Container{
		{
			Image: img,
		},
	}
}
func testPodSpecUCPTolerations(spec *coretypes.PodTemplateSpec) {
	spec.Spec.Tolerations = []coretypes.Toleration{
		{
			Key:      "com.docker.ucp.orchestrator.kubernetes",
			Operator: coretypes.TolerationOpExists,
		},
		{
			Key:      "com.docker.ucp.manager",
			Operator: coretypes.TolerationOpExists,
		},
	}
}
func withImage(img string) resourceOpt {
	return func(res interface{}) {
		switch v := res.(type) {
		case *appstypes.Deployment:
			testPodSpecImage(&v.Spec.Template, img)
		case *appstypes.StatefulSet:
			testPodSpecImage(&v.Spec.Template, img)
		case *appstypes.DaemonSet:
			testPodSpecImage(&v.Spec.Template, img)
		}
	}
}

func withUcpTolerations() resourceOpt {
	return func(res interface{}) {
		switch v := res.(type) {
		case *appstypes.Deployment:
			testPodSpecUCPTolerations(&v.Spec.Template)
		case *appstypes.StatefulSet:
			testPodSpecUCPTolerations(&v.Spec.Template)
		case *appstypes.DaemonSet:
			testPodSpecUCPTolerations(&v.Spec.Template)
		}
	}
}

func newStackStateOrPanic(objects ...interface{}) *stackresources.StackState {
	res, err := stackresources.NewStackState(objects...)
	if err != nil {
		panic(err)
	}
	return res
}

func TestStackDiff(t *testing.T) {
	cases := []struct {
		name     string
		current  *stackresources.StackState
		desired  *stackresources.StackState
		expected *StackStateDiff
	}{
		{
			name:     "EmptyToEmpty",
			current:  newStackStateOrPanic(),
			desired:  newStackStateOrPanic(),
			expected: &StackStateDiff{},
		},
		{
			name:    "EmptyToNonEmpty",
			current: newStackStateOrPanic(),
			desired: newStackStateOrPanic(
				testDeployment("spec", nil),
				testStatefulset("spec", nil),
				testDaemonset("spec", nil),
				testService("spec", nil),
			),
			expected: &StackStateDiff{
				DeploymentsToAdd: []appstypes.Deployment{
					*testDeployment("spec", nil),
				},
				StatefulsetsToAdd: []appstypes.StatefulSet{
					*testStatefulset("spec", nil),
				},
				DaemonsetsToAdd: []appstypes.DaemonSet{
					*testDaemonset("spec", nil),
				},
				ServicesToAdd: []coretypes.Service{
					*testService("spec", nil),
				},
			},
		}, {
			name:    "NonEmptyToEmpty",
			desired: newStackStateOrPanic(),
			current: newStackStateOrPanic(
				testDeployment("spec", nil),
				testStatefulset("spec", nil),
				testDaemonset("spec", nil),
				testService("spec", nil),
			),
			expected: &StackStateDiff{
				DeploymentsToDelete: []appstypes.Deployment{
					*testDeployment("spec", nil),
				},
				StatefulsetsToDelete: []appstypes.StatefulSet{
					*testStatefulset("spec", nil),
				},
				DaemonsetsToDelete: []appstypes.DaemonSet{
					*testDaemonset("spec", nil),
				},
				ServicesToDelete: []coretypes.Service{
					*testService("spec", nil),
				},
			},
		}, {
			name: "UpdateSpec",
			current: newStackStateOrPanic(
				testDeployment("specold", nil),
				testStatefulset("specold", nil),
				testDaemonset("specold", nil),
				testService("specold", nil),
			),
			desired: newStackStateOrPanic(
				testDeployment("spec", nil),
				testStatefulset("spec", nil),
				testDaemonset("spec", nil),
				testService("spec", nil),
			),
			expected: &StackStateDiff{
				DeploymentsToUpdate: []appstypes.Deployment{
					*testDeployment("spec", nil),
				},
				StatefulsetsToUpdate: []appstypes.StatefulSet{
					*testStatefulset("spec", nil),
				},
				DaemonsetsToUpdate: []appstypes.DaemonSet{
					*testDaemonset("spec", nil),
				},
				ServicesToUpdate: []coretypes.Service{
					*testService("spec", nil),
				},
			},
		},

		{
			name: "Same",
			current: newStackStateOrPanic(
				testDeployment("spec", map[string]string{"key": "val"}),
				testStatefulset("spec", map[string]string{"key": "val"}),
				testDaemonset("spec", map[string]string{"key": "val"}),
				testService("spec", map[string]string{"key": "val"}),
			),
			desired: newStackStateOrPanic(
				testDeployment("spec", map[string]string{"key": "val"}),
				testStatefulset("spec", map[string]string{"key": "val"}),
				testDaemonset("spec", map[string]string{"key": "val"}),
				testService("spec", map[string]string{"key": "val"}),
			),
			expected: &StackStateDiff{},
		},
		{
			name: "DCT-Image-Patching",
			current: newStackStateOrPanic(
				testDeployment("spec", map[string]string{"key": "val"}, withImage("testimg:testtag@test-sha")),
				testStatefulset("spec", map[string]string{"key": "val"}, withImage("testimg:testtag@test-sha")),
				testDaemonset("spec", map[string]string{"key": "val"}, withImage("testimg:testtag@test-sha")),
				testService("spec", map[string]string{"key": "val"}),
			),
			desired: newStackStateOrPanic(
				testDeployment("spec", map[string]string{"key": "val"}, withImage("testimg:testtag")),
				testStatefulset("spec", map[string]string{"key": "val"}, withImage("testimg:testtag")),
				testDaemonset("spec", map[string]string{"key": "val"}, withImage("testimg:testtag")),
				testService("spec", map[string]string{"key": "val"}),
			),
			expected: &StackStateDiff{},
		},
		{
			name: "image-tag-update",
			current: newStackStateOrPanic(
				testDeployment("spec", map[string]string{"key": "val"}, withImage("testimg:oldtag")),
				testStatefulset("spec", map[string]string{"key": "val"}, withImage("testimg:oldtag")),
				testDaemonset("spec", map[string]string{"key": "val"}, withImage("testimg:oldtag")),
				testService("spec", map[string]string{"key": "val"}),
			),
			desired: newStackStateOrPanic(
				testDeployment("spec", map[string]string{"key": "val"}, withImage("testimg:testtag")),
				testStatefulset("spec", map[string]string{"key": "val"}, withImage("testimg:testtag")),
				testDaemonset("spec", map[string]string{"key": "val"}, withImage("testimg:testtag")),
				testService("spec", map[string]string{"key": "val"}),
			),
			expected: &StackStateDiff{
				DeploymentsToUpdate: []appstypes.Deployment{
					*testDeployment("spec", map[string]string{"key": "val"}, withImage("testimg:testtag")),
				},
				StatefulsetsToUpdate: []appstypes.StatefulSet{
					*testStatefulset("spec", map[string]string{"key": "val"}, withImage("testimg:testtag")),
				},
				DaemonsetsToUpdate: []appstypes.DaemonSet{
					*testDaemonset("spec", map[string]string{"key": "val"}, withImage("testimg:testtag")),
				},
			},
		},
		{
			name: "UCP-Tolerations",
			current: newStackStateOrPanic(
				testDeployment("spec", map[string]string{"key": "val"}, withUcpTolerations()),
				testStatefulset("spec", map[string]string{"key": "val"}, withUcpTolerations()),
				testDaemonset("spec", map[string]string{"key": "val"}, withUcpTolerations()),
				testService("spec", map[string]string{"key": "val"}),
			),
			desired: newStackStateOrPanic(
				testDeployment("spec", map[string]string{"key": "val"}),
				testStatefulset("spec", map[string]string{"key": "val"}),
				testDaemonset("spec", map[string]string{"key": "val"}),
				testService("spec", map[string]string{"key": "val"}),
			),
			expected: &StackStateDiff{},
		},
		{
			name: "labels-changes",
			current: newStackStateOrPanic(
				testDeployment("spec", map[string]string{"key": "val"}),
				testStatefulset("spec", map[string]string{"key": "val"}),
				testDaemonset("spec", map[string]string{"key": "val"}),
				testService("spec", map[string]string{"key": "val"}),
			),
			desired: newStackStateOrPanic(
				testDeployment("spec", map[string]string{"key": "val2"}),
				testStatefulset("spec", map[string]string{"key": "val2"}),
				testDaemonset("spec", map[string]string{"key": "val2"}),
				testService("spec", map[string]string{"key": "val2"}),
			),
			expected: &StackStateDiff{
				DeploymentsToUpdate: []appstypes.Deployment{
					*testDeployment("spec", map[string]string{"key": "val2"}),
				},
				StatefulsetsToUpdate: []appstypes.StatefulSet{
					*testStatefulset("spec", map[string]string{"key": "val2"}),
				},
				DaemonsetsToUpdate: []appstypes.DaemonSet{
					*testDaemonset("spec", map[string]string{"key": "val2"}),
				},
				ServicesToUpdate: []coretypes.Service{
					*testService("spec", map[string]string{"key": "val2"}),
				},
			},
		},
		{
			name: "service-headless-to-cluster-ip",
			current: newStackStateOrPanic(
				testService("spec", nil, testServiceType(coretypes.ServiceTypeClusterIP), testClusterIP(coretypes.ClusterIPNone)),
			),
			desired: newStackStateOrPanic(
				testService("spec", nil, testServiceType(coretypes.ServiceTypeClusterIP), testClusterIP("")),
			),
			expected: &StackStateDiff{
				ServicesToDelete: []coretypes.Service{
					*testService("spec", nil, testServiceType(coretypes.ServiceTypeClusterIP), testClusterIP(coretypes.ClusterIPNone)),
				},
				ServicesToAdd: []coretypes.Service{
					*testService("spec", nil, testServiceType(coretypes.ServiceTypeClusterIP), testClusterIP("")),
				},
			},
		},
		{
			name: "service-cluster-ip-to-headless",
			current: newStackStateOrPanic(
				testService("spec", nil, testServiceType(coretypes.ServiceTypeClusterIP), testClusterIP("1.2.3.4")),
			),
			desired: newStackStateOrPanic(
				testService("spec", nil, testServiceType(coretypes.ServiceTypeClusterIP), testClusterIP(coretypes.ClusterIPNone)),
			),
			expected: &StackStateDiff{
				ServicesToDelete: []coretypes.Service{
					*testService("spec", nil, testServiceType(coretypes.ServiceTypeClusterIP), testClusterIP("1.2.3.4")),
				},
				ServicesToAdd: []coretypes.Service{
					*testService("spec", nil, testServiceType(coretypes.ServiceTypeClusterIP), testClusterIP(coretypes.ClusterIPNone)),
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.expected, ComputeDiff(c.current, c.desired))
		})
	}
}
