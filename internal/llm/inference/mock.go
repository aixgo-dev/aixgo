package inference

import (
	"context"
	"fmt"
	"sync"
)

// MockInferenceService provides a mock inference service for testing and development
// This should be replaced with real implementations (Ollama, vLLM, HuggingFace API, etc.)
type MockInferenceService struct {
	model     string
	available bool
	mu        sync.RWMutex
}

// NewMockInferenceService creates a new mock inference service
func NewMockInferenceService(model string) *MockInferenceService {
	return &MockInferenceService{
		model:     model,
		available: true,
	}
}

// Generate returns a mock response
func (m *MockInferenceService) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	// Check context first
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	m.mu.RLock()
	available := m.available
	m.mu.RUnlock()

	if !available {
		return nil, fmt.Errorf("mock inference service not available")
	}

	// Return a mock response indicating this is a placeholder
	return &GenerateResponse{
		Text:         fmt.Sprintf("Mock response for model %s. Please implement a real inference service (Ollama, vLLM, or HuggingFace API).", m.model),
		FinishReason: "stop",
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}, nil
}

// Available returns whether the service is available
func (m *MockInferenceService) Available() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.available
}

// SetAvailable sets the availability status (for testing)
func (m *MockInferenceService) SetAvailable(available bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.available = available
}
