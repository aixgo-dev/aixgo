package security

import "strings"

// maxLogFieldLength caps sanitised log field length to prevent log flooding
// from adversarial inputs. 256 bytes is enough for any legitimate config
// value (env name, region, project id, etc.) and short enough to keep log
// lines readable.
const maxLogFieldLength = 256

// SanitizeLogField returns v with CR/LF, tabs, and other control characters
// stripped so it is safe to embed in a single log line. This is the shared
// defence used to address gosec G706 (log injection via taint) across the
// codebase.
//
// The transformation is intentionally lossy:
//
//   - newline (\n) and carriage return (\r) are REMOVED to prevent an
//     attacker from forging additional log lines.
//   - tab (\t) is converted to a single space to keep columnar output
//     tidy without introducing whitespace anomalies.
//   - all other ASCII control characters (< 0x20) and DEL (0x7f) are
//     removed entirely.
//   - non-ASCII runes are passed through unchanged so international
//     identifiers (e.g. non-ASCII project names) still log legibly.
//   - strings longer than maxLogFieldLength are truncated and suffixed
//     with "…(truncated)".
//
// SanitizeLogField should be applied at every log site that embeds an
// operator-supplied value (env var, config file, CLI flag, HTTP header,
// etc.) — even values that look trusted — because gosec G706 uses taint
// analysis and cannot reason about allowlists or regex validation that
// happens further up the stack.
func SanitizeLogField(v string) string {
	if v == "" {
		return v
	}

	var b strings.Builder
	b.Grow(len(v))
	for _, r := range v {
		switch {
		case r == '\n' || r == '\r':
			// drop entirely; attackers cannot forge log lines.
		case r == '\t':
			b.WriteByte(' ')
		case r < 0x20 || r == 0x7f:
			// strip other control characters.
		default:
			b.WriteRune(r)
		}
	}

	out := b.String()
	if len(out) > maxLogFieldLength {
		out = out[:maxLogFieldLength] + "…(truncated)"
	}
	return out
}
