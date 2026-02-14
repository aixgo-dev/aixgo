package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/aixgo-dev/aixgo/internal/llm/inference"
)

// StreamingInferenceService extends InferenceService with streaming support
type StreamingInferenceService interface {
	inference.InferenceService
	GenerateStream(ctx context.Context, req inference.GenerateRequest) (InferenceStream, error)
}

// InferenceStream represents a streaming inference response
type InferenceStream interface {
	Recv() (*inference.GenerateResponse, error)
	Close() error
}

// HuggingFaceStream implements the Stream interface for HuggingFace providers
type HuggingFaceStream struct {
	ctx       context.Context
	cancel    context.CancelFunc
	chunks    chan *StreamChunk
	err       error
	errMu     sync.Mutex
	closed    bool
	closeMu   sync.Mutex
	closeOnce sync.Once
}

// NewHuggingFaceStream creates a new streaming response handler
func NewHuggingFaceStream(ctx context.Context) *HuggingFaceStream {
	ctx, cancel := context.WithCancel(ctx)
	return &HuggingFaceStream{
		ctx:    ctx,
		cancel: cancel,
		chunks: make(chan *StreamChunk, 100),
	}
}

// Recv receives the next chunk from the stream
func (s *HuggingFaceStream) Recv() (*StreamChunk, error) {
	s.closeMu.Lock()
	if s.closed {
		s.closeMu.Unlock()
		return nil, io.EOF
	}
	s.closeMu.Unlock()

	select {
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	case chunk, ok := <-s.chunks:
		if !ok {
			s.errMu.Lock()
			err := s.err
			s.errMu.Unlock()
			if err != nil {
				return nil, err
			}
			return nil, io.EOF
		}
		return chunk, nil
	}
}

// Close closes the stream
func (s *HuggingFaceStream) Close() error {
	s.closeOnce.Do(func() {
		s.closeMu.Lock()
		s.closed = true
		s.closeMu.Unlock()
		s.cancel()
		close(s.chunks)
	})
	return nil
}

// SendChunk sends a chunk to the stream
func (s *HuggingFaceStream) SendChunk(chunk *StreamChunk) error {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()

	if s.closed {
		return errors.New("stream closed")
	}

	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	case s.chunks <- chunk:
		return nil
	default:
		// Channel full, try with context
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case s.chunks <- chunk:
			return nil
		}
	}
}

// SetError sets an error on the stream
func (s *HuggingFaceStream) SetError(err error) {
	s.errMu.Lock()
	s.err = err
	s.errMu.Unlock()
}

// SSEStreamParser parses Server-Sent Events (SSE) streams from HuggingFace API
type SSEStreamParser struct {
	reader *bufio.Reader
}

// NewSSEStreamParser creates a new SSE parser
func NewSSEStreamParser(r io.Reader) *SSEStreamParser {
	return &SSEStreamParser{
		reader: bufio.NewReader(r),
	}
}

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	Event string
	Data  string
	ID    string
}

// ReadEvent reads the next SSE event
func (p *SSEStreamParser) ReadEvent() (*SSEEvent, error) {
	event := &SSEEvent{}
	var dataLines []string

	for {
		line, err := p.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF && len(dataLines) > 0 {
				event.Data = strings.Join(dataLines, "\n")
				return event, nil
			}
			return nil, err
		}

		line = strings.TrimRight(line, "\r\n")

		// Empty line signals end of event
		if line == "" {
			if len(dataLines) > 0 || event.Event != "" {
				event.Data = strings.Join(dataLines, "\n")
				return event, nil
			}
			continue
		}

		// Parse field
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimPrefix(data, " ")
			dataLines = append(dataLines, data)
		} else if strings.HasPrefix(line, "event:") {
			event.Event = strings.TrimPrefix(line, "event:")
			event.Event = strings.TrimPrefix(event.Event, " ")
		} else if strings.HasPrefix(line, "id:") {
			event.ID = strings.TrimPrefix(line, "id:")
			event.ID = strings.TrimPrefix(event.ID, " ")
		}
	}
}

// HuggingFaceStreamResponse represents a streaming response chunk from HuggingFace
type HuggingFaceStreamResponse struct {
	Token struct {
		ID      int     `json:"id"`
		Text    string  `json:"text"`
		Logprob float64 `json:"logprob"`
		Special bool    `json:"special"`
	} `json:"token"`
	GeneratedText string `json:"generated_text,omitempty"`
	Details       *struct {
		FinishReason string `json:"finish_reason"`
		Tokens       int    `json:"generated_tokens"`
	} `json:"details,omitempty"`
}

// ParseHuggingFaceStreamChunk parses a HuggingFace streaming response chunk
func ParseHuggingFaceStreamChunk(data string) (*StreamChunk, error) {
	if data == "[DONE]" {
		return &StreamChunk{
			FinishReason: "stop",
		}, nil
	}

	var resp HuggingFaceStreamResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		return nil, fmt.Errorf("parse stream chunk: %w", err)
	}

	chunk := &StreamChunk{
		Delta: resp.Token.Text,
	}

	if resp.Details != nil {
		chunk.FinishReason = resp.Details.FinishReason
	}

	return chunk, nil
}

// SimulatedStream creates a simulated stream from a complete response
// This is useful for providers that don't support native streaming
type SimulatedStream struct {
	content      string
	position     int
	chunkSize    int
	finishReason string
	closed       bool
	mu           sync.Mutex
}

// NewSimulatedStream creates a stream that simulates streaming from a complete response
func NewSimulatedStream(content string, finishReason string, chunkSize int) *SimulatedStream {
	if chunkSize <= 0 {
		chunkSize = 10 // Default chunk size in characters
	}
	return &SimulatedStream{
		content:      content,
		chunkSize:    chunkSize,
		finishReason: finishReason,
	}
}

// Recv receives the next simulated chunk
func (s *SimulatedStream) Recv() (*StreamChunk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, io.EOF
	}

	if s.position >= len(s.content) {
		return nil, io.EOF
	}

	end := s.position + s.chunkSize
	if end > len(s.content) {
		end = len(s.content)
	}

	chunk := &StreamChunk{
		Delta: s.content[s.position:end],
	}
	s.position = end

	// Add finish reason on last chunk
	if s.position >= len(s.content) {
		chunk.FinishReason = s.finishReason
	}

	return chunk, nil
}

// Close closes the simulated stream
func (s *SimulatedStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// StreamCollector collects chunks from a stream into a complete response
type StreamCollector struct {
	chunks       []*StreamChunk
	content      strings.Builder
	finishReason string
}

// NewStreamCollector creates a new stream collector
func NewStreamCollector() *StreamCollector {
	return &StreamCollector{
		chunks: make([]*StreamChunk, 0),
	}
}

// Collect reads all chunks from a stream and returns the complete response
func (c *StreamCollector) Collect(stream Stream) (*CompletionResponse, error) {
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		c.chunks = append(c.chunks, chunk)
		c.content.WriteString(chunk.Delta)

		if chunk.FinishReason != "" {
			c.finishReason = chunk.FinishReason
		}
	}

	return &CompletionResponse{
		Content:      c.content.String(),
		FinishReason: c.finishReason,
	}, nil
}

// GetChunks returns all collected chunks
func (c *StreamCollector) GetChunks() []*StreamChunk {
	return c.chunks
}
