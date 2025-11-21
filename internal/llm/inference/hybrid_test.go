package inference

import (
	"context"
	"errors"
	"testing"
)

func TestNewHybridInference(t *testing.T) {
	local := &mockInferenceService{available: true}
	cloud := &mockInferenceService{available: true}

	hybrid := NewHybridInference(local, cloud)

	if hybrid == nil {
		t.Fatal("NewHybridInference() returned nil")
		return
	}
	if hybrid.local != local {
		t.Error("hybrid.local not set correctly")
	}
	if hybrid.cloud != cloud {
		t.Error("hybrid.cloud not set correctly")
	}
	if !hybrid.preferLocal {
		t.Error("hybrid.preferLocal = false, want true")
	}
}

func TestHybridInference_Generate_LocalSuccess(t *testing.T) {
	local := &mockInferenceService{
		available: true,
		response: &GenerateResponse{
			Text:         "Local response",
			FinishReason: "stop",
			Usage: Usage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		},
	}
	cloud := &mockInferenceService{
		available: true,
		response: &GenerateResponse{
			Text:         "Cloud response",
			FinishReason: "stop",
		},
	}

	hybrid := NewHybridInference(local, cloud)
	ctx := context.Background()

	req := GenerateRequest{
		Model:       "test-model",
		Prompt:      "Test prompt",
		MaxTokens:   100,
		Temperature: 0.7,
	}

	resp, err := hybrid.Generate(ctx, req)
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}

	if resp.Text != "Local response" {
		t.Errorf("Generate() text = %q, want 'Local response'", resp.Text)
	}

	// Verify local was called
	if local.callCount != 1 {
		t.Errorf("local.callCount = %d, want 1", local.callCount)
	}

	// Verify cloud was NOT called
	if cloud.callCount != 0 {
		t.Errorf("cloud.callCount = %d, want 0", cloud.callCount)
	}
}

func TestHybridInference_Generate_LocalFallbackToCloud(t *testing.T) {
	local := &mockInferenceService{
		available: true,
		err:       errors.New("local inference failed"),
	}
	cloud := &mockInferenceService{
		available: true,
		response: &GenerateResponse{
			Text:         "Cloud response",
			FinishReason: "stop",
		},
	}

	hybrid := NewHybridInference(local, cloud)
	ctx := context.Background()

	req := GenerateRequest{
		Model:  "test-model",
		Prompt: "Test prompt",
	}

	resp, err := hybrid.Generate(ctx, req)
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}

	if resp.Text != "Cloud response" {
		t.Errorf("Generate() text = %q, want 'Cloud response'", resp.Text)
	}

	// Verify both were called
	if local.callCount != 1 {
		t.Errorf("local.callCount = %d, want 1", local.callCount)
	}
	if cloud.callCount != 1 {
		t.Errorf("cloud.callCount = %d, want 1", cloud.callCount)
	}
}

func TestHybridInference_Generate_LocalUnavailable(t *testing.T) {
	local := &mockInferenceService{
		available: false,
	}
	cloud := &mockInferenceService{
		available: true,
		response: &GenerateResponse{
			Text:         "Cloud response",
			FinishReason: "stop",
		},
	}

	hybrid := NewHybridInference(local, cloud)
	ctx := context.Background()

	req := GenerateRequest{
		Model:  "test-model",
		Prompt: "Test prompt",
	}

	resp, err := hybrid.Generate(ctx, req)
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}

	if resp.Text != "Cloud response" {
		t.Errorf("Generate() text = %q, want 'Cloud response'", resp.Text)
	}

	// Verify local was NOT called
	if local.callCount != 0 {
		t.Errorf("local.callCount = %d, want 0", local.callCount)
	}

	// Verify cloud was called
	if cloud.callCount != 1 {
		t.Errorf("cloud.callCount = %d, want 1", cloud.callCount)
	}
}

func TestHybridInference_Generate_PreferCloud(t *testing.T) {
	local := &mockInferenceService{
		available: true,
		response: &GenerateResponse{
			Text: "Local response",
		},
	}
	cloud := &mockInferenceService{
		available: true,
		response: &GenerateResponse{
			Text: "Cloud response",
		},
	}

	hybrid := NewHybridInference(local, cloud)
	hybrid.SetPreferLocal(false)

	ctx := context.Background()
	req := GenerateRequest{
		Model:  "test-model",
		Prompt: "Test prompt",
	}

	resp, err := hybrid.Generate(ctx, req)
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}

	if resp.Text != "Cloud response" {
		t.Errorf("Generate() text = %q, want 'Cloud response'", resp.Text)
	}

	// Verify cloud was called
	if cloud.callCount != 1 {
		t.Errorf("cloud.callCount = %d, want 1", cloud.callCount)
	}

	// Verify local was NOT called when preferLocal is false
	if local.callCount != 0 {
		t.Errorf("local.callCount = %d, want 0", local.callCount)
	}
}

func TestHybridInference_Generate_NoServicesAvailable(t *testing.T) {
	hybrid := NewHybridInference(nil, nil)
	ctx := context.Background()

	req := GenerateRequest{
		Model:  "test-model",
		Prompt: "Test prompt",
	}

	_, err := hybrid.Generate(ctx, req)
	if err == nil {
		t.Error("Generate() error = nil, want error")
	}
}

func TestHybridInference_Generate_CloudUnavailable(t *testing.T) {
	local := &mockInferenceService{
		available: true,
		err:       errors.New("local failed"),
	}
	cloud := &mockInferenceService{
		available: false,
	}

	hybrid := NewHybridInference(local, cloud)
	ctx := context.Background()

	req := GenerateRequest{
		Model:  "test-model",
		Prompt: "Test prompt",
	}

	_, err := hybrid.Generate(ctx, req)
	if err == nil {
		t.Error("Generate() error = nil, want error")
	}
}

func TestHybridInference_Generate_OnlyLocalAvailable(t *testing.T) {
	local := &mockInferenceService{
		available: true,
		response: &GenerateResponse{
			Text: "Local response",
		},
	}

	hybrid := NewHybridInference(local, nil)
	ctx := context.Background()

	req := GenerateRequest{
		Model:  "test-model",
		Prompt: "Test prompt",
	}

	resp, err := hybrid.Generate(ctx, req)
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}

	if resp.Text != "Local response" {
		t.Errorf("Generate() text = %q, want 'Local response'", resp.Text)
	}
}

func TestHybridInference_Generate_OnlyCloudAvailable(t *testing.T) {
	cloud := &mockInferenceService{
		available: true,
		response: &GenerateResponse{
			Text: "Cloud response",
		},
	}

	hybrid := NewHybridInference(nil, cloud)
	ctx := context.Background()

	req := GenerateRequest{
		Model:  "test-model",
		Prompt: "Test prompt",
	}

	resp, err := hybrid.Generate(ctx, req)
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}

	if resp.Text != "Cloud response" {
		t.Errorf("Generate() text = %q, want 'Cloud response'", resp.Text)
	}
}

func TestHybridInference_Available(t *testing.T) {
	tests := []struct {
		name           string
		localAvailable bool
		cloudAvailable bool
		want           bool
	}{
		{
			name:           "both available",
			localAvailable: true,
			cloudAvailable: true,
			want:           true,
		},
		{
			name:           "only local available",
			localAvailable: true,
			cloudAvailable: false,
			want:           true,
		},
		{
			name:           "only cloud available",
			localAvailable: false,
			cloudAvailable: true,
			want:           true,
		},
		{
			name:           "neither available",
			localAvailable: false,
			cloudAvailable: false,
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			local := &mockInferenceService{available: tt.localAvailable}
			cloud := &mockInferenceService{available: tt.cloudAvailable}

			hybrid := NewHybridInference(local, cloud)
			available := hybrid.Available()

			if available != tt.want {
				t.Errorf("Available() = %v, want %v", available, tt.want)
			}
		})
	}
}

func TestHybridInference_Available_NilServices(t *testing.T) {
	tests := []struct {
		name  string
		local InferenceService
		cloud InferenceService
		want  bool
	}{
		{
			name:  "both nil",
			local: nil,
			cloud: nil,
			want:  false,
		},
		{
			name:  "local nil, cloud available",
			local: nil,
			cloud: &mockInferenceService{available: true},
			want:  true,
		},
		{
			name:  "local available, cloud nil",
			local: &mockInferenceService{available: true},
			cloud: nil,
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hybrid := NewHybridInference(tt.local, tt.cloud)
			available := hybrid.Available()

			if available != tt.want {
				t.Errorf("Available() = %v, want %v", available, tt.want)
			}
		})
	}
}

func TestHybridInference_SetPreferLocal(t *testing.T) {
	hybrid := NewHybridInference(nil, nil)

	// Default should be true
	if !hybrid.preferLocal {
		t.Error("default preferLocal = false, want true")
	}

	// Set to false
	hybrid.SetPreferLocal(false)
	if hybrid.preferLocal {
		t.Error("SetPreferLocal(false) did not update preferLocal")
	}

	// Set to true
	hybrid.SetPreferLocal(true)
	if !hybrid.preferLocal {
		t.Error("SetPreferLocal(true) did not update preferLocal")
	}
}

func TestHybridInference_Generate_ContextPropagation(t *testing.T) {
	local := &mockInferenceService{
		available: true,
		response: &GenerateResponse{
			Text: "Response",
		},
	}

	hybrid := NewHybridInference(local, nil)

	// Create a context with a value
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "test-value")

	req := GenerateRequest{
		Model:  "test-model",
		Prompt: "Test prompt",
	}

	_, err := hybrid.Generate(ctx, req)
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}

	// Verify context was passed through
	if local.lastCtx == nil {
		t.Error("Context was not propagated to inference service")
	}

	if local.lastCtx.Value(key{}) != "test-value" {
		t.Error("Context value was not propagated correctly")
	}
}

// Mock inference service for testing
type mockInferenceService struct {
	available bool
	response  *GenerateResponse
	err       error
	callCount int
	lastCtx   context.Context
	lastReq   GenerateRequest
}

func (m *mockInferenceService) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	m.callCount++
	m.lastCtx = ctx
	m.lastReq = req

	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockInferenceService) Available() bool {
	return m.available
}
