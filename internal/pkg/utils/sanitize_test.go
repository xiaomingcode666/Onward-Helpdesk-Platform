package utils

import "testing"

func TestSanitizeForLogNeverKeepsSensitivePrefix(t *testing.T) {
	for _, limit := range []int{-1, 0, 1, 8, 4096} {
		got := SanitizeForLog("customer@example.test secret", limit)
		if len([]rune(got)) > max(0, limit) {
			t.Fatal("log value exceeded the limit")
		}
		if got != "" && got[0] != '[' {
			t.Fatal("log retained sensitive content")
		}
	}
	if SanitizeForLog("", 4096) != "" {
		t.Fatal("empty content must remain empty")
	}
}
