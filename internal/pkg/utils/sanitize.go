package utils

import "remotehelpdesk/internal/pkg/logprivacy"

// SanitizeForLog omits untrusted content rather than merely truncating it.
// The limit is retained for callers, but never allows a prefix of the raw input.
func SanitizeForLog(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	safe := []rune(logprivacy.Value(s))
	if len(safe) > maxLen {
		safe = safe[:maxLen]
	}
	return string(safe)
}
