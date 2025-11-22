package vectorstore

import (
	"context"
	"fmt"
	"time"
)

// VectorStore is the main interface for vector database operations.
// It provides methods for storing, searching, and managing documents with embeddings.
type VectorStore interface {
	// Upsert inserts or updates documents with embeddings
	Upsert(ctx context.Context, documents []Document) error

	// Search performs similarity search and returns the most similar documents
	Search(ctx context.Context, query SearchQuery) ([]SearchResult, error)

	// Delete removes documents by their IDs
	Delete(ctx context.Context, ids []string) error

	// Get retrieves documents by their IDs
	Get(ctx context.Context, ids []string) ([]Document, error)

	// Close closes the connection to the vector database
	Close() error
}

// Document represents a document with embeddings and metadata.
type Document struct {
	// ID is the unique identifier for the document
	ID string `json:"id"`

	// Content is the text content of the document
	Content string `json:"content"`

	// Embedding is the vector representation of the content
	Embedding []float32 `json:"embedding"`

	// Metadata contains additional information about the document
	// Common fields: source, author, date, document_type, tags, etc.
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// CreatedAt is when the document was first created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the document was last updated
	UpdatedAt time.Time `json:"updated_at"`
}

// SearchQuery defines the parameters for a similarity search.
type SearchQuery struct {
	// Embedding is the query vector to search for
	Embedding []float32

	// TopK is the number of results to return (default: 10)
	TopK int

	// Filter is optional metadata filtering for hybrid search
	Filter *MetadataFilter

	// MinScore is the minimum similarity score (0.0-1.0)
	// Documents with lower similarity scores will be excluded
	MinScore float32

	// DistanceMetric specifies how to calculate similarity
	// Supported values: DistanceMetricCosine (default), DistanceMetricEuclidean, DistanceMetricDotProduct
	DistanceMetric DistanceMetric
}

// SearchResult represents a single search result with similarity score.
type SearchResult struct {
	// Document is the matched document
	Document Document

	// Score is the similarity score (higher is more similar)
	// For cosine similarity: 0.0 (opposite) to 1.0 (identical)
	Score float32

	// Distance is the raw distance metric (optional)
	Distance float32
}

// MetadataFilter defines conditions for filtering documents by metadata.
type MetadataFilter struct {
	// Must contains conditions that all must be true (AND)
	// Example: {"source": "documentation", "status": "published"}
	Must map[string]interface{}

	// Should contains conditions where at least one must be true (OR)
	// Note: Since this is a map, each key must be unique. Use a slice of conditions
	// for multiple values of the same key. Example: {"category": "guide", "type": "tutorial"}
	Should map[string]interface{}

	// MustNot contains conditions that must not be true (NOT)
	// Example: {"status": "draft"}
	MustNot map[string]interface{}
}

// DistanceMetric represents the method for calculating vector similarity.
type DistanceMetric string

const (
	// DistanceMetricCosine calculates cosine similarity (default)
	// Range: -1 (opposite) to 1 (identical)
	// Best for: Most text embeddings (normalized vectors)
	DistanceMetricCosine DistanceMetric = "cosine"

	// DistanceMetricEuclidean calculates Euclidean (L2) distance
	// Range: 0 (identical) to infinity (different)
	// Best for: When magnitude matters
	DistanceMetricEuclidean DistanceMetric = "euclidean"

	// DistanceMetricDotProduct calculates dot product similarity
	// Range: -infinity to +infinity
	// Best for: Normalized vectors, faster than cosine
	DistanceMetricDotProduct DistanceMetric = "dot_product"
)

// ValidateDocument checks if a document is valid before storage.
func ValidateDocument(doc *Document) error {
	// Validate document ID
	if err := ValidateDocumentID(doc.ID); err != nil {
		return fmt.Errorf("invalid document ID: %w", err)
	}
	if doc.Content == "" {
		return fmt.Errorf("document content cannot be empty")
	}
	if len(doc.Embedding) == 0 {
		return fmt.Errorf("document embedding cannot be empty")
	}
	// Check for NaN or Inf values in embedding
	for i, val := range doc.Embedding {
		if isNaN(val) || isInf(val) {
			return fmt.Errorf("embedding contains invalid value at index %d: %f", i, val)
		}
	}
	// Validate metadata keys to prevent injection attacks
	for key := range doc.Metadata {
		if err := ValidateMetadataKey(key); err != nil {
			return fmt.Errorf("invalid metadata key %q: %w", key, err)
		}
	}
	return nil
}

// ValidateSearchQuery checks if a search query is valid.
func ValidateSearchQuery(query *SearchQuery) error {
	if len(query.Embedding) == 0 {
		return fmt.Errorf("query embedding cannot be empty")
	}

	// Check for NaN or Inf values in query embedding
	for i, val := range query.Embedding {
		if isNaN(val) || isInf(val) {
			return fmt.Errorf("query embedding contains invalid value at index %d: %f", i, val)
		}
	}

	if query.TopK < 1 {
		return fmt.Errorf("TopK must be at least 1, got %d", query.TopK)
	}
	if query.TopK > 1000 {
		return fmt.Errorf("TopK cannot exceed 1000, got %d", query.TopK)
	}

	// Validate MinScore based on distance metric
	if query.MinScore != 0 { // Only validate if MinScore is set
		switch query.DistanceMetric {
		case DistanceMetricDotProduct:
			// Dot product can be any value, so we only check for invalid values
			if isNaN(query.MinScore) || isInf(query.MinScore) {
				return fmt.Errorf("MinScore contains invalid value: %f", query.MinScore)
			}
		case DistanceMetricCosine, DistanceMetricEuclidean, "":
			// For cosine and euclidean (and default), require 0 to 1
			// Cosine: typically 0-1 for normalized vectors (standard for embeddings)
			// Euclidean: our conversion formula (1/(1+dist)) always yields 0-1
			if query.MinScore < 0 || query.MinScore > 1 {
				return fmt.Errorf("MinScore must be between 0 and 1, got %f", query.MinScore)
			}
		default:
			// Unknown metric - use safe default of 0 to 1
			if query.MinScore < 0 || query.MinScore > 1 {
				return fmt.Errorf("MinScore must be between 0 and 1, got %f", query.MinScore)
			}
		}
	}

	// Validate distance metric
	if query.DistanceMetric != "" {
		if query.DistanceMetric != DistanceMetricCosine &&
			query.DistanceMetric != DistanceMetricEuclidean &&
			query.DistanceMetric != DistanceMetricDotProduct {
			return fmt.Errorf("invalid distance metric: %s", query.DistanceMetric)
		}
	}

	return nil
}

// ValidateMetadataKey checks if a metadata key is safe to use.
// This prevents NoSQL injection attacks via metadata keys.
func ValidateMetadataKey(key string) error {
	if key == "" {
		return fmt.Errorf("metadata key cannot be empty")
	}
	if len(key) > 256 {
		return fmt.Errorf("metadata key too long: maximum 256 characters, got %d", len(key))
	}
	// Disallow control characters, null bytes, and special characters that could be used in injection attacks
	for i, r := range key {
		if r < 0x20 || r == 0x7F { // Control characters
			return fmt.Errorf("metadata key contains control character at position %d", i)
		}
		if r == '$' || r == '.' {
			return fmt.Errorf("metadata key contains forbidden character '%c' at position %d (reserved for internal use)", r, i)
		}
	}
	return nil
}

// ValidateDocumentID checks if a document ID is safe to use.
// This prevents path traversal and injection attacks.
func ValidateDocumentID(id string) error {
	if id == "" {
		return fmt.Errorf("document ID cannot be empty")
	}
	if len(id) > 512 {
		return fmt.Errorf("document ID too long: maximum 512 characters, got %d", len(id))
	}
	// Disallow path traversal sequences
	if id == "." || id == ".." {
		return fmt.Errorf("document ID cannot be '.' or '..'")
	}
	// Disallow path separators and control characters
	for i, r := range id {
		if r < 0x20 || r == 0x7F { // Control characters
			return fmt.Errorf("document ID contains control character at position %d", i)
		}
		if r == '/' || r == '\\' {
			return fmt.Errorf("document ID contains path separator at position %d", i)
		}
		if r == 0 { // Null byte
			return fmt.Errorf("document ID contains null byte at position %d", i)
		}
	}
	return nil
}

// Helper functions for float validation
func isNaN(f float32) bool {
	return f != f
}

func isInf(f float32) bool {
	return f > maxFloat32 || f < -maxFloat32
}

const maxFloat32 = 3.40282346638528859811704183484516925440e+38
