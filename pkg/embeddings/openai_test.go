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

// TestNewOpenAI tests creating a new OpenAI embeddings service.
func TestNewOpenAI(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Config{
				OpenAI: &OpenAIConfig{
					APIKey:  "test-key",
					Model:   "text-embedding-3-small",
					BaseURL: "https://api.openai.com/v1",
				},
			},
			wantErr: false,
		},
		{
			name: "missing config",
			config: Config{
				OpenAI: nil,
			},
			wantErr: true,
			errMsg:  "openai configuration is required",
		},
		{
			name: "with custom dimensions",
			config: Config{
				OpenAI: &OpenAIConfig{
					APIKey:     "test-key",
					Model:      "text-embedding-3-large",
					Dimensions: 1024,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := NewOpenAI(tt.config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, service)
				assert.Equal(t, tt.config.OpenAI.Model, service.ModelName())
			}
		})
	}
}

// TestOpenAIEmbed tests single text embedding.
func TestOpenAIEmbed(t *testing.T) {
	tests := []struct {
		name           string
		text           string
		mockResponse   *openAIResponse
		mockStatusCode int
		wantErr        bool
		errMsg         string
	}{
		{
			name: "successful embedding",
			text: "test text",
			mockResponse: &openAIResponse{
				Object: "list",
				Data: []struct {
					Object    string    `json:"object"`
					Embedding []float32 `json:"embedding"`
					Index     int       `json:"index"`
				}{
					{
						Object:    "embedding",
						Embedding: []float32{0.1, 0.2, 0.3, 0.4},
						Index:     0,
					},
				},
				Model: "text-embedding-3-small",
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
			name: "API error response",
			text: "test text",
			mockResponse: &openAIResponse{
				Object: "error",
			},
			mockStatusCode: http.StatusBadRequest,
			wantErr:        true,
			errMsg:         "OpenAI API error",
		},
		{
			name: "unauthorized error",
			text: "test text",
			mockResponse: &openAIResponse{
				Object: "error",
			},
			mockStatusCode: http.StatusUnauthorized,
			wantErr:        true,
			errMsg:         "API error",
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
				assert.Contains(t, r.Header.Get("Authorization"), "Bearer ")

				// Verify URL path
				assert.Equal(t, "/embeddings", r.URL.Path)

				// Verify request body
				if tt.text != "" {
					var reqBody openAIRequest
					err := json.NewDecoder(r.Body).Decode(&reqBody)
					require.NoError(t, err)
					assert.Equal(t, tt.text, reqBody.Input)
				}

				// Send mock response
				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
				} else {
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"error": map[string]string{
							"message": "test error",
							"type":    "invalid_request_error",
						},
					})
				}
			}))
			defer server.Close()

			// Create service with mock server
			config := Config{
				OpenAI: &OpenAIConfig{
					APIKey:  "test-key",
					Model:   "text-embedding-3-small",
					BaseURL: server.URL,
				},
			}

			service, err := NewOpenAI(config)
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

// TestOpenAIEmbedBatch tests batch text embedding.
func TestOpenAIEmbedBatch(t *testing.T) {
	tests := []struct {
		name           string
		texts          []string
		mockResponse   *openAIResponse
		mockStatusCode int
		wantErr        bool
		errMsg         string
	}{
		{
			name:  "successful batch embedding",
			texts: []string{"text1", "text2", "text3"},
			mockResponse: &openAIResponse{
				Object: "list",
				Data: []struct {
					Object    string    `json:"object"`
					Embedding []float32 `json:"embedding"`
					Index     int       `json:"index"`
				}{
					{Object: "embedding", Embedding: []float32{0.1, 0.2, 0.3}, Index: 0},
					{Object: "embedding", Embedding: []float32{0.4, 0.5, 0.6}, Index: 1},
					{Object: "embedding", Embedding: []float32{0.7, 0.8, 0.9}, Index: 2},
				},
				Model: "text-embedding-3-small",
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
			name:  "API error",
			texts: []string{"text1", "text2"},
			mockResponse: &openAIResponse{
				Object: "error",
			},
			mockStatusCode: http.StatusBadRequest,
			wantErr:        true,
			errMsg:         "OpenAI API error",
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
					var reqBody openAIRequest
					err := json.NewDecoder(r.Body).Decode(&reqBody)
					require.NoError(t, err)
				}

				// Send mock response
				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
				} else {
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"error": map[string]string{
							"message": "test error",
							"type":    "invalid_request_error",
						},
					})
				}
			}))
			defer server.Close()

			// Create service with mock server
			config := Config{
				OpenAI: &OpenAIConfig{
					APIKey:  "test-key",
					Model:   "text-embedding-3-small",
					BaseURL: server.URL,
				},
			}

			service, err := NewOpenAI(config)
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

// TestOpenAIDimensions tests getting embedding dimensions.
func TestOpenAIDimensions(t *testing.T) {
	tests := []struct {
		name              string
		model             string
		customDimensions  int
		expectedDimension int
	}{
		{"ada-002", "text-embedding-ada-002", 0, 1536},
		{"3-small default", "text-embedding-3-small", 0, 1536},
		{"3-large default", "text-embedding-3-large", 0, 3072},
		{"3-small custom", "text-embedding-3-small", 512, 512},
		{"3-large custom", "text-embedding-3-large", 1024, 1024},
		{"unknown model", "unknown-model", 0, 1536}, // default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				OpenAI: &OpenAIConfig{
					APIKey:     "test-key",
					Model:      tt.model,
					Dimensions: tt.customDimensions,
				},
			}

			service, err := NewOpenAI(config)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedDimension, service.Dimensions())
		})
	}
}

// TestOpenAIModelName tests getting model name.
func TestOpenAIModelName(t *testing.T) {
	model := "text-embedding-3-small"
	config := Config{
		OpenAI: &OpenAIConfig{
			APIKey: "test-key",
			Model:  model,
		},
	}

	service, err := NewOpenAI(config)
	require.NoError(t, err)

	assert.Equal(t, model, service.ModelName())
}

// TestOpenAIClose tests closing the service.
func TestOpenAIClose(t *testing.T) {
	config := Config{
		OpenAI: &OpenAIConfig{
			APIKey: "test-key",
			Model:  "text-embedding-3-small",
		},
	}

	service, err := NewOpenAI(config)
	require.NoError(t, err)

	err = service.Close()
	assert.NoError(t, err)
}

// TestOpenAIWithCustomDimensions tests using custom dimensions.
func TestOpenAIWithCustomDimensions(t *testing.T) {
	customDims := 512

	// Create mock server that verifies dimensions parameter
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody openAIRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		// Verify dimensions parameter
		assert.NotNil(t, reqBody.Dimensions)
		assert.Equal(t, customDims, *reqBody.Dimensions)

		// Send mock response
		w.WriteHeader(http.StatusOK)
		mockResp := openAIResponse{
			Object: "list",
			Data: []struct {
				Object    string    `json:"object"`
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{
					Object:    "embedding",
					Embedding: make([]float32, customDims),
					Index:     0,
				},
			},
			Model: "text-embedding-3-small",
		}
		_ = json.NewEncoder(w).Encode(mockResp)
	}))
	defer server.Close()

	// Create service with custom dimensions
	config := Config{
		OpenAI: &OpenAIConfig{
			APIKey:     "test-key",
			Model:      "text-embedding-3-small",
			BaseURL:    server.URL,
			Dimensions: customDims,
		},
	}

	service, err := NewOpenAI(config)
	require.NoError(t, err)

	// Test embedding
	ctx := context.Background()
	embedding, err := service.Embed(ctx, "test text")
	require.NoError(t, err)
	assert.Len(t, embedding, customDims)
}

// TestOpenAIWithoutCustomDimensions tests that non-embedding-3 models don't set dimensions.
func TestOpenAIWithoutCustomDimensions(t *testing.T) {
	// Create mock server that verifies no dimensions parameter
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody openAIRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		// Verify no dimensions parameter for ada-002
		assert.Nil(t, reqBody.Dimensions)

		// Send mock response
		w.WriteHeader(http.StatusOK)
		mockResp := openAIResponse{
			Object: "list",
			Data: []struct {
				Object    string    `json:"object"`
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{
					Object:    "embedding",
					Embedding: make([]float32, 1536),
					Index:     0,
				},
			},
			Model: "text-embedding-ada-002",
		}
		_ = json.NewEncoder(w).Encode(mockResp)
	}))
	defer server.Close()

	// Create service with ada-002
	config := Config{
		OpenAI: &OpenAIConfig{
			APIKey:  "test-key",
			Model:   "text-embedding-ada-002",
			BaseURL: server.URL,
		},
	}

	service, err := NewOpenAI(config)
	require.NoError(t, err)

	// Test embedding
	ctx := context.Background()
	_, err = service.Embed(ctx, "test text")
	require.NoError(t, err)
}

// TestOpenAIContextCancellation tests context cancellation.
func TestOpenAIContextCancellation(t *testing.T) {
	// Create mock server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(openAIResponse{})
	}))
	defer server.Close()

	config := Config{
		OpenAI: &OpenAIConfig{
			APIKey:  "test-key",
			Model:   "text-embedding-3-small",
			BaseURL: server.URL,
		},
	}

	service, err := NewOpenAI(config)
	require.NoError(t, err)

	// Create context with immediate cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Test embedding with cancelled context
	_, err = service.Embed(ctx, "test text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

// TestOpenAITimeout tests request timeout.
func TestOpenAITimeout(t *testing.T) {
	// Create mock server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(openAIResponse{})
	}))
	defer server.Close()

	config := Config{
		OpenAI: &OpenAIConfig{
			APIKey:  "test-key",
			Model:   "text-embedding-3-small",
			BaseURL: server.URL,
		},
	}

	service, err := NewOpenAI(config)
	require.NoError(t, err)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Test embedding with timeout
	_, err = service.Embed(ctx, "test text")
	require.Error(t, err)
}

// TestOpenAIInvalidJSON tests handling of invalid JSON responses.
func TestOpenAIInvalidJSON(t *testing.T) {
	// Create mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	config := Config{
		OpenAI: &OpenAIConfig{
			APIKey:  "test-key",
			Model:   "text-embedding-3-small",
			BaseURL: server.URL,
		},
	}

	service, err := NewOpenAI(config)
	require.NoError(t, err)

	// Test embedding
	ctx := context.Background()
	_, err = service.Embed(ctx, "test text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
}

// TestOpenAIErrorResponse tests handling of API error responses.
func TestOpenAIErrorResponse(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"message": "Invalid API key",
				"type":    "invalid_request_error",
			},
		})
	}))
	defer server.Close()

	config := Config{
		OpenAI: &OpenAIConfig{
			APIKey:  "invalid-key",
			Model:   "text-embedding-3-small",
			BaseURL: server.URL,
		},
	}

	service, err := NewOpenAI(config)
	require.NoError(t, err)

	// Test embedding
	ctx := context.Background()
	_, err = service.Embed(ctx, "test text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid API key")
}

// TestOpenAIInvalidEmbeddingIndex tests handling of invalid embedding indices.
func TestOpenAIInvalidEmbeddingIndex(t *testing.T) {
	// Create mock server that returns invalid index
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		mockResp := openAIResponse{
			Object: "list",
			Data: []struct {
				Object    string    `json:"object"`
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{
					Object:    "embedding",
					Embedding: []float32{0.1, 0.2, 0.3},
					Index:     999, // Invalid index
				},
			},
			Model: "text-embedding-3-small",
		}
		_ = json.NewEncoder(w).Encode(mockResp)
	}))
	defer server.Close()

	config := Config{
		OpenAI: &OpenAIConfig{
			APIKey:  "test-key",
			Model:   "text-embedding-3-small",
			BaseURL: server.URL,
		},
	}

	service, err := NewOpenAI(config)
	require.NoError(t, err)

	// Test embedding
	ctx := context.Background()
	_, err = service.Embed(ctx, "test text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedding index out of bounds")
}

// TestGetOpenAIModelDimensions tests the model dimensions lookup.
func TestGetOpenAIModelDimensions(t *testing.T) {
	tests := []struct {
		model    string
		expected int
	}{
		{"text-embedding-ada-002", 1536},
		{"text-embedding-3-small", 1536},
		{"text-embedding-3-large", 3072},
		{"unknown-model", 1536}, // default
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			dims := getOpenAIModelDimensions(tt.model)
			assert.Equal(t, tt.expected, dims)
		})
	}
}

// TestIsTextEmbedding3Model tests the text-embedding-3 model check.
func TestIsTextEmbedding3Model(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"text-embedding-3-small", true},
		{"text-embedding-3-large", true},
		{"text-embedding-ada-002", false},
		{"unknown-model", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := isTextEmbedding3Model(tt.model)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestOpenAIEmptyResponse tests handling of empty response.
func TestOpenAIEmptyResponse(t *testing.T) {
	// Create mock server that returns empty data
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(openAIResponse{
			Object: "list",
			Data:   []struct {
				Object    string    `json:"object"`
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{},
			Model: "text-embedding-3-small",
		})
	}))
	defer server.Close()

	config := Config{
		OpenAI: &OpenAIConfig{
			APIKey:  "test-key",
			Model:   "text-embedding-3-small",
			BaseURL: server.URL,
		},
	}

	service, err := NewOpenAI(config)
	require.NoError(t, err)

	// Test embedding
	ctx := context.Background()
	_, err = service.Embed(ctx, "test text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no embeddings returned")
}

// BenchmarkOpenAIEmbed benchmarks single embedding.
func BenchmarkOpenAIEmbed(b *testing.B) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		embedding := make([]float32, 1536)
		for i := range embedding {
			embedding[i] = float32(i) * 0.001
		}
		mockResp := openAIResponse{
			Object: "list",
			Data: []struct {
				Object    string    `json:"object"`
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{Object: "embedding", Embedding: embedding, Index: 0},
			},
			Model: "text-embedding-3-small",
		}
		_ = json.NewEncoder(w).Encode(mockResp)
	}))
	defer server.Close()

	config := Config{
		OpenAI: &OpenAIConfig{
			APIKey:  "test-key",
			Model:   "text-embedding-3-small",
			BaseURL: server.URL,
		},
	}

	service, err := NewOpenAI(config)
	if err != nil {
		b.Fatalf("NewOpenAI error: %v", err)
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.Embed(ctx, "benchmark text")
	}
}

// BenchmarkOpenAIEmbedBatch benchmarks batch embedding.
func BenchmarkOpenAIEmbedBatch(b *testing.B) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		mockResp := openAIResponse{
			Object: "list",
			Data:   make([]struct {
				Object    string    `json:"object"`
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}, 10),
			Model: "text-embedding-3-small",
		}
		for i := range mockResp.Data {
			embedding := make([]float32, 1536)
			for j := range embedding {
				embedding[j] = float32(j) * 0.001
			}
			mockResp.Data[i] = struct {
				Object    string    `json:"object"`
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{Object: "embedding", Embedding: embedding, Index: i}
		}
		_ = json.NewEncoder(w).Encode(mockResp)
	}))
	defer server.Close()

	config := Config{
		OpenAI: &OpenAIConfig{
			APIKey:  "test-key",
			Model:   "text-embedding-3-small",
			BaseURL: server.URL,
		},
	}

	service, err := NewOpenAI(config)
	if err != nil {
		b.Fatalf("NewOpenAI error: %v", err)
	}
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
