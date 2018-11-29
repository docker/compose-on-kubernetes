package deduplication

import "sync"

// StringChan is a deduplicating string channel
type StringChan struct {
	mut     sync.Mutex
	content map[string]struct{}
	ch      chan string
	input   chan string
	output  chan string
}

// NewStringChan creates a StringChan with the specified buffer size
func NewStringChan(bufferSize int) *StringChan {
	result := &StringChan{ch: make(chan string, bufferSize), content: make(map[string]struct{}, bufferSize+1), input: make(chan string), output: make(chan string)}
	go func() {
		defer close(result.ch)
		for v := range result.input {
			result.push(v)
		}
	}()
	go func() {
		defer close(result.output)
		for {
			v, ok := result.pull()
			if !ok {
				return
			}
			result.output <- v
		}
	}()
	return result
}

// Close releases all resources
func (c *StringChan) Close() {
	close(c.input)
}

// Push pushes a value if it is not in the buffer
func (c *StringChan) push(v string) {
	added := func() bool {
		c.mut.Lock()
		defer c.mut.Unlock()
		if _, ok := c.content[v]; ok {
			return false
		}
		c.content[v] = struct{}{}
		return true
	}()
	if added {
		c.ch <- v
	}
}

// Pull consumes a value
func (c *StringChan) pull() (string, bool) {
	v, ok := <-c.ch
	c.mut.Lock()
	defer c.mut.Unlock()
	delete(c.content, v)
	return v, ok
}

// In returns the input channel of the deduplicator
func (c *StringChan) In() chan<- string {
	return c.input
}

// Out returns the output channel of the deduplicator
func (c *StringChan) Out() <-chan string {
	return c.output
}
