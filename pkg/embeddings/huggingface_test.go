package embeddings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewHuggingFace tests creating a new HuggingFace embeddings service.
func TestNewHuggingFace(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Config{
				HuggingFace: &HuggingFaceConfig{
					Model:    "sentence-transformers/all-MiniLM-L6-v2",
					Endpoint: "https://api-inference.huggingface.co",
				},
			},
			wantErr: false,
		},
		{
			name: "missing config",
			config: Config{
				HuggingFace: nil,
			},
			wantErr: true,
			errMsg:  "huggingface configuration is required",
		},
		{
			name: "with API key",
			config: Config{
				HuggingFace: &HuggingFaceConfig{
					APIKey: "test-key",
					Model:  "sentence-transformers/all-MiniLM-L6-v2",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := NewHuggingFace(tt.config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, service)
				assert.Equal(t, tt.config.HuggingFace.Model, service.ModelName())
			}
		})
	}
}

// TestHuggingFaceEmbed tests single text embedding.
func TestHuggingFaceEmbed(t *testing.T) {
	tests := []struct {
		name           string
		text           string
		mockResponse   interface{}
		mockStatusCode int
		wantErr        bool
		errMsg         string
	}{
		{
			name: "successful embedding",
			text: "test text",
			mockResponse: [][]float32{
				{0.1, 0.2, 0.3, 0.4},
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "empty text",
			text:           "",
			mockStatusCode: http.StatusOK,
			wantErr:        true,
			errMsg:         "text cannot be empty",
		},
		{
			name:           "API error",
			text:           "test text",
			mockResponse:   map[string]string{"error": "model not found"},
			mockStatusCode: http.StatusNotFound,
			wantErr:        true,
			errMsg:         "API error",
		},
		{
			name:           "server error",
			text:           "test text",
			mockResponse:   map[string]string{"error": "internal server error"},
			mockStatusCode: http.StatusInternalServerError,
			wantErr:        true,
			errMsg:         "API error",
		},
		{
			name: "single embedding response",
			text: "test text",
			mockResponse: []float32{
				0.1, 0.2, 0.3, 0.4,
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method
				assert.Equal(t, "POST", r.Method)

				// Verify headers
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				// Verify request body
				if tt.text != "" {
					var reqBody hfRequest
					err := json.NewDecoder(r.Body).Decode(&reqBody)
					require.NoError(t, err)
					assert.Equal(t, tt.text, reqBody.Inputs)
				}

				// Send mock response
				w.WriteHeader(tt.mockStatusCode)
				_ = json.NewEncoder(w).Encode(tt.mockResponse)
			}))
			defer server.Close()

			// Create service with mock server
			config := Config{
				HuggingFace: &HuggingFaceConfig{
					Model:        "test-model",
					Endpoint:     server.URL,
					WaitForModel: true,
					UseCache:     false,
				},
			}

			service, err := NewHuggingFace(config)
			require.NoError(t, err)

			// Test embedding
			ctx := context.Background()
			embedding, err := service.Embed(ctx, tt.text)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, embedding)
			}
		})
	}
}

// TestHuggingFaceEmbedBatch tests batch text embedding.
func TestHuggingFaceEmbedBatch(t *testing.T) {
	tests := []struct {
		name           string
		texts          []string
		mockResponse   [][]float32
		mockStatusCode int
		wantErr        bool
		errMsg         string
	}{
		{
			name:  "successful batch embedding",
			texts: []string{"text1", "text2", "text3"},
			mockResponse: [][]float32{
				{0.1, 0.2, 0.3},
				{0.4, 0.5, 0.6},
				{0.7, 0.8, 0.9},
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "empty texts",
			texts:          []string{},
			mockStatusCode: http.StatusOK,
			wantErr:        true,
			errMsg:         "texts cannot be empty",
		},
		{
			name:           "API error",
			texts:          []string{"text1", "text2"},
			mockResponse:   nil,
			mockStatusCode: http.StatusBadRequest,
			wantErr:        true,
			errMsg:         "API error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				assert.Equal(t, "POST", r.Method)

				// Verify request body
				if len(tt.texts) > 0 {
					var reqBody hfRequest
					err := json.NewDecoder(r.Body).Decode(&reqBody)
					require.NoError(t, err)
				}

				// Send mock response
				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
				} else {
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "bad request"})
				}
			}))
			defer server.Close()

			// Create service with mock server
			config := Config{
				HuggingFace: &HuggingFaceConfig{
					Model:    "test-model",
					Endpoint: server.URL,
				},
			}

			service, err := NewHuggingFace(config)
			require.NoError(t, err)

			// Test batch embedding
			ctx := context.Background()
			embeddings, err := service.EmbedBatch(ctx, tt.texts)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.Len(t, embeddings, len(tt.texts))
			}
		})
	}
}

// TestHuggingFaceDimensions tests getting embedding dimensions.
func TestHuggingFaceDimensions(t *testing.T) {
	tests := []struct {
		name              string
		model             string
		expectedDimension int
	}{
		{"all-MiniLM-L6-v2", "sentence-transformers/all-MiniLM-L6-v2", 384},
		{"all-mpnet-base-v2", "sentence-transformers/all-mpnet-base-v2", 768},
		{"bge-small", "BAAI/bge-small-en-v1.5", 384},
		{"bge-base", "BAAI/bge-base-en-v1.5", 768},
		{"bge-large", "BAAI/bge-large-en-v1.5", 1024},
		{"gte-large", "thenlper/gte-large", 1024},
		{"unknown model", "unknown/model", 768}, // default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				HuggingFace: &HuggingFaceConfig{
					Model: tt.model,
				},
			}

			service, err := NewHuggingFace(config)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedDimension, service.Dimensions())
		})
	}
}

// TestHuggingFaceModelName tests getting model name.
func TestHuggingFaceModelName(t *testing.T) {
	model := "sentence-transformers/all-MiniLM-L6-v2"
	config := Config{
		HuggingFace: &HuggingFaceConfig{
			Model: model,
		},
	}

	service, err := NewHuggingFace(config)
	require.NoError(t, err)

	assert.Equal(t, model, service.ModelName())
}

// TestHuggingFaceClose tests closing the service.
func TestHuggingFaceClose(t *testing.T) {
	config := Config{
		HuggingFace: &HuggingFaceConfig{
			Model: "sentence-transformers/all-MiniLM-L6-v2",
		},
	}

	service, err := NewHuggingFace(config)
	require.NoError(t, err)

	err = service.Close()
	assert.NoError(t, err)
}

// TestHuggingFaceWithAPIKey tests using API key for authentication.
func TestHuggingFaceWithAPIKey(t *testing.T) {
	apiKey := "test-api-key"

	// Create mock server that verifies API key
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Authorization header
		authHeader := r.Header.Get("Authorization")
		assert.Equal(t, "Bearer "+apiKey, authHeader)

		// Send mock response
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([][]float32{{0.1, 0.2, 0.3}})
	}))
	defer server.Close()

	// Create service with API key
	config := Config{
		HuggingFace: &HuggingFaceConfig{
			APIKey:   apiKey,
			Model:    "test-model",
			Endpoint: server.URL,
		},
	}

	service, err := NewHuggingFace(config)
	require.NoError(t, err)

	// Test embedding
	ctx := context.Background()
	_, err = service.Embed(ctx, "test text")
	require.NoError(t, err)
}

// TestHuggingFaceWithoutAPIKey tests using service without API key.
func TestHuggingFaceWithoutAPIKey(t *testing.T) {
	// Create mock server that verifies no API key
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify no Authorization header
		authHeader := r.Header.Get("Authorization")
		assert.Empty(t, authHeader)

		// Send mock response
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([][]float32{{0.1, 0.2, 0.3}})
	}))
	defer server.Close()

	// Create service without API key
	config := Config{
		HuggingFace: &HuggingFaceConfig{
			Model:    "test-model",
			Endpoint: server.URL,
		},
	}

	service, err := NewHuggingFace(config)
	require.NoError(t, err)

	// Test embedding
	ctx := context.Background()
	_, err = service.Embed(ctx, "test text")
	require.NoError(t, err)
}

// TestHuggingFaceRequestOptions tests request options.
func TestHuggingFaceRequestOptions(t *testing.T) {
	// Create mock server that verifies options
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody hfRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		// Verify options
		assert.NotNil(t, reqBody.Options)
		assert.True(t, reqBody.Options.WaitForModel)
		assert.False(t, reqBody.Options.UseCache)

		// Send mock response
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([][]float32{{0.1, 0.2, 0.3}})
	}))
	defer server.Close()

	// Create service with specific options
	config := Config{
		HuggingFace: &HuggingFaceConfig{
			Model:        "test-model",
			Endpoint:     server.URL,
			WaitForModel: true,
			UseCache:     false,
		},
	}

	service, err := NewHuggingFace(config)
	require.NoError(t, err)

	// Test embedding
	ctx := context.Background()
	_, err = service.Embed(ctx, "test text")
	require.NoError(t, err)
}

// TestHuggingFaceContextCancellation tests context cancellation.
func TestHuggingFaceContextCancellation(t *testing.T) {
	// Create mock server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([][]float32{{0.1, 0.2, 0.3}})
	}))
	defer server.Close()

	config := Config{
		HuggingFace: &HuggingFaceConfig{
			Model:    "test-model",
			Endpoint: server.URL,
		},
	}

	service, err := NewHuggingFace(config)
	require.NoError(t, err)

	// Create context with immediate cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Test embedding with cancelled context
	_, err = service.Embed(ctx, "test text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

// TestHuggingFaceTimeout tests request timeout.
func TestHuggingFaceTimeout(t *testing.T) {
	// Create mock server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([][]float32{{0.1, 0.2, 0.3}})
	}))
	defer server.Close()

	config := Config{
		HuggingFace: &HuggingFaceConfig{
			Model:    "test-model",
			Endpoint: server.URL,
		},
	}

	service, err := NewHuggingFace(config)
	require.NoError(t, err)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Test embedding with timeout
	_, err = service.Embed(ctx, "test text")
	require.Error(t, err)
}

// TestHuggingFaceInvalidJSON tests handling of invalid JSON responses.
func TestHuggingFaceInvalidJSON(t *testing.T) {
	// Create mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	config := Config{
		HuggingFace: &HuggingFaceConfig{
			Model:    "test-model",
			Endpoint: server.URL,
		},
	}

	service, err := NewHuggingFace(config)
	require.NoError(t, err)

	// Test embedding
	ctx := context.Background()
	_, err = service.Embed(ctx, "test text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
}

// TestGetHuggingFaceModelDimensions tests the model dimensions lookup.
func TestGetHuggingFaceModelDimensions(t *testing.T) {
	tests := []struct {
		model    string
		expected int
	}{
		{"sentence-transformers/all-MiniLM-L6-v2", 384},
		{"sentence-transformers/all-MiniLM-L12-v2", 384},
		{"sentence-transformers/all-mpnet-base-v2", 768},
		{"BAAI/bge-small-en-v1.5", 384},
		{"BAAI/bge-base-en-v1.5", 768},
		{"BAAI/bge-large-en-v1.5", 1024},
		{"thenlper/gte-small", 384},
		{"thenlper/gte-base", 768},
		{"thenlper/gte-large", 1024},
		{"intfloat/e5-small-v2", 384},
		{"intfloat/e5-base-v2", 768},
		{"intfloat/e5-large-v2", 1024},
		{"jinaai/jina-embeddings-v2-small-en", 512},
		{"jinaai/jina-embeddings-v2-base-en", 768},
		{"unknown/model", 768}, // default
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			dims := getHuggingFaceModelDimensions(tt.model)
			assert.Equal(t, tt.expected, dims)
		})
	}
}

// TestHuggingFaceEmptyResponse tests handling of empty response.
func TestHuggingFaceEmptyResponse(t *testing.T) {
	// Create mock server that returns empty array
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([][]float32{})
	}))
	defer server.Close()

	config := Config{
		HuggingFace: &HuggingFaceConfig{
			Model:    "test-model",
			Endpoint: server.URL,
		},
	}

	service, err := NewHuggingFace(config)
	require.NoError(t, err)

	// Test embedding
	ctx := context.Background()
	_, err = service.Embed(ctx, "test text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no embeddings returned")
}

// BenchmarkHuggingFaceEmbed benchmarks single embedding.
func BenchmarkHuggingFaceEmbed(b *testing.B) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		embedding := make([]float32, 384)
		for i := range embedding {
			embedding[i] = float32(i) * 0.001
		}
		_ = json.NewEncoder(w).Encode([][]float32{embedding})
	}))
	defer server.Close()

	config := Config{
		HuggingFace: &HuggingFaceConfig{
			Model:    "sentence-transformers/all-MiniLM-L6-v2",
			Endpoint: server.URL,
		},
	}

	service, _ := NewHuggingFace(config)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.Embed(ctx, "benchmark text")
	}
}

// BenchmarkHuggingFaceEmbedBatch benchmarks batch embedding.
func BenchmarkHuggingFaceEmbedBatch(b *testing.B) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		embeddings := make([][]float32, 10)
		for i := range embeddings {
			embeddings[i] = make([]float32, 384)
			for j := range embeddings[i] {
				embeddings[i][j] = float32(j) * 0.001
			}
		}
		_ = json.NewEncoder(w).Encode(embeddings)
	}))
	defer server.Close()

	config := Config{
		HuggingFace: &HuggingFaceConfig{
			Model:    "sentence-transformers/all-MiniLM-L6-v2",
			Endpoint: server.URL,
		},
	}

	service, _ := NewHuggingFace(config)
	ctx := context.Background()
	texts := make([]string, 10)
	for i := range texts {
		texts[i] = "benchmark text"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.EmbedBatch(ctx, texts)
	}
}
