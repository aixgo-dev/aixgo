// Package session provides session management for the chat assistant.
package session

import (
	"time"
)

// Message represents a single message in a conversation.
type Message struct {
	Role      string    `json:"role"`       // "user", "assistant", or "system"
	Content   string    `json:"content"`    // Message content
	Timestamp time.Time `json:"timestamp"`  // When the message was sent
	Model     string    `json:"model,omitempty"`     // Model used (for assistant messages)
	Cost      float64   `json:"cost,omitempty"`      // Cost of this message (USD)
	ToolCalls []ToolCall `json:"tool_calls,omitempty"` // Tool calls made
}

// ToolCall represents a tool invocation.
type ToolCall struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Args     string `json:"args"`     // JSON-encoded arguments
	Result   string `json:"result"`   // Tool result
	Error    string `json:"error,omitempty"`
	Duration int64  `json:"duration"` // Duration in milliseconds
}

// Session represents a chat session.
type Session struct {
	ID          string    `json:"id"`
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	TotalCost   float64   `json:"total_cost"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	WorkingDir  string    `json:"working_dir,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// AddMessage adds a message to the session.
func (s *Session) AddMessage(msg Message) {
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
}

// LastMessage returns the last message in the session.
func (s *Session) LastMessage() *Message {
	if len(s.Messages) == 0 {
		return nil
	}
	return &s.Messages[len(s.Messages)-1]
}

// UserMessages returns only user messages.
func (s *Session) UserMessages() []Message {
	var msgs []Message
	for _, m := range s.Messages {
		if m.Role == "user" {
			msgs = append(msgs, m)
		}
	}
	return msgs
}

// AssistantMessages returns only assistant messages.
func (s *Session) AssistantMessages() []Message {
	var msgs []Message
	for _, m := range s.Messages {
		if m.Role == "assistant" {
			msgs = append(msgs, m)
		}
	}
	return msgs
}

// Summary provides a brief session summary.
type Summary struct {
	ID        string    `json:"id"`
	Model     string    `json:"model"`
	Messages  int       `json:"messages"`
	TotalCost float64   `json:"total_cost"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ToSummary converts a session to a summary.
func (s *Session) ToSummary() Summary {
	return Summary{
		ID:        s.ID,
		Model:     s.Model,
		Messages:  len(s.Messages),
		TotalCost: s.TotalCost,
		UpdatedAt: s.UpdatedAt,
	}
}
