package deduplication

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDeduplicateCorrectly(t *testing.T) {
	c := NewStringChan(5)
	c.In() <- "test"
	c.In() <- "test"
	c.In() <- "test"
	c.In() <- "test"
	c.In() <- "test"
	c.Close()
	count := 0
	for v := range c.Out() {
		assert.Equal(t, "test", v)
		count++
	}
	assert.True(t, count >= 1 && count <= 5)
}

func TestWithLoadAtLeastOne(t *testing.T) {
	rand.Seed(time.Now().UTC().UnixNano())
	c := NewStringChan(20)
	generated := make(map[string]struct{})
	consumed := make(map[string]struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer c.Close()
		for i := 0; i < 1000000; i++ {
			v := rand.Intn(20)
			testVal := fmt.Sprintf("test %d", v)
			c.In() <- testVal
			generated[testVal] = struct{}{}
		}
	}()
	go func() {
		defer wg.Done()
		for v := range c.Out() {
			consumed[v] = struct{}{}
		}
	}()
	wg.Wait()
	assert.Equal(t, generated, consumed)
}
