package event_handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestMeetingEndedTimelineHandlerIsIdempotent(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.MeetingRoomJitsi{}, &models.TicketProgress{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqls.SetDB(db)
	meeting := &models.MeetingRoomJitsi{
		ID:       "meeting-idempotent",
		TenantID: 9,
		TicketID: "42",
		RoomName: "repair-room",
		Status:   "ended",
		BaseModel: models.BaseModel{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	if err := db.Create(meeting).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	event := events.MeetingEndedEvent{
		MeetingID:  meeting.ID,
		TicketID:   meeting.TicketID,
		TenantID:   "9",
		DurationMs: 90_000,
	}
	for i := 0; i < 2; i++ {
		if err := handleMeetingEndedWriteTicketTimeline(context.Background(), event); err != nil {
			t.Fatalf("handle meeting event %d: %v", i, err)
		}
	}
	var count int64
	if err := db.Model(&models.TicketProgress{}).
		Where("ticket_id = ? AND event_type = ?", 42, enums.TicketProgressEventMeetingEnded).
		Count(&count).Error; err != nil {
		t.Fatalf("count timeline entries: %v", err)
	}
	if count != 1 {
		t.Fatalf("timeline entries = %d, want 1", count)
	}
}
