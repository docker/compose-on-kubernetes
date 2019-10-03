package registry

import (
	"testing"

	composelabels "github.com/docker/compose-on-kubernetes/api/labels"
	iv "github.com/docker/compose-on-kubernetes/internal/internalversion"
	. "github.com/docker/compose-on-kubernetes/internal/test/builders"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestValidateCollisionsExistingServiceHeadlessOnly(t *testing.T) {
	s := Stack("test",
		WithService("redis",
			Image("redis:alpine")))

	fake := kubefake.NewSimpleClientset(&apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "redis",
		},
	}, &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "redis-published",
		},
	}, &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "redis-random-ports",
		},
	})
	stack := iv.Stack{
		Spec: iv.StackSpec{
			Stack: s.Spec,
		},
	}

	err := validateCollisions(fake.CoreV1(), fake.AppsV1())(nil, &stack)
	assert.Len(t, err, 1)
}

func TestValidateCollisionsExistingServicePublished(t *testing.T) {
	s := Stack("test",
		WithService("redis",
			Image("redis:alpine"),
			WithPort(8080, Published(8080))))

	fake := kubefake.NewSimpleClientset(&apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "redis",
		},
	}, &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "redis-published",
		},
	}, &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "redis-random-ports",
		},
	})
	stack := iv.Stack{
		Spec: iv.StackSpec{
			Stack: s.Spec,
		},
	}

	err := validateCollisions(fake.CoreV1(), fake.AppsV1())(nil, &stack)
	assert.Len(t, err, 2)
}
func TestValidateCollisionsExistingServicePublishedAndRandom(t *testing.T) {
	s := Stack("test",
		WithService("redis",
			Image("redis:alpine"),
			WithPort(8080, Published(8080)),
			WithPort(22)))

	fake := kubefake.NewSimpleClientset(&apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "redis",
		},
	}, &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "redis-published",
		},
	}, &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "redis-random-ports",
		},
	})
	stack := iv.Stack{
		Spec: iv.StackSpec{
			Stack: s.Spec,
		},
	}

	err := validateCollisions(fake.CoreV1(), fake.AppsV1())(nil, &stack)
	assert.Len(t, err, 3)
}

func TestValidateCollisionsExistingServicesCorrectLabels(t *testing.T) {
	s := Stack("test",
		WithService("redis",
			Image("redis:alpine"),
			WithPort(8080, Published(8080)),
			WithPort(22)))

	fake := kubefake.NewSimpleClientset(&apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "redis",
			Labels: map[string]string{
				composelabels.ForStackName: "test",
			},
		},
	}, &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "redis-published",
			Labels: map[string]string{
				composelabels.ForStackName: "test",
			},
		},
	}, &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "redis-random-ports",
			Labels: map[string]string{
				composelabels.ForStackName: "test",
			},
		},
	})
	stack := iv.Stack{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: iv.StackSpec{
			Stack: s.Spec,
		},
	}

	err := validateCollisions(fake.CoreV1(), fake.AppsV1())(nil, &stack)
	assert.Len(t, err, 0)
}

func TestValidateCollisionsExistingServicesIncorrectLabels(t *testing.T) {
	s := Stack("test",
		WithService("redis",
			Image("redis:alpine"),
			WithPort(8080, Published(8080)),
			WithPort(22)))

	fake := kubefake.NewSimpleClientset(&apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "redis",
			Labels: map[string]string{
				composelabels.ForStackName: "test2",
			},
		},
	}, &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "redis-published",
			Labels: map[string]string{
				composelabels.ForStackName: "test2",
			},
		},
	}, &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "redis-random-ports",
			Labels: map[string]string{
				composelabels.ForStackName: "test2",
			},
		},
	})
	stack := iv.Stack{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: iv.StackSpec{
			Stack: s.Spec,
		},
	}

	err := validateCollisions(fake.CoreV1(), fake.AppsV1())(nil, &stack)
	assert.Len(t, err, 3)
}

func TestValidateCollisionsExistingDeployment(t *testing.T) {
	s := Stack("test",
		WithService("redis",
			Image("redis:alpine")))

	fake := kubefake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "redis",
		},
	})
	stack := iv.Stack{
		Spec: iv.StackSpec{
			Stack: s.Spec,
		},
	}

	err := validateCollisions(fake.CoreV1(), fake.AppsV1())(nil, &stack)
	assert.Len(t, err, 1)
}

func TestValidateCollisionsExistingStatefulset(t *testing.T) {
	s := Stack("test",
		WithService("redis",
			Image("redis:alpine"),
			WithVolume(
				Source("dbdata"),
				Target("/var/lib/postgresql/data"),
				Volume,
			)))

	fake := kubefake.NewSimpleClientset(&appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "redis",
		},
	})
	stack := iv.Stack{
		Spec: iv.StackSpec{
			Stack: s.Spec,
		},
	}

	err := validateCollisions(fake.CoreV1(), fake.AppsV1())(nil, &stack)
	assert.Len(t, err, 1)
}

func TestValidateCollisionsExistingDaemonset(t *testing.T) {
	s := Stack("test",
		WithService("redis",
			Image("redis:alpine"),
			Deploy(ModeGlobal)))

	fake := kubefake.NewSimpleClientset(&appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "redis",
		},
	})
	stack := iv.Stack{
		Spec: iv.StackSpec{
			Stack: s.Spec,
		},
	}

	err := validateCollisions(fake.CoreV1(), fake.AppsV1())(nil, &stack)
	assert.Len(t, err, 1)
}

func TestValidateServiceName(t *testing.T) {
	s := Stack("test",
		WithService("redis:test",
			Image("redis:alpine")))

	stack := iv.Stack{
		Spec: iv.StackSpec{
			Stack: s.Spec,
		},
	}
	err := validateObjectNames()(nil, &stack)
	assert.Len(t, err, 1)
}
func TestValidateVolumeName(t *testing.T) {
	s := Stack("test",
		WithService("redis",
			Image("redis:alpine"),
			WithVolume(
				Source("dbdata:test"),
				Target("/var/lib/postgresql/data"),
				Volume,
			)))

	stack := iv.Stack{
		Spec: iv.StackSpec{
			Stack: s.Spec,
		},
	}
	err := validateObjectNames()(nil, &stack)
	assert.Len(t, err, 1)
}

func TestValidateSecretName(t *testing.T) {
	s := Stack("test",
		WithService("redis",
			Image("redis:alpine")),
		WithSecretConfig("secret:test"))

	stack := iv.Stack{
		Spec: iv.StackSpec{
			Stack: s.Spec,
		},
	}
	err := validateObjectNames()(nil, &stack)
	assert.Len(t, err, 1)
}

func TestValidateDryRunOk(t *testing.T) {
	s := Stack("test",
		WithService("redis",
			Image("redis:alpine")))
	stack := iv.Stack{
		Spec: iv.StackSpec{
			Stack: s.Spec,
		},
	}
	err := validateDryRun()(nil, &stack)
	assert.Len(t, err, 0)
}
func TestValidateDryRunFail(t *testing.T) {
	s := Stack("test",
		WithService("redis",
			Deploy(ModeGlobal),
			Image("redis:alpine"),
			WithVolume(
				Source("dbdata"),
				Target("/var/lib/postgresql/data"),
				Volume,
			)))
	stack := iv.Stack{
		Spec: iv.StackSpec{
			Stack: s.Spec,
		},
	}
	err := validateDryRun()(nil, &stack)
	assert.Len(t, err, 1)
}

func TestValidateCreationStatusNil(t *testing.T) {
	stack := iv.Stack{}
	err := validateCreationStatus()(nil, &stack)
	assert.Len(t, err, 0)
}

func TestValidateCreationStatusSuccess(t *testing.T) {
	stack := iv.Stack{
		Status: &iv.StackStatus{
			Phase: iv.StackAvailable,
		},
	}
	err := validateCreationStatus()(nil, &stack)
	assert.Len(t, err, 0)
}

func TestValidateCreationStatusFailed(t *testing.T) {
	stack := iv.Stack{
		Status: &iv.StackStatus{
			Phase:   iv.StackFailure,
			Message: "error",
		},
	}
	err := validateCreationStatus()(nil, &stack)
	assert.Len(t, err, 1)
}

func TestValidateStackNotNilWithStatus(t *testing.T) {
	stack := iv.Stack{
		Status: &iv.StackStatus{
			Phase:   iv.StackFailure,
			Message: "test",
		},
	}
	err := validateStackNotNil()(nil, &stack)
	assert.Len(t, err, 1)
}

func TestValidateStackNotNilWithoutStatus(t *testing.T) {
	stack := iv.Stack{}
	err := validateStackNotNil()(nil, &stack)
	assert.Len(t, err, 1)
}

func TestValidateInvalidPullPolicy(t *testing.T) {
	s := Stack("test",
		WithService("redis",
			Image("redis"),
			PullPolicy("Invalid")))
	stack := iv.Stack{
		Spec: iv.StackSpec{
			Stack: s.Spec,
		},
	}
	err := validateDryRun()(nil, &stack)
	assert.Len(t, err, 1)
	assert.Contains(t, err[0].Error(), `invalid pull policy "Invalid", must be "Always", "IfNotPresent" or "Never"`)
}
