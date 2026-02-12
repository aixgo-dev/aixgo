package orchestration

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/observability"
	pb "github.com/aixgo-dev/aixgo/proto"
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
	if msg == nil || msg.Message == nil {
		return make(map[string]*agent.Message)
	}

	// Check if metadata contains team assignments
	if msg.Metadata != nil {
		if assignments, ok := msg.Metadata["team_assignments"].(map[string]any); ok {
			result := make(map[string]*agent.Message)
			for teamName, taskData := range assignments {
				var taskPayload string
				switch v := taskData.(type) {
				case string:
					taskPayload = v
				case map[string]any:
					// If task is structured, use the payload field
					if payload, ok := v["payload"].(string); ok {
						taskPayload = payload
					} else {
						taskPayload = fmt.Sprintf("%v", v)
					}
				default:
					taskPayload = fmt.Sprintf("%v", v)
				}

				result[teamName] = &agent.Message{
					Message: &pb.Message{
						Type:      "team_task",
						Payload:   taskPayload,
						Timestamp: msg.Timestamp,
						Metadata: map[string]any{
							"team":           teamName,
							"source_message": msg.Id,
						},
					},
				}
			}
			return result
		}
	}

	// Fallback: treat the entire message payload as a single task for parsing
	// This is a simple implementation that expects JSON with team assignments
	// In production, this would use more sophisticated parsing
	return make(map[string]*agent.Message)
}

// aggregateTeamResults combines results from team workers (deterministic)
func aggregateTeamResults(results map[string]*agent.Message) *agent.Message {
	if len(results) == 0 {
		return &agent.Message{
			Message: &pb.Message{
				Type:    "team_result",
				Payload: "",
			},
		}
	}

	// If only one result, return it directly
	if len(results) == 1 {
		for _, msg := range results {
			return msg
		}
	}

	// Sort keys for deterministic iteration
	var keys []string
	for k := range results {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Combine all worker results into a single aggregated result
	var combinedPayload string
	metadata := make(map[string]any)

	for i, key := range keys {
		msg := results[key]
		if msg != nil && msg.Message != nil {
			if i > 0 {
				combinedPayload += "\n\n---\n\n"
			}
			combinedPayload += fmt.Sprintf("Worker %s:\n%s", key, msg.Payload)

			// Collect metadata from all workers
			if msg.Metadata != nil {
				metadata[fmt.Sprintf("worker_%s", key)] = msg.Metadata
			}
		}
	}

	metadata["worker_count"] = len(results)

	return &agent.Message{
		Message: &pb.Message{
			Type:     "team_aggregated",
			Payload:  combinedPayload,
			Metadata: metadata,
		},
	}
}

// combineTeamResults prepares input for final synthesis
func combineTeamResults(original *agent.Message, teamResults map[string]*agent.Message) *agent.Message {
	if original == nil || original.Message == nil {
		return original
	}

	if len(teamResults) == 0 {
		return original
	}

	// Sort team names for deterministic ordering
	var teamNames []string
	for name := range teamResults {
		teamNames = append(teamNames, name)
	}
	sort.Strings(teamNames)

	// Build combined payload with original query and all team results
	var combinedPayload string
	combinedPayload = fmt.Sprintf("Original Task:\n%s\n\n", original.Payload)
	combinedPayload += "Team Results:\n\n"

	metadata := make(map[string]any)
	if original.Metadata != nil {
		maps.Copy(metadata, original.Metadata)
	}

	for i, teamName := range teamNames {
		msg := teamResults[teamName]
		if msg != nil && msg.Message != nil {
			if i > 0 {
				combinedPayload += "\n---\n\n"
			}
			combinedPayload += fmt.Sprintf("Team '%s':\n%s", teamName, msg.Payload)

			// Store team results in metadata
			metadata[fmt.Sprintf("team_%s_result", teamName)] = msg.Payload
		}
	}

	metadata["synthesis_stage"] = true
	metadata["team_count"] = len(teamResults)

	return &agent.Message{
		Message: &pb.Message{
			Id:        original.Id,
			Type:      "synthesis_input",
			Payload:   combinedPayload,
			Timestamp: original.Timestamp,
			Metadata:  metadata,
		},
	}
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
