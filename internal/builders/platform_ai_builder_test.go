package builders

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/services"
)

func TestBuildPlatformSub2APIUserKeyListMasksCredential(t *testing.T) {
	lastUsedAt := "2026-08-03T21:26:39+08:00"
	aggregate := &services.PlatformSub2APIUserKeyListAggregate{
		GeneratedAt: time.Now(),
		UserID:      15,
		Total:       1,
		Page:        1,
		PageSize:    20,
		Pages:       1,
		Keys: []providers.Sub2APIKey{{
			ID:         42,
			UserID:     15,
			Key:        "sk-a4f02571f488d70a3514021349a07bf1db74f72adfa2f86c3dd38a6892f5522e",
			Name:       "Work",
			Status:     "active",
			LastUsedAt: &lastUsedAt,
		}},
	}

	result := BuildPlatformSub2APIUserKeyList(aggregate)
	if result == nil || len(result.Items) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if strings.Contains(result.Items[0].KeyPreview, "a4f02571") {
		t.Fatalf("key preview leaked credential: %s", result.Items[0].KeyPreview)
	}
	if result.Items[0].KeyPreview != "sk-a4f0…f5522e" {
		t.Fatalf("key preview = %s", result.Items[0].KeyPreview)
	}
}
