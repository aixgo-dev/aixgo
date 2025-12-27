// Package agent provides the public interfaces for building agents with Aixgo.
//
// This package exports the core Agent, Message, and Runtime interfaces that external
// projects need to build custom agents or interact with the Aixgo framework.
//
// # Basic Usage
//
// To create a custom agent, implement the Agent interface:
//
//	type MyAgent struct {
//	    name string
//	    ready bool
//	}
//
//	func (a *MyAgent) Name() string { return a.name }
//	func (a *MyAgent) Role() string { return "custom" }
//	func (a *MyAgent) Ready() bool { return a.ready }
//
//	func (a *MyAgent) Start(ctx context.Context) error {
//	    a.ready = true
//	    // Start any background processing
//	    return nil
//	}
//
//	func (a *MyAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
//	    // Process the input and return a response
//	    return agent.NewMessage("response", result), nil
//	}
//
//	func (a *MyAgent) Stop(ctx context.Context) error {
//	    a.ready = false
//	    return nil
//	}
//
// # Runtime Usage
//
// Use LocalRuntime to coordinate multiple agents:
//
//	rt := agent.NewLocalRuntime()
//	rt.Register(myAgent)
//	rt.Start(ctx)
//
//	// Call an agent synchronously
//	response, err := rt.Call(ctx, "myagent", input)
//
//	// Call multiple agents in parallel
//	results, errs := rt.CallParallel(ctx, []string{"agent1", "agent2"}, input)
//
// # Message Format
//
// Messages are the standard unit of communication between agents:
//
//	msg := agent.NewMessage("analysis_request", payload).
//	    WithMetadata("priority", "high").
//	    WithMetadata("source", "api")
//
// See the Aixgo documentation at https://aixgo.dev for more examples and patterns.
package agent
