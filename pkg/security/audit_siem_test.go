package security

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestElasticsearchBackend(t *testing.T) {
	var receivedRequests [][]byte
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_bulk" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		receivedRequests = append(receivedRequests, body)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &ElasticsearchConfig{
		URLs:      []string{server.URL},
		Index:     "test-audit",
		TLSVerify: false,
	}

	backend, err := newElasticsearchBackendUnsafe(config, 2, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	// Write events
	event1 := &StructuredAuditEvent{
		ID:        "test-1",
		Timestamp: time.Now(),
		Type:      AuditEventToolCall,
		Resource:  "test-tool",
		Result:    "success",
	}
	event2 := &StructuredAuditEvent{
		ID:        "test-2",
		Timestamp: time.Now(),
		Type:      AuditEventAuthSuccess,
		Resource:  "auth",
		Result:    "success",
	}

	if err := backend.Write(event1); err != nil {
		t.Errorf("write event1 failed: %v", err)
	}
	if err := backend.Write(event2); err != nil {
		t.Errorf("write event2 failed: %v", err)
	}

	// Wait for async send
	time.Sleep(200 * time.Millisecond)

	_ = backend.Close()

	mu.Lock()
	if len(receivedRequests) == 0 {
		t.Error("expected at least one bulk request")
	}
	mu.Unlock()
}

func TestSplunkBackend(t *testing.T) {
	var receivedEvents []SplunkEvent
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Splunk test-token" {
			t.Errorf("unexpected auth header: %s", auth)
		}

		body, _ := io.ReadAll(r.Body)
		// Splunk receives concatenated JSON objects
		var event SplunkEvent
		if err := json.Unmarshal(body, &event); err == nil {
			mu.Lock()
			receivedEvents = append(receivedEvents, event)
			mu.Unlock()
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &SplunkConfig{
		URL:       server.URL,
		Token:     "test-token",
		Index:     "main",
		Source:    "audit",
		TLSVerify: false,
	}

	backend, err := newSplunkBackendUnsafe(config, 1, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	event := &StructuredAuditEvent{
		ID:        "splunk-test-1",
		Timestamp: time.Now(),
		Type:      AuditEventToolCall,
		Resource:  "test-tool",
		Result:    "success",
	}

	if err := backend.Write(event); err != nil {
		t.Errorf("write failed: %v", err)
	}

	// Wait for async send
	time.Sleep(200 * time.Millisecond)

	_ = backend.Close()

	mu.Lock()
	if len(receivedEvents) == 0 {
		t.Error("expected at least one event sent to Splunk")
	}
	mu.Unlock()
}

func TestWebhookBackend(t *testing.T) {
	var receivedPayloads []WebhookPayload
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("X-Custom-Header") != "test-value" {
			t.Errorf("missing custom header")
		}

		body, _ := io.ReadAll(r.Body)
		var payload WebhookPayload
		if err := json.Unmarshal(body, &payload); err == nil {
			mu.Lock()
			receivedPayloads = append(receivedPayloads, payload)
			mu.Unlock()
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &WebhookConfig{
		URL:    server.URL,
		Method: "POST",
		Headers: map[string]string{
			"X-Custom-Header": "test-value",
		},
		TLSVerify: false,
	}

	backend, err := newWebhookBackendUnsafe(config, 1, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	event := &StructuredAuditEvent{
		ID:        "webhook-test-1",
		Timestamp: time.Now(),
		Type:      AuditEventAuthFailure,
		Resource:  "auth",
		Result:    "failure",
	}

	if err := backend.Write(event); err != nil {
		t.Errorf("write failed: %v", err)
	}

	// Wait for async send
	time.Sleep(200 * time.Millisecond)

	_ = backend.Close()

	mu.Lock()
	if len(receivedPayloads) == 0 {
		t.Error("expected at least one webhook payload")
	} else if receivedPayloads[0].Count != 1 {
		t.Errorf("expected count 1, got %d", receivedPayloads[0].Count)
	}
	mu.Unlock()
}

func TestBackendValidation(t *testing.T) {
	// Elasticsearch requires URLs
	_, err := NewElasticsearchBackend(nil, 10, time.Second)
	if err == nil {
		t.Error("expected error for nil config")
	}

	_, err = NewElasticsearchBackend(&ElasticsearchConfig{}, 10, time.Second)
	if err == nil {
		t.Error("expected error for empty URLs")
	}

	// Splunk requires URL and token
	_, err = NewSplunkBackend(nil, 10, time.Second)
	if err == nil {
		t.Error("expected error for nil config")
	}

	_, err = NewSplunkBackend(&SplunkConfig{URL: "http://localhost"}, 10, time.Second)
	if err == nil {
		t.Error("expected error for missing token")
	}

	// Webhook requires URL
	_, err = NewWebhookBackend(nil, 10, time.Second)
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestBatchFlushOnInterval(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &WebhookConfig{
		URL:       server.URL,
		TLSVerify: false,
	}

	// Large batch size, short flush interval
	backend, err := newWebhookBackendUnsafe(config, 1000, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	// Write single event (won't trigger batch size)
	event := &StructuredAuditEvent{
		ID:        "interval-test",
		Timestamp: time.Now(),
		Type:      AuditEventToolCall,
		Resource:  "test",
		Result:    "success",
	}

	if err := backend.Write(event); err != nil {
		t.Errorf("write failed: %v", err)
	}

	// Wait for interval flush
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	count := requestCount
	mu.Unlock()

	if count == 0 {
		t.Error("expected flush on interval")
	}

	_ = backend.Close()
}

func TestConnectionFailureGraceful(t *testing.T) {
	// Point to non-existent server
	config := &WebhookConfig{
		URL:       "http://localhost:59999/nonexistent",
		TLSVerify: false,
	}

	backend, err := newWebhookBackendUnsafe(config, 1, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	event := &StructuredAuditEvent{
		ID:        "failure-test",
		Timestamp: time.Now(),
		Type:      AuditEventError,
		Resource:  "test",
		Result:    "failure",
	}

	// Should not block or panic
	err = backend.Write(event)
	if err != nil {
		t.Errorf("write should not fail synchronously: %v", err)
	}

	// Wait for async send attempt
	time.Sleep(150 * time.Millisecond)

	// Close should succeed
	if err := backend.Close(); err != nil {
		t.Errorf("close failed: %v", err)
	}
}

func TestDefaultSIEMConfig(t *testing.T) {
	config := DefaultSIEMConfig()

	if config.BatchSize != 100 {
		t.Errorf("expected batch size 100, got %d", config.BatchSize)
	}
	if config.FlushInterval != 5*time.Second {
		t.Errorf("expected flush interval 5s, got %v", config.FlushInterval)
	}
}

func TestWriteAfterClose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &WebhookConfig{
		URL:       server.URL,
		TLSVerify: false,
	}

	backend, err := newWebhookBackendUnsafe(config, 10, time.Second)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	_ = backend.Close()

	event := &StructuredAuditEvent{
		ID:        "after-close",
		Timestamp: time.Now(),
		Type:      AuditEventToolCall,
		Resource:  "test",
		Result:    "success",
	}

	err = backend.Write(event)
	if err == nil {
		t.Error("expected error when writing after close")
	}
}
