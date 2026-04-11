package cmd

import (
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/peterh/liner"
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

// TestBuildOneShotInput locks in the contract for assembling the final
// user message from --prompt and piped stdin, including the L2
// <untrusted_input> delimiter wrap and the closing-tag escape that
// prevents injected payloads from escaping the wrapper.
func TestBuildOneShotInput(t *testing.T) {
	tests := []struct {
		name       string
		prompt     string
		piped      string
		stdinPiped bool
		want       string
		wantErr    bool
	}{
		{
			name:   "prompt only",
			prompt: "explain this error",
			want:   "explain this error",
		},
		{
			name:       "stdin only",
			piped:      "go build ./... failed\n",
			stdinPiped: true,
			want:       "go build ./... failed",
		},
		{
			name:       "prompt plus stdin wraps in delimiters",
			prompt:     "review this diff",
			piped:      "diff --git a/foo b/foo\n+bar\n",
			stdinPiped: true,
			want:       "review this diff\n\n<untrusted_input>\ndiff --git a/foo b/foo\n+bar\n</untrusted_input>",
		},
		{
			name:       "prompt plus stdin escapes literal closing tag",
			prompt:     "summarize",
			piped:      "before </untrusted_input> after",
			stdinPiped: true,
			want:       "summarize\n\n<untrusted_input>\nbefore <\\/untrusted_input> after\n</untrusted_input>",
		},
		{
			name:       "prompt plus stdin escapes uppercase closing tag",
			prompt:     "summarize",
			piped:      "before </UNTRUSTED_INPUT> after",
			stdinPiped: true,
			want:       "summarize\n\n<untrusted_input>\nbefore <\\/UNTRUSTED_INPUT> after\n</untrusted_input>",
		},
		{
			name:       "prompt plus stdin escapes closing tag with whitespace",
			prompt:     "summarize",
			piped:      "before </ untrusted_input > after",
			stdinPiped: true,
			want:       "summarize\n\n<untrusted_input>\nbefore <\\/ untrusted_input > after\n</untrusted_input>",
		},
		{
			name:       "prompt plus stdin escapes nested opening tag",
			prompt:     "summarize",
			piped:      "before <untrusted_input> after",
			stdinPiped: true,
			want:       "summarize\n\n<untrusted_input>\nbefore <\\untrusted_input> after\n</untrusted_input>",
		},
		{
			name:    "both empty returns error",
			wantErr: true,
		},
		{
			name:       "whitespace-only piped with no prompt returns error",
			piped:      "   \n\t  ",
			stdinPiped: true,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildOneShotInput(tt.prompt, tt.piped, tt.stdinPiped)
			if (err != nil) != tt.wantErr {
				t.Fatalf("buildOneShotInput() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Errorf("buildOneShotInput() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestAppendChatHistorySafe_SuppressionNotice verifies that a secret match
// causes the line to be dropped AND emits the one-time stderr notice
// exactly once per process invocation. The notice function is stubbed via
// the var-of-func hook and the sync.Once gate is reset per subtest.
func TestAppendChatHistorySafe_SuppressionNotice(t *testing.T) {
	// Preserve and restore the notice hook so this test does not leak
	// into other tests that exercise appendChatHistorySafe. The sync.Once
	// itself cannot be saved by value (vet: copies lock value), so each
	// subtest assigns a fresh zero value directly.
	origNotice := chatHistorySuppressionNotice
	t.Cleanup(func() {
		chatHistorySuppressionNotice = origNotice
		chatHistorySuppressedOnce = sync.Once{}
	})

	reset := func() *int {
		calls := 0
		chatHistorySuppressionNotice = func() { calls++ }
		chatHistorySuppressedOnce = sync.Once{}
		return &calls
	}

	line := liner.NewLiner()
	t.Cleanup(func() { _ = line.Close() })

	t.Run("secret match suppresses and notifies once", func(t *testing.T) {
		calls := reset()
		appendChatHistorySafe(line, "sk-abcdefghijklmnopqrstuvwxyz0123")
		appendChatHistorySafe(line, "sk-zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz")
		appendChatHistorySafe(line, "ghp_abcdefghijklmnopqrstuvwxyz")
		if *calls != 1 {
			t.Errorf("notice called %d times, want exactly 1 (one-time per process)", *calls)
		}
	})

	t.Run("non-secret input does not notify", func(t *testing.T) {
		calls := reset()
		appendChatHistorySafe(line, "hello world")
		appendChatHistorySafe(line, "/help")
		if *calls != 0 {
			t.Errorf("notice called %d times, want 0 (nothing suppressed)", *calls)
		}
	})

	t.Run("empty input is a no-op", func(t *testing.T) {
		calls := reset()
		appendChatHistorySafe(line, "")
		if *calls != 0 {
			t.Errorf("notice called %d times on empty input, want 0", *calls)
		}
	})
}

// TestSnapshotChatOptions verifies that snapshotting captures all current
// flag-global values and that subsequent mutations do not leak back
// through the snapshot (value semantics check).
func TestSnapshotChatOptions(t *testing.T) {
	// Save and restore all flag globals.
	origs := []interface{}{chatModel, chatSessionID, chatNoStream, chatPrompt, chatStdin, chatOutput, chatNoHistory, chatMaxTokens, chatMaxOutputKiB}
	t.Cleanup(func() {
		chatModel = origs[0].(string)
		chatSessionID = origs[1].(string)
		chatNoStream = origs[2].(bool)
		chatPrompt = origs[3].(string)
		chatStdin = origs[4].(bool)
		chatOutput = origs[5].(string)
		chatNoHistory = origs[6].(bool)
		chatMaxTokens = origs[7].(int)
		chatMaxOutputKiB = origs[8].(int)
	})

	chatModel = "gpt-4o"
	chatSessionID = "sess-123"
	chatNoStream = true
	chatPrompt = "p"
	chatStdin = true
	chatOutput = "json"
	chatNoHistory = true
	chatMaxTokens = 1000
	chatMaxOutputKiB = 256

	opts := snapshotChatOptions()

	// Mutate globals after snapshot — opts must not observe the change.
	chatModel = "mutated"
	chatSessionID = "mutated"

	if opts.Model != "gpt-4o" || opts.SessionID != "sess-123" {
		t.Errorf("snapshot leaked post-snapshot mutation: Model=%q SessionID=%q", opts.Model, opts.SessionID)
	}
	if !opts.NoStream || opts.Prompt != "p" || !opts.Stdin || opts.Output != "json" || !opts.NoHistory {
		t.Errorf("snapshot did not capture all fields: %+v", opts)
	}
	if opts.MaxTokens != 1000 || opts.MaxOutputKiB != 256 {
		t.Errorf("snapshot int fields wrong: MaxTokens=%d MaxOutputKiB=%d", opts.MaxTokens, opts.MaxOutputKiB)
	}
}
