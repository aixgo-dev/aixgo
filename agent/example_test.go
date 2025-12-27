package agent_test

import (
	"context"
	"fmt"

	"github.com/aixgo-dev/aixgo/agent"
)

// AnalyzerAgent is an example custom agent for document analysis
type AnalyzerAgent struct {
	name  string
	ready bool
}

func NewAnalyzerAgent(name string) *AnalyzerAgent {
	return &AnalyzerAgent{
		name:  name,
		ready: false,
	}
}

func (a *AnalyzerAgent) Name() string { return a.name }
func (a *AnalyzerAgent) Role() string { return "analyzer" }
func (a *AnalyzerAgent) Ready() bool  { return a.ready }

func (a *AnalyzerAgent) Start(ctx context.Context) error {
	a.ready = true
	<-ctx.Done()
	return nil
}

func (a *AnalyzerAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	// Simulate document analysis
	type AnalysisRequest struct {
		DocumentID string `json:"document_id"`
		Content    string `json:"content"`
	}

	var req AnalysisRequest
	if err := input.UnmarshalPayload(&req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Perform analysis (simplified)
	result := map[string]interface{}{
		"document_id": req.DocumentID,
		"status":      "analyzed",
		"word_count":  len(req.Content),
	}

	return agent.NewMessage("analysis_result", result), nil
}

func (a *AnalyzerAgent) Stop(ctx context.Context) error {
	a.ready = false
	return nil
}

// Example demonstrates how to use the agent package
func Example() {
	// Create a runtime
	rt := agent.NewLocalRuntime()

	// Register custom agents
	analyzer := NewAnalyzerAgent("document-analyzer")
	rt.Register(analyzer)

	// Start the runtime
	ctx := context.Background()
	analyzer.ready = true // Simulate ready state for example

	// Create an analysis request
	type Request struct {
		DocumentID string `json:"document_id"`
		Content    string `json:"content"`
	}

	input := agent.NewMessage("analyze", Request{
		DocumentID: "doc-123",
		Content:    "This is a sample privacy policy document.",
	}).WithMetadata("priority", "high")

	// Call the analyzer synchronously
	response, err := rt.Call(ctx, "document-analyzer", input)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Process the response
	type Result struct {
		DocumentID string `json:"document_id"`
		Status     string `json:"status"`
		WordCount  int    `json:"word_count"`
	}

	var result Result
	if err := response.UnmarshalPayload(&result); err != nil {
		fmt.Printf("Error unmarshaling: %v\n", err)
		return
	}

	fmt.Printf("Analysis complete: %s (status: %s, words: %d)\n",
		result.DocumentID, result.Status, result.WordCount)

	// Output:
	// Analysis complete: doc-123 (status: analyzed, words: 41)
}

// Example_parallelAnalysis demonstrates parallel agent execution
func Example_parallelAnalysis() {
	rt := agent.NewLocalRuntime()

	// Register multiple analyzers
	rt.Register(NewAnalyzerAgent("syntax-analyzer"))
	rt.Register(NewAnalyzerAgent("risk-analyzer"))
	rt.Register(NewAnalyzerAgent("compliance-analyzer"))

	ctx := context.Background()

	// Mark all as ready (in real usage, Start would be called)
	for _, name := range []string{"syntax-analyzer", "risk-analyzer", "compliance-analyzer"} {
		a, _ := rt.Get(name)
		if analyzer, ok := a.(*AnalyzerAgent); ok {
			analyzer.ready = true
		}
	}

	// Prepare input
	type Request struct {
		DocumentID string `json:"document_id"`
		Content    string `json:"content"`
	}

	input := agent.NewMessage("analyze", Request{
		DocumentID: "doc-456",
		Content:    "Privacy policy content...",
	})

	// Call all analyzers in parallel
	targets := []string{"syntax-analyzer", "risk-analyzer", "compliance-analyzer"}
	results, errors := rt.CallParallel(ctx, targets, input)

	// Check results
	fmt.Printf("Completed: %d/%d analyzers\n", len(results), len(targets))
	if len(errors) > 0 {
		fmt.Printf("Errors: %d\n", len(errors))
	} else {
		fmt.Println("All analyzers completed successfully")
	}

	// Output:
	// Completed: 3/3 analyzers
	// All analyzers completed successfully
}

// Example_messageMetadata demonstrates metadata usage
func Example_messageMetadata() {
	// Create a message with metadata for tracing and correlation
	msg := agent.NewMessage("request", map[string]string{"action": "analyze"}).
		WithMetadata("correlation_id", "req-123").
		WithMetadata("user_id", "user-456").
		WithMetadata("priority", "high").
		WithMetadata("source", "api")

	// Access metadata
	correlationID := msg.GetMetadataString("correlation_id", "")
	priority := msg.GetMetadataString("priority", "normal")

	fmt.Printf("Processing request %s with priority %s\n", correlationID, priority)

	// Output:
	// Processing request req-123 with priority high
}

// Example_asyncCommunication demonstrates asynchronous message passing
func Example_asyncCommunication() {
	rt := agent.NewLocalRuntime()

	agent1 := NewAnalyzerAgent("agent1")
	agent1.ready = true
	rt.Register(agent1)

	// Get a channel to receive messages
	recvCh, _ := rt.Recv("agent1")

	// Send a message asynchronously
	msg := agent.NewMessage("notification", map[string]string{
		"event": "document_updated",
	})
	rt.Send("agent1", msg)

	// Receive and process
	received := <-recvCh
	fmt.Printf("Received message type: %s\n", received.Type)

	// Output:
	// Received message type: notification
}
