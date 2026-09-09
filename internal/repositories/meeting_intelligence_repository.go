package repositories

import (
	"strings"
	"time"

	"remotehelpdesk/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var MeetingIntelligenceRepository = &meetingIntelligenceRepository{}

type meetingIntelligenceRepository struct{}

type MeetingArchiveStats struct {
	TranscriptCount int64
	AnnotationCount int64
}

type meetingArchiveCountRow struct {
	MeetingID string `gorm:"column:meeting_id"`
	Count     int64  `gorm:"column:item_count"`
}

func (r *meetingIntelligenceRepository) ArchiveStatsByMeetingIDs(db *gorm.DB, meetingIDs []string) map[string]MeetingArchiveStats {
	stats := make(map[string]MeetingArchiveStats, len(meetingIDs))
	if len(meetingIDs) == 0 {
		return stats
	}
	var transcriptCounts []meetingArchiveCountRow
	db.Model(&models.MeetingTranscriptSegment{}).
		Select("meeting_id, COUNT(*) AS item_count").
		Where("meeting_id IN ?", meetingIDs).
		Where("ingest_source <> ? AND provider <> ?", "system", "system").
		Group("meeting_id").
		Scan(&transcriptCounts)
	for _, row := range transcriptCounts {
		item := stats[row.MeetingID]
		item.TranscriptCount = row.Count
		stats[row.MeetingID] = item
	}
	var annotationCounts []meetingArchiveCountRow
	db.Model(&models.MeetingARAnnotation{}).
		Select("meeting_id, COUNT(*) AS item_count").
		Where("meeting_id IN ?", meetingIDs).
		Group("meeting_id").
		Scan(&annotationCounts)
	for _, row := range annotationCounts {
		item := stats[row.MeetingID]
		item.AnnotationCount = row.Count
		stats[row.MeetingID] = item
	}
	return stats
}

func (r *meetingIntelligenceRepository) CountTranscripts(db *gorm.DB, tenantID int64, meetingID string) (int64, error) {
	var count int64
	err := db.Model(&models.MeetingTranscriptSegment{}).
		Where("tenant_id = ? AND meeting_id = ?", tenantID, meetingID).
		Where("ingest_source <> ? AND provider <> ?", "system", "system").
		Count(&count).Error
	return count, err
}

func (r *meetingIntelligenceRepository) ListTranscripts(db *gorm.DB, tenantID int64, meetingID string, limit int) ([]models.MeetingTranscriptSegment, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	var items []models.MeetingTranscriptSegment
	err := db.Where("tenant_id = ? AND meeting_id = ?", tenantID, meetingID).
		Order("started_at_ms DESC, created_at DESC, id DESC").Limit(limit).Find(&items).Error
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	return items, err
}

func (r *meetingIntelligenceRepository) ListTranscriptPage(
	db *gorm.DB,
	tenantID int64,
	meetingID string,
	cursorStartedAtMS int64,
	cursorCreatedAt time.Time,
	cursorID string,
	hasCursor bool,
	limit int,
) ([]models.MeetingTranscriptSegment, bool, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	query := db.Where("tenant_id = ? AND meeting_id = ?", tenantID, meetingID)
	if hasCursor {
		query = query.Where(
			"started_at_ms < ? OR (started_at_ms = ? AND created_at < ?) OR "+
				"(started_at_ms = ? AND created_at = ? AND id < ?)",
			cursorStartedAtMS,
			cursorStartedAtMS,
			cursorCreatedAt,
			cursorStartedAtMS,
			cursorCreatedAt,
			cursorID,
		)
	}
	var items []models.MeetingTranscriptSegment
	if err := query.Order("started_at_ms DESC, created_at DESC, id DESC").Limit(limit + 1).Find(&items).Error; err != nil {
		return nil, false, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	return items, hasMore, nil
}

func (r *meetingIntelligenceRepository) CreateTranscriptIfAbsent(db *gorm.DB, item *models.MeetingTranscriptSegment) (bool, error) {
	if item == nil {
		return false, nil
	}
	var existing models.MeetingTranscriptSegment
	if err := db.Where(
		"tenant_id = ? AND meeting_id = ? AND provider = ? AND provider_event_id = ?",
		item.TenantID, item.MeetingID, item.Provider, item.ProviderEventID,
	).First(&existing).Error; err == nil {
		return false, nil
	} else if err != nil && err != gorm.ErrRecordNotFound {
		return false, err
	}
	result := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "meeting_id"},
			{Name: "provider"},
			{Name: "provider_event_id"},
		},
		DoNothing: true,
	}).Create(item)
	if result.Error != nil && isMissingTranscriptConflictConstraint(result.Error) {
		fallback := db.Create(item)
		return fallback.RowsAffected == 1, fallback.Error
	}
	return result.RowsAffected == 1, result.Error
}

func isMissingTranscriptConflictConstraint(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "on conflict clause does not match") ||
		strings.Contains(message, "no unique or exclusion constraint matching the on conflict")
}

func (r *meetingIntelligenceRepository) ReconcileClientTranscript(db *gorm.DB, item *models.MeetingTranscriptSegment) (bool, error) {
	result := db.Model(&models.MeetingTranscriptSegment{}).
		Where("tenant_id = ? AND meeting_id = ? AND provider = ? AND provider_event_id = ? AND ingest_source = ?",
			item.TenantID, item.MeetingID, item.Provider, item.ProviderEventID, "client").
		Updates(map[string]any{
			"participant_id":            item.ParticipantID,
			"speaker_name":              item.SpeakerName,
			"ingest_source":             item.IngestSource,
			"language":                  item.Language,
			"text":                      item.Text,
			"is_final":                  item.IsFinal,
			"started_at_ms":             item.StartedAtMS,
			"ended_at_ms":               item.EndedAtMS,
			"confidence":                item.Confidence,
			"raw_json":                  item.RawJSON,
			"translated_language":       item.TranslatedLanguage,
			"translated_text":           item.TranslatedText,
			"translation_provider":      item.TranslationProvider,
			"translation_status":        item.TranslationStatus,
			"translation_error":         item.TranslationError,
			"translation_updated_at":    item.TranslationUpdatedAt,
			"translation_retry_count":   item.TranslationRetryCount,
			"translation_next_retry_at": item.TranslationNextRetryAt,
			"translation_lease_until":   item.TranslationLeaseUntil,
			"updated_at":                item.UpdatedAt,
		})
	return result.RowsAffected == 1, result.Error
}

func (r *meetingIntelligenceRepository) UpdateTranscriptTranslation(db *gorm.DB, item *models.MeetingTranscriptSegment, expectedLeaseUntil time.Time) (bool, error) {
	result := db.Model(&models.MeetingTranscriptSegment{}).
		Where("id = ? AND tenant_id = ? AND translation_status = ? AND translation_lease_until = ?",
			item.ID, item.TenantID, "processing", expectedLeaseUntil).
		Updates(map[string]any{
			"translated_language":       item.TranslatedLanguage,
			"translated_text":           item.TranslatedText,
			"translation_provider":      item.TranslationProvider,
			"translation_status":        item.TranslationStatus,
			"translation_error":         item.TranslationError,
			"translation_updated_at":    item.TranslationUpdatedAt,
			"translation_retry_count":   item.TranslationRetryCount,
			"translation_next_retry_at": item.TranslationNextRetryAt,
			"translation_lease_until":   item.TranslationLeaseUntil,
			"updated_at":                item.UpdatedAt,
		})
	return result.RowsAffected == 1, result.Error
}

func (r *meetingIntelligenceRepository) ListDueTranscriptTranslations(db *gorm.DB, now time.Time, limit int) ([]models.MeetingTranscriptSegment, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	var items []models.MeetingTranscriptSegment
	err := db.Where(
		"(translation_status = ? AND (translation_next_retry_at IS NULL OR translation_next_retry_at <= ?)) OR "+
			"(translation_status = ? AND translation_next_retry_at IS NOT NULL AND translation_next_retry_at <= ?) OR "+
			"(translation_status = ? AND translation_lease_until IS NOT NULL AND translation_lease_until <= ?)",
		"pending", now, "failed", now, "processing", now,
	).Order("COALESCE(translation_next_retry_at, created_at) ASC, created_at ASC").Limit(limit).Find(&items).Error
	return items, err
}

func (r *meetingIntelligenceRepository) ClaimTranscriptTranslation(db *gorm.DB, id string, now, leaseUntil time.Time) (bool, error) {
	result := db.Model(&models.MeetingTranscriptSegment{}).
		Where("id = ? AND ((translation_status = ? AND (translation_next_retry_at IS NULL OR translation_next_retry_at <= ?)) OR "+
			"(translation_status = ? AND translation_next_retry_at IS NOT NULL AND translation_next_retry_at <= ?) OR "+
			"(translation_status = ? AND translation_lease_until IS NOT NULL AND translation_lease_until <= ?))",
			id, "pending", now, "failed", now, "processing", now).
		Updates(map[string]any{
			"translation_status":      "processing",
			"translation_lease_until": leaseUntil,
			"updated_at":              now,
		})
	return result.RowsAffected == 1, result.Error
}

func (r *meetingIntelligenceRepository) GetTranscriptByProviderEvent(db *gorm.DB, tenantID int64, meetingID, provider, providerEventID string) (*models.MeetingTranscriptSegment, error) {
	var item models.MeetingTranscriptSegment
	err := db.Where(
		"tenant_id = ? AND meeting_id = ? AND provider = ? AND provider_event_id = ?",
		tenantID, meetingID, provider, providerEventID,
	).First(&item).Error
	return &item, err
}

func (r *meetingIntelligenceRepository) ListAnnotations(db *gorm.DB, tenantID int64, meetingID string) ([]models.MeetingARAnnotation, error) {
	var items []models.MeetingARAnnotation
	err := db.Where("tenant_id = ? AND meeting_id = ?", tenantID, meetingID).
		Order("created_at ASC").Find(&items).Error
	return items, err
}

func (r *meetingIntelligenceRepository) CreateAnnotation(db *gorm.DB, item *models.MeetingARAnnotation) error {
	return db.Create(item).Error
}

func (r *meetingIntelligenceRepository) ListFrameAnnotations(db *gorm.DB, tenantID int64, meetingID string, frameAssetID int64, provider string) ([]models.MeetingARAnnotation, error) {
	var items []models.MeetingARAnnotation
	err := db.Where(
		"tenant_id = ? AND meeting_id = ? AND frame_asset_id = ? AND detection_provider = ?",
		tenantID,
		meetingID,
		frameAssetID,
		provider,
	).Order("created_at ASC").Find(&items).Error
	return items, err
}
