package controller

import (
	"sync"
	"time"

	"github.com/docker/compose-on-kubernetes/api/client/clientset"
	"github.com/docker/compose-on-kubernetes/api/client/clientset/scheme"
	"github.com/docker/compose-on-kubernetes/api/compose/latest"
	"github.com/docker/compose-on-kubernetes/internal/stackresources"
	log "github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
)

// StackOwnerCacher describes a component capable of caching stack ownership data
type StackOwnerCacher interface {
	remove(key string)
	setDirty(key string)
	getWithRetries(stack *latest.Stack, acceptDirty bool) (rest.ImpersonationConfig, error)
}

type ownerGetter interface {
	get(*latest.Stack) (*latest.Owner, error)
}

type stackOwnerCacheEntry struct {
	config rest.ImpersonationConfig
	dirty  bool
}

type stackOwnerCache struct {
	mut    sync.Mutex
	data   map[string]stackOwnerCacheEntry
	getter ownerGetter
}

type apiOwnerGetter struct {
	rest.Interface
}

func (g *apiOwnerGetter) get(stack *latest.Stack) (*latest.Owner, error) {
	var owner latest.Owner
	if err := g.Get().Namespace(stack.Namespace).Name(stack.Name).
		Resource("stacks").
		SubResource("owner").
		VersionedParams(&metav1.GetOptions{}, scheme.ParameterCodec).
		Do().
		Into(&owner); err != nil {
		return nil, err
	}
	return &owner, nil
}

// NewStackOwnerCache creates a stackOwnerCache
func NewStackOwnerCache(config *rest.Config) (StackOwnerCacher, error) {
	cs, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &stackOwnerCache{
		data:   make(map[string]stackOwnerCacheEntry),
		getter: &apiOwnerGetter{cs.ComposeLatest().RESTClient()},
	}, nil
}

func (s *stackOwnerCache) remove(key string) {
	s.mut.Lock()
	defer s.mut.Unlock()
	delete(s.data, key)
}

func (s *stackOwnerCache) setDirty(key string) {
	s.mut.Lock()
	defer s.mut.Unlock()
	if entry, ok := s.data[key]; ok {
		entry.dirty = true
		s.data[key] = entry
	}
}

func (s *stackOwnerCache) getWithError(stack *latest.Stack, acceptDirty bool) (rest.ImpersonationConfig, error) {
	s.mut.Lock()
	defer s.mut.Unlock()
	objKey := stackresources.ObjKey(stack.Namespace, stack.Name)
	if v, ok := s.data[objKey]; ok {
		if !v.dirty || acceptDirty {
			return v.config, nil
		}
	}
	owner, err := s.getter.get(stack)
	if err != nil {
		log.Errorf("Unable to get stack %q owner: %s", objKey, err)
		if kerrors.IsNotFound(err) {
			if v, ok := s.data[objKey]; ok {
				log.Infof("Stack %q seem to have been deleted. Fallback to dirty impersonation config", objKey)
				return v.config, nil

			}
		}
		return rest.ImpersonationConfig{}, err
	}
	log.Debugf("Stack %s/%s owner is %s", stack.Namespace, stack.Name, owner.Owner.UserName)
	ic := rest.ImpersonationConfig{
		UserName: owner.Owner.UserName,
		Groups:   owner.Owner.Groups,
		Extra:    owner.Owner.Extra,
	}
	s.data[objKey] = stackOwnerCacheEntry{dirty: false, config: ic}
	return ic, nil
}

func (s *stackOwnerCache) getWithRetries(stack *latest.Stack, acceptDirty bool) (rest.ImpersonationConfig, error) {
	var ic rest.ImpersonationConfig
	err := wait.ExponentialBackoff(wait.Backoff{
		Duration: time.Second,
		Factor:   2,
		Jitter:   float64(100 * time.Millisecond),
		Steps:    4,
	}, func() (done bool, err error) {
		res, err := s.getWithError(stack, acceptDirty)
		if err == nil {
			ic = res
			return true, nil
		}
		if kerrors.IsNotFound(err) {
			// stack has been removed and we don't have anything in cache, but still the reconciler wants to update
			// this can happen when a stack is removed while the controller is starting, or when an informer
			// somehow fails to watch an event (which seems to be possible on Docker Desktop after a machine gets to sleep and a compaction
			// occurs immediately after)
			// So here we pass the error to the caller to let the process crash and be re-scheduled
			return false, err
		}
		return false, nil
	})
	return ic, err
}
