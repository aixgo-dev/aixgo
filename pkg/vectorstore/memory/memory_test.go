package memory

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aixgo-dev/aixgo/pkg/vectorstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper functions

func createTestDoc(id string, content string, vector []float32) *vectorstore.Document {
	return &vectorstore.Document{
		ID:        id,
		Content:   vectorstore.NewTextContent(content),
		Embedding: vectorstore.NewEmbedding(vector, "test-model"),
	}
}

func createTestDocWithTags(id string, content string, vector []float32, tags []string) *vectorstore.Document {
	doc := createTestDoc(id, content, vector)
	doc.Tags = tags
	return doc
}

func createTestDocWithScope(id string, content string, vector []float32, tenant, user, session string) *vectorstore.Document {
	doc := createTestDoc(id, content, vector)
	doc.Scope = &vectorstore.Scope{
		Tenant:  tenant,
		User:    user,
		Session: session,
	}
	return doc
}

// Tests

func TestNew(t *testing.T) {
	store, err := New()
	require.NoError(t, err)
	require.NotNil(t, store)

	err = store.Close()
	require.NoError(t, err)
}

func TestCollectionIsolation(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()

	// Create two collections
	coll1 := store.Collection("collection1")
	coll2 := store.Collection("collection2")

	require.Equal(t, "collection1", coll1.Name())
	require.Equal(t, "collection2", coll2.Name())

	// Insert into collection1
	doc1 := createTestDoc("doc1", "content1", []float32{1, 0, 0})
	result, err := coll1.Upsert(ctx, doc1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Inserted)

	// Insert into collection2
	doc2 := createTestDoc("doc2", "content2", []float32{0, 1, 0})
	result, err = coll2.Upsert(ctx, doc2)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Inserted)

	// Verify isolation
	count1, err := coll1.Count(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count1)

	count2, err := coll2.Count(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count2)

	// Get from coll1 should not return doc2
	docs, err := coll1.Get(ctx, "doc2")
	require.NoError(t, err)
	assert.Empty(t, docs)
}

func TestListCollections(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()

	// Create collections
	store.Collection("alpha")
	store.Collection("beta")
	store.Collection("gamma")

	names, err := store.ListCollections(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"alpha", "beta", "gamma"}, names)
}

func TestDeleteCollection(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()

	coll := store.Collection("test")
	doc := createTestDoc("doc1", "content", []float32{1, 0, 0})
	_, err := coll.Upsert(ctx, doc)
	require.NoError(t, err)

	// Delete collection
	err = store.DeleteCollection(ctx, "test")
	require.NoError(t, err)

	// Verify collection is gone
	names, err := store.ListCollections(ctx)
	require.NoError(t, err)
	assert.NotContains(t, names, "test")
}

func TestUpsert(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()
	coll := store.Collection("test")

	t.Run("insert new document", func(t *testing.T) {
		doc := createTestDoc("doc1", "test content", []float32{0.1, 0.2, 0.3})

		result, err := coll.Upsert(ctx, doc)
		require.NoError(t, err)
		assert.Equal(t, int64(1), result.Inserted)
		assert.Equal(t, int64(0), result.Updated)
	})

	t.Run("update existing document", func(t *testing.T) {
		doc := createTestDoc("doc1", "updated content", []float32{0.4, 0.5, 0.6})

		result, err := coll.Upsert(ctx, doc)
		require.NoError(t, err)
		assert.Equal(t, int64(0), result.Inserted)
		assert.Equal(t, int64(1), result.Updated)

		// Verify update
		docs, err := coll.Get(ctx, "doc1")
		require.NoError(t, err)
		require.Len(t, docs, 1)
		assert.Equal(t, "updated content", docs[0].Content.Text)
	})

	t.Run("batch insert", func(t *testing.T) {
		coll2 := store.Collection("test2")
		docs := []*vectorstore.Document{
			createTestDoc("batch1", "content1", []float32{0.1, 0.2, 0.3}),
			createTestDoc("batch2", "content2", []float32{0.4, 0.5, 0.6}),
			createTestDoc("batch3", "content3", []float32{0.7, 0.8, 0.9}),
		}

		result, err := coll2.Upsert(ctx, docs...)
		require.NoError(t, err)
		assert.Equal(t, int64(3), result.Inserted)
	})

	t.Run("invalid document", func(t *testing.T) {
		coll3 := store.Collection("test3")
		doc := &vectorstore.Document{
			ID:        "", // Invalid: empty ID
			Content:   vectorstore.NewTextContent("test"),
			Embedding: vectorstore.NewEmbedding([]float32{1, 2, 3}, "test"),
		}

		_, err := coll3.Upsert(ctx, doc)
		require.Error(t, err)
	})
}

func TestUpsertWithTTL(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()

	// Create collection with TTL
	coll := store.Collection("test-ttl", vectorstore.WithTTL(100*time.Millisecond))

	doc := createTestDoc("doc1", "content", []float32{1, 0, 0})
	_, err := coll.Upsert(ctx, doc)
	require.NoError(t, err)

	// Document should exist immediately
	count, err := coll.Count(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Wait for expiration + cleanup cycle
	time.Sleep(150 * time.Millisecond)

	// Manually trigger cleanup
	memColl := coll.(*MemoryCollection)
	memColl.cleanupExpired()

	// Document should be expired
	count, err = coll.Count(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestDeduplication(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()

	// Create collection with deduplication enabled
	coll := store.Collection("test-dedup", vectorstore.WithDeduplication(true))

	// Insert first document
	doc1 := createTestDoc("doc1", "content", []float32{1, 0, 0})
	result, err := coll.Upsert(ctx, doc1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Inserted)

	// Try to insert duplicate (same content and embedding)
	doc2 := createTestDoc("doc2", "content", []float32{1, 0, 0})
	result, err = coll.Upsert(ctx, doc2)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Inserted)
	assert.Equal(t, int64(1), result.Deduplicated)
	assert.Contains(t, result.DeduplicatedIDs, "doc2")

	// Verify only one document exists
	count, err := coll.Count(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestUpsertBatch(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()
	coll := store.Collection("test")

	// Create many documents
	docs := make([]*vectorstore.Document, 100)
	for i := 0; i < 100; i++ {
		docs[i] = createTestDoc(
			fmt.Sprintf("doc%d", i),
			fmt.Sprintf("content%d", i),
			[]float32{float32(i) * 0.01, 0, 0},
		)
	}

	progressCalled := false
	result, err := coll.UpsertBatch(ctx, docs,
		vectorstore.WithBatchSize(20),
		vectorstore.WithProgressCallback(func(processed, total int) {
			progressCalled = true
			assert.LessOrEqual(t, processed, total)
		}),
	)

	require.NoError(t, err)
	assert.Equal(t, int64(100), result.Inserted)
	assert.True(t, progressCalled)

	// Verify all documents were inserted
	count, err := coll.Count(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(100), count)
}

func TestQuery(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()
	coll := store.Collection("test")

	// Insert test documents
	docs := []*vectorstore.Document{
		createTestDoc("doc1", "content1", []float32{1.0, 0.0, 0.0}),
		createTestDoc("doc2", "content2", []float32{0.0, 1.0, 0.0}),
		createTestDoc("doc3", "content3", []float32{0.9, 0.1, 0.0}),
		createTestDoc("doc4", "content4", []float32{0.1, 0.9, 0.0}),
	}
	_, err := coll.Upsert(ctx, docs...)
	require.NoError(t, err)

	t.Run("basic similarity search", func(t *testing.T) {
		query := &vectorstore.Query{
			Embedding: vectorstore.NewEmbedding([]float32{1.0, 0.0, 0.0}, "test"),
			Limit:     2,
		}

		result, err := coll.Query(ctx, query)
		require.NoError(t, err)
		assert.Len(t, result.Matches, 2)
		assert.Equal(t, "doc1", result.Matches[0].Document.ID) // Most similar
		assert.Equal(t, "doc3", result.Matches[1].Document.ID)
	})

	t.Run("query with min score", func(t *testing.T) {
		query := &vectorstore.Query{
			Embedding: vectorstore.NewEmbedding([]float32{1.0, 0.0, 0.0}, "test"),
			Limit:     10,
			MinScore:  0.8,
		}

		result, err := coll.Query(ctx, query)
		require.NoError(t, err)
		for _, match := range result.Matches {
			assert.GreaterOrEqual(t, match.Score, float32(0.8))
		}
	})

	t.Run("query with offset and limit", func(t *testing.T) {
		query := &vectorstore.Query{
			Embedding: vectorstore.NewEmbedding([]float32{1.0, 0.0, 0.0}, "test"),
			Limit:     2,
			Offset:    1,
		}

		result, err := coll.Query(ctx, query)
		require.NoError(t, err)
		assert.Len(t, result.Matches, 2)
		assert.Equal(t, 1, result.Offset)
		assert.True(t, result.Total >= 2)
	})
}

func TestQueryWithFilters(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()
	coll := store.Collection("test")

	// Insert documents with metadata
	docs := []*vectorstore.Document{
		createTestDoc("doc1", "content1", []float32{1.0, 0.0, 0.0}),
		createTestDoc("doc2", "content2", []float32{0.9, 0.1, 0.0}),
		createTestDoc("doc3", "content3", []float32{0.8, 0.2, 0.0}),
	}
	docs[0].Metadata = map[string]any{"category": "A", "score": 10}
	docs[1].Metadata = map[string]any{"category": "B", "score": 20}
	docs[2].Metadata = map[string]any{"category": "A", "score": 30}

	_, err := coll.Upsert(ctx, docs...)
	require.NoError(t, err)

	t.Run("field filter", func(t *testing.T) {
		query := &vectorstore.Query{
			Embedding: vectorstore.NewEmbedding([]float32{1.0, 0.0, 0.0}, "test"),
			Limit:     10,
			Filters:   vectorstore.Eq("category", "A"),
		}

		result, err := coll.Query(ctx, query)
		require.NoError(t, err)
		assert.Len(t, result.Matches, 2)
		for _, match := range result.Matches {
			assert.Equal(t, "A", match.Document.Metadata["category"])
		}
	})

	t.Run("numeric comparison filter", func(t *testing.T) {
		query := &vectorstore.Query{
			Embedding: vectorstore.NewEmbedding([]float32{1.0, 0.0, 0.0}, "test"),
			Limit:     10,
			Filters:   vectorstore.Gt("score", 15),
		}

		result, err := coll.Query(ctx, query)
		require.NoError(t, err)
		assert.Len(t, result.Matches, 2)
		for _, match := range result.Matches {
			score, _ := match.Document.Metadata["score"].(int)
			assert.Greater(t, score, 15)
		}
	})

	t.Run("AND filter", func(t *testing.T) {
		query := &vectorstore.Query{
			Embedding: vectorstore.NewEmbedding([]float32{1.0, 0.0, 0.0}, "test"),
			Limit:     10,
			Filters: vectorstore.And(
				vectorstore.Eq("category", "A"),
				vectorstore.Gt("score", 15),
			),
		}

		result, err := coll.Query(ctx, query)
		require.NoError(t, err)
		assert.Len(t, result.Matches, 1)
		assert.Equal(t, "doc3", result.Matches[0].Document.ID)
	})

	t.Run("OR filter", func(t *testing.T) {
		query := &vectorstore.Query{
			Embedding: vectorstore.NewEmbedding([]float32{1.0, 0.0, 0.0}, "test"),
			Limit:     10,
			Filters: vectorstore.Or(
				vectorstore.Eq("category", "B"),
				vectorstore.Gt("score", 25),
			),
		}

		result, err := coll.Query(ctx, query)
		require.NoError(t, err)
		assert.Len(t, result.Matches, 2) // doc2 and doc3
	})

	t.Run("NOT filter", func(t *testing.T) {
		query := &vectorstore.Query{
			Embedding: vectorstore.NewEmbedding([]float32{1.0, 0.0, 0.0}, "test"),
			Limit:     10,
			Filters:   vectorstore.Not(vectorstore.Eq("category", "A")),
		}

		result, err := coll.Query(ctx, query)
		require.NoError(t, err)
		assert.Len(t, result.Matches, 1)
		assert.Equal(t, "doc2", result.Matches[0].Document.ID)
	})
}

func TestTagFilters(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()
	coll := store.Collection("test")

	// Insert documents with tags
	docs := []*vectorstore.Document{
		createTestDocWithTags("doc1", "content1", []float32{1, 0, 0}, []string{"tag1", "tag2"}),
		createTestDocWithTags("doc2", "content2", []float32{0.9, 0.1, 0}, []string{"tag2", "tag3"}),
		createTestDocWithTags("doc3", "content3", []float32{0.8, 0.2, 0}, []string{"tag3"}),
	}
	_, err := coll.Upsert(ctx, docs...)
	require.NoError(t, err)

	t.Run("single tag filter", func(t *testing.T) {
		query := &vectorstore.Query{
			Embedding: vectorstore.NewEmbedding([]float32{1.0, 0.0, 0.0}, "test"),
			Limit:     10,
			Filters:   vectorstore.TagFilter("tag2"),
		}

		result, err := coll.Query(ctx, query)
		require.NoError(t, err)
		assert.Len(t, result.Matches, 2) // doc1 and doc2
	})

	t.Run("multiple tags (all required)", func(t *testing.T) {
		query := &vectorstore.Query{
			Embedding: vectorstore.NewEmbedding([]float32{1.0, 0.0, 0.0}, "test"),
			Limit:     10,
			Filters:   vectorstore.TagsFilter("tag2", "tag3"),
		}

		result, err := coll.Query(ctx, query)
		require.NoError(t, err)
		assert.Len(t, result.Matches, 1) // only doc2 has both
		assert.Equal(t, "doc2", result.Matches[0].Document.ID)
	})

	t.Run("any tag filter", func(t *testing.T) {
		query := &vectorstore.Query{
			Embedding: vectorstore.NewEmbedding([]float32{1.0, 0.0, 0.0}, "test"),
			Limit:     10,
			Filters:   vectorstore.AnyTagFilter("tag1", "tag3"),
		}

		result, err := coll.Query(ctx, query)
		require.NoError(t, err)
		assert.Len(t, result.Matches, 3) // all docs have at least one
	})
}

func TestScopeFilters(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()
	coll := store.Collection("test")

	// Insert documents with scope
	docs := []*vectorstore.Document{
		createTestDocWithScope("doc1", "content1", []float32{1, 0, 0}, "tenant1", "user1", "session1"),
		createTestDocWithScope("doc2", "content2", []float32{0.9, 0.1, 0}, "tenant1", "user2", "session2"),
		createTestDocWithScope("doc3", "content3", []float32{0.8, 0.2, 0}, "tenant2", "user1", "session3"),
	}
	_, err := coll.Upsert(ctx, docs...)
	require.NoError(t, err)

	t.Run("tenant filter", func(t *testing.T) {
		query := &vectorstore.Query{
			Embedding: vectorstore.NewEmbedding([]float32{1.0, 0.0, 0.0}, "test"),
			Limit:     10,
			Filters:   vectorstore.TenantFilter("tenant1"),
		}

		result, err := coll.Query(ctx, query)
		require.NoError(t, err)
		assert.Len(t, result.Matches, 2)
	})

	t.Run("user filter", func(t *testing.T) {
		query := &vectorstore.Query{
			Embedding: vectorstore.NewEmbedding([]float32{1.0, 0.0, 0.0}, "test"),
			Limit:     10,
			Filters:   vectorstore.UserFilter("user1"),
		}

		result, err := coll.Query(ctx, query)
		require.NoError(t, err)
		assert.Len(t, result.Matches, 2)
	})

	t.Run("combined scope filter", func(t *testing.T) {
		query := &vectorstore.Query{
			Embedding: vectorstore.NewEmbedding([]float32{1.0, 0.0, 0.0}, "test"),
			Limit:     10,
			Filters: vectorstore.ScopeFilter(&vectorstore.Scope{
				Tenant: "tenant1",
				User:   "user1",
			}),
		}

		result, err := coll.Query(ctx, query)
		require.NoError(t, err)
		assert.Len(t, result.Matches, 1)
		assert.Equal(t, "doc1", result.Matches[0].Document.ID)
	})
}

func TestTimeFilters(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()
	coll := store.Collection("test")

	// Insert documents with temporal information
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	docs := []*vectorstore.Document{
		createTestDoc("doc1", "content1", []float32{1, 0, 0}),
		createTestDoc("doc2", "content2", []float32{0.9, 0.1, 0}),
		createTestDoc("doc3", "content3", []float32{0.8, 0.2, 0}),
	}
	docs[0].Temporal = &vectorstore.Temporal{CreatedAt: past, UpdatedAt: past}
	docs[1].Temporal = &vectorstore.Temporal{CreatedAt: now, UpdatedAt: now}
	docs[2].Temporal = &vectorstore.Temporal{CreatedAt: future, UpdatedAt: future}

	_, err := coll.Upsert(ctx, docs...)
	require.NoError(t, err)

	t.Run("created after filter", func(t *testing.T) {
		query := &vectorstore.Query{
			Embedding: vectorstore.NewEmbedding([]float32{1.0, 0.0, 0.0}, "test"),
			Limit:     10,
			Filters:   vectorstore.CreatedAfter(now.Add(-30 * time.Minute)),
		}

		result, err := coll.Query(ctx, query)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(result.Matches), 1)
	})

	t.Run("created before filter", func(t *testing.T) {
		query := &vectorstore.Query{
			Embedding: vectorstore.NewEmbedding([]float32{1.0, 0.0, 0.0}, "test"),
			Limit:     10,
			Filters:   vectorstore.CreatedBefore(now.Add(30 * time.Minute)),
		}

		result, err := coll.Query(ctx, query)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(result.Matches), 1)
	})
}

func TestQueryStream(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()
	coll := store.Collection("test")

	// Insert test documents
	docs := []*vectorstore.Document{
		createTestDoc("doc1", "content1", []float32{1.0, 0.0, 0.0}),
		createTestDoc("doc2", "content2", []float32{0.9, 0.1, 0.0}),
		createTestDoc("doc3", "content3", []float32{0.8, 0.2, 0.0}),
	}
	_, err := coll.Upsert(ctx, docs...)
	require.NoError(t, err)

	query := &vectorstore.Query{
		Embedding: vectorstore.NewEmbedding([]float32{1.0, 0.0, 0.0}, "test"),
		Limit:     10,
	}

	iter, err := coll.QueryStream(ctx, query)
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	count := 0
	for iter.Next() {
		match := iter.Match()
		require.NotNil(t, match)
		require.NotNil(t, match.Document)
		count++
	}

	require.NoError(t, iter.Err())
	assert.Equal(t, 3, count)
}

func TestGet(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()
	coll := store.Collection("test")

	// Insert documents
	docs := []*vectorstore.Document{
		createTestDoc("doc1", "content1", []float32{1, 0, 0}),
		createTestDoc("doc2", "content2", []float32{0, 1, 0}),
	}
	_, err := coll.Upsert(ctx, docs...)
	require.NoError(t, err)

	t.Run("get single document", func(t *testing.T) {
		retrieved, err := coll.Get(ctx, "doc1")
		require.NoError(t, err)
		assert.Len(t, retrieved, 1)
		assert.Equal(t, "doc1", retrieved[0].ID)
		assert.Equal(t, "content1", retrieved[0].Content.Text)
	})

	t.Run("get multiple documents", func(t *testing.T) {
		retrieved, err := coll.Get(ctx, "doc1", "doc2")
		require.NoError(t, err)
		assert.Len(t, retrieved, 2)
	})

	t.Run("get non-existent document", func(t *testing.T) {
		retrieved, err := coll.Get(ctx, "nonexistent")
		require.NoError(t, err)
		assert.Empty(t, retrieved)
	})

	t.Run("get returns copy", func(t *testing.T) {
		retrieved, err := coll.Get(ctx, "doc1")
		require.NoError(t, err)
		require.Len(t, retrieved, 1)

		// Modify retrieved document
		retrieved[0].Content.Text = "modified"

		// Get again and verify original is unchanged
		retrieved2, err := coll.Get(ctx, "doc1")
		require.NoError(t, err)
		assert.Equal(t, "content1", retrieved2[0].Content.Text)
	})
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()
	coll := store.Collection("test")

	// Insert documents
	docs := []*vectorstore.Document{
		createTestDoc("doc1", "content1", []float32{1, 0, 0}),
		createTestDoc("doc2", "content2", []float32{0, 1, 0}),
		createTestDoc("doc3", "content3", []float32{0, 0, 1}),
	}
	_, err := coll.Upsert(ctx, docs...)
	require.NoError(t, err)

	t.Run("delete single document", func(t *testing.T) {
		result, err := coll.Delete(ctx, "doc1")
		require.NoError(t, err)
		assert.Equal(t, int64(1), result.Deleted)

		count, err := coll.Count(ctx, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})

	t.Run("delete multiple documents", func(t *testing.T) {
		result, err := coll.Delete(ctx, "doc2", "doc3")
		require.NoError(t, err)
		assert.Equal(t, int64(2), result.Deleted)

		count, err := coll.Count(ctx, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("delete non-existent document", func(t *testing.T) {
		result, err := coll.Delete(ctx, "nonexistent")
		require.NoError(t, err)
		assert.Equal(t, int64(0), result.Deleted)
		assert.Equal(t, int64(1), result.NotFound)
	})
}

func TestDeleteByFilter(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()
	coll := store.Collection("test")

	// Insert documents with metadata
	docs := []*vectorstore.Document{
		createTestDoc("doc1", "content1", []float32{1, 0, 0}),
		createTestDoc("doc2", "content2", []float32{0, 1, 0}),
		createTestDoc("doc3", "content3", []float32{0, 0, 1}),
	}
	docs[0].Metadata = map[string]any{"status": "draft"}
	docs[1].Metadata = map[string]any{"status": "published"}
	docs[2].Metadata = map[string]any{"status": "draft"}

	_, err := coll.Upsert(ctx, docs...)
	require.NoError(t, err)

	// Delete all drafts
	result, err := coll.DeleteByFilter(ctx, vectorstore.Eq("status", "draft"))
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.Deleted)

	// Verify only published remains
	count, err := coll.Count(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestCount(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()
	coll := store.Collection("test")

	// Insert documents
	docs := []*vectorstore.Document{
		createTestDoc("doc1", "content1", []float32{1, 0, 0}),
		createTestDoc("doc2", "content2", []float32{0, 1, 0}),
		createTestDoc("doc3", "content3", []float32{0, 0, 1}),
	}
	docs[0].Metadata = map[string]any{"type": "A"}
	docs[1].Metadata = map[string]any{"type": "B"}
	docs[2].Metadata = map[string]any{"type": "A"}

	_, err := coll.Upsert(ctx, docs...)
	require.NoError(t, err)

	t.Run("count all", func(t *testing.T) {
		count, err := coll.Count(ctx, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(3), count)
	})

	t.Run("count with filter", func(t *testing.T) {
		count, err := coll.Count(ctx, vectorstore.Eq("type", "A"))
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})
}

func TestStats(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()

	// Create collections and add documents
	coll1 := store.Collection("coll1")
	coll2 := store.Collection("coll2")

	doc1 := createTestDoc("doc1", "content1", []float32{1, 0, 0})
	doc2 := createTestDoc("doc2", "content2", []float32{0, 1, 0})

	_, err := coll1.Upsert(ctx, doc1)
	require.NoError(t, err)
	_, err = coll2.Upsert(ctx, doc2)
	require.NoError(t, err)

	t.Run("store stats", func(t *testing.T) {
		stats, err := store.Stats(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(2), stats.Collections)
		assert.Equal(t, int64(2), stats.Documents)
		assert.Equal(t, "memory", stats.Provider)
		assert.Greater(t, stats.StorageBytes, int64(0))
	})

	t.Run("collection stats", func(t *testing.T) {
		stats, err := coll1.Stats(ctx)
		require.NoError(t, err)
		assert.Equal(t, "coll1", stats.Name)
		assert.Equal(t, int64(1), stats.Documents)
		assert.Equal(t, 3, stats.EmbeddingDimensions)
		assert.Greater(t, stats.StorageBytes, int64(0))
	})
}

func TestClear(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()
	coll := store.Collection("test")

	// Insert documents
	docs := []*vectorstore.Document{
		createTestDoc("doc1", "content1", []float32{1, 0, 0}),
		createTestDoc("doc2", "content2", []float32{0, 1, 0}),
	}
	_, err := coll.Upsert(ctx, docs...)
	require.NoError(t, err)

	// Clear collection
	err = coll.Clear(ctx)
	require.NoError(t, err)

	// Verify empty
	count, err := coll.Count(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestDistanceMetrics(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()
	coll := store.Collection("test")

	// Insert documents
	docs := []*vectorstore.Document{
		createTestDoc("doc1", "content1", []float32{1.0, 0.0, 0.0}),
		createTestDoc("doc2", "content2", []float32{0.0, 1.0, 0.0}),
	}
	_, err := coll.Upsert(ctx, docs...)
	require.NoError(t, err)

	t.Run("cosine similarity", func(t *testing.T) {
		query := &vectorstore.Query{
			Embedding: vectorstore.NewEmbedding([]float32{1.0, 0.0, 0.0}, "test"),
			Limit:     2,
			Metric:    vectorstore.DistanceMetricCosine,
		}

		result, err := coll.Query(ctx, query)
		require.NoError(t, err)
		assert.Len(t, result.Matches, 2)
		assert.Equal(t, float32(1.0), result.Matches[0].Score)
	})

	t.Run("dot product", func(t *testing.T) {
		query := &vectorstore.Query{
			Embedding: vectorstore.NewEmbedding([]float32{1.0, 0.0, 0.0}, "test"),
			Limit:     2,
			Metric:    vectorstore.DistanceMetricDotProduct,
		}

		result, err := coll.Query(ctx, query)
		require.NoError(t, err)
		assert.Len(t, result.Matches, 2)
	})

	t.Run("euclidean distance", func(t *testing.T) {
		query := &vectorstore.Query{
			Embedding: vectorstore.NewEmbedding([]float32{1.0, 0.0, 0.0}, "test"),
			Limit:     2,
			Metric:    vectorstore.DistanceMetricEuclidean,
		}

		result, err := coll.Query(ctx, query)
		require.NoError(t, err)
		assert.Len(t, result.Matches, 2)
	})
}

func TestConcurrency(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()
	coll := store.Collection("test")

	t.Run("concurrent upserts", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 10
		docsPerGoroutine := 10

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(routineID int) {
				defer wg.Done()

				docs := make([]*vectorstore.Document, docsPerGoroutine)
				for j := 0; j < docsPerGoroutine; j++ {
					docs[j] = createTestDoc(
						fmt.Sprintf("doc_%d_%d", routineID, j),
						fmt.Sprintf("content_%d_%d", routineID, j),
						[]float32{float32(routineID), float32(j), 0.5},
					)
				}

				_, err := coll.Upsert(ctx, docs...)
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()

		count, err := coll.Count(ctx, nil)
		require.NoError(t, err)
		assert.Equal(t, int64(numGoroutines*docsPerGoroutine), count)
	})

	t.Run("concurrent queries", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 20

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(routineID int) {
				defer wg.Done()

				query := &vectorstore.Query{
					Embedding: vectorstore.NewEmbedding([]float32{float32(routineID % 5), 0.5, 0.5}, "test"),
					Limit:     5,
				}

				result, err := coll.Query(ctx, query)
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}(i)
		}

		wg.Wait()
	})
}

func TestHelperFunctions(t *testing.T) {
	t.Run("cosine similarity", func(t *testing.T) {
		assert.InDelta(t, 1.0, cosineSimilarity([]float32{1, 0, 0}, []float32{1, 0, 0}), 0.001)
		assert.InDelta(t, 0.0, cosineSimilarity([]float32{1, 0, 0}, []float32{0, 1, 0}), 0.001)
		assert.InDelta(t, -1.0, cosineSimilarity([]float32{1, 0, 0}, []float32{-1, 0, 0}), 0.001)
	})

	t.Run("dot product", func(t *testing.T) {
		assert.InDelta(t, 32.0, dotProduct([]float32{1, 2, 3}, []float32{4, 5, 6}), 0.001)
		assert.InDelta(t, 0.0, dotProduct([]float32{1, 0, 0}, []float32{0, 1, 0}), 0.001)
	})

	t.Run("euclidean distance", func(t *testing.T) {
		assert.InDelta(t, 0.0, euclideanDistance([]float32{1, 2, 3}, []float32{1, 2, 3}), 0.001)
		assert.InDelta(t, 5.0, euclideanDistance([]float32{0, 0, 0}, []float32{3, 4, 0}), 0.001)
	})
}

func TestRequiredScope(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()

	// Create collection with required scope fields
	coll := store.Collection("test", vectorstore.WithScope("tenant", "user"))

	t.Run("document without scope fails", func(t *testing.T) {
		doc := createTestDoc("doc1", "content", []float32{1, 0, 0})
		_, err := coll.Upsert(ctx, doc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "scope")
	})

	t.Run("document with incomplete scope fails", func(t *testing.T) {
		doc := createTestDoc("doc1", "content", []float32{1, 0, 0})
		doc.Scope = &vectorstore.Scope{Tenant: "tenant1"} // Missing user
		_, err := coll.Upsert(ctx, doc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "user")
	})

	t.Run("document with complete scope succeeds", func(t *testing.T) {
		doc := createTestDoc("doc1", "content", []float32{1, 0, 0})
		doc.Scope = &vectorstore.Scope{Tenant: "tenant1", User: "user1"}
		_, err := coll.Upsert(ctx, doc)
		require.NoError(t, err)
	})
}

func TestEmbeddingDimensions(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()

	// Create collection with specific dimensions
	coll := store.Collection("test", vectorstore.WithDimensions(3))

	t.Run("correct dimensions succeeds", func(t *testing.T) {
		doc := createTestDoc("doc1", "content", []float32{1, 0, 0})
		_, err := coll.Upsert(ctx, doc)
		require.NoError(t, err)
	})

	t.Run("wrong dimensions fails", func(t *testing.T) {
		doc := createTestDoc("doc2", "content", []float32{1, 0, 0, 0}) // 4 dimensions
		_, err := coll.Upsert(ctx, doc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dimension mismatch")
	})
}

func TestFilterOnlyQuery(t *testing.T) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()
	coll := store.Collection("test")

	// Insert documents without embeddings
	docs := []*vectorstore.Document{
		{
			ID:       "doc1",
			Content:  vectorstore.NewTextContent("content1"),
			Metadata: map[string]any{"category": "A"},
		},
		{
			ID:       "doc2",
			Content:  vectorstore.NewTextContent("content2"),
			Metadata: map[string]any{"category": "B"},
		},
	}
	_, err := coll.Upsert(ctx, docs...)
	require.NoError(t, err)

	// Query with only filters (no embedding)
	query := &vectorstore.Query{
		Filters: vectorstore.Eq("category", "A"),
		Limit:   10,
	}

	result, err := coll.Query(ctx, query)
	require.NoError(t, err)
	assert.Len(t, result.Matches, 1)
	assert.Equal(t, "doc1", result.Matches[0].Document.ID)
}

// Benchmarks

func BenchmarkUpsert(b *testing.B) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()
	coll := store.Collection("test")

	doc := createTestDoc("bench_doc", "benchmark content", make([]float32, 768))
	for i := range doc.Embedding.Vector {
		doc.Embedding.Vector[i] = float32(i) * 0.001
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc.ID = fmt.Sprintf("bench_doc_%d", i)
		_, _ = coll.Upsert(ctx, doc)
	}
}

func BenchmarkQuery(b *testing.B) {
	ctx := context.Background()
	store, _ := New()
	defer func() { _ = store.Close() }()
	coll := store.Collection("test")

	// Seed with documents
	numDocs := 1000
	docs := make([]*vectorstore.Document, numDocs)
	for i := 0; i < numDocs; i++ {
		embedding := make([]float32, 768)
		for j := range embedding {
			embedding[j] = float32(i*j) * 0.001
		}
		docs[i] = createTestDoc(fmt.Sprintf("doc_%d", i), fmt.Sprintf("content_%d", i), embedding)
	}
	_, _ = coll.Upsert(ctx, docs...)

	queryEmbedding := make([]float32, 768)
	for i := range queryEmbedding {
		queryEmbedding[i] = float32(i) * 0.001
	}

	query := &vectorstore.Query{
		Embedding: vectorstore.NewEmbedding(queryEmbedding, "test"),
		Limit:     10,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = coll.Query(ctx, query)
	}
}

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
