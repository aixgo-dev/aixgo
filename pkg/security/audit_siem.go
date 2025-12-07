package security

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// SIEMConfig holds configuration for SIEM backends
type SIEMConfig struct {
	// Elasticsearch configuration
	Elasticsearch *ElasticsearchConfig `yaml:"elasticsearch,omitempty"`

	// Splunk configuration
	Splunk *SplunkConfig `yaml:"splunk,omitempty"`

	// Generic webhook configuration
	Webhook *WebhookConfig `yaml:"webhook,omitempty"`

	// Batch configuration
	BatchSize     int           `yaml:"batch_size"`
	FlushInterval time.Duration `yaml:"flush_interval"`
}

// ElasticsearchConfig holds Elasticsearch-specific configuration
type ElasticsearchConfig struct {
	URLs      []string `yaml:"urls"`
	Index     string   `yaml:"index"`
	Username  string   `yaml:"username,omitempty"`
	Password  string   `yaml:"password,omitempty"`
	TLSVerify bool     `yaml:"tls_verify"`
}

// SplunkConfig holds Splunk HEC configuration
type SplunkConfig struct {
	URL       string `yaml:"url"`
	Token     string `yaml:"token"`
	Index     string `yaml:"index,omitempty"`
	Source    string `yaml:"source,omitempty"`
	TLSVerify bool   `yaml:"tls_verify"`
}

// WebhookConfig holds generic webhook configuration
type WebhookConfig struct {
	URL       string            `yaml:"url"`
	Method    string            `yaml:"method"`
	Headers   map[string]string `yaml:"headers,omitempty"`
	TLSVerify bool              `yaml:"tls_verify"`
}

// DefaultSIEMConfig returns sensible defaults
func DefaultSIEMConfig() *SIEMConfig {
	return &SIEMConfig{
		BatchSize:     100,
		FlushInterval: 5 * time.Second,
	}
}

// baseSIEMBackend provides common batching functionality
type baseSIEMBackend struct {
	batchSize     int
	flushInterval time.Duration
	buffer        []*StructuredAuditEvent
	mu            sync.Mutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	sendFn        func([]*StructuredAuditEvent) error
	closed        bool
}

func newBaseSIEMBackend(batchSize int, flushInterval time.Duration, sendFn func([]*StructuredAuditEvent) error) *baseSIEMBackend {
	if batchSize <= 0 {
		batchSize = 100
	}
	if flushInterval <= 0 {
		flushInterval = 5 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())
	b := &baseSIEMBackend{
		batchSize:     batchSize,
		flushInterval: flushInterval,
		buffer:        make([]*StructuredAuditEvent, 0, batchSize),
		ctx:           ctx,
		cancel:        cancel,
		sendFn:        sendFn,
	}

	b.wg.Add(1)
	go b.flushLoop()

	return b
}

func (b *baseSIEMBackend) Write(event *StructuredAuditEvent) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return fmt.Errorf("backend is closed")
	}

	b.buffer = append(b.buffer, event)

	if len(b.buffer) >= b.batchSize {
		return b.flushLocked()
	}

	return nil
}

func (b *baseSIEMBackend) flushLoop() {
	defer b.wg.Done()
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			b.mu.Lock()
			if len(b.buffer) > 0 {
				_ = b.flushLocked() // Log errors but don't fail
			}
			b.mu.Unlock()
		}
	}
}

func (b *baseSIEMBackend) flushLocked() error {
	if len(b.buffer) == 0 {
		return nil
	}

	events := b.buffer
	b.buffer = make([]*StructuredAuditEvent, 0, b.batchSize)

	// Send asynchronously to not block
	go func() {
		if err := b.sendFn(events); err != nil {
			// Log error but don't fail - audit should not block main flow
			fmt.Printf("SIEM send error: %v\n", err)
		}
	}()

	return nil
}

func (b *baseSIEMBackend) Close() error {
	b.mu.Lock()
	b.closed = true
	// Flush remaining events synchronously on close
	if len(b.buffer) > 0 {
		events := b.buffer
		b.buffer = nil
		b.mu.Unlock()
		_ = b.sendFn(events)
	} else {
		b.mu.Unlock()
	}

	b.cancel()
	b.wg.Wait()
	return nil
}

// ElasticsearchBackend sends audit events to Elasticsearch
type ElasticsearchBackend struct {
	*baseSIEMBackend
	config *ElasticsearchConfig
	client *http.Client
}

// NewElasticsearchBackend creates a new Elasticsearch audit backend
func NewElasticsearchBackend(config *ElasticsearchConfig, batchSize int, flushInterval time.Duration) (*ElasticsearchBackend, error) {
	if config == nil || len(config.URLs) == 0 {
		return nil, fmt.Errorf("elasticsearch configuration with at least one URL is required")
	}
	if config.Index == "" {
		config.Index = "audit-logs"
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !config.TLSVerify,
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	backend := &ElasticsearchBackend{
		config: config,
		client: client,
	}

	backend.baseSIEMBackend = newBaseSIEMBackend(batchSize, flushInterval, backend.sendBatch)

	return backend, nil
}

func (b *ElasticsearchBackend) sendBatch(events []*StructuredAuditEvent) error {
	if len(events) == 0 {
		return nil
	}

	// Build bulk request body
	var buf bytes.Buffer
	for _, event := range events {
		// Action line
		meta := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": b.config.Index,
			},
		}
		metaJSON, _ := json.Marshal(meta)
		buf.Write(metaJSON)
		buf.WriteByte('\n')

		// Document line
		eventJSON, err := json.Marshal(event)
		if err != nil {
			continue
		}
		buf.Write(eventJSON)
		buf.WriteByte('\n')
	}

	// Try each URL until one succeeds
	var lastErr error
	for _, url := range b.config.URLs {
		req, err := http.NewRequest("POST", url+"/_bulk", &buf)
		if err != nil {
			lastErr = err
			continue
		}

		req.Header.Set("Content-Type", "application/x-ndjson")
		if b.config.Username != "" {
			req.SetBasicAuth(b.config.Username, b.config.Password)
		}

		resp, err := b.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		lastErr = fmt.Errorf("elasticsearch returned status %d", resp.StatusCode)
	}

	return fmt.Errorf("all elasticsearch URLs failed: %w", lastErr)
}

// SplunkBackend sends audit events to Splunk HEC
type SplunkBackend struct {
	*baseSIEMBackend
	config *SplunkConfig
	client *http.Client
}

// NewSplunkBackend creates a new Splunk HEC audit backend
func NewSplunkBackend(config *SplunkConfig, batchSize int, flushInterval time.Duration) (*SplunkBackend, error) {
	if config == nil || config.URL == "" {
		return nil, fmt.Errorf("splunk configuration with URL is required")
	}
	if config.Token == "" {
		return nil, fmt.Errorf("splunk HEC token is required")
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !config.TLSVerify,
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	backend := &SplunkBackend{
		config: config,
		client: client,
	}

	backend.baseSIEMBackend = newBaseSIEMBackend(batchSize, flushInterval, backend.sendBatch)

	return backend, nil
}

// SplunkEvent wraps audit event for Splunk HEC format
type SplunkEvent struct {
	Time   int64                 `json:"time"`
	Host   string                `json:"host,omitempty"`
	Source string                `json:"source,omitempty"`
	Index  string                `json:"index,omitempty"`
	Event  *StructuredAuditEvent `json:"event"`
}

func (b *SplunkBackend) sendBatch(events []*StructuredAuditEvent) error {
	if len(events) == 0 {
		return nil
	}

	// Build batch of Splunk events
	var buf bytes.Buffer
	for _, event := range events {
		splunkEvent := SplunkEvent{
			Time:   event.Timestamp.Unix(),
			Source: b.config.Source,
			Index:  b.config.Index,
			Event:  event,
		}

		eventJSON, err := json.Marshal(splunkEvent)
		if err != nil {
			continue
		}
		buf.Write(eventJSON)
	}

	req, err := http.NewRequest("POST", b.config.URL, &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Splunk "+b.config.Token)

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("splunk request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("splunk returned status %d", resp.StatusCode)
}

// WebhookBackend sends audit events to a generic webhook
type WebhookBackend struct {
	*baseSIEMBackend
	config *WebhookConfig
	client *http.Client
}

// NewWebhookBackend creates a new generic webhook audit backend
func NewWebhookBackend(config *WebhookConfig, batchSize int, flushInterval time.Duration) (*WebhookBackend, error) {
	if config == nil || config.URL == "" {
		return nil, fmt.Errorf("webhook configuration with URL is required")
	}
	if config.Method == "" {
		config.Method = "POST"
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !config.TLSVerify,
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	backend := &WebhookBackend{
		config: config,
		client: client,
	}

	backend.baseSIEMBackend = newBaseSIEMBackend(batchSize, flushInterval, backend.sendBatch)

	return backend, nil
}

// WebhookPayload wraps events for webhook delivery
type WebhookPayload struct {
	Events    []*StructuredAuditEvent `json:"events"`
	Timestamp time.Time               `json:"timestamp"`
	Count     int                     `json:"count"`
}

func (b *WebhookBackend) sendBatch(events []*StructuredAuditEvent) error {
	if len(events) == 0 {
		return nil
	}

	payload := WebhookPayload{
		Events:    events,
		Timestamp: time.Now(),
		Count:     len(events),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest(b.config.Method, b.config.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range b.config.Headers {
		req.Header.Set(k, v)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("webhook returned status %d", resp.StatusCode)
}
