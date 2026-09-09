package migration

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestNormalizeMeetingTranscriptOffsets(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.MeetingRoomJitsi{}, &models.MeetingTranscriptSegment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	startedAt := time.UnixMilli(1_700_000_000_000)
	meeting := models.MeetingRoomJitsi{
		ID: "meeting-offset", TenantID: 7, TicketID: "42", RoomName: "meeting-offset", Status: "ended",
		StartedAt: &startedAt, BaseModel: models.BaseModel{CreatedAt: startedAt, UpdatedAt: startedAt},
	}
	if err := db.Create(&meeting).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	if db.Migrator().HasIndex(&models.MeetingTranscriptSegment{}, "uk_meeting_transcript_event_v2") {
		t.Fatal("AutoMigrate must not create the unique index before legacy rows are deduplicated")
	}
	segment := models.MeetingTranscriptSegment{
		ID: "segment-offset", TenantID: 7, MeetingID: meeting.ID, ParticipantID: "1",
		Provider: "jitsi_caption", ProviderEventID: "event-1", Language: "zh-CN", Text: "测试",
		IsFinal: true, StartedAtMS: startedAt.Add(1500 * time.Millisecond).UnixMilli(),
		EndedAtMS: startedAt.Add(1800 * time.Millisecond).UnixMilli(),
		BaseModel: models.BaseModel{CreatedAt: startedAt, UpdatedAt: startedAt},
	}
	if err := db.Create(&segment).Error; err != nil {
		t.Fatalf("create segment: %v", err)
	}
	duplicate := segment
	duplicate.ID = "segment-offset-translated"
	duplicate.ParticipantID = "2"
	duplicate.RawJSON = `{"providerEventId":"event-1"}`
	duplicate.TranslationStatus = "completed"
	duplicate.TranslatedText = "Test"
	if err := db.Create(&duplicate).Error; err != nil {
		t.Fatalf("create duplicate segment: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := hardenMeetingTranscriptPersistence(db); err != nil {
			t.Fatalf("normalize pass %d: %v", i, err)
		}
	}
	var segments []models.MeetingTranscriptSegment
	if err := db.Find(&segments).Error; err != nil {
		t.Fatalf("reload segments: %v", err)
	}
	if len(segments) != 1 || segments[0].ID != duplicate.ID {
		t.Fatalf("deduplicated segments = %#v", segments)
	}
	if segments[0].StartedAtMS != 1500 || segments[0].EndedAtMS != 1800 {
		t.Fatalf("normalized offsets = %d..%d", segments[0].StartedAtMS, segments[0].EndedAtMS)
	}
	if !db.Migrator().HasIndex(&models.MeetingTranscriptSegment{}, "uk_meeting_transcript_event_v2") {
		t.Fatal("current transcript event unique index was not created")
	}
}
