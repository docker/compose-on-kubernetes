package controller

import (
	"testing"

	"github.com/docker/compose-on-kubernetes/api/compose/impersonation"
	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	"github.com/stretchr/testify/assert"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

type ownerGetterFunc struct {
	getFunc func(*latest.Stack) (*latest.Owner, error)
}

func (o *ownerGetterFunc) get(stack *latest.Stack) (*latest.Owner, error) {
	return o.getFunc(stack)
}

func TestUpdateDeleteSequence(t *testing.T) {
	var return404 bool
	getter := &ownerGetterFunc{
		getFunc: func(stack *latest.Stack) (*latest.Owner, error) {
			if return404 {
				return nil, kerrors.NewNotFound(latest.GroupResource("stacks"), stack.Name)
			}
			return &latest.Owner{
				Owner: impersonation.Config{
					UserName: "test",
				},
			}, nil
		},
	}
	testee := &stackOwnerCache{data: make(map[string]stackOwnerCacheEntry), getter: getter}

	testStack := &latest.Stack{}
	testStack.Name = "test"
	testStack.Namespace = "ns"
	// as of create
	cfg, _ := testee.getWithRetries(testStack, false)
	assert.Equal(t, "test", cfg.UserName)
	// as of update
	testee.setDirty(stackresources.ObjKey(testStack.Namespace, testStack.Name))
	cfg, _ = testee.getWithRetries(testStack, false)
	assert.Equal(t, "test", cfg.UserName)
	// as of update followed by delete
	testee.setDirty(stackresources.ObjKey(testStack.Namespace, testStack.Name))
	return404 = true
	cfg, _ = testee.getWithRetries(testStack, false)
	assert.Equal(t, "test", cfg.UserName)
	// as of delete
	cfg, _ = testee.getWithRetries(testStack, true)
	assert.Equal(t, "test", cfg.UserName)
	testee.remove(stackresources.ObjKey(testStack.Namespace, testStack.Name))

}

// this is a test to reproduce the hang of DockerCon EU 2018
// that occurred because of an informer's "delete" event missed triggered by
// the demo machine going to sleep at a particularly bad time, and etcd compaction occurring
// just after.
func TestStackOwnerCachePanicOnUnresolvableOwner(t *testing.T) {
	getter := &ownerGetterFunc{
		getFunc: func(stack *latest.Stack) (*latest.Owner, error) {
			return nil, kerrors.NewNotFound(latest.GroupResource("stacks"), stack.Name)
		},
	}
	testee := &stackOwnerCache{data: make(map[string]stackOwnerCacheEntry), getter: getter}
	testStack := &latest.Stack{}
	testStack.Name = "test"
	testStack.Namespace = "ns"
	_, err := testee.getWithRetries(testStack, true)
	assert.Error(t, err)
}
