package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/aixgo-dev/aixgo/internal/llm/cost"
	"github.com/aixgo-dev/aixgo/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// InstrumentedProvider wraps a Provider with automatic observability and cost tracking.
// All LLM calls are automatically instrumented with:
// - Token usage tracking
// - Cost calculation
// - Performance metrics
// - Error tracking
type InstrumentedProvider struct {
	provider   Provider
	calculator *cost.Calculator
	enabled    bool
}

// InstrumentedConfig contains configuration for instrumented providers
type InstrumentedConfig struct {
	// Calculator for cost tracking (defaults to cost.DefaultCalculator)
	Calculator *cost.Calculator

	// Enabled controls whether instrumentation is active
	Enabled bool
}

// NewInstrumentedProvider wraps a provider with automatic observability
func NewInstrumentedProvider(provider Provider, config *InstrumentedConfig) *InstrumentedProvider {
	if config == nil {
		config = &InstrumentedConfig{
			Calculator: cost.DefaultCalculator,
			Enabled:    true,
		}
	}

	if config.Calculator == nil {
		config.Calculator = cost.DefaultCalculator
	}

	return &InstrumentedProvider{
		provider:   provider,
		calculator: config.Calculator,
		enabled:    config.Enabled,
	}
}

// CreateCompletion creates a completion with automatic instrumentation
func (p *InstrumentedProvider) CreateCompletion(ctx context.Context, request CompletionRequest) (*CompletionResponse, error) {
	if !p.enabled {
		return p.provider.CreateCompletion(ctx, request)
	}

	// Create span for this completion
	ctx, span := observability.StartSpanWithOtel(ctx, fmt.Sprintf("llm.%s.completion", p.provider.Name()),
		trace.WithAttributes(
			attribute.String("llm.provider", p.provider.Name()),
			attribute.String("llm.model", request.Model),
			attribute.Float64("llm.temperature", request.Temperature),
			attribute.Int("llm.max_tokens", request.MaxTokens),
			attribute.Int("llm.messages_count", len(request.Messages)),
			attribute.Int("llm.tools_count", len(request.Tools)),
		),
	)
	defer span.End()

	// Track start time
	startTime := time.Now()

	// Call underlying provider
	response, err := p.provider.CreateCompletion(ctx, request)

	// Track duration
	duration := time.Since(startTime)

	// Record metrics
	span.SetAttributes(
		attribute.Int64("llm.duration_ms", duration.Milliseconds()),
		attribute.Bool("llm.success", err == nil),
	)

	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("llm.error", err.Error()))
		return nil, err
	}

	// Track token usage
	if response != nil {
		span.SetAttributes(
			attribute.Int("llm.usage.prompt_tokens", response.Usage.PromptTokens),
			attribute.Int("llm.usage.completion_tokens", response.Usage.CompletionTokens),
			attribute.Int("llm.usage.total_tokens", response.Usage.TotalTokens),
			attribute.String("llm.finish_reason", response.FinishReason),
		)

		// Calculate and track cost
		usage := &cost.Usage{
			Model:        request.Model,
			InputTokens:  response.Usage.PromptTokens,
			OutputTokens: response.Usage.CompletionTokens,
			TotalTokens:  response.Usage.TotalTokens,
		}

		if costResult, err := p.calculator.Calculate(usage); err == nil {
			span.SetAttributes(
				attribute.Float64("llm.cost.input_usd", costResult.InputCost),
				attribute.Float64("llm.cost.output_usd", costResult.OutputCost),
				attribute.Float64("llm.cost.total_usd", costResult.TotalCost),
			)
		}

		// Track tool calls
		if len(response.ToolCalls) > 0 {
			span.SetAttributes(attribute.Int("llm.tool_calls_count", len(response.ToolCalls)))
		}
	}

	return response, nil
}

// CreateStructured creates a structured response with automatic instrumentation
func (p *InstrumentedProvider) CreateStructured(ctx context.Context, request StructuredRequest) (*StructuredResponse, error) {
	if !p.enabled {
		return p.provider.CreateStructured(ctx, request)
	}

	// Create span for structured output
	ctx, span := observability.StartSpanWithOtel(ctx, fmt.Sprintf("llm.%s.structured", p.provider.Name()),
		trace.WithAttributes(
			attribute.String("llm.provider", p.provider.Name()),
			attribute.String("llm.model", request.Model),
			attribute.String("llm.response_format", request.ResponseFormat),
			attribute.Bool("llm.strict_schema", request.StrictSchema),
		),
	)
	defer span.End()

	startTime := time.Now()
	response, err := p.provider.CreateStructured(ctx, request)
	duration := time.Since(startTime)

	span.SetAttributes(
		attribute.Int64("llm.duration_ms", duration.Milliseconds()),
		attribute.Bool("llm.success", err == nil),
	)

	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("llm.error", err.Error()))
		return nil, err
	}

	// Track token usage and cost
	if response != nil {
		span.SetAttributes(
			attribute.Int("llm.usage.prompt_tokens", response.Usage.PromptTokens),
			attribute.Int("llm.usage.completion_tokens", response.Usage.CompletionTokens),
			attribute.Int("llm.usage.total_tokens", response.Usage.TotalTokens),
		)

		usage := &cost.Usage{
			Model:        request.Model,
			InputTokens:  response.Usage.PromptTokens,
			OutputTokens: response.Usage.CompletionTokens,
			TotalTokens:  response.Usage.TotalTokens,
		}

		if costResult, err := p.calculator.Calculate(usage); err == nil {
			span.SetAttributes(
				attribute.Float64("llm.cost.input_usd", costResult.InputCost),
				attribute.Float64("llm.cost.output_usd", costResult.OutputCost),
				attribute.Float64("llm.cost.total_usd", costResult.TotalCost),
			)
		}
	}

	return response, nil
}

// CreateStreaming creates a streaming response with instrumentation
func (p *InstrumentedProvider) CreateStreaming(ctx context.Context, request CompletionRequest) (Stream, error) {
	if !p.enabled {
		return p.provider.CreateStreaming(ctx, request)
	}

	// Create span for streaming
	ctx, span := observability.StartSpanWithOtel(ctx, fmt.Sprintf("llm.%s.streaming", p.provider.Name()),
		trace.WithAttributes(
			attribute.String("llm.provider", p.provider.Name()),
			attribute.String("llm.model", request.Model),
			attribute.Bool("llm.streaming", true),
		),
	)

	stream, err := p.provider.CreateStreaming(ctx, request)
	if err != nil {
		span.RecordError(err)
		span.End()
		return nil, err
	}

	// Wrap stream with instrumentation
	return &instrumentedStream{
		stream:   stream,
		span:     span,
		provider: p.provider.Name(),
		model:    request.Model,
		calc:     p.calculator,
	}, nil
}

// Name returns the underlying provider name
func (p *InstrumentedProvider) Name() string {
	return p.provider.Name()
}

// instrumentedStream wraps a Stream with observability
type instrumentedStream struct {
	stream        Stream
	span          trace.Span
	provider      string
	model         string
	calc          *cost.Calculator
	chunksCount   int
	totalDuration time.Duration
	startTime     time.Time
}

// Recv receives the next chunk and tracks metrics
func (s *instrumentedStream) Recv() (*StreamChunk, error) {
	if s.startTime.IsZero() {
		s.startTime = time.Now()
	}

	chunkStart := time.Now()
	chunk, err := s.stream.Recv()
	chunkDuration := time.Since(chunkStart)

	s.totalDuration += chunkDuration
	s.chunksCount++

	if err != nil {
		s.span.RecordError(err)
		s.span.SetAttributes(
			attribute.Int("llm.streaming.chunks_received", s.chunksCount),
			attribute.Int64("llm.streaming.total_duration_ms", s.totalDuration.Milliseconds()),
		)
		return nil, err
	}

	// Track finish and calculate final metrics
	if chunk != nil && chunk.FinishReason != "" {
		s.span.SetAttributes(
			attribute.String("llm.finish_reason", chunk.FinishReason),
			attribute.Int("llm.streaming.chunks_received", s.chunksCount),
			attribute.Int64("llm.streaming.total_duration_ms", s.totalDuration.Milliseconds()),
		)

		// Note: Streaming doesn't provide token counts in chunks
		// Token counts would need to be estimated or retrieved after stream completion
	}

	return chunk, nil
}

// Close closes the stream and finalizes metrics
func (s *instrumentedStream) Close() error {
	err := s.stream.Close()

	// Finalize span
	s.span.SetAttributes(
		attribute.Int("llm.streaming.chunks_total", s.chunksCount),
		attribute.Int64("llm.streaming.duration_ms", s.totalDuration.Milliseconds()),
	)

	if err != nil {
		s.span.RecordError(err)
	}

	s.span.End()
	return err
}

// WrapProvider wraps a provider with instrumentation if not already wrapped
func WrapProvider(provider Provider) Provider {
	// Don't double-wrap
	if _, ok := provider.(*InstrumentedProvider); ok {
		return provider
	}

	return NewInstrumentedProvider(provider, &InstrumentedConfig{
		Calculator: cost.DefaultCalculator,
		Enabled:    true,
	})
}

// UnwrapProvider returns the underlying provider if wrapped, otherwise returns the provider as-is
func UnwrapProvider(provider Provider) Provider {
	if instrumented, ok := provider.(*InstrumentedProvider); ok {
		return instrumented.provider
	}
	return provider
}
