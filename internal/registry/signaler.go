package registry

import (
	"sync"
)

// signaler is a single-shot notification system through channel and callbacks
type signaler struct {
	callbacks map[int]func()
	signals   []chan struct{}
	mutex     sync.Mutex
	triggered bool
	nextIndex int
}

// newSignaler returns a new signaler
func newSignaler() *signaler {
	return &signaler{
		callbacks: make(map[int]func()),
	}
}

// Register registers a callback function on trigger and returns an uid
func (s *signaler) Register(f func()) int {
	s.mutex.Lock()
	if s.triggered {
		s.mutex.Unlock()
		f()
		return -1
	}
	idx := s.nextIndex
	s.nextIndex++
	s.callbacks[idx] = f
	s.mutex.Unlock()
	return idx
}

// Unregister unregisters a callback function by uid
func (s *signaler) Unregister(id int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.callbacks, id)
}

// Channel registers a channel reader.
func (s *signaler) Channel() chan struct{} {
	res := make(chan struct{}, 1)
	s.mutex.Lock()
	if s.triggered {
		res <- struct{}{}
	} else {
		s.signals = append(s.signals, res)
	}
	s.mutex.Unlock()
	return res
}

// Signal triggers the signaler
func (s *signaler) Signal() {
	s.mutex.Lock()
	if s.triggered {
		s.mutex.Unlock()
		return
	}
	cb := s.callbacks
	s.triggered = true
	for _, s := range s.signals {
		s <- struct{}{}
	}
	s.mutex.Unlock()
	// callbacks must be invoked without holding the lock since they might Unregister()
	for _, f := range cb {
		f()
	}
}

// Triggered check if the signaler was triggered
func (s *signaler) Triggered() bool {
	return s.triggered
}
