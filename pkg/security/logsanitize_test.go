package security

import (
	"strings"
	"testing"
)

func TestSanitizeLogField(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty passes through", "", ""},
		{"plain ascii preserved", "production", "production"},
		{"newline removed", "foo\nbar", "foobar"},
		{"crlf removed", "foo\r\nbar", "foobar"},
		{"carriage return removed", "foo\rbar", "foobar"},
		{"tab becomes space", "foo\tbar", "foo bar"},
		{"null byte removed", "foo\x00bar", "foobar"},
		{"low ascii control removed", "foo\x01\x02\x03bar", "foobar"},
		{"del (0x7f) removed", "foo\x7fbar", "foobar"},
		{"non-ascii preserved", "production-日本", "production-日本"},
		{"mixed attacker payload", "dev\n[FAKE] user=root\nreal=", "dev[FAKE] user=rootreal="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeLogField(tt.in); got != tt.want {
				t.Errorf("SanitizeLogField(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}

	t.Run("long value is truncated", func(t *testing.T) {
		in := strings.Repeat("a", maxLogFieldLength+50)
		got := SanitizeLogField(in)
		if !strings.HasSuffix(got, "…(truncated)") {
			t.Errorf("expected truncation suffix, got %q", got[len(got)-20:])
		}
		// Runtime note: maxLogFieldLength is byte-counted on the sanitised
		// prefix; the suffix "…(truncated)" contains a multi-byte ellipsis.
		prefix := strings.TrimSuffix(got, "…(truncated)")
		if len(prefix) != maxLogFieldLength {
			t.Errorf("prefix len = %d, want %d", len(prefix), maxLogFieldLength)
		}
	})
}
