package services

import (
	"context"
	"encoding/json"
	"mime/multipart"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mlogclub/simple/sqls"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/pkg/openidentity"
	"remotehelpdesk/internal/repositories"
)

const (
	customerVoiceTranscriptMaxRunes = 4000
	customerVoiceRequestKeyMaxLen   = 64
	customerVoiceProgressSource     = "customer_voice"
)

var CustomerVoiceTicketService = &customerVoiceTicketService{}

// uploadCustomerVoiceAudio stores the original recording; replaced in tests.
var uploadCustomerVoiceAudio = func(file *multipart.FileHeader, principal *dto.AuthPrincipal) (*models.Asset, error) {
	return AssetService.UploadFile(file, "audio", principal)
}

type customerVoiceTicketService struct{}

// CustomerVoiceTicketInput is a signed-in customer's reviewed transcript plus
// the original recording. RequestKey is generated once per dialog so a
// repeated submit returns the same ticket.
type CustomerVoiceTicketInput struct {
	Transcript string
	RequestKey string
	Locale     string
	Audio      *multipart.FileHeader
}

type CustomerVoiceTicketResult struct {
	TicketID int64  `json:"ticketId"`
	TicketNo string `json:"ticketNo"`
}

type customerVoiceProgressMetadata struct {
	Source       string `json:"source"`
	AudioAssetID string `json:"audioAssetId"`
}

// Submit creates a standalone ticket (not tied to any conversation) for the
// signed-in customer in tenantID, attaching the original recording.
func (s *customerVoiceTicketService) Submit(ctx context.Context, external openidentity.ExternalUser, tenantID int64, input CustomerVoiceTicketInput) (*CustomerVoiceTicketResult, error) {
	scope, err := CustomerPortalService.resolveScope(external)
	if err != nil {
		return nil, err
	}
	// The ticket must land in a tenant the customer's portal lists, otherwise
	// the customer could not see what they just submitted.
	if tenantID <= 0 || !slices.Contains(scope.tenantIDs, tenantID) {
		return nil, errorsx.ForbiddenI18n("error.e0222")
	}
	transcript := strings.TrimSpace(input.Transcript)
	if transcript == "" {
		return nil, errorsx.InvalidParamI18n("voiceTicket.transcriptRequired")
	}
	if utf8.RuneCountInString(transcript) > customerVoiceTranscriptMaxRunes {
		return nil, errorsx.InvalidParamI18n("voiceTicket.transcriptTooLong", customerVoiceTranscriptMaxRunes)
	}
	requestKey := strings.TrimSpace(input.RequestKey)
	if requestKey == "" || len(requestKey) > customerVoiceRequestKeyMaxLen {
		return nil, errorsx.InvalidParam("voice ticket request key is required")
	}
	idempotencyKey := "customer-voice:" + requestKey
	if existing := repositories.TicketRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("customer_id", scope.customer.ID).
		Eq("idempotency_key", idempotencyKey)); existing != nil {
		return &CustomerVoiceTicketResult{TicketID: existing.ID, TicketNo: existing.TicketNo}, nil
	}
	if input.Audio == nil || input.Audio.Size <= 0 {
		return nil, errorsx.InvalidParamI18n("voiceTicket.audioRequired")
	}
	if input.Audio.Size > maxVoiceTranscriptionBytes {
		return nil, errorsx.InvalidParam("audio file is too large")
	}

	operator := customerVoiceTicketOperator(external, tenantID)
	asset, err := uploadCustomerVoiceAudio(input.Audio, operator)
	if err != nil {
		return nil, err
	}
	title := buildVoiceTicketTitle(ai.WithCapabilityScope(ctx, ai.CapabilityScope{
		TenantID:        tenantID,
		CredentialChain: []string{dsl.ModelCredentialScopeTenantDefault},
	}), transcript)
	ticket, err := TicketService.CreateTicket(request.CreateTicketRequest{
		IdempotencyKey: idempotencyKey,
		Title:          title,
		Description:    transcript,
		Source:         string(enums.TicketSourceCustomerVoice),
		Channel:        enums.ChannelTypeWeb,
		CustomerID:     scope.customer.ID,
		TenantID:       tenantID,
	}, operator)
	if err != nil {
		return nil, err
	}
	metadata, _ := json.Marshal(customerVoiceProgressMetadata{Source: customerVoiceProgressSource, AudioAssetID: asset.AssetID})
	if err := repositories.TicketProgressRepository.Create(sqls.DB(), &models.TicketProgress{
		TenantID:          ticket.TenantID,
		TicketID:          ticket.ID,
		EventType:         enums.TicketProgressEventProgress,
		ContentText:       i18nx.NewText("voiceTicket.recordingAttached", nil),
		VisibleToCustomer: true,
		MetadataJSON:      string(metadata),
		CreatedAt:         time.Now(),
	}); err != nil {
		return nil, err
	}
	return &CustomerVoiceTicketResult{TicketID: ticket.ID, TicketNo: ticket.TicketNo}, nil
}

func customerVoiceTicketOperator(external openidentity.ExternalUser, tenantID int64) *dto.AuthPrincipal {
	name := strings.TrimSpace(external.ExternalName)
	if name == "" {
		name = "customer"
	}
	return &dto.AuthPrincipal{
		Username:    name,
		Nickname:    name,
		TenantID:    tenantID,
		DomainType:  "service_account",
		SubjectType: "service_account",
	}
}
