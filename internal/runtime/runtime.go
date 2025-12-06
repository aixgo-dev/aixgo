package runtime

import (
	"errors"
)

var (
	// ErrAgentNotFound is returned when an agent is not registered
	ErrAgentNotFound = errors.New("agent not found")

	// ErrAgentAlreadyRegistered is returned when trying to register an agent with a duplicate name
	ErrAgentAlreadyRegistered = errors.New("agent already registered")

	// ErrAgentNotReady is returned when trying to execute an agent that is not ready
	ErrAgentNotReady = errors.New("agent not ready")

	// ErrRuntimeNotStarted is returned when trying to use a runtime that hasn't been started
	ErrRuntimeNotStarted = errors.New("runtime not started")

	// ErrRuntimeAlreadyStarted is returned when trying to start an already running runtime
	ErrRuntimeAlreadyStarted = errors.New("runtime already started")
)

// RuntimeConfig contains configuration options for creating a runtime
type RuntimeConfig struct {
	// ChannelBufferSize sets the buffer size for message channels (LocalRuntime only)
	// Default: 100
	ChannelBufferSize int

	// MaxConcurrentCalls limits parallel agent executions (0 = unlimited)
	// Default: 0 (unlimited)
	MaxConcurrentCalls int

	// EnableMetrics enables runtime performance metrics collection
	// Default: true
	EnableMetrics bool
}

// DefaultConfig returns a RuntimeConfig with sensible defaults
func DefaultConfig() *RuntimeConfig {
	return &RuntimeConfig{
		ChannelBufferSize:  100,
		MaxConcurrentCalls: 0,
		EnableMetrics:      true,
	}
}

// Option is a functional option for configuring a runtime
type Option func(*RuntimeConfig)

// WithChannelBufferSize sets the channel buffer size for LocalRuntime
func WithChannelBufferSize(size int) Option {
	return func(cfg *RuntimeConfig) {
		cfg.ChannelBufferSize = size
	}
}

// WithMaxConcurrentCalls sets the maximum number of concurrent agent calls
func WithMaxConcurrentCalls(max int) Option {
	return func(cfg *RuntimeConfig) {
		cfg.MaxConcurrentCalls = max
	}
}

// WithMetrics enables or disables metrics collection
func WithMetrics(enabled bool) Option {
	return func(cfg *RuntimeConfig) {
		cfg.EnableMetrics = enabled
	}
}
