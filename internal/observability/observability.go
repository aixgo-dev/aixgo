package observability

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	// Default service name for traces
	DefaultServiceName = "aixgo"

	// Langfuse default endpoint
	LangfuseEndpoint = "https://cloud.langfuse.com/api/public/otel"
)

var (
	// Global tracer provider
	tracerProvider *sdktrace.TracerProvider

	// Global tracer
	tracer trace.Tracer
)

// Config holds observability configuration
type Config struct {
	// ServiceName is the name of the service (defaults to "aixgo")
	ServiceName string

	// Enabled controls whether tracing is enabled (defaults to true)
	Enabled bool

	// ExporterType specifies the exporter: "otlp", "stdout", or "none"
	ExporterType string

	// OTLPEndpoint is the OTLP endpoint URL (defaults to Langfuse)
	OTLPEndpoint string

	// OTLPHeaders are additional headers for OTLP requests (e.g., authorization)
	OTLPHeaders map[string]string
}

// InitFromEnv initializes observability from environment variables
// Supports standard OpenTelemetry environment variables:
// - OTEL_SERVICE_NAME: Service name (default: "aixgo")
// - OTEL_TRACES_EXPORTER: Exporter type - "otlp", "stdout", or "none" (default: "otlp")
// - OTEL_EXPORTER_OTLP_ENDPOINT: OTLP endpoint (default: Langfuse endpoint)
// - OTEL_EXPORTER_OTLP_HEADERS: Headers in format "key1=value1,key2=value2"
// - LANGFUSE_PUBLIC_KEY: Langfuse public key (for Basic Auth)
// - LANGFUSE_SECRET_KEY: Langfuse secret key (for Basic Auth)
func InitFromEnv() error {
	config := Config{
		ServiceName:  getEnv("OTEL_SERVICE_NAME", DefaultServiceName),
		Enabled:      getEnv("OTEL_TRACES_ENABLED", "true") == "true",
		ExporterType: getEnv("OTEL_TRACES_EXPORTER", "otlp"),
		OTLPEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", LangfuseEndpoint),
		OTLPHeaders:  parseHeaders(getEnv("OTEL_EXPORTER_OTLP_HEADERS", "")),
	}

	// Add Langfuse credentials if provided
	publicKey := os.Getenv("LANGFUSE_PUBLIC_KEY")
	secretKey := os.Getenv("LANGFUSE_SECRET_KEY")
	if publicKey != "" && secretKey != "" {
		if config.OTLPHeaders == nil {
			config.OTLPHeaders = make(map[string]string)
		}
		// Langfuse uses Basic Auth with public key as username and secret key as password
		config.OTLPHeaders["Authorization"] = fmt.Sprintf("Basic %s:%s", publicKey, secretKey)
	}

	return Init(config)
}

// Init initializes the observability system with the given configuration
func Init(config Config) error {
	if !config.Enabled || config.ExporterType == "none" {
		log.Println("Observability disabled")
		tracer = otel.GetTracerProvider().Tracer(config.ServiceName)
		return nil
	}

	// Create resource
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(config.ServiceName),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter based on type
	var exporter sdktrace.SpanExporter
	switch config.ExporterType {
	case "otlp":
		exporter, err = createOTLPExporter(config)
		if err != nil {
			return fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
		log.Printf("Observability initialized with OTLP exporter (endpoint: %s)", config.OTLPEndpoint)

	case "stdout":
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return fmt.Errorf("failed to create stdout exporter: %w", err)
		}
		log.Println("Observability initialized with stdout exporter")

	default:
		return fmt.Errorf("unknown exporter type: %s", config.ExporterType)
	}

	// Create tracer provider
	tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tracerProvider)

	// Create tracer
	tracer = tracerProvider.Tracer(config.ServiceName)

	return nil
}

// Shutdown gracefully shuts down the observability system
func Shutdown(ctx context.Context) error {
	if tracerProvider == nil {
		return nil
	}

	// Create a timeout context if not already provided
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	return tracerProvider.Shutdown(ctx)
}

// StartSpan creates a new span with the given name and attributes (legacy version with map)
// Deprecated: Use StartSpanWithContext for context-aware tracing
func StartSpan(name string, data map[string]any) *Span {
	// If tracer is not initialized, use a noop tracer
	if tracer == nil {
		tracer = otel.GetTracerProvider().Tracer(DefaultServiceName)
	}

	ctx := context.Background()
	spanCtx, span := tracer.Start(ctx, name)

	// Convert data to attributes
	if data != nil {
		attrs := make([]attribute.KeyValue, 0, len(data))
		for k, v := range data {
			attrs = append(attrs, convertToAttribute(k, v))
		}
		span.SetAttributes(attrs...)
	}

	return &Span{
		ctx:   spanCtx,
		span:  span,
		name:  name,
		data:  data,
		ended: false,
	}
}

// StartSpanWithOtel creates a new span with the given name and OpenTelemetry options.
// This is the preferred method for context-aware tracing.
// Returns a context with the span and the raw OpenTelemetry span.
func StartSpanWithOtel(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	// Get the tracer (will use global if not initialized)
	tr := tracer
	if tr == nil {
		tr = otel.GetTracerProvider().Tracer(DefaultServiceName)
	}

	return tr.Start(ctx, name, opts...)
}

// StartSpanWithContext creates a new span from a parent context
func StartSpanWithContext(ctx context.Context, name string, data map[string]any) (context.Context, *Span) {
	// If tracer is not initialized, use a noop tracer
	if tracer == nil {
		tracer = otel.GetTracerProvider().Tracer(DefaultServiceName)
	}

	spanCtx, span := tracer.Start(ctx, name)

	// Convert data to attributes
	if data != nil {
		attrs := make([]attribute.KeyValue, 0, len(data))
		for k, v := range data {
			attrs = append(attrs, convertToAttribute(k, v))
		}
		span.SetAttributes(attrs...)
	}

	wrappedSpan := &Span{
		ctx:   spanCtx,
		span:  span,
		name:  name,
		data:  data,
		ended: false,
	}

	return spanCtx, wrappedSpan
}

// Span represents an observability span
type Span struct {
	ctx   context.Context
	span  trace.Span
	name  string
	data  map[string]any
	ended bool
}

// End finishes the span
func (s *Span) End() {
	if !s.ended && s.span != nil {
		s.span.End()
		s.ended = true
	}
}

// Name returns the span name
func (s *Span) Name() string {
	return s.name
}

// Data returns the span data
func (s *Span) Data() map[string]any {
	return s.data
}

// IsEnded returns whether the span has been ended
func (s *Span) IsEnded() bool {
	return s.ended
}

// Context returns the span's context
func (s *Span) Context() context.Context {
	return s.ctx
}

// SetAttribute adds an attribute to the span
func (s *Span) SetAttribute(key string, value any) {
	if s.span != nil {
		s.span.SetAttributes(convertToAttribute(key, value))
	}
}

// SetError marks the span as having an error
func (s *Span) SetError(err error) {
	if s.span != nil && err != nil {
		s.span.RecordError(err)
	}
}

// Helper functions

func createOTLPExporter(config Config) (sdktrace.SpanExporter, error) {
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(config.OTLPEndpoint),
	}

	// Add headers if provided
	if len(config.OTLPHeaders) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(config.OTLPHeaders))
	}

	client := otlptracehttp.NewClient(opts...)
	return otlptrace.New(context.Background(), client)
}

func convertToAttribute(key string, value any) attribute.KeyValue {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case float64:
		return attribute.Float64(key, v)
	case bool:
		return attribute.Bool(key, v)
	default:
		return attribute.String(key, fmt.Sprintf("%v", v))
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseHeaders(headerStr string) map[string]string {
	if headerStr == "" {
		return nil
	}

	headers := make(map[string]string)
	// Simple parsing of "key1=value1,key2=value2" format
	// For production, you might want more robust parsing
	pairs := splitHeaders(headerStr)
	for _, pair := range pairs {
		if kv := splitKeyValue(pair); len(kv) == 2 {
			headers[kv[0]] = kv[1]
		}
	}
	return headers
}

func splitHeaders(s string) []string {
	var result []string
	var current string
	for _, char := range s {
		if char == ',' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func splitKeyValue(s string) []string {
	for i, char := range s {
		if char == '=' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}
