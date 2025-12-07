package orchestration

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Hierarchical implements multi-level delegation pattern.
// Managers delegate to sub-managers who delegate to workers.
// Perfect for complex task decomposition and organizational hierarchies.
//
// Use cases:
// - Enterprise workflows
// - Project management
// - Complex multi-step processes
// - Organizational hierarchies
type Hierarchical struct {
	*BaseOrchestrator
	manager  string
	teams    map[string][]string // team name → worker agents
	maxDepth int
}

// HierarchicalOption configures a Hierarchical orchestrator
type HierarchicalOption func(*Hierarchical)

// WithMaxDepth sets the maximum delegation depth
func WithMaxDepth(depth int) HierarchicalOption {
	return func(h *Hierarchical) {
		h.maxDepth = depth
	}
}

// NewHierarchical creates a new Hierarchical orchestrator
func NewHierarchical(name string, runtime agent.Runtime, manager string, teams map[string][]string, opts ...HierarchicalOption) *Hierarchical {
	h := &Hierarchical{
		BaseOrchestrator: NewBaseOrchestrator(name, "hierarchical", runtime),
		manager:          manager,
		teams:            teams,
		maxDepth:         3, // Default max depth
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// Execute delegates task through hierarchical structure
func (h *Hierarchical) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	ctx, span := observability.StartSpanWithOtel(ctx, fmt.Sprintf("orchestration.hierarchical.%s", h.name),
		trace.WithAttributes(
			attribute.String("orchestration.pattern", "hierarchical"),
			attribute.String("orchestration.manager", h.manager),
			attribute.Int("orchestration.team_count", len(h.teams)),
			attribute.Int("orchestration.max_depth", h.maxDepth),
		),
	)
	defer span.End()

	startTime := time.Now()

	// Step 1: Manager decomposes task and assigns to teams
	managementResult, err := h.runtime.Call(ctx, h.manager, input)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("manager failed: %w", err)
	}

	// Extract team assignments from manager's output
	assignments := extractTeamAssignments(managementResult)

	span.SetAttributes(
		attribute.Int("orchestration.assignments_count", len(assignments)),
	)

	// Step 2: Execute team assignments in parallel
	teamResults := make(map[string]*agent.Message)
	var teamNames []string

	for teamName, task := range assignments {
		teamNames = append(teamNames, teamName)

		// Get workers for this team
		workers, exists := h.teams[teamName]
		if !exists {
			span.SetAttributes(
				attribute.String("warning.team_not_found", teamName),
			)
			continue
		}

		// Execute team's task (could delegate to sub-orchestrator for the team)
		if len(workers) == 1 {
			// Single worker - direct execution
			result, err := h.runtime.Call(ctx, workers[0], task)
			if err != nil {
				span.RecordError(err)
				return nil, fmt.Errorf("worker %s failed: %w", workers[0], err)
			}
			teamResults[teamName] = result
		} else {
			// Multiple workers - parallel execution
			results, errors := h.runtime.CallParallel(ctx, workers, task)
			if len(errors) > 0 {
				span.RecordError(fmt.Errorf("team %s had errors: %v", teamName, errors))
			}

			// Aggregate team results
			aggregated := aggregateTeamResults(results)
			teamResults[teamName] = aggregated
		}
	}

	// Step 3: Manager synthesizes team results into final output
	synthesisInput := combineTeamResults(input, teamResults)
	finalResult, err := h.runtime.Call(ctx, h.manager, synthesisInput)

	totalDuration := time.Since(startTime)

	span.SetAttributes(
		attribute.Int64("orchestration.total_duration_ms", totalDuration.Milliseconds()),
		attribute.Bool("orchestration.success", err == nil),
		attribute.StringSlice("orchestration.teams_executed", teamNames),
	)

	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("synthesis failed: %w", err)
	}

	return finalResult, nil
}

// extractTeamAssignments extracts task assignments from manager's output
func extractTeamAssignments(msg *agent.Message) map[string]*agent.Message {
	// TODO: Implement proper extraction based on Message structure
	// Should return map of team name → task
	return make(map[string]*agent.Message)
}

// aggregateTeamResults combines results from team workers (deterministic)
func aggregateTeamResults(results map[string]*agent.Message) *agent.Message {
	if len(results) == 0 {
		return &agent.Message{}
	}

	// Sort keys for deterministic iteration
	var keys []string
	for k := range results {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Return first result by sorted key order
	// TODO: Implement proper multi-result aggregation
	return results[keys[0]]
}

// combineTeamResults prepares input for final synthesis
func combineTeamResults(original *agent.Message, teamResults map[string]*agent.Message) *agent.Message {
	// TODO: Implement proper combination
	return original
}

// Hierarchical variants

// NewProjectManager creates a hierarchical orchestrator for project management
func NewProjectManager(name string, runtime agent.Runtime) *Hierarchical {
	teams := map[string][]string{
		"frontend": {"ui-engineer", "ux-engineer"},
		"backend":  {"api-engineer", "db-engineer"},
		"qa":       {"test-engineer", "qa-engineer"},
	}

	return NewHierarchical(name, runtime, "project-manager", teams)
}

// NewEnterpriseWorkflow creates a hierarchical orchestrator for enterprise workflows
func NewEnterpriseWorkflow(name string, runtime agent.Runtime, departments map[string][]string) *Hierarchical {
	return NewHierarchical(name, runtime, "ceo-agent", departments,
		WithMaxDepth(5),
	)
}
