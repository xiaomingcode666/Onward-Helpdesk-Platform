package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/pkg/openidentity"
	"remotehelpdesk/internal/repositories"
)

const (
	customerVoiceTranscriptMaxRunes = 4000
	customerVoiceProgressSource     = "customer_voice"
)

var CustomerVoiceTicketService = &customerVoiceTicketService{}

type customerVoiceTicketService struct{}

type CustomerVoiceTicketResult struct {
	TicketID int64  `json:"ticketId"`
	TicketNo string `json:"ticketNo"`
	Created  bool   `json:"created"`
}

type customerVoiceProgressMetadata struct {
	Source       string `json:"source"`
	AudioAssetID string `json:"audioAssetId"`
}

// Submit turns a customer's reviewed voice transcript into a ticket on the
// conversation, or appends it to the conversation's open ticket. Resubmitting
// the same recording returns the original result without side effects.
func (s *customerVoiceTicketService) Submit(ctx context.Context, req request.CustomerVoiceTicketRequest, external openidentity.ExternalUser, requestID string) (*CustomerVoiceTicketResult, error) {
	conversation := ConversationService.Get(req.ConversationID)
	if conversation == nil {
		return nil, errorsx.InvalidParamI18n("error.e0116")
	}
	if !ConversationService.IsCustomerConversationOwner(conversation, external) {
		return nil, errorsx.ForbiddenI18n("error.e0222")
	}
	assetID := strings.TrimSpace(req.AudioAssetID)
	if !s.assetBelongsToConversation(sqls.DB(), conversation, assetID) {
		return nil, errorsx.ForbiddenI18n("error.e0222")
	}
	if conversation.Status == enums.IMConversationStatusClosed {
		return nil, errorsx.InvalidParamI18n("voiceTicket.conversationClosed")
	}
	transcript := strings.TrimSpace(req.Transcript)
	if transcript == "" {
		return nil, errorsx.InvalidParamI18n("voiceTicket.transcriptRequired")
	}
	if utf8.RuneCountInString(transcript) > customerVoiceTranscriptMaxRunes {
		return nil, errorsx.InvalidParamI18n("voiceTicket.transcriptTooLong", customerVoiceTranscriptMaxRunes)
	}

	// The LLM title call can take seconds, so it runs before taking the
	// conversation ticket lock; inside the lock only the writes happen.
	title := ""
	if s.needsNewTicket(sqls.DB(), conversation, assetID) {
		title = buildVoiceTicketTitle(ai.WithCapabilityScope(ctx, ai.CapabilityScope{
			TenantID:        conversation.TenantID,
			ProductID:       conversation.ProductID,
			CredentialChain: []string{dsl.ModelCredentialScopeTenantDefault},
		}), transcript)
	}

	var result *CustomerVoiceTicketResult
	_, err := TicketService.withConversationTicketCreateLock(conversation.TenantID, conversation.ID, func() (*models.Ticket, error) {
		var submitErr error
		result, submitErr = s.submitLocked(conversation, transcript, title, assetID, req.Locale, requestID)
		return nil, submitErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func customerVoiceIdempotencyKey(assetID string) string {
	return "customer-voice:" + assetID
}

func customerVoiceProgressMetadataJSON(assetID string) string {
	metadata, _ := json.Marshal(customerVoiceProgressMetadata{Source: customerVoiceProgressSource, AudioAssetID: assetID})
	return string(metadata)
}

// previousResult returns the outcome of an earlier submission of the same
// recording, if any.
func (s *customerVoiceTicketService) previousResult(db *gorm.DB, conversation *models.Conversation, assetID string) *CustomerVoiceTicketResult {
	if existing := repositories.TicketRepository.FindOne(db, sqls.NewCnd().
		Eq("tenant_id", conversation.TenantID).
		Eq("idempotency_key", customerVoiceIdempotencyKey(assetID))); existing != nil {
		return &CustomerVoiceTicketResult{TicketID: existing.ID, TicketNo: existing.TicketNo, Created: true}
	}
	var appended models.TicketProgress
	if db.Where("tenant_id = ? AND metadata_json = ?", conversation.TenantID, customerVoiceProgressMetadataJSON(assetID)).First(&appended).Error == nil {
		if ticket := TicketService.Get(appended.TicketID); ticket != nil {
			return &CustomerVoiceTicketResult{TicketID: ticket.ID, TicketNo: ticket.TicketNo, Created: false}
		}
	}
	return nil
}

func (s *customerVoiceTicketService) needsNewTicket(db *gorm.DB, conversation *models.Conversation, assetID string) bool {
	return s.previousResult(db, conversation, assetID) == nil &&
		TicketService.findActiveConversationTicket(db, conversation.TenantID, conversation.ID) == nil
}

func (s *customerVoiceTicketService) submitLocked(conversation *models.Conversation, transcript, title, assetID, locale, requestID string) (*CustomerVoiceTicketResult, error) {
	db := sqls.DB()
	if previous := s.previousResult(db, conversation, assetID); previous != nil {
		return previous, nil
	}

	result := &CustomerVoiceTicketResult{}
	if active := TicketService.findActiveConversationTicket(db, conversation.TenantID, conversation.ID); active != nil {
		progress := &models.TicketProgress{
			TenantID:          active.TenantID,
			TicketID:          active.ID,
			EventType:         enums.TicketProgressEventProgress,
			Content:           transcript,
			VisibleToCustomer: true,
			MetadataJSON:      customerVoiceProgressMetadataJSON(assetID),
			CreatedAt:         time.Now(),
		}
		if err := repositories.TicketProgressRepository.Create(db, progress); err != nil {
			return nil, err
		}
		result.TicketID, result.TicketNo = active.ID, active.TicketNo
	} else {
		if title == "" {
			// The open ticket closed between the pre-check and the lock.
			title = voiceTicketTitleFallback(transcript)
		}
		ticket, err := TicketService.createFromConversationLocked(request.CreateTicketFromConversationRequest{
			ConversationID: conversation.ID,
			TenantID:       conversation.TenantID,
			Title:          title,
			Description:    transcript,
			IdempotencyKey: customerVoiceIdempotencyKey(assetID),
		}, handoffTicketOperator(conversation, s.ticketAgent(conversation)), conversation, conversation.TenantID)
		if err != nil {
			return nil, err
		}
		result.TicketID, result.TicketNo, result.Created = ticket.ID, ticket.TicketNo, true
	}

	key := "voiceTicket.appended"
	if result.Created {
		key = "voiceTicket.created"
	}
	if _, err := MessageService.SendSystemMessageWithRequestID(conversation.ID, "voice-ticket:"+assetID, i18nx.Getf(locale, key, result.TicketNo), "", requestID); err != nil {
		// The ticket exists; a missing notice must not make the customer retry.
		slog.Warn("voice ticket system message failed", "conversation_id", conversation.ID, "ticket_id", result.TicketID, "error", err)
	}
	return result, nil
}

func (s *customerVoiceTicketService) assetBelongsToConversation(db *gorm.DB, conversation *models.Conversation, assetID string) bool {
	if assetID == "" || db == nil {
		return false
	}
	var count int64
	if err := db.Model(&models.Asset{}).
		Where("asset_id = ? AND tenant_id = ? AND conversation_id = ?", assetID, conversation.TenantID, conversation.ID).
		Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

func (s *customerVoiceTicketService) ticketAgent(conversation *models.Conversation) models.AIAgent {
	if conversation.AIAgentID > 0 {
		if agent := AIAgentService.Get(conversation.AIAgentID); agent != nil {
			return *agent
		}
	}
	if agent := ConversationService.ticketOnlyConversationAgent(resolvedConversationCreateContext{
		tenantID:  conversation.TenantID,
		productID: conversation.ProductID,
	}); agent != nil {
		return *agent
	}
	return models.AIAgent{}
}
