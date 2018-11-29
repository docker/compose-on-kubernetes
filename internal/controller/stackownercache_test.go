package controller

import (
	"testing"

	"github.com/docker/compose-on-kubernetes/api/compose/impersonation"
	"github.com/docker/compose-on-kubernetes/api/compose/v1beta2"
	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	"github.com/stretchr/testify/assert"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

type ownerGetterFunc struct {
	getFunc func(*v1beta2.Stack) (*v1beta2.Owner, error)
}

func (o *ownerGetterFunc) get(stack *v1beta2.Stack) (*v1beta2.Owner, error) {
	return o.getFunc(stack)
}

func TestUpdateDeleteSequence(t *testing.T) {
	var return404 bool
	getter := &ownerGetterFunc{
		getFunc: func(stack *v1beta2.Stack) (*v1beta2.Owner, error) {
			if return404 {
				return nil, kerrors.NewNotFound(v1beta2.GroupResource("stacks"), stack.Name)
			}
			return &v1beta2.Owner{
				Owner: impersonation.Config{
					UserName: "test",
				},
			}, nil
		},
	}
	testee := &stackOwnerCache{data: make(map[string]stackOwnerCacheEntry), getter: getter}

	testStack := &v1beta2.Stack{}
	testStack.Name = "test"
	testStack.Namespace = "ns"
	// as of create
	cfg := testee.get(testStack, false)
	assert.Equal(t, "test", cfg.UserName)
	// as of update
	testee.setDirty(stackresources.ObjKey(testStack.Namespace, testStack.Name))
	cfg = testee.get(testStack, false)
	assert.Equal(t, "test", cfg.UserName)
	// as of update followed by delete
	testee.setDirty(stackresources.ObjKey(testStack.Namespace, testStack.Name))
	return404 = true
	cfg = testee.get(testStack, false)
	assert.Equal(t, "test", cfg.UserName)
	// as of delete
	cfg = testee.get(testStack, true)
	assert.Equal(t, "test", cfg.UserName)
	testee.remove(stackresources.ObjKey(testStack.Namespace, testStack.Name))

}
