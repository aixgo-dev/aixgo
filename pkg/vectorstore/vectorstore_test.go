package vectorstore

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateDocument tests the document validation function.
func TestValidateDocument(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		doc     *Document
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid document",
			doc: &Document{
				ID:        "doc1",
				Content:   "test content",
				Embedding: []float32{0.1, 0.2, 0.3},
				Metadata: map[string]interface{}{
					"source": "test",
				},
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr: false,
		},
		{
			name: "empty ID",
			doc: &Document{
				ID:        "",
				Content:   "test content",
				Embedding: []float32{0.1, 0.2, 0.3},
			},
			wantErr: true,
			errMsg:  "document ID cannot be empty",
		},
		{
			name: "empty content",
			doc: &Document{
				ID:        "doc1",
				Content:   "",
				Embedding: []float32{0.1, 0.2, 0.3},
			},
			wantErr: true,
			errMsg:  "document content cannot be empty",
		},
		{
			name: "empty embedding",
			doc: &Document{
				ID:        "doc1",
				Content:   "test content",
				Embedding: []float32{},
			},
			wantErr: true,
			errMsg:  "document embedding cannot be empty",
		},
		{
			name: "nil embedding",
			doc: &Document{
				ID:        "doc1",
				Content:   "test content",
				Embedding: nil,
			},
			wantErr: true,
			errMsg:  "document embedding cannot be empty",
		},
		{
			name: "NaN in embedding",
			doc: &Document{
				ID:        "doc1",
				Content:   "test content",
				Embedding: []float32{0.1, float32(math.NaN()), 0.3},
			},
			wantErr: true,
			errMsg:  "embedding contains invalid value at index 1",
		},
		{
			name: "Inf in embedding",
			doc: &Document{
				ID:        "doc1",
				Content:   "test content",
				Embedding: []float32{0.1, float32(math.Inf(1)), 0.3},
			},
			wantErr: true,
			errMsg:  "embedding contains invalid value at index 1",
		},
		{
			name: "negative Inf in embedding",
			doc: &Document{
				ID:        "doc1",
				Content:   "test content",
				Embedding: []float32{float32(math.Inf(-1)), 0.2, 0.3},
			},
			wantErr: true,
			errMsg:  "embedding contains invalid value at index 0",
		},
		{
			name: "valid with nil metadata",
			doc: &Document{
				ID:        "doc1",
				Content:   "test content",
				Embedding: []float32{0.1, 0.2, 0.3},
				Metadata:  nil,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDocument(tt.doc)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidateSearchQuery tests the search query validation function.
func TestValidateSearchQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   *SearchQuery
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid query",
			query: &SearchQuery{
				Embedding:      []float32{0.1, 0.2, 0.3},
				TopK:           10,
				MinScore:       0.5,
				DistanceMetric: DistanceMetricCosine,
			},
			wantErr: false,
		},
		{
			name: "empty embedding",
			query: &SearchQuery{
				Embedding: []float32{},
				TopK:      10,
			},
			wantErr: true,
			errMsg:  "query embedding cannot be empty",
		},
		{
			name: "nil embedding",
			query: &SearchQuery{
				Embedding: nil,
				TopK:      10,
			},
			wantErr: true,
			errMsg:  "query embedding cannot be empty",
		},
		{
			name: "TopK less than 1",
			query: &SearchQuery{
				Embedding: []float32{0.1, 0.2, 0.3},
				TopK:      0,
			},
			wantErr: true,
			errMsg:  "TopK must be at least 1",
		},
		{
			name: "TopK negative",
			query: &SearchQuery{
				Embedding: []float32{0.1, 0.2, 0.3},
				TopK:      -5,
			},
			wantErr: true,
			errMsg:  "TopK must be at least 1",
		},
		{
			name: "TopK exceeds max",
			query: &SearchQuery{
				Embedding: []float32{0.1, 0.2, 0.3},
				TopK:      1001,
			},
			wantErr: true,
			errMsg:  "TopK cannot exceed 1000",
		},
		{
			name: "MinScore negative",
			query: &SearchQuery{
				Embedding: []float32{0.1, 0.2, 0.3},
				TopK:      10,
				MinScore:  -0.1,
			},
			wantErr: true,
			errMsg:  "MinScore must be between 0 and 1",
		},
		{
			name: "MinScore exceeds 1",
			query: &SearchQuery{
				Embedding: []float32{0.1, 0.2, 0.3},
				TopK:      10,
				MinScore:  1.5,
			},
			wantErr: true,
			errMsg:  "MinScore must be between 0 and 1",
		},
		{
			name: "invalid distance metric",
			query: &SearchQuery{
				Embedding:      []float32{0.1, 0.2, 0.3},
				TopK:           10,
				DistanceMetric: "invalid_metric",
			},
			wantErr: true,
			errMsg:  "invalid distance metric",
		},
		{
			name: "valid cosine metric",
			query: &SearchQuery{
				Embedding:      []float32{0.1, 0.2, 0.3},
				TopK:           10,
				DistanceMetric: DistanceMetricCosine,
			},
			wantErr: false,
		},
		{
			name: "valid euclidean metric",
			query: &SearchQuery{
				Embedding:      []float32{0.1, 0.2, 0.3},
				TopK:           10,
				DistanceMetric: DistanceMetricEuclidean,
			},
			wantErr: false,
		},
		{
			name: "valid dot_product metric",
			query: &SearchQuery{
				Embedding:      []float32{0.1, 0.2, 0.3},
				TopK:           10,
				DistanceMetric: DistanceMetricDotProduct,
			},
			wantErr: false,
		},
		{
			name: "empty distance metric (should be allowed)",
			query: &SearchQuery{
				Embedding:      []float32{0.1, 0.2, 0.3},
				TopK:           10,
				DistanceMetric: "",
			},
			wantErr: false,
		},
		{
			name: "valid query with filter",
			query: &SearchQuery{
				Embedding: []float32{0.1, 0.2, 0.3},
				TopK:      10,
				Filter: &MetadataFilter{
					Must: map[string]interface{}{
						"source": "test",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid query with MinScore zero",
			query: &SearchQuery{
				Embedding: []float32{0.1, 0.2, 0.3},
				TopK:      10,
				MinScore:  0.0,
			},
			wantErr: false,
		},
		{
			name: "valid query with MinScore one",
			query: &SearchQuery{
				Embedding: []float32{0.1, 0.2, 0.3},
				TopK:      10,
				MinScore:  1.0,
			},
			wantErr: false,
		},
		{
			name: "TopK exactly 1000",
			query: &SearchQuery{
				Embedding: []float32{0.1, 0.2, 0.3},
				TopK:      1000,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSearchQuery(tt.query)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestDistanceMetricConstants tests the distance metric constants.
func TestDistanceMetricConstants(t *testing.T) {
	assert.Equal(t, DistanceMetric("cosine"), DistanceMetricCosine)
	assert.Equal(t, DistanceMetric("euclidean"), DistanceMetricEuclidean)
	assert.Equal(t, DistanceMetric("dot_product"), DistanceMetricDotProduct)
}

// TestIsNaN tests the isNaN helper function.
func TestIsNaN(t *testing.T) {
	tests := []struct {
		name  string
		value float32
		want  bool
	}{
		{"NaN", float32(math.NaN()), true},
		{"normal value", 1.5, false},
		{"zero", 0, false},
		{"negative", -1.5, false},
		{"infinity", float32(math.Inf(1)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isNaN(tt.value))
		})
	}
}

// TestIsInf tests the isInf helper function.
func TestIsInf(t *testing.T) {
	tests := []struct {
		name  string
		value float32
		want  bool
	}{
		{"positive infinity", float32(math.Inf(1)), true},
		{"negative infinity", float32(math.Inf(-1)), true},
		{"normal value", 1.5, false},
		{"zero", 0, false},
		{"max float32", math.MaxFloat32, false},
		{"NaN", float32(math.NaN()), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isInf(tt.value))
		})
	}
}

// TestMetadataFilter tests metadata filter structure.
func TestMetadataFilter(t *testing.T) {
	filter := &MetadataFilter{
		Must: map[string]interface{}{
			"source": "documentation",
			"status": "published",
		},
		Should: map[string]interface{}{
			"category": "guide",
		},
		MustNot: map[string]interface{}{
			"status": "draft",
		},
	}

	assert.NotNil(t, filter.Must)
	assert.NotNil(t, filter.Should)
	assert.NotNil(t, filter.MustNot)
	assert.Equal(t, "documentation", filter.Must["source"])
	assert.Equal(t, "published", filter.Must["status"])
	assert.Equal(t, "guide", filter.Should["category"])
	assert.Equal(t, "draft", filter.MustNot["status"])
}

// TestDocumentStructure tests the Document structure.
func TestDocumentStructure(t *testing.T) {
	now := time.Now()
	doc := Document{
		ID:        "test-id",
		Content:   "test content",
		Embedding: []float32{0.1, 0.2, 0.3},
		Metadata: map[string]interface{}{
			"key": "value",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	assert.Equal(t, "test-id", doc.ID)
	assert.Equal(t, "test content", doc.Content)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, doc.Embedding)
	assert.Equal(t, "value", doc.Metadata["key"])
	assert.Equal(t, now, doc.CreatedAt)
	assert.Equal(t, now, doc.UpdatedAt)
}

// TestSearchResult tests the SearchResult structure.
func TestSearchResult(t *testing.T) {
	doc := Document{
		ID:        "test-id",
		Content:   "test content",
		Embedding: []float32{0.1, 0.2, 0.3},
	}

	result := SearchResult{
		Document: doc,
		Score:    0.95,
		Distance: 0.05,
	}

	assert.Equal(t, doc, result.Document)
	assert.Equal(t, float32(0.95), result.Score)
	assert.Equal(t, float32(0.05), result.Distance)
}

// TestSearchQuery tests the SearchQuery structure.
func TestSearchQuery(t *testing.T) {
	filter := &MetadataFilter{
		Must: map[string]interface{}{
			"source": "test",
		},
	}

	query := SearchQuery{
		Embedding:      []float32{0.1, 0.2, 0.3},
		TopK:           10,
		Filter:         filter,
		MinScore:       0.5,
		DistanceMetric: DistanceMetricCosine,
	}

	assert.Equal(t, []float32{0.1, 0.2, 0.3}, query.Embedding)
	assert.Equal(t, 10, query.TopK)
	assert.Equal(t, filter, query.Filter)
	assert.Equal(t, float32(0.5), query.MinScore)
	assert.Equal(t, DistanceMetricCosine, query.DistanceMetric)
}

// BenchmarkValidateDocument benchmarks document validation.
func BenchmarkValidateDocument(b *testing.B) {
	doc := &Document{
		ID:        "doc1",
		Content:   "test content",
		Embedding: make([]float32, 768), // Common embedding size
		Metadata: map[string]interface{}{
			"source": "test",
		},
	}

	// Fill embedding with valid values
	for i := range doc.Embedding {
		doc.Embedding[i] = float32(i) * 0.001
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateDocument(doc)
	}
}

// BenchmarkValidateSearchQuery benchmarks search query validation.
func BenchmarkValidateSearchQuery(b *testing.B) {
	query := &SearchQuery{
		Embedding:      make([]float32, 768),
		TopK:           10,
		MinScore:       0.5,
		DistanceMetric: DistanceMetricCosine,
	}

	// Fill embedding with valid values
	for i := range query.Embedding {
		query.Embedding[i] = float32(i) * 0.001
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateSearchQuery(query)
	}
}
