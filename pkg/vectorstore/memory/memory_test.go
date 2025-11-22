package memory

import (
	"context"
	"fmt"
	"math"
	"sync"
	"testing"

	"github.com/aixgo-dev/aixgo/pkg/vectorstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNew tests creating a new memory vector store.
func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		config      vectorstore.Config
		wantErr     bool
		maxDocs     int
		defaultTopK int
	}{
		{
			name: "valid config with memory settings",
			config: vectorstore.Config{
				Provider:            "memory",
				EmbeddingDimensions: 384,
				DefaultTopK:         10,
				Memory: &vectorstore.MemoryConfig{
					MaxDocuments: 5000,
				},
			},
			wantErr:     false,
			maxDocs:     5000,
			defaultTopK: 10,
		},
		{
			name: "valid config without memory settings",
			config: vectorstore.Config{
				Provider:            "memory",
				EmbeddingDimensions: 768,
				DefaultTopK:         20,
			},
			wantErr:     false,
			maxDocs:     10000, // default
			defaultTopK: 20,
		},
		{
			name: "valid config with zero max documents (should use default)",
			config: vectorstore.Config{
				Provider:            "memory",
				EmbeddingDimensions: 384,
				DefaultTopK:         10,
				Memory: &vectorstore.MemoryConfig{
					MaxDocuments: 0,
				},
			},
			wantErr:     false,
			maxDocs:     10000, // default
			defaultTopK: 10,
		},
		{
			name: "invalid config with zero embedding dimensions",
			config: vectorstore.Config{
				Provider:            "memory",
				EmbeddingDimensions: 0,
				DefaultTopK:         10,
			},
			wantErr: true,
		},
		{
			name: "invalid config with negative embedding dimensions",
			config: vectorstore.Config{
				Provider:            "memory",
				EmbeddingDimensions: -1,
				DefaultTopK:         10,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := New(tt.config)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, store)

				ms := store.(*MemoryVectorStore)
				assert.Equal(t, tt.maxDocs, ms.maxDocuments)
				assert.Equal(t, tt.defaultTopK, ms.defaultTopK)
			}
		})
	}
}

// TestUpsert tests inserting and updating documents.
func TestUpsert(t *testing.T) {
	ctx := context.Background()
	store, _ := New(vectorstore.Config{
		Provider:            "memory",
		EmbeddingDimensions: 3,
		DefaultTopK:         10,
	})

	t.Run("insert new document", func(t *testing.T) {
		docs := []vectorstore.Document{
			{
				ID:        "doc1",
				Content:   "test content",
				Embedding: []float32{0.1, 0.2, 0.3},
				Metadata:  map[string]interface{}{"source": "test"},
			},
		}

		err := store.Upsert(ctx, docs)
		require.NoError(t, err)

		ms := store.(*MemoryVectorStore)
		assert.Equal(t, 1, ms.Count())
	})

	t.Run("update existing document", func(t *testing.T) {
		docs := []vectorstore.Document{
			{
				ID:        "doc1",
				Content:   "updated content",
				Embedding: []float32{0.4, 0.5, 0.6},
				Metadata:  map[string]interface{}{"source": "updated"},
			},
		}

		err := store.Upsert(ctx, docs)
		require.NoError(t, err)

		retrieved, err := store.Get(ctx, []string{"doc1"})
		require.NoError(t, err)
		assert.Equal(t, "updated content", retrieved[0].Content)
		assert.Equal(t, []float32{0.4, 0.5, 0.6}, retrieved[0].Embedding)
	})

	t.Run("empty documents slice", func(t *testing.T) {
		err := store.Upsert(ctx, []vectorstore.Document{})
		require.NoError(t, err)
	})

	t.Run("invalid document - empty ID", func(t *testing.T) {
		docs := []vectorstore.Document{
			{
				ID:        "",
				Content:   "test",
				Embedding: []float32{0.1, 0.2, 0.3},
			},
		}

		err := store.Upsert(ctx, docs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid document")
	})

	t.Run("invalid document - dimension mismatch", func(t *testing.T) {
		docs := []vectorstore.Document{
			{
				ID:        "doc2",
				Content:   "test",
				Embedding: []float32{0.1, 0.2}, // wrong dimension
			},
		}

		err := store.Upsert(ctx, docs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dimension mismatch")
	})

	t.Run("batch insert", func(t *testing.T) {
		ms := store.(*MemoryVectorStore)
		ms.Clear()

		docs := []vectorstore.Document{
			{ID: "batch1", Content: "content1", Embedding: []float32{0.1, 0.2, 0.3}},
			{ID: "batch2", Content: "content2", Embedding: []float32{0.4, 0.5, 0.6}},
			{ID: "batch3", Content: "content3", Embedding: []float32{0.7, 0.8, 0.9}},
		}

		err := store.Upsert(ctx, docs)
		require.NoError(t, err)
		assert.Equal(t, 3, ms.Count())
	})
}

// TestUpsertMaxDocuments tests the max documents limit.
func TestUpsertMaxDocuments(t *testing.T) {
	ctx := context.Background()
	store, _ := New(vectorstore.Config{
		Provider:            "memory",
		EmbeddingDimensions: 3,
		DefaultTopK:         10,
		Memory: &vectorstore.MemoryConfig{
			MaxDocuments: 5,
		},
	})

	// Insert 5 documents (at limit)
	docs := make([]vectorstore.Document, 5)
	for i := 0; i < 5; i++ {
		docs[i] = vectorstore.Document{
			ID:        fmt.Sprintf("doc%d", i),
			Content:   fmt.Sprintf("content%d", i),
			Embedding: []float32{float32(i), float32(i + 1), float32(i + 2)},
		}
	}

	err := store.Upsert(ctx, docs)
	require.NoError(t, err)

	// Try to insert one more (should fail)
	moreDocs := []vectorstore.Document{
		{
			ID:        "doc6",
			Content:   "content6",
			Embedding: []float32{0.1, 0.2, 0.3},
		},
	}

	err = store.Upsert(ctx, moreDocs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "would exceed max documents limit")

	// Update existing document (should succeed)
	updateDocs := []vectorstore.Document{
		{
			ID:        "doc1",
			Content:   "updated",
			Embedding: []float32{0.9, 0.8, 0.7},
		},
	}

	err = store.Upsert(ctx, updateDocs)
	require.NoError(t, err)
}

// TestSearch tests similarity search functionality.
func TestSearch(t *testing.T) {
	ctx := context.Background()
	store, _ := New(vectorstore.Config{
		Provider:              "memory",
		EmbeddingDimensions:   3,
		DefaultTopK:           10,
		DefaultDistanceMetric: "cosine",
	})

	// Insert test documents
	docs := []vectorstore.Document{
		{ID: "doc1", Content: "content1", Embedding: []float32{1.0, 0.0, 0.0}},
		{ID: "doc2", Content: "content2", Embedding: []float32{0.0, 1.0, 0.0}},
		{ID: "doc3", Content: "content3", Embedding: []float32{0.9, 0.1, 0.0}},
		{ID: "doc4", Content: "content4", Embedding: []float32{0.1, 0.9, 0.0}},
	}

	err := store.Upsert(ctx, docs)
	require.NoError(t, err)

	t.Run("basic search", func(t *testing.T) {
		query := vectorstore.SearchQuery{
			Embedding: []float32{1.0, 0.0, 0.0},
			TopK:      2,
		}

		results, err := store.Search(ctx, query)
		require.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Equal(t, "doc1", results[0].Document.ID) // Most similar
		assert.Equal(t, "doc3", results[1].Document.ID)
	})

	t.Run("search with min score", func(t *testing.T) {
		query := vectorstore.SearchQuery{
			Embedding: []float32{1.0, 0.0, 0.0},
			TopK:      10,
			MinScore:  0.8,
		}

		results, err := store.Search(ctx, query)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(results), 2)
		for _, result := range results {
			assert.GreaterOrEqual(t, result.Score, float32(0.8))
		}
	})

	t.Run("search with default TopK", func(t *testing.T) {
		query := vectorstore.SearchQuery{
			Embedding: []float32{1.0, 0.0, 0.0},
			TopK:      0, // Should use default
		}

		results, err := store.Search(ctx, query)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(results), 10) // default TopK
	})

	t.Run("search with invalid query", func(t *testing.T) {
		query := vectorstore.SearchQuery{
			Embedding: []float32{},
			TopK:      10,
		}

		_, err := store.Search(ctx, query)
		require.Error(t, err)
	})

	t.Run("search with dimension mismatch", func(t *testing.T) {
		query := vectorstore.SearchQuery{
			Embedding: []float32{1.0, 0.0}, // wrong dimension
			TopK:      10,
		}

		_, err := store.Search(ctx, query)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dimension mismatch")
	})

	t.Run("search returns sorted results", func(t *testing.T) {
		query := vectorstore.SearchQuery{
			Embedding: []float32{1.0, 0.0, 0.0},
			TopK:      10,
		}

		results, err := store.Search(ctx, query)
		require.NoError(t, err)

		// Verify results are sorted by score (descending)
		for i := 0; i < len(results)-1; i++ {
			assert.GreaterOrEqual(t, results[i].Score, results[i+1].Score)
		}
	})
}

// TestSearchWithMetadataFilter tests search with metadata filtering.
func TestSearchWithMetadataFilter(t *testing.T) {
	ctx := context.Background()
	store, _ := New(vectorstore.Config{
		Provider:            "memory",
		EmbeddingDimensions: 3,
		DefaultTopK:         10,
	})

	// Insert test documents with metadata
	docs := []vectorstore.Document{
		{
			ID:        "doc1",
			Content:   "content1",
			Embedding: []float32{1.0, 0.0, 0.0},
			Metadata:  map[string]interface{}{"source": "web", "status": "published"},
		},
		{
			ID:        "doc2",
			Content:   "content2",
			Embedding: []float32{0.9, 0.1, 0.0},
			Metadata:  map[string]interface{}{"source": "pdf", "status": "published"},
		},
		{
			ID:        "doc3",
			Content:   "content3",
			Embedding: []float32{0.8, 0.2, 0.0},
			Metadata:  map[string]interface{}{"source": "web", "status": "draft"},
		},
	}

	err := store.Upsert(ctx, docs)
	require.NoError(t, err)

	t.Run("filter with Must condition", func(t *testing.T) {
		query := vectorstore.SearchQuery{
			Embedding: []float32{1.0, 0.0, 0.0},
			TopK:      10,
			Filter: &vectorstore.MetadataFilter{
				Must: map[string]interface{}{
					"source": "web",
				},
			},
		}

		results, err := store.Search(ctx, query)
		require.NoError(t, err)
		assert.Len(t, results, 2)
		for _, result := range results {
			assert.Equal(t, "web", result.Document.Metadata["source"])
		}
	})

	t.Run("filter with MustNot condition", func(t *testing.T) {
		query := vectorstore.SearchQuery{
			Embedding: []float32{1.0, 0.0, 0.0},
			TopK:      10,
			Filter: &vectorstore.MetadataFilter{
				MustNot: map[string]interface{}{
					"status": "draft",
				},
			},
		}

		results, err := store.Search(ctx, query)
		require.NoError(t, err)
		assert.Len(t, results, 2)
		for _, result := range results {
			assert.NotEqual(t, "draft", result.Document.Metadata["status"])
		}
	})

	t.Run("filter with Should condition", func(t *testing.T) {
		query := vectorstore.SearchQuery{
			Embedding: []float32{1.0, 0.0, 0.0},
			TopK:      10,
			Filter: &vectorstore.MetadataFilter{
				Should: map[string]interface{}{
					"source": "pdf",
				},
			},
		}

		results, err := store.Search(ctx, query)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "doc2", results[0].Document.ID)
	})

	t.Run("filter with combined conditions", func(t *testing.T) {
		query := vectorstore.SearchQuery{
			Embedding: []float32{1.0, 0.0, 0.0},
			TopK:      10,
			Filter: &vectorstore.MetadataFilter{
				Must: map[string]interface{}{
					"source": "web",
				},
				MustNot: map[string]interface{}{
					"status": "draft",
				},
			},
		}

		results, err := store.Search(ctx, query)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "doc1", results[0].Document.ID)
	})
}

// TestSearchDistanceMetrics tests different distance metrics.
func TestSearchDistanceMetrics(t *testing.T) {
	ctx := context.Background()
	store, _ := New(vectorstore.Config{
		Provider:            "memory",
		EmbeddingDimensions: 3,
		DefaultTopK:         10,
	})

	docs := []vectorstore.Document{
		{ID: "doc1", Content: "content1", Embedding: []float32{1.0, 0.0, 0.0}},
		{ID: "doc2", Content: "content2", Embedding: []float32{0.0, 1.0, 0.0}},
	}

	err := store.Upsert(ctx, docs)
	require.NoError(t, err)

	t.Run("cosine similarity", func(t *testing.T) {
		query := vectorstore.SearchQuery{
			Embedding:      []float32{1.0, 0.0, 0.0},
			TopK:           10,
			DistanceMetric: vectorstore.DistanceMetricCosine,
		}

		results, err := store.Search(ctx, query)
		require.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Equal(t, float32(1.0), results[0].Score) // Perfect match
	})

	t.Run("dot product", func(t *testing.T) {
		query := vectorstore.SearchQuery{
			Embedding:      []float32{1.0, 0.0, 0.0},
			TopK:           10,
			DistanceMetric: vectorstore.DistanceMetricDotProduct,
		}

		results, err := store.Search(ctx, query)
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("euclidean distance", func(t *testing.T) {
		query := vectorstore.SearchQuery{
			Embedding:      []float32{1.0, 0.0, 0.0},
			TopK:           10,
			DistanceMetric: vectorstore.DistanceMetricEuclidean,
		}

		results, err := store.Search(ctx, query)
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})
}

// TestDelete tests deleting documents.
func TestDelete(t *testing.T) {
	ctx := context.Background()
	store, _ := New(vectorstore.Config{
		Provider:            "memory",
		EmbeddingDimensions: 3,
		DefaultTopK:         10,
	})

	docs := []vectorstore.Document{
		{ID: "doc1", Content: "content1", Embedding: []float32{0.1, 0.2, 0.3}},
		{ID: "doc2", Content: "content2", Embedding: []float32{0.4, 0.5, 0.6}},
		{ID: "doc3", Content: "content3", Embedding: []float32{0.7, 0.8, 0.9}},
	}

	err := store.Upsert(ctx, docs)
	require.NoError(t, err)

	t.Run("delete single document", func(t *testing.T) {
		err := store.Delete(ctx, []string{"doc1"})
		require.NoError(t, err)

		ms := store.(*MemoryVectorStore)
		assert.Equal(t, 2, ms.Count())

		retrieved, err := store.Get(ctx, []string{"doc1"})
		require.NoError(t, err)
		assert.Empty(t, retrieved)
	})

	t.Run("delete multiple documents", func(t *testing.T) {
		err := store.Delete(ctx, []string{"doc2", "doc3"})
		require.NoError(t, err)

		ms := store.(*MemoryVectorStore)
		assert.Equal(t, 0, ms.Count())
	})

	t.Run("delete non-existent document", func(t *testing.T) {
		err := store.Delete(ctx, []string{"nonexistent"})
		require.NoError(t, err) // Should not error
	})

	t.Run("delete empty slice", func(t *testing.T) {
		err := store.Delete(ctx, []string{})
		require.NoError(t, err)
	})
}

// TestGet tests retrieving documents by ID.
func TestGet(t *testing.T) {
	ctx := context.Background()
	store, _ := New(vectorstore.Config{
		Provider:            "memory",
		EmbeddingDimensions: 3,
		DefaultTopK:         10,
	})

	docs := []vectorstore.Document{
		{ID: "doc1", Content: "content1", Embedding: []float32{0.1, 0.2, 0.3}},
		{ID: "doc2", Content: "content2", Embedding: []float32{0.4, 0.5, 0.6}},
	}

	err := store.Upsert(ctx, docs)
	require.NoError(t, err)

	t.Run("get single document", func(t *testing.T) {
		retrieved, err := store.Get(ctx, []string{"doc1"})
		require.NoError(t, err)
		assert.Len(t, retrieved, 1)
		assert.Equal(t, "doc1", retrieved[0].ID)
		assert.Equal(t, "content1", retrieved[0].Content)
	})

	t.Run("get multiple documents", func(t *testing.T) {
		retrieved, err := store.Get(ctx, []string{"doc1", "doc2"})
		require.NoError(t, err)
		assert.Len(t, retrieved, 2)
	})

	t.Run("get non-existent document", func(t *testing.T) {
		retrieved, err := store.Get(ctx, []string{"nonexistent"})
		require.NoError(t, err)
		assert.Empty(t, retrieved)
	})

	t.Run("get mixed existent and non-existent", func(t *testing.T) {
		retrieved, err := store.Get(ctx, []string{"doc1", "nonexistent", "doc2"})
		require.NoError(t, err)
		assert.Len(t, retrieved, 2)
	})

	t.Run("get empty slice", func(t *testing.T) {
		retrieved, err := store.Get(ctx, []string{})
		require.NoError(t, err)
		assert.Empty(t, retrieved)
	})

	t.Run("get returns copy not reference", func(t *testing.T) {
		retrieved, err := store.Get(ctx, []string{"doc1"})
		require.NoError(t, err)
		require.Len(t, retrieved, 1)

		// Modify retrieved document
		retrieved[0].Content = "modified"

		// Get again and verify original is unchanged
		retrieved2, err := store.Get(ctx, []string{"doc1"})
		require.NoError(t, err)
		assert.Equal(t, "content1", retrieved2[0].Content)
	})
}

// TestClose tests closing the store.
func TestClose(t *testing.T) {
	store, _ := New(vectorstore.Config{
		Provider:            "memory",
		EmbeddingDimensions: 3,
		DefaultTopK:         10,
	})

	err := store.Close()
	require.NoError(t, err)
}

// TestConcurrency tests concurrent operations.
func TestConcurrency(t *testing.T) {
	ctx := context.Background()
	store, _ := New(vectorstore.Config{
		Provider:            "memory",
		EmbeddingDimensions: 3,
		DefaultTopK:         10,
		Memory: &vectorstore.MemoryConfig{
			MaxDocuments: 1000,
		},
	})

	t.Run("concurrent upserts", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 10
		docsPerGoroutine := 10

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(routineID int) {
				defer wg.Done()

				docs := make([]vectorstore.Document, docsPerGoroutine)
				for j := 0; j < docsPerGoroutine; j++ {
					docID := fmt.Sprintf("doc_%d_%d", routineID, j)
					docs[j] = vectorstore.Document{
						ID:        docID,
						Content:   fmt.Sprintf("content_%d_%d", routineID, j),
						Embedding: []float32{float32(routineID), float32(j), 0.5},
					}
				}

				err := store.Upsert(ctx, docs)
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()

		ms := store.(*MemoryVectorStore)
		assert.Equal(t, numGoroutines*docsPerGoroutine, ms.Count())
	})

	t.Run("concurrent searches", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 20

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(routineID int) {
				defer wg.Done()

				query := vectorstore.SearchQuery{
					Embedding: []float32{float32(routineID % 5), 0.5, 0.5},
					TopK:      5,
				}

				results, err := store.Search(ctx, query)
				assert.NoError(t, err)
				assert.NotNil(t, results)
			}(i)
		}

		wg.Wait()
	})

	t.Run("concurrent reads and writes", func(t *testing.T) {
		ms := store.(*MemoryVectorStore)
		ms.Clear()

		// Seed with some documents
		seedDocs := make([]vectorstore.Document, 50)
		for i := 0; i < 50; i++ {
			seedDocs[i] = vectorstore.Document{
				ID:        fmt.Sprintf("seed_%d", i),
				Content:   fmt.Sprintf("content_%d", i),
				Embedding: []float32{float32(i), 0.5, 0.5},
			}
		}
		_ = store.Upsert(ctx, seedDocs)

		var wg sync.WaitGroup
		numOperations := 100

		// Concurrent writers
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				doc := vectorstore.Document{
					ID:        fmt.Sprintf("write_%d", id),
					Content:   fmt.Sprintf("content_%d", id),
					Embedding: []float32{float32(id), 0.5, 0.5},
				}
				_ = store.Upsert(ctx, []vectorstore.Document{doc})
			}(i)
		}

		// Concurrent readers
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				query := vectorstore.SearchQuery{
					Embedding: []float32{float32(id % 10), 0.5, 0.5},
					TopK:      5,
				}
				_, _ = store.Search(ctx, query)
			}(i)
		}

		// Concurrent getters
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				_, _ = store.Get(ctx, []string{fmt.Sprintf("seed_%d", id%50)})
			}(i)
		}

		wg.Wait()
	})
}

// TestHelperFunctions tests the similarity calculation functions.
func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		vec1     []float32
		vec2     []float32
		expected float32
	}{
		{
			name:     "identical vectors",
			vec1:     []float32{1.0, 0.0, 0.0},
			vec2:     []float32{1.0, 0.0, 0.0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			vec1:     []float32{1.0, 0.0, 0.0},
			vec2:     []float32{0.0, 1.0, 0.0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			vec1:     []float32{1.0, 0.0, 0.0},
			vec2:     []float32{-1.0, 0.0, 0.0},
			expected: -1.0,
		},
		{
			name:     "different lengths",
			vec1:     []float32{1.0, 0.0},
			vec2:     []float32{1.0, 0.0, 0.0},
			expected: 0.0,
		},
		{
			name:     "zero vector",
			vec1:     []float32{0.0, 0.0, 0.0},
			vec2:     []float32{1.0, 1.0, 1.0},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.vec1, tt.vec2)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

func TestDotProduct(t *testing.T) {
	tests := []struct {
		name     string
		vec1     []float32
		vec2     []float32
		expected float32
	}{
		{
			name:     "simple dot product",
			vec1:     []float32{1.0, 2.0, 3.0},
			vec2:     []float32{4.0, 5.0, 6.0},
			expected: 32.0, // 1*4 + 2*5 + 3*6 = 32
		},
		{
			name:     "zero result",
			vec1:     []float32{1.0, 0.0, 0.0},
			vec2:     []float32{0.0, 1.0, 0.0},
			expected: 0.0,
		},
		{
			name:     "different lengths",
			vec1:     []float32{1.0, 2.0},
			vec2:     []float32{1.0, 2.0, 3.0},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dotProduct(tt.vec1, tt.vec2)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

func TestEuclideanDistance(t *testing.T) {
	tests := []struct {
		name     string
		vec1     []float32
		vec2     []float32
		expected float32
	}{
		{
			name:     "identical vectors",
			vec1:     []float32{1.0, 2.0, 3.0},
			vec2:     []float32{1.0, 2.0, 3.0},
			expected: 0.0,
		},
		{
			name:     "simple distance",
			vec1:     []float32{0.0, 0.0, 0.0},
			vec2:     []float32{3.0, 4.0, 0.0},
			expected: 5.0, // sqrt(9 + 16) = 5
		},
		{
			name:     "different lengths",
			vec1:     []float32{1.0, 2.0},
			vec2:     []float32{1.0, 2.0, 3.0},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := euclideanDistance(tt.vec1, tt.vec2)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

func TestSqrt(t *testing.T) {
	tests := []struct {
		name     string
		input    float32
		expected float32
	}{
		{"zero", 0, 0},
		{"one", 1, 1},
		{"four", 4, 2},
		{"nine", 9, 3},
		{"two", 2, float32(math.Sqrt(2))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sqrt(tt.input)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

// BenchmarkUpsert benchmarks document insertion.
func BenchmarkUpsert(b *testing.B) {
	ctx := context.Background()
	store, _ := New(vectorstore.Config{
		Provider:            "memory",
		EmbeddingDimensions: 768,
		DefaultTopK:         10,
		Memory: &vectorstore.MemoryConfig{
			MaxDocuments: 100000,
		},
	})

	doc := vectorstore.Document{
		ID:        "bench_doc",
		Content:   "benchmark content",
		Embedding: make([]float32, 768),
		Metadata:  map[string]interface{}{"source": "benchmark"},
	}

	for i := range doc.Embedding {
		doc.Embedding[i] = float32(i) * 0.001
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc.ID = fmt.Sprintf("bench_doc_%d", i)
		_ = store.Upsert(ctx, []vectorstore.Document{doc})
	}
}

// BenchmarkSearch benchmarks similarity search.
func BenchmarkSearch(b *testing.B) {
	ctx := context.Background()
	store, _ := New(vectorstore.Config{
		Provider:            "memory",
		EmbeddingDimensions: 768,
		DefaultTopK:         10,
	})

	// Seed with documents
	numDocs := 1000
	docs := make([]vectorstore.Document, numDocs)
	for i := 0; i < numDocs; i++ {
		embedding := make([]float32, 768)
		for j := range embedding {
			embedding[j] = float32(i*j) * 0.001
		}
		docs[i] = vectorstore.Document{
			ID:        fmt.Sprintf("doc_%d", i),
			Content:   fmt.Sprintf("content_%d", i),
			Embedding: embedding,
		}
	}
	_ = store.Upsert(ctx, docs)

	queryEmbedding := make([]float32, 768)
	for i := range queryEmbedding {
		queryEmbedding[i] = float32(i) * 0.001
	}

	query := vectorstore.SearchQuery{
		Embedding: queryEmbedding,
		TopK:      10,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Search(ctx, query)
	}
}

// BenchmarkCosineSimilarity benchmarks cosine similarity calculation.
func BenchmarkCosineSimilarity(b *testing.B) {
	vec1 := make([]float32, 768)
	vec2 := make([]float32, 768)
	for i := range vec1 {
		vec1[i] = float32(i) * 0.001
		vec2[i] = float32(i+1) * 0.001
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cosineSimilarity(vec1, vec2)
	}
}
