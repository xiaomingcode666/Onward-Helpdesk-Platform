package services

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var CustomerTicketActionService = newCustomerTicketActionService()

type customerTicketActionService struct{}

func newCustomerTicketActionService() *customerTicketActionService {
	return &customerTicketActionService{}
}

func (s *customerTicketActionService) SubmitFeedback(ticket *models.Ticket, customerUserID int64, req request.SubmitTicketFeedbackRequest, operator *dto.AuthPrincipal) (*models.TicketFeedback, error) {
	if ticket == nil || operator == nil {
		return nil, errorsx.Unauthorized("customer ticket is not available")
	}
	if req.Rating < 1 || req.Rating > 5 {
		return nil, errorsx.InvalidParam("rating must be between 1 and 5")
	}
	status := enums.NormalizeTicketStatus(string(ticket.Status))
	if !customerCanRate(string(status)) {
		return nil, errorsx.InvalidParam("ticket cannot be rated in its current status")
	}
	tagsJSON, err := json.Marshal(req.Tags)
	if err != nil {
		return nil, errorsx.InvalidParam("invalid feedback tags")
	}
	comment := strings.TrimSpace(req.Comment)
	var saved *models.TicketFeedback
	changed := false
	err = sqls.DB().Transaction(func(tx *gorm.DB) error {
		var current models.Ticket
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, ticket.ID).Error; err != nil {
			return errorsx.Unauthorized("customer ticket is not available")
		}
		if current.TenantID != ticket.TenantID || current.TenantID != operator.TenantID {
			return errorsx.Unauthorized("customer ticket is not available")
		}

		var existing models.TicketFeedback
		findResult := tx.Where("tenant_id = ? AND ticket_id = ?", current.TenantID, current.ID).
			Order("submitted_at DESC").Order("id DESC").First(&existing)
		if findResult.Error != nil && findResult.Error != gorm.ErrRecordNotFound {
			return findResult.Error
		}
		hasExisting := findResult.Error == nil
		_, _, canRate := customerTicketActionFlags(&current, func() *models.TicketFeedback {
			if hasExisting {
				return &existing
			}
			return nil
		}())
		if !canRate {
			if hasExisting && existing.Rating == req.Rating && existing.TagsJSON == string(tagsJSON) && existing.Comment == comment {
				saved = &existing
				return nil
			}
			return errorsx.InvalidParam("feedback has already been submitted for this resolution")
		}

		now := time.Now()
		existing = models.TicketFeedback{
			TenantID:       current.TenantID,
			TicketID:       current.ID,
			CustomerUserID: customerUserID,
			Rating:         req.Rating,
			TagsJSON:       string(tagsJSON),
			Comment:        comment,
			Status:         "submitted",
			SubmittedAt:    now,
			AuditFields:    utils.BuildAuditFields(operator),
		}
		if err := tx.Create(&existing).Error; err != nil {
			return err
		}
		eventID := "tenant:" + strconv.FormatInt(current.TenantID, 10) + ":customer_rating.created:" + strconv.FormatInt(existing.ID, 10)
		if _, err := eventbus.EnqueueTx(tx, eventbus.DurableEvent{
			TenantID:       current.TenantID,
			IdempotencyKey: eventID,
			EventType:      events.FaultStatsEventCustomerRating,
			Payload: events.CustomerRatingCreatedEvent{
				EventID:    eventID,
				FeedbackID: existing.ID,
				TenantID:   current.TenantID,
				TicketID:   current.ID,
				Score:      req.Rating,
				OperatorID: operator.UserID,
			},
			Source:      "customer_ticket_action_service",
			AggregateID: strconv.FormatInt(existing.ID, 10),
			ActorID:     strconv.FormatInt(operator.UserID, 10),
			ActorType:   "customer",
			CreatedAt:   now,
		}); err != nil {
			return err
		}
		if err := KnowledgeCandidateScoringService.RescoreTicketCandidatesDB(tx, current.TenantID, current.ID); err != nil {
			return err
		}
		if err := quarantineReassessmentKnowledgeEntriesTx(tx, current.TenantID, current.ID, now, operator); err != nil {
			return err
		}
		changed = true
		saved = &existing
		return nil
	})
	if err != nil {
		return nil, err
	}
	if changed {
		recordCustomerTicketAudit("ticket.feedback_submitted", ticket, operator, map[string]any{
			"feedbackId": saved.ID,
			"rating":     saved.Rating,
			"tagCount":   len(feedbackTags(saved.TagsJSON)),
		})
		// Best-effort immediate UI update; the durable event retries the same
		// idempotent message if the process exits or this call fails.
		_ = s.PublishFeedbackConversationEvent(ticket, saved)
		eventbus.WakeDefaultOutboxPublisher()
		KnowledgeIndexSyncService.ReconcileEntryTasks(20)
	}
	return saved, nil
}

func quarantineReassessmentKnowledgeEntriesTx(tx *gorm.DB, tenantID, ticketID int64, now time.Time, operator *dto.AuthPrincipal) error {
	var candidates []models.KnowledgeCandidate
	if err := tx.Where("tenant_id = ? AND ticket_id = ? AND requires_reassessment = ? AND knowledge_entry_id > 0 AND status <> ?",
		tenantID, ticketID, true, enums.StatusDeleted).Find(&candidates).Error; err != nil {
		return err
	}
	for i := range candidates {
		updates := map[string]any{
			"review_status":    "deprecated",
			"status":           enums.StatusDisabled,
			"index_status":     enums.KnowledgeDocumentIndexStatusPending,
			"index_error":      "customer feedback requires reassessment",
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
			"updated_at":       now,
		}
		entryType, actualID, knowledgeBaseID := resolveCandidateKnowledgeEntryForQuarantine(tx, &candidates[i])
		switch entryType {
		case "document":
			if err := repositories.KnowledgeDocumentRepository.Updates(tx, actualID, updates); err != nil {
				return err
			}
		case "faq":
			if err := repositories.KnowledgeFAQRepository.Updates(tx, actualID, updates); err != nil {
				return err
			}
		default:
			continue
		}
		if err := EnterpriseKnowledgeService.syncEntryLinkPublishStatusTx(tx, tenantID, knowledgeBaseID, actualID, "deprecated"); err != nil {
			return err
		}
	}
	return nil
}

func resolveCandidateKnowledgeEntryForQuarantine(tx *gorm.DB, candidate *models.KnowledgeCandidate) (string, int64, int64) {
	if tx == nil || candidate == nil {
		return "", 0, 0
	}
	entryType, actualID, err := decodeEnterpriseKnowledgeID(candidate.KnowledgeEntryID)
	if err == nil {
		switch entryType {
		case "document":
			if document := repositories.KnowledgeDocumentRepository.Get(tx, actualID); document != nil && document.TenantID == candidate.TenantID && document.Status != enums.StatusDeleted {
				return entryType, actualID, document.KnowledgeBaseID
			}
		case "faq":
			if faq := repositories.KnowledgeFAQRepository.Get(tx, actualID); faq != nil && faq.TenantID == candidate.TenantID && faq.Status != enums.StatusDeleted {
				return entryType, actualID, faq.KnowledgeBaseID
			}
		}
	}
	var document models.KnowledgeDocument
	if result := tx.Where("tenant_id = ? AND source_type IN ? AND source_reference_id = ? AND status <> ?",
		candidate.TenantID, []string{"knowledge_candidate", "knowledge_entry"}, candidate.ID, enums.StatusDeleted).
		Order("id ASC").First(&document); result.Error == nil {
		return "document", document.ID, document.KnowledgeBaseID
	}
	return "", 0, 0
}

func (s *customerTicketActionService) PublishFeedbackConversationEvent(ticket *models.Ticket, feedback *models.TicketFeedback) error {
	if ticket == nil || feedback == nil || ticket.ConversationID <= 0 {
		return nil
	}
	payloadJSON, err := json.Marshal(map[string]any{
		"eventType":  "ticket_feedback_submitted",
		"source":     "customer_feedback",
		"ticketId":   ticket.ID,
		"feedbackId": feedback.ID,
		"rating":     feedback.Rating,
		"tags":       feedbackTags(feedback.TagsJSON),
		"comment":    feedback.Comment,
	})
	if err != nil {
		return err
	}
	content := "客户已提交服务评价：" + strconv.Itoa(feedback.Rating) + "/5"
	if strings.TrimSpace(feedback.Comment) != "" {
		content += " · " + strings.TrimSpace(feedback.Comment)
	}
	_, err = MessageService.SendSystemLifecycleMessageWithRequestID(
		ticket.ConversationID,
		"ticket_feedback_submitted_"+strconv.FormatInt(feedback.ID, 10),
		content,
		string(payloadJSON),
		"",
	)
	return err
}

func feedbackTags(tagsJSON string) []string {
	var tags []string
	_ = json.Unmarshal([]byte(tagsJSON), &tags)
	return tags
}

func (s *customerTicketActionService) ConfirmResolved(ticket *models.Ticket, operator *dto.AuthPrincipal) error {
	if ticket == nil || operator == nil {
		return errorsx.Unauthorized("customer ticket is not available")
	}
	status := enums.NormalizeTicketStatus(string(ticket.Status))
	if status == enums.TicketStatusClosed {
		return nil
	}
	if !customerCanEndTicket(string(status)) {
		return errorsx.InvalidParam("ticket cannot be ended in its current status")
	}
	if err := TicketLifecycleService.Close(ticket.ID, "客户主动结束工单", operator); err != nil {
		return err
	}
	if status != enums.TicketStatusClosed {
		updated := repositories.TicketRepository.Get(sqls.DB(), ticket.ID)
		if updated == nil {
			updated = ticket
		}
		action := "ticket.customer_ended"
		if status == enums.TicketStatusResolved || status == enums.TicketStatusPendingCustomerConfirm {
			action = "ticket.customer_confirmed"
		}
		recordCustomerTicketAudit(action, updated, operator, map[string]any{
			"fromStatus": status,
			"toStatus":   enums.NormalizeTicketStatus(string(updated.Status)),
			"reason":     "customer_initiated",
		})
	}
	return nil
}

func (s *customerTicketActionService) Reopen(ticket *models.Ticket, reason string, operator *dto.AuthPrincipal) error {
	if ticket == nil || operator == nil {
		return errorsx.Unauthorized("customer ticket is not available")
	}
	status := enums.NormalizeTicketStatus(string(ticket.Status))
	if status != enums.TicketStatusResolved && status != enums.TicketStatusPendingCustomerConfirm && status != enums.TicketStatusClosed && status != enums.TicketStatusReopened {
		return errorsx.InvalidParam("ticket cannot be reopened in current status")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errorsx.InvalidParam("reopen reason is required")
	}
	return TicketLifecycleService.Reopen(ticket.ID, reason, operator)
}

func customerTicketActionFlags(ticket *models.Ticket, feedback *models.TicketFeedback) (canConfirm, canReopen, canRate bool) {
	if ticket == nil {
		return false, false, false
	}
	status := enums.NormalizeTicketStatus(string(ticket.Status))
	canConfirm = customerCanEndTicket(string(status))
	canReopen = status == enums.TicketStatusResolved || status == enums.TicketStatusPendingCustomerConfirm || status == enums.TicketStatusClosed
	ratingWindowOpen := customerCanRate(string(status))
	feedbackIsFromPreviousResolution := feedback != nil && ticket.ResolvedAt != nil && feedback.SubmittedAt.Before(*ticket.ResolvedAt)
	canRate = ratingWindowOpen && (feedback == nil || feedbackIsFromPreviousResolution)
	return canConfirm, canReopen, canRate
}

func customerCanEndTicket(status string) bool {
	switch enums.NormalizeTicketStatus(status) {
	case enums.TicketStatusPending, enums.TicketStatusAccepted, enums.TicketStatusAssigned,
		enums.TicketStatusInProgress, enums.TicketStatusProcessing, enums.TicketStatusEscalated, enums.TicketStatusWaitingCustomer,
		enums.TicketStatusResolved, enums.TicketStatusPendingCustomerConfirm, enums.TicketStatusSupplierSupport,
		enums.TicketStatusVideoSupport, enums.TicketStatusReopened:
		return true
	default:
		return false
	}
}

func customerCanRate(status string) bool {
	if customerCanEndTicket(status) {
		return true
	}
	return enums.NormalizeTicketStatus(status) == enums.TicketStatusClosed
}

func recordCustomerTicketAudit(action string, ticket *models.Ticket, operator *dto.AuthPrincipal, after map[string]any) {
	if ticket == nil || operator == nil || action == "" {
		return
	}
	if after == nil {
		after = make(map[string]any)
	}
	after["ticketId"] = ticket.ID
	if ticket.ConversationID > 0 {
		after["conversationId"] = ticket.ConversationID
	}
	_ = AuditService.RecordAudit(context.Background(), RecordAuditInput{
		TenantID:     ticket.TenantID,
		ActorID:      customerAuditActorID(operator),
		ActorType:    "customer",
		Domain:       "ticket",
		ResourceType: "ticket",
		ResourceID:   strconv.FormatInt(ticket.ID, 10),
		Action:       action,
		AfterState:   after,
		RiskLevel:    models.RiskLevelLow,
	})
}

func customerAuditActorID(operator *dto.AuthPrincipal) string {
	switch {
	case operator.UserID > 0:
		return strconv.FormatInt(operator.UserID, 10)
	case operator.CustomerUserID > 0:
		return strconv.FormatInt(operator.CustomerUserID, 10)
	case operator.SubjectID > 0:
		return strconv.FormatInt(operator.SubjectID, 10)
	case operator.SessionID > 0:
		return "session:" + strconv.FormatInt(operator.SessionID, 10)
	default:
		return ""
	}
}
