package controller

import (
	"sync"
	"testing"

	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type dummyOwnerCache struct {
	mut  sync.Mutex
	data map[string]stackOwnerCacheEntry
}

func (s *dummyOwnerCache) remove(key string) {
	s.mut.Lock()
	defer s.mut.Unlock()
	delete(s.data, key)
}

func (s *dummyOwnerCache) setDirty(key string) {
	s.mut.Lock()
	defer s.mut.Unlock()
	if entry, ok := s.data[key]; ok {
		entry.dirty = true
		s.data[key] = entry
	}
}

func (s *dummyOwnerCache) getWithRetries(stack *latest.Stack, acceptDirty bool) (rest.ImpersonationConfig, error) {
	return rest.ImpersonationConfig{}, nil
}
func TestStackListenerCacheInvalidation(t *testing.T) {
	cache := &dummyOwnerCache{
		data: map[string]stackOwnerCacheEntry{
			"ns/test-add":    {config: rest.ImpersonationConfig{UserName: "test"}},
			"ns/test-update": {config: rest.ImpersonationConfig{UserName: "test"}},
			"ns/test-delete": {config: rest.ImpersonationConfig{UserName: "test"}},
		},
	}
	reconcileQueue := make(chan string)
	defer close(reconcileQueue)
	deleteQueue := make(chan *latest.Stack)
	defer close(deleteQueue)
	go func() {
		for range reconcileQueue {
		}
	}()
	go func() {
		for range deleteQueue {
		}
	}()
	testee := &StackListener{
		reconcileQueue:         reconcileQueue,
		reconcileDeletionQueue: deleteQueue,
		ownerCache:             cache,
	}
	testee.onAdd(&latest.Stack{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns",
			Name:      "test-add",
		},
	})
	testee.onUpdate(nil, &latest.Stack{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns",
			Name:      "test-update",
		},
	})
	testee.onDelete(&latest.Stack{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns",
			Name:      "test-delete",
		},
	})

	// delete should not invalidate cache (to make children deletion possible)
	// add and update should

	addEntry, hasAdd := cache.data["ns/test-add"]
	updateEntry, hasUpdate := cache.data["ns/test-update"]
	deleteEntry, hasDelete := cache.data["ns/test-delete"]
	assert.True(t, hasAdd)
	assert.True(t, addEntry.dirty)
	assert.True(t, hasUpdate)
	assert.True(t, updateEntry.dirty)
	assert.True(t, hasDelete)
	assert.False(t, deleteEntry.dirty)
}

type testStore struct {
	store cache.Store
}

func (s *testStore) GetStore() cache.Store {
	return s.store
}

func (s *testStore) Run(<-chan struct{}) {
}

func TestStackListenerGetByKey(t *testing.T) {
	storeMock := &testStore{store: cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc)}
	storeMock.store.Add(&latest.Stack{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test-name",
		},
	})
	testee := &StackListener{stacks: storeMock}

	v, err := testee.get("test-ns/test-name")
	assert.NoError(t, err)
	assert.NotNil(t, v)
	_, err = testee.get("test-ns/no-exists")
	assert.EqualError(t, err, "not found: test-ns/no-exists")
}
