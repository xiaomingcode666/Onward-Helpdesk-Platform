package migration

import (
	"testing"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestHardenMeetingTranscriptRuntime(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE meeting_transcript_segments (
		id text PRIMARY KEY,
		tenant_id integer NOT NULL,
		meeting_id text NOT NULL,
		provider text NOT NULL,
		provider_event_id text NOT NULL,
		started_at_ms integer NOT NULL DEFAULT 0,
		created_at datetime NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create legacy transcript table: %v", err)
	}
	if err := db.Exec("CREATE INDEX uk_meeting_transcript_provider_event ON meeting_transcript_segments (meeting_id, provider, provider_event_id)").Error; err != nil {
		t.Fatalf("create legacy index: %v", err)
	}

	for pass := 0; pass < 2; pass++ {
		if err := hardenMeetingTranscriptRuntime(db); err != nil {
			t.Fatalf("harden pass %d: %v", pass, err)
		}
	}
	for _, field := range []string{"TranslationRetryCount", "TranslationNextRetryAt", "TranslationLeaseUntil"} {
		if !db.Migrator().HasColumn(&models.MeetingTranscriptSegment{}, field) {
			t.Fatalf("column %s was not added", field)
		}
	}
	if db.Migrator().HasIndex(&models.MeetingTranscriptSegment{}, "uk_meeting_transcript_provider_event") {
		t.Fatal("legacy transcript index was not removed")
	}
	if !db.Migrator().HasIndex(&models.MeetingTranscriptSegment{}, "idx_meeting_transcript_timeline") {
		t.Fatal("transcript timeline index was not created")
	}
}
