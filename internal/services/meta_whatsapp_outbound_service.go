package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
)

const (
	metaWhatsAppHTTPTimeout     = 15 * time.Second
	metaWhatsAppOutboxBatchSize = 20
	metaWhatsAppOutboxMaxRetry  = 6
)

var MetaWhatsAppOutboundService = newMetaWhatsAppOutboundService()

type metaWhatsAppOutboundService struct{}

type metaWhatsAppRuntimeConfig struct {
	PhoneNumberID string
	AccessToken   string
	APIBaseURL    string
	APIVersion    string
}

func newMetaWhatsAppOutboundService() *metaWhatsAppOutboundService {
	return &metaWhatsAppOutboundService{}
}

func (s *metaWhatsAppOutboundService) DispatchPendingOutbox() int {
	items := ChannelMessageOutboxService.ListPending(enums.ChannelTypeWhatsApp, metaWhatsAppOutboxBatchSize)
	successCount := 0
	for i := range items {
		handled, err := s.processOutbox(items[i].ID)
		if err != nil {
			slog.Warn("process Meta WhatsApp outbox failed", "outbox_id", items[i].ID, "error", err)
			continue
		}
		if handled {
			successCount++
		}
	}
	return successCount
}

func (s *metaWhatsAppOutboundService) processOutbox(outboxID int64) (bool, error) {
	outbox := ChannelMessageOutboxService.Get(outboxID)
	if outbox == nil || outbox.ChannelType != enums.ChannelTypeWhatsApp || outbox.SendStatus == string(enums.ChannelMessageOutboxStatusSent) {
		return false, nil
	}
	if outbox.NextRetryAt != nil && outbox.NextRetryAt.After(time.Now()) {
		return false, nil
	}
	message := MessageService.Get(outbox.MessageID)
	conversation := ConversationService.Get(outbox.ConversationID)
	if message == nil || conversation == nil {
		return true, s.markFailed(outbox, "平台消息或会话不存在")
	}
	channel := ChannelService.Get(conversation.ChannelID)
	if channel == nil || channel.ChannelType != enums.ChannelTypeWhatsApp || channel.Status != enums.StatusOk {
		return false, nil
	}
	cfg, err := ChannelService.ParseWhatsAppChannelConfig(channel.ConfigJSON)
	if err != nil || cfg.MetaPhoneNumberID == "" || !ChannelSecretService.Available(channel, ChannelSecretMetaAccessToken, cfg.MetaAccessTokenSecretRef) {
		return false, nil
	}
	accessToken, err := ChannelSecretService.Resolve(channel, ChannelSecretMetaAccessToken, cfg.MetaAccessTokenSecretRef)
	if err != nil {
		return true, s.markFailed(outbox, "Meta WhatsApp access token is unavailable")
	}
	identity := ConversationService.GetConversationExternalIdentity(conversation)
	if identity == nil || identity.ExternalSource != enums.ExternalSourceWhatsApp {
		return true, s.markFailed(outbox, "WhatsApp 客户身份不存在")
	}
	if err := ChannelMessageOutboxService.Updates(outbox.ID, map[string]any{
		"send_status": string(enums.ChannelMessageOutboxStatusSending), "updated_at": time.Now(),
	}); err != nil {
		return true, err
	}
	runtimeCfg := metaWhatsAppRuntimeConfig{
		PhoneNumberID: cfg.MetaPhoneNumberID,
		AccessToken:   accessToken,
		APIBaseURL:    metaWhatsAppGraphAPIBaseURL(),
		APIVersion:    metaWhatsAppGraphAPIVersion(),
	}
	payload, err := s.buildOutboundPayload(runtimeCfg, conversation, message, identity.ExternalID)
	if err != nil {
		return true, s.markFailed(outbox, err.Error())
	}
	messageID, err := s.sendMessage(runtimeCfg, payload)
	if err != nil {
		return true, s.markFailed(outbox, err.Error())
	}
	now := time.Now()
	return true, ChannelMessageOutboxService.Updates(outbox.ID, map[string]any{
		"send_status": string(enums.ChannelMessageOutboxStatusSent), "sent_at": now,
		"external_message_id": messageID, "provider_status": "sent", "last_error": "", "updated_at": now,
	})
}

func (s *metaWhatsAppOutboundService) buildOutboundPayload(cfg metaWhatsAppRuntimeConfig, conversation *models.Conversation, message *models.Message, recipient string) ([]byte, error) {
	if message == nil || conversation == nil {
		return nil, errors.New("Meta WhatsApp message and conversation are required")
	}
	if message.MessageType == enums.IMMessageTypeText || message.MessageType == enums.IMMessageTypeHTML {
		return buildMetaWhatsAppTextPayload(recipient, message.Content)
	}
	if message.MessageType != enums.IMMessageTypeImage && message.MessageType != enums.IMMessageTypeAudio && message.MessageType != enums.IMMessageTypeAttachment {
		return nil, fmt.Errorf("unsupported Meta WhatsApp message type %q", message.MessageType)
	}
	assetPayload, err := parseIMMessageAssetPayload(message.Payload)
	if err != nil {
		return nil, err
	}
	asset := AssetService.GetByAssetID(assetPayload.AssetID)
	if err := validateConversationAsset(asset, conversation.ID, message.MessageType); err != nil {
		return nil, err
	}
	reader, err := AssetService.OpenReader(asset)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	mediaID, err := s.uploadMedia(cfg, reader, asset.Filename, asset.MimeType)
	if err != nil {
		return nil, err
	}
	caption := strings.TrimSpace(message.Content)
	if caption == strings.TrimSpace(asset.Filename) {
		caption = ""
	}
	return buildMetaWhatsAppMediaPayload(recipient, mediaID, asset.MimeType, asset.Filename, caption)
}

func buildMetaWhatsAppTextPayload(to, body string) ([]byte, error) {
	to = normalizeWhatsAppRecipient(to)
	body = strings.TrimSpace(body)
	if to == "" || body == "" {
		return nil, errors.New("Meta WhatsApp message requires recipient and body")
	}
	return json.Marshal(map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                to,
		"type":              "text",
		"text": map[string]any{
			"preview_url": false,
			"body":        body,
		},
	})
}

func buildMetaWhatsAppMediaPayload(to, mediaID, mimeType, filename, caption string) ([]byte, error) {
	to = normalizeWhatsAppRecipient(to)
	mediaID = strings.TrimSpace(mediaID)
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	filename = filepath.Base(strings.TrimSpace(filename))
	caption = strings.TrimSpace(caption)
	if to == "" || mediaID == "" || mimeType == "" {
		return nil, errors.New("Meta WhatsApp media message requires recipient, media ID, and MIME type")
	}
	messageType := "document"
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		messageType = "image"
	case strings.HasPrefix(mimeType, "audio/"):
		messageType = "audio"
	case strings.HasPrefix(mimeType, "video/"):
		messageType = "video"
	}
	media := map[string]any{"id": mediaID}
	if messageType == "document" && filename != "" && filename != "." {
		media["filename"] = filename
	}
	if caption != "" && messageType != "audio" {
		media["caption"] = caption
	}
	return json.Marshal(map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                to,
		"type":              messageType,
		messageType:         media,
	})
}

func normalizeWhatsAppRecipient(value string) string {
	var out strings.Builder
	for _, char := range strings.TrimSpace(value) {
		if char >= '0' && char <= '9' {
			out.WriteRune(char)
		}
	}
	return out.String()
}

func metaWhatsAppGraphAPIBaseURL() string {
	if value := strings.TrimRight(strings.TrimSpace(os.Getenv("RHD_META_GRAPH_API_BASE_URL")), "/"); value != "" {
		return value
	}
	return "https://graph.facebook.com"
}

func metaWhatsAppGraphAPIVersion() string {
	if value := strings.Trim(strings.TrimSpace(os.Getenv("RHD_META_GRAPH_API_VERSION")), "/"); value != "" {
		return value
	}
	return "v25.0"
}

func (s *metaWhatsAppOutboundService) sendMessage(cfg metaWhatsAppRuntimeConfig, payload []byte) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.APIBaseURL), "/")
	version := strings.Trim(strings.TrimSpace(cfg.APIVersion), "/")
	phoneNumberID := strings.TrimSpace(cfg.PhoneNumberID)
	accessToken := strings.TrimSpace(cfg.AccessToken)
	if baseURL == "" || version == "" || phoneNumberID == "" || accessToken == "" || len(payload) == 0 {
		return "", errors.New("Meta WhatsApp runtime configuration is incomplete")
	}
	endpoint := baseURL + "/" + url.PathEscape(version) + "/" + url.PathEscape(phoneNumberID) + "/messages"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: metaWhatsAppHTTPTimeout}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("Meta Graph API returned %d", resp.StatusCode)
	}
	var result struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	if len(result.Messages) == 0 || strings.TrimSpace(result.Messages[0].ID) == "" {
		return "", errors.New("Meta Graph API response did not contain a message ID")
	}
	return strings.TrimSpace(result.Messages[0].ID), nil
}

func (s *metaWhatsAppOutboundService) uploadMedia(cfg metaWhatsAppRuntimeConfig, reader io.Reader, filename, mimeType string) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.APIBaseURL), "/")
	version := strings.Trim(strings.TrimSpace(cfg.APIVersion), "/")
	phoneNumberID := strings.TrimSpace(cfg.PhoneNumberID)
	accessToken := strings.TrimSpace(cfg.AccessToken)
	filename = filepath.Base(strings.TrimSpace(filename))
	mimeType = strings.TrimSpace(mimeType)
	if baseURL == "" || version == "" || phoneNumberID == "" || accessToken == "" || reader == nil || filename == "" || filename == "." || mimeType == "" {
		return "", errors.New("Meta WhatsApp media upload configuration is incomplete")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("messaging_product", "whatsapp"); err != nil {
		return "", err
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filename))
	header.Set("Content-Type", mimeType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, reader); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	endpoint := baseURL + "/" + url.PathEscape(version) + "/" + url.PathEscape(phoneNumberID) + "/media"
	req, err := http.NewRequest(http.MethodPost, endpoint, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := (&http.Client{Timeout: metaWhatsAppHTTPTimeout}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("Meta Graph media API returned %d", resp.StatusCode)
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	if strings.TrimSpace(result.ID) == "" {
		return "", errors.New("Meta Graph media API response did not contain a media ID")
	}
	return strings.TrimSpace(result.ID), nil
}

func (s *metaWhatsAppOutboundService) markFailed(outbox *models.ChannelMessageOutbox, message string) error {
	if outbox == nil {
		return nil
	}
	retryCount := outbox.RetryCount + 1
	var retryAt *time.Time
	if retryCount < metaWhatsAppOutboxMaxRetry {
		delay := 30 * time.Second
		if retryCount >= 2 {
			delay = time.Duration(1<<(retryCount-2)) * time.Minute
		}
		next := time.Now().Add(delay)
		retryAt = &next
	}
	return ChannelMessageOutboxService.Updates(outbox.ID, map[string]any{
		"send_status": string(enums.ChannelMessageOutboxStatusFailed), "retry_count": retryCount,
		"next_retry_at": retryAt, "last_error": strings.TrimSpace(message), "updated_at": time.Now(),
	})
}
