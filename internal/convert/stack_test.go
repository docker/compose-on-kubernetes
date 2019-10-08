package convert

import (
	"testing"

	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	. "github.com/docker/compose-on-kubernetes/internal/test/builders"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
)

var expectedAnnotationsOnCreate = map[string]string{
	expectedGenerationAnnotation: "1",
}

func TestLabels(t *testing.T) {
	s, err := StackToStack(*Stack("demo",
		WithService("foo",
			Image("redis"),
			WithLabel("foo", "bar"),
			Deploy(WithDeployLabel("bar", "baz")),
		),
	), loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	assert.Equal(t, s.Services["foo"].Labels, map[string]string{
		"com.docker.service.id":      "demo-foo",
		"com.docker.service.name":    "foo",
		"com.docker.stack.namespace": "demo",
		"bar":                        "baz",
	})
	assert.NotNil(t, s.Deployments["foo"].Labels, map[string]string{
		"foo": "bar",
	})
}

func TestSample(t *testing.T) {
	s, err := StackToStack(*Stack("demo",
		WithService("front",
			Image("dockerdemos/lab-web"),
			WithPort(80, Published(80)),
		),
		WithService("back",
			Image("dockerdemos/lab-words-dispatcher"),
		),
		WithService("words",
			Image("dockerdemos/lab-words-java"),
			Deploy(
				Resources(
					Limits(Memory(64*1024*1024)),
					Reservations(Memory(64*1024*1024)),
				),
			),
		),
		WithService("mongo", Image("mongo:3.5.8")),
	), loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)

	assert.Contains(t, s.Deployments, "back")
	assertImage(t, "dockerdemos/lab-words-dispatcher", s.Deployments["back"])

	assert.Contains(t, s.Deployments, "front")
	assertImage(t, "dockerdemos/lab-web", s.Deployments["front"])

	assert.Contains(t, s.Deployments, "mongo")
	assertImage(t, "mongo:3.5.8", s.Deployments["mongo"])

	assert.Contains(t, s.Deployments, "words")
	assertImage(t, "dockerdemos/lab-words-java", s.Deployments["words"])
}

func assertImage(t *testing.T, expected string, deployment appsv1.Deployment) {
	assert.Equal(t, expected, deployment.Spec.Template.Spec.Containers[0].Image)
}

func TestUnsupportedPVInDaemonSet(t *testing.T) {
	_, err := StackToStack(*Stack("demo",
		WithService("front",
			Image("nginx"),
			WithVolume(Source("dbdata"), Target("/data"), Volume),
			Deploy(ModeGlobal),
		),
	), loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.EqualError(t, err, "using persistent volumes in a global service is not supported yet")
}

func TestNilStackSpec(t *testing.T) {
	stack := Stack("nilstack",
		WithService("foo",
			Image("redis")))
	stack.Spec = nil
	_, err := StackToStack(*stack, loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.EqualError(t, err, "stack spec is nil")
}

func TestPreserveServiceClusterIPOnDirty(t *testing.T) {
	s, err := StackToStack(*Stack("demo",
		WithService("front",
			Image("dockerdemos/lab-web"),
			WithPort(80, Published(80)),
		),
	), loadBalancerServiceStrategy{}, stackresources.EmptyStackState)
	assert.NoError(t, err)
	existingService := s.Services["front-published"]
	existingService.Spec.ClusterIP = "1.2.3.4"
	newState, err := stackresources.NewStackState(&existingService)
	assert.NoError(t, err)
	s, err = StackToStack(*Stack("demo",
		WithService("front",
			Image("dockerdemos/lab-web"),
			WithPort(80, Published(81)),
		),
	), loadBalancerServiceStrategy{}, newState)
	assert.NoError(t, err)
	updatedService := s.Services["front-published"]
	assert.Equal(t, int32(81), updatedService.Spec.Ports[0].Port)
	assert.Equal(t, "1.2.3.4", updatedService.Spec.ClusterIP)
}
