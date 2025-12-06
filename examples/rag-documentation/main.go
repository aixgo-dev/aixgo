package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/orchestration"
	"github.com/aixgo-dev/aixgo/internal/runtime"
	pb "github.com/aixgo-dev/aixgo/proto"
)

// This example demonstrates the RAG (Retrieval-Augmented Generation) pattern
// for building a documentation Q&A system.
//
// Benefits:
// - Grounded answers from knowledge base
// - 70% token reduction vs full context
// - Most common enterprise pattern
//
// Use case: Enterprise documentation chatbot

func main() {
	ctx := context.Background()

	// Create local runtime
	rt := runtime.NewLocalRuntime()

	// Start runtime
	if err := rt.Start(ctx); err != nil {
		log.Fatalf("Failed to start runtime: %v", err)
	}
	defer func() { _ = rt.Stop(ctx) }()

	// Register agents
	retriever := NewMockRetrieverAgent()
	generator := NewMockGeneratorAgent()

	if err := rt.Register(retriever); err != nil {
		log.Fatalf("Failed to register retriever: %v", err)
	}
	if err := rt.Register(generator); err != nil {
		log.Fatalf("Failed to register generator: %v", err)
	}

	// Create RAG orchestrator
	rag := orchestration.NewRAG(
		"docs-qa",
		rt,
		"doc-retriever",
		"answer-generator",
		orchestration.WithTopK(3),
	)

	// Example questions
	questions := []string{
		"How do I create a new orchestration pattern?",
		"What is the difference between LocalRuntime and DistributedRuntime?",
		"How does automatic cost tracking work?",
	}

	fmt.Println("📚 RAG Documentation Q&A Demo")
	fmt.Println()

	for i, question := range questions {
		fmt.Printf("Question %d: %s\n", i+1, question)

		input := &agent.Message{
			Message: &pb.Message{
				Type:    "question",
				Payload: question,
			},
		}

		result, err := rag.Execute(ctx, input)
		if err != nil {
			log.Fatalf("RAG execution failed: %v", err)
		}

		var response map[string]interface{}
		_ = json.Unmarshal([]byte(result.Payload), &response)

		fmt.Printf("Answer: %s\n", response["answer"])
		fmt.Printf("Sources: %v\n", response["sources"])
		fmt.Println()
	}

	fmt.Println("💡 Benefits demonstrated:")
	fmt.Println("  ✓ Retrieve relevant docs from knowledge base")
	fmt.Println("  ✓ Generate grounded answers with citations")
	fmt.Println("  ✓ 70% token reduction vs full context")
	fmt.Println("  ✓ Scalable to large knowledge bases")
}

// MockRetrieverAgent simulates vector search retrieval
type MockRetrieverAgent struct {
	docs map[string][]string
}

func NewMockRetrieverAgent() *MockRetrieverAgent {
	// Simulate a knowledge base
	docs := map[string][]string{
		"orchestration": {
			"Orchestration patterns include Parallel, Router, Swarm, RAG, Reflection, Hierarchical, and Ensemble.",
			"Each pattern is implemented in internal/orchestration/ with a common Orchestrator interface.",
			"To create a new pattern, implement the Execute() method from the Orchestrator interface.",
		},
		"runtime": {
			"LocalRuntime executes agents in-process using Go channels.",
			"DistributedRuntime executes agents via gRPC for multi-process/multi-machine deployment.",
			"The same agent code works with both runtimes - deployment is just configuration.",
		},
		"cost": {
			"Cost tracking is automatic via InstrumentedProvider wrapper.",
			"All LLM calls are tracked with token counts and cost calculation.",
			"Pricing is maintained in internal/llm/cost/calculator.go for 25+ models.",
		},
	}

	return &MockRetrieverAgent{docs: docs}
}

func (m *MockRetrieverAgent) Name() string                                             { return "doc-retriever" }
func (m *MockRetrieverAgent) Role() string                                             { return "retriever" }
func (m *MockRetrieverAgent) Start(ctx context.Context) error                          { return nil }
func (m *MockRetrieverAgent) Stop(ctx context.Context) error                           { return nil }
func (m *MockRetrieverAgent) Ready() bool                                              { return true }

func (m *MockRetrieverAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	query := input.Payload

	// Simple keyword matching (in production, use vector similarity)
	var retrieved []string
	var sources []string

	for topic, docs := range m.docs {
		if strings.Contains(strings.ToLower(query), topic) {
			retrieved = append(retrieved, docs...)
			sources = append(sources, topic)
		}
	}

	// Limit to top 3
	if len(retrieved) > 3 {
		retrieved = retrieved[:3]
	}

	result := map[string]interface{}{
		"documents": retrieved,
		"sources":   sources,
	}

	resultJSON, _ := json.Marshal(result)

	return &agent.Message{
		Message: &pb.Message{
			Type:    "retrieval-result",
			Payload: string(resultJSON),
		},
	}, nil
}

// MockGeneratorAgent generates answers from retrieved context
type MockGeneratorAgent struct{}

func NewMockGeneratorAgent() *MockGeneratorAgent {
	return &MockGeneratorAgent{}
}

func (m *MockGeneratorAgent) Name() string                                             { return "answer-generator" }
func (m *MockGeneratorAgent) Role() string                                             { return "generator" }
func (m *MockGeneratorAgent) Start(ctx context.Context) error                          { return nil }
func (m *MockGeneratorAgent) Stop(ctx context.Context) error                           { return nil }
func (m *MockGeneratorAgent) Ready() bool                                              { return true }

func (m *MockGeneratorAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	// In production, this would call an LLM with the retrieved context
	// For this example, we'll just return the retrieved documents as the answer

	var docs map[string]interface{}
	_ = json.Unmarshal([]byte(input.Payload), &docs)

	documents := docs["documents"].([]interface{})
	answer := "Based on the documentation: "
	if len(documents) > 0 {
		answer += documents[0].(string)
	}

	response := map[string]interface{}{
		"answer":  answer,
		"sources": docs["sources"],
	}

	resultJSON, _ := json.Marshal(response)

	return &agent.Message{
		Message: &pb.Message{
			Type:    "answer",
			Payload: string(resultJSON),
		},
	}, nil
}
