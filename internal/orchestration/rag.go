package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/observability"
	pb "github.com/aixgo-dev/aixgo/proto"
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
	retriever        string              // Agent that retrieves relevant documents
	generator        string              // Agent that generates the answer
	topK             int                 // Number of documents to retrieve
	rerank           bool                // Whether to rerank retrieved documents
	reranker         string              // Optional reranker agent
	conversationHist []ConversationTurn  // For conversational RAG
	historyAgent     string              // Agent for managing history
	queryExpander    string              // For multi-query RAG
	keywordRetriever string              // For hybrid RAG
}

// ConversationTurn represents a single turn in conversation history
type ConversationTurn struct {
	Query    string
	Response string
	Context  string
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
			attribute.Bool("orchestration.conversational", len(r.conversationHist) > 0 || r.historyAgent != ""),
			attribute.Bool("orchestration.multi_query", r.queryExpander != ""),
			attribute.Bool("orchestration.hybrid", r.keywordRetriever != ""),
		),
	)
	defer span.End()

	startTime := time.Now()

	// Handle conversational RAG: augment query with history
	queryInput := input
	if len(r.conversationHist) > 0 {
		queryInput = r.augmentWithHistory(input)
	}

	// Step 1: Retrieve relevant documents
	var documents *agent.Message
	var err error

	if r.queryExpander != "" {
		// Multi-query RAG: expand query into multiple variants
		documents, err = r.multiQueryRetrieve(ctx, queryInput)
	} else if r.keywordRetriever != "" {
		// Hybrid RAG: combine semantic and keyword retrieval
		documents, err = r.hybridRetrieve(ctx, queryInput)
	} else {
		// Standard retrieval
		retrieveStart := time.Now()
		retrieved, retrieveErr := r.runtime.Call(ctx, r.retriever, queryInput)
		retrieveDuration := time.Since(retrieveStart)

		if retrieveErr != nil {
			span.RecordError(retrieveErr)
			return nil, fmt.Errorf("retrieval failed: %w", retrieveErr)
		}

		span.SetAttributes(
			attribute.Int64("orchestration.retrieve_duration_ms", retrieveDuration.Milliseconds()),
		)

		// Step 2: Optional reranking
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
	}

	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	// Step 3: Generate answer with retrieved context
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

	// Store conversation turn if conversational
	if len(r.conversationHist) > 0 || r.historyAgent != "" {
		r.storeConversationTurn(input.Payload, result.Payload, documents.Payload)
	}

	return result, nil
}

// augmentWithHistory adds conversation history to the query
func (r *RAG) augmentWithHistory(query *agent.Message) *agent.Message {
	if len(r.conversationHist) == 0 {
		return query
	}

	// Build history context (keep last 5 turns)
	historyContext := "Conversation history:\n"
	start := 0
	if len(r.conversationHist) > 5 {
		start = len(r.conversationHist) - 5
	}

	for i := start; i < len(r.conversationHist); i++ {
		turn := r.conversationHist[i]
		historyContext += fmt.Sprintf("User: %s\nAssistant: %s\n\n", turn.Query, turn.Response)
	}

	// Augment query with history
	augmentedPayload := historyContext + "Current query: " + query.Payload

	return &agent.Message{
		Message: &pb.Message{
			Id:        query.Id,
			Type:      "conversational_query",
			Payload:   augmentedPayload,
			Timestamp: query.Timestamp,
			Metadata:  query.Metadata,
		},
	}
}

// storeConversationTurn saves a conversation turn
func (r *RAG) storeConversationTurn(query, response, context string) {
	turn := ConversationTurn{
		Query:    query,
		Response: response,
		Context:  context,
	}
	r.conversationHist = append(r.conversationHist, turn)

	// Keep only last 100 turns
	if len(r.conversationHist) > 100 {
		r.conversationHist = r.conversationHist[len(r.conversationHist)-100:]
	}
}

// multiQueryRetrieve expands query into multiple variants and retrieves for each
func (r *RAG) multiQueryRetrieve(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	// Expand query into multiple variants
	expandedQueries, err := r.runtime.Call(ctx, r.queryExpander, input)
	if err != nil {
		return nil, fmt.Errorf("query expansion failed: %w", err)
	}

	// Parse expanded queries (assume JSON array of strings)
	var queries []string
	if err := json.Unmarshal([]byte(expandedQueries.Payload), &queries); err != nil {
		// If not JSON, treat as single query
		queries = []string{expandedQueries.Payload}
	}

	// Retrieve for each query variant
	allDocs := make([]string, 0)
	docScores := make(map[string]float64)

	for i, q := range queries {
		queryMsg := &agent.Message{
			Message: &pb.Message{
				Payload: q,
			},
		}

		docs, err := r.runtime.Call(ctx, r.retriever, queryMsg)
		if err != nil {
			continue
		}

		// Apply reciprocal rank fusion scoring
		// Score = sum(1 / (rank + 60)) for each query
		docList := strings.Split(docs.Payload, "\n---\n")
		for rank, doc := range docList {
			score := 1.0 / float64(rank+60)
			docScores[doc] += score
			if _, exists := docScores[doc]; !exists {
				allDocs = append(allDocs, doc)
			}
		}
		_ = i // Use i to avoid unused variable warning
	}

	// Sort documents by score
	type scoredDoc struct {
		doc   string
		score float64
	}
	scored := make([]scoredDoc, 0, len(allDocs))
	for _, doc := range allDocs {
		scored = append(scored, scoredDoc{doc, docScores[doc]})
	}

	// Simple bubble sort by score (descending)
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Combine top documents
	topK := r.topK
	if topK == 0 || topK > len(scored) {
		topK = len(scored)
	}

	mergedDocs := make([]string, 0, topK)
	for i := 0; i < topK && i < len(scored); i++ {
		mergedDocs = append(mergedDocs, scored[i].doc)
	}

	return &agent.Message{
		Message: &pb.Message{
			Payload: strings.Join(mergedDocs, "\n---\n"),
		},
	}, nil
}

// hybridRetrieve combines semantic and keyword retrieval
func (r *RAG) hybridRetrieve(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	// Retrieve from both semantic and keyword retrievers in parallel
	type result struct {
		docs *agent.Message
		err  error
	}

	semanticCh := make(chan result, 1)
	keywordCh := make(chan result, 1)

	// Semantic retrieval
	go func() {
		docs, err := r.runtime.Call(ctx, r.retriever, input)
		semanticCh <- result{docs, err}
	}()

	// Keyword retrieval
	go func() {
		docs, err := r.runtime.Call(ctx, r.keywordRetriever, input)
		keywordCh <- result{docs, err}
	}()

	// Collect results
	semanticResult := <-semanticCh
	keywordResult := <-keywordCh

	if semanticResult.err != nil && keywordResult.err != nil {
		return nil, fmt.Errorf("both retrievals failed: semantic=%v, keyword=%v",
			semanticResult.err, keywordResult.err)
	}

	// Merge results with reciprocal rank fusion
	allDocs := make(map[string]float64)

	if semanticResult.err == nil && semanticResult.docs != nil {
		docs := strings.Split(semanticResult.docs.Payload, "\n---\n")
		for rank, doc := range docs {
			allDocs[doc] = 1.0 / float64(rank+60)
		}
	}

	if keywordResult.err == nil && keywordResult.docs != nil {
		docs := strings.Split(keywordResult.docs.Payload, "\n---\n")
		for rank, doc := range docs {
			score := 1.0 / float64(rank+60)
			allDocs[doc] += score
		}
	}

	// Sort by combined score
	type scoredDoc struct {
		doc   string
		score float64
	}
	scored := make([]scoredDoc, 0, len(allDocs))
	for doc, score := range allDocs {
		scored = append(scored, scoredDoc{doc, score})
	}

	// Sort descending by score
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Take top-K
	topK := r.topK
	if topK == 0 || topK > len(scored) {
		topK = len(scored)
	}

	mergedDocs := make([]string, 0, topK)
	for i := 0; i < topK && i < len(scored); i++ {
		mergedDocs = append(mergedDocs, scored[i].doc)
	}

	return &agent.Message{
		Message: &pb.Message{
			Payload: strings.Join(mergedDocs, "\n---\n"),
		},
	}, nil
}

// augmentInput combines the original query with retrieved documents
func augmentInput(query, documents *agent.Message) *agent.Message {
	if query == nil || query.Message == nil {
		return query
	}

	if documents == nil || documents.Message == nil || documents.Payload == "" {
		// No documents retrieved, return original query
		return query
	}

	// Create augmented message with both query and retrieved context
	// Format: "Context:\n{documents}\n\nQuery:\n{query}"
	augmentedPayload := fmt.Sprintf("Context:\n%s\n\nQuery:\n%s", documents.Payload, query.Payload)

	// Preserve metadata from both messages
	metadata := make(map[string]any)
	if query.Metadata != nil {
		maps.Copy(metadata, query.Metadata)
	}
	if documents.Metadata != nil {
		metadata["retrieved_context"] = documents.Metadata
	}

	return &agent.Message{
		Message: &pb.Message{
			Id:        query.Id,
			Type:      "rag_augmented",
			Payload:   augmentedPayload,
			Timestamp: query.Timestamp,
			Metadata:  metadata,
		},
	}
}

// RAG variants

// NewConversationalRAG creates a RAG with conversation history tracking
func NewConversationalRAG(name string, runtime agent.Runtime, retriever, generator string, historyAgent string) *RAG {
	r := NewRAG(name, runtime, retriever, generator)
	r.conversationHist = make([]ConversationTurn, 0, 100)
	r.historyAgent = historyAgent
	return r
}

// NewMultiQueryRAG creates a RAG that generates multiple queries for retrieval
func NewMultiQueryRAG(name string, runtime agent.Runtime, queryExpander, retriever, generator string) *RAG {
	r := NewRAG(name, runtime, retriever, generator)
	r.queryExpander = queryExpander
	return r
}

// NewHybridRAG creates a RAG with both semantic and keyword search
func NewHybridRAG(name string, runtime agent.Runtime, semanticRetriever, keywordRetriever, generator string) *RAG {
	r := NewRAG(name, runtime, semanticRetriever, generator)
	r.keywordRetriever = keywordRetriever
	return r
}
