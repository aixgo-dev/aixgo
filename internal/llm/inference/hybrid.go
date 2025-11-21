package inference

import (
	"context"
	"fmt"
	"log"
	"sync"
)

// InferenceService defines the interface for LLM inference
type InferenceService interface {
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
	Available() bool
}

// GenerateRequest represents an inference request
type GenerateRequest struct {
	Model       string
	Prompt      string
	MaxTokens   int
	Temperature float64
	Stop        []string
}

// GenerateResponse represents an inference response
type GenerateResponse struct {
	Text         string
	FinishReason string
	Usage        Usage
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// HybridInference provides inference with fallback strategy
type HybridInference struct {
	local       InferenceService // Ollama or local model
	cloud       InferenceService // Cloud API (xAI, OpenAI, etc.)
	preferLocal bool
	mu          sync.RWMutex
}

// NewHybridInference creates a new hybrid inference service
func NewHybridInference(local, cloud InferenceService) *HybridInference {
	return &HybridInference{
		local:       local,
		cloud:       cloud,
		preferLocal: true,
	}
}

// Generate attempts local inference first, falls back to cloud
func (h *HybridInference) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	// Check context first
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	h.mu.RLock()
	preferLocal := h.preferLocal
	h.mu.RUnlock()

	// Try local first if available and preferred
	if preferLocal && h.local != nil && h.local.Available() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		resp, err := h.local.Generate(ctx, req)
		if err == nil {
			log.Printf("Used local inference for model: %s", req.Model)
			return resp, nil
		}
		log.Printf("Local inference failed: %v, falling back to cloud", err)
	}

	// Check context before fallback
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Fallback to cloud
	if h.cloud == nil {
		return nil, fmt.Errorf("no inference service available")
	}

	if !h.cloud.Available() {
		return nil, fmt.Errorf("cloud inference not available")
	}

	log.Printf("Using cloud inference for model: %s", req.Model)
	return h.cloud.Generate(ctx, req)
}

// Available returns true if any inference service is available
func (h *HybridInference) Available() bool {
	if h.local != nil && h.local.Available() {
		return true
	}
	if h.cloud != nil && h.cloud.Available() {
		return true
	}
	return false
}

// SetPreferLocal sets whether to prefer local inference
func (h *HybridInference) SetPreferLocal(prefer bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.preferLocal = prefer
}
