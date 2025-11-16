package proto

import (
	"testing"
)

func TestMessage_Fields(t *testing.T) {
	tests := []struct {
		name      string
		msg       Message
		wantId    string
		wantType  string
		wantPayload string
		wantTimestamp string
	}{
		{
			name: "empty message",
			msg:  Message{},
			wantId: "",
			wantType: "",
			wantPayload: "",
			wantTimestamp: "",
		},
		{
			name: "message with all fields",
			msg: Message{
				Id:        "msg-123",
				Type:      "test-type",
				Payload:   "test payload",
				Timestamp: "2024-01-01T00:00:00Z",
			},
			wantId:    "msg-123",
			wantType:  "test-type",
			wantPayload: "test payload",
			wantTimestamp: "2024-01-01T00:00:00Z",
		},
		{
			name: "message with unicode payload",
			msg: Message{
				Id:      "msg-456",
				Type:    "unicode",
				Payload: "🎉 Unicode test 日本語",
				Timestamp: "2024-01-02T12:00:00Z",
			},
			wantId:    "msg-456",
			wantType:  "unicode",
			wantPayload: "🎉 Unicode test 日本語",
			wantTimestamp: "2024-01-02T12:00:00Z",
		},
		{
			name: "message with long payload",
			msg: Message{
				Id:      "msg-789",
				Type:    "large",
				Payload: string(make([]byte, 10000)),
				Timestamp: "2024-01-03T18:30:00Z",
			},
			wantId:    "msg-789",
			wantType:  "large",
			wantPayload: string(make([]byte, 10000)),
			wantTimestamp: "2024-01-03T18:30:00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.msg.Id != tt.wantId {
				t.Errorf("Id = %v, want %v", tt.msg.Id, tt.wantId)
			}
			if tt.msg.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", tt.msg.Type, tt.wantType)
			}
			if tt.msg.Payload != tt.wantPayload {
				t.Errorf("Payload = %v, want %v", tt.msg.Payload, tt.wantPayload)
			}
			if tt.msg.Timestamp != tt.wantTimestamp {
				t.Errorf("Timestamp = %v, want %v", tt.msg.Timestamp, tt.wantTimestamp)
			}
		})
	}
}

func TestMessage_Assignment(t *testing.T) {
	msg := Message{
		Id:        "original-id",
		Type:      "original-type",
		Payload:   "original-payload",
		Timestamp: "2024-01-01T00:00:00Z",
	}

	// Test field reassignment
	msg.Id = "new-id"
	if msg.Id != "new-id" {
		t.Errorf("Id reassignment failed: got %v, want new-id", msg.Id)
	}

	msg.Type = "new-type"
	if msg.Type != "new-type" {
		t.Errorf("Type reassignment failed: got %v, want new-type", msg.Type)
	}

	msg.Payload = "new-payload"
	if msg.Payload != "new-payload" {
		t.Errorf("Payload reassignment failed: got %v, want new-payload", msg.Payload)
	}

	msg.Timestamp = "2024-12-31T23:59:59Z"
	if msg.Timestamp != "2024-12-31T23:59:59Z" {
		t.Errorf("Timestamp reassignment failed: got %v, want 2024-12-31T23:59:59Z", msg.Timestamp)
	}
}

func TestMessage_ZeroValue(t *testing.T) {
	var msg Message

	if msg.Id != "" {
		t.Errorf("zero value Id = %v, want empty string", msg.Id)
	}
	if msg.Type != "" {
		t.Errorf("zero value Type = %v, want empty string", msg.Type)
	}
	if msg.Payload != "" {
		t.Errorf("zero value Payload = %v, want empty string", msg.Payload)
	}
	if msg.Timestamp != "" {
		t.Errorf("zero value Timestamp = %v, want empty string", msg.Timestamp)
	}
}

func TestMessage_Copy(t *testing.T) {
	original := Message{
		Id:        "msg-001",
		Type:      "copy-test",
		Payload:   "original payload",
		Timestamp: "2024-01-01T00:00:00Z",
	}

	// Test that assignment creates a copy
	copy := original
	copy.Id = "msg-002"
	copy.Payload = "modified payload"

	if original.Id != "msg-001" {
		t.Errorf("original Id was modified: got %v, want msg-001", original.Id)
	}
	if original.Payload != "original payload" {
		t.Errorf("original Payload was modified: got %v, want 'original payload'", original.Payload)
	}
}

func TestMessage_Pointer(t *testing.T) {
	msg := &Message{
		Id:        "ptr-msg",
		Type:      "pointer-test",
		Payload:   "test",
		Timestamp: "2024-01-01T00:00:00Z",
	}

	if msg.Id != "ptr-msg" {
		t.Errorf("pointer message Id = %v, want ptr-msg", msg.Id)
	}

	// Test modification through pointer
	msg.Id = "modified-ptr-msg"
	if msg.Id != "modified-ptr-msg" {
		t.Errorf("pointer modification failed: got %v, want modified-ptr-msg", msg.Id)
	}
}
