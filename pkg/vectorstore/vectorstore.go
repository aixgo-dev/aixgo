package vectorstore

import (
	"context"
	"io"
)

// VectorStore is the top-level interface for vector database operations.
// It provides methods for creating and managing collections, which are isolated
// namespaces for documents and embeddings.
//
// VectorStore acts as a factory for Collections, enabling multi-tenancy,
// use-case isolation, and organized storage of embeddings.
//
// Example:
//
//	store, err := memory.New()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer store.Close()
//
//	// Create collection for semantic caching
//	cache := store.Collection("cache",
//	    WithTTL(5*time.Minute),
//	    WithDeduplication(true),
//	)
//
//	// Create collection for agent memory
//	memory := store.Collection("agent-memory",
//	    WithScope("user", "session"),
//	)
type VectorStore interface {
	// Collection returns a Collection with the specified name and options.
	// Collections provide isolated namespaces for documents and embeddings.
	//
	// The name must be a valid collection identifier (alphanumeric, hyphens, underscores).
	// Options configure behavior like TTL, deduplication, indexing, etc.
	//
	// Collections are created lazily on first use. Calling Collection multiple times
	// with the same name returns the same logical collection.
	//
	// Example:
	//
	//	cache := store.Collection("cache", WithTTL(5*time.Minute))
	//	docs := store.Collection("documents", WithIndexing(IndexTypeHNSW))
	Collection(name string, opts ...CollectionOption) Collection

	// ListCollections returns the names of all collections in this store.
	// This is useful for administration and debugging.
	ListCollections(ctx context.Context) ([]string, error)

	// DeleteCollection permanently deletes a collection and all its documents.
	// This operation cannot be undone.
	//
	// Returns an error if the collection doesn't exist or cannot be deleted.
	DeleteCollection(ctx context.Context, name string) error

	// Stats returns statistics about the vector store.
	// This includes total collections, documents, storage size, etc.
	Stats(ctx context.Context) (*StoreStats, error)

	// Close closes the connection to the vector database and releases resources.
	// After Close is called, the VectorStore should not be used.
	Close() error
}

// Collection represents an isolated namespace for documents and embeddings.
// Collections enable use-case specific configurations like TTL, deduplication,
// scoping, and indexing strategies.
//
// All Collection methods are safe for concurrent use.
type Collection interface {
	// Name returns the collection name.
	Name() string

	// Upsert inserts or updates documents in the collection.
	// If a document with the same ID exists, it is updated. Otherwise, a new document is created.
	//
	// Documents are validated before insertion. Invalid documents cause the entire
	// operation to fail (all-or-nothing semantics).
	//
	// For large batches, consider using UpsertBatch for better performance.
	//
	// Example:
	//
	//	doc := &Document{
	//	    ID: "doc1",
	//	    Content: NewTextContent("Hello world"),
	//	    Embedding: NewEmbedding([]float32{0.1, 0.2, 0.3}, "text-embedding-3-small"),
	//	    Tags: []string{"greeting", "english"},
	//	}
	//	result, err := coll.Upsert(ctx, doc)
	Upsert(ctx context.Context, documents ...*Document) (*UpsertResult, error)

	// UpsertBatch performs batch upsert with progress tracking and error handling.
	// This is optimized for inserting large numbers of documents.
	//
	// Options can control batch size, parallelism, and progress callbacks.
	//
	// Example:
	//
	//	result, err := coll.UpsertBatch(ctx, documents,
	//	    WithBatchSize(100),
	//	    WithProgressCallback(func(processed, total int) {
	//	        log.Printf("Progress: %d/%d", processed, total)
	//	    }),
	//	)
	UpsertBatch(ctx context.Context, documents []*Document, opts ...BatchOption) (*UpsertResult, error)

	// Query performs similarity search and returns matching documents.
	// The query can include vector similarity, metadata filters, temporal constraints,
	// scope filters, and more.
	//
	// Example:
	//
	//	results, err := coll.Query(ctx, &Query{
	//	    Embedding: queryEmbedding,
	//	    Limit: 10,
	//	    Filters: And(
	//	        TagFilter("category", "product"),
	//	        ScoreFilter(GreaterThan(0.8)),
	//	    ),
	//	})
	Query(ctx context.Context, query *Query) (*QueryResult, error)

	// QueryStream performs similarity search and streams results via an iterator.
	// This is useful for processing large result sets without loading everything into memory.
	//
	// The iterator must be closed when done to release resources.
	//
	// Example:
	//
	//	iter, err := coll.QueryStream(ctx, query)
	//	if err != nil {
	//	    log.Fatal(err)
	//	}
	//	defer iter.Close()
	//
	//	for iter.Next() {
	//	    match := iter.Match()
	//	    fmt.Printf("Score: %.4f, Content: %s\n", match.Score, match.Document.Content)
	//	}
	//	if err := iter.Err(); err != nil {
	//	    log.Fatal(err)
	//	}
	QueryStream(ctx context.Context, query *Query) (ResultIterator, error)

	// Get retrieves documents by their IDs.
	// Documents that don't exist are omitted from the result (no error is returned).
	//
	// The order of returned documents may not match the order of requested IDs.
	//
	// Example:
	//
	//	docs, err := coll.Get(ctx, "doc1", "doc2", "doc3")
	Get(ctx context.Context, ids ...string) ([]*Document, error)

	// Delete removes documents by their IDs.
	// IDs that don't exist are silently ignored (no error is returned).
	//
	// Example:
	//
	//	result, err := coll.Delete(ctx, "doc1", "doc2")
	//	fmt.Printf("Deleted %d documents\n", result.Deleted)
	Delete(ctx context.Context, ids ...string) (*DeleteResult, error)

	// DeleteByFilter removes all documents matching the filter.
	// This is useful for bulk deletion based on criteria.
	//
	// WARNING: This can delete many documents. Use with caution.
	//
	// Example:
	//
	//	// Delete all expired documents
	//	result, err := coll.DeleteByFilter(ctx,
	//	    TimeFilter(Before(time.Now())),
	//	)
	DeleteByFilter(ctx context.Context, filter Filter) (*DeleteResult, error)

	// Count returns the number of documents in the collection.
	// If a filter is provided, only documents matching the filter are counted.
	//
	// Example:
	//
	//	total, err := coll.Count(ctx, nil) // All documents
	//	active, err := coll.Count(ctx, TagFilter("status", "active"))
	Count(ctx context.Context, filter Filter) (int64, error)

	// Stats returns statistics about the collection.
	// This includes document count, storage size, index info, etc.
	Stats(ctx context.Context) (*CollectionStats, error)

	// Clear removes all documents from the collection.
	// This is primarily useful for testing.
	//
	// WARNING: This permanently deletes all data in the collection.
	Clear(ctx context.Context) error
}

// ResultIterator provides streaming access to query results.
// It follows the iterator pattern common in Go database libraries.
type ResultIterator interface {
	// Next advances to the next result.
	// Returns true if there is a result, false if iteration is complete or an error occurred.
	//
	// Always check Err() after Next returns false to distinguish between
	// normal completion and errors.
	Next() bool

	// Match returns the current search match.
	// Only valid after Next returns true.
	Match() *Match

	// Err returns any error that occurred during iteration.
	// Should be checked after Next returns false.
	Err() error

	// Close releases resources associated with the iterator.
	// Always call Close when done, typically via defer.
	Close() error
}

// StoreStats contains statistics about the entire vector store.
type StoreStats struct {
	// Collections is the total number of collections
	Collections int64

	// Documents is the total number of documents across all collections
	Documents int64

	// StorageBytes is the total storage used in bytes
	StorageBytes int64

	// Provider is the vector store provider name (e.g., "memory", "firestore", "qdrant")
	Provider string

	// Version is the provider version
	Version string

	// Extra contains provider-specific statistics
	Extra map[string]any
}

// CollectionStats contains statistics about a specific collection.
type CollectionStats struct {
	// Name is the collection name
	Name string

	// Documents is the number of documents in the collection
	Documents int64

	// StorageBytes is the storage used by this collection in bytes
	StorageBytes int64

	// EmbeddingDimensions is the dimensionality of embeddings in this collection
	EmbeddingDimensions int

	// IndexType is the index type used (e.g., "flat", "hnsw", "ivf")
	IndexType string

	// CreatedAt is when the collection was created
	CreatedAt *TimestampValue

	// UpdatedAt is when the collection was last modified
	UpdatedAt *TimestampValue

	// Extra contains provider-specific statistics
	Extra map[string]any
}

// Ensure io.Closer is satisfied
var _ io.Closer = (VectorStore)(nil)
var _ io.Closer = (ResultIterator)(nil)
