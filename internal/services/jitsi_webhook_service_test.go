package services

import (
	"encoding/json"
	"testing"
	"time"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestJitsiWebhookServiceDeduplicatesParticipantJoinAndResolvesRoomName(t *testing.T) {
	db := setupJitsiWebhookTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	meeting := models.MeetingRoomJitsi{
		ID: "meeting-inbox-1", TenantID: 1, TicketID: "0", RoomName: "room-inbox-1", Status: "active",
		BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&meeting).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	input := JitsiWebhookProcessInput{
		EventID: "jitsi-event-1", Action: "participant.joined", PayloadHash: "hash-1",
		RoomName: meeting.RoomName, ParticipantID: "external-1", ParticipantName: "供应商工程师", ReceivedAt: now,
	}
	duplicate, err := JitsiWebhookService.Process(nil, input)
	if err != nil || duplicate {
		t.Fatalf("first process = duplicate:%v err:%v", duplicate, err)
	}
	duplicate, err = JitsiWebhookService.Process(nil, input)
	if err != nil || !duplicate {
		t.Fatalf("duplicate process = duplicate:%v err:%v", duplicate, err)
	}

	var participantCount int64
	if err := db.Model(&models.MeetingParticipant{}).
		Where("meeting_id = ? AND user_id = ? AND user_type = ?", meeting.ID, input.ParticipantID, "external").
		Count(&participantCount).Error; err != nil {
		t.Fatalf("count participant: %v", err)
	}
	if participantCount != 1 {
		t.Fatalf("participant count = %d, want 1", participantCount)
	}
	var inbox models.WebhookEventInbox
	if err := db.Where("provider = ? AND event_id = ?", "jitsi", input.EventID).First(&inbox).Error; err != nil {
		t.Fatalf("load inbox: %v", err)
	}
	if inbox.Status != "processed" || inbox.AttemptCount != 1 || inbox.ProcessedAt == nil {
		t.Fatalf("unexpected inbox state: %+v", inbox)
	}

	collision := input
	collision.PayloadHash = "different-hash"
	if _, err := JitsiWebhookService.Process(nil, collision); err == nil {
		t.Fatal("event id reuse with a different payload must be rejected")
	}
}

func TestJitsiWebhookServiceRetriesFailedEvent(t *testing.T) {
	db := setupJitsiWebhookTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	input := JitsiWebhookProcessInput{
		EventID: "jitsi-retry-1", Action: "participant.joined", PayloadHash: "retry-hash",
		RoomName: "room-created-after-callback", ParticipantID: "external-retry", ReceivedAt: now,
	}
	if _, err := JitsiWebhookService.Process(nil, input); err == nil {
		t.Fatal("missing meeting should leave the event retryable")
	}
	meeting := models.MeetingRoomJitsi{
		ID: "meeting-retry-1", TenantID: 1, TicketID: "0", RoomName: input.RoomName, Status: "active",
		BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&meeting).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	if err := db.Model(&models.WebhookEventInbox{}).
		Where("provider = ? AND event_id = ?", "jitsi", input.EventID).
		Update("locked_until", now.Add(-time.Second)).Error; err != nil {
		t.Fatalf("make failed callback due: %v", err)
	}
	recovered, err := JitsiWebhookService.RecoverPending(nil, 10)
	if err != nil || recovered != 1 {
		t.Fatalf("recover pending = recovered:%d err:%v", recovered, err)
	}
	var inbox models.WebhookEventInbox
	if err := db.Where("provider = ? AND event_id = ?", "jitsi", input.EventID).First(&inbox).Error; err != nil {
		t.Fatalf("load retry inbox: %v", err)
	}
	if inbox.Status != "processed" || inbox.AttemptCount != 2 || inbox.LastError != "" {
		t.Fatalf("unexpected retry inbox: %+v", inbox)
	}
}

func TestJitsiWebhookServiceRecoversAbandonedProcessingEvent(t *testing.T) {
	db := setupJitsiWebhookTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	meeting := models.MeetingRoomJitsi{
		ID: "meeting-abandoned-1", TenantID: 1, TicketID: "0", RoomName: "room-abandoned-1", Status: "active",
		BaseModel: models.BaseModel{CreatedAt: now.Add(-time.Minute), UpdatedAt: now},
	}
	if err := db.Create(&meeting).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	input := JitsiWebhookProcessInput{
		EventID: "jitsi-abandoned-1", Action: "participant.joined", PayloadHash: "abandoned-hash",
		MeetingID: meeting.ID, ParticipantID: "supplier-abandoned", ParticipantName: "恢复的供应商",
		OccurredAt: now.Add(-30 * time.Second), ReceivedAt: now.Add(-20 * time.Second),
	}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal replay payload: %v", err)
	}
	lockedUntil := now.Add(-time.Second)
	item := models.WebhookEventInbox{
		Provider: "jitsi", EventID: input.EventID, EventType: input.Action, PayloadHash: input.PayloadHash,
		PayloadJSON: string(payload), Status: "processing", AttemptCount: 1, LockedUntil: &lockedUntil,
		OccurredAt: &input.OccurredAt, ReceivedAt: input.ReceivedAt, CreatedAt: input.ReceivedAt, UpdatedAt: input.ReceivedAt,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create abandoned inbox item: %v", err)
	}

	recovered, err := JitsiWebhookService.RecoverPending(nil, 10)
	if err != nil || recovered != 1 {
		t.Fatalf("recover abandoned event = recovered:%d err:%v", recovered, err)
	}
	var participant models.MeetingParticipant
	if err := db.Where("meeting_id = ? AND user_id = ?", meeting.ID, input.ParticipantID).First(&participant).Error; err != nil {
		t.Fatalf("load recovered participant: %v", err)
	}
	if participant.JoinedAt == nil || !participant.JoinedAt.Equal(input.OccurredAt) || participant.LeftAt != nil {
		t.Fatalf("unexpected recovered participant: %+v", participant)
	}
	if err := db.First(&item, item.ID).Error; err != nil {
		t.Fatalf("reload recovered inbox: %v", err)
	}
	if item.Status != "processed" || item.AttemptCount != 2 || item.ProcessedAt == nil {
		t.Fatalf("unexpected recovered inbox: %+v", item)
	}
}

func TestJitsiWebhookServiceMergesOutOfOrderParticipantEvents(t *testing.T) {
	db := setupJitsiWebhookTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	meeting := models.MeetingRoomJitsi{
		ID: "meeting-out-of-order-1", TenantID: 1, TicketID: "0", RoomName: "room-out-of-order-1", Status: "active",
		BaseModel: models.BaseModel{CreatedAt: now.Add(-10 * time.Minute), UpdatedAt: now},
	}
	if err := db.Create(&meeting).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	joinedAt := now.Add(-5 * time.Minute)
	leftAt := now.Add(-4 * time.Minute)
	participantID := "supplier-out-of-order"
	leave := JitsiWebhookProcessInput{
		EventID: "jitsi-out-of-order-left", Action: "participant.left", PayloadHash: "left-hash",
		MeetingID: meeting.ID, ParticipantID: participantID, OccurredAt: leftAt, ReceivedAt: now,
	}
	if duplicate, err := JitsiWebhookService.Process(nil, leave); err != nil || duplicate {
		t.Fatalf("process early leave = duplicate:%v err:%v", duplicate, err)
	}
	join := JitsiWebhookProcessInput{
		EventID: "jitsi-out-of-order-join", Action: "participant.joined", PayloadHash: "join-hash",
		MeetingID: meeting.ID, ParticipantID: participantID, ParticipantName: "乱序供应商",
		OccurredAt: joinedAt, ReceivedAt: now.Add(time.Second),
	}
	if duplicate, err := JitsiWebhookService.Process(nil, join); err != nil || duplicate {
		t.Fatalf("process delayed join = duplicate:%v err:%v", duplicate, err)
	}

	var participant models.MeetingParticipant
	if err := db.Where("meeting_id = ? AND user_id = ?", meeting.ID, participantID).First(&participant).Error; err != nil {
		t.Fatalf("load merged participant: %v", err)
	}
	if participant.JoinedAt == nil || participant.LeftAt == nil || !participant.JoinedAt.Equal(joinedAt) || !participant.LeftAt.Equal(leftAt) || participant.Duration != 60 {
		t.Fatalf("out-of-order events were not merged: %+v", participant)
	}

	rejoinedAt := now.Add(-2 * time.Minute)
	rejoin := join
	rejoin.EventID = "jitsi-out-of-order-rejoin"
	rejoin.PayloadHash = "rejoin-hash"
	rejoin.OccurredAt = rejoinedAt
	rejoin.ReceivedAt = now.Add(2 * time.Second)
	if duplicate, err := JitsiWebhookService.Process(nil, rejoin); err != nil || duplicate {
		t.Fatalf("process rejoin = duplicate:%v err:%v", duplicate, err)
	}
	var rejoinedMeeting models.MeetingRoomJitsi
	if err := db.First(&rejoinedMeeting, "id = ?", meeting.ID).Error; err != nil {
		t.Fatalf("load rejoined meeting: %v", err)
	}
	if rejoinedMeeting.StartedAt == nil || !rejoinedMeeting.StartedAt.Equal(joinedAt) {
		t.Fatalf("rejoin changed first meeting join from %v to %v", joinedAt, rejoinedMeeting.StartedAt)
	}
	lateOldLeave := leave
	lateOldLeave.EventID = "jitsi-out-of-order-old-left"
	lateOldLeave.PayloadHash = "old-left-hash"
	lateOldLeave.ReceivedAt = now.Add(3 * time.Second)
	if duplicate, err := JitsiWebhookService.Process(nil, lateOldLeave); err != nil || duplicate {
		t.Fatalf("process late old leave = duplicate:%v err:%v", duplicate, err)
	}
	participant = models.MeetingParticipant{}
	if err := db.Where("meeting_id = ? AND user_id = ?", meeting.ID, participantID).First(&participant).Error; err != nil {
		t.Fatalf("reload active participant: %v", err)
	}
	if participant.LeftAt != nil || participant.JoinedAt == nil || !participant.JoinedAt.Equal(rejoinedAt) {
		t.Fatalf("old leave closed a newer join: %+v", participant)
	}

	finalLeftAt := now.Add(-time.Minute)
	finalLeave := leave
	finalLeave.EventID = "jitsi-out-of-order-final-left"
	finalLeave.PayloadHash = "final-left-hash"
	finalLeave.OccurredAt = finalLeftAt
	finalLeave.ReceivedAt = now.Add(4 * time.Second)
	if duplicate, err := JitsiWebhookService.Process(nil, finalLeave); err != nil || duplicate {
		t.Fatalf("process final leave = duplicate:%v err:%v", duplicate, err)
	}
	participant = models.MeetingParticipant{}
	if err := db.Where("meeting_id = ? AND user_id = ?", meeting.ID, participantID).First(&participant).Error; err != nil {
		t.Fatalf("reload final participant: %v", err)
	}
	if participant.LeftAt == nil || !participant.LeftAt.Equal(finalLeftAt) || participant.Duration != 60 {
		t.Fatalf("final leave was not applied: %+v", participant)
	}
}

func TestJitsiWebhookServiceReusesAuthenticatedParticipantIdentity(t *testing.T) {
	db := setupJitsiWebhookTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	meeting := models.MeetingRoomJitsi{
		ID: "meeting-authenticated-1", TenantID: 1, TicketID: "0", RoomName: "room-authenticated-1", Status: "waiting",
		BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&meeting).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	joinedEarlier := now.Add(-time.Minute)
	participant := models.MeetingParticipant{
		ID: "authenticated-participant", MeetingID: meeting.ID, UserID: "25", UserType: "enterprise",
		ParticipantName: "维修工程师", Role: "moderator", JoinedAt: &joinedEarlier,
		BaseModel: models.BaseModel{CreatedAt: joinedEarlier, UpdatedAt: joinedEarlier},
	}
	if err := db.Create(&participant).Error; err != nil {
		t.Fatalf("create authenticated participant: %v", err)
	}
	input := JitsiWebhookProcessInput{
		EventID: "jitsi-authenticated-join", Action: "participant.joined", PayloadHash: "authenticated-hash",
		MeetingID: meeting.ID, ParticipantID: participant.UserID, ParticipantName: "维修工程师（会议）", ReceivedAt: now,
	}
	if duplicate, err := JitsiWebhookService.Process(nil, input); err != nil || duplicate {
		t.Fatalf("process authenticated join = duplicate:%v err:%v", duplicate, err)
	}
	var participants []models.MeetingParticipant
	if err := db.Where("meeting_id = ? AND user_id = ?", meeting.ID, participant.UserID).Find(&participants).Error; err != nil {
		t.Fatalf("load participants: %v", err)
	}
	if len(participants) != 1 || participants[0].UserType != "enterprise" || participants[0].Role != "moderator" {
		t.Fatalf("webhook duplicated or downgraded authenticated participant: %+v", participants)
	}
	if participants[0].ParticipantName != input.ParticipantName || participants[0].JoinedAt == nil || participants[0].JoinedAt.Before(now) {
		t.Fatalf("authenticated participant was not refreshed: %+v", participants[0])
	}
	var refreshedMeeting models.MeetingRoomJitsi
	if err := db.First(&refreshedMeeting, "id = ?", meeting.ID).Error; err != nil {
		t.Fatalf("load refreshed meeting: %v", err)
	}
	if refreshedMeeting.StartedAt == nil || !refreshedMeeting.StartedAt.Equal(now) {
		t.Fatalf("meeting start = %v, want provider-confirmed first join %v", refreshedMeeting.StartedAt, now)
	}
	if refreshedMeeting.Status != "active" {
		t.Fatalf("meeting status = %q, want active after provider-confirmed join", refreshedMeeting.Status)
	}
}

func TestJitsiWebhookServiceParticipantLeaveIsIdempotent(t *testing.T) {
	db := setupJitsiWebhookTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	meeting := models.MeetingRoomJitsi{
		ID: "meeting-left-1", TenantID: 1, TicketID: "0", RoomName: "room-left-1", Status: "active",
		BaseModel: models.BaseModel{CreatedAt: now.Add(-3 * time.Minute), UpdatedAt: now},
	}
	if err := db.Create(&meeting).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	joinedAt := now.Add(-2 * time.Minute)
	participant := models.MeetingParticipant{
		ID: "leaving-participant", MeetingID: meeting.ID, UserID: "supplier-9", UserType: "partner",
		ParticipantName: "供应商", Role: "participant", JoinedAt: &joinedAt,
		BaseModel: models.BaseModel{CreatedAt: joinedAt, UpdatedAt: joinedAt},
	}
	if err := db.Create(&participant).Error; err != nil {
		t.Fatalf("create participant: %v", err)
	}
	input := JitsiWebhookProcessInput{
		EventID: "jitsi-left-1", Action: "participant.left", PayloadHash: "left-hash",
		RoomName: meeting.RoomName, ParticipantID: participant.UserID, ReceivedAt: now,
	}
	if duplicate, err := JitsiWebhookService.Process(nil, input); err != nil || duplicate {
		t.Fatalf("process participant leave = duplicate:%v err:%v", duplicate, err)
	}
	var leftOnce models.MeetingParticipant
	if err := db.First(&leftOnce, "id = ?", participant.ID).Error; err != nil {
		t.Fatalf("reload participant: %v", err)
	}
	if leftOnce.LeftAt == nil || leftOnce.Duration < 119 {
		t.Fatalf("participant leave was not recorded: %+v", leftOnce)
	}
	leftAt := *leftOnce.LeftAt
	duration := leftOnce.Duration
	if duplicate, err := JitsiWebhookService.Process(nil, input); err != nil || !duplicate {
		t.Fatalf("duplicate participant leave = duplicate:%v err:%v", duplicate, err)
	}
	var leftTwice models.MeetingParticipant
	if err := db.First(&leftTwice, "id = ?", participant.ID).Error; err != nil {
		t.Fatalf("reload duplicate leave: %v", err)
	}
	if leftTwice.LeftAt == nil || !leftTwice.LeftAt.Equal(leftAt) || leftTwice.Duration != duration {
		t.Fatalf("duplicate leave changed participant metrics: before=%+v after=%+v", leftOnce, leftTwice)
	}
}

func setupJitsiWebhookTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.WebhookEventInbox{},
		&models.MeetingRoomJitsi{},
		&models.MeetingParticipant{},
		&models.TicketProgress{},
		&models.DomainEvent{},
		&models.OutboxRecord{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}
