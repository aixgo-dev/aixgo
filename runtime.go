package aixgo

import (
	"fmt"
	"sync"

	"github.com/aixgo-dev/aixgo/internal/agent"
)

// SimpleRuntime is a basic in-memory implementation of the Runtime interface
type SimpleRuntime struct {
	channels map[string]chan *agent.Message
	mu       sync.RWMutex
}

// NewSimpleRuntime creates a new SimpleRuntime
func NewSimpleRuntime() *SimpleRuntime {
	return &SimpleRuntime{
		channels: make(map[string]chan *agent.Message),
	}
}

// Send sends a message to a target channel
func (r *SimpleRuntime) Send(target string, msg *agent.Message) error {
	r.mu.RLock()
	ch, ok := r.channels[target]
	r.mu.RUnlock()

	if !ok {
		// Create channel if it doesn't exist
		r.mu.Lock()
		if _, exists := r.channels[target]; !exists {
			r.channels[target] = make(chan *agent.Message, 100)
		}
		ch = r.channels[target]
		r.mu.Unlock()
	}

	select {
	case ch <- msg:
		return nil
	default:
		return fmt.Errorf("channel %s is full", target)
	}
}

// Recv returns a channel to receive messages from a source
func (r *SimpleRuntime) Recv(source string) (<-chan *agent.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.channels[source]; !ok {
		r.channels[source] = make(chan *agent.Message, 100)
	}

	return r.channels[source], nil
}
