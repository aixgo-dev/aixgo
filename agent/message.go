package agent

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Message is the standard message format for agent communication.
// Messages are used for both synchronous (Call) and asynchronous (Send/Recv) communication.
type Message struct {
	// ID is a unique identifier for this message, automatically generated.
	ID string

	// Type identifies the message type (e.g., "analysis_request", "analysis_result").
	// The type is used by agents to route and process messages appropriately.
	Type string

	// Payload contains the message data as a JSON string.
	// Use UnmarshalPayload to deserialize into a specific type.
	Payload string

	// Timestamp is the ISO 8601 timestamp when the message was created.
	Timestamp string

	// Metadata contains optional key-value pairs for routing, tracing, correlation, etc.
	Metadata map[string]interface{}
}

// NewMessage creates a new message with the given type and payload.
// The payload is automatically serialized to JSON.
// A unique ID and timestamp are automatically generated.
func NewMessage(msgType string, payload interface{}) *Message {
	payloadJSON, _ := json.Marshal(payload)
	return &Message{
		ID:        uuid.New().String(),
		Type:      msgType,
		Payload:   string(payloadJSON),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Metadata:  make(map[string]interface{}),
	}
}

// WithMetadata adds metadata to the message and returns it for chaining.
// This allows for fluent construction:
//
//	msg := NewMessage("request", data).
//	    WithMetadata("priority", "high").
//	    WithMetadata("source", "api")
func (m *Message) WithMetadata(key string, value interface{}) *Message {
	if m.Metadata == nil {
		m.Metadata = make(map[string]interface{})
	}
	m.Metadata[key] = value
	return m
}

// GetMetadata retrieves metadata by key, returning the default value if not found.
func (m *Message) GetMetadata(key string, defaultValue interface{}) interface{} {
	if m.Metadata == nil {
		return defaultValue
	}
	if val, ok := m.Metadata[key]; ok {
		return val
	}
	return defaultValue
}

// GetMetadataString is a convenience method to get metadata as a string.
func (m *Message) GetMetadataString(key, defaultValue string) string {
	val := m.GetMetadata(key, defaultValue)
	if str, ok := val.(string); ok {
		return str
	}
	return defaultValue
}

// UnmarshalPayload deserializes the message payload into the provided value.
// The value should be a pointer to the desired type.
//
//	var req AnalysisRequest
//	if err := msg.UnmarshalPayload(&req); err != nil {
//	    return err
//	}
func (m *Message) UnmarshalPayload(v interface{}) error {
	if m.Payload == "" {
		return fmt.Errorf("message payload is empty")
	}
	return json.Unmarshal([]byte(m.Payload), v)
}

// MarshalPayload is a convenience method that returns the payload as JSON bytes.
// This is equivalent to []byte(m.Payload).
func (m *Message) MarshalPayload() []byte {
	return []byte(m.Payload)
}

// Clone creates a deep copy of the message.
// This is useful when you need to modify a message without affecting the original.
func (m *Message) Clone() *Message {
	clone := &Message{
		ID:        m.ID,
		Type:      m.Type,
		Payload:   m.Payload,
		Timestamp: m.Timestamp,
		Metadata:  make(map[string]interface{}),
	}
	for k, v := range m.Metadata {
		clone.Metadata[k] = v
	}
	return clone
}

// String returns a human-readable representation of the message for debugging.
func (m *Message) String() string {
	return fmt.Sprintf("Message{ID:%s, Type:%s, Timestamp:%s}", m.ID, m.Type, m.Timestamp)
}
