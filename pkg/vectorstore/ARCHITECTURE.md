# VectorStore Architecture

This document describes the architecture of the Collection-based VectorStore interface for aixgo.

## Overview

The VectorStore interface provides a clean, type-safe abstraction for vector database operations with native support for 10 key use cases:

1. **Semantic Caching** - TTL-based document expiration
2. **Agent Memory** - Scoped, user/session-specific storage
3. **TTL/Expiration** - Automatic cleanup of expired documents
4. **Conversation History** - Temporal ordering and retrieval
5. **Deduplication** - Content-based duplicate detection
6. **User Profiles** - Multi-tenant scope isolation
7. **Recommendations** - Similarity search with metadata filters
8. **Anomaly Detection** - Score-based filtering
9. **Knowledge Graphs** - Structured metadata relationships
10. **Multi-modal Search** - Text, image, audio, video embeddings

## Design Principles

### 1. Simplicity Over Cleverness
- Clean, idiomatic Go interfaces
- Zero-config defaults with opt-in complexity
- Explicit over implicit behavior

### 2. Composition Over Inheritance
- Collection abstraction for isolation
- Composable filters using And/Or/Not
- Functional options pattern

### 3. Type Safety
- Typed fields (Scope, Temporal, Tags) not just metadata maps
- Strongly-typed filters instead of interface{} everywhere
- Compile-time safety where possible

### 4. Performance-Oriented
- Batch operations for bulk inserts
- Streaming iterators for large result sets
- Zero-copy where possible
- No unnecessary allocations

### 5. Security-Conscious
- Input validation at all boundaries
- Injection-proof metadata keys
- Path traversal prevention
- NaN/Inf detection in embeddings

## Architecture Layers

```
┌─────────────────────────────────────────────────┐
│           Application Code                      │
└─────────────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────┐
│         VectorStore Interface                   │
│  - Collection factory                           │
│  - Store-level operations                       │
└─────────────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────┐
│          Collection Interface                   │
│  - CRUD operations                              │
│  - Query & filtering                            │
│  - Batch & streaming                            │
└─────────────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────┐
│         Provider Implementation                 │
│  - Memory (in-memory, testing)                  │
│  - Firestore (Google Cloud)                     │
│  - Qdrant (dedicated vector DB)                 │
│  - Custom providers                             │
└─────────────────────────────────────────────────┘
```

## Core Types

### Document

Enhanced from simple string content to multi-modal with typed metadata:

```go
type Document struct {
    ID        string        // Unique identifier
    Content   *Content      // Multi-modal (text/image/audio/video)
    Embedding *Embedding    // Vector + metadata
    Scope     *Scope        // Multi-tenant isolation
    Temporal  *Temporal     // Time-based info
    Tags      []string      // Indexed labels
    Metadata  map[string]any // Free-form data
}
```

**Design rationale:**
- Typed fields enable provider optimizations (indexed scope, temporal queries)
- Multi-modal Content supports diverse AI models
- Embedding metadata tracks provenance
- Separation of indexed (Tags) vs free-form (Metadata) data

### Content

Multi-modal content representation:

```go
type Content struct {
    Type     ContentType  // text/image/audio/video
    Text     string       // For text
    Data     []byte       // For binary (base64 in JSON)
    MimeType string       // MIME type
    URL      string       // External reference
    Chunks   []string     // Text chunks
}
```

**Design rationale:**
- Single type handles all modalities
- URL support avoids storing large media inline
- Chunks enable document splitting
- MimeType allows format-specific handling

### Query

Expressive query builder with composable filters:

```go
type Query struct {
    Embedding         *Embedding  // Vector similarity
    Filters           Filter      // Composable conditions
    Limit/Offset      int         // Pagination
    MinScore          float32     // Threshold
    Metric            DistanceMetric
    IncludeEmbeddings bool        // Control payload size
    IncludeContent    bool
    SortBy            []SortBy    // Hybrid ranking
    Explain           bool        // Debug info
}
```

**Design rationale:**
- Optional embedding for pure metadata queries
- Composable filters using And/Or/Not
- Hybrid ranking (similarity + metadata)
- Explain mode for optimization

### Filter

Type-safe composable filters:

```go
// Composite
And(filters...)
Or(filters...)
Not(filter)

// Field-based
Eq(field, value)
Gt/Gte/Lt/Lte(field, value)
In/NotIn(field, values...)
Contains/StartsWith/EndsWith(field, substring)

// Tag-based
TagFilter(tag)
TagsFilter(tags...)  // All tags
AnyTagFilter(tags...) // Any tag

// Scope-based
ScopeFilter(scope)
TenantFilter(tenant)
UserFilter(user)
SessionFilter(session)

// Time-based
CreatedAfter/Before(time)
UpdatedAfter/Before(time)
ExpiresAfter/Before(time)
NotExpired()

// Score-based
ScoreAbove/Below/AtLeast(threshold)
```

**Design rationale:**
- Type-safe builders prevent runtime errors
- Composability enables complex queries
- Domain-specific helpers (TagFilter, ScopeFilter)
- Internal interface prevents external implementations

### Results

Rich result types with metadata:

```go
type QueryResult struct {
    Matches   []*Match       // Results
    Total     int64          // For pagination
    Timing    *QueryTiming   // Performance metrics
    Explain   *QueryExplain  // Execution plan
}

type Match struct {
    Document *Document
    Score    float32        // Similarity
    Distance float32        // Raw metric
    Rank     int           // Position
}
```

**Design rationale:**
- Separate Match type for query-specific data (score, rank)
- Timing info for performance monitoring
- Explain mode for debugging slow queries
- Pagination helpers (HasMore, NextOffset)

## Collection Abstraction

Collections provide isolated namespaces with use-case specific configuration:

```go
// Semantic cache
cache := store.Collection("cache",
    WithTTL(5*time.Minute),
    WithDeduplication(true),
    WithMaxDocuments(10000),
)

// Agent memory
memory := store.Collection("agent-memory",
    WithScope("user", "session"),
    WithMaxDocuments(1000),
)

// Document store
docs := store.Collection("documents",
    WithIndexing(IndexTypeHNSW),
    WithDimensions(768),
)
```

**Design rationale:**
- Isolation: Each use case gets its own configuration
- Lazy creation: Collections created on first use
- Reusable: Same name returns same collection
- Flexible: Functional options for customization

## Functional Options

All configuration uses functional options (no config structs):

```go
// Collection options
WithTTL(duration)
WithDeduplication(enabled)
WithDeduplicationThreshold(threshold)
WithIndexing(indexType)
WithDimensions(dims)
WithScope(fields...)
WithMaxDocuments(max)
WithVersioning(enabled)

// Batch options
WithBatchSize(size)
WithParallelism(n)
WithProgressCallback(func(processed, total int))
WithRetry(enabled)
WithMaxRetries(max)
```

**Design rationale:**
- Zero-config defaults for simple cases
- Discoverability through IDE autocomplete
- Extensibility without breaking changes
- Type-safe configuration

## Streaming & Iteration

Iterator pattern for large result sets:

```go
iter, err := coll.QueryStream(ctx, query)
defer iter.Close()

for iter.Next() {
    match := iter.Match()
    // Process match
}

if err := iter.Err(); err != nil {
    // Handle error
}
```

**Design rationale:**
- Memory-efficient for large results
- Standard Go iterator pattern (similar to sql.Rows)
- Composable with Filter/Map helpers
- Provider can stream from database cursor

## Batch Operations

Optimized bulk operations with progress tracking:

```go
result, err := coll.UpsertBatch(ctx, documents,
    WithBatchSize(100),
    WithParallelism(4),
    WithProgressCallback(func(processed, total int) {
        log.Printf("Progress: %d/%d", processed, total)
    }),
)
```

**Design rationale:**
- Better performance than individual upserts
- Progress callbacks for UX
- Configurable parallelism
- Partial success support

## Provider Implementation Guide

### Minimal Provider

```go
type MyProvider struct {
    // Provider-specific state
}

func (p *MyProvider) Collection(name string, opts ...CollectionOption) Collection {
    config := ApplyOptions(opts)
    return &myCollection{
        name:     name,
        config:   config,
        provider: p,
    }
}

type myCollection struct {
    name     string
    config   *CollectionConfig
    provider *MyProvider
}

func (c *myCollection) Upsert(ctx context.Context, docs ...*Document) (*UpsertResult, error) {
    // Validate documents
    for _, doc := range docs {
        if err := Validate(doc); err != nil {
            return nil, err
        }
    }

    // Apply collection config (TTL, deduplication, etc.)
    // Store documents
    // Return result
}

func (c *myCollection) Query(ctx context.Context, query *Query) (*QueryResult, error) {
    // Validate query
    // Execute vector search
    // Apply filters
    // Return results
}
```

### Filter Processing

Providers should decompose filters recursively:

```go
func applyFilter(doc *Document, filter Filter) bool {
    if IsAndFilter(filter) {
        for _, f := range GetFilters(filter) {
            if !applyFilter(doc, f) {
                return false
            }
        }
        return true
    }

    if IsOrFilter(filter) {
        for _, f := range GetFilters(filter) {
            if applyFilter(doc, f) {
                return true
            }
        }
        return false
    }

    if field, op, value, ok := GetFieldFilter(filter); ok {
        return applyFieldFilter(doc, field, op, value)
    }

    // Handle other filter types...
}
```

## Use Case Implementations

### 1. Semantic Caching

```go
cache := store.Collection("cache",
    WithTTL(5*time.Minute),
    WithDeduplication(true),
    WithMaxDocuments(10000),
)

// Cache query result
doc := &Document{
    ID:        queryHash,
    Content:   NewTextContent(query),
    Embedding: queryEmbedding,
    Temporal:  NewTemporalWithTTL(5*time.Minute),
    Metadata:  map[string]any{"result": cachedResult},
}
cache.Upsert(ctx, doc)

// Lookup with high threshold
query := &Query{
    Embedding: queryEmbedding,
    MinScore:  0.95, // Cache hit threshold
    Limit:     1,
}
result, _ := cache.Query(ctx, query)
```

### 2. Agent Memory

```go
memory := store.Collection("agent-memory",
    WithScope("user", "session"),
)

// Store memory with scope
doc := &Document{
    ID:        memoryID,
    Content:   NewTextContent(memoryText),
    Embedding: memoryEmbedding,
    Scope:     NewScope("tenant1", "user123", "session456"),
    Tags:      []string{"preference", "context"},
}
memory.Upsert(ctx, doc)

// Retrieve scoped memories
query := &Query{
    Embedding: queryEmbedding,
    Filters:   UserFilter("user123"),
    Limit:     5,
}
```

### 3. Conversation History

```go
conversations := store.Collection("conversations",
    WithScope("user", "thread"),
)

// Store turn with temporal info
doc := &Document{
    ID:      turnID,
    Content: NewTextContent(turnText),
    Scope:   &Scope{User: userID, Thread: threadID},
    Temporal: &Temporal{
        CreatedAt: time.Now(),
        EventTime: &eventTime,
    },
    Metadata: map[string]any{"turn_number": turnNum},
}

// Retrieve chronologically
query := &Query{
    Filters: And(
        UserFilter(userID),
        Eq("thread", threadID),
    ),
    SortBy: []SortBy{SortByCreatedAt(false)}, // Ascending
    Limit:  20,
}
```

### 4. Multi-modal Search

```go
media := store.Collection("media",
    WithDimensions(512), // CLIP dimensions
)

// Store image
doc := &Document{
    ID:        imageID,
    Content:   NewImageURL(imageURL),
    Embedding: clipEmbedding,
    Tags:      []string{"photo", "product"},
}

// Query with image embedding
query := &Query{
    Embedding: queryImageEmbedding,
    Filters:   TagFilter("product"),
}
```

## Migration from Old Interface

The old interface can be mapped to the new one:

```go
// Old
store.Upsert(ctx, documents)

// New
coll := store.Collection("default")
coll.Upsert(ctx, documents...)

// Old
results, err := store.Search(ctx, SearchQuery{
    Embedding: vec,
    TopK:      10,
    Filter:    &MetadataFilter{Must: map[string]any{"key": "value"}},
})

// New
results, err := coll.Query(ctx, &Query{
    Embedding: NewEmbedding(vec, "model"),
    Limit:     10,
    Filters:   Eq("key", "value"),
})
```

## Performance Considerations

### Indexing Strategy

- `IndexTypeFlat`: <10K documents, need exact results
- `IndexTypeHNSW`: 10K-10M documents, best general-purpose
- `IndexTypeIVF`: >10M documents, can trade accuracy for speed

### Batch Operations

Always use `UpsertBatch` for >10 documents:

```go
// Bad: Individual upserts
for _, doc := range documents {
    coll.Upsert(ctx, doc)
}

// Good: Batch upsert
coll.UpsertBatch(ctx, documents, WithBatchSize(100))
```

### Streaming

Use `QueryStream` for large result sets:

```go
// Bad: Load all into memory
result, _ := coll.Query(ctx, &Query{Limit: 100000})

// Good: Stream results
iter, _ := coll.QueryStream(ctx, query)
defer iter.Close()
for iter.Next() {
    processMatch(iter.Match())
}
```

### Include Flags

Don't include embeddings unless needed:

```go
query := &Query{
    Embedding:         queryEmbedding,
    IncludeEmbeddings: false, // Don't return large vectors
    IncludeContent:    true,  // Do return content
}
```

## Testing

The design enables easy testing:

```go
// Use memory provider for tests
store, _ := memory.New()
coll := store.Collection("test")

// Test with real operations
doc := &Document{...}
coll.Upsert(ctx, doc)
result, _ := coll.Query(ctx, query)
assert.Equal(t, 1, len(result.Matches))
```

## Future Extensions

The architecture supports future enhancements:

1. **Hybrid Search**: Add BM25/keyword search alongside vector search
2. **Reranking**: Add reranker support in Query
3. **Quantization**: Add quantized embedding support
4. **Sharding**: Add shard key to Scope
5. **Backup/Restore**: Add collection-level backup
6. **Schema Validation**: Add JSON schema for metadata
7. **Triggers**: Add insert/update/delete hooks
8. **Computed Fields**: Add derived metadata fields

All can be added without breaking existing code.
