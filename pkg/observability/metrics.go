package observability

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP metrics
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aixgo_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "aixgo_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// MCP metrics
	mcpToolCallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aixgo_mcp_tool_calls_total",
			Help: "Total number of MCP tool calls",
		},
		[]string{"tool", "status"},
	)

	mcpToolCallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "aixgo_mcp_tool_call_duration_seconds",
			Help:    "MCP tool call duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"tool"},
	)

	// gRPC metrics
	grpcRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aixgo_grpc_requests_total",
			Help: "Total number of gRPC requests",
		},
		[]string{"method", "status"},
	)

	grpcRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "aixgo_grpc_request_duration_seconds",
			Help:    "gRPC request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)

	// Agent metrics
	agentMessagesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aixgo_agent_messages_total",
			Help: "Total number of agent messages",
		},
		[]string{"agent", "type"},
	)

	agentExecutionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "aixgo_agent_execution_duration_seconds",
			Help:    "Agent execution duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"agent"},
	)

	// System metrics
	activeConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aixgo_active_connections",
			Help: "Number of active connections",
		},
	)

	memoryUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aixgo_memory_usage_bytes",
			Help: "Memory usage in bytes",
		},
	)

	goroutines = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aixgo_goroutines",
			Help: "Number of goroutines",
		},
	)

	initOnce sync.Once
)

// InitMetrics initializes Prometheus metrics
func InitMetrics() {
	initOnce.Do(func() {
		prometheus.MustRegister(
			httpRequestsTotal,
			httpRequestDuration,
			mcpToolCallsTotal,
			mcpToolCallDuration,
			grpcRequestsTotal,
			grpcRequestDuration,
			agentMessagesTotal,
			agentExecutionDuration,
			activeConnections,
			memoryUsage,
			goroutines,
		)
	})
}

// MetricsHandler returns an HTTP handler for Prometheus metrics
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// RecordHTTPRequest records HTTP request metrics
func RecordHTTPRequest(method, path, status string, duration time.Duration) {
	httpRequestsTotal.WithLabelValues(method, path, status).Inc()
	httpRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

// RecordMCPToolCall records MCP tool call metrics
func RecordMCPToolCall(tool, status string, duration time.Duration) {
	mcpToolCallsTotal.WithLabelValues(tool, status).Inc()
	mcpToolCallDuration.WithLabelValues(tool).Observe(duration.Seconds())
}

// RecordGRPCRequest records gRPC request metrics
func RecordGRPCRequest(method, status string, duration time.Duration) {
	grpcRequestsTotal.WithLabelValues(method, status).Inc()
	grpcRequestDuration.WithLabelValues(method).Observe(duration.Seconds())
}

// RecordAgentMessage records agent message metrics
func RecordAgentMessage(agent, msgType string) {
	agentMessagesTotal.WithLabelValues(agent, msgType).Inc()
}

// RecordAgentExecution records agent execution metrics
func RecordAgentExecution(agent string, duration time.Duration) {
	agentExecutionDuration.WithLabelValues(agent).Observe(duration.Seconds())
}

// SetActiveConnections sets the active connections gauge
func SetActiveConnections(count int) {
	activeConnections.Set(float64(count))
}

// SetMemoryUsage sets the memory usage gauge
func SetMemoryUsage(bytes uint64) {
	memoryUsage.Set(float64(bytes))
}

// SetGoroutines sets the goroutines gauge
func SetGoroutines(count int) {
	goroutines.Set(float64(count))
}
