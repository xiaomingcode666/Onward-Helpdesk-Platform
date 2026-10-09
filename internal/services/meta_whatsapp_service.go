package services

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	metapkg "remotehelpdesk/internal/pkg/meta"
	"remotehelpdesk/internal/pkg/projectconfig"

	"github.com/mlogclub/simple/sqls"
)

var MetaWhatsAppService = newMetaWhatsAppService()

type metaWhatsAppService struct{}

type MetaWhatsAppWebhookResult struct {
	ProcessedMessages int `json:"processedMessages"`
	DuplicateMessages int `json:"duplicateMessages"`
	ProcessedStatuses int `json:"processedStatuses"`
}

func newMetaWhatsAppService() *metaWhatsAppService {
	return &metaWhatsAppService{}
}

func (s *metaWhatsAppService) VerifyToken(provided string) bool {
	provided = strings.TrimSpace(provided)
	if provided == "" {
		return false
	}
	channels := ChannelService.Find(sqls.NewCnd().
		Eq("channel_type", enums.ChannelTypeWhatsApp).
		Asc("id"))
	for i := range channels {
		if channels[i].Status == enums.StatusDeleted {
			continue
		}
		cfg, err := ChannelService.ParseWhatsAppChannelConfig(channels[i].ConfigJSON)
		if err != nil || !ChannelSecretService.Available(&channels[i], ChannelSecretWebhookVerifyToken, cfg.WebhookVerifyTokenSecretRef) {
			continue
		}
		secret, err := ChannelSecretService.Resolve(&channels[i], ChannelSecretWebhookVerifyToken, cfg.WebhookVerifyTokenSecretRef)
		if err == nil && constantTimeStringEqual(provided, secret) {
			return true
		}
	}
	return false
}

func (s *metaWhatsAppService) ProcessWebhook(ctx context.Context, body []byte, signature string) (*MetaWhatsAppWebhookResult, error) {
	event, err := metapkg.ParseWhatsAppWebhook(body)
	if err != nil {
		return nil, err
	}
	phoneNumberID := webhookPhoneNumberID(event)
	channel := ChannelService.GetEnabledWhatsAppChannelByPhoneNumberID(phoneNumberID)
	if channel == nil {
		return nil, errors.New("enabled Meta WhatsApp channel not found")
	}
	cfg, err := ChannelService.ParseWhatsAppChannelConfig(channel.ConfigJSON)
	if err != nil {
		return nil, errors.New("invalid Meta WhatsApp channel configuration")
	}
	appSecret, err := ChannelSecretService.Resolve(channel, ChannelSecretMetaAppSecret, cfg.MetaAppSecretSecretRef)
	if err != nil {
		return nil, errors.New("Meta WhatsApp app secret is unavailable")
	}
	if !metapkg.ValidateWhatsAppSignature(appSecret, body, signature) {
		return nil, errors.New("invalid Meta WhatsApp signature")
	}
	agent := AIAgentService.Get(channel.AIAgentID)
	if agent == nil || agent.Status != enums.StatusOk {
		return nil, errors.New("Meta WhatsApp channel AI agent is not enabled")
	}
	result := &MetaWhatsAppWebhookResult{}
	for _, message := range event.Messages {
		if message.PhoneNumberID != cfg.MetaPhoneNumberID || message.WABAID != cfg.MetaBusinessAccountID {
			return nil, errors.New("Meta WhatsApp webhook routing identifiers do not match channel")
		}
		messageType, err := metaWhatsAppInboundMessageType(message.Type)
		if err != nil {
			return nil, err
		}
		var media *WhatsAppInboundMedia
		if messageType != enums.IMMessageTypeText {
			accessToken, secretErr := ChannelSecretService.Resolve(channel, ChannelSecretMetaAccessToken, cfg.MetaAccessTokenSecretRef)
			if secretErr != nil {
				return nil, errors.New("Meta WhatsApp access token is unavailable")
			}
			media = &WhatsAppInboundMedia{
				MediaID: message.MediaID, Filename: metaWhatsAppInboundFilename(message), MimeType: message.MimeType,
				SHA256: message.SHA256, Caption: message.Caption,
			}
			media.Load = func(loadCtx context.Context) (*WhatsAppMediaDownload, error) {
				return s.downloadMedia(loadCtx, message.MediaID, accessToken, media.Filename, message.MimeType)
			}
		}
		processed, processErr := WhatsAppLocalService.ProcessInbound(ctx, WhatsAppLocalInbound{
			EventID: message.MessageID, ChannelID: channel.ChannelID, TenantID: agent.TenantID,
			From: message.From, ProfileName: message.ProfileName, MessageID: message.MessageID,
			Text: message.Text, MessageType: messageType, Media: media,
			Provider: "meta_whatsapp", ClientPrefix: "meta-whatsapp:",
		})
		if processErr != nil {
			return nil, processErr
		}
		if processed.Duplicate {
			result.DuplicateMessages++
		} else {
			result.ProcessedMessages++
		}
	}
	for _, status := range event.Statuses {
		if status.PhoneNumberID != cfg.MetaPhoneNumberID {
			return nil, errors.New("Meta WhatsApp status routing identifier does not match channel")
		}
		if ChannelMessageOutboxService.ApplyWhatsAppProviderStatus(status.MessageID, status.Status) {
			result.ProcessedStatuses++
		}
	}
	return result, nil
}

func metaWhatsAppInboundMessageType(value string) (enums.IMMessageType, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "text":
		return enums.IMMessageTypeText, nil
	case "image":
		return enums.IMMessageTypeImage, nil
	case "audio":
		return enums.IMMessageTypeAudio, nil
	case "document", "video":
		return enums.IMMessageTypeAttachment, nil
	default:
		return "", fmt.Errorf("unsupported Meta WhatsApp message type %q", value)
	}
}

func metaWhatsAppInboundFilename(message metapkg.WhatsAppInboundMessage) string {
	if name := filepath.Base(strings.TrimSpace(message.Filename)); name != "" && name != "." {
		return name
	}
	extension := ".bin"
	switch strings.ToLower(strings.TrimSpace(message.MimeType)) {
	case "image/jpeg":
		extension = ".jpg"
	case "image/png":
		extension = ".png"
	case "image/gif":
		extension = ".gif"
	case "image/webp":
		extension = ".webp"
	case "audio/ogg":
		extension = ".ogg"
	case "audio/mpeg":
		extension = ".mp3"
	case "video/mp4":
		extension = ".mp4"
	case "application/pdf":
		extension = ".pdf"
	}
	return "whatsapp-" + strings.ToLower(strings.TrimSpace(message.Type)) + extension
}

func (s *metaWhatsAppService) downloadMedia(ctx context.Context, mediaID, accessToken, filename, webhookMimeType string) (*WhatsAppMediaDownload, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	baseURL := metaWhatsAppGraphAPIBaseURL()
	version := metaWhatsAppGraphAPIVersion()
	mediaID, accessToken = strings.TrimSpace(mediaID), strings.TrimSpace(accessToken)
	if mediaID == "" || accessToken == "" {
		return nil, errors.New("Meta WhatsApp media download configuration is incomplete")
	}
	metadataURL := strings.TrimRight(baseURL, "/") + "/" + url.PathEscape(version) + "/" + url.PathEscape(mediaID)
	metadataReq, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return nil, err
	}
	metadataReq.Header.Set("Authorization", "Bearer "+accessToken)
	client := &http.Client{
		Timeout: metaWhatsAppHTTPTimeout,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			if !allowedMetaWhatsAppMediaURL(req.URL.String(), baseURL) {
				return errors.New("Meta WhatsApp media redirect is not trusted")
			}
			req.Header.Set("Authorization", "Bearer "+accessToken)
			return nil
		},
	}
	metadataResp, err := client.Do(metadataReq)
	if err != nil {
		return nil, err
	}
	defer metadataResp.Body.Close()
	metadataRaw, err := io.ReadAll(io.LimitReader(metadataResp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if metadataResp.StatusCode < http.StatusOK || metadataResp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Meta Graph media metadata API returned %d", metadataResp.StatusCode)
	}
	var metadata struct {
		URL      string `json:"url"`
		MimeType string `json:"mime_type"`
		SHA256   string `json:"sha256"`
		FileSize int64  `json:"file_size"`
	}
	if err := json.Unmarshal(metadataRaw, &metadata); err != nil {
		return nil, err
	}
	if !allowedMetaWhatsAppMediaURL(metadata.URL, baseURL) {
		return nil, errors.New("Meta WhatsApp media URL is not trusted")
	}
	downloadReq, err := http.NewRequestWithContext(ctx, http.MethodGet, metadata.URL, nil)
	if err != nil {
		return nil, err
	}
	downloadReq.Header.Set("Authorization", "Bearer "+accessToken)
	downloadResp, err := client.Do(downloadReq)
	if err != nil {
		return nil, err
	}
	if downloadResp.StatusCode < http.StatusOK || downloadResp.StatusCode >= http.StatusMultipleChoices {
		downloadResp.Body.Close()
		return nil, fmt.Errorf("Meta media download returned %d", downloadResp.StatusCode)
	}
	mimeType := strings.TrimSpace(metadata.MimeType)
	if mimeType == "" {
		mimeType = strings.TrimSpace(webhookMimeType)
	}
	return &WhatsAppMediaDownload{
		Reader: downloadResp.Body, Filename: filename, MimeType: mimeType,
		FileSize: metadata.FileSize, SHA256: strings.TrimSpace(metadata.SHA256),
	}, nil
}

func allowedMetaWhatsAppMediaURL(rawURL, apiBaseURL string) bool {
	mediaURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || mediaURL.Hostname() == "" || mediaURL.User != nil {
		return false
	}
	apiURL, _ := url.Parse(strings.TrimSpace(apiBaseURL))
	if apiURL != nil && strings.EqualFold(mediaURL.Host, apiURL.Host) && mediaURL.Scheme == apiURL.Scheme {
		return true
	}
	if mediaURL.Scheme != "https" {
		return false
	}
	host := strings.ToLower(mediaURL.Hostname())
	return host == "facebook.com" || strings.HasSuffix(host, ".facebook.com") ||
		host == "fbcdn.net" || strings.HasSuffix(host, ".fbcdn.net") ||
		host == "fbsbx.com" || strings.HasSuffix(host, ".fbsbx.com")
}

func webhookPhoneNumberID(event *metapkg.WhatsAppWebhook) string {
	if event == nil {
		return ""
	}
	for _, message := range event.Messages {
		if value := strings.TrimSpace(message.PhoneNumberID); value != "" {
			return value
		}
	}
	for _, status := range event.Statuses {
		if value := strings.TrimSpace(status.PhoneNumberID); value != "" {
			return value
		}
	}
	return ""
}

func resolveWhatsAppChannelSecret(channel *models.Channel, ref string) (string, error) {
	if channel == nil {
		return "", errors.New("WhatsApp channel is required")
	}
	agent := AIAgentService.Get(channel.AIAgentID)
	if agent == nil {
		return "", errors.New("WhatsApp channel AI agent is unavailable")
	}
	raw, err := projectconfig.ReadSecret(os.Getenv("RHD_PROJECT_SECRET_DIR"), agent.TenantID, projectconfig.Environment(), strings.TrimSpace(ref))
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func constantTimeStringEqual(left, right string) bool {
	leftBytes, rightBytes := []byte(left), []byte(right)
	if len(leftBytes) != len(rightBytes) {
		return false
	}
	return subtle.ConstantTimeCompare(leftBytes, rightBytes) == 1
}
