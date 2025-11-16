package agents

import (
	"context"
	"sync"

	"github.com/sashabaranov/go-openai"
)

// MockOpenAIClient is a mock implementation of OpenAIClient for testing
type MockOpenAIClient struct {
	responses []openai.ChatCompletionResponse
	errors    []error
	calls     []openai.ChatCompletionRequest
	callIndex int
	mu        sync.Mutex
}

// NewMockOpenAIClient creates a new mock OpenAI client
func NewMockOpenAIClient() *MockOpenAIClient {
	return &MockOpenAIClient{
		responses: make([]openai.ChatCompletionResponse, 0),
		errors:    make([]error, 0),
		calls:     make([]openai.ChatCompletionRequest, 0),
	}
}

// CreateChatCompletion implements OpenAIClient.CreateChatCompletion
func (m *MockOpenAIClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, req)

	if m.callIndex >= len(m.responses) {
		// Return empty response if no more responses configured
		return openai.ChatCompletionResponse{}, nil
	}

	resp := m.responses[m.callIndex]
	var err error
	if m.callIndex < len(m.errors) {
		err = m.errors[m.callIndex]
	}

	m.callIndex++
	return resp, err
}

// AddResponse adds a response to return from CreateChatCompletion
func (m *MockOpenAIClient) AddResponse(resp openai.ChatCompletionResponse, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.responses = append(m.responses, resp)
	m.errors = append(m.errors, err)
}

// GetCalls returns all recorded calls to CreateChatCompletion
func (m *MockOpenAIClient) GetCalls() []openai.ChatCompletionRequest {
	m.mu.Lock()
	defer m.mu.Unlock()

	calls := make([]openai.ChatCompletionRequest, len(m.calls))
	copy(calls, m.calls)
	return calls
}

// Reset resets the mock state
func (m *MockOpenAIClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.responses = make([]openai.ChatCompletionResponse, 0)
	m.errors = make([]error, 0)
	m.calls = make([]openai.ChatCompletionRequest, 0)
	m.callIndex = 0
}
