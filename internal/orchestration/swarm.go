package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Swarm implements decentralized agent handoffs based on conversational context.
// Agents decide when to hand off to other agents dynamically.
// Popularized by OpenAI Swarm.
//
// Use cases:
// - Customer service handoffs (general → billing → technical)
// - Adaptive routing based on conversation flow
// - Collaborative problem-solving
type Swarm struct {
	*BaseOrchestrator
	agents      []string
	entryAgent  string // Starting agent
	maxHandoffs int    // Maximum number of handoffs to prevent loops
}

// SwarmOption configures a Swarm orchestrator
type SwarmOption func(*Swarm)

// WithMaxHandoffs sets the maximum number of handoffs
func WithMaxHandoffs(max int) SwarmOption {
	return func(s *Swarm) {
		s.maxHandoffs = max
	}
}

// NewSwarm creates a new Swarm orchestrator
func NewSwarm(name string, runtime agent.Runtime, entryAgent string, agents []string, opts ...SwarmOption) *Swarm {
	s := &Swarm{
		BaseOrchestrator: NewBaseOrchestrator(name, "swarm", runtime),
		agents:           agents,
		entryAgent:       entryAgent,
		maxHandoffs:      10, // Default max handoffs
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Execute runs the swarm starting from the entry agent
func (s *Swarm) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	ctx, span := observability.StartSpanWithOtel(ctx, fmt.Sprintf("orchestration.swarm.%s", s.name),
		trace.WithAttributes(
			attribute.String("orchestration.pattern", "swarm"),
			attribute.String("orchestration.entry_agent", s.entryAgent),
			attribute.Int("orchestration.max_handoffs", s.maxHandoffs),
		),
	)
	defer span.End()

	startTime := time.Now()
	currentAgent := s.entryAgent
	currentInput := input
	handoffCount := 0

	var conversationHistory []*agent.Message
	conversationHistory = append(conversationHistory, input)

	for {
		// Check handoff limit
		if handoffCount >= s.maxHandoffs {
			err := fmt.Errorf("max handoffs (%d) exceeded", s.maxHandoffs)
			span.RecordError(err)
			return nil, err
		}

		span.SetAttributes(
			attribute.String(fmt.Sprintf("handoff.%d.agent", handoffCount), currentAgent),
		)

		// Execute current agent
		result, err := s.runtime.Call(ctx, currentAgent, currentInput)
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("agent %s failed: %w", currentAgent, err)
		}

		conversationHistory = append(conversationHistory, result)

		// Check if agent wants to handoff
		nextAgent, shouldHandoff := extractHandoff(result)

		if !shouldHandoff {
			// No handoff - return final result
			duration := time.Since(startTime)
			span.SetAttributes(
				attribute.Int64("orchestration.duration_ms", duration.Milliseconds()),
				attribute.Int("orchestration.handoff_count", handoffCount),
				attribute.Bool("orchestration.success", true),
			)
			return result, nil
		}

		// Validate handoff target
		if !s.isValidAgent(nextAgent) {
			err := fmt.Errorf("invalid handoff target: %s", nextAgent)
			span.RecordError(err)
			return nil, err
		}

		// Perform handoff
		currentAgent = nextAgent
		currentInput = result // Pass previous result as input to next agent
		handoffCount++
	}
}

// isValidAgent checks if an agent is in the swarm
func (s *Swarm) isValidAgent(name string) bool {
	for _, agent := range s.agents {
		if agent == name {
			return true
		}
	}
	return name == s.entryAgent
}

// extractHandoff determines if the agent wants to handoff and to whom
// This is a placeholder - in practice, you'd define a standard format in Message
func extractHandoff(msg *agent.Message) (string, bool) {
	if msg == nil || msg.Message == nil {
		return "", false
	}

	// TODO: Implement proper handoff extraction based on Message structure
	// Could use metadata, special markers, or structured format
	// For now, return no handoff
	return "", false
}

// HandoffInstruction represents a handoff instruction from an agent
type HandoffInstruction struct {
	TargetAgent string
	Context     string
	Reason      string
}
