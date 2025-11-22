package memory

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/aixgo-dev/aixgo/pkg/vectorstore"
)

// MemoryVectorStore implements an in-memory vector store for testing and development.
// It uses brute-force search and is not suitable for production with large datasets.
type MemoryVectorStore struct {
	documents       map[string]vectorstore.Document
	maxDocuments    int
	defaultTopK     int
	defaultMetric   string
	embeddingDims   int
	mu              sync.RWMutex
}

func init() {
	// Register the memory provider with the vector store registry
	vectorstore.Register("memory", New)
}

// New creates a new MemoryVectorStore from the provided configuration.
func New(config vectorstore.Config) (vectorstore.VectorStore, error) {
	if config.EmbeddingDimensions <= 0 {
		return nil, fmt.Errorf("embedding dimensions must be greater than 0, got %d", config.EmbeddingDimensions)
	}

	maxDocs := 10000
	if config.Memory != nil && config.Memory.MaxDocuments > 0 {
		maxDocs = config.Memory.MaxDocuments
	}

	return &MemoryVectorStore{
		documents:     make(map[string]vectorstore.Document),
		maxDocuments:  maxDocs,
		defaultTopK:   config.DefaultTopK,
		defaultMetric: config.DefaultDistanceMetric,
		embeddingDims: config.EmbeddingDimensions,
	}, nil
}

// Upsert inserts or updates documents with embeddings.
func (m *MemoryVectorStore) Upsert(ctx context.Context, documents []vectorstore.Document) error {
	if len(documents) == 0 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate all documents before upserting
	for i := range documents {
		if err := vectorstore.ValidateDocument(&documents[i]); err != nil {
			return fmt.Errorf("invalid document at index %d: %w", i, err)
		}
		// Verify embedding dimensions match configuration
		if len(documents[i].Embedding) != m.embeddingDims {
			return fmt.Errorf("document %s embedding dimension mismatch: expected %d, got %d",
				documents[i].ID, m.embeddingDims, len(documents[i].Embedding))
		}
	}

	// Check if we would exceed max documents
	newDocsCount := 0
	for _, doc := range documents {
		if _, exists := m.documents[doc.ID]; !exists {
			newDocsCount++
		}
	}

	if len(m.documents)+newDocsCount > m.maxDocuments {
		return fmt.Errorf("would exceed max documents limit: %d (current: %d, adding: %d)",
			m.maxDocuments, len(m.documents), newDocsCount)
	}

	// Upsert documents
	for _, doc := range documents {
		// Deep copy to prevent external mutations
		docCopy := deepCopyDocument(doc)
		m.documents[doc.ID] = docCopy
	}

	return nil
}

// Search performs brute-force similarity search.
func (m *MemoryVectorStore) Search(ctx context.Context, query vectorstore.SearchQuery) ([]vectorstore.SearchResult, error) {
	// Set defaults
	if query.TopK == 0 {
		query.TopK = m.defaultTopK
	}
	if query.DistanceMetric == "" {
		query.DistanceMetric = vectorstore.DistanceMetric(m.defaultMetric)
	}

	// Validate query
	if err := vectorstore.ValidateSearchQuery(&query); err != nil {
		return nil, fmt.Errorf("invalid search query: %w", err)
	}

	// Verify embedding dimensions
	if len(query.Embedding) != m.embeddingDims {
		return nil, fmt.Errorf("query embedding dimension mismatch: expected %d, got %d",
			m.embeddingDims, len(query.Embedding))
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Brute-force search through all documents
	var candidates []vectorstore.SearchResult

	for _, doc := range m.documents {
		// Apply metadata filters if provided
		if query.Filter != nil && !matchesFilter(doc, query.Filter) {
			continue
		}

		// Calculate similarity
		score := calculateSimilarity(query.Embedding, doc.Embedding, query.DistanceMetric)

		// Filter by minimum score
		if query.MinScore > 0 && score < query.MinScore {
			continue
		}

		candidates = append(candidates, vectorstore.SearchResult{
			Document: doc,
			Score:    score,
		})
	}

	// Sort by score (descending)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	// Return top-K results
	if len(candidates) > query.TopK {
		candidates = candidates[:query.TopK]
	}

	return candidates, nil
}

// Delete removes documents by their IDs.
func (m *MemoryVectorStore) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, id := range ids {
		delete(m.documents, id)
	}

	return nil
}

// Get retrieves documents by their IDs.
func (m *MemoryVectorStore) Get(ctx context.Context, ids []string) ([]vectorstore.Document, error) {
	if len(ids) == 0 {
		return []vectorstore.Document{}, nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var documents []vectorstore.Document

	for _, id := range ids {
		if doc, exists := m.documents[id]; exists {
			// Deep copy to prevent external mutations
			docCopy := deepCopyDocument(doc)
			documents = append(documents, docCopy)
		}
	}

	return documents, nil
}

// Close is a no-op for memory store but implements the interface.
func (m *MemoryVectorStore) Close() error {
	return nil
}

// Count returns the number of documents stored (useful for testing).
func (m *MemoryVectorStore) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.documents)
}

// Clear removes all documents (useful for testing).
func (m *MemoryVectorStore) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.documents = make(map[string]vectorstore.Document)
}

// Helper functions

func matchesFilter(doc vectorstore.Document, filter *vectorstore.MetadataFilter) bool {
	// Check Must conditions (all must be true)
	if filter.Must != nil {
		for key, expectedValue := range filter.Must {
			actualValue, exists := doc.Metadata[key]
			if !exists || actualValue != expectedValue {
				return false
			}
		}
	}

	// Check Should conditions (at least one must be true)
	if len(filter.Should) > 0 {
		matchedAny := false
		for key, expectedValue := range filter.Should {
			actualValue, exists := doc.Metadata[key]
			if exists && actualValue == expectedValue {
				matchedAny = true
				break
			}
		}
		if !matchedAny {
			return false
		}
	}

	// Check MustNot conditions (none must be true)
	if filter.MustNot != nil {
		for key, rejectedValue := range filter.MustNot {
			actualValue, exists := doc.Metadata[key]
			if exists && actualValue == rejectedValue {
				return false
			}
		}
	}

	return true
}

func calculateSimilarity(vec1, vec2 []float32, metric vectorstore.DistanceMetric) float32 {
	switch metric {
	case vectorstore.DistanceMetricCosine:
		return cosineSimilarity(vec1, vec2)
	case vectorstore.DistanceMetricDotProduct:
		return dotProduct(vec1, vec2)
	case vectorstore.DistanceMetricEuclidean:
		// Convert distance to similarity (closer = higher score)
		dist := euclideanDistance(vec1, vec2)
		return 1.0 / (1.0 + dist)
	default:
		return cosineSimilarity(vec1, vec2)
	}
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProd, normA, normB float32
	for i := range a {
		dotProd += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProd / (sqrt(normA) * sqrt(normB))
}

func dotProduct(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var result float32
	for i := range a {
		result += a[i] * b[i]
	}
	return result
}

func euclideanDistance(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var sum float32
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return sqrt(sum)
}

func sqrt(x float32) float32 {
	return float32(math.Sqrt(float64(x)))
}

// deepCopyDocument creates a deep copy of a document to prevent external mutations.
func deepCopyDocument(doc vectorstore.Document) vectorstore.Document {
	// Deep copy embedding slice
	embeddingCopy := make([]float32, len(doc.Embedding))
	copy(embeddingCopy, doc.Embedding)

	// Deep copy metadata map
	var metadataCopy map[string]interface{}
	if doc.Metadata != nil {
		metadataCopy = make(map[string]interface{}, len(doc.Metadata))
		for k, v := range doc.Metadata {
			// For nested structures, we'd need recursive deep copy,
			// but for most use cases, shallow copy of values is sufficient
			// since metadata values are typically primitives
			metadataCopy[k] = v
		}
	}

	return vectorstore.Document{
		ID:        doc.ID,
		Content:   doc.Content,
		Embedding: embeddingCopy,
		Metadata:  metadataCopy,
		CreatedAt: doc.CreatedAt,
		UpdatedAt: doc.UpdatedAt,
	}
}
