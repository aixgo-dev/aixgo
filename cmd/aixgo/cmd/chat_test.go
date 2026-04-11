package cmd

import "testing"

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
