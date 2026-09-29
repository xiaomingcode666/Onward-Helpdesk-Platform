package services

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/openidentity"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services/storage"

	"github.com/mlogclub/simple/sqls"
)

// WhatsAppLocalInbound is the deliberately small local-test contract. It
// mirrors the stable fields needed from a WhatsApp Cloud API text message
// without pretending to be a Meta payload.
type WhatsAppLocalInbound struct {
	EventID      string                `json:"eventId"`
	ChannelID    string                `json:"channelId"`
	TenantID     int64                 `json:"tenantId"`
	From         string                `json:"from"`
	ProfileName  string                `json:"profileName"`
	MessageID    string                `json:"messageId"`
	Text         string                `json:"text"`
	MessageType  enums.IMMessageType   `json:"messageType,omitempty"`
	Media        *WhatsAppInboundMedia `json:"media,omitempty"`
	Locale       string                `json:"locale"`
	Provider     string                `json:"-"`
	ClientPrefix string                `json:"-"`
}

type WhatsAppInboundMedia struct {
	MediaID  string                                                `json:"mediaId"`
	Filename string                                                `json:"filename,omitempty"`
	MimeType string                                                `json:"mimeType,omitempty"`
	SHA256   string                                                `json:"sha256,omitempty"`
	Caption  string                                                `json:"caption,omitempty"`
	Load     func(context.Context) (*WhatsAppMediaDownload, error) `json:"-"`
}

type WhatsAppMediaDownload struct {
	Reader   io.ReadCloser
	Filename string
	MimeType string
	FileSize int64
	SHA256   string
}

type WhatsAppLocalInboundResult struct {
	EventID        string `json:"eventId"`
	Duplicate      bool   `json:"duplicate"`
	ConversationID int64  `json:"conversationId,omitempty"`
	MessageID      int64  `json:"messageId,omitempty"`
}

var WhatsAppLocalService = newWhatsAppLocalService()

type whatsappLocalService struct{}

func newWhatsAppLocalService() *whatsappLocalService {
	return &whatsappLocalService{}
}

func (s *whatsappLocalService) ProcessInbound(ctx context.Context, input WhatsAppLocalInbound) (*WhatsAppLocalInboundResult, error) {
	if err := ValidateWhatsAppLocalInbound(input); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	payloadHash := hex.EncodeToString(sum[:])
	now := time.Now()
	provider := strings.TrimSpace(input.Provider)
	if provider == "" {
		provider = "whatsapp_local"
	}
	clientPrefix := strings.TrimSpace(input.ClientPrefix)
	if clientPrefix == "" {
		clientPrefix = "whatsapp-local:"
	}
	inbox, claimed, err := repositories.WebhookEventInboxRepository.Claim(sqls.DB(), &models.WebhookEventInbox{
		Provider:    provider,
		EventID:     strings.TrimSpace(input.EventID),
		EventType:   "message",
		PayloadHash: payloadHash,
		PayloadJSON: string(raw),
		ReceivedAt:  now,
		OccurredAt:  &now,
	}, now, 5*time.Minute)
	if err != nil {
		return nil, err
	}
	if !claimed {
		if inbox != nil && inbox.PayloadHash != "" && inbox.PayloadHash != payloadHash {
			return nil, errors.New("whatsapp local event id was reused with a different payload")
		}
		return &WhatsAppLocalInboundResult{EventID: input.EventID, Duplicate: true}, nil
	}

	fail := func(processErr error) (*WhatsAppLocalInboundResult, error) {
		_ = repositories.WebhookEventInboxRepository.MarkFailed(sqls.DB(), inbox.ID, processErr.Error(), time.Now(), time.Now().Add(30*time.Second))
		return nil, processErr
	}
	channel := ChannelService.GetEnabledWhatsAppChannelByChannelID(input.ChannelID)
	if channel == nil {
		return fail(errors.New("enabled WhatsApp channel not found"))
	}
	agent := AIAgentService.Get(channel.AIAgentID)
	if agent == nil || agent.Status != enums.StatusOk {
		return fail(errors.New("WhatsApp channel AI agent is not enabled"))
	}
	externalName, externalSource := buildWhatsAppLocalExternalUser(input)
	external := openidentity.ExternalUser{
		ExternalSource: externalSource,
		ExternalID:     strings.TrimSpace(input.From),
		ExternalName:   externalName,
	}
	conversation, err := ConversationService.CreateWithContext(external, channel.ID, agent.ID, request.CreateOrMatchConversationRequest{
		ContextTenantID: input.TenantID,
		Locale:          input.Locale,
	})
	if err != nil {
		return fail(err)
	}
	messageType := input.MessageType
	if messageType == "" {
		messageType = enums.IMMessageTypeText
	}
	content, payload := strings.TrimSpace(input.Text), ""
	var uploadedAsset *models.Asset
	if input.Media != nil {
		download, loadErr := input.Media.Load(ctx)
		if loadErr != nil {
			return fail(loadErr)
		}
		if download == nil || download.Reader == nil {
			return fail(errors.New("WhatsApp media download is empty"))
		}
		defer download.Reader.Close()
		principal := &dto.AuthPrincipal{TenantID: conversation.TenantID, Username: "meta_whatsapp", DomainType: "service_account"}
		if conversation.TenantID == 0 {
			principal.DomainType = "platform"
		}
		uploadedAsset, err = AssetService.upload(download.Reader, storage.UploadInfo{
			Prefix: "whatsapp", Source: "meta_whatsapp", Filename: download.Filename,
			FileSize: download.FileSize, MimeType: download.MimeType, Principal: principal,
		}, conversation.ID)
		if err != nil {
			return fail(err)
		}
		expectedHashes := []string{input.Media.SHA256, download.SHA256}
		for _, expected := range expectedHashes {
			normalized, hashErr := normalizeWhatsAppSHA256(expected)
			if hashErr != nil || (normalized != "" && !strings.EqualFold(normalized, uploadedAsset.SHA256)) {
				AssetService.discardConversationAsset(uploadedAsset, principal)
				if hashErr != nil {
					return fail(hashErr)
				}
				return fail(errors.New("Meta WhatsApp media SHA-256 mismatch"))
			}
		}
		payload, err = buildIMMessageAssetPayload(uploadedAsset)
		if err != nil {
			AssetService.discardConversationAsset(uploadedAsset, principal)
			return fail(err)
		}
		content = strings.TrimSpace(input.Media.Caption)
	}
	message, err := MessageService.SendCustomerMessageWithRequestID(
		conversation.ID,
		buildWhatsAppClientMessageID(clientPrefix, input.MessageID),
		messageType,
		content,
		payload,
		external,
		input.EventID,
	)
	if err != nil {
		if uploadedAsset != nil {
			principal := &dto.AuthPrincipal{TenantID: conversation.TenantID, Username: "meta_whatsapp", DomainType: "service_account"}
			if conversation.TenantID == 0 {
				principal.DomainType = "platform"
			}
			AssetService.discardConversationAsset(uploadedAsset, principal)
		}
		return fail(err)
	}
	if err := repositories.WebhookEventInboxRepository.MarkProcessed(sqls.DB(), inbox.ID, time.Now()); err != nil {
		return nil, err
	}
	return &WhatsAppLocalInboundResult{
		EventID: input.EventID, ConversationID: conversation.ID, MessageID: message.ID,
	}, nil
}

func ValidateWhatsAppLocalInbound(input WhatsAppLocalInbound) error {
	for name, value := range map[string]string{
		"eventId": input.EventID, "channelId": input.ChannelID, "from": input.From,
		"messageId": input.MessageID,
	} {
		if strings.TrimSpace(value) == "" {
			return errors.New("whatsapp local inbound " + name + " is required")
		}
	}
	messageType := input.MessageType
	if messageType == "" {
		messageType = enums.IMMessageTypeText
	}
	if messageType == enums.IMMessageTypeText {
		if strings.TrimSpace(input.Text) == "" {
			return errors.New("whatsapp local inbound text is required")
		}
	} else {
		if messageType != enums.IMMessageTypeImage && messageType != enums.IMMessageTypeAudio && messageType != enums.IMMessageTypeAttachment {
			return errors.New("whatsapp local inbound message type is unsupported")
		}
		if input.Media == nil || strings.TrimSpace(input.Media.MediaID) == "" || input.Media.Load == nil {
			return errors.New("whatsapp local inbound media is required")
		}
		if _, err := normalizeWhatsAppSHA256(input.Media.SHA256); err != nil {
			return err
		}
	}
	if len([]rune(input.EventID)) > 160 || len([]rune(input.MessageID)) > 160 || len([]rune(input.From)) > 128 {
		return errors.New("whatsapp local inbound identifier is too long")
	}
	if len([]rune(input.Text)) > 4096 {
		return errors.New("whatsapp local inbound text is too long")
	}
	return nil
}

func normalizeWhatsAppSHA256(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if decoded, err := hex.DecodeString(value); err == nil && len(decoded) == sha256.Size {
		return strings.ToLower(value), nil
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		return "", errors.New("invalid Meta WhatsApp media SHA-256")
	}
	return hex.EncodeToString(decoded), nil
}

func BuildWhatsAppLocalClientMessageID(messageID string) string {
	return buildWhatsAppClientMessageID("whatsapp-local:", messageID)
}

func buildWhatsAppClientMessageID(prefix, messageID string) string {
	return strings.TrimSpace(prefix) + strings.TrimSpace(messageID)
}

func buildWhatsAppLocalExternalUser(input WhatsAppLocalInbound) (string, enums.ExternalSource) {
	name := strings.TrimSpace(input.ProfileName)
	if name == "" {
		name = strings.TrimSpace(input.From)
	}
	return name, enums.ExternalSourceWhatsApp
}
