package firestore

import (
	"math"
	"testing"
	"time"

	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"github.com/aixgo-dev/aixgo/pkg/vectorstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNew tests creating a new Firestore vector store.
func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  vectorstore.Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "missing firestore config",
			config: vectorstore.Config{
				Provider:            "firestore",
				EmbeddingDimensions: 768,
			},
			wantErr: true,
			errMsg:  "firestore configuration is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := New(tt.config)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, store)
			}
		})
	}
}

// TestToFirestoreDistanceType tests distance metric conversion.
func TestToFirestoreDistanceType(t *testing.T) {
	tests := []struct {
		name     string
		metric   vectorstore.DistanceMetric
		expected string
	}{
		{"cosine", vectorstore.DistanceMetricCosine, "COSINE"},
		{"euclidean", vectorstore.DistanceMetricEuclidean, "EUCLIDEAN"},
		{"dot_product", vectorstore.DistanceMetricDotProduct, "DOT_PRODUCT"},
		{"unknown defaults to cosine", "unknown", "COSINE"},
		{"empty defaults to cosine", "", "COSINE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toFirestoreDistanceType(tt.metric)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCosineSimilarity tests the cosine similarity function.
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
		{
			name:     "normalized vectors",
			vec1:     []float32{0.6, 0.8, 0.0},
			vec2:     []float32{0.8, 0.6, 0.0},
			expected: 0.96, // 0.6*0.8 + 0.8*0.6 = 0.96
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.vec1, tt.vec2)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

// TestDotProduct tests the dot product function.
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
			name:     "negative values",
			vec1:     []float32{1.0, -2.0, 3.0},
			vec2:     []float32{-1.0, 2.0, -3.0},
			expected: -14.0, // 1*(-1) + (-2)*2 + 3*(-3) = -1 - 4 - 9 = -14
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dotProduct(tt.vec1, tt.vec2)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

// TestEuclideanDistance tests the Euclidean distance function.
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
			name:     "3D distance",
			vec1:     []float32{1.0, 2.0, 3.0},
			vec2:     []float32{4.0, 6.0, 8.0},
			expected: float32(math.Sqrt(50)), // sqrt((3^2 + 4^2 + 5^2)) = sqrt(50)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := euclideanDistance(tt.vec1, tt.vec2)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

// TestCalculateSimilarity tests the similarity calculation dispatcher.
func TestCalculateSimilarity(t *testing.T) {
	vec1 := []float32{1.0, 0.0, 0.0}
	vec2 := []float32{1.0, 0.0, 0.0}

	tests := []struct {
		name     string
		metric   vectorstore.DistanceMetric
		expected float32
	}{
		{"cosine", vectorstore.DistanceMetricCosine, 1.0},
		{"dot_product", vectorstore.DistanceMetricDotProduct, 1.0},
		{"euclidean", vectorstore.DistanceMetricEuclidean, 1.0}, // 1 / (1 + 0) = 1
		{"default to cosine", "", 1.0},
		{"unknown defaults to cosine", "invalid", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateSimilarity(vec1, vec2, tt.metric)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

// TestSqrt tests the square root function.
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
		{"sixteen", 16, 4},
		{"two", 2, float32(math.Sqrt(2))},
		{"ten", 10, float32(math.Sqrt(10))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sqrt(tt.input)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

// TestFloat32SliceToFirestoreArray tests conversion to Firestore array format.
func TestFloat32SliceToFirestoreArray(t *testing.T) {
	tests := []struct {
		name  string
		input []float32
	}{
		{
			name:  "empty slice",
			input: []float32{},
		},
		{
			name:  "single value",
			input: []float32{1.0},
		},
		{
			name:  "multiple values",
			input: []float32{1.0, 2.0, 3.0, 4.0, 5.0},
		},
		{
			name:  "with negative values",
			input: []float32{-1.0, 0.0, 1.0},
		},
		{
			name:  "with decimals",
			input: []float32{0.1, 0.2, 0.3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := float32SliceToFirestoreArray(tt.input)
			assert.Len(t, result, len(tt.input))

			// Verify each value is correctly converted to DoubleValue
			for i, val := range tt.input {
				assert.NotNil(t, result[i])
				assert.NotNil(t, result[i].ValueType)
				// The value should be a DoubleValue
				doubleVal, ok := result[i].ValueType.(*firestorepb.Value_DoubleValue)
				assert.True(t, ok, "Expected DoubleValue type")
				assert.InDelta(t, float64(val), doubleVal.DoubleValue, 0.0001)
			}
		})
	}
}

// TestExtractEmbeddingFromFirestore tests embedding extraction.
func TestExtractEmbeddingFromFirestore(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected []float32
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		// Note: The actual implementation returns empty slice as TODO
		// These tests document the expected behavior
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractEmbeddingFromFirestore(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFirestoreDocument tests the firestoreDocument structure.
func TestFirestoreDocument(t *testing.T) {
	now := time.Now()

	doc := firestoreDocument{
		ID:      "test-id",
		Content: "test content",
		Metadata: map[string]interface{}{
			"key": "value",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	assert.Equal(t, "test-id", doc.ID)
	assert.Equal(t, "test content", doc.Content)
	assert.Equal(t, "value", doc.Metadata["key"])
	assert.Equal(t, now, doc.CreatedAt)
	assert.Equal(t, now, doc.UpdatedAt)
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

// BenchmarkDotProduct benchmarks dot product calculation.
func BenchmarkDotProduct(b *testing.B) {
	vec1 := make([]float32, 768)
	vec2 := make([]float32, 768)
	for i := range vec1 {
		vec1[i] = float32(i) * 0.001
		vec2[i] = float32(i+1) * 0.001
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dotProduct(vec1, vec2)
	}
}

// BenchmarkEuclideanDistance benchmarks Euclidean distance calculation.
func BenchmarkEuclideanDistance(b *testing.B) {
	vec1 := make([]float32, 768)
	vec2 := make([]float32, 768)
	for i := range vec1 {
		vec1[i] = float32(i) * 0.001
		vec2[i] = float32(i+1) * 0.001
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = euclideanDistance(vec1, vec2)
	}
}

// BenchmarkFloat32SliceToFirestoreArray benchmarks conversion to Firestore format.
func BenchmarkFloat32SliceToFirestoreArray(b *testing.B) {
	slice := make([]float32, 768)
	for i := range slice {
		slice[i] = float32(i) * 0.001
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = float32SliceToFirestoreArray(slice)
	}
}
