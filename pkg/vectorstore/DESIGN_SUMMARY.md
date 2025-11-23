# VectorStore Collection-Based Interface - Design Summary

## Overview

This is a complete rewrite of the aixgo VectorStore interface with native support for 10 key use cases through a Collection-based abstraction.

**Status**: Core interfaces complete and compiling. Provider implementations (memory, firestore) need updates.

## Files Created/Updated

### Core Interface Files

1. **`vectorstore.go`** (1,275 lines)
   - `VectorStore` interface - Collection factory and store-level operations
   - `Collection` interface - CRUD, query, batch, streaming operations
   - `ResultIterator` interface - Streaming query results
   - `StoreStats`, `CollectionStats` - Statistics types

2. **`document.go`** (644 lines)
   - `Document` - Enhanced with typed fields (Scope, Temporal, Tags)
   - `Content` - Multi-modal support (text, image, audio, video)
   - `Embedding` - Vector with model metadata
   - `Scope` - Multi-tenant isolation fields
   - `Temporal` - Time-based metadata (TTL, validity windows)
   - Comprehensive validation functions

3. **`query.go`** (503 lines)
   - `Query` - Enhanced query with filters, pagination, sorting
   - `Filter` interface - Composable type-safe filters
   - Filter constructors: `And`, `Or`, `Not`, `Eq`, `Gt`, `Lt`, `In`, `Contains`, etc.
   - Domain-specific filters: `TagFilter`, `ScopeFilter`, `TimeFilter`, `ScoreFilter`
   - `DistanceMetric` - Cosine, Euclidean, Dot Product, Manhattan, Hamming
   - Filter extraction helpers for provider implementations

4. **`results.go`** (412 lines)
   - `QueryResult` - Results with matches, timing, explain info
   - `Match` - Single result with document, score, distance, rank
   - `UpsertResult` - Insert/update counts, deduplication info
   - `DeleteResult` - Deletion counts, not found tracking
   - `QueryTiming` - Performance metrics
   - `QueryExplain` - Execution plan details
   - Convenience methods: `TopMatch()`, `HasMore()`, `FilterByScore()`, etc.

5. **`options.go`** (509 lines)
   - `CollectionOption` - Functional options for collections
   - `CollectionConfig` - Configuration built from options
   - `BatchOption` - Functional options for batch operations
   - `BatchConfig` - Batch operation configuration
   - Options: `WithTTL`, `WithDeduplication`, `WithIndexing`, `WithScope`, etc.
   - Validation for all configs

6. **`iterator.go`** (395 lines)
   - `sliceIterator` - In-memory iterator implementation
   - `channelIterator` - Channel-based streaming iterator
   - `errorIterator` - Error propagation
   - `emptyIterator` - No results
   - Helper functions: `CollectAll`, `CollectN`, `ForEach`, `FilterIterator`, `MapIterator`

7. **`examples_test.go`** (488 lines)
   - Comprehensive examples for all 10 use cases
   - Basic CRUD operations
   - Semantic caching
   - Agent memory
   - Conversation history
   - Deduplication
   - Multi-modal search
   - Streaming queries
   - Batch operations
   - Complex filters
   - Pagination

8. **`ARCHITECTURE.md`** (752 lines)
   - Complete architecture documentation
   - Design principles and rationale
   - Type descriptions and examples
   - Provider implementation guide
   - Use case implementations
   - Performance considerations
   - Migration guide from old interface

## Key Design Features

### 1. Collection Abstraction

Collections provide isolated namespaces with use-case specific configuration:

```go
// Semantic cache
cache := store.Collection("cache",
    WithTTL(5*time.Minute),
    WithDeduplication(true),
)

// Agent memory
memory := store.Collection("agent-memory",
    WithScope("user", "session"),
)
```

### 2. Enhanced Document Model

Typed fields instead of just metadata map:

```go
type Document struct {
    ID        string       // Unique identifier
    Content   *Content     // Multi-modal (text/image/audio/video)
    Embedding *Embedding   // Vector + model metadata
    Scope     *Scope       // Multi-tenant isolation
    Temporal  *Temporal    // Time-based info
    Tags      []string     // Indexed labels
    Metadata  map[string]any // Free-form data
}
```

### 3. Composable Filters

Type-safe filter builders:

```go
query := &Query{
    Embedding: queryEmbedding,
    Filters: And(
        UserFilter("user123"),
        TagFilter("preference"),
        CreatedAfter(time.Now().Add(-24*time.Hour)),
        ScoreAbove(0.8),
    ),
    Limit: 10,
}
```

### 4. Rich Results

Results include timing and explain info:

```go
type QueryResult struct {
    Matches   []*Match
    Total     int64           // For pagination
    Timing    *QueryTiming    // Performance metrics
    Explain   *QueryExplain   // Execution plan
}
```

### 5. Streaming Support

Iterator pattern for large result sets:

```go
iter, _ := coll.QueryStream(ctx, query)
defer iter.Close()

for iter.Next() {
    match := iter.Match()
    // Process match
}
```

### 6. Batch Operations

Optimized bulk operations with progress tracking:

```go
result, _ := coll.UpsertBatch(ctx, documents,
    WithBatchSize(100),
    WithParallelism(4),
    WithProgressCallback(func(processed, total int) {
        log.Printf("Progress: %d/%d", processed, total)
    }),
)
```

### 7. Multi-modal Content

Support for text, images, audio, video:

```go
// Text
content := NewTextContent("Hello world")

// Image
content := NewImageURL("https://example.com/photo.jpg")
content := NewImageContent(imageBytes, "image/jpeg")

// Audio/Video
content := NewAudioContent(audioBytes, "audio/mp3")
content := NewVideoContent(videoBytes, "video/mp4")
```

## Use Case Support

### 1. Semantic Caching
- `WithTTL()` - Automatic expiration
- `WithDeduplication()` - Avoid duplicate cache entries
- `MinScore` in queries - High threshold for cache hits

### 2. Agent Memory
- `WithScope("user", "session")` - Scope requirement
- `Scope` struct - User/session/tenant isolation
- `ScopeFilter()` - Retrieve scoped memories

### 3. TTL/Expiration
- `Temporal.ExpiresAt` - Expiration timestamp
- `NewTemporalWithTTL()` - Helper for TTL
- `NotExpired()` filter - Exclude expired documents

### 4. Conversation History
- `Temporal.EventTime` - Event timestamp
- `SortByCreatedAt()` - Chronological ordering
- `Scope.Thread` - Conversation thread isolation

### 5. Deduplication
- `WithDeduplicationThreshold(0.95)` - Similarity threshold
- `UpsertResult.Deduplicated` - Tracking
- Content-based or embedding-based

### 6. User Profiles
- `Scope.User` - User identifier
- `UserFilter()` - User-specific queries
- `Scope.Tenant` - Organization-level isolation

### 7. Recommendations
- Vector similarity search
- Complex filters (tags, metadata, temporal)
- Hybrid ranking (similarity + metadata)

### 8. Anomaly Detection
- `ScoreBelow()` filter - Low similarity threshold
- `MinScore` in queries - Outlier detection
- Statistical filtering on results

### 9. Knowledge Graphs
- Metadata for relationships
- Tags for entity types
- Scope for graph partitioning

### 10. Multi-modal Search
- `ContentType` - text/image/audio/video
- `Content.Data` - Binary embeddings
- `Content.URL` - External media references

## API Comparison: Old vs New

### Old Interface

```go
// Old: Single flat interface
type VectorStore interface {
    Upsert(ctx, []Document) error
    Search(ctx, SearchQuery) ([]SearchResult, error)
    Delete(ctx, []string) error
    Get(ctx, []string) ([]Document, error)
}

// Old: Simple document
type Document struct {
    ID        string
    Content   string  // Text only
    Embedding []float32
    Metadata  map[string]interface{}
    CreatedAt time.Time
    UpdatedAt time.Time
}

// Old: Basic query
type SearchQuery struct {
    Embedding      []float32
    TopK           int
    Filter         *MetadataFilter
    MinScore       float32
    DistanceMetric DistanceMetric
}
```

### New Interface

```go
// New: Collection-based
type VectorStore interface {
    Collection(name string, opts ...CollectionOption) Collection
    ListCollections(ctx) ([]string, error)
    DeleteCollection(ctx, string) error
    Stats(ctx) (*StoreStats, error)
    Close() error
}

type Collection interface {
    Upsert(ctx, ...*Document) (*UpsertResult, error)
    UpsertBatch(ctx, []*Document, ...BatchOption) (*UpsertResult, error)
    Query(ctx, *Query) (*QueryResult, error)
    QueryStream(ctx, *Query) (ResultIterator, error)
    Get(ctx, ...string) ([]*Document, error)
    Delete(ctx, ...string) (*DeleteResult, error)
    DeleteByFilter(ctx, Filter) (*DeleteResult, error)
    Count(ctx, Filter) (int64, error)
    // ... more methods
}

// New: Enhanced document
type Document struct {
    ID        string
    Content   *Content     // Multi-modal
    Embedding *Embedding   // With metadata
    Scope     *Scope       // Typed isolation
    Temporal  *Temporal    // Rich time info
    Tags      []string     // Indexed labels
    Metadata  map[string]any
}

// New: Rich query
type Query struct {
    Embedding         *Embedding
    Filters           Filter      // Composable
    Limit/Offset      int
    MinScore          float32
    Metric            DistanceMetric
    IncludeEmbeddings bool
    IncludeContent    bool
    SortBy            []SortBy
    Explain           bool
}
```

## Implementation Status

### ✅ Completed

- Core interfaces (`VectorStore`, `Collection`)
- Enhanced document types (`Document`, `Content`, `Embedding`)
- Query and filter system (`Query`, `Filter` interface)
- Result types (`QueryResult`, `UpsertResult`, `DeleteResult`)
- Options system (functional options)
- Iterator implementations
- Validation functions
- Comprehensive examples
- Architecture documentation
- All code compiles successfully

### ⚠️ Requires Updates

The existing provider implementations need updates to match the new interface:

1. **`memory/memory.go`** - Needs:
   - Implement `Collection()` method on `MemoryVectorStore`
   - Update to use new `Document`, `Embedding`, `Query` types
   - Update validation to use new `Validate()` functions
   - Add collection support with configuration

2. **`firestore/firestore.go`** - Needs:
   - Implement `Collection()` method on `FirestoreVectorStore`
   - Update to use new types
   - Handle `Scope`, `Temporal`, `Tags` fields
   - Add collection management

3. **`registry.go`** - May need minor updates for new types

4. **`config.go`** - Consider deprecating in favor of functional options

## Migration Path for Providers

For provider implementers, here's the migration approach:

### 1. Add Collection Support

```go
type MyVectorStore struct {
    collections sync.Map // map[string]*myCollection
}

func (s *MyVectorStore) Collection(name string, opts ...CollectionOption) Collection {
    config := ApplyOptions(opts)

    // Return cached or create new
    if coll, ok := s.collections.Load(name); ok {
        return coll.(Collection)
    }

    coll := &myCollection{
        name:   name,
        config: config,
        store:  s,
    }
    s.collections.Store(name, coll)
    return coll
}
```

### 2. Update Document Handling

```go
// Old
doc.Content // string
doc.Embedding // []float32

// New
doc.Content.String() // Get text
doc.Embedding.Vector // []float32
doc.Embedding.Model // Track provenance

// Handle new fields
if doc.Scope != nil {
    // Apply scope-based isolation
}
if doc.Temporal != nil && doc.Temporal.ExpiresAt != nil {
    // Handle TTL
}
for _, tag := range doc.Tags {
    // Index tags
}
```

### 3. Implement Filter Processing

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

    if field, op, value, ok := GetFieldFilter(filter); ok {
        return applyFieldFilter(doc, field, op, value)
    }

    if tag, ok := GetTagFilter(filter); ok {
        return containsTag(doc.Tags, tag)
    }

    if scope, ok := GetScopeFilter(filter); ok {
        return doc.Scope.Match(scope)
    }

    // Handle other filter types...
}
```

## Next Steps

1. **Update Memory Provider** - Reference implementation
2. **Update Firestore Provider** - Production implementation
3. **Add Tests** - Comprehensive test suite
4. **Benchmark** - Performance testing
5. **Documentation** - godoc and usage guides
6. **Migration Guide** - For existing users

## Performance Characteristics

### Memory Overhead

- Old Document: ~200 bytes (string content, []float32)
- New Document: ~250 bytes (typed fields add ~50 bytes)
- Benefit: Better query performance through indexed fields

### Query Performance

- Filter composition: O(1) construction
- Filter evaluation: O(filters) per document
- Streaming: O(1) memory regardless of result size
- Batch operations: Configurable parallelism

### Trade-offs

- **More structure = More validation**: Good for correctness
- **Typed fields = More storage**: But better query performance
- **Rich results = Larger responses**: Use `IncludeEmbeddings: false`
- **Composable filters = More code**: But type-safe and testable

## Security Considerations

All implemented:

- ✅ Input validation on all boundaries
- ✅ NoSQL injection prevention (metadata keys)
- ✅ Path traversal prevention (document IDs)
- ✅ NaN/Inf detection (embeddings)
- ✅ Length limits (IDs, keys, tags)
- ✅ Control character filtering
- ✅ Reserved character blocking

## Backward Compatibility

**This is a clean rewrite - no backward compatibility requirements.**

Existing code using the old interface will need updates. The migration is straightforward:

```go
// Old
store.Upsert(ctx, docs)

// New
coll := store.Collection("default")
coll.Upsert(ctx, docs...)
```

For applications wanting gradual migration, consider:
1. Keep old interface as deprecated
2. Implement old interface as wrapper around new
3. Provide migration utilities

## Conclusion

This design delivers:

- ✅ Clean, idiomatic Go interfaces
- ✅ Native support for all 10 use cases
- ✅ Type safety without excessive complexity
- ✅ Performance-oriented (batch, streaming, zero-copy)
- ✅ Security-conscious validation
- ✅ Extensible through functional options
- ✅ Well-documented with examples

The core interfaces are complete and compiling. Provider implementations need updates but have clear migration paths.
