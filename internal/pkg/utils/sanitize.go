package utils

// SanitizeForLog truncates a string to maxLen characters for safe storage in log tables.
// Returns empty string if input is empty. This prevents accidental storage of large
// sensitive payloads in access logs.
func SanitizeForLog(s string, maxLen int) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "...[truncated]"
	}
	return s
}
