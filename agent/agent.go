package agent

import "context"

// Agent is the interface that all agents must implement.
// External packages should implement this interface for custom agents.
//
// Agents support both synchronous (Execute) and asynchronous (Start) execution modes.
// The Execute method is used for request-response patterns, while Start is used for
// agents that run continuously and process messages asynchronously.
type Agent interface {
	// Name returns the unique identifier for this agent instance.
	// Agent names must be unique within a Runtime.
	Name() string

	// Role returns the agent's role type (e.g., "react", "classifier", "supervisor").
	// The role determines the agent's behavior and capabilities.
	Role() string

	// Start initializes the agent and prepares it to receive messages.
	// This method is called when the Runtime starts the agent.
	// For asynchronous agents, this typically runs a message processing loop.
	// The method should block until the context is canceled or the agent encounters a fatal error.
	Start(ctx context.Context) error

	// Execute processes an input message and returns a response synchronously.
	// This method is used by orchestration patterns for direct agent invocation.
	// The implementation should be idempotent and thread-safe.
	Execute(ctx context.Context, input *Message) (*Message, error)

	// Stop gracefully shuts down the agent.
	// This method is called when the Runtime stops the agent or when the context is canceled.
	// Implementations should clean up resources and ensure all pending operations complete.
	Stop(ctx context.Context) error

	// Ready returns true if the agent is ready to process messages.
	// The Runtime will not invoke Execute on an agent that is not ready.
	Ready() bool
}
