package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var ServiceOutcomeFactService = newServiceOutcomeFactService()

func newServiceOutcomeFactService() *serviceOutcomeFactService {
	return &serviceOutcomeFactService{}
}

type serviceOutcomeFactService struct{}

// RebuildMissingBatch gradually backfills historical terminal tickets without
// making report requests scan and rebuild the full ticket history.
func (s *serviceOutcomeFactService) RebuildMissingBatch(ctx context.Context, limit int) (int, error) {
	db := sqls.DB()
	if db == nil || !db.Migrator().HasTable(&models.TicketServiceOutcomeFact{}) {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	ticketTable := db.NamingStrategy.TableName("ticket")
	factTable := db.NamingStrategy.TableName("ticket_service_outcome_fact")
	var tickets []models.Ticket
	if err := db.WithContext(ctx).Model(&models.Ticket{}).
		Select(ticketTable+".tenant_id, "+ticketTable+".id").
		Joins("LEFT JOIN "+factTable+" AS outcome_fact ON outcome_fact.tenant_id = "+ticketTable+".tenant_id AND outcome_fact.ticket_id = "+ticketTable+".id AND outcome_fact.metric_version = ?", models.ServiceOutcomeMetricVersionV1).
		Where(ticketTable+".tenant_id > 0 AND "+ticketTable+".status IN ? AND outcome_fact.id IS NULL", []enums.TicketStatus{
			enums.TicketStatusResolved,
			enums.TicketStatusClosed,
			enums.TicketStatusDone,
		}).
		Order(ticketTable + ".id ASC").
		Limit(limit).
		Find(&tickets).Error; err != nil {
		return 0, fmt.Errorf("load missing service outcome facts: %w", err)
	}

	processed := 0
	var rebuildErrors []error
	for i := range tickets {
		if _, err := s.RebuildTicket(ctx, tickets[i].TenantID, tickets[i].ID); err != nil {
			rebuildErrors = append(rebuildErrors, fmt.Errorf("ticket %d: %w", tickets[i].ID, err))
			continue
		}
		processed++
	}
	return processed, errors.Join(rebuildErrors...)
}

func (s *serviceOutcomeFactService) RebuildTicket(ctx context.Context, tenantID, ticketID int64) (*models.TicketServiceOutcomeFact, error) {
	if tenantID <= 0 || ticketID <= 0 {
		return nil, fmt.Errorf("tenant id and ticket id are required")
	}
	db := sqls.DB()
	if db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var rebuilt *models.TicketServiceOutcomeFact
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ").Error; err != nil {
				return fmt.Errorf("set service outcome transaction isolation: %w", err)
			}
		}
		sources, err := repositories.ServiceOutcomeFactRepository.LoadSources(tx, tenantID, ticketID)
		if err != nil {
			return fmt.Errorf("load ticket service outcome sources: %w", err)
		}
		fact, err := buildTicketServiceOutcomeFact(sources)
		if err != nil {
			return err
		}

		existing, findErr := repositories.ServiceOutcomeFactRepository.Find(tx, tenantID, ticketID, fact.MetricVersion)
		if findErr != nil && findErr != gorm.ErrRecordNotFound {
			return fmt.Errorf("load existing ticket service outcome fact: %w", findErr)
		}
		if existing != nil && existing.SourceFingerprint == fact.SourceFingerprint {
			rebuilt = existing
			return nil
		}

		now := time.Now().UTC()
		fact.BuiltAt = now
		fact.AuditFields = models.AuditFields{
			CreatedAt:      now,
			CreateUserName: "system",
			UpdatedAt:      now,
			UpdateUserName: "system",
		}
		if existing != nil {
			fact.ID = existing.ID
			fact.CreatedAt = existing.CreatedAt
			fact.CreateUserID = existing.CreateUserID
			fact.CreateUserName = existing.CreateUserName
		}
		if err := repositories.ServiceOutcomeFactRepository.Upsert(tx, fact); err != nil {
			return fmt.Errorf("upsert ticket service outcome fact: %w", err)
		}
		rebuilt, err = repositories.ServiceOutcomeFactRepository.Find(tx, tenantID, ticketID, fact.MetricVersion)
		if err != nil {
			return fmt.Errorf("reload ticket service outcome fact: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rebuilt, nil
}

func buildTicketServiceOutcomeFact(sources *repositories.ServiceOutcomeFactSources) (*models.TicketServiceOutcomeFact, error) {
	if sources == nil {
		return nil, fmt.Errorf("service outcome sources are required")
	}
	ticket := sources.Ticket
	fact := &models.TicketServiceOutcomeFact{
		TenantID:           ticket.TenantID,
		TicketID:           ticket.ID,
		MetricVersion:      models.ServiceOutcomeMetricVersionV1,
		ProductID:          ticket.ProductID,
		ProductModelID:     ticket.ProductModelID,
		DeviceID:           ticket.DeviceID,
		ConversationID:     ticket.ConversationID,
		TicketStatus:       string(ticket.Status),
		TicketCreatedAt:    ticket.CreatedAt,
		TicketResolvedAt:   ticket.ResolvedAt,
		SourceMaxUpdatedAt: latestOutcomeSourceTime(sources),
	}

	fact.DiagnosisSessionCount = len(sources.Diagnoses)
	fact.AIEngaged = len(sources.Diagnoses) > 0
	if sources.Conversation != nil {
		fact.AIEngaged = fact.AIEngaged || sources.Conversation.AIReplyRounds > 0
		fact.HumanEscalated = sources.Conversation.HandoffAt != nil
	}
	aiResolved := false
	for i := range sources.Diagnoses {
		switch strings.ToLower(strings.TrimSpace(sources.Diagnoses[i].Status)) {
		case "resolved":
			aiResolved = true
		case "escalated":
			fact.HumanEscalated = true
		}
	}
	fact.ExpertIntervened = ticket.AcceptedAt != nil || ticket.HandledAt != nil
	fact.ExpertInterventionKnown = true
	fact.AISelfServiceResolved = fact.AIEngaged && aiResolved && !fact.HumanEscalated && !fact.ExpertIntervened

	fact.SupplierCollaborationCount = int64(len(sources.Collaborations))
	fact.SupplierInvolved = fact.SupplierCollaborationCount > 0
	fact.MeetingCount = int64(len(sources.Meetings))
	for i := range sources.Meetings {
		if sources.Meetings[i].StartedAt != nil {
			fact.VideoUsed = true
			break
		}
	}
	if !fact.VideoUsed {
		for i := range sources.Participants {
			if sources.Participants[i].JoinedAt != nil {
				fact.VideoUsed = true
				break
			}
		}
	}

	fact.ResolutionKnown = len(sources.Repairs) > 0
	allServiceMethodsKnown := len(sources.Repairs) > 0
	successfulRepair := false
	repairOutcomeKnown := false
	for i := range sources.Repairs {
		repair := sources.Repairs[i]
		method := strings.ToLower(strings.TrimSpace(repair.ServiceMethod))
		if method == "" {
			allServiceMethodsKnown = false
		}
		if method == "onsite" {
			fact.OnsiteVisitOccurred = true
		}
		if repair.RemoteResolved {
			fact.RemoteResolved = true
		}
		switch strings.ToLower(strings.TrimSpace(repair.TestResult)) {
		case "passed":
			successfulRepair = true
			repairOutcomeKnown = true
		case "failed", "partial":
			repairOutcomeKnown = true
		}
		if repair.CostHours > 0 {
			fact.WorkHours += repair.CostHours
		}
		applyRepairOutcomeMetadata(fact, repair.MetadataJSON)
	}
	fact.OnsiteVisitKnown = fact.OnsiteVisitKnown || allServiceMethodsKnown
	if fact.OnsiteVisitOccurred {
		fact.AvoidedOnsiteVisitKnown = true
		fact.AvoidedOnsiteVisit = false
	}

	fact.ReopenCount = int(sources.ReopenCount)
	fact.Reopened = fact.ReopenCount > 0 || enums.NormalizeTicketStatus(string(ticket.Status)) == enums.TicketStatusReopened
	fact.RepeatTicketCount30Days = int(sources.RepeatTicketCount30Days)
	status := enums.NormalizeTicketStatus(string(ticket.Status))
	repeatDetectionKnown := ticket.DeviceID > 0 && ticket.ResolvedAt != nil &&
		(strings.TrimSpace(ticket.FaultCode) != "" || ticket.ProductModelID > 0)
	fact.FirstTimeFixKnown = fact.ResolutionKnown && repairOutcomeKnown && repeatDetectionKnown &&
		(status == enums.TicketStatusResolved || status == enums.TicketStatusClosed)
	fact.FirstTimeFixEligible = fact.FirstTimeFixKnown
	fact.FirstTimeFix = fact.FirstTimeFixKnown && successfulRepair && !fact.Reopened && fact.RepeatTicketCount30Days == 0

	knowledgeLogUsed := make(map[int64]bool, len(sources.RetrieveLogs))
	knowledgeLogCitations := make(map[int64]int64, len(sources.RetrieveLogs))
	for i := range sources.RetrieveHits {
		hit := sources.RetrieveHits[i]
		if hit.UsedInAnswer || hit.IsCitation {
			knowledgeLogUsed[hit.RetrieveLogID] = true
		}
		if hit.IsCitation {
			knowledgeLogCitations[hit.RetrieveLogID]++
		}
	}
	fact.KnowledgeRetrieveCount = int64(len(sources.RetrieveLogs))
	fact.KnowledgeReuseKnown = ticket.ConversationID > 0
	for i := range sources.RetrieveLogs {
		log := sources.RetrieveLogs[i]
		used := knowledgeLogUsed[log.ID] || log.UsedChunkCount > 0 || log.CitationCount > 0
		if used {
			fact.KnowledgeUsedCount++
		}
		if citationCount := knowledgeLogCitations[log.ID]; citationCount > 0 {
			fact.KnowledgeCitationCount += citationCount
		} else if log.CitationCount > 0 {
			fact.KnowledgeCitationCount += int64(log.CitationCount)
		}
	}
	fact.KnowledgeReused = fact.KnowledgeUsedCount > 0
	if sources.Candidate != nil {
		fact.KnowledgeCandidateID = sources.Candidate.ID
		fact.PublishedKnowledgeEntryID = sources.Candidate.KnowledgeEntryID
	}

	fact.ResponseDurationSeconds = outcomeDurationSeconds(ticket.CreatedAt, ticket.HandledAt)
	if ticket.AssignedAt != nil {
		fact.AcceptanceDurationSeconds = outcomeDurationSeconds(*ticket.AssignedAt, ticket.AcceptedAt)
	}
	fact.ResolutionDurationSeconds = outcomeDurationSeconds(ticket.CreatedAt, ticket.ResolvedAt)
	if sources.Feedback != nil {
		fact.RatingKnown = true
		fact.CustomerRating = sources.Feedback.Rating
	}

	evidence := map[string]any{
		"diagnosisSessions":      len(sources.Diagnoses),
		"repairRecords":          len(sources.Repairs),
		"supplierCollaborations": len(sources.Collaborations),
		"meetings":               len(sources.Meetings),
		"meetingParticipants":    len(sources.Participants),
		"knowledgeRetrieveLogs":  len(sources.RetrieveLogs),
		"knowledgeRetrieveHits":  len(sources.RetrieveHits),
		"reopenEvents":           sources.ReopenCount,
		"repeatTickets30Days":    sources.RepeatTicketCount30Days,
		"avoidanceEvidenceKnown": fact.AvoidedOnsiteVisitKnown,
		"downtimeEvidenceKnown":  fact.DowntimeKnown,
	}
	evidenceJSON, err := json.Marshal(evidence)
	if err != nil {
		return nil, fmt.Errorf("marshal ticket service outcome evidence: %w", err)
	}
	fact.EvidenceJSON = string(evidenceJSON)
	fingerprint, err := serviceOutcomeFingerprint(fact)
	if err != nil {
		return nil, err
	}
	fact.SourceFingerprint = fingerprint
	return fact, nil
}

func applyRepairOutcomeMetadata(fact *models.TicketServiceOutcomeFact, raw string) {
	if fact == nil || strings.TrimSpace(raw) == "" {
		return
	}
	var metadata any
	if json.Unmarshal([]byte(raw), &metadata) != nil {
		return
	}
	if value, ok := findOutcomeMetadataValue(metadata, "avoidedonsitevisit", "avoidedtrip"); ok {
		if parsed, valid := outcomeBool(value); valid {
			fact.AvoidedOnsiteVisitKnown = true
			fact.AvoidedOnsiteVisit = parsed
		}
	}
	if value, ok := findOutcomeMetadataValue(metadata, "onsitevisitoccurred", "onsitetripoccurred"); ok {
		if parsed, valid := outcomeBool(value); valid {
			fact.OnsiteVisitKnown = true
			fact.OnsiteVisitOccurred = fact.OnsiteVisitOccurred || parsed
		}
	}
	if value, ok := findOutcomeMetadataValue(metadata, "downtimeminutes"); ok {
		if parsed, valid := outcomeInt64(value); valid && parsed >= 0 {
			fact.DowntimeKnown = true
			fact.DowntimeMinutes += parsed
			return
		}
	}
	startedValue, hasStarted := findOutcomeMetadataValue(metadata, "faultstartedat", "downtimestartedat")
	restoredValue, hasRestored := findOutcomeMetadataValue(metadata, "restoredat", "serviceconnectedat")
	if hasStarted && hasRestored {
		startedAt, startedOK := outcomeTime(startedValue)
		restoredAt, restoredOK := outcomeTime(restoredValue)
		if startedOK && restoredOK && !restoredAt.Before(startedAt) {
			fact.DowntimeKnown = true
			fact.DowntimeMinutes += int64(restoredAt.Sub(startedAt).Minutes())
		}
	}
}

func findOutcomeMetadataValue(value any, keys ...string) (any, bool) {
	targets := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		targets[normalizeOutcomeMetadataKey(key)] = struct{}{}
	}
	var find func(any) (any, bool)
	find = func(current any) (any, bool) {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				if _, ok := targets[normalizeOutcomeMetadataKey(key)]; ok {
					return child, true
				}
			}
			for _, child := range typed {
				if found, ok := find(child); ok {
					return found, true
				}
			}
		case []any:
			for _, child := range typed {
				if found, ok := find(child); ok {
					return found, true
				}
			}
		}
		return nil, false
	}
	return find(value)
}

func normalizeOutcomeMetadataKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, "-", "")
	return value
}

func outcomeBool(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		return parsed, err == nil
	default:
		return false, false
	}
}

func outcomeInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), typed >= 0
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func outcomeTime(value any) (time.Time, bool) {
	raw, ok := value.(string)
	if !ok {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	return parsed, err == nil
}

func outcomeDurationSeconds(start time.Time, end *time.Time) *int64 {
	if end == nil || end.Before(start) {
		return nil
	}
	seconds := int64(end.Sub(start).Seconds())
	return &seconds
}

func latestOutcomeSourceTime(sources *repositories.ServiceOutcomeFactSources) time.Time {
	latest := sources.Ticket.UpdatedAt
	if latest.IsZero() {
		latest = sources.Ticket.CreatedAt
	}
	consider := func(candidate time.Time) {
		if candidate.After(latest) {
			latest = candidate
		}
	}
	if sources.Conversation != nil {
		consider(sources.Conversation.UpdatedAt)
	}
	for i := range sources.Diagnoses {
		consider(sources.Diagnoses[i].UpdatedAt)
	}
	for i := range sources.Repairs {
		consider(sources.Repairs[i].UpdatedAt)
	}
	if sources.Feedback != nil {
		consider(sources.Feedback.UpdatedAt)
		consider(sources.Feedback.SubmittedAt)
	}
	for i := range sources.Collaborations {
		consider(sources.Collaborations[i].UpdatedAt)
	}
	for i := range sources.Meetings {
		consider(sources.Meetings[i].UpdatedAt)
	}
	for i := range sources.Participants {
		consider(sources.Participants[i].UpdatedAt)
	}
	for i := range sources.RetrieveLogs {
		consider(sources.RetrieveLogs[i].CreatedAt)
	}
	for i := range sources.RetrieveHits {
		consider(sources.RetrieveHits[i].CreatedAt)
	}
	if sources.Candidate != nil {
		consider(sources.Candidate.UpdatedAt)
	}
	return latest
}

func serviceOutcomeFingerprint(fact *models.TicketServiceOutcomeFact) (string, error) {
	copyValue := *fact
	copyValue.ID = 0
	copyValue.SourceFingerprint = ""
	copyValue.BuiltAt = time.Time{}
	copyValue.AuditFields = models.AuditFields{}
	raw, err := json.Marshal(copyValue)
	if err != nil {
		return "", fmt.Errorf("marshal ticket service outcome fingerprint: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
