package vectorstore_test

import (
	"context"
	"fmt"
	"time"

	"github.com/aixgo-dev/aixgo/pkg/vectorstore"
)

// Example demonstrates basic usage of the VectorStore interface.
func Example_basic() {
	// NOTE: This is a documentation example showing the API.
	// It won't run without a real store implementation.

	// Create a vector store (implementation-specific)
	var store vectorstore.VectorStore // = memory.New() or firestore.New() etc.
	defer func() { _ = store.Close() }()

	// Create a collection for documents
	docs := store.Collection("documents")

	// Create a document
	doc := &vectorstore.Document{
		ID:      "doc1",
		Content: vectorstore.NewTextContent("The quick brown fox jumps over the lazy dog"),
		Embedding: vectorstore.NewEmbedding(
			[]float32{0.1, 0.2, 0.3, 0.4, 0.5},
			"text-embedding-3-small",
		),
		Tags: []string{"example", "demo"},
	}

	// Insert the document
	ctx := context.Background()
	result, err := docs.Upsert(ctx, doc)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Inserted: %d documents\n", result.Inserted)

	// Query for similar documents
	query := &vectorstore.Query{
		Embedding: vectorstore.NewEmbedding(
			[]float32{0.1, 0.2, 0.3, 0.4, 0.5},
			"text-embedding-3-small",
		),
		Limit: 10,
	}

	queryResult, _ := docs.Query(ctx, query)
	fmt.Printf("Found: %d matches\n", queryResult.Count())
}

// Example_semanticCache demonstrates using collections for semantic caching.
func Example_semanticCache() {
	var store vectorstore.VectorStore
	defer func() { _ = store.Close() }()

	// Create a cache collection with TTL and deduplication
	cache := store.Collection("cache",
		vectorstore.WithTTL(5*time.Minute),
		vectorstore.WithDeduplication(true),
		vectorstore.WithMaxDocuments(10000),
	)

	ctx := context.Background()

	// Cache a query result
	cacheDoc := &vectorstore.Document{
		ID:      "query-hash-123",
		Content: vectorstore.NewTextContent("What is the capital of France?"),
		Embedding: vectorstore.NewEmbedding(
			[]float32{0.1, 0.2, 0.3},
			"text-embedding-3-small",
		),
		Temporal: vectorstore.NewTemporalWithTTL(5 * time.Minute),
		Tags:     []string{"qa", "geography"},
		Metadata: map[string]any{
			"answer": "Paris",
			"cached": time.Now(),
		},
	}

	_, err := cache.Upsert(ctx, cacheDoc)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Lookup cached result by similarity
	query := &vectorstore.Query{
		Embedding: vectorstore.NewEmbedding(
			[]float32{0.11, 0.19, 0.31},
			"text-embedding-3-small",
		),
		Limit:    1,
		MinScore: 0.95, // High threshold for cache hits
	}

	result, _ := cache.Query(ctx, query)
	if result.HasMatches() {
		answer := result.TopMatch().Document.Metadata["answer"]
		fmt.Printf("Cache hit! Answer: %s\n", answer)
	}
}

// Example_agentMemory demonstrates using collections for agent memory.
func Example_agentMemory() {
	var store vectorstore.VectorStore
	defer func() { _ = store.Close() }()

	// Create memory collection with scope requirements
	memory := store.Collection("agent-memory",
		vectorstore.WithScope("user", "session"),
		vectorstore.WithMaxDocuments(1000),
	)

	ctx := context.Background()

	// Store a memory
	memoryDoc := &vectorstore.Document{
		ID:      "memory-1",
		Content: vectorstore.NewTextContent("User prefers dark mode"),
		Embedding: vectorstore.NewEmbedding(
			[]float32{0.5, 0.6, 0.7},
			"text-embedding-3-small",
		),
		Scope: vectorstore.NewScope("tenant1", "user123", "session456"),
		Tags:  []string{"preference", "ui"},
		Temporal: &vectorstore.Temporal{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	_, err := memory.Upsert(ctx, memoryDoc)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Retrieve memories for a specific user
	query := &vectorstore.Query{
		Embedding: vectorstore.NewEmbedding(
			[]float32{0.5, 0.6, 0.7},
			"text-embedding-3-small",
		),
		Filters: vectorstore.And(
			vectorstore.UserFilter("user123"),
			vectorstore.TagFilter("preference"),
		),
		Limit: 5,
	}

	result, _ := memory.Query(ctx, query)
	for _, match := range result.Matches {
		fmt.Printf("Memory: %s (score: %.2f)\n",
			match.Document.Content.String(),
			match.Score,
		)
	}
}

// Example_conversationHistory demonstrates storing conversation history.
func Example_conversationHistory() {
	var store vectorstore.VectorStore
	defer func() { _ = store.Close() }()

	// Create conversation collection
	conversations := store.Collection("conversations",
		vectorstore.WithScope("user", "thread"),
		vectorstore.WithMaxDocuments(100000),
	)

	ctx := context.Background()

	// Store a conversation turn
	turn := &vectorstore.Document{
		ID:      "turn-1",
		Content: vectorstore.NewTextContent("User: What's the weather?\nAssistant: It's sunny today."),
		Embedding: vectorstore.NewEmbedding(
			[]float32{0.2, 0.3, 0.4},
			"text-embedding-3-small",
		),
		Scope: &vectorstore.Scope{
			User:   "user123",
			Thread: "thread-abc",
		},
		Temporal: &vectorstore.Temporal{
			CreatedAt: time.Now(),
			EventTime: &[]time.Time{time.Now()}[0],
		},
		Tags: []string{"weather", "conversation"},
		Metadata: map[string]any{
			"turn_number": 1,
			"user_id":     "user123",
		},
	}

	_, err := conversations.Upsert(ctx, turn)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Retrieve recent conversation history
	query := &vectorstore.Query{
		Filters: vectorstore.And(
			vectorstore.UserFilter("user123"),
			vectorstore.Eq("thread", "thread-abc"),
			vectorstore.CreatedAfter(time.Now().Add(-24*time.Hour)),
		),
		SortBy: []vectorstore.SortBy{
			vectorstore.SortByCreatedAt(false), // Ascending (chronological)
		},
		Limit: 20,
	}

	result, _ := conversations.Query(ctx, query)
	fmt.Printf("Found %d conversation turns\n", result.Count())
}

// Example_deduplication demonstrates content deduplication.
func Example_deduplication() {
	var store vectorstore.VectorStore
	defer func() { _ = store.Close() }()

	// Create collection with aggressive deduplication
	docs := store.Collection("documents",
		vectorstore.WithDeduplicationThreshold(0.95),
	)

	ctx := context.Background()

	// Insert multiple similar documents
	documents := []*vectorstore.Document{
		{
			ID:        "doc1",
			Content:   vectorstore.NewTextContent("The quick brown fox"),
			Embedding: vectorstore.NewEmbedding([]float32{0.1, 0.2, 0.3}, "model"),
		},
		{
			ID:        "doc2",
			Content:   vectorstore.NewTextContent("The quick brown fox"), // Duplicate
			Embedding: vectorstore.NewEmbedding([]float32{0.1, 0.2, 0.3}, "model"),
		},
		{
			ID:        "doc3",
			Content:   vectorstore.NewTextContent("A different document"),
			Embedding: vectorstore.NewEmbedding([]float32{0.9, 0.8, 0.7}, "model"),
		},
	}

	result, err := docs.UpsertBatch(ctx, documents)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Inserted: %d, Deduplicated: %d\n",
		result.Inserted,
		result.Deduplicated,
	)
}

// Example_multimodal demonstrates multi-modal content.
func Example_multimodal() {
	var store vectorstore.VectorStore
	defer func() { _ = store.Close() }()

	media := store.Collection("media",
		vectorstore.WithDimensions(512),
	)

	ctx := context.Background()

	// Store an image
	imageDoc := &vectorstore.Document{
		ID: "img1",
		Content: vectorstore.NewImageURL(
			"https://example.com/photo.jpg",
		),
		Embedding: vectorstore.NewEmbedding(
			make([]float32, 512), // CLIP embedding
			"clip-vit-base-patch32",
		),
		Tags: []string{"photo", "landscape"},
	}

	_, err := media.Upsert(ctx, imageDoc)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Query with image embedding
	query := &vectorstore.Query{
		Embedding: vectorstore.NewEmbedding(
			make([]float32, 512),
			"clip-vit-base-patch32",
		),
		Filters: vectorstore.TagFilter("photo"),
		Limit:   10,
	}

	result, _ := media.Query(ctx, query)
	fmt.Printf("Found %d similar images\n", result.Count())
}

// Example_streaming demonstrates streaming query results.
func Example_streaming() {
	var store vectorstore.VectorStore
	defer func() { _ = store.Close() }()

	docs := store.Collection("documents")
	ctx := context.Background()

	query := &vectorstore.Query{
		Embedding: vectorstore.NewEmbedding(
			[]float32{0.1, 0.2, 0.3},
			"model",
		),
		Limit: 1000, // Large result set
	}

	// Stream results
	iter, _ := docs.QueryStream(ctx, query)
	defer func() { _ = iter.Close() }()

	count := 0
	for iter.Next() {
		match := iter.Match()
		if match.Score >= 0.8 {
			count++
			fmt.Printf("High score match: %s\n", match.Document.ID)
		}
	}

	if err := iter.Err(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	fmt.Printf("Found %d high-score matches\n", count)
}

// Example_batchOperations demonstrates batch upsert with progress tracking.
func Example_batchOperations() {
	var store vectorstore.VectorStore
	defer func() { _ = store.Close() }()

	docs := store.Collection("documents")
	ctx := context.Background()

	// Create many documents
	documents := make([]*vectorstore.Document, 1000)
	for i := range documents {
		documents[i] = &vectorstore.Document{
			ID:      fmt.Sprintf("doc-%d", i),
			Content: vectorstore.NewTextContent(fmt.Sprintf("Document %d", i)),
			Embedding: vectorstore.NewEmbedding(
				[]float32{float32(i) / 1000.0, 0.5, 0.5},
				"model",
			),
		}
	}

	// Batch insert with progress tracking
	result, err := docs.UpsertBatch(ctx, documents,
		vectorstore.WithBatchSize(100),
		vectorstore.WithParallelism(4),
		vectorstore.WithProgressCallback(func(processed, total int) {
			pct := float64(processed) / float64(total) * 100
			fmt.Printf("Progress: %d/%d (%.1f%%)\n", processed, total, pct)
		}),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Inserted: %d, Failed: %d\n", result.Inserted, result.Failed)
}

// Example_complexFilters demonstrates complex filter queries.
func Example_complexFilters() {
	var store vectorstore.VectorStore
	defer func() { _ = store.Close() }()

	docs := store.Collection("products")
	ctx := context.Background()

	// Complex filter: recent, high-rated electronics in stock
	query := &vectorstore.Query{
		Embedding: vectorstore.NewEmbedding(
			[]float32{0.1, 0.2, 0.3},
			"model",
		),
		Filters: vectorstore.And(
			vectorstore.TagFilter("electronics"),
			vectorstore.Gte("rating", 4.5),
			vectorstore.Eq("in_stock", true),
			vectorstore.CreatedAfter(time.Now().Add(-30*24*time.Hour)),
			vectorstore.Or(
				vectorstore.Contains("category", "phone"),
				vectorstore.Contains("category", "laptop"),
			),
		),
		SortBy: []vectorstore.SortBy{
			vectorstore.SortByScore(),
			vectorstore.SortByField("rating", true),
		},
		Limit: 20,
	}

	result, _ := docs.Query(ctx, query)
	fmt.Printf("Found %d matching products\n", result.Count())
}

// Example_timeBasedQueries demonstrates temporal queries.
func Example_timeBasedQueries() {
	var store vectorstore.VectorStore
	defer func() { _ = store.Close() }()

	events := store.Collection("events")
	ctx := context.Background()

	// Query for events in the last week that haven't expired
	query := &vectorstore.Query{
		Filters: vectorstore.And(
			vectorstore.CreatedAfter(time.Now().Add(-7*24*time.Hour)),
			vectorstore.NotExpired(),
			vectorstore.TagsFilter("important", "scheduled"),
		),
		SortBy: []vectorstore.SortBy{
			vectorstore.SortByCreatedAt(true), // Most recent first
		},
		Limit: 50,
	}

	result, _ := events.Query(ctx, query)
	for _, match := range result.Matches {
		fmt.Printf("Event: %s at %s\n",
			match.Document.Content.String(),
			match.Document.Temporal.CreatedAt,
		)
	}
}

// Example_pagination demonstrates paginated queries.
func Example_pagination() {
	var store vectorstore.VectorStore
	defer func() { _ = store.Close() }()

	docs := store.Collection("documents")
	ctx := context.Background()

	pageSize := 20
	page := 0

	for {
		query := &vectorstore.Query{
			Embedding: vectorstore.NewEmbedding(
				[]float32{0.1, 0.2, 0.3},
				"model",
			),
			Limit:  pageSize,
			Offset: page * pageSize,
		}

		result, _ := docs.Query(ctx, query)

		fmt.Printf("Page %d: %d results\n", page+1, result.Count())

		// Process results...
		for _, match := range result.Matches {
			_ = match // Process match
		}

		// Check if there are more pages
		if !result.HasMore() {
			break
		}

		page++
	}
}
