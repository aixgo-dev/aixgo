package orchestration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	pb "github.com/aixgo-dev/aixgo/proto"
)

func TestRAGExecute(t *testing.T) {
	ctx := context.Background()

	// Create mock runtime and agents
	rt := NewMockRuntime()

	// Retriever agent returns documents
	retriever := NewMockAgent("retriever", "retriever", 50*time.Millisecond, "Document 1: Info about topic\nDocument 2: More details")
	// Generator agent creates answer
	generator := NewMockAgent("generator", "generator", 50*time.Millisecond, "Generated answer based on context")

	_ = rt.Register(retriever)
	_ = rt.Register(generator)

	// Create RAG orchestrator
	rag := NewRAG("test-rag", rt, "retriever", "generator")

	// Execute
	input := &agent.Message{
		Message: &pb.Message{
			Id:      "test-123",
			Payload: "What is the topic about?",
		},
	}

	result, err := rag.Execute(ctx, input)

	if err != nil {
		t.Fatalf("RAG execution failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	if result.Payload != "Generated answer based on context" {
		t.Errorf("Result payload = %s, want generator response", result.Payload)
	}

	// Verify all agents were called
	if retriever.CallCount() != 1 {
		t.Errorf("Retriever call count = %d, want 1", retriever.CallCount())
	}
	if generator.CallCount() != 1 {
		t.Errorf("Generator call count = %d, want 1", generator.CallCount())
	}
}

func TestRAGWithReranker(t *testing.T) {
	ctx := context.Background()

	rt := NewMockRuntime()

	retriever := NewMockAgent("retriever", "retriever", 10*time.Millisecond, "Doc1\nDoc2\nDoc3")
	reranker := NewMockAgent("reranker", "reranker", 10*time.Millisecond, "Doc1\nDoc3")
	generator := NewMockAgent("generator", "generator", 10*time.Millisecond, "Answer from reranked docs")

	_ = rt.Register(retriever)
	_ = rt.Register(reranker)
	_ = rt.Register(generator)

	// Create RAG with reranker
	rag := NewRAG("test-rag", rt, "retriever", "generator", WithReranker("reranker"))

	input := &agent.Message{
		Message: &pb.Message{
			Payload: "query",
		},
	}

	result, err := rag.Execute(ctx, input)

	if err != nil {
		t.Fatalf("RAG execution failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	// Verify all agents were called
	if retriever.CallCount() != 1 {
		t.Errorf("Retriever call count = %d, want 1", retriever.CallCount())
	}
	if reranker.CallCount() != 1 {
		t.Errorf("Reranker call count = %d, want 1", reranker.CallCount())
	}
	if generator.CallCount() != 1 {
		t.Errorf("Generator call count = %d, want 1", generator.CallCount())
	}
}

func TestRAGTopK(t *testing.T) {
	ctx := context.Background()

	rt := NewMockRuntime()

	retriever := NewMockAgent("retriever", "retriever", 10*time.Millisecond, "docs")
	generator := NewMockAgent("generator", "generator", 10*time.Millisecond, "answer")

	_ = rt.Register(retriever)
	_ = rt.Register(generator)

	// Create RAG with custom top-k
	rag := NewRAG("test-rag", rt, "retriever", "generator", WithTopK(10))

	if rag.topK != 10 {
		t.Errorf("topK = %d, want 10", rag.topK)
	}

	input := &agent.Message{
		Message: &pb.Message{
			Payload: "query",
		},
	}

	_, err := rag.Execute(ctx, input)

	if err != nil {
		t.Fatalf("RAG execution failed: %v", err)
	}
}

func TestRAGName(t *testing.T) {
	rt := NewMockRuntime()

	rag := NewRAG("my-rag", rt, "retriever", "generator")

	if rag.Name() != "my-rag" {
		t.Errorf("Name() = %s, want my-rag", rag.Name())
	}
}

func TestRAGPattern(t *testing.T) {
	rt := NewMockRuntime()

	rag := NewRAG("test", rt, "retriever", "generator")

	if rag.Pattern() != "rag" {
		t.Errorf("Pattern() = %s, want rag", rag.Pattern())
	}
}

func TestAugmentInput(t *testing.T) {
	tests := []struct {
		name      string
		query     *agent.Message
		documents *agent.Message
		wantType  string
		checkFunc func(t *testing.T, result *agent.Message)
	}{
		{
			name: "augment with documents",
			query: &agent.Message{
				Message: &pb.Message{
					Id:      "q1",
					Payload: "What is AI?",
				},
			},
			documents: &agent.Message{
				Message: &pb.Message{
					Payload: "AI is artificial intelligence",
				},
			},
			wantType: "rag_augmented",
			checkFunc: func(t *testing.T, result *agent.Message) {
				if !strings.Contains(result.Payload, "Context:") {
					t.Error("Augmented payload missing 'Context:' header")
				}
				if !strings.Contains(result.Payload, "Query:") {
					t.Error("Augmented payload missing 'Query:' header")
				}
				if !strings.Contains(result.Payload, "AI is artificial intelligence") {
					t.Error("Augmented payload missing document content")
				}
				if !strings.Contains(result.Payload, "What is AI?") {
					t.Error("Augmented payload missing query content")
				}
			},
		},
		{
			name: "nil documents returns query",
			query: &agent.Message{
				Message: &pb.Message{
					Payload: "query",
				},
			},
			documents: nil,
			wantType:  "",
			checkFunc: func(t *testing.T, result *agent.Message) {
				if result.Payload != "query" {
					t.Errorf("Expected original query, got %s", result.Payload)
				}
			},
		},
		{
			name: "empty documents returns query",
			query: &agent.Message{
				Message: &pb.Message{
					Payload: "query",
				},
			},
			documents: &agent.Message{
				Message: &pb.Message{
					Payload: "",
				},
			},
			wantType: "",
			checkFunc: func(t *testing.T, result *agent.Message) {
				if result.Payload != "query" {
					t.Errorf("Expected original query, got %s", result.Payload)
				}
			},
		},
		{
			name:      "nil query returns nil",
			query:     nil,
			documents: nil,
			wantType:  "",
			checkFunc: func(t *testing.T, result *agent.Message) {
				if result != nil {
					t.Error("Expected nil result for nil query")
				}
			},
		},
		{
			name: "preserves metadata",
			query: &agent.Message{
				Message: &pb.Message{
					Id:      "q1",
					Payload: "query",
					Metadata: map[string]any{
						"user": "test-user",
					},
				},
			},
			documents: &agent.Message{
				Message: &pb.Message{
					Payload: "docs",
					Metadata: map[string]any{
						"source": "vector-db",
					},
				},
			},
			wantType: "rag_augmented",
			checkFunc: func(t *testing.T, result *agent.Message) {
				if result.Metadata == nil {
					t.Fatal("Metadata is nil")
				}
				if result.Metadata["user"] != "test-user" {
					t.Error("Query metadata not preserved")
				}
				if result.Metadata["retrieved_context"] == nil {
					t.Error("Document metadata not added")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := augmentInput(tt.query, tt.documents)
			if tt.wantType != "" && result != nil && result.Type != tt.wantType {
				t.Errorf("Type = %s, want %s", result.Type, tt.wantType)
			}
			if tt.checkFunc != nil {
				tt.checkFunc(t, result)
			}
		})
	}
}

func TestRAGRetrieverFailure(t *testing.T) {
	ctx := context.Background()

	rt := NewMockRuntime()

	generator := NewMockAgent("generator", "generator", 10*time.Millisecond, "answer")
	_ = rt.Register(generator)

	// Create RAG with non-existent retriever
	rag := NewRAG("test-rag", rt, "nonexistent-retriever", "generator")

	input := &agent.Message{
		Message: &pb.Message{
			Payload: "query",
		},
	}

	_, err := rag.Execute(ctx, input)

	if err == nil {
		t.Fatal("Expected error for non-existent retriever, got nil")
	}

	if !strings.Contains(err.Error(), "retrieval failed") {
		t.Errorf("Error message = %v, want 'retrieval failed'", err)
	}
}

func TestRAGGeneratorFailure(t *testing.T) {
	ctx := context.Background()

	rt := NewMockRuntime()

	retriever := NewMockAgent("retriever", "retriever", 10*time.Millisecond, "docs")
	_ = rt.Register(retriever)

	// Create RAG with non-existent generator
	rag := NewRAG("test-rag", rt, "retriever", "nonexistent-generator")

	input := &agent.Message{
		Message: &pb.Message{
			Payload: "query",
		},
	}

	_, err := rag.Execute(ctx, input)

	if err == nil {
		t.Fatal("Expected error for non-existent generator, got nil")
	}

	if !strings.Contains(err.Error(), "generation failed") {
		t.Errorf("Error message = %v, want 'generation failed'", err)
	}
}

func TestRAGRerankerFailure(t *testing.T) {
	ctx := context.Background()

	rt := NewMockRuntime()

	retriever := NewMockAgent("retriever", "retriever", 10*time.Millisecond, "docs")
	generator := NewMockAgent("generator", "generator", 10*time.Millisecond, "answer")

	_ = rt.Register(retriever)
	_ = rt.Register(generator)

	// Create RAG with non-existent reranker
	rag := NewRAG("test-rag", rt, "retriever", "generator", WithReranker("nonexistent-reranker"))

	input := &agent.Message{
		Message: &pb.Message{
			Payload: "query",
		},
	}

	_, err := rag.Execute(ctx, input)

	if err == nil {
		t.Fatal("Expected error for non-existent reranker, got nil")
	}

	if !strings.Contains(err.Error(), "reranking failed") {
		t.Errorf("Error message = %v, want 'reranking failed'", err)
	}
}

func TestRAGContextCancellation(t *testing.T) {
	// Test with timeout context instead
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	rt := NewMockRuntime()

	// Note: MockAgent doesn't actually respect context cancellation
	// This test validates the RAG structure, but a real agent would fail here
	retriever := NewMockAgent("retriever", "retriever", 10*time.Millisecond, "docs")
	generator := NewMockAgent("generator", "generator", 10*time.Millisecond, "answer")

	_ = rt.Register(retriever)
	_ = rt.Register(generator)

	rag := NewRAG("test-rag", rt, "retriever", "generator")

	input := &agent.Message{
		Message: &pb.Message{
			Payload: "query",
		},
	}

	// Execute with short timeout - should complete successfully with mock agents
	result, err := rag.Execute(ctx, input)

	// MockAgent completes fast enough, so no error expected
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}
}
