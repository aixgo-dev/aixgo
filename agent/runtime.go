package agent

import "context"

// Runtime provides the message passing and coordination infrastructure for agents.
// It supports both local (single binary) and distributed (gRPC) deployment modes.
//
// The Runtime manages agent lifecycle, message routing, and orchestration patterns.
// It provides both synchronous (Call) and asynchronous (Send/Recv) communication.
type Runtime interface {
	// Register adds an agent to the runtime.
	// Returns an error if an agent with the same name is already registered.
	Register(agent Agent) error

	// Unregister removes an agent from the runtime.
	// Returns an error if the agent is not found.
	Unregister(name string) error

	// Get retrieves a registered agent by name.
	// Returns an error if the agent is not found.
	Get(name string) (Agent, error)

	// List returns all registered agent names.
	List() []string

	// Call sends a message to an agent and waits for a synchronous response.
	// This is used for request-response patterns and orchestration.
	// The target agent's Execute method is invoked.
	// Returns an error if the agent is not found, not ready, or execution fails.
	Call(ctx context.Context, target string, input *Message) (*Message, error)

	// CallParallel invokes multiple agents concurrently and returns all results.
	// Execution continues even if some agents fail (partial results are returned).
	// Returns a map of successful responses and a map of errors keyed by agent name.
	CallParallel(ctx context.Context, targets []string, input *Message) (map[string]*Message, map[string]error)

	// Send sends a message to an agent asynchronously without waiting for a response.
	// The message is placed in the target agent's message channel.
	// Returns an error if the channel is full or the target is not found.
	Send(target string, msg *Message) error

	// Recv returns a channel to receive messages from an agent.
	// This is used for asynchronous message passing patterns.
	// The channel is created if it doesn't exist.
	Recv(source string) (<-chan *Message, error)

	// Broadcast sends a message to all registered agents asynchronously.
	// Returns an error if any send operation fails (but continues sending to others).
	Broadcast(msg *Message) error

	// Start starts the runtime and all registered agents.
	// For distributed runtimes, this starts the gRPC server.
	// For local runtimes, this starts all agent Start methods.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the runtime and all registered agents.
	// This should clean up all resources and ensure pending operations complete.
	Stop(ctx context.Context) error
}
