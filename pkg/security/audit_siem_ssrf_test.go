package security

import (
	"strings"
	"testing"
	"time"
)

// Regression coverage for Aikido finding #190 — "HTTP request might enable
// SSRF attack" in pkg/security/audit_siem.go. The SIEM backends construct
// HTTP clients using arbitrary configuration URLs, so they MUST:
//
//  1. reject loopback / private / link-local / metadata destinations at
//     construction time (ValidateSIEMURL, wired via the public New* ctors);
//  2. refuse to disable TLS verification when ENVIRONMENT is unset or set
//     to "production" (fail-safe).
//
// These properties are enforced in audit_siem.go. If someone removes
// ValidateSIEMURL from a backend ctor, or loosens the production TLS check,
// this test will fail.

func TestValidateSIEMURL_RejectsSSRFDestinations(t *testing.T) {
	// Covers the most common SSRF targets: loopback, RFC1918, link-local
	// (including the AWS metadata address), IPv6 loopback, and schemes
	// other than http/https.
	cases := []struct {
		name string
		url  string
	}{
		{"loopback ipv4", "http://127.0.0.1:9200"},
		{"loopback hostname", "http://localhost:8088/services/collector"},
		{"rfc1918 10.x", "https://10.0.0.5/_bulk"},
		{"rfc1918 192.168", "https://192.168.1.10/_bulk"},
		{"rfc1918 172.16", "https://172.16.0.1/_bulk"},
		{"link-local", "http://169.254.169.254/latest/meta-data/"}, // AWS IMDS
		{"ipv6 loopback", "http://[::1]:9200"},
		{"file scheme", "file:///etc/passwd"},
		{"gopher scheme", "gopher://evil.example/"},
		{"empty host", "http:///path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateSIEMURL(tc.url); err == nil {
				t.Fatalf("ValidateSIEMURL(%q) returned nil error, want rejection", tc.url)
			}
		})
	}
}

func TestSIEMBackends_RejectSSRFURLsAtConstruction(t *testing.T) {
	// Every public backend constructor must run ValidateSIEMURL; otherwise
	// a caller-supplied loopback URL could be used to exfiltrate secrets
	// (Splunk HEC token, basic-auth creds) to 127.0.0.1:<attacker-process>.
	//
	// The test env is set so the TLS guard does NOT short-circuit first.
	t.Setenv("ENVIRONMENT", "test")

	loopback := "http://127.0.0.1:9200"

	t.Run("elasticsearch", func(t *testing.T) {
		_, err := NewElasticsearchBackend(&ElasticsearchConfig{
			URLs:      []string{loopback},
			Index:     "audit",
			TLSVerify: true,
		}, 1, time.Millisecond)
		if err == nil {
			t.Fatal("expected SSRF rejection for loopback Elasticsearch URL")
		}
	})

	t.Run("splunk", func(t *testing.T) {
		_, err := NewSplunkBackend(&SplunkConfig{
			URL:       loopback,
			Token:     "tok",
			TLSVerify: true,
		}, 1, time.Millisecond)
		if err == nil {
			t.Fatal("expected SSRF rejection for loopback Splunk URL")
		}
	})

	t.Run("webhook", func(t *testing.T) {
		_, err := NewWebhookBackend(&WebhookConfig{
			URL:       loopback,
			Method:    "POST",
			TLSVerify: true,
		}, 1, time.Millisecond)
		if err == nil {
			t.Fatal("expected SSRF rejection for loopback webhook URL")
		}
	})
}

func TestSIEMBackends_RejectInsecureTLSInProduction(t *testing.T) {
	// TLSVerify=false must be refused unless ENVIRONMENT is explicitly one
	// of development / dev / staging / local / test. An unset ENVIRONMENT
	// is treated as production (fail-safe).
	t.Setenv("ENVIRONMENT", "") // treated as production

	// Use a non-private hostname so ValidateSIEMURL succeeds and we actually
	// reach the TLS guard. example.com is a stable IANA-reserved host.
	public := "https://example.com/_bulk"

	t.Run("elasticsearch", func(t *testing.T) {
		_, err := NewElasticsearchBackend(&ElasticsearchConfig{
			URLs:      []string{public},
			Index:     "audit",
			TLSVerify: false,
		}, 1, time.Millisecond)
		if err == nil || !strings.Contains(err.Error(), "TLS certificate verification cannot be disabled") {
			t.Fatalf("expected production TLS guard to reject InsecureSkipVerify, got %v", err)
		}
	})

	t.Run("splunk", func(t *testing.T) {
		_, err := NewSplunkBackend(&SplunkConfig{
			URL:       "https://example.com/services/collector",
			Token:     "tok",
			TLSVerify: false,
		}, 1, time.Millisecond)
		if err == nil || !strings.Contains(err.Error(), "TLS certificate verification cannot be disabled") {
			t.Fatalf("expected production TLS guard to reject InsecureSkipVerify, got %v", err)
		}
	})

	t.Run("webhook", func(t *testing.T) {
		_, err := NewWebhookBackend(&WebhookConfig{
			URL:       "https://example.com/hook",
			Method:    "POST",
			TLSVerify: false,
		}, 1, time.Millisecond)
		if err == nil || !strings.Contains(err.Error(), "TLS certificate verification cannot be disabled") {
			t.Fatalf("expected production TLS guard to reject InsecureSkipVerify, got %v", err)
		}
	})
}
