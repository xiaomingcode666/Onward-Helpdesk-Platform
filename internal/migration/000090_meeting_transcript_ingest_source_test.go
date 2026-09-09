package migration

import (
	"testing"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestBackfillMeetingTranscriptIngestSource(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.MeetingTranscriptSegment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	items := []models.MeetingTranscriptSegment{
		{ID: "client", TenantID: 1, MeetingID: "meeting-1", Provider: "jitsi_caption", ProviderEventID: "client-event", IngestSource: "provider", Language: "zh-CN", Text: "客户端", RawJSON: `{"providerEventId":"client-event"}`},
		{ID: "callback", TenantID: 1, MeetingID: "meeting-1", Provider: "jitsi_caption", ProviderEventID: "callback-event", IngestSource: "provider", Language: "zh-CN", Text: "回调", RawJSON: "{}"},
	}
	if err := db.Create(&items).Error; err != nil {
		t.Fatalf("create transcripts: %v", err)
	}
	for pass := 0; pass < 2; pass++ {
		if err := backfillMeetingTranscriptIngestSource(db); err != nil {
			t.Fatalf("backfill pass %d: %v", pass, err)
		}
	}
	var reloaded []models.MeetingTranscriptSegment
	if err := db.Order("id ASC").Find(&reloaded).Error; err != nil {
		t.Fatalf("reload transcripts: %v", err)
	}
	if len(reloaded) != 2 || reloaded[1].IngestSource != "client" || reloaded[0].IngestSource != "provider" {
		t.Fatalf("ingest sources = %#v", reloaded)
	}
}
