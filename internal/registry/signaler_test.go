package registry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSignaler(t *testing.T) {
	s := newSignaler()
	t1 := false
	h1 := s.Register(func() { t1 = true })
	s.Unregister(h1)
	t2 := false
	s.Register(func() { t2 = true })
	forwarder := make(chan struct{}, 10)
	c1 := s.Channel()
	go func() {
		<-c1
		forwarder <- struct{}{}
	}()
	c2 := s.Channel()
	go func() {
		<-c2
		forwarder <- struct{}{}
	}()
	s.Signal()
	assert.Equal(t, t1, false)
	assert.Equal(t, t2, true)
	for i := 0; i < 2; i++ {
		select {
		case <-time.After(1 * time.Second):
			t.Errorf("Timeout waiting for signaler channel")
		case <-forwarder:
		}
	}
	// check registrations after signal
	s.Register(func() { t1 = true })
	assert.Equal(t, t1, true)
	c3 := s.Channel()
	go func() {
		<-c3
		forwarder <- struct{}{}
	}()
	select {
	case <-time.After(1 * time.Second):
		t.Errorf("Timeout waiting for signaler channel")
	case <-forwarder:
	}
}
