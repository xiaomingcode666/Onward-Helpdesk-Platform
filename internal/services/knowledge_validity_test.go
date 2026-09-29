package services

import (
	"testing"
	"time"

	"remotehelpdesk/internal/pkg/enums"
)

func TestKnowledgeEntryUsableRequiresPublishedCurrentRevisionAndValidity(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	tests := []struct {
		name        string
		status      string
		entryStatus enums.Status
		revision    int64
		expires     *time.Time
		want        bool
	}{
		{"published future", "published", enums.StatusOk, 1, &future, true},
		{"draft blocked", "draft", enums.StatusOk, 1, &future, false},
		{"disabled blocked", "published", enums.StatusDisabled, 1, &future, false},
		{"missing revision blocked", "published", enums.StatusOk, 0, &future, false},
		{"expired blocked", "published", enums.StatusOk, 1, &past, false},
		{"no expiry", "published", enums.StatusOk, 1, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := KnowledgeEntryUsable(tt.status, tt.entryStatus, tt.revision, tt.expires, now); got != tt.want {
				t.Fatalf("usable = %v, want %v", got, tt.want)
			}
		})
	}
}
