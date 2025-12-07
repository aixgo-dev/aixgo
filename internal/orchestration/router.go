package orchestration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Router routes requests to different agents based on classification.
// Provides 25-50% cost reduction by routing simple queries to cheaper models.
//
// Use cases:
// - Cost optimization (simple → cheap, complex → expensive models)
// - Intent-based routing
// - Skill-based agent selection
// - Load balancing
type Router struct {
	*BaseOrchestrator
	classifier   string            // Agent that classifies the input
	routes       map[string]string // Map of classification → agent name
	defaultRoute string            // Fallback agent if classification not found
}

// RouterOption configures a Router orchestrator
type RouterOption func(*Router)

// WithDefaultRoute sets the fallback agent
func WithDefaultRoute(agent string) RouterOption {
	return func(r *Router) {
		r.defaultRoute = agent
	}
}

// NewRouter creates a new Router orchestrator
func NewRouter(name string, runtime agent.Runtime, classifier string, routes map[string]string, opts ...RouterOption) *Router {
	r := &Router{
		BaseOrchestrator: NewBaseOrchestrator(name, "router", runtime),
		classifier:       classifier,
		routes:           routes,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Execute classifies the input and routes to the appropriate agent
func (r *Router) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	ctx, span := observability.StartSpanWithOtel(ctx, fmt.Sprintf("orchestration.router.%s", r.name),
		trace.WithAttributes(
			attribute.String("orchestration.pattern", "router"),
			attribute.String("orchestration.classifier", r.classifier),
			attribute.Int("orchestration.routes_count", len(r.routes)),
		),
	)
	defer span.End()

	startTime := time.Now()

	// Step 1: Classify the input
	classifyStart := time.Now()
	classification, err := r.runtime.Call(ctx, r.classifier, input)
	classifyDuration := time.Since(classifyStart)

	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("classification failed: %w", err)
	}

	span.SetAttributes(
		attribute.Int64("orchestration.classify_duration_ms", classifyDuration.Milliseconds()),
	)

	// Step 2: Extract classification result (assume it's in the message content)
	// TODO: Define a standard way to extract classification from Message
	classResult := extractClassification(classification)

	span.SetAttributes(attribute.String("orchestration.classification", classResult))

	// Step 3: Route to appropriate agent
	targetAgent, ok := r.routes[classResult]
	if !ok {
		if r.defaultRoute != "" {
			targetAgent = r.defaultRoute
			span.SetAttributes(attribute.Bool("orchestration.used_default_route", true))
		} else {
			err := fmt.Errorf("no route found for classification: %s", classResult)
			span.RecordError(err)
			return nil, err
		}
	}

	span.SetAttributes(attribute.String("orchestration.target_agent", targetAgent))

	// Step 4: Execute target agent
	executeStart := time.Now()
	result, err := r.runtime.Call(ctx, targetAgent, input)
	executeDuration := time.Since(executeStart)

	totalDuration := time.Since(startTime)

	span.SetAttributes(
		attribute.Int64("orchestration.execute_duration_ms", executeDuration.Milliseconds()),
		attribute.Int64("orchestration.total_duration_ms", totalDuration.Milliseconds()),
		attribute.Bool("orchestration.success", err == nil),
	)

	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("execution failed on agent %s: %w", targetAgent, err)
	}

	return result, nil
}

// extractClassification extracts the classification result from the message
func extractClassification(msg *agent.Message) string {
	if msg == nil || msg.Message == nil {
		return ""
	}

	// Extract from payload (classifier returns classification string)
	classification := strings.TrimSpace(msg.Payload)

	if classification == "" {
		return "default"
	}

	// Validate classification format (prevent injection)
	if !isValidClassification(classification) {
		return "default"
	}

	return classification
}

// isValidClassification validates classification format
func isValidClassification(class string) bool {
	// Only allow lowercase alphanumeric and hyphens, max 32 chars
	if len(class) == 0 || len(class) > 32 {
		return false
	}
	for i, r := range class {
		if i == 0 {
			// Must start with lowercase letter
			if r < 'a' || r > 'z' {
				return false
			}
		} else {
			// Can contain lowercase letters, digits, hyphens
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}

// Common routing strategies

// NewCostOptimizingRouter creates a router that optimizes for cost
// Simple queries → cheap models (gpt-3.5, claude-haiku)
// Complex queries → expensive models (gpt-4, claude-opus)
func NewCostOptimizingRouter(name string, runtime agent.Runtime, classifier string) *Router {
	routes := map[string]string{
		"simple":  "cheap-model-agent",
		"complex": "expensive-model-agent",
	}

	return NewRouter(name, runtime, classifier, routes,
		WithDefaultRoute("cheap-model-agent"),
	)
}

// NewIntentRouter creates a router based on user intent
func NewIntentRouter(name string, runtime agent.Runtime, classifier string, intentMap map[string]string) *Router {
	return NewRouter(name, runtime, classifier, intentMap,
		WithDefaultRoute("general-agent"),
	)
}

// NewSkillRouter creates a router based on required skills
func NewSkillRouter(name string, runtime agent.Runtime, classifier string, skillMap map[string]string) *Router {
	return NewRouter(name, runtime, classifier, skillMap)
}
