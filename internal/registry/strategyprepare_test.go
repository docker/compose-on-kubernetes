package registry

import (
	"testing"

	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	iv "github.com/docker/compose-on-kubernetes/internal/internalversion"
	"github.com/stretchr/testify/assert"
	"k8s.io/apiserver/pkg/authentication/user"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
)

func TestPrepareStackOwnership(t *testing.T) {
	testuser := &user.DefaultInfo{
		Name:   "test-user",
		Groups: []string{"group1", "group2"},
		Extra:  map[string][]string{"extra1": {"v1", "v2"}},
	}
	ctx := genericapirequest.WithUser(genericapirequest.NewDefaultContext(), testuser)
	stack := iv.Stack{}
	err := prepareStackOwnership()(ctx, nil, &stack)
	assert.NoError(t, err)
	assert.Equal(t, testuser.Name, stack.Spec.Owner.UserName)
	assert.True(t, assert.ObjectsAreEqual(testuser.Groups, stack.Spec.Owner.Groups))
	assert.True(t, assert.ObjectsAreEqual(testuser.Extra, stack.Spec.Owner.Extra))
}

func TestPrepareStackOwnershipNoUser(t *testing.T) {
	stack := iv.Stack{}
	err := prepareStackOwnership()(genericapirequest.NewDefaultContext(), nil, &stack)
	assert.Error(t, err)
}

func TestSetFieldsV1beta1SameCompose(t *testing.T) {
	oldStack := iv.Stack{
		Spec: iv.StackSpec{
			Stack:       &latest.StackSpec{},
			ComposeFile: "test",
		},
	}
	newStack := iv.Stack{
		Spec: iv.StackSpec{
			ComposeFile: "test",
		},
	}
	err := prepareFieldsForUpdate(APIV1beta1)(nil, &oldStack, &newStack)
	assert.NoError(t, err)
	assert.Equal(t, oldStack.Spec.Stack, newStack.Spec.Stack)
}

func TestSetFieldsV1beta1DifferentCompose(t *testing.T) {
	oldStack := iv.Stack{
		Spec: iv.StackSpec{
			Stack:       &latest.StackSpec{},
			ComposeFile: "test",
		},
	}
	newStack := iv.Stack{
		Spec: iv.StackSpec{
			Stack:       &latest.StackSpec{},
			ComposeFile: "test2",
		},
	}
	err := prepareFieldsForUpdate(APIV1beta1)(nil, &oldStack, &newStack)
	assert.NoError(t, err)
	assert.Nil(t, newStack.Spec.Stack)
}

func TestSetFieldsV1beta2SameStack(t *testing.T) {
	oldStack := iv.Stack{
		Spec: iv.StackSpec{
			Stack:       &latest.StackSpec{},
			ComposeFile: "test",
		},
	}
	newStack := iv.Stack{
		Spec: iv.StackSpec{
			Stack: &latest.StackSpec{},
		},
	}
	err := prepareFieldsForUpdate(APIV1beta2)(nil, &oldStack, &newStack)
	assert.NoError(t, err)
	assert.Equal(t, oldStack.Spec.ComposeFile, newStack.Spec.ComposeFile)
}

func TestSetFieldsV1beta2DifferentStack(t *testing.T) {
	oldStack := iv.Stack{
		Spec: iv.StackSpec{
			Stack:       &latest.StackSpec{},
			ComposeFile: "test",
		},
	}
	newStack := iv.Stack{
		Spec: iv.StackSpec{
			Stack: &latest.StackSpec{
				Services: []latest.ServiceConfig{
					{},
				},
			},
		},
	}
	err := prepareFieldsForUpdate(APIV1beta2)(nil, &oldStack, &newStack)
	assert.NoError(t, err)
	assert.Equal(t, composeOutOfDate+oldStack.Spec.ComposeFile, newStack.Spec.ComposeFile)
}

func TestPrepareStackFromValidComposefile(t *testing.T) {
	stack := iv.Stack{
		Spec: iv.StackSpec{
			ComposeFile: `version: '3.2'
services:
  front:
    image: nginx:1.12.1-alpine
    ports:
    - 80:80`,
		},
	}
	err := prepareStackFromComposefile(false)(nil, nil, &stack)
	assert.NoError(t, err)
	assert.NotNil(t, stack.Spec.Stack)
}

func TestPrepareStackFromInvalidComposefile(t *testing.T) {
	stack := iv.Stack{
		Spec: iv.StackSpec{
			ComposeFile: `test`,
		},
	}
	err := prepareStackFromComposefile(false)(nil, nil, &stack)
	assert.Error(t, err)
}
