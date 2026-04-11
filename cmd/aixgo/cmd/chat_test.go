package cmd

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestChatSecretPattern locks in the categories chatSecretPattern is required
// to suppress (positive cases) AND the categories it must NOT suppress
// (negative cases). Keeping these as table tests makes future regex tweaks
// safe — any silent over- or under-matching will fail this test.
func TestChatSecretPattern(t *testing.T) {
	tests := []struct {
		name  string
		input string
		match bool
	}{
		// Positive: real-shaped secrets that must be filtered.
		{"openai sk-", "sk-abcdefghijklmnopqrstuvwxyz0123", true},
		{"anthropic sk-ant-", "sk-ant-api03-abcdefghijklmnopqrstuvwxyz", true},
		{"xai", "xai-abcdefghijklmnopqrstuvwxyz0123", true},
		{"github pat", "ghp_abcdefghijklmnopqrstuvwxyz", true},
		{"github server", "ghs_abcdefghijklmnopqrstuvwxyz", true},
		{"aws akia", "AKIAIOSFODNN7EXAMPLE", true},
		{"slack bot", "xoxb-1234567890-abcdefghij", true},
		{"slack user", "xoxp-1234567890-abcdefghij", true},
		{"bearer token", "Authorization: Bearer abcdefghijklmnopqrstuvwxyz", true},
		{"jwt", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0", true},
		{"pem", "-----BEGIN RSA PRIVATE KEY-----", true},
		{"pem generic", "-----BEGIN PRIVATE KEY-----", true},

		// Negative: ordinary prose and short strings must NOT be filtered.
		{"empty", "", false},
		{"plain question", "How do I configure an API key for OpenAI?", false},
		{"sk- prefix only", "the sk- prefix is short for secret key", false},
		{"slash command", "/help", false},
		{"go code", "import \"github.com/aixgo-dev/aixgo\"", false},
		{"akia too short", "AKIA12345", false},
		{"bearer prose", "the bearer of the message left", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chatSecretPattern.MatchString(tt.input)
			if got != tt.match {
				t.Errorf("chatSecretPattern.MatchString(%q) = %v, want %v", tt.input, got, tt.match)
			}
		})
	}
}

// TestTruncateForOutput covers the soft byte cap applied to non-interactive
// chat responses. The function must be a no-op when the cap is disabled or
// the content fits, must truncate oversized content, and must always return
// valid UTF-8 (never slice in the middle of a multi-byte rune).
func TestTruncateForOutput(t *testing.T) {
	t.Run("disabled cap passes through", func(t *testing.T) {
		in := strings.Repeat("x", 10_000)
		got, trunc := truncateForOutput(in, 0)
		if got != in || trunc {
			t.Errorf("cap=0: got trunc=%v len=%d, want passthrough", trunc, len(got))
		}
	})

	t.Run("negative cap passes through", func(t *testing.T) {
		got, trunc := truncateForOutput("hello", -1)
		if got != "hello" || trunc {
			t.Errorf("cap=-1: got trunc=%v content=%q, want passthrough", trunc, got)
		}
	})

	t.Run("under cap unchanged", func(t *testing.T) {
		in := strings.Repeat("x", 500)
		got, trunc := truncateForOutput(in, 1)
		if got != in || trunc {
			t.Errorf("under cap: got trunc=%v len=%d, want unchanged", trunc, len(got))
		}
	})

	t.Run("over cap truncated", func(t *testing.T) {
		in := strings.Repeat("x", 3000)
		got, trunc := truncateForOutput(in, 1)
		if !trunc {
			t.Error("over cap: expected truncated=true")
		}
		if len(got) > 1024 {
			t.Errorf("over cap: got len=%d, want <=1024", len(got))
		}
	})

	t.Run("multibyte rune boundary", func(t *testing.T) {
		// Japanese chars are 3 bytes in UTF-8. Build content that would slice
		// a rune if truncation used a raw byte cut.
		in := strings.Repeat("あ", 500) // 1500 bytes
		got, trunc := truncateForOutput(in, 1)
		if !trunc {
			t.Error("expected truncated=true")
		}
		if !utf8.ValidString(got) {
			t.Errorf("truncated output is not valid UTF-8: %q", got)
		}
		if len(got) > 1024 {
			t.Errorf("got len=%d, want <=1024", len(got))
		}
	})
}
