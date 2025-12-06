package orchestration

import (
	"testing"

	"github.com/aixgo-dev/aixgo/internal/agent"
	pb "github.com/aixgo-dev/aixgo/proto"
)

func TestIsValidClassification(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"valid lowercase", "simple", true},
		{"valid with hyphen", "very-complex", true},
		{"valid with numbers", "level-2", true},
		{"empty string", "", false},
		{"uppercase start", "Simple", false},
		{"contains uppercase", "simPle", false},
		{"contains space", "simple query", false},
		{"contains special chars", "simple!", false},
		{"too long", "this-is-a-very-long-classification-name-that-exceeds-limit", false},
		{"starts with number", "1simple", false},
		{"starts with hyphen", "-simple", false},
		{"SQL injection attempt", "simple'; DROP TABLE", false},
		{"path traversal", "../../../etc/passwd", false},
		{"command injection", "simple && rm -rf /", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidClassification(tt.input)
			if result != tt.valid {
				t.Errorf("isValidClassification(%q) = %v, want %v", tt.input, result, tt.valid)
			}
		})
	}
}

func TestExtractClassification(t *testing.T) {
	tests := []struct {
		name     string
		msg      *agent.Message
		expected string
	}{
		{
			name: "valid classification",
			msg: &agent.Message{
				Message: &pb.Message{
					Payload: "simple",
				},
			},
			expected: "simple",
		},
		{
			name: "classification with whitespace",
			msg: &agent.Message{
				Message: &pb.Message{
					Payload: "  complex  ",
				},
			},
			expected: "complex",
		},
		{
			name: "invalid classification returns default",
			msg: &agent.Message{
				Message: &pb.Message{
					Payload: "INVALID!",
				},
			},
			expected: "default",
		},
		{
			name:     "nil message",
			msg:      nil,
			expected: "",
		},
		{
			name: "empty payload",
			msg: &agent.Message{
				Message: &pb.Message{
					Payload: "",
				},
			},
			expected: "default",
		},
		{
			name: "injection attempt",
			msg: &agent.Message{
				Message: &pb.Message{
					Payload: "'; DROP TABLE users--",
				},
			},
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractClassification(tt.msg)
			if result != tt.expected {
				t.Errorf("extractClassification() = %q, want %q", result, tt.expected)
			}
		})
	}
}
