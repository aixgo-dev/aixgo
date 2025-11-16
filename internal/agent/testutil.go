package agent

import (
	"context"
	"sync"
	"time"
)

// MockRuntime is a mock implementation of the Runtime interface for testing
type MockRuntime struct {
	channels    map[string]chan *Message
	sendCalls   []SendCall
	recvCalls   []string
	sendError   error
	recvError   error
	mu          sync.RWMutex
}

// SendCall records a call to Send
type SendCall struct {
	Target  string
	Message *Message
}

// NewMockRuntime creates a new mock runtime
func NewMockRuntime() *MockRuntime {
	return &MockRuntime{
		channels:  make(map[string]chan *Message),
		sendCalls: make([]SendCall, 0),
		recvCalls: make([]string, 0),
	}
}

// Send implements Runtime.Send
func (m *MockRuntime) Send(target string, msg *Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sendCalls = append(m.sendCalls, SendCall{Target: target, Message: msg})

	if m.sendError != nil {
		return m.sendError
	}

	if ch, ok := m.channels[target]; ok {
		select {
		case ch <- msg:
			return nil
		case <-time.After(100 * time.Millisecond):
			// Non-blocking send with timeout to prevent test hangs
			return nil
		}
	}

	return nil
}

// Recv implements Runtime.Recv
func (m *MockRuntime) Recv(source string) (<-chan *Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.recvCalls = append(m.recvCalls, source)

	if m.recvError != nil {
		return nil, m.recvError
	}

	if _, ok := m.channels[source]; !ok {
		m.channels[source] = make(chan *Message, 100)
	}

	return m.channels[source], nil
}

// GetSendCalls returns all recorded Send calls
func (m *MockRuntime) GetSendCalls() []SendCall {
	m.mu.RLock()
	defer m.mu.RUnlock()

	calls := make([]SendCall, len(m.sendCalls))
	copy(calls, m.sendCalls)
	return calls
}

// GetRecvCalls returns all recorded Recv calls
func (m *MockRuntime) GetRecvCalls() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	calls := make([]string, len(m.recvCalls))
	copy(calls, m.recvCalls)
	return calls
}

// SetSendError sets an error to return from Send
func (m *MockRuntime) SetSendError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendError = err
}

// SetRecvError sets an error to return from Recv
func (m *MockRuntime) SetRecvError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recvError = err
}

// SendMessage sends a message to a channel (for testing)
func (m *MockRuntime) SendMessage(source string, msg *Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.channels[source]; !ok {
		m.channels[source] = make(chan *Message, 100)
	}

	select {
	case m.channels[source] <- msg:
	case <-time.After(100 * time.Millisecond):
		// Non-blocking to prevent test hangs
	}
}

// Close closes all channels
func (m *MockRuntime) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ch := range m.channels {
		close(ch)
	}
}

// ContextWithRuntime creates a context with the given runtime
func ContextWithRuntime(ctx context.Context, rt Runtime) context.Context {
	return context.WithValue(ctx, RuntimeKey{}, rt)
}

// TestAgentDef creates a test AgentDef with sensible defaults
func TestAgentDef(name, role string) AgentDef {
	return AgentDef{
		Name:     name,
		Role:     role,
		Interval: Duration{Duration: 1 * time.Second},
		Inputs:   []Input{{Source: "test-input"}},
		Outputs:  []Output{{Target: "test-output"}},
		Model:    "test-model",
		Prompt:   "test prompt",
		Tools:    []Tool{},
		Extra:    make(map[string]any),
	}
}
