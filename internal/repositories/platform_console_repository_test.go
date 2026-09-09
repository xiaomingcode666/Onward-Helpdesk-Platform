package repositories

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetJitsiSnapshotTracksParticipantHeartbeats(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:platform_console_jitsi_heartbeat?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.MeetingRoomJitsi{}, &models.MeetingParticipant{}); err != nil {
		t.Fatalf("migrate meeting tables: %v", err)
	}
	now := time.Now().Truncate(time.Second)
	cutoff := now.Add(-3 * time.Minute)
	staleAt := cutoff.Add(-time.Minute)
	meetings := []models.MeetingRoomJitsi{
		{ID: "meeting-online", TenantID: 1, TicketID: "1", RoomName: "room-online", Status: "active", BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now}},
		{ID: "meeting-stale", TenantID: 1, TicketID: "2", RoomName: "room-stale", Status: "active", BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: staleAt}},
		{ID: "meeting-waiting", TenantID: 1, TicketID: "3", RoomName: "room-waiting", Status: "active", BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: staleAt}},
		{ID: "meeting-ended", TenantID: 1, TicketID: "4", RoomName: "room-ended", Status: "ended", BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&meetings).Error; err != nil {
		t.Fatalf("create meetings: %v", err)
	}
	participants := []models.MeetingParticipant{
		{ID: "participant-online", MeetingID: "meeting-online", JoinedAt: &now, BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now}},
		{ID: "participant-stale", MeetingID: "meeting-stale", JoinedAt: &staleAt, BaseModel: models.BaseModel{CreatedAt: staleAt, UpdatedAt: staleAt}},
		{ID: "participant-ended", MeetingID: "meeting-ended", JoinedAt: &now, BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&participants).Error; err != nil {
		t.Fatalf("create participants: %v", err)
	}

	snapshot, err := PlatformConsoleRepository.GetJitsiSnapshot(db, now, cutoff)
	if err != nil {
		t.Fatalf("get Jitsi snapshot: %v", err)
	}
	if snapshot.ActiveMeetings != 3 || snapshot.OnlineParticipants != 1 || snapshot.StaleParticipants != 1 || snapshot.StaleMeetings != 1 {
		t.Fatalf("heartbeat snapshot = %#v", snapshot)
	}
	if snapshot.LastHeartbeatAt == nil || snapshot.LastHeartbeatAt.Before(now) {
		t.Fatalf("last heartbeat = %v, want %v", snapshot.LastHeartbeatAt, now)
	}
}

func TestGetDatabaseAndAssetStorageSnapshots(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:platform_console_storage?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Asset{}); err != nil {
		t.Fatalf("migrate assets: %v", err)
	}
	assets := []models.Asset{
		{AssetID: "asset-1", Provider: "minio", StorageKey: "a/1", FileSize: 1024, Status: 2},
		{AssetID: "asset-2", Provider: "minio", StorageKey: "a/2", FileSize: 2048, Status: 2},
		{AssetID: "asset-3", Provider: "local", StorageKey: "a/3", FileSize: 4096, Status: 4},
	}
	if err := db.Create(&assets).Error; err != nil {
		t.Fatalf("create assets: %v", err)
	}
	database, err := PlatformConsoleRepository.GetDatabaseStorageSnapshot(db, 8)
	if err != nil {
		t.Fatalf("database storage snapshot: %v", err)
	}
	if database.Engine != "sqlite" || database.SizeBytes <= 0 || database.TableCount != 1 {
		t.Fatalf("database storage snapshot = %#v", database)
	}
	rows, err := PlatformConsoleRepository.GetAssetStorageRows(db)
	if err != nil {
		t.Fatalf("asset storage rows: %v", err)
	}
	if len(rows) != 1 || rows[0].Provider != "minio" || rows[0].ObjectCount != 2 || rows[0].SizeBytes != 3072 {
		t.Fatalf("asset storage rows = %#v", rows)
	}
}
