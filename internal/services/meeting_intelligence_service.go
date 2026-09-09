package services

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime/multipart"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dbtime"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var MeetingIntelligenceService = &meetingIntelligenceService{}

type meetingIntelligenceService struct{}

type meetingTranscriptCursor struct {
	StartedAtMS int64     `json:"startedAtMs"`
	CreatedAt   time.Time `json:"createdAt"`
	ID          string    `json:"id"`
}

func (s *meetingIntelligenceService) ListTranscriptsForOperator(meetingID string, operator *dto.AuthPrincipal) ([]models.MeetingTranscriptSegment, error) {
	meeting, _, err := MeetingService.requireMeetingTicketAccess(meetingID, operator, false)
	if err != nil {
		return nil, err
	}
	return repositories.MeetingIntelligenceRepository.ListTranscripts(sqls.DB(), meeting.TenantID, meeting.ID, 100)
}

func (s *meetingIntelligenceService) ListTranscriptPageForOperator(meetingID, cursor string, limit int, operator *dto.AuthPrincipal) ([]models.MeetingTranscriptSegment, string, bool, error) {
	meeting, _, err := MeetingService.requireMeetingTicketAccess(meetingID, operator, false)
	if err != nil {
		return nil, "", false, err
	}
	return s.listTranscriptPage(meeting.TenantID, meeting.ID, cursor, limit)
}

func (s *meetingIntelligenceService) listTranscriptPage(tenantID int64, meetingID, cursor string, limit int) ([]models.MeetingTranscriptSegment, string, bool, error) {
	decoded, hasCursor, err := decodeMeetingTranscriptCursor(cursor)
	if err != nil {
		return nil, "", false, errorsx.InvalidParam("invalid transcript cursor")
	}
	items, hasMore, err := repositories.MeetingIntelligenceRepository.ListTranscriptPage(
		sqls.DB(), tenantID, meetingID, decoded.StartedAtMS, decoded.CreatedAt, decoded.ID, hasCursor, limit,
	)
	if err != nil {
		return nil, "", false, err
	}
	nextCursor := ""
	if hasMore && len(items) > 0 {
		nextCursor = encodeMeetingTranscriptCursor(items[0])
	}
	return items, nextCursor, hasMore, nil
}

func decodeMeetingTranscriptCursor(value string) (meetingTranscriptCursor, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return meetingTranscriptCursor{}, false, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return meetingTranscriptCursor{}, false, err
	}
	var cursor meetingTranscriptCursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return meetingTranscriptCursor{}, false, err
	}
	if cursor.StartedAtMS < 0 || cursor.CreatedAt.IsZero() || strings.TrimSpace(cursor.ID) == "" {
		return meetingTranscriptCursor{}, false, errors.New("incomplete transcript cursor")
	}
	return cursor, true, nil
}

func encodeMeetingTranscriptCursor(item models.MeetingTranscriptSegment) string {
	payload, _ := json.Marshal(meetingTranscriptCursor{
		StartedAtMS: item.StartedAtMS,
		CreatedAt:   item.CreatedAt,
		ID:          item.ID,
	})
	return base64.RawURLEncoding.EncodeToString(payload)
}

func (s *meetingIntelligenceService) ListAnnotationsForOperator(meetingID string, operator *dto.AuthPrincipal) ([]models.MeetingARAnnotation, error) {
	meeting, _, err := MeetingService.requireMeetingTicketAccess(meetingID, operator, false)
	if err != nil {
		return nil, err
	}
	return repositories.MeetingIntelligenceRepository.ListAnnotations(sqls.DB(), meeting.TenantID, meeting.ID)
}

func (s *meetingIntelligenceService) ResolveAnnotationFrameURLs(items []models.MeetingARAnnotation) map[int64]string {
	result := make(map[int64]string)
	for i := range items {
		assetID := items[i].FrameAssetID
		if assetID <= 0 {
			continue
		}
		if _, exists := result[assetID]; exists {
			continue
		}
		asset := AssetService.Get(assetID)
		meeting := &models.MeetingRoomJitsi{ID: items[i].MeetingID, TenantID: items[i].TenantID}
		if validateMeetingFrameAsset(meeting, asset) != nil {
			continue
		}
		if url, err := AssetService.GetSignedURL(assetID); err == nil {
			result[assetID] = url
		}
	}
	return result
}

func (s *meetingIntelligenceService) CreateAnnotationForOperator(meetingID string, input request.MeetingARAnnotationCreateRequest, operator *dto.AuthPrincipal) (*models.MeetingARAnnotation, error) {
	meeting, _, err := MeetingService.requireMeetingTicketAccess(meetingID, operator, true)
	if err != nil {
		return nil, err
	}
	if meeting.Status == "ended" {
		return nil, errorsx.InvalidParam("meeting has ended")
	}
	asset := AssetService.Get(input.FrameAssetID)
	if err := validateMeetingFrameAsset(meeting, asset); err != nil {
		return nil, err
	}
	label := strings.TrimSpace(input.Label)
	if label == "" || len(label) > 128 {
		return nil, errorsx.InvalidParam("annotation label is required and must not exceed 128 characters")
	}
	if err := validateNormalizedBounds(input.Bounds); err != nil {
		return nil, err
	}
	if input.Confidence < 0 || input.Confidence > 1 {
		return nil, errorsx.InvalidParam("annotation confidence must be between 0 and 1")
	}
	provider := strings.TrimSpace(input.DetectionProvider)
	if provider == "" {
		provider = "manual"
	}
	color := strings.TrimSpace(input.Color)
	if color == "" {
		color = "#ef4444"
	}
	metadataJSON := strings.TrimSpace(input.MetadataJSON)
	if metadataJSON == "" {
		metadataJSON = "{}"
	}
	if !json.Valid([]byte(metadataJSON)) {
		return nil, errorsx.InvalidParam("annotation metadataJson must be valid JSON")
	}
	now := time.Now()
	item := &models.MeetingARAnnotation{
		ID: utils.UUID(), TenantID: meeting.TenantID, MeetingID: meeting.ID,
		TicketID: meeting.TicketID, FrameAssetID: input.FrameAssetID,
		DetectionProvider: provider, ExternalDetectionID: strings.TrimSpace(input.ExternalDetectionID),
		Label: label, PartCode: strings.TrimSpace(input.PartCode), Confidence: input.Confidence,
		X: input.Bounds.X, Y: input.Bounds.Y, Width: input.Bounds.Width, Height: input.Bounds.Height,
		Color: color, Note: strings.TrimSpace(input.Note), CreatedBy: operator.UserID,
		MetadataJSON: metadataJSON, CreatedAt: now, UpdatedAt: now,
	}
	if err := repositories.MeetingIntelligenceRepository.CreateAnnotation(sqls.DB(), item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *meetingIntelligenceService) DetectFrameForOperator(ctx context.Context, meetingID string, frameAssetID int64, operator *dto.AuthPrincipal) ([]models.MeetingARAnnotation, error) {
	meeting, _, err := MeetingService.requireMeetingTicketAccess(meetingID, operator, true)
	if err != nil {
		return nil, err
	}
	if meeting.Status == "ended" {
		return nil, errorsx.InvalidParam("meeting has ended")
	}
	asset := AssetService.Get(frameAssetID)
	if err := validateMeetingFrameAsset(meeting, asset); err != nil {
		return nil, err
	}

	detectionProvider := providers.CurrentMeetingARDetectionProvider()
	if detectionProvider == nil || !detectionProvider.Configured() {
		return nil, errorsx.BusinessError(100, "AR 识别服务尚未配置")
	}
	reader, err := AssetService.OpenReader(asset)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	if ctx == nil {
		ctx = context.Background()
	}
	detections, err := detectionProvider.Detect(ctx, providers.MeetingARDetectionInput{
		TenantID: meeting.TenantID, MeetingID: meeting.ID, TicketID: meeting.TicketID,
		FrameAssetID: frameAssetID, Filename: asset.Filename, ContentType: asset.MimeType, Image: reader,
	})
	if err != nil {
		return nil, errorsx.BusinessError(100, "AR 识别服务当前不可用")
	}
	if len(detections) == 0 {
		return []models.MeetingARAnnotation{}, nil
	}
	providerName := limitRunes(strings.TrimSpace(detectionProvider.Name()), 32)
	if providerName == "" || providerName == "disabled" {
		return nil, errorsx.BusinessError(100, "AR 识别服务尚未配置")
	}
	var items []models.MeetingARAnnotation
	err = sqls.DB().Transaction(func(tx *gorm.DB) error {
		var locked models.MeetingRoomJitsi
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, "id = ?", meeting.ID).Error; err != nil {
			return err
		}
		if locked.Status == "ended" {
			return errorsx.InvalidParam("meeting has ended")
		}
		existing, err := repositories.MeetingIntelligenceRepository.ListFrameAnnotations(
			tx, locked.TenantID, locked.ID, frameAssetID, providerName,
		)
		if err != nil {
			return err
		}
		if len(existing) > 0 {
			items = existing
			return nil
		}
		now := time.Now()
		items = make([]models.MeetingARAnnotation, 0, len(detections))
		for _, detection := range detections {
			bounds := request.MeetingARBoundsRequest{X: detection.X, Y: detection.Y, Width: detection.Width, Height: detection.Height}
			if err := validateNormalizedBounds(bounds); err != nil {
				return fmt.Errorf("AR provider returned invalid bounds: %w", err)
			}
			label := limitRunes(strings.TrimSpace(detection.Label), 128)
			if label == "" || detection.Confidence < 0 || detection.Confidence > 1 {
				return errorsx.BusinessError(100, "AR 识别服务返回了无效结果")
			}
			metadataJSON := strings.TrimSpace(detection.MetadataJSON)
			if metadataJSON == "" {
				metadataJSON = "{}"
			}
			if !json.Valid([]byte(metadataJSON)) {
				return errorsx.BusinessError(100, "AR 识别服务返回了无效元数据")
			}
			color := strings.TrimSpace(detection.Color)
			if color == "" {
				color = "#ef4444"
			}
			item := models.MeetingARAnnotation{
				ID: utils.UUID(), TenantID: locked.TenantID, MeetingID: locked.ID, TicketID: locked.TicketID,
				FrameAssetID: frameAssetID, DetectionProvider: providerName,
				ExternalDetectionID: limitRunes(strings.TrimSpace(detection.ExternalID), 128),
				Label:               label, PartCode: limitRunes(strings.TrimSpace(detection.PartCode), 128),
				Confidence: detection.Confidence, X: detection.X, Y: detection.Y,
				Width: detection.Width, Height: detection.Height, Color: color,
				Note: strings.TrimSpace(detection.Note), CreatedBy: operator.UserID,
				MetadataJSON: metadataJSON, CreatedAt: now, UpdatedAt: now,
			}
			if err := repositories.MeetingIntelligenceRepository.CreateAnnotation(tx, &item); err != nil {
				return err
			}
			items = append(items, item)
		}
		return nil
	})
	return items, err
}

func (s *meetingIntelligenceService) UploadFrameForOperator(meetingID string, file *multipart.FileHeader, operator *dto.AuthPrincipal) (*models.Asset, string, error) {
	meeting, _, err := MeetingService.requireMeetingTicketAccess(meetingID, operator, true)
	if err != nil {
		return nil, "", err
	}
	if meeting.Status == "ended" {
		return nil, "", errorsx.InvalidParam("meeting has ended")
	}
	asset, err := AssetService.UploadFile(file, fmt.Sprintf("meeting-frames/%s", meeting.ID), operator)
	if err != nil {
		return nil, "", err
	}
	url, err := AssetService.GetSignedURL(asset.ID)
	if err != nil {
		return nil, "", err
	}
	return asset, url, nil
}

func validateNormalizedBounds(bounds request.MeetingARBoundsRequest) error {
	if bounds.X < 0 || bounds.Y < 0 || bounds.Width <= 0 || bounds.Height <= 0 ||
		bounds.X > 1 || bounds.Y > 1 || bounds.Width > 1 || bounds.Height > 1 ||
		bounds.X+bounds.Width > 1.000001 || bounds.Y+bounds.Height > 1.000001 {
		return errorsx.InvalidParam("annotation bounds must be normalized within the frame")
	}
	return nil
}

func validateMeetingFrameAsset(meeting *models.MeetingRoomJitsi, asset *models.Asset) error {
	if meeting == nil || asset == nil || asset.TenantID != meeting.TenantID || asset.Status != enums.AssetStatusSuccess {
		return errorsx.InvalidParam("annotation frame does not belong to this meeting")
	}
	expectedFramePath := "/meeting-frames/" + meeting.ID + "/"
	if !strings.Contains("/"+asset.StorageKey, expectedFramePath) {
		return errorsx.InvalidParam("annotation frame does not belong to this meeting")
	}
	return nil
}

type MeetingTranscriptSpeaker struct {
	ParticipantID string
	Name          string
	Language      string
}

const maxMeetingTranscriptOffsetMS = int64((7 * 24 * time.Hour) / time.Millisecond)

const (
	meetingTranscriptSourceProvider        = "provider"
	meetingTranscriptSourceClient          = "client"
	meetingTranscriptSourceJigasi          = "jigasi"
	meetingTranscriptSourceSystem          = "system"
	meetingTranscriptMaxTranslationRetries = 8
	meetingTranscriptTranslationWorkers    = 4
)

func (s *meetingIntelligenceService) SaveTranscriptEvent(ctx context.Context, meeting *models.MeetingRoomJitsi, speaker MeetingTranscriptSpeaker, providerName string, event *providers.SpeechTranscriptionEvent) (*models.MeetingTranscriptSegment, error) {
	return s.saveTranscriptEvent(ctx, meeting, speaker, providerName, meetingTranscriptSourceProvider, event)
}

func (s *meetingIntelligenceService) EnsureClosureTranscriptArchiveDB(db *gorm.DB, meeting *models.MeetingRoomJitsi, durationMs int64, endedAt time.Time) error {
	if db == nil || meeting == nil {
		return nil
	}
	if !db.Migrator().HasTable(&models.MeetingTranscriptSegment{}) {
		return nil
	}
	count, err := repositories.MeetingIntelligenceRepository.CountTranscripts(db, meeting.TenantID, meeting.ID)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if endedAt.IsZero() {
		endedAt = time.Now()
	}
	if durationMs < 0 {
		durationMs = 0
	}
	metadata, err := json.Marshal(map[string]any{
		"type":       "empty_transcript_closure",
		"meetingId":  meeting.ID,
		"roomName":   meeting.RoomName,
		"ticketId":   meeting.TicketID,
		"durationMs": durationMs,
		"endedAt":    endedAt.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	text := fmt.Sprintf("会议已结束，系统已归档本次视频会话记录。时长：%s；未收到实时语音转写片段。", formatMeetingDurationText(durationMs))
	if strings.TrimSpace(meeting.RoomName) != "" {
		text = fmt.Sprintf("会议已结束，系统已归档本次视频会话记录。会议室：%s；时长：%s；未收到实时语音转写片段。", meeting.RoomName, formatMeetingDurationText(durationMs))
	}
	item := &models.MeetingTranscriptSegment{
		ID:              utils.UUID(),
		TenantID:        meeting.TenantID,
		MeetingID:       meeting.ID,
		ParticipantID:   "system",
		SpeakerName:     "系统记录",
		Provider:        "system",
		ProviderEventID: "meeting-ended:" + meeting.ID,
		IngestSource:    meetingTranscriptSourceSystem,
		Language:        "zh-CN",
		Text:            text,
		IsFinal:         true,
		StartedAtMS:     0,
		EndedAtMS:       durationMs,
		Confidence:      1,
		RawJSON:         string(metadata),
		BaseModel:       models.BaseModel{CreatedAt: endedAt, UpdatedAt: endedAt},
	}
	_, err = repositories.MeetingIntelligenceRepository.CreateTranscriptIfAbsent(db, item)
	return err
}

func (s *meetingIntelligenceService) BackfillEndedMeetingClosureArchivesDB(db *gorm.DB, limit int) (int, error) {
	if db == nil ||
		!db.Migrator().HasTable(&models.MeetingRoomJitsi{}) ||
		!db.Migrator().HasTable(&models.MeetingTranscriptSegment{}) {
		return 0, nil
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > 2000 {
		limit = 2000
	}
	var meetings []models.MeetingRoomJitsi
	if err := db.Model(&models.MeetingRoomJitsi{}).
		Where("status = ?", "ended").
		Where("NOT EXISTS (?)",
			db.Model(&models.MeetingTranscriptSegment{}).
				Select("1").
				Where("meeting_transcript_segments.tenant_id = meeting_rooms_jitsi.tenant_id").
				Where("meeting_transcript_segments.meeting_id = meeting_rooms_jitsi.id"),
		).
		Order("ended_at ASC, created_at ASC, id ASC").
		Limit(limit).
		Find(&meetings).Error; err != nil {
		return 0, err
	}
	backfilled := 0
	for i := range meetings {
		meeting := meetings[i]
		endedAt := firstNonNilTime(meeting.EndedAt, meeting.UpdatedAt)
		durationMs := endedMeetingDurationMs(db, &meeting, endedAt)
		if err := s.EnsureClosureTranscriptArchiveDB(db, &meeting, durationMs, endedAt); err != nil {
			return backfilled, err
		}
		backfilled++
	}
	return backfilled, nil
}

func (s *meetingIntelligenceService) saveTranscriptEvent(ctx context.Context, meeting *models.MeetingRoomJitsi, speaker MeetingTranscriptSpeaker, providerName, ingestSource string, event *providers.SpeechTranscriptionEvent) (*models.MeetingTranscriptSegment, error) {
	if meeting == nil || event == nil {
		return nil, errorsx.InvalidParam("invalid transcript event")
	}
	text := strings.TrimSpace(event.Text)
	if text == "" || len([]rune(text)) > 4000 || event.Confidence < 0 || event.Confidence > 1 {
		return nil, errorsx.InvalidParam("invalid transcript text or confidence")
	}
	providerEventID := normalizeProviderEventID(event.ProviderEventID)
	if providerEventID == "" {
		providerEventID = utils.UUID()
	}
	participantID := limitRunes(strings.TrimSpace(speaker.ParticipantID), 64)
	if participantID == "" {
		participantID = "unknown"
	}
	speakerName := limitRunes(strings.TrimSpace(speaker.Name), 100)
	language := limitRunes(normalizeTranscriptLanguage(speaker.Language, text), 16)
	providerName = limitRunes(strings.TrimSpace(providerName), 32)
	if providerName == "" {
		providerName = "speech_gateway"
	}
	ingestSource = limitRunes(strings.TrimSpace(ingestSource), 24)
	if ingestSource == "" {
		ingestSource = meetingTranscriptSourceProvider
	}
	startedAtMS, endedAtMS, err := normalizeMeetingTranscriptTimingDB(sqls.DB(), meeting, event.StartedAtMS, event.EndedAtMS)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	item := &models.MeetingTranscriptSegment{
		ID: utils.UUID(), TenantID: meeting.TenantID, MeetingID: meeting.ID,
		ParticipantID: participantID, SpeakerName: speakerName,
		Provider: providerName, ProviderEventID: providerEventID, IngestSource: ingestSource, Language: language,
		Text: text, IsFinal: event.IsFinal,
		StartedAtMS: startedAtMS, EndedAtMS: endedAtMS,
		Confidence: event.Confidence, RawJSON: event.RawJSON,
		TranslationStatus: "not_configured",
		BaseModel:         models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
	translator := providers.CurrentTextTranslationProvider()
	if event.IsFinal && translator != nil && translator.Configured() {
		item.TranslationStatus = "pending"
		item.TranslationProvider = limitRunes(translator.Name(), 32)
		item.TranslationNextRetryAt = &now
	}
	created, err := repositories.MeetingIntelligenceRepository.CreateTranscriptIfAbsent(sqls.DB(), item)
	if err != nil {
		return nil, err
	}
	if !created {
		reconciled := false
		if ingestSource == meetingTranscriptSourceJigasi {
			reconciled, err = repositories.MeetingIntelligenceRepository.ReconcileClientTranscript(sqls.DB(), item)
			if err != nil {
				return nil, err
			}
		}
		if !reconciled {
			return repositories.MeetingIntelligenceRepository.GetTranscriptByProviderEvent(
				sqls.DB(), meeting.TenantID, meeting.ID, item.Provider, item.ProviderEventID,
			)
		}
	}
	persisted, err := repositories.MeetingIntelligenceRepository.GetTranscriptByProviderEvent(
		sqls.DB(), meeting.TenantID, meeting.ID, item.Provider, item.ProviderEventID,
	)
	if err != nil {
		return nil, err
	}
	return persisted, nil
}

func firstNonNilTime(value *time.Time, fallback time.Time) time.Time {
	if value != nil && !value.IsZero() {
		return *value
	}
	if !fallback.IsZero() {
		return fallback
	}
	return time.Now()
}

func endedMeetingDurationMs(db *gorm.DB, meeting *models.MeetingRoomJitsi, endedAt time.Time) int64 {
	if meeting == nil || endedAt.IsZero() {
		return 0
	}
	attendance := repositories.MeetingRoomRepository.AttendanceStats(db, meeting.ID)
	startedAt := confirmedMeetingStartedAt(meeting, attendance)
	if startedAt == nil || !endedAt.After(*startedAt) {
		return 0
	}
	return endedAt.Sub(*startedAt).Milliseconds()
}

func (s *meetingIntelligenceService) ProcessPendingTranscriptTranslations(ctx context.Context, limit int) int {
	translator := providers.CurrentTextTranslationProvider()
	if translator == nil || !translator.Configured() {
		return 0
	}
	if ctx == nil {
		ctx = context.Background()
	}
	now := time.Now()
	items, err := repositories.MeetingIntelligenceRepository.ListDueTranscriptTranslations(sqls.DB(), now, limit)
	if err != nil {
		slog.Warn("list pending meeting transcript translations failed", "error", err)
		return 0
	}
	workerCount := min(meetingTranscriptTranslationWorkers, len(items))
	if workerCount == 0 {
		return 0
	}
	jobs := make(chan models.MeetingTranscriptSegment)
	var processed atomic.Int64
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for item := range jobs {
				if ctx.Err() != nil {
					continue
				}
				claimTime := time.Now()
				leaseUntil := claimTime.Add(30 * time.Second)
				claimed, claimErr := repositories.MeetingIntelligenceRepository.ClaimTranscriptTranslation(sqls.DB(), item.ID, claimTime, leaseUntil)
				if claimErr != nil {
					slog.Warn("claim meeting transcript translation failed", "transcript_id", item.ID, "error", claimErr)
					continue
				}
				if !claimed {
					continue
				}
				processed.Add(1)
				s.processTranscriptTranslation(ctx, translator, &item, leaseUntil)
			}
		}()
	}
	for i := range items {
		jobs <- items[i]
	}
	close(jobs)
	workers.Wait()
	return int(processed.Load())
}

func (s *meetingIntelligenceService) processTranscriptTranslation(ctx context.Context, translator providers.TextTranslationProvider, item *models.MeetingTranscriptSegment, expectedLeaseUntil time.Time) {
	translationCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	translation, translationErr := translator.Translate(translationCtx, providers.TextTranslationRequest{
		Text: item.Text, SourceLanguage: item.Language,
	})
	if translationErr == nil && (translation == nil || strings.TrimSpace(translation.Text) == "") {
		translationErr = fmt.Errorf("translation provider returned an empty result")
	}
	now := time.Now()
	item.TranslationUpdatedAt = &now
	item.TranslationLeaseUntil = nil
	item.UpdatedAt = now
	if translationErr != nil {
		item.TranslationStatus = "failed"
		item.TranslationRetryCount++
		if item.TranslationRetryCount < meetingTranscriptMaxTranslationRetries {
			nextRetryAt := now.Add(meetingTranscriptTranslationBackoff(item.TranslationRetryCount))
			item.TranslationNextRetryAt = &nextRetryAt
		} else {
			item.TranslationNextRetryAt = nil
		}
		item.TranslationError = limitRunes(translationErr.Error(), 500)
		slog.Warn("meeting transcript translation failed", "meeting_id", item.MeetingID, "event_id", item.ProviderEventID, "retry_count", item.TranslationRetryCount, "error", translationErr)
	} else {
		item.TranslatedText = strings.TrimSpace(translation.Text)
		item.TranslatedLanguage = limitRunes(strings.TrimSpace(translation.TargetLanguage), 16)
		item.TranslationProvider = limitRunes(firstNonEmptyString(translation.Provider, translator.Name()), 32)
		item.TranslationStatus = "completed"
		item.TranslationError = ""
		item.TranslationNextRetryAt = nil
	}
	updated, err := repositories.MeetingIntelligenceRepository.UpdateTranscriptTranslation(sqls.DB(), item, expectedLeaseUntil)
	if err != nil {
		slog.Warn("persist meeting transcript translation result failed", "transcript_id", item.ID, "error", err)
	} else if !updated {
		slog.Debug("discarded stale meeting transcript translation result", "transcript_id", item.ID)
	}
}

func meetingTranscriptTranslationBackoff(retryCount int) time.Duration {
	if retryCount < 1 {
		retryCount = 1
	}
	if retryCount > meetingTranscriptMaxTranslationRetries {
		retryCount = meetingTranscriptMaxTranslationRetries
	}
	return time.Duration(1<<(retryCount-1)) * 5 * time.Second
}

func (s *meetingIntelligenceService) IngestClientTranscriptForOperator(ctx context.Context, meetingID string, input request.MeetingTranscriptIngestRequest, operator *dto.AuthPrincipal) (*models.MeetingTranscriptSegment, error) {
	meeting, _, err := MeetingService.requireMeetingTicketAccess(meetingID, operator, true)
	if err != nil {
		return nil, err
	}
	if meeting.Status == "ended" {
		return nil, errorsx.InvalidParam("meeting has ended")
	}
	speaker := MeetingTranscriptSpeaker{
		ParticipantID: strconv.FormatInt(operator.UserID, 10),
		Name:          firstNonEmptyString(strings.TrimSpace(operator.Nickname), strings.TrimSpace(operator.Username), fmt.Sprintf("Engineer %d", operator.UserID)),
		Language:      input.Language,
	}
	return s.IngestClientTranscript(ctx, meeting, speaker, input)
}

func (s *meetingIntelligenceService) IngestClientTranscript(ctx context.Context, meeting *models.MeetingRoomJitsi, speaker MeetingTranscriptSpeaker, input request.MeetingTranscriptIngestRequest) (*models.MeetingTranscriptSegment, error) {
	if meeting == nil || meeting.Status == "ended" {
		return nil, errorsx.InvalidParam("meeting is not active")
	}
	if !input.IsFinal {
		return nil, errorsx.InvalidParam("only final transcript segments can be archived")
	}
	text := strings.TrimSpace(input.Text)
	if text == "" || len([]rune(text)) > 4000 {
		return nil, errorsx.InvalidParam("transcript text is required and must not exceed 4000 characters")
	}
	if input.Confidence < 0 || input.Confidence > 1 || input.StartedAtMS < 0 || input.EndedAtMS < 0 {
		return nil, errorsx.InvalidParam("invalid transcript timing or confidence")
	}
	providerEventID := normalizeProviderEventID(input.ProviderEventID)
	if providerEventID == "" {
		digest := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|%s", meeting.ID, speaker.ParticipantID, input.StartedAtMS, text)))
		providerEventID = fmt.Sprintf("sha256:%x", digest[:16])
	}
	rawJSON, _ := json.Marshal(input)
	provider := "jitsi_caption"
	if strings.EqualFold(strings.TrimSpace(input.Provider), "jitsi_chat") {
		provider = "jitsi_chat"
	}
	return s.saveTranscriptEvent(ctx, meeting, speaker, provider, meetingTranscriptSourceClient, &providers.SpeechTranscriptionEvent{
		ProviderEventID: providerEventID, Text: text, IsFinal: true,
		StartedAtMS: input.StartedAtMS, EndedAtMS: input.EndedAtMS,
		Confidence: input.Confidence, RawJSON: string(rawJSON),
	})
}

func (s *meetingIntelligenceService) IngestJigasiTranscriptEvent(ctx context.Context, input request.JigasiTranscriptEventRequest) (bool, error) {
	if !strings.EqualFold(strings.TrimSpace(input.Event), "SPEECH") {
		return false, nil
	}
	if input.IsInterim {
		return false, nil
	}
	roomName := strings.TrimSpace(input.RoomName)
	if roomName == "" || len(input.Transcript) == 0 {
		return false, errorsx.InvalidParam("jigasi transcript room and text are required")
	}
	text := strings.TrimSpace(input.Transcript[0].Text)
	if text == "" {
		return false, nil
	}
	meeting, err := MeetingService.resolveWebhookMeeting("", roomName)
	if err != nil {
		return false, err
	}
	participantID := firstNonEmptyString(
		strings.TrimSpace(input.Participant.IdentityUserID),
		strings.TrimSpace(input.Participant.ID),
		strings.TrimSpace(input.Participant.IdentityName),
	)
	// Jigasi callbacks do not carry the upstream engine identity. Keep the
	// source stable across provider switches so delayed callbacks deduplicate.
	providerName := "jitsi_caption"
	providerEventID := normalizeProviderEventID(input.MessageID)
	if providerEventID == "" {
		digest := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|%s", meeting.RoomName, participantID, input.Timestamp, text)))
		providerEventID = fmt.Sprintf("sha256:%x", digest[:16])
	}
	relativeTimestamp := input.Timestamp
	if meeting.StartedAt != nil && input.Timestamp >= meeting.StartedAt.UnixMilli() {
		relativeTimestamp = input.Timestamp - meeting.StartedAt.UnixMilli()
	}
	event := &providers.SpeechTranscriptionEvent{
		ProviderEventID: providerEventID,
		Text:            text,
		IsFinal:         !input.IsInterim,
		StartedAtMS:     relativeTimestamp,
		EndedAtMS:       relativeTimestamp,
		Confidence:      input.Transcript[0].Confidence,
		RawJSON:         "{}",
	}
	_, err = s.saveTranscriptEvent(ctx, meeting, MeetingTranscriptSpeaker{
		ParticipantID: participantID,
		Name:          input.Participant.Name,
		Language:      input.Language,
	}, providerName, meetingTranscriptSourceJigasi, event)
	return err == nil, err
}

func normalizeProviderEventID(value string) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) <= 128 {
		return value
	}
	digest := sha256.Sum256([]byte(value))
	return fmt.Sprintf("sha256:%x", digest[:])
}

func normalizeMeetingTranscriptTiming(meeting *models.MeetingRoomJitsi, startedAtMS, endedAtMS int64) (int64, int64, error) {
	return normalizeMeetingTranscriptTimingDB(nil, meeting, startedAtMS, endedAtMS)
}

func normalizeMeetingTranscriptTimingDB(db *gorm.DB, meeting *models.MeetingRoomJitsi, startedAtMS, endedAtMS int64) (int64, int64, error) {
	if meeting == nil || startedAtMS < 0 || endedAtMS < 0 {
		return 0, 0, errorsx.InvalidParam("invalid transcript timing")
	}
	origin := meeting.CreatedAt
	if meeting.StartedAt != nil && !meeting.StartedAt.IsZero() {
		origin = *meeting.StartedAt
	}
	normalize := func(value int64) int64 {
		if value > maxMeetingTranscriptOffsetMS && !origin.IsZero() {
			return value - dbtime.WallClockUnixMilli(db, origin)
		}
		return value
	}
	startedAtMS = normalize(startedAtMS)
	endedAtMS = normalize(endedAtMS)
	if startedAtMS < 0 || endedAtMS < startedAtMS || startedAtMS > maxMeetingTranscriptOffsetMS || endedAtMS > maxMeetingTranscriptOffsetMS {
		return 0, 0, errorsx.InvalidParam("transcript timing is outside the meeting timeline")
	}
	return startedAtMS, endedAtMS, nil
}

func normalizeTranscriptLanguage(value, text string) string {
	language := strings.TrimSpace(value)
	normalized := strings.ToLower(strings.ReplaceAll(language, "_", "-"))
	switch normalized {
	case "zh", "zh-cn", "cn", "cmn", "cmn-hans":
		return "zh-CN"
	case "zh-tw", "zh-hk", "zh-hant", "cmn-hant":
		return language
	}
	if transcriptLooksChinese(text) {
		return "zh-CN"
	}
	if normalized == "en" || normalized == "en-us" || normalized == "en-gb" {
		return "en-US"
	}
	if language == "" {
		if transcriptLooksLatin(text) {
			return "en-US"
		}
		return "zh-CN"
	}
	return language
}

func transcriptLooksChinese(value string) bool {
	hanCount := 0
	latinCount := 0
	for _, character := range value {
		switch {
		case unicode.Is(unicode.Han, character):
			hanCount++
		case unicode.Is(unicode.Latin, character):
			latinCount++
		}
	}
	return hanCount >= 2 && hanCount*2 >= latinCount
}

func transcriptLooksLatin(value string) bool {
	latinCount := 0
	for _, character := range value {
		if unicode.Is(unicode.Latin, character) {
			latinCount++
		}
	}
	return latinCount >= 2
}

func limitRunes(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}
