package repositories

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMeetingArchiveStatsAreGroupedByMeeting(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.MeetingTranscriptSegment{}, &models.MeetingARAnnotation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now().UTC()
	transcripts := []models.MeetingTranscriptSegment{
		archiveStatsTestSegment("t-1", "meeting-a", now),
		archiveStatsTestSegment("t-2", "meeting-a", now),
		archiveStatsTestSegment("t-3", "meeting-b", now),
		archiveStatsSystemClosureSegment("t-system-a", "meeting-a", now),
		archiveStatsSystemClosureSegment("t-system-c", "meeting-c", now),
	}
	if err := db.Create(&transcripts).Error; err != nil {
		t.Fatalf("create transcripts: %v", err)
	}
	annotations := []models.MeetingARAnnotation{
		{ID: "a-1", TenantID: 1, MeetingID: "meeting-a", TicketID: "1", Label: "阀组", CreatedAt: now, UpdatedAt: now},
		{ID: "a-2", TenantID: 1, MeetingID: "meeting-a", TicketID: "1", Label: "端子", CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&annotations).Error; err != nil {
		t.Fatalf("create annotations: %v", err)
	}

	stats := MeetingIntelligenceRepository.ArchiveStatsByMeetingIDs(db, []string{"meeting-a", "meeting-b"})
	if stats["meeting-a"].TranscriptCount != 2 || stats["meeting-a"].AnnotationCount != 2 {
		t.Fatalf("meeting-a stats = %#v", stats["meeting-a"])
	}
	if stats["meeting-b"].TranscriptCount != 1 || stats["meeting-b"].AnnotationCount != 0 {
		t.Fatalf("meeting-b stats = %#v", stats["meeting-b"])
	}
	if stats["meeting-c"].TranscriptCount != 0 {
		t.Fatalf("meeting-c system closure counted as transcript: %#v", stats["meeting-c"])
	}
	count, err := MeetingIntelligenceRepository.CountTranscripts(db, 1, "meeting-a")
	if err != nil {
		t.Fatalf("count real transcripts: %v", err)
	}
	if count != 2 {
		t.Fatalf("real transcript count = %d, want 2", count)
	}
}

func archiveStatsTestSegment(id, meetingID string, now time.Time) models.MeetingTranscriptSegment {
	return models.MeetingTranscriptSegment{
		ID: id, TenantID: 1, MeetingID: meetingID, ParticipantID: "speaker",
		Provider: "test", ProviderEventID: "event-" + id, IngestSource: "test",
		Language: "zh-CN", Text: id, IsFinal: true, RawJSON: "{}",
		BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
}

func archiveStatsSystemClosureSegment(id, meetingID string, now time.Time) models.MeetingTranscriptSegment {
	item := archiveStatsTestSegment(id, meetingID, now)
	item.ParticipantID = "system"
	item.SpeakerName = "系统记录"
	item.Provider = "system"
	item.ProviderEventID = "meeting-ended:" + meetingID
	item.IngestSource = "system"
	item.Text = "会议已结束，系统已归档本次视频会话记录。未收到实时语音转写片段。"
	return item
}

func TestCreateTranscriptIfAbsentFallsBackWithoutUniqueIndex(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.MeetingTranscriptSegment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now().UTC()
	item := archiveStatsTestSegment("missing-index", "meeting-no-unique", now)

	created, err := MeetingIntelligenceRepository.CreateTranscriptIfAbsent(db, &item)
	if err != nil || !created {
		t.Fatalf("first insert without unique index = created %v err %v", created, err)
	}
	duplicate := archiveStatsTestSegment("missing-index-duplicate", "meeting-no-unique", now)
	duplicate.Provider = item.Provider
	duplicate.ProviderEventID = item.ProviderEventID
	created, err = MeetingIntelligenceRepository.CreateTranscriptIfAbsent(db, &duplicate)
	if err != nil || created {
		t.Fatalf("duplicate insert without unique index = created %v err %v", created, err)
	}
	var count int64
	if err := db.Model(&models.MeetingTranscriptSegment{}).Where(
		"meeting_id = ? AND provider = ? AND provider_event_id = ?",
		item.MeetingID, item.Provider, item.ProviderEventID,
	).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("transcript duplicate count = %d err %v", count, err)
	}
}

func TestMeetingTranscriptTranslationQueueClaimsOnlyDueWork(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.MeetingTranscriptSegment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now().Truncate(time.Millisecond)
	past := now.Add(-time.Minute)
	future := now.Add(time.Minute)
	items := []models.MeetingTranscriptSegment{
		translationQueueTestSegment("pending-due", "pending", nil, nil, now),
		translationQueueTestSegment("pending-future", "pending", &future, nil, now),
		translationQueueTestSegment("failed-due", "failed", &past, nil, now),
		translationQueueTestSegment("failed-terminal", "failed", nil, nil, now),
		translationQueueTestSegment("processing-expired", "processing", nil, &past, now),
		translationQueueTestSegment("processing-no-lease", "processing", nil, nil, now),
		translationQueueTestSegment("processing-active", "processing", nil, &future, now),
	}
	if err := db.Create(&items).Error; err != nil {
		t.Fatalf("create queue rows: %v", err)
	}

	due, err := MeetingIntelligenceRepository.ListDueTranscriptTranslations(db, now, 20)
	if err != nil {
		t.Fatalf("list due translations: %v", err)
	}
	got := make(map[string]bool, len(due))
	for _, item := range due {
		got[item.ID] = true
	}
	for _, id := range []string{"pending-due", "failed-due", "processing-expired"} {
		if !got[id] {
			t.Fatalf("due translation %q was not selected: %#v", id, got)
		}
	}
	if len(got) != 3 {
		t.Fatalf("unexpected due translations: %#v", got)
	}

	leaseUntil := now.Add(30 * time.Second)
	claimed, err := MeetingIntelligenceRepository.ClaimTranscriptTranslation(db, "pending-due", now, leaseUntil)
	if err != nil || !claimed {
		t.Fatalf("first claim = %v, %v", claimed, err)
	}
	claimed, err = MeetingIntelligenceRepository.ClaimTranscriptTranslation(db, "pending-due", now, leaseUntil)
	if err != nil || claimed {
		t.Fatalf("duplicate claim = %v, %v", claimed, err)
	}
	item := translationQueueTestSegment("pending-due", "completed", nil, nil, now)
	item.TranslatedText = "translated"
	updated, err := MeetingIntelligenceRepository.UpdateTranscriptTranslation(db, &item, leaseUntil.Add(time.Second))
	if err != nil || updated {
		t.Fatalf("stale lease update = %v, %v", updated, err)
	}
	updated, err = MeetingIntelligenceRepository.UpdateTranscriptTranslation(db, &item, leaseUntil)
	if err != nil || !updated {
		t.Fatalf("current lease update = %v, %v", updated, err)
	}
}

func TestMeetingTranscriptPageUsesStableTimelineCursor(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.MeetingTranscriptSegment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	base := time.Now().UTC().Truncate(time.Millisecond)
	items := []models.MeetingTranscriptSegment{
		transcriptPageTestSegment("a", 100, base),
		transcriptPageTestSegment("b", 200, base.Add(time.Second)),
		transcriptPageTestSegment("c", 200, base.Add(time.Second)),
		transcriptPageTestSegment("d", 300, base.Add(2*time.Second)),
		transcriptPageTestSegment("e", 400, base.Add(3*time.Second)),
	}
	if err := db.Create(&items).Error; err != nil {
		t.Fatalf("create transcript rows: %v", err)
	}

	first, hasMore, err := MeetingIntelligenceRepository.ListTranscriptPage(db, 7, "meeting-page", 0, time.Time{}, "", false, 2)
	if err != nil || !hasMore {
		t.Fatalf("first page = %#v, hasMore=%v, err=%v", first, hasMore, err)
	}
	assertTranscriptPageIDs(t, first, "d", "e")
	second, hasMore, err := MeetingIntelligenceRepository.ListTranscriptPage(
		db, 7, "meeting-page", first[0].StartedAtMS, first[0].CreatedAt, first[0].ID, true, 2,
	)
	if err != nil || !hasMore {
		t.Fatalf("second page = %#v, hasMore=%v, err=%v", second, hasMore, err)
	}
	assertTranscriptPageIDs(t, second, "b", "c")
	third, hasMore, err := MeetingIntelligenceRepository.ListTranscriptPage(
		db, 7, "meeting-page", second[0].StartedAtMS, second[0].CreatedAt, second[0].ID, true, 2,
	)
	if err != nil || hasMore {
		t.Fatalf("third page = %#v, hasMore=%v, err=%v", third, hasMore, err)
	}
	assertTranscriptPageIDs(t, third, "a")
}

func transcriptPageTestSegment(id string, startedAtMS int64, createdAt time.Time) models.MeetingTranscriptSegment {
	return models.MeetingTranscriptSegment{
		ID: id, TenantID: 7, MeetingID: "meeting-page", ParticipantID: "speaker",
		Provider: "jitsi_caption", ProviderEventID: "event-" + id, IngestSource: "jigasi",
		Language: "zh-CN", Text: id, IsFinal: true, StartedAtMS: startedAtMS, RawJSON: "{}",
		BaseModel: models.BaseModel{CreatedAt: createdAt, UpdatedAt: createdAt},
	}
}

func assertTranscriptPageIDs(t *testing.T, items []models.MeetingTranscriptSegment, want ...string) {
	t.Helper()
	if len(items) != len(want) {
		t.Fatalf("page length = %d, want %d: %#v", len(items), len(want), items)
	}
	for index := range want {
		if items[index].ID != want[index] {
			t.Fatalf("page[%d] = %q, want %q", index, items[index].ID, want[index])
		}
	}
}

func translationQueueTestSegment(id, status string, nextRetryAt, leaseUntil *time.Time, now time.Time) models.MeetingTranscriptSegment {
	return models.MeetingTranscriptSegment{
		ID: id, TenantID: 1, MeetingID: "meeting-queue", ParticipantID: "speaker",
		Provider: "jitsi_caption", ProviderEventID: id, IngestSource: "jigasi",
		Language: "zh-CN", Text: id, IsFinal: true, RawJSON: "{}",
		TranslationStatus: status, TranslationNextRetryAt: nextRetryAt, TranslationLeaseUntil: leaseUntil,
		BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
}
