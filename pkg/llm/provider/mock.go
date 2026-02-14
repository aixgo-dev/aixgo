package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// MockProvider is a mock LLM provider for testing
type MockProvider struct {
	name string

	// Responses to return for each request
	CompletionResponses []*CompletionResponse
	StructuredResponses []*StructuredResponse
	StreamChunks        [][]*StreamChunk
	Errors              []error

	// Track calls
	CompletionCalls []CompletionRequest
	StructuredCalls []StructuredRequest
	StreamCalls     []CompletionRequest

	currentIndex int
}

// NewMockProvider creates a new mock provider
func NewMockProvider(name string) *MockProvider {
	return &MockProvider{
		name:                name,
		CompletionResponses: []*CompletionResponse{},
		StructuredResponses: []*StructuredResponse{},
		StreamChunks:        [][]*StreamChunk{},
		Errors:              []error{},
		CompletionCalls:     []CompletionRequest{},
		StructuredCalls:     []StructuredRequest{},
		StreamCalls:         []CompletionRequest{},
		currentIndex:        0,
	}
}

// CreateCompletion implements Provider
func (m *MockProvider) CreateCompletion(ctx context.Context, request CompletionRequest) (*CompletionResponse, error) {
	m.CompletionCalls = append(m.CompletionCalls, request)

	// Check for errors first
	if m.currentIndex < len(m.Errors) && m.Errors[m.currentIndex] != nil {
		err := m.Errors[m.currentIndex]
		m.currentIndex++
		return nil, err
	}

	// Return response
	if m.currentIndex < len(m.CompletionResponses) {
		response := m.CompletionResponses[m.currentIndex]
		m.currentIndex++
		return response, nil
	}

	// Default response
	return &CompletionResponse{
		Content:      "Mock response",
		FinishReason: "stop",
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}, nil
}

// CreateStructured implements Provider
func (m *MockProvider) CreateStructured(ctx context.Context, request StructuredRequest) (*StructuredResponse, error) {
	m.StructuredCalls = append(m.StructuredCalls, request)

	// Check for errors first
	if m.currentIndex < len(m.Errors) && m.Errors[m.currentIndex] != nil {
		err := m.Errors[m.currentIndex]
		m.currentIndex++
		return nil, err
	}

	// Return response
	if m.currentIndex < len(m.StructuredResponses) {
		response := m.StructuredResponses[m.currentIndex]
		m.currentIndex++
		return response, nil
	}

	// Default response - return a simple JSON object
	defaultData, _ := json.Marshal(map[string]any{
		"message": "Mock structured response",
	})

	return &StructuredResponse{
		Data: defaultData,
		CompletionResponse: CompletionResponse{
			Content:      string(defaultData),
			FinishReason: "stop",
			Usage: Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		},
	}, nil
}

// CreateStreaming implements Provider
func (m *MockProvider) CreateStreaming(ctx context.Context, request CompletionRequest) (Stream, error) {
	m.StreamCalls = append(m.StreamCalls, request)

	// Check for errors first
	if m.currentIndex < len(m.Errors) && m.Errors[m.currentIndex] != nil {
		err := m.Errors[m.currentIndex]
		m.currentIndex++
		return nil, err
	}

	// Return stream
	var chunks []*StreamChunk
	if m.currentIndex < len(m.StreamChunks) {
		chunks = m.StreamChunks[m.currentIndex]
		m.currentIndex++
	} else {
		// Default stream
		chunks = []*StreamChunk{
			{Delta: "Mock "},
			{Delta: "stream "},
			{Delta: "response", FinishReason: "stop"},
		}
	}

	return &MockStream{chunks: chunks}, nil
}

// Name implements Provider
func (m *MockProvider) Name() string {
	return m.name
}

// AddCompletionResponse adds a completion response to return
func (m *MockProvider) AddCompletionResponse(response *CompletionResponse) *MockProvider {
	m.CompletionResponses = append(m.CompletionResponses, response)
	return m
}

// AddStructuredResponse adds a structured response to return
func (m *MockProvider) AddStructuredResponse(response *StructuredResponse) *MockProvider {
	m.StructuredResponses = append(m.StructuredResponses, response)
	return m
}

// AddStreamChunks adds stream chunks to return
func (m *MockProvider) AddStreamChunks(chunks []*StreamChunk) *MockProvider {
	m.StreamChunks = append(m.StreamChunks, chunks)
	return m
}

// AddError adds an error to return
func (m *MockProvider) AddError(err error) *MockProvider {
	m.Errors = append(m.Errors, err)
	return m
}

// Reset resets the mock provider
func (m *MockProvider) Reset() {
	m.CompletionResponses = []*CompletionResponse{}
	m.StructuredResponses = []*StructuredResponse{}
	m.StreamChunks = [][]*StreamChunk{}
	m.Errors = []error{}
	m.CompletionCalls = []CompletionRequest{}
	m.StructuredCalls = []StructuredRequest{}
	m.StreamCalls = []CompletionRequest{}
	m.currentIndex = 0
}

// MockStream is a mock stream implementation
type MockStream struct {
	chunks       []*StreamChunk
	currentIndex int
	closed       bool
}

// Recv implements Stream
func (s *MockStream) Recv() (*StreamChunk, error) {
	if s.closed {
		return nil, errors.New("stream closed")
	}

	if s.currentIndex >= len(s.chunks) {
		return nil, errors.New("no more chunks")
	}

	chunk := s.chunks[s.currentIndex]
	s.currentIndex++
	return chunk, nil
}

// Close implements Stream
func (s *MockStream) Close() error {
	if s.closed {
		return errors.New("stream already closed")
	}
	s.closed = true
	return nil
}

// Helper functions for creating mock responses

// MockCompletionResponse creates a mock completion response
func MockCompletionResponse(content string) *CompletionResponse {
	return &CompletionResponse{
		Content:      content,
		FinishReason: "stop",
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: len(content) / 4, // Rough token estimate
			TotalTokens:      10 + len(content)/4,
		},
	}
}

// MockStructuredResponse creates a mock structured response
func MockStructuredResponse(data any) *StructuredResponse {
	jsonData, err := json.Marshal(data)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal mock data: %v", err))
	}

	return &StructuredResponse{
		Data: jsonData,
		CompletionResponse: CompletionResponse{
			Content:      string(jsonData),
			FinishReason: "stop",
			Usage: Usage{
				PromptTokens:     10,
				CompletionTokens: len(jsonData) / 4,
				TotalTokens:      10 + len(jsonData)/4,
			},
		},
	}
}
