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

// TestNewHuggingFaceTEI tests creating a new HuggingFace TEI embeddings service.
func TestNewHuggingFaceTEI(t *testing.T) {
	tests := []struct {
		name          string
		config        Config
		mockResponse  [][]float32
		wantErr       bool
		errMsg        string
		skipDimsProbe bool
	}{
		{
			name: "valid config with successful probe",
			config: Config{
				HuggingFaceTEI: &HuggingFaceTEIConfig{
					Endpoint: "http://localhost:8080",
				},
			},
			mockResponse:  [][]float32{{0.1, 0.2, 0.3, 0.4}},
			wantErr:       false,
			skipDimsProbe: false,
		},
		{
			name: "missing config",
			config: Config{
				HuggingFaceTEI: nil,
			},
			wantErr: true,
			errMsg:  "huggingface_tei configuration is required",
		},
		{
			name: "with normalize flag",
			config: Config{
				HuggingFaceTEI: &HuggingFaceTEIConfig{
					Endpoint:  "http://localhost:8080",
					Normalize: true,
				},
			},
			mockResponse:  [][]float32{{0.1, 0.2, 0.3, 0.4}},
			wantErr:       false,
			skipDimsProbe: false,
		},
		{
			name: "with model name",
			config: Config{
				HuggingFaceTEI: &HuggingFaceTEIConfig{
					Endpoint: "http://localhost:8080",
					Model:    "BAAI/bge-large-en-v1.5",
				},
			},
			mockResponse:  [][]float32{{0.1, 0.2, 0.3, 0.4}},
			wantErr:       false,
			skipDimsProbe: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			if !tt.skipDimsProbe && tt.config.HuggingFaceTEI != nil {
				// Create mock server for dimension probe
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
				}))
				defer server.Close()
				tt.config.HuggingFaceTEI.Endpoint = server.URL
			}

			service, err := NewHuggingFaceTEI(tt.config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, service)
			}
		})
	}
}

// TestHuggingFaceTEIEmbed tests single text embedding.
func TestHuggingFaceTEIEmbed(t *testing.T) {
	tests := []struct {
		name           string
		text           string
		mockResponse   [][]float32
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
			mockResponse:   nil,
			mockStatusCode: http.StatusInternalServerError,
			wantErr:        true,
			errMsg:         "TEI API error",
		},
		{
			name:           "bad request",
			text:           "test text",
			mockResponse:   nil,
			mockStatusCode: http.StatusBadRequest,
			wantErr:        true,
			errMsg:         "TEI API error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Track call count to handle probe call
			callCount := 0

			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++

				// Verify request method
				assert.Equal(t, "POST", r.Method)

				// Verify headers
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				// Verify URL path
				assert.Equal(t, "/embed", r.URL.Path)

				// First call is the probe (with "test" text), subsequent calls are actual tests
				if callCount > 1 && tt.text != "" {
					var reqBody teiRequest
					err := json.NewDecoder(r.Body).Decode(&reqBody)
					require.NoError(t, err)
					assert.Equal(t, tt.text, reqBody.Inputs)
				}

				// Send mock response
				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
				} else {
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "test error"})
				}
			}))
			defer server.Close()

			// Create service with mock server
			config := Config{
				HuggingFaceTEI: &HuggingFaceTEIConfig{
					Endpoint: server.URL,
				},
			}

			service, err := NewHuggingFaceTEI(config)
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

// TestHuggingFaceTEIEmbedBatch tests batch text embedding.
func TestHuggingFaceTEIEmbedBatch(t *testing.T) {
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
			errMsg:         "TEI API error",
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
					var reqBody teiRequest
					err := json.NewDecoder(r.Body).Decode(&reqBody)
					require.NoError(t, err)
				}

				// Send mock response
				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
				} else {
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "test error"})
				}
			}))
			defer server.Close()

			// Create service with mock server
			config := Config{
				HuggingFaceTEI: &HuggingFaceTEIConfig{
					Endpoint: server.URL,
				},
			}

			service, err := NewHuggingFaceTEI(config)
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

// TestHuggingFaceTEIDimensions tests getting embedding dimensions.
func TestHuggingFaceTEIDimensions(t *testing.T) {
	// Create mock server that returns embeddings of specific size
	dimensions := 768
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		embedding := make([]float32, dimensions)
		for i := range embedding {
			embedding[i] = float32(i) * 0.001
		}
		_ = json.NewEncoder(w).Encode([][]float32{embedding})
	}))
	defer server.Close()

	config := Config{
		HuggingFaceTEI: &HuggingFaceTEIConfig{
			Endpoint: server.URL,
		},
	}

	service, err := NewHuggingFaceTEI(config)
	require.NoError(t, err)

	// Dimensions should be set from probe
	assert.Equal(t, dimensions, service.Dimensions())
}

// TestHuggingFaceTEIModelName tests getting model name.
func TestHuggingFaceTEIModelName(t *testing.T) {
	tests := []struct {
		name          string
		configModel   string
		expectedModel string
	}{
		{
			name:          "with model name",
			configModel:   "BAAI/bge-large-en-v1.5",
			expectedModel: "BAAI/bge-large-en-v1.5",
		},
		{
			name:          "without model name",
			configModel:   "",
			expectedModel: "huggingface-tei",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode([][]float32{{0.1, 0.2, 0.3}})
			}))
			defer server.Close()

			config := Config{
				HuggingFaceTEI: &HuggingFaceTEIConfig{
					Endpoint: server.URL,
					Model:    tt.configModel,
				},
			}

			service, err := NewHuggingFaceTEI(config)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedModel, service.ModelName())
		})
	}
}

// TestHuggingFaceTEIClose tests closing the service.
func TestHuggingFaceTEIClose(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([][]float32{{0.1, 0.2, 0.3}})
	}))
	defer server.Close()

	config := Config{
		HuggingFaceTEI: &HuggingFaceTEIConfig{
			Endpoint: server.URL,
		},
	}

	service, err := NewHuggingFaceTEI(config)
	require.NoError(t, err)

	err = service.Close()
	assert.NoError(t, err)
}

// TestHuggingFaceTEIWithNormalize tests using normalize flag.
func TestHuggingFaceTEIWithNormalize(t *testing.T) {
	// Create mock server that verifies normalize parameter
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody teiRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		// Verify normalize parameter
		assert.NotNil(t, reqBody.Normalize)
		assert.True(t, *reqBody.Normalize)

		// Send mock response
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([][]float32{{0.1, 0.2, 0.3}})
	}))
	defer server.Close()

	// Create service with normalize flag
	config := Config{
		HuggingFaceTEI: &HuggingFaceTEIConfig{
			Endpoint:  server.URL,
			Normalize: true,
		},
	}

	service, err := NewHuggingFaceTEI(config)
	require.NoError(t, err)

	// Test embedding
	ctx := context.Background()
	_, err = service.Embed(ctx, "test text")
	require.NoError(t, err)
}

// TestHuggingFaceTEIWithoutNormalize tests without normalize flag.
func TestHuggingFaceTEIWithoutNormalize(t *testing.T) {
	// Create mock server that verifies no normalize parameter
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody teiRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		// Verify no normalize parameter
		assert.Nil(t, reqBody.Normalize)

		// Send mock response
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([][]float32{{0.1, 0.2, 0.3}})
	}))
	defer server.Close()

	// Create service without normalize flag
	config := Config{
		HuggingFaceTEI: &HuggingFaceTEIConfig{
			Endpoint:  server.URL,
			Normalize: false,
		},
	}

	service, err := NewHuggingFaceTEI(config)
	require.NoError(t, err)

	// Test embedding
	ctx := context.Background()
	_, err = service.Embed(ctx, "test text")
	require.NoError(t, err)
}

// TestHuggingFaceTEIContextCancellation tests context cancellation.
func TestHuggingFaceTEIContextCancellation(t *testing.T) {
	// Create mock server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([][]float32{{0.1, 0.2, 0.3}})
	}))
	defer server.Close()

	config := Config{
		HuggingFaceTEI: &HuggingFaceTEIConfig{
			Endpoint: server.URL,
		},
	}

	service, err := NewHuggingFaceTEI(config)
	require.NoError(t, err)

	// Create context with immediate cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Test embedding with cancelled context
	_, err = service.Embed(ctx, "test text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

// TestHuggingFaceTEITimeout tests request timeout.
func TestHuggingFaceTEITimeout(t *testing.T) {
	// Create mock server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([][]float32{{0.1, 0.2, 0.3}})
	}))
	defer server.Close()

	config := Config{
		HuggingFaceTEI: &HuggingFaceTEIConfig{
			Endpoint: server.URL,
		},
	}

	service, err := NewHuggingFaceTEI(config)
	require.NoError(t, err)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Test embedding with timeout
	_, err = service.Embed(ctx, "test text")
	require.Error(t, err)
}

// TestHuggingFaceTEIInvalidJSON tests handling of invalid JSON responses.
func TestHuggingFaceTEIInvalidJSON(t *testing.T) {
	// Create mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	config := Config{
		HuggingFaceTEI: &HuggingFaceTEIConfig{
			Endpoint: server.URL,
		},
	}

	service, err := NewHuggingFaceTEI(config)
	require.NoError(t, err)

	// Test embedding
	ctx := context.Background()
	_, err = service.Embed(ctx, "test text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
}

// TestHuggingFaceTEIEmptyResponse tests handling of empty response.
func TestHuggingFaceTEIEmptyResponse(t *testing.T) {
	// Create mock server that returns empty array
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([][]float32{})
	}))
	defer server.Close()

	config := Config{
		HuggingFaceTEI: &HuggingFaceTEIConfig{
			Endpoint: server.URL,
		},
	}

	service, err := NewHuggingFaceTEI(config)
	require.NoError(t, err)

	// Test embedding
	ctx := context.Background()
	_, err = service.Embed(ctx, "test text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no embeddings returned")
}

// TestHuggingFaceTEIProbeDimensionsFailure tests handling probe failure.
func TestHuggingFaceTEIProbeDimensionsFailure(t *testing.T) {
	// Create mock server that fails probe but succeeds later
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First call (probe) fails
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Subsequent calls succeed
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([][]float32{{0.1, 0.2, 0.3}})
	}))
	defer server.Close()

	config := Config{
		HuggingFaceTEI: &HuggingFaceTEIConfig{
			Endpoint: server.URL,
		},
	}

	// Service should still be created even if probe fails
	service, err := NewHuggingFaceTEI(config)
	require.NoError(t, err)
	require.NotNil(t, service)

	// Dimensions should be 0 initially
	assert.Equal(t, 0, service.Dimensions())

	// Embedding should work and set dimensions
	ctx := context.Background()
	embedding, err := service.Embed(ctx, "test text")
	require.NoError(t, err)
	assert.NotEmpty(t, embedding)

	// Dimensions should now be set
	assert.Equal(t, 3, service.Dimensions())
}

// TestHuggingFaceTEIDimensionUpdateOnFirstEmbed tests dimension update.
func TestHuggingFaceTEIDimensionUpdateOnFirstEmbed(t *testing.T) {
	dimensions := 512
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		embedding := make([]float32, dimensions)
		for i := range embedding {
			embedding[i] = float32(i) * 0.001
		}
		_ = json.NewEncoder(w).Encode([][]float32{embedding})
	}))
	defer server.Close()

	config := Config{
		HuggingFaceTEI: &HuggingFaceTEIConfig{
			Endpoint: server.URL,
		},
	}

	service, err := NewHuggingFaceTEI(config)
	require.NoError(t, err)

	// Get embedding
	ctx := context.Background()
	embedding, err := service.Embed(ctx, "test text")
	require.NoError(t, err)
	assert.Len(t, embedding, dimensions)

	// Dimensions should be updated
	assert.Equal(t, dimensions, service.Dimensions())
}

// BenchmarkHuggingFaceTEIEmbed benchmarks single embedding.
func BenchmarkHuggingFaceTEIEmbed(b *testing.B) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		embedding := make([]float32, 768)
		for i := range embedding {
			embedding[i] = float32(i) * 0.001
		}
		_ = json.NewEncoder(w).Encode([][]float32{embedding})
	}))
	defer server.Close()

	config := Config{
		HuggingFaceTEI: &HuggingFaceTEIConfig{
			Endpoint: server.URL,
		},
	}

	service, _ := NewHuggingFaceTEI(config)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.Embed(ctx, "benchmark text")
	}
}

// BenchmarkHuggingFaceTEIEmbedBatch benchmarks batch embedding.
func BenchmarkHuggingFaceTEIEmbedBatch(b *testing.B) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		embeddings := make([][]float32, 10)
		for i := range embeddings {
			embeddings[i] = make([]float32, 768)
			for j := range embeddings[i] {
				embeddings[i][j] = float32(j) * 0.001
			}
		}
		_ = json.NewEncoder(w).Encode(embeddings)
	}))
	defer server.Close()

	config := Config{
		HuggingFaceTEI: &HuggingFaceTEIConfig{
			Endpoint: server.URL,
		},
	}

	service, _ := NewHuggingFaceTEI(config)
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
