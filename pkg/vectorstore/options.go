package vectorstore

import (
	"fmt"
	"time"
)

// CollectionOption configures a Collection.
// Options are applied when creating or accessing a collection.
type CollectionOption func(*CollectionConfig)

// CollectionConfig contains the configuration for a collection.
// This is built from CollectionOption functions.
type CollectionConfig struct {
	// TTL specifies the time-to-live for documents in this collection.
	// Documents are automatically deleted after TTL expires.
	// Zero means no TTL (documents never expire).
	TTL time.Duration

	// EnableDeduplication enables content-based deduplication.
	// When enabled, documents with identical embeddings (or content hashes)
	// are deduplicated automatically.
	EnableDeduplication bool

	// DeduplicationThreshold is the similarity threshold for deduplication (0.0-1.0).
	// Documents with similarity >= threshold are considered duplicates.
	// Default: 0.99 (nearly identical)
	DeduplicationThreshold float32

	// IndexType specifies the vector index type.
	// Examples: "flat", "hnsw", "ivf"
	IndexType IndexType

	// EmbeddingDimensions is the expected dimensionality of embeddings.
	// If set, documents with different dimensions will be rejected.
	// Zero means no dimension validation.
	EmbeddingDimensions int

	// AutoGenerateEmbeddings enables automatic embedding generation.
	// When enabled, documents without embeddings will have them generated
	// using the specified embedding function.
	AutoGenerateEmbeddings bool

	// EmbeddingFunction is the function to generate embeddings.
	// Only used if AutoGenerateEmbeddings is true.
	EmbeddingFunction EmbeddingFunction

	// ScopeRequired specifies required scope fields.
	// Documents without these scope fields will be rejected.
	ScopeRequired []string

	// MaxDocuments limits the number of documents in the collection.
	// Oldest documents are removed when limit is exceeded (FIFO).
	// Zero means no limit.
	MaxDocuments int64

	// EnableVersioning enables document versioning.
	// Previous versions are retained and can be queried.
	EnableVersioning bool

	// MaxVersions limits the number of versions per document.
	// Only meaningful if EnableVersioning is true.
	// Zero means unlimited versions.
	MaxVersions int

	// EnableAuditLog enables audit logging for all operations.
	EnableAuditLog bool

	// Metadata contains additional provider-specific configuration.
	Metadata map[string]any
}

// IndexType represents the type of vector index.
type IndexType string

const (
	// IndexTypeFlat performs brute-force (exact) search.
	// Best for: Small collections (<10K documents), maximum accuracy.
	IndexTypeFlat IndexType = "flat"

	// IndexTypeHNSW uses Hierarchical Navigable Small World graph.
	// Best for: Large collections, good balance of speed and accuracy.
	IndexTypeHNSW IndexType = "hnsw"

	// IndexTypeIVF uses Inverted File with Product Quantization.
	// Best for: Very large collections, faster but less accurate.
	IndexTypeIVF IndexType = "ivf"

	// IndexTypeAuto lets the provider choose based on collection size.
	IndexTypeAuto IndexType = "auto"
)

// EmbeddingFunction generates embeddings for content.
type EmbeddingFunction func(content *Content) (*Embedding, error)

// WithTTL sets the time-to-live for documents in the collection.
//
// Example:
//
//	cache := store.Collection("cache", WithTTL(5*time.Minute))
func WithTTL(ttl time.Duration) CollectionOption {
	return func(c *CollectionConfig) {
		c.TTL = ttl
	}
}

// WithDeduplication enables content-based deduplication.
// Documents with similarity >= 0.99 are considered duplicates.
//
// Example:
//
//	docs := store.Collection("docs", WithDeduplication(true))
func WithDeduplication(enabled bool) CollectionOption {
	return func(c *CollectionConfig) {
		c.EnableDeduplication = enabled
		if enabled && c.DeduplicationThreshold == 0 {
			c.DeduplicationThreshold = 0.99
		}
	}
}

// WithDeduplicationThreshold sets the similarity threshold for deduplication.
// Also enables deduplication.
//
// Example:
//
//	docs := store.Collection("docs", WithDeduplicationThreshold(0.95))
func WithDeduplicationThreshold(threshold float32) CollectionOption {
	return func(c *CollectionConfig) {
		c.EnableDeduplication = true
		c.DeduplicationThreshold = threshold
	}
}

// WithIndexing sets the vector index type.
//
// Example:
//
//	docs := store.Collection("docs", WithIndexing(IndexTypeHNSW))
func WithIndexing(indexType IndexType) CollectionOption {
	return func(c *CollectionConfig) {
		c.IndexType = indexType
	}
}

// WithDimensions sets the expected embedding dimensions.
// Documents with different dimensions will be rejected.
//
// Example:
//
//	docs := store.Collection("docs", WithDimensions(768))
func WithDimensions(dimensions int) CollectionOption {
	return func(c *CollectionConfig) {
		c.EmbeddingDimensions = dimensions
	}
}

// WithAutoEmbeddings enables automatic embedding generation.
//
// Example:
//
//	docs := store.Collection("docs",
//	    WithAutoEmbeddings(myEmbeddingFunc),
//	)
func WithAutoEmbeddings(fn EmbeddingFunction) CollectionOption {
	return func(c *CollectionConfig) {
		c.AutoGenerateEmbeddings = true
		c.EmbeddingFunction = fn
	}
}

// WithScope specifies required scope fields.
// Documents without these fields will be rejected.
//
// Example:
//
//	memory := store.Collection("memory",
//	    WithScope("user", "session"),
//	)
func WithScope(fields ...string) CollectionOption {
	return func(c *CollectionConfig) {
		c.ScopeRequired = fields
	}
}

// WithMaxDocuments limits the collection size.
// Oldest documents are removed when limit is exceeded.
//
// Example:
//
//	cache := store.Collection("cache",
//	    WithMaxDocuments(10000),
//	)
func WithMaxDocuments(max int64) CollectionOption {
	return func(c *CollectionConfig) {
		c.MaxDocuments = max
	}
}

// WithVersioning enables document versioning.
//
// Example:
//
//	docs := store.Collection("docs",
//	    WithVersioning(true),
//	)
func WithVersioning(enabled bool) CollectionOption {
	return func(c *CollectionConfig) {
		c.EnableVersioning = enabled
	}
}

// WithMaxVersions limits the number of versions per document.
// Also enables versioning.
//
// Example:
//
//	docs := store.Collection("docs",
//	    WithMaxVersions(5),
//	)
func WithMaxVersions(max int) CollectionOption {
	return func(c *CollectionConfig) {
		c.EnableVersioning = true
		c.MaxVersions = max
	}
}

// WithAuditLog enables audit logging for the collection.
//
// Example:
//
//	docs := store.Collection("docs",
//	    WithAuditLog(true),
//	)
func WithAuditLog(enabled bool) CollectionOption {
	return func(c *CollectionConfig) {
		c.EnableAuditLog = enabled
	}
}

// WithMetadata adds custom metadata to the collection config.
//
// Example:
//
//	docs := store.Collection("docs",
//	    WithMetadata(map[string]any{
//	        "shard_count": 4,
//	        "replication": 3,
//	    }),
//	)
func WithMetadata(metadata map[string]any) CollectionOption {
	return func(c *CollectionConfig) {
		if c.Metadata == nil {
			c.Metadata = make(map[string]any)
		}
		for k, v := range metadata {
			c.Metadata[k] = v
		}
	}
}

// ApplyOptions applies a list of options to a config.
// This is used internally by collection implementations.
func ApplyOptions(opts []CollectionOption) *CollectionConfig {
	config := &CollectionConfig{
		DeduplicationThreshold: 0.99,
		IndexType:              IndexTypeAuto,
	}
	for _, opt := range opts {
		opt(config)
	}
	return config
}

// BatchOption configures batch operations.
type BatchOption func(*BatchConfig)

// BatchConfig contains configuration for batch operations.
type BatchConfig struct {
	// BatchSize is the number of documents to process per batch.
	// Default: 100
	BatchSize int

	// Parallelism is the number of concurrent batches.
	// Default: 1 (sequential)
	Parallelism int

	// ContinueOnError controls whether to continue on individual document errors.
	// If false, the entire batch fails on first error.
	// If true, failed documents are collected in UpsertResult.Errors.
	// Default: true
	ContinueOnError bool

	// ProgressCallback is called after each batch completes.
	// Receives (processed, total) counts.
	ProgressCallback func(processed, total int)

	// ValidateBeforeBatch validates all documents before starting batch.
	// If true, invalid documents cause immediate failure.
	// If false, validation happens per-batch.
	// Default: false
	ValidateBeforeBatch bool

	// RetryOnError enables retry for failed batches.
	// Default: false
	RetryOnError bool

	// MaxRetries is the maximum number of retries per batch.
	// Only meaningful if RetryOnError is true.
	// Default: 3
	MaxRetries int

	// RetryDelay is the delay between retries.
	// Default: 1 second
	RetryDelay time.Duration
}

// WithBatchSize sets the batch size.
//
// Example:
//
//	result, err := coll.UpsertBatch(ctx, docs,
//	    WithBatchSize(100),
//	)
func WithBatchSize(size int) BatchOption {
	return func(c *BatchConfig) {
		c.BatchSize = size
	}
}

// WithParallelism sets the number of concurrent batches.
//
// Example:
//
//	result, err := coll.UpsertBatch(ctx, docs,
//	    WithBatchSize(100),
//	    WithParallelism(4),
//	)
func WithParallelism(n int) BatchOption {
	return func(c *BatchConfig) {
		c.Parallelism = n
	}
}

// WithContinueOnError sets whether to continue on errors.
//
// Example:
//
//	result, err := coll.UpsertBatch(ctx, docs,
//	    WithContinueOnError(false), // Fail fast
//	)
func WithContinueOnError(enabled bool) BatchOption {
	return func(c *BatchConfig) {
		c.ContinueOnError = enabled
	}
}

// WithProgressCallback sets a progress callback.
//
// Example:
//
//	result, err := coll.UpsertBatch(ctx, docs,
//	    WithProgressCallback(func(processed, total int) {
//	        log.Printf("Progress: %d/%d (%.1f%%)",
//	            processed, total,
//	            float64(processed)/float64(total)*100)
//	    }),
//	)
func WithProgressCallback(callback func(processed, total int)) BatchOption {
	return func(c *BatchConfig) {
		c.ProgressCallback = callback
	}
}

// WithValidation enables pre-batch validation.
//
// Example:
//
//	result, err := coll.UpsertBatch(ctx, docs,
//	    WithValidation(true),
//	)
func WithValidation(enabled bool) BatchOption {
	return func(c *BatchConfig) {
		c.ValidateBeforeBatch = enabled
	}
}

// WithRetry enables retry on batch failures.
//
// Example:
//
//	result, err := coll.UpsertBatch(ctx, docs,
//	    WithRetry(true),
//	    WithMaxRetries(5),
//	    WithRetryDelay(2*time.Second),
//	)
func WithRetry(enabled bool) BatchOption {
	return func(c *BatchConfig) {
		c.RetryOnError = enabled
		if enabled && c.MaxRetries == 0 {
			c.MaxRetries = 3
		}
		if enabled && c.RetryDelay == 0 {
			c.RetryDelay = time.Second
		}
	}
}

// WithMaxRetries sets the maximum number of retries.
// Also enables retry.
//
// Example:
//
//	result, err := coll.UpsertBatch(ctx, docs,
//	    WithMaxRetries(5),
//	)
func WithMaxRetries(max int) BatchOption {
	return func(c *BatchConfig) {
		c.RetryOnError = true
		c.MaxRetries = max
		if c.RetryDelay == 0 {
			c.RetryDelay = time.Second
		}
	}
}

// WithRetryDelay sets the delay between retries.
// Also enables retry.
//
// Example:
//
//	result, err := coll.UpsertBatch(ctx, docs,
//	    WithRetryDelay(2*time.Second),
//	)
func WithRetryDelay(delay time.Duration) BatchOption {
	return func(c *BatchConfig) {
		c.RetryOnError = true
		c.RetryDelay = delay
		if c.MaxRetries == 0 {
			c.MaxRetries = 3
		}
	}
}

// ApplyBatchOptions applies a list of batch options to a config.
// This is used internally by collection implementations.
func ApplyBatchOptions(opts []BatchOption) *BatchConfig {
	config := &BatchConfig{
		BatchSize:       100,
		Parallelism:     1,
		ContinueOnError: true,
		MaxRetries:      3,
		RetryDelay:      time.Second,
	}
	for _, opt := range opts {
		opt(config)
	}
	return config
}

// Validate validates a CollectionConfig.
func (c *CollectionConfig) Validate() error {
	if c.TTL < 0 {
		return fmt.Errorf("TTL cannot be negative")
	}

	if c.DeduplicationThreshold < 0 || c.DeduplicationThreshold > 1 {
		return fmt.Errorf("deduplication threshold must be between 0 and 1, got %f", c.DeduplicationThreshold)
	}

	if c.EmbeddingDimensions < 0 {
		return fmt.Errorf("embedding dimensions cannot be negative")
	}

	if c.MaxDocuments < 0 {
		return fmt.Errorf("max documents cannot be negative")
	}

	if c.MaxVersions < 0 {
		return fmt.Errorf("max versions cannot be negative")
	}

	if c.AutoGenerateEmbeddings && c.EmbeddingFunction == nil {
		return fmt.Errorf("embedding function required when auto embeddings enabled")
	}

	return nil
}

// Validate validates a BatchConfig.
func (c *BatchConfig) Validate() error {
	if c.BatchSize < 1 {
		return fmt.Errorf("batch size must be at least 1, got %d", c.BatchSize)
	}

	if c.Parallelism < 1 {
		return fmt.Errorf("parallelism must be at least 1, got %d", c.Parallelism)
	}

	if c.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}

	if c.RetryDelay < 0 {
		return fmt.Errorf("retry delay cannot be negative")
	}

	return nil
}
