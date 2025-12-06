package orchestration

import (
	"testing"

	"github.com/aixgo-dev/aixgo/internal/agent"
	pb "github.com/aixgo-dev/aixgo/proto"
)

func TestIsValidAgentName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"valid lowercase", "agent", true},
		{"valid with hyphen", "billing-agent", true},
		{"valid with underscore", "technical_support", true},
		{"valid with numbers", "agent-2", true},
		{"empty string", "", false},
		{"uppercase start", "Agent", false},
		{"contains uppercase", "agEnt", false},
		{"contains space", "billing agent", false},
		{"contains special chars", "agent!", false},
		{"too long", "this-is-a-very-long-agent-name-that-exceeds-the-maximum-limit-of-64", false},
		{"starts with number", "1agent", false},
		{"starts with hyphen", "-agent", false},
		{"starts with underscore", "_agent", false},
		{"SQL injection", "agent'; DROP TABLE", false},
		{"path traversal", "../../../etc/passwd", false},
		{"command injection", "agent && rm -rf /", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidAgentName(tt.input)
			if result != tt.valid {
				t.Errorf("isValidAgentName(%q) = %v, want %v", tt.input, result, tt.valid)
			}
		})
	}
}

func TestExtractHandoff(t *testing.T) {
	tests := []struct {
		name           string
		msg            *agent.Message
		expectedAgent  string
		expectedHandoff bool
	}{
		{
			name: "metadata handoff",
			msg: &agent.Message{
				Message: &pb.Message{
					Metadata: map[string]interface{}{
						"handoff_to": "billing-agent",
					},
				},
			},
			expectedAgent:  "billing-agent",
			expectedHandoff: true,
		},
		{
			name: "payload handoff",
			msg: &agent.Message{
				Message: &pb.Message{
					Payload: "HANDOFF:technical-support",
				},
			},
			expectedAgent:  "technical-support",
			expectedHandoff: true,
		},
		{
			name: "payload handoff with whitespace",
			msg: &agent.Message{
				Message: &pb.Message{
					Payload: "HANDOFF:  support-agent  ",
				},
			},
			expectedAgent:  "support-agent",
			expectedHandoff: true,
		},
		{
			name: "invalid agent name in metadata",
			msg: &agent.Message{
				Message: &pb.Message{
					Metadata: map[string]interface{}{
						"handoff_to": "INVALID!",
					},
				},
			},
			expectedAgent:  "",
			expectedHandoff: false,
		},
		{
			name: "invalid agent name in payload",
			msg: &agent.Message{
				Message: &pb.Message{
					Payload: "HANDOFF:'; DROP TABLE users--",
				},
			},
			expectedAgent:  "",
			expectedHandoff: false,
		},
		{
			name:           "nil message",
			msg:            nil,
			expectedAgent:  "",
			expectedHandoff: false,
		},
		{
			name: "no handoff",
			msg: &agent.Message{
				Message: &pb.Message{
					Payload: "Just a regular message",
				},
			},
			expectedAgent:  "",
			expectedHandoff: false,
		},
		{
			name: "non-string metadata value",
			msg: &agent.Message{
				Message: &pb.Message{
					Metadata: map[string]interface{}{
						"handoff_to": 12345,
					},
				},
			},
			expectedAgent:  "",
			expectedHandoff: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, handoff := extractHandoff(tt.msg)
			if agent != tt.expectedAgent {
				t.Errorf("extractHandoff() agent = %q, want %q", agent, tt.expectedAgent)
			}
			if handoff != tt.expectedHandoff {
				t.Errorf("extractHandoff() handoff = %v, want %v", handoff, tt.expectedHandoff)
			}
		})
	}
}
