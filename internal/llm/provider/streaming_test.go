package provider

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestSimulatedStream(t *testing.T) {
	content := "Hello, this is a test response from the LLM."
	stream := NewSimulatedStream(content, "stop", 10)

	var result strings.Builder
	var lastChunk *StreamChunk

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		result.WriteString(chunk.Delta)
		lastChunk = chunk
	}

	if result.String() != content {
		t.Errorf("expected content %q, got %q", content, result.String())
	}

	if lastChunk == nil || lastChunk.FinishReason != "stop" {
		t.Errorf("expected last chunk to have finish_reason 'stop'")
	}
}

func TestSimulatedStreamClose(t *testing.T) {
	stream := NewSimulatedStream("test content", "stop", 5)

	// Read one chunk
	_, err := stream.Recv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Close stream
	if err := stream.Close(); err != nil {
		t.Fatalf("close error: %v", err)
	}

	// Subsequent reads should return EOF
	_, err = stream.Recv()
	if err != io.EOF {
		t.Errorf("expected EOF after close, got %v", err)
	}
}

func TestHuggingFaceStream(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream := NewHuggingFaceStream(ctx)

	// Send chunks in goroutine
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = stream.SendChunk(&StreamChunk{Delta: "Hello "})
		_ = stream.SendChunk(&StreamChunk{Delta: "World"})
		_ = stream.SendChunk(&StreamChunk{Delta: "!", FinishReason: "stop"})
	}()

	// Wait for sender to complete
	<-done

	var result strings.Builder
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Break on context error which happens after all chunks received
			break
		}
		result.WriteString(chunk.Delta)
		if chunk.FinishReason == "stop" {
			break
		}
	}
	_ = stream.Close()

	expected := "Hello World!"
	if result.String() != expected {
		t.Errorf("expected %q, got %q", expected, result.String())
	}
}

func TestStreamCollector(t *testing.T) {
	stream := NewSimulatedStream("Collected content", "stop", 5)
	collector := NewStreamCollector()

	response, err := collector.Collect(stream)
	if err != nil {
		t.Fatalf("collect error: %v", err)
	}

	if response.Content != "Collected content" {
		t.Errorf("expected 'Collected content', got %q", response.Content)
	}

	if response.FinishReason != "stop" {
		t.Errorf("expected finish_reason 'stop', got %q", response.FinishReason)
	}

	chunks := collector.GetChunks()
	if len(chunks) == 0 {
		t.Error("expected some chunks")
	}
}

func TestSSEStreamParser(t *testing.T) {
	sseData := `data: {"token": {"text": "Hello"}}

data: {"token": {"text": " World"}}

data: [DONE]

`
	parser := NewSSEStreamParser(strings.NewReader(sseData))

	// First event
	event1, err := parser.ReadEvent()
	if err != nil {
		t.Fatalf("read event 1: %v", err)
	}
	if event1.Data != `{"token": {"text": "Hello"}}` {
		t.Errorf("unexpected event 1 data: %q", event1.Data)
	}

	// Second event
	event2, err := parser.ReadEvent()
	if err != nil {
		t.Fatalf("read event 2: %v", err)
	}
	if event2.Data != `{"token": {"text": " World"}}` {
		t.Errorf("unexpected event 2 data: %q", event2.Data)
	}

	// Done event
	event3, err := parser.ReadEvent()
	if err != nil {
		t.Fatalf("read event 3: %v", err)
	}
	if event3.Data != "[DONE]" {
		t.Errorf("unexpected event 3 data: %q", event3.Data)
	}
}

func TestParseHuggingFaceStreamChunk(t *testing.T) {
	tests := []struct {
		name       string
		data       string
		wantDelta  string
		wantFinish string
		wantErr    bool
	}{
		{
			name:       "token chunk",
			data:       `{"token": {"id": 1, "text": "Hello", "logprob": -0.5, "special": false}}`,
			wantDelta:  "Hello",
			wantFinish: "",
		},
		{
			name:       "done marker",
			data:       "[DONE]",
			wantDelta:  "",
			wantFinish: "stop",
		},
		{
			name:    "invalid json",
			data:    "not json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, err := ParseHuggingFaceStreamChunk(tt.data)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if chunk.Delta != tt.wantDelta {
				t.Errorf("expected delta %q, got %q", tt.wantDelta, chunk.Delta)
			}
			if chunk.FinishReason != tt.wantFinish {
				t.Errorf("expected finish %q, got %q", tt.wantFinish, chunk.FinishReason)
			}
		})
	}
}
