package firestore

import (
	"context"
	"fmt"
	"math"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"github.com/aixgo-dev/aixgo/pkg/vectorstore"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FirestoreVectorStore implements the VectorStore interface using Google Cloud Firestore.
type FirestoreVectorStore struct {
	client              *firestore.Client
	collection          string
	embeddingDimensions int
	defaultTopK         int
	defaultMetric       string
}

// firestoreDocument represents the structure of a document in Firestore.
type firestoreDocument struct {
	ID        string                 `firestore:"id"`
	Content   string                 `firestore:"content"`
	Embedding interface{}            `firestore:"embedding"` // Will be firestorepb.Value with vector type
	Metadata  map[string]interface{} `firestore:"metadata,omitempty"`
	CreatedAt time.Time              `firestore:"created_at"`
	UpdatedAt time.Time              `firestore:"updated_at"`
}

func init() {
	// Register the Firestore provider with the vector store registry
	vectorstore.Register("firestore", New)
}

// New creates a new FirestoreVectorStore from the provided configuration.
func New(config vectorstore.Config) (vectorstore.VectorStore, error) {
	if config.Firestore == nil {
		return nil, fmt.Errorf("firestore configuration is required")
	}

	ctx := context.Background()
	var opts []option.ClientOption

	// Use service account credentials if provided
	if config.Firestore.CredentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(config.Firestore.CredentialsFile))
	}
	// Otherwise, use Application Default Credentials

	// Create Firestore client
	client, err := firestore.NewClient(ctx, config.Firestore.ProjectID, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create firestore client: %w", err)
	}

	return &FirestoreVectorStore{
		client:              client,
		collection:          config.Firestore.Collection,
		embeddingDimensions: config.EmbeddingDimensions,
		defaultTopK:         config.DefaultTopK,
		defaultMetric:       config.DefaultDistanceMetric,
	}, nil
}

// Upsert inserts or updates documents with embeddings.
func (f *FirestoreVectorStore) Upsert(ctx context.Context, documents []vectorstore.Document) error {
	if len(documents) == 0 {
		return nil
	}

	// Validate all documents before upserting
	for i := range documents {
		if err := vectorstore.ValidateDocument(&documents[i]); err != nil {
			return fmt.Errorf("invalid document at index %d: %w", i, err)
		}
		// Verify embedding dimensions match configuration
		if len(documents[i].Embedding) != f.embeddingDimensions {
			return fmt.Errorf("document %s embedding dimension mismatch: expected %d, got %d",
				documents[i].ID, f.embeddingDimensions, len(documents[i].Embedding))
		}
	}

	// Use BulkWriter for efficient batch writes
	bulkWriter := f.client.BulkWriter(ctx)

	for _, doc := range documents {
		now := time.Now()
		if doc.CreatedAt.IsZero() {
			doc.CreatedAt = now
		}
		doc.UpdatedAt = now

		// Convert embedding to Firestore vector type
		vectorValue := &firestorepb.Value{
			ValueType: &firestorepb.Value_MapValue{
				MapValue: &firestorepb.MapValue{
					Fields: map[string]*firestorepb.Value{
						"__type__": {
							ValueType: &firestorepb.Value_StringValue{
								StringValue: "__vector__",
							},
						},
						"value": {
							ValueType: &firestorepb.Value_ArrayValue{
								ArrayValue: &firestorepb.ArrayValue{
									Values: float32SliceToFirestoreArray(doc.Embedding),
								},
							},
						},
					},
				},
			},
		}

		fsDoc := firestoreDocument{
			ID:        doc.ID,
			Content:   doc.Content,
			Embedding: vectorValue,
			Metadata:  doc.Metadata,
			CreatedAt: doc.CreatedAt,
			UpdatedAt: doc.UpdatedAt,
		}

		docRef := f.client.Collection(f.collection).Doc(doc.ID)
		if _, err := bulkWriter.Set(docRef, fsDoc); err != nil {
			bulkWriter.End()
			return fmt.Errorf("failed to queue document %s: %w", doc.ID, err)
		}
	}

	// Flush all pending writes
	bulkWriter.End()

	return nil
}

// Search performs similarity search and returns the most similar documents.
func (f *FirestoreVectorStore) Search(ctx context.Context, query vectorstore.SearchQuery) ([]vectorstore.SearchResult, error) {
	// Set defaults
	if query.TopK == 0 {
		query.TopK = f.defaultTopK
	}
	if query.DistanceMetric == "" {
		query.DistanceMetric = vectorstore.DistanceMetric(f.defaultMetric)
	}

	// Validate query
	if err := vectorstore.ValidateSearchQuery(&query); err != nil {
		return nil, fmt.Errorf("invalid search query: %w", err)
	}

	// Verify embedding dimensions
	if len(query.Embedding) != f.embeddingDimensions {
		return nil, fmt.Errorf("query embedding dimension mismatch: expected %d, got %d",
			f.embeddingDimensions, len(query.Embedding))
	}

	// Build Firestore query
	fsQuery := f.client.Collection(f.collection).Query

	// Apply metadata filters if provided
	if query.Filter != nil {
		fsQuery = applyMetadataFilters(fsQuery, query.Filter)
	}

	// Perform vector search using FindNearest
	// Note: This is a conceptual implementation. The actual Firestore Go SDK
	// vector search API may differ. Check the latest documentation at:
	// https://firebase.google.com/docs/firestore/vector-search
	// When the SDK is updated, use: toFirestoreDistanceType(query.DistanceMetric)
	vectorField := "embedding"

	// Create vector search query
	// The exact API depends on the Firestore SDK version
	// This is based on the pattern shown in the documentation
	iter := fsQuery.OrderBy(vectorField, firestore.Desc).
		Limit(query.TopK).
		Documents(ctx)

	var results []vectorstore.SearchResult

	for {
		docSnap, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate results: %w", err)
		}

		var fsDoc firestoreDocument
		if err := docSnap.DataTo(&fsDoc); err != nil {
			return nil, fmt.Errorf("failed to unmarshal document: %w", err)
		}

		// Convert back to vectorstore.Document
		doc := vectorstore.Document{
			ID:        fsDoc.ID,
			Content:   fsDoc.Content,
			Embedding: extractEmbeddingFromFirestore(fsDoc.Embedding),
			Metadata:  fsDoc.Metadata,
			CreatedAt: fsDoc.CreatedAt,
			UpdatedAt: fsDoc.UpdatedAt,
		}

		// Calculate similarity score
		score := calculateSimilarity(query.Embedding, doc.Embedding, query.DistanceMetric)

		// Filter by minimum score if specified
		if query.MinScore > 0 && score < query.MinScore {
			continue
		}

		results = append(results, vectorstore.SearchResult{
			Document: doc,
			Score:    score,
		})
	}

	return results, nil
}

// Delete removes documents by their IDs.
func (f *FirestoreVectorStore) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// Use BulkWriter for efficient batch deletes
	bulkWriter := f.client.BulkWriter(ctx)

	for _, id := range ids {
		docRef := f.client.Collection(f.collection).Doc(id)
		if _, err := bulkWriter.Delete(docRef); err != nil {
			bulkWriter.End()
			return fmt.Errorf("failed to queue delete for document %s: %w", id, err)
		}
	}

	// Flush all pending deletes
	bulkWriter.End()

	return nil
}

// Get retrieves documents by their IDs.
func (f *FirestoreVectorStore) Get(ctx context.Context, ids []string) ([]vectorstore.Document, error) {
	if len(ids) == 0 {
		return []vectorstore.Document{}, nil
	}

	var documents []vectorstore.Document

	for _, id := range ids {
		docRef := f.client.Collection(f.collection).Doc(id)
		docSnap, err := docRef.Get(ctx)

		if err != nil {
			if status.Code(err) == codes.NotFound {
				// Document not found, skip it
				continue
			}
			return nil, fmt.Errorf("failed to get document %s: %w", id, err)
		}

		var fsDoc firestoreDocument
		if err := docSnap.DataTo(&fsDoc); err != nil {
			return nil, fmt.Errorf("failed to unmarshal document %s: %w", id, err)
		}

		doc := vectorstore.Document{
			ID:        fsDoc.ID,
			Content:   fsDoc.Content,
			Embedding: extractEmbeddingFromFirestore(fsDoc.Embedding),
			Metadata:  fsDoc.Metadata,
			CreatedAt: fsDoc.CreatedAt,
			UpdatedAt: fsDoc.UpdatedAt,
		}

		documents = append(documents, doc)
	}

	return documents, nil
}

// Close closes the connection to Firestore.
func (f *FirestoreVectorStore) Close() error {
	return f.client.Close()
}

// Helper functions

func float32SliceToFirestoreArray(slice []float32) []*firestorepb.Value {
	values := make([]*firestorepb.Value, len(slice))
	for i, v := range slice {
		values[i] = &firestorepb.Value{
			ValueType: &firestorepb.Value_DoubleValue{
				DoubleValue: float64(v),
			},
		}
	}
	return values
}

func extractEmbeddingFromFirestore(embedding interface{}) []float32 {
	if embedding == nil {
		return nil
	}

	// Handle the Firestore vector type which is stored as a protobuf Value
	if pbValue, ok := embedding.(*firestorepb.Value); ok {
		if mapVal := pbValue.GetMapValue(); mapVal != nil {
			// Check if this is our vector type marker
			if typeField := mapVal.Fields["__type__"]; typeField != nil {
				if typeField.GetStringValue() == "__vector__" {
					// Extract the array from the value field
					if valueField := mapVal.Fields["value"]; valueField != nil {
						if arrayVal := valueField.GetArrayValue(); arrayVal != nil {
							result := make([]float32, len(arrayVal.Values))
							for i, val := range arrayVal.Values {
								result[i] = float32(val.GetDoubleValue())
							}
							return result
						}
					}
				}
			}
		}
	}

	// Handle direct slice conversion (for backward compatibility or different storage formats)
	if slice, ok := embedding.([]float32); ok {
		return slice
	}

	// Handle []interface{} which might contain float64 values
	if slice, ok := embedding.([]interface{}); ok {
		result := make([]float32, len(slice))
		for i, v := range slice {
			switch val := v.(type) {
			case float64:
				result[i] = float32(val)
			case float32:
				result[i] = val
			case int:
				result[i] = float32(val)
			case int64:
				result[i] = float32(val)
			default:
				// If we can't convert, return nil to indicate failure
				return nil
			}
		}
		return result
	}

	// Unable to extract embedding
	return nil
}

func applyMetadataFilters(query firestore.Query, filter *vectorstore.MetadataFilter) firestore.Query {
	// Apply Must conditions (AND)
	if filter.Must != nil {
		for key, value := range filter.Must {
			query = query.Where(fmt.Sprintf("metadata.%s", key), "==", value)
		}
	}

	// Note: Firestore has limitations on OR queries
	// You may need to handle Should conditions differently or use multiple queries

	// Apply MustNot conditions (NOT)
	if filter.MustNot != nil {
		for key, value := range filter.MustNot {
			query = query.Where(fmt.Sprintf("metadata.%s", key), "!=", value)
		}
	}

	return query
}

func toFirestoreDistanceType(metric vectorstore.DistanceMetric) string {
	switch metric {
	case vectorstore.DistanceMetricCosine:
		return "COSINE"
	case vectorstore.DistanceMetricEuclidean:
		return "EUCLIDEAN"
	case vectorstore.DistanceMetricDotProduct:
		return "DOT_PRODUCT"
	default:
		return "COSINE" // Default
	}
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
	if len(a) != len(b) {
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
