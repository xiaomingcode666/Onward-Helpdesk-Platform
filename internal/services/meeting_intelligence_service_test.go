package services

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/providers"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestIngestJigasiTranscriptEventUpsertsFinalSegment(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	migrateMeetingTranscriptTestSchema(t, db)
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	startedAt := time.UnixMilli(1_700_000_000_000)
	meeting := models.MeetingRoomJitsi{
		ID: "meeting-1", TenantID: 9, TicketID: "42", RoomName: "rhd-9-42-demo", Status: "active",
		StartedAt: &startedAt, BaseModel: models.BaseModel{CreatedAt: startedAt, UpdatedAt: startedAt},
	}
	if err := db.Create(&meeting).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	input := request.JigasiTranscriptEventRequest{
		Type: "transcription-result", RoomName: meeting.RoomName + "@muc.meet.jitsi/jigasi", Event: "SPEECH",
		Timestamp: startedAt.Add(1500 * time.Millisecond).UnixMilli(), MessageID: "message-7",
		Language: "en", IsInterim: false,
		Participant: request.JigasiTranscriptParticipant{ID: "xmpp-resource", IdentityUserID: "customer:18", Name: "客户张先生"},
		Transcript:  []request.JigasiTranscriptAlternative{{Text: "设备已经重新启动", Confidence: 0.92}},
	}
	for i := 0; i < 2; i++ {
		ingested, err := MeetingIntelligenceService.IngestJigasiTranscriptEvent(context.Background(), input)
		if err != nil || !ingested {
			t.Fatalf("ingest %d = %v, %v", i, ingested, err)
		}
		input.Timestamp += 10_000
		input.Participant.Name = "不应覆盖"
		input.Transcript[0].Text = "重复事件不应覆盖原始字幕"
	}
	var segments []models.MeetingTranscriptSegment
	if err := db.Find(&segments).Error; err != nil {
		t.Fatalf("list segments: %v", err)
	}
	if len(segments) != 1 {
		t.Fatalf("segments = %d, want 1", len(segments))
	}
	segment := segments[0]
	if segment.ParticipantID != "customer:18" || segment.SpeakerName != "客户张先生" {
		t.Fatalf("speaker = %#v", segment)
	}
	if segment.Text != "设备已经重新启动" || !segment.IsFinal || segment.StartedAtMS != 1500 {
		t.Fatalf("segment = %#v", segment)
	}
	if segment.Provider != "jitsi_caption" {
		t.Fatalf("provider = %q, want stable callback source", segment.Provider)
	}
	if segment.Language != "zh-CN" {
		t.Fatalf("language = %q, want Chinese content to override Jigasi's default", segment.Language)
	}
}

func TestJigasiTranscriptReconcilesEarlierClientArchive(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	migrateMeetingTranscriptTestSchema(t, db)
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	startedAt := time.UnixMilli(1_700_000_000_000)
	meeting := models.MeetingRoomJitsi{
		ID: "meeting-client-race", TenantID: 9, TicketID: "42", RoomName: "rhd-9-42-client-race", Status: "active",
		StartedAt: &startedAt, BaseModel: models.BaseModel{CreatedAt: startedAt, UpdatedAt: startedAt},
	}
	if err := db.Create(&meeting).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	if _, err := MeetingIntelligenceService.IngestClientTranscript(context.Background(), &meeting, MeetingTranscriptSpeaker{
		ParticipantID: "operator-7", Name: "Submitting operator", Language: "zh-CN",
	}, request.MeetingTranscriptIngestRequest{
		ProviderEventID: "message-authoritative", Text: "客户端占位内容", IsFinal: true,
		StartedAtMS: startedAt.Add(time.Second).UnixMilli(), EndedAtMS: startedAt.Add(time.Second).UnixMilli(),
	}); err != nil {
		t.Fatalf("archive client transcript: %v", err)
	}
	input := request.JigasiTranscriptEventRequest{
		Type: "transcription-result", RoomName: meeting.RoomName, Event: "SPEECH",
		Timestamp: startedAt.Add(1200 * time.Millisecond).UnixMilli(), MessageID: "message-authoritative",
		Language: "zh-CN", Participant: request.JigasiTranscriptParticipant{
			ID: "xmpp-speaker", IdentityUserID: "customer:18", Name: "客户张先生",
		},
		Transcript: []request.JigasiTranscriptAlternative{{Text: "设备已经恢复运行", Confidence: 0.94}},
	}
	if ingested, err := MeetingIntelligenceService.IngestJigasiTranscriptEvent(context.Background(), input); err != nil || !ingested {
		t.Fatalf("ingest authoritative transcript = %v, %v", ingested, err)
	}
	var segments []models.MeetingTranscriptSegment
	if err := db.Find(&segments).Error; err != nil {
		t.Fatalf("list transcripts: %v", err)
	}
	if len(segments) != 1 {
		t.Fatalf("segments = %d, want 1", len(segments))
	}
	segment := segments[0]
	if segment.IngestSource != meetingTranscriptSourceJigasi || segment.ParticipantID != "customer:18" || segment.SpeakerName != "客户张先生" || segment.Text != "设备已经恢复运行" {
		t.Fatalf("client transcript was not reconciled from Jigasi: %#v", segment)
	}
}

func TestJitsiRoomLookupNamesPreservesExactThenNormalizesJID(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{input: "  rhd-9-42-demo  ", want: []string{"rhd-9-42-demo"}},
		{input: "rhd-9-42-demo@muc.meet.jitsi", want: []string{"rhd-9-42-demo@muc.meet.jitsi", "rhd-9-42-demo"}},
		{input: "rhd-9-42-demo@muc.meet.jitsi/jigasi", want: []string{"rhd-9-42-demo@muc.meet.jitsi/jigasi", "rhd-9-42-demo"}},
		{input: "  ", want: nil},
	}
	for _, test := range tests {
		got := jitsiRoomLookupNames(test.input)
		if strings.Join(got, "|") != strings.Join(test.want, "|") {
			t.Fatalf("jitsiRoomLookupNames(%q) = %#v, want %#v", test.input, got, test.want)
		}
	}
}

func TestJitsiRoomCandidateMatchesNormalizedRoom(t *testing.T) {
	candidates := jitsiRoomLookupNames("rhd-9-42-demo@muc.meet.jitsi/jigasi")
	if !jitsiRoomCandidateMatches(candidates, "rhd-9-42-demo") {
		t.Fatal("normalized Jitsi room did not match")
	}
	if jitsiRoomCandidateMatches(candidates, "another-room") {
		t.Fatal("different Jitsi room matched")
	}
}

func TestIngestJigasiTranscriptEventIgnoresLifecycleEvents(t *testing.T) {
	ingested, err := MeetingIntelligenceService.IngestJigasiTranscriptEvent(context.Background(), request.JigasiTranscriptEventRequest{Event: "START"})
	if err != nil || ingested {
		t.Fatalf("lifecycle event = %v, %v", ingested, err)
	}
}

func TestIngestJigasiTranscriptEventIgnoresInterimEvents(t *testing.T) {
	ingested, err := MeetingIntelligenceService.IngestJigasiTranscriptEvent(context.Background(), request.JigasiTranscriptEventRequest{
		Event: "SPEECH", IsInterim: true,
	})
	if err != nil || ingested {
		t.Fatalf("interim event = %v, %v", ingested, err)
	}
}

func TestNormalizeProviderEventIDHashesOversizedValues(t *testing.T) {
	input := strings.Repeat("事件", 100)
	first := normalizeProviderEventID(input)
	second := normalizeProviderEventID(input)
	if first != second || !strings.HasPrefix(first, "sha256:") || len(first) > 128 {
		t.Fatalf("normalized provider event ID = %q", first)
	}
}

func TestNormalizeTranscriptLanguage(t *testing.T) {
	tests := []struct {
		language string
		text     string
		want     string
	}{
		{language: "en", text: "中文实时字幕验证开始，液压系统压力异常", want: "zh-CN"},
		{language: "zh_CN", text: "设备已经重新启动", want: "zh-CN"},
		{language: "zh-TW", text: "設備已經重新啟動", want: "zh-TW"},
		{language: "en", text: "Restart the RHD-42 controller", want: "en-US"},
		{language: "", text: "Restart the controller", want: "en-US"},
		{language: "", text: "", want: "zh-CN"},
	}
	for _, test := range tests {
		if got := normalizeTranscriptLanguage(test.language, test.text); got != test.want {
			t.Fatalf("normalizeTranscriptLanguage(%q, %q) = %q, want %q", test.language, test.text, got, test.want)
		}
	}
}

func TestNormalizeMeetingTranscriptTiming(t *testing.T) {
	startedAt := time.UnixMilli(1_700_000_000_000)
	meeting := &models.MeetingRoomJitsi{
		StartedAt: &startedAt,
		BaseModel: models.BaseModel{CreatedAt: startedAt},
	}
	start, end, err := normalizeMeetingTranscriptTiming(
		meeting,
		startedAt.Add(1500*time.Millisecond).UnixMilli(),
		startedAt.Add(1800*time.Millisecond).UnixMilli(),
	)
	if err != nil || start != 1500 || end != 1800 {
		t.Fatalf("absolute timing = %d..%d, %v", start, end, err)
	}
	start, end, err = normalizeMeetingTranscriptTiming(meeting, 2500, 3000)
	if err != nil || start != 2500 || end != 3000 {
		t.Fatalf("relative timing = %d..%d, %v", start, end, err)
	}
	if _, _, err := normalizeMeetingTranscriptTiming(meeting, 3000, 2500); err == nil {
		t.Fatal("reversed transcript timing was accepted")
	}
}

func TestSaveTranscriptEventConcurrentDuplicateCreatesOnce(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	migrateMeetingTranscriptTestSchema(t, db)
	sqls.SetDB(db)
	t.Cleanup(func() { _ = sqlDB.Close() })

	startedAt := time.UnixMilli(1_700_000_000_000)
	meeting := models.MeetingRoomJitsi{
		ID: "meeting-concurrent", TenantID: 9, TicketID: "42", RoomName: "meeting-concurrent", Status: "active",
		StartedAt: &startedAt, BaseModel: models.BaseModel{CreatedAt: startedAt, UpdatedAt: startedAt},
	}
	if err := db.Create(&meeting).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}

	const workers = 16
	start := make(chan struct{})
	errorsByWorker := make(chan error, workers)
	var wait sync.WaitGroup
	for i := 0; i < workers; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := MeetingIntelligenceService.SaveTranscriptEvent(context.Background(), &meeting, MeetingTranscriptSpeaker{
				ParticipantID: "1", Name: "Engineer", Language: "zh-CN",
			}, "jitsi_caption", &providers.SpeechTranscriptionEvent{
				ProviderEventID: "same-event", Text: "并发字幕", IsFinal: true,
				StartedAtMS: startedAt.Add(time.Second).UnixMilli(), EndedAtMS: startedAt.Add(1200 * time.Millisecond).UnixMilli(),
				Confidence: 0.9,
			})
			errorsByWorker <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errorsByWorker)
	for err := range errorsByWorker {
		if err != nil {
			t.Fatalf("concurrent save: %v", err)
		}
	}
	var count int64
	if err := db.Model(&models.MeetingTranscriptSegment{}).Count(&count).Error; err != nil {
		t.Fatalf("count segments: %v", err)
	}
	if count != 1 {
		t.Fatalf("segments = %d, want 1", count)
	}
}

func TestBackfillEndedMeetingClosureArchivesCreatesReadableSystemTranscript(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	migrateMeetingTranscriptTestSchema(t, db)
	if err := db.AutoMigrate(&models.MeetingParticipant{}); err != nil {
		t.Fatalf("migrate participants: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() { _ = sqlDB.Close() })

	startedAt := time.Now().Add(-20 * time.Minute)
	endedAt := startedAt.Add(12 * time.Minute)
	emptyMeeting := models.MeetingRoomJitsi{
		ID: "meeting-empty-ended", TenantID: 9, TicketID: "42", RoomName: "meeting-empty-ended-room", Status: "ended",
		StartedAt: &startedAt, EndedAt: &endedAt, BaseModel: models.BaseModel{CreatedAt: startedAt, UpdatedAt: endedAt},
	}
	withTranscript := models.MeetingRoomJitsi{
		ID: "meeting-ended-with-transcript", TenantID: 9, TicketID: "43", RoomName: "meeting-ended-with-transcript-room", Status: "ended",
		StartedAt: &startedAt, EndedAt: &endedAt, BaseModel: models.BaseModel{CreatedAt: startedAt, UpdatedAt: endedAt},
	}
	activeMeeting := models.MeetingRoomJitsi{
		ID: "meeting-active-no-transcript", TenantID: 9, TicketID: "44", RoomName: "meeting-active-no-transcript-room", Status: "active",
		StartedAt: &startedAt, BaseModel: models.BaseModel{CreatedAt: startedAt, UpdatedAt: startedAt},
	}
	if err := db.Create(&[]models.MeetingRoomJitsi{emptyMeeting, withTranscript, activeMeeting}).Error; err != nil {
		t.Fatalf("create meetings: %v", err)
	}
	if err := db.Create(&models.MeetingParticipant{
		ID: "meeting-empty-participant", MeetingID: emptyMeeting.ID, UserID: "customer-1", UserType: "customer",
		JoinedAt: &startedAt, LeftAt: &endedAt, BaseModel: models.BaseModel{CreatedAt: startedAt, UpdatedAt: endedAt},
	}).Error; err != nil {
		t.Fatalf("create participant: %v", err)
	}
	if err := db.Create(&models.MeetingTranscriptSegment{
		ID: "existing-transcript", TenantID: withTranscript.TenantID, MeetingID: withTranscript.ID,
		ParticipantID: "customer-1", Provider: "jitsi_caption", ProviderEventID: "event-1", IngestSource: "jigasi",
		Language: "zh-CN", Text: "已有真实字幕", IsFinal: true, RawJSON: "{}",
		BaseModel: models.BaseModel{CreatedAt: startedAt, UpdatedAt: startedAt},
	}).Error; err != nil {
		t.Fatalf("create existing transcript: %v", err)
	}

	backfilled, err := MeetingIntelligenceService.BackfillEndedMeetingClosureArchivesDB(db, 100)
	if err != nil {
		t.Fatalf("backfill closure transcript: %v", err)
	}
	if backfilled != 1 {
		t.Fatalf("backfilled = %d, want 1", backfilled)
	}
	backfilled, err = MeetingIntelligenceService.BackfillEndedMeetingClosureArchivesDB(db, 100)
	if err != nil || backfilled != 0 {
		t.Fatalf("second backfill = %d, %v; want 0", backfilled, err)
	}
	var segments []models.MeetingTranscriptSegment
	if err := db.Order("meeting_id ASC, provider ASC").Find(&segments).Error; err != nil {
		t.Fatalf("list transcripts: %v", err)
	}
	if len(segments) != 2 {
		t.Fatalf("segments = %d, want existing plus one closure transcript: %+v", len(segments), segments)
	}
	var closure models.MeetingTranscriptSegment
	if err := db.Where("meeting_id = ? AND provider = ?", emptyMeeting.ID, "system").First(&closure).Error; err != nil {
		t.Fatalf("load closure transcript: %v", err)
	}
	if closure.IngestSource != meetingTranscriptSourceSystem || !strings.Contains(closure.Text, "未收到实时语音转写片段") || closure.EndedAtMS <= 0 {
		t.Fatalf("closure transcript = %#v", closure)
	}
	var activeCount int64
	if err := db.Model(&models.MeetingTranscriptSegment{}).Where("meeting_id = ?", activeMeeting.ID).Count(&activeCount).Error; err != nil || activeCount != 0 {
		t.Fatalf("active meeting transcript count = %d, %v; want 0", activeCount, err)
	}
}

func migrateMeetingTranscriptTestSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(&models.MeetingRoomJitsi{}, &models.MeetingTranscriptSegment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX uk_meeting_transcript_event_v2 ON meeting_transcript_segments (meeting_id, provider, provider_event_id)").Error; err != nil {
		t.Fatalf("create transcript event unique index: %v", err)
	}
}

func TestValidateMeetingFrameAssetRequiresCurrentMeetingFrame(t *testing.T) {
	meeting := &models.MeetingRoomJitsi{ID: "meeting-1", TenantID: 9}
	valid := &models.Asset{
		TenantID: 9, Status: enums.AssetStatusSuccess,
		StorageKey: "development/meeting-frames/meeting-1/2026/07/30/frame.png",
	}
	if err := validateMeetingFrameAsset(meeting, valid); err != nil {
		t.Fatalf("valid frame rejected: %v", err)
	}

	invalid := []*models.Asset{
		nil,
		{TenantID: 10, Status: enums.AssetStatusSuccess, StorageKey: valid.StorageKey},
		{TenantID: 9, Status: enums.AssetStatusPending, StorageKey: valid.StorageKey},
		{TenantID: 9, Status: enums.AssetStatusSuccess, StorageKey: "development/meeting-frames/other-meeting/frame.png"},
	}
	for index, asset := range invalid {
		if err := validateMeetingFrameAsset(meeting, asset); err == nil {
			t.Fatalf("invalid frame %d was accepted", index)
		}
	}
}
