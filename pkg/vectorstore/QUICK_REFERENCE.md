# VectorStore Quick Reference

## Table of Contents
- [Basic Usage](#basic-usage)
- [Creating Collections](#creating-collections)
- [Documents](#documents)
- [Queries](#queries)
- [Filters](#filters)
- [Results](#results)
- [Common Patterns](#common-patterns)

## Basic Usage

```go
import "github.com/aixgo-dev/aixgo/pkg/vectorstore"

// Create a store (provider-specific)
store, err := memory.New()
defer store.Close()

// Get a collection
coll := store.Collection("my-collection")

// Create a document
doc := &vectorstore.Document{
    ID:      "doc1",
    Content: vectorstore.NewTextContent("Hello world"),
    Embedding: vectorstore.NewEmbedding(
        []float32{0.1, 0.2, 0.3},
        "text-embedding-3-small",
    ),
    Tags: []string{"greeting"},
}

// Insert
result, err := coll.Upsert(ctx, doc)

// Query
query := &vectorstore.Query{
    Embedding: vectorstore.NewEmbedding(
        []float32{0.1, 0.2, 0.3},
        "text-embedding-3-small",
    ),
    Limit: 10,
}
results, err := coll.Query(ctx, query)
```

## Creating Collections

### Basic Collection
```go
coll := store.Collection("documents")
```

### With TTL (Semantic Caching)
```go
cache := store.Collection("cache",
    vectorstore.WithTTL(5*time.Minute),
)
```

### With Deduplication
```go
docs := store.Collection("docs",
    vectorstore.WithDeduplication(true),
    vectorstore.WithDeduplicationThreshold(0.95),
)
```

### With Scope Requirements (Multi-tenancy)
```go
memory := store.Collection("agent-memory",
    vectorstore.WithScope("user", "session"),
)
```

### With HNSW Indexing
```go
large := store.Collection("large-dataset",
    vectorstore.WithIndexing(vectorstore.IndexTypeHNSW),
    vectorstore.WithDimensions(768),
)
```

### Full Configuration
```go
coll := store.Collection("production",
    vectorstore.WithTTL(24*time.Hour),
    vectorstore.WithDeduplication(true),
    vectorstore.WithIndexing(vectorstore.IndexTypeHNSW),
    vectorstore.WithDimensions(768),
    vectorstore.WithScope("tenant", "user"),
    vectorstore.WithMaxDocuments(100000),
    vectorstore.WithVersioning(true),
)
```

## Documents

### Text Document
```go
doc := &vectorstore.Document{
    ID:      "doc1",
    Content: vectorstore.NewTextContent("The quick brown fox"),
    Embedding: vectorstore.NewEmbedding(
        embeddings,
        "text-embedding-3-small",
    ),
}
```

### With Tags
```go
doc := &vectorstore.Document{
    ID:      "doc1",
    Content: vectorstore.NewTextContent("Product description"),
    Embedding: embedding,
    Tags:    []string{"product", "electronics", "featured"},
}
```

### With Scope (Multi-tenant)
```go
doc := &vectorstore.Document{
    ID:      "doc1",
    Content: content,
    Embedding: embedding,
    Scope: &vectorstore.Scope{
        Tenant:  "acme-corp",
        User:    "user123",
        Session: "session456",
    },
}

// Or use helper
doc.Scope = vectorstore.NewScope("acme-corp", "user123", "session456")
```

### With Temporal (TTL, Event Time)
```go
doc := &vectorstore.Document{
    ID:      "doc1",
    Content: content,
    Embedding: embedding,
    Temporal: vectorstore.NewTemporalWithTTL(5*time.Minute),
}

// Or manual
expiresAt := time.Now().Add(1 * time.Hour)
doc.Temporal = &vectorstore.Temporal{
    CreatedAt: time.Now(),
    UpdatedAt: time.Now(),
    ExpiresAt: &expiresAt,
}
```

### With Metadata
```go
doc := &vectorstore.Document{
    ID:      "doc1",
    Content: content,
    Embedding: embedding,
    Metadata: map[string]any{
        "author":   "John Doe",
        "category": "tutorial",
        "rating":   4.5,
        "published": true,
    },
}
```

### Image Document
```go
doc := &vectorstore.Document{
    ID: "img1",
    Content: vectorstore.NewImageURL("https://example.com/photo.jpg"),
    Embedding: vectorstore.NewEmbedding(
        clipEmbedding,
        "clip-vit-base-patch32",
    ),
    Tags: []string{"photo", "landscape"},
}

// Or with data
doc := &vectorstore.Document{
    ID: "img1",
    Content: vectorstore.NewImageContent(imageBytes, "image/jpeg"),
    Embedding: embedding,
}
```

## Queries

### Basic Vector Search
```go
query := &vectorstore.Query{
    Embedding: vectorstore.NewEmbedding(queryVector, "model"),
    Limit:     10,
}
results, err := coll.Query(ctx, query)
```

### With Minimum Score
```go
query := &vectorstore.Query{
    Embedding: embedding,
    Limit:     10,
    MinScore:  0.8, // Only return matches with score >= 0.8
}
```

### With Filters
```go
query := &vectorstore.Query{
    Embedding: embedding,
    Limit:     10,
    Filters: vectorstore.And(
        vectorstore.Eq("category", "tutorial"),
        vectorstore.Gte("rating", 4.0),
    ),
}
```

### Filter-Only Query (No Vector Search)
```go
query := vectorstore.NewFilterQuery(
    vectorstore.And(
        vectorstore.TagFilter("featured"),
        vectorstore.Eq("status", "published"),
    ),
)
query.Limit = 20
results, err := coll.Query(ctx, query)
```

### With Pagination
```go
query := &vectorstore.Query{
    Embedding: embedding,
    Limit:     20,
    Offset:    40, // Page 3 (20 per page)
}
```

### With Sorting
```go
query := &vectorstore.Query{
    Embedding: embedding,
    Limit:     10,
    SortBy: []vectorstore.SortBy{
        vectorstore.SortByScore(),                 // Primary: similarity
        vectorstore.SortByField("rating", true),   // Secondary: rating desc
        vectorstore.SortByCreatedAt(true),         // Tertiary: newest first
    },
}
```

### With Explain
```go
query := &vectorstore.Query{
    Embedding: embedding,
    Limit:     10,
    Explain:   true, // Get execution details
}
results, err := coll.Query(ctx, query)
fmt.Println(results.Explain.ExplainString())
```

### Streaming Query
```go
iter, err := coll.QueryStream(ctx, query)
defer iter.Close()

for iter.Next() {
    match := iter.Match()
    fmt.Printf("Score: %.4f, Doc: %s\n", match.Score, match.Document.ID)
}

if err := iter.Err(); err != nil {
    log.Fatal(err)
}
```

## Filters

### Comparison Filters
```go
vectorstore.Eq("status", "active")
vectorstore.Ne("status", "deleted")
vectorstore.Gt("rating", 4.0)
vectorstore.Gte("rating", 4.0)
vectorstore.Lt("price", 100.0)
vectorstore.Lte("price", 100.0)
```

### Set Filters
```go
vectorstore.In("category", "electronics", "computers", "phones")
vectorstore.NotIn("status", "draft", "deleted")
```

### String Filters
```go
vectorstore.Contains("description", "wireless")
vectorstore.StartsWith("sku", "PROD-")
vectorstore.EndsWith("email", "@example.com")
```

### Existence Filters
```go
vectorstore.Exists("thumbnail")
vectorstore.NotExists("deleted_at")
```

### Tag Filters
```go
vectorstore.TagFilter("featured")                    // Has tag
vectorstore.TagsFilter("featured", "sale")          // Has all tags
vectorstore.AnyTagFilter("new", "trending", "hot")  // Has any tag
```

### Scope Filters
```go
vectorstore.TenantFilter("acme-corp")
vectorstore.UserFilter("user123")
vectorstore.SessionFilter("session456")

// Or composite
vectorstore.ScopeFilter(&vectorstore.Scope{
    Tenant: "acme-corp",
    User:   "user123",
})
```

### Time Filters
```go
vectorstore.CreatedAfter(time.Now().Add(-24*time.Hour))
vectorstore.CreatedBefore(time.Now())
vectorstore.UpdatedAfter(yesterday)
vectorstore.UpdatedBefore(today)

vectorstore.NotExpired()                              // Not expired yet
vectorstore.Expired()                                 // Already expired
vectorstore.ExpiresAfter(time.Now().Add(1*time.Hour)) // Expires later
```

### Score Filters
```go
vectorstore.ScoreAbove(0.8)
vectorstore.ScoreBelow(0.5)
vectorstore.ScoreAtLeast(0.9)
```

### Composite Filters
```go
// AND
vectorstore.And(
    vectorstore.Eq("status", "active"),
    vectorstore.Gte("rating", 4.0),
    vectorstore.TagFilter("featured"),
)

// OR
vectorstore.Or(
    vectorstore.Eq("category", "electronics"),
    vectorstore.Eq("category", "computers"),
)

// NOT
vectorstore.Not(
    vectorstore.Eq("status", "deleted"),
)

// Complex combination
vectorstore.And(
    vectorstore.Or(
        vectorstore.Eq("category", "electronics"),
        vectorstore.Eq("category", "computers"),
    ),
    vectorstore.Gte("rating", 4.0),
    vectorstore.Not(
        vectorstore.Eq("status", "deleted"),
    ),
)
```

## Results

### Access Matches
```go
results, err := coll.Query(ctx, query)

// Check if has results
if results.HasMatches() {
    // Get top match
    top := results.TopMatch()
    fmt.Printf("Best match: %s (score: %.4f)\n", top.Document.ID, top.Score)

    // Iterate all matches
    for _, match := range results.Matches {
        fmt.Printf("%s: %.4f\n", match.Document.ID, match.Score)
    }
}
```

### Pagination
```go
// Check if more results
if results.HasMore() {
    nextQuery := query
    nextQuery.Offset = results.NextOffset()
}

// Previous page
if results.Offset > 0 {
    prevQuery := query
    prevQuery.Offset = results.PrevOffset()
}
```

### Filter Results
```go
// Filter by score
highScore := results.FilterByScore(0.9)

// Filter by tag
featured := results.FilterByTag("featured")

// Group by tag
groups := results.GroupByTag()
for tag, matches := range groups {
    fmt.Printf("%s: %d matches\n", tag, len(matches))
}
```

### Statistics
```go
fmt.Printf("Total matches: %d\n", results.Total)
fmt.Printf("Returned: %d\n", results.Count())
fmt.Printf("Avg score: %.4f\n", results.AvgScore())
fmt.Printf("Max score: %.4f\n", results.MaxScore())
fmt.Printf("Min score: %.4f\n", results.MinScore())
```

### Performance Metrics
```go
if results.Timing != nil {
    fmt.Printf("Total time: %v\n", results.Timing.Total)
    fmt.Printf("Vector search: %v\n", results.Timing.VectorSearch)
    fmt.Printf("Filter application: %v\n", results.Timing.FilterApplication)
}
```

## Common Patterns

### Semantic Caching
```go
cache := store.Collection("cache",
    vectorstore.WithTTL(5*time.Minute),
    vectorstore.WithDeduplication(true),
)

// Store
doc := &vectorstore.Document{
    ID:        queryHash,
    Content:   vectorstore.NewTextContent(query),
    Embedding: queryEmbedding,
    Temporal:  vectorstore.NewTemporalWithTTL(5*time.Minute),
    Metadata:  map[string]any{"result": cachedResult},
}
cache.Upsert(ctx, doc)

// Lookup
query := &vectorstore.Query{
    Embedding: queryEmbedding,
    MinScore:  0.95, // High threshold
    Limit:     1,
}
results, _ := cache.Query(ctx, query)
if results.HasMatches() {
    // Cache hit
    result := results.TopMatch().Document.Metadata["result"]
}
```

### Conversation History
```go
conversations := store.Collection("conversations",
    vectorstore.WithScope("user", "thread"),
)

// Store turn
doc := &vectorstore.Document{
    ID:      turnID,
    Content: vectorstore.NewTextContent(turnText),
    Scope:   vectorstore.NewScope("", userID, threadID),
    Temporal: &vectorstore.Temporal{
        CreatedAt: time.Now(),
        EventTime: &eventTime,
    },
    Metadata: map[string]any{"turn_number": turnNum},
}
conversations.Upsert(ctx, doc)

// Retrieve chronologically
query := &vectorstore.Query{
    Filters: vectorstore.And(
        vectorstore.UserFilter(userID),
        vectorstore.Eq("thread_id", threadID),
    ),
    SortBy: []vectorstore.SortBy{
        vectorstore.SortByCreatedAt(false), // Ascending
    },
    Limit: 20,
}
```

### Batch Insert
```go
result, err := coll.UpsertBatch(ctx, documents,
    vectorstore.WithBatchSize(100),
    vectorstore.WithParallelism(4),
    vectorstore.WithProgressCallback(func(processed, total int) {
        pct := float64(processed) / float64(total) * 100
        log.Printf("Progress: %.1f%%", pct)
    }),
)

fmt.Printf("Inserted: %d, Updated: %d, Failed: %d\n",
    result.Inserted, result.Updated, result.Failed)
```

### Delete Expired Documents
```go
result, err := coll.DeleteByFilter(ctx,
    vectorstore.Expired(),
)
fmt.Printf("Deleted %d expired documents\n", result.Deleted)
```

### Multi-tenant Query
```go
query := &vectorstore.Query{
    Embedding: embedding,
    Filters: vectorstore.And(
        vectorstore.TenantFilter("acme-corp"),
        vectorstore.UserFilter("user123"),
        vectorstore.TagFilter("preference"),
    ),
    Limit: 10,
}
```

### Hybrid Search (Vector + Metadata)
```go
query := &vectorstore.Query{
    Embedding: embedding,
    Filters: vectorstore.And(
        vectorstore.Eq("status", "published"),
        vectorstore.Gte("rating", 4.0),
        vectorstore.TagsFilter("featured", "electronics"),
        vectorstore.CreatedAfter(thirtyDaysAgo),
    ),
    SortBy: []vectorstore.SortBy{
        vectorstore.SortByScore(),
        vectorstore.SortByField("rating", true),
    },
    Limit: 20,
}
```

### Streaming Large Results
```go
iter, err := coll.QueryStream(ctx, &vectorstore.Query{
    Embedding: embedding,
    Limit:     10000,
})
defer iter.Close()

count := 0
for iter.Next() {
    match := iter.Match()
    if processMatch(match) {
        count++
    }
}
fmt.Printf("Processed %d matches\n", count)
```
