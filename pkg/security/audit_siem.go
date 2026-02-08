package security

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// ValidateSIEMURL validates that a URL is safe for SIEM connections
// and prevents SSRF attacks by blocking private IP ranges
func ValidateSIEMURL(rawURL string) error {
	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Only allow http and https schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme: %s (only http/https allowed)", parsedURL.Scheme)
	}

	// Extract hostname (may include port)
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL must contain a hostname")
	}

	// Resolve hostname to IP addresses
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname %s: %w", hostname, err)
	}

	// Check each resolved IP address
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("URL resolves to private IP address %s (potential SSRF)", ip.String())
		}
	}

	return nil
}

// isPrivateIP checks if an IP address is in a private range
func isPrivateIP(ip net.IP) bool {
	// Check for IPv4 private ranges
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return true
	}

	// Additional checks for special ranges
	if ip.To4() != nil {
		// 0.0.0.0/8
		if ip[0] == 0 {
			return true
		}
		// 169.254.0.0/16 (link-local)
		if ip[0] == 169 && ip[1] == 254 {
			return true
		}
		// 127.0.0.0/8 (loopback)
		if ip[0] == 127 {
			return true
		}
		// 10.0.0.0/8
		if ip[0] == 10 {
			return true
		}
		// 172.16.0.0/12
		if ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31 {
			return true
		}
		// 192.168.0.0/16
		if ip[0] == 192 && ip[1] == 168 {
			return true
		}
		// 224.0.0.0/4 (multicast)
		if ip[0] >= 224 && ip[0] <= 239 {
			return true
		}
		// 240.0.0.0/4 (reserved)
		if ip[0] >= 240 {
			return true
		}
	}

	// Check for IPv6 private ranges
	if len(ip) == net.IPv6len {
		// ::1/128 (loopback)
		if ip.IsLoopback() {
			return true
		}
		// fe80::/10 (link-local)
		if len(ip) >= 2 && ip[0] == 0xfe && (ip[1]&0xc0) == 0x80 {
			return true
		}
		// fc00::/7 (unique local)
		if len(ip) >= 1 && (ip[0]&0xfe) == 0xfc {
			return true
		}
		// ff00::/8 (multicast)
		if len(ip) >= 1 && ip[0] == 0xff {
			return true
		}
	}

	return false
}

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
	return newElasticsearchBackend(config, batchSize, flushInterval, true)
}

// newElasticsearchBackendUnsafe creates an Elasticsearch backend without URL validation (for testing only)
func newElasticsearchBackendUnsafe(config *ElasticsearchConfig, batchSize int, flushInterval time.Duration) (*ElasticsearchBackend, error) {
	return newElasticsearchBackend(config, batchSize, flushInterval, false)
}

// newElasticsearchBackend is the internal constructor with optional validation
func newElasticsearchBackend(config *ElasticsearchConfig, batchSize int, flushInterval time.Duration, validateURLs bool) (*ElasticsearchBackend, error) {
	if config == nil || len(config.URLs) == 0 {
		return nil, fmt.Errorf("elasticsearch configuration with at least one URL is required")
	}
	if config.Index == "" {
		config.Index = "audit-logs"
	}

	// Validate each Elasticsearch URL to prevent SSRF (unless disabled for testing)
	if validateURLs {
		for _, esURL := range config.URLs {
			if err := ValidateSIEMURL(esURL); err != nil {
				return nil, fmt.Errorf("invalid Elasticsearch URL %s: %w", esURL, err)
			}
		}
	}

	// Log warning if TLS verification is disabled
	if !config.TLSVerify {
		log.Printf("WARNING: Elasticsearch TLS certificate verification is disabled (TLSVerify=false). "+
			"This is a security risk and should NEVER be used in production. "+
			"Connections are vulnerable to man-in-the-middle attacks.")
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !config.TLSVerify, //nolint:gosec // G402: intentionally configurable for dev/test
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
	return newSplunkBackend(config, batchSize, flushInterval, true)
}

// newSplunkBackendUnsafe creates a Splunk backend without URL validation (for testing only)
func newSplunkBackendUnsafe(config *SplunkConfig, batchSize int, flushInterval time.Duration) (*SplunkBackend, error) {
	return newSplunkBackend(config, batchSize, flushInterval, false)
}

// newSplunkBackend is the internal constructor with optional validation
func newSplunkBackend(config *SplunkConfig, batchSize int, flushInterval time.Duration, validateURL bool) (*SplunkBackend, error) {
	if config == nil || config.URL == "" {
		return nil, fmt.Errorf("splunk configuration with URL is required")
	}
	if config.Token == "" {
		return nil, fmt.Errorf("splunk HEC token is required")
	}

	// Validate Splunk URL to prevent SSRF (unless disabled for testing)
	if validateURL {
		if err := ValidateSIEMURL(config.URL); err != nil {
			return nil, fmt.Errorf("invalid Splunk URL: %w", err)
		}
	}

	// Log warning if TLS verification is disabled
	if !config.TLSVerify {
		log.Printf("WARNING: Splunk TLS certificate verification is disabled (TLSVerify=false). "+
			"This is a security risk and should NEVER be used in production. "+
			"Connections are vulnerable to man-in-the-middle attacks.")
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !config.TLSVerify, //nolint:gosec // G402: intentionally configurable for dev/test
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
	return newWebhookBackend(config, batchSize, flushInterval, true)
}

// newWebhookBackendUnsafe creates a webhook backend without URL validation (for testing only)
func newWebhookBackendUnsafe(config *WebhookConfig, batchSize int, flushInterval time.Duration) (*WebhookBackend, error) {
	return newWebhookBackend(config, batchSize, flushInterval, false)
}

// newWebhookBackend is the internal constructor with optional validation
func newWebhookBackend(config *WebhookConfig, batchSize int, flushInterval time.Duration, validateURL bool) (*WebhookBackend, error) {
	if config == nil || config.URL == "" {
		return nil, fmt.Errorf("webhook configuration with URL is required")
	}
	if config.Method == "" {
		config.Method = "POST"
	}

	// Validate webhook URL to prevent SSRF (unless disabled for testing)
	if validateURL {
		if err := ValidateSIEMURL(config.URL); err != nil {
			return nil, fmt.Errorf("invalid webhook URL: %w", err)
		}
	}

	// Log warning if TLS verification is disabled
	if !config.TLSVerify {
		log.Printf("WARNING: Webhook TLS certificate verification is disabled (TLSVerify=false). "+
			"This is a security risk and should NEVER be used in production. "+
			"Connections are vulnerable to man-in-the-middle attacks.")
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !config.TLSVerify, //nolint:gosec // G402: intentionally configurable for dev/test
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
