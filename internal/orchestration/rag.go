package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// RAG implements Retrieval-Augmented Generation pattern.
// Retrieves relevant documents from a vector store, then generates grounded answers.
// Most common enterprise pattern for chatbots and Q&A systems.
//
// Use cases:
// - Enterprise chatbots
// - Documentation Q&A
// - Knowledge retrieval
// - Context-aware generation
type RAG struct {
	*BaseOrchestrator
	retriever string // Agent that retrieves relevant documents
	generator string // Agent that generates the answer
	topK      int    // Number of documents to retrieve
	rerank    bool   // Whether to rerank retrieved documents
	reranker  string // Optional reranker agent
}

// RAGOption configures a RAG orchestrator
type RAGOption func(*RAG)

// WithTopK sets the number of documents to retrieve
func WithTopK(k int) RAGOption {
	return func(r *RAG) {
		r.topK = k
	}
}

// WithReranker enables reranking with the specified agent
func WithReranker(reranker string) RAGOption {
	return func(r *RAG) {
		r.rerank = true
		r.reranker = reranker
	}
}

// NewRAG creates a new RAG orchestrator
func NewRAG(name string, runtime agent.Runtime, retriever, generator string, opts ...RAGOption) *RAG {
	r := &RAG{
		BaseOrchestrator: NewBaseOrchestrator(name, "rag", runtime),
		retriever:        retriever,
		generator:        generator,
		topK:             5, // Default top-5
		rerank:           false,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Execute performs RAG: retrieve → (optional rerank) → generate
func (r *RAG) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	ctx, span := observability.StartSpanWithOtel(ctx, fmt.Sprintf("orchestration.rag.%s", r.name),
		trace.WithAttributes(
			attribute.String("orchestration.pattern", "rag"),
			attribute.String("orchestration.retriever", r.retriever),
			attribute.String("orchestration.generator", r.generator),
			attribute.Int("orchestration.top_k", r.topK),
			attribute.Bool("orchestration.rerank", r.rerank),
		),
	)
	defer span.End()

	startTime := time.Now()

	// Step 1: Retrieve relevant documents
	retrieveStart := time.Now()
	retrieved, err := r.runtime.Call(ctx, r.retriever, input)
	retrieveDuration := time.Since(retrieveStart)

	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("retrieval failed: %w", err)
	}

	span.SetAttributes(
		attribute.Int64("orchestration.retrieve_duration_ms", retrieveDuration.Milliseconds()),
	)

	// Step 2: Optional reranking
	var documents *agent.Message
	if r.rerank && r.reranker != "" {
		rerankStart := time.Now()
		documents, err = r.runtime.Call(ctx, r.reranker, retrieved)
		rerankDuration := time.Since(rerankStart)

		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("reranking failed: %w", err)
		}

		span.SetAttributes(
			attribute.Int64("orchestration.rerank_duration_ms", rerankDuration.Milliseconds()),
		)
	} else {
		documents = retrieved
	}

	// Step 3: Generate answer with retrieved context
	// Combine original query with retrieved documents
	augmentedInput := augmentInput(input, documents)

	generateStart := time.Now()
	result, err := r.runtime.Call(ctx, r.generator, augmentedInput)
	generateDuration := time.Since(generateStart)

	totalDuration := time.Since(startTime)

	span.SetAttributes(
		attribute.Int64("orchestration.generate_duration_ms", generateDuration.Milliseconds()),
		attribute.Int64("orchestration.total_duration_ms", totalDuration.Milliseconds()),
		attribute.Bool("orchestration.success", err == nil),
	)

	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("generation failed: %w", err)
	}

	return result, nil
}

// augmentInput combines the original query with retrieved documents
func augmentInput(query, documents *agent.Message) *agent.Message {
	// TODO: Implement proper augmentation based on Message structure
	// Should create a new message with both query and context
	return query
}

// RAG variants

// NewConversationalRAG creates a RAG with conversation history
func NewConversationalRAG(name string, runtime agent.Runtime, retriever, generator string, historyAgent string) *RAG {
	// TODO: Implement conversation history tracking
	return NewRAG(name, runtime, retriever, generator)
}

// NewMultiQueryRAG creates a RAG that generates multiple queries for retrieval
func NewMultiQueryRAG(name string, runtime agent.Runtime, queryExpander, retriever, generator string) *RAG {
	// TODO: Implement multi-query expansion
	return NewRAG(name, runtime, retriever, generator)
}

// NewHybridRAG creates a RAG with both semantic and keyword search
func NewHybridRAG(name string, runtime agent.Runtime, semanticRetriever, keywordRetriever, generator string) *RAG {
	// TODO: Implement hybrid retrieval
	return NewRAG(name, runtime, semanticRetriever, generator)
}
