package migration

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestMeetingActiveUniqueness(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.MeetingRoomJitsi{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	now := time.Now()
	first := meetingIndexFixture("meeting-1", "active", now)
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first active meeting: %v", err)
	}
	duplicate := meetingIndexFixture("meeting-2", "scheduled", now.Add(time.Minute))
	if err := db.Create(&duplicate).Error; err != nil {
		t.Fatalf("seed historical duplicate meeting: %v", err)
	}
	if err := ensureMeetingActiveUniqueness(db); err != nil {
		t.Fatalf("ensureMeetingActiveUniqueness() error = %v", err)
	}
	var repaired models.MeetingRoomJitsi
	if err := db.First(&repaired, "id = ?", first.ID).Error; err != nil || repaired.Status != "ended" || repaired.EndedAt == nil {
		t.Fatalf("older duplicate meeting was not ended: meeting=%+v err=%v", repaired, err)
	}
	secondActive := meetingIndexFixture("meeting-4", "active", now.Add(2*time.Minute))
	if err := db.Create(&secondActive).Error; err == nil {
		t.Fatal("partial unique index accepted a second active meeting")
	}
	if err := db.Model(&duplicate).Update("status", "ended").Error; err != nil {
		t.Fatalf("end first meeting: %v", err)
	}
	next := meetingIndexFixture("meeting-3", "active", now.Add(3*time.Minute))
	if err := db.Create(&next).Error; err != nil {
		t.Fatalf("create meeting after previous meeting ended: %v", err)
	}
	if !db.Migrator().HasIndex(&models.MeetingRoomJitsi{}, "uk_meeting_tenant_ticket_active") {
		t.Fatal("active meeting partial unique index was not created")
	}
}

func meetingIndexFixture(id, status string, createdAt time.Time) models.MeetingRoomJitsi {
	return models.MeetingRoomJitsi{
		ID:        id,
		TenantID:  1,
		TicketID:  "9",
		RoomName:  "room-" + id,
		Status:    status,
		CreatedBy: "11",
		StartedAt: &createdAt,
		BaseModel: models.BaseModel{
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		},
	}
}
