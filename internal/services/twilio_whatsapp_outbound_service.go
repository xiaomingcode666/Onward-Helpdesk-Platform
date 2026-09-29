package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/twilio"
)

const (
	twilioWhatsAppOutboxBatchSize = 20
	twilioWhatsAppOutboxMaxRetry  = 6
)

var TwilioWhatsAppOutboundService = newTwilioWhatsAppOutboundService()

type twilioWhatsAppOutboundService struct{}

func newTwilioWhatsAppOutboundService() *twilioWhatsAppOutboundService {
	return &twilioWhatsAppOutboundService{}
}

func (s *twilioWhatsAppOutboundService) DispatchPendingOutbox() int {
	cfg := twilio.CurrentWhatsAppConfig()
	if !cfg.Enabled || cfg.AccountSID == "" || cfg.AuthToken == "" || cfg.From == "" {
		return 0
	}
	items := ChannelMessageOutboxService.ListPending(enums.ChannelTypeWhatsApp, twilioWhatsAppOutboxBatchSize)
	successCount := 0
	for i := range items {
		if err := s.processOutbox(items[i].ID, cfg); err != nil {
			slog.Warn("process Twilio WhatsApp outbox failed", "outbox_id", items[i].ID, "error", err)
			continue
		}
		successCount++
	}
	return successCount
}

func (s *twilioWhatsAppOutboundService) processOutbox(outboxID int64, cfg twilio.WhatsAppConfig) error {
	outbox := ChannelMessageOutboxService.Get(outboxID)
	if outbox == nil || outbox.ChannelType != enums.ChannelTypeWhatsApp || outbox.SendStatus == string(enums.ChannelMessageOutboxStatusSent) {
		return nil
	}
	if outbox.NextRetryAt != nil && outbox.NextRetryAt.After(time.Now()) {
		return nil
	}
	message := MessageService.Get(outbox.MessageID)
	conversation := ConversationService.Get(outbox.ConversationID)
	if message == nil || conversation == nil {
		return s.markFailed(outbox, "平台消息或会话不存在")
	}
	channel := ChannelService.Get(conversation.ChannelID)
	if channel != nil {
		channelCfg, parseErr := ChannelService.ParseWhatsAppChannelConfig(channel.ConfigJSON)
		if parseErr == nil && channelCfg.MetaPhoneNumberID != "" {
			return nil
		}
	}
	if err := ChannelMessageOutboxService.Updates(outbox.ID, map[string]any{
		"send_status": string(enums.ChannelMessageOutboxStatusSending),
		"updated_at":  time.Now(),
	}); err != nil {
		return err
	}
	identity := ConversationService.GetConversationExternalIdentity(conversation)
	if identity == nil || identity.ExternalSource != enums.ExternalSourceWhatsApp {
		return s.markFailed(outbox, "WhatsApp 客户身份不存在")
	}
	form, err := buildTwilioWhatsAppMessageForm(cfg.From, identity.ExternalID, strings.TrimSpace(message.Content))
	if err != nil {
		return s.markFailed(outbox, err.Error())
	}
	if _, err := s.sendMessage(cfg, form); err != nil {
		return s.markFailed(outbox, err.Error())
	}
	now := time.Now()
	return ChannelMessageOutboxService.Updates(outbox.ID, map[string]any{
		"send_status": string(enums.ChannelMessageOutboxStatusSent),
		"sent_at":     now,
		"last_error":  "",
		"updated_at":  now,
	})
}

func buildTwilioWhatsAppMessageForm(from, to, body string) (url.Values, error) {
	from = normalizeTwilioWhatsAppAddress(from)
	to = normalizeTwilioWhatsAppAddress(to)
	body = strings.TrimSpace(body)
	if from == "" || to == "" || body == "" {
		return nil, errors.New("Twilio WhatsApp message requires from, to and body")
	}
	return url.Values{"From": {from}, "To": {to}, "Body": {body}}, nil
}

func normalizeTwilioWhatsAppAddress(value string) string {
	value = twilio.NormalizeWhatsAppAddress(value)
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "+") {
		value = "+" + value
	}
	return "whatsapp:" + value
}

func (s *twilioWhatsAppOutboundService) sendMessage(cfg twilio.WhatsAppConfig, form url.Values) (string, error) {
	endpoint := strings.TrimRight(cfg.APIBaseURL, "/") + "/2010-04-01/Accounts/" + url.PathEscape(cfg.AccountSID) + "/Messages.json"
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(cfg.AccountSID, cfg.AuthToken)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("Twilio API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var result struct {
		SID string `json:"sid"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	if strings.TrimSpace(result.SID) == "" {
		return "", errors.New("Twilio API response did not contain a message SID")
	}
	return result.SID, nil
}

func (s *twilioWhatsAppOutboundService) markFailed(outbox *models.ChannelMessageOutbox, message string) error {
	if outbox == nil {
		return nil
	}
	retryCount := outbox.RetryCount + 1
	var retryAt *time.Time
	if retryCount < twilioWhatsAppOutboxMaxRetry {
		delay := 30 * time.Second
		if retryCount >= 2 {
			delay = time.Duration(1<<(retryCount-2)) * time.Minute
		}
		next := time.Now().Add(delay)
		retryAt = &next
	}
	return ChannelMessageOutboxService.Updates(outbox.ID, map[string]any{
		"send_status":   string(enums.ChannelMessageOutboxStatusFailed),
		"retry_count":   retryCount,
		"next_retry_at": retryAt,
		"last_error":    strings.TrimSpace(message),
		"updated_at":    time.Now(),
	})
}
