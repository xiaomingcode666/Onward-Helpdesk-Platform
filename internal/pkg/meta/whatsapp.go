package meta

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

type WhatsAppWebhook struct {
	Messages []WhatsAppInboundMessage
	Statuses []WhatsAppStatusUpdate
}

type WhatsAppInboundMessage struct {
	WABAID        string
	PhoneNumberID string
	MessageID     string
	From          string
	ProfileName   string
	Type          string
	Text          string
	MediaID       string
	MimeType      string
	SHA256        string
	Filename      string
	Caption       string
	Voice         bool
}

type WhatsAppStatusUpdate struct {
	PhoneNumberID string
	MessageID     string
	RecipientID   string
	Status        string
}

type webhookEnvelope struct {
	Object string `json:"object"`
	Entry  []struct {
		ID      string `json:"id"`
		Changes []struct {
			Field string `json:"field"`
			Value struct {
				Metadata struct {
					PhoneNumberID string `json:"phone_number_id"`
				} `json:"metadata"`
				Contacts []struct {
					Profile struct {
						Name string `json:"name"`
					} `json:"profile"`
					WAID string `json:"wa_id"`
				} `json:"contacts"`
				Messages []struct {
					From string `json:"from"`
					ID   string `json:"id"`
					Type string `json:"type"`
					Text struct {
						Body string `json:"body"`
					} `json:"text"`
					Image struct {
						ID       string `json:"id"`
						MimeType string `json:"mime_type"`
						SHA256   string `json:"sha256"`
						Caption  string `json:"caption"`
					} `json:"image"`
					Document struct {
						ID       string `json:"id"`
						MimeType string `json:"mime_type"`
						SHA256   string `json:"sha256"`
						Filename string `json:"filename"`
						Caption  string `json:"caption"`
					} `json:"document"`
					Audio struct {
						ID       string `json:"id"`
						MimeType string `json:"mime_type"`
						SHA256   string `json:"sha256"`
						Voice    bool   `json:"voice"`
					} `json:"audio"`
					Video struct {
						ID       string `json:"id"`
						MimeType string `json:"mime_type"`
						SHA256   string `json:"sha256"`
						Caption  string `json:"caption"`
					} `json:"video"`
				} `json:"messages"`
				Statuses []struct {
					ID          string `json:"id"`
					Status      string `json:"status"`
					RecipientID string `json:"recipient_id"`
				} `json:"statuses"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

func ValidateWhatsAppSignature(appSecret string, body []byte, signature string) bool {
	appSecret = strings.TrimSpace(appSecret)
	signature = strings.TrimSpace(signature)
	if appSecret == "" || !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil || len(provided) != sha256.Size {
		return false
	}
	mac := hmac.New(sha256.New, []byte(appSecret))
	_, _ = mac.Write(body)
	return hmac.Equal(provided, mac.Sum(nil))
}

func ParseWhatsAppWebhook(body []byte) (*WhatsAppWebhook, error) {
	var envelope webhookEnvelope
	if len(body) == 0 || json.Unmarshal(body, &envelope) != nil {
		return nil, errors.New("invalid Meta WhatsApp webhook payload")
	}
	if envelope.Object != "whatsapp_business_account" {
		return nil, errors.New("unsupported Meta webhook object")
	}
	result := &WhatsAppWebhook{Messages: []WhatsAppInboundMessage{}, Statuses: []WhatsAppStatusUpdate{}}
	for _, entry := range envelope.Entry {
		for _, change := range entry.Changes {
			if change.Field != "messages" {
				continue
			}
			profiles := make(map[string]string, len(change.Value.Contacts))
			for _, contact := range change.Value.Contacts {
				profiles[strings.TrimSpace(contact.WAID)] = strings.TrimSpace(contact.Profile.Name)
			}
			phoneNumberID := strings.TrimSpace(change.Value.Metadata.PhoneNumberID)
			for _, message := range change.Value.Messages {
				parsed := WhatsAppInboundMessage{
					WABAID: strings.TrimSpace(entry.ID), PhoneNumberID: phoneNumberID,
					MessageID: strings.TrimSpace(message.ID), From: strings.TrimSpace(message.From),
					Type: strings.ToLower(strings.TrimSpace(message.Type)),
				}
				parsed.ProfileName = profiles[parsed.From]
				switch parsed.Type {
				case "text":
					parsed.Text = strings.TrimSpace(message.Text.Body)
					if parsed.Text == "" {
						continue
					}
				case "image":
					parsed.MediaID, parsed.MimeType, parsed.SHA256, parsed.Caption = strings.TrimSpace(message.Image.ID), strings.TrimSpace(message.Image.MimeType), strings.TrimSpace(message.Image.SHA256), strings.TrimSpace(message.Image.Caption)
				case "document":
					parsed.MediaID, parsed.MimeType, parsed.SHA256 = strings.TrimSpace(message.Document.ID), strings.TrimSpace(message.Document.MimeType), strings.TrimSpace(message.Document.SHA256)
					parsed.Filename, parsed.Caption = strings.TrimSpace(message.Document.Filename), strings.TrimSpace(message.Document.Caption)
				case "audio":
					parsed.MediaID, parsed.MimeType, parsed.SHA256, parsed.Voice = strings.TrimSpace(message.Audio.ID), strings.TrimSpace(message.Audio.MimeType), strings.TrimSpace(message.Audio.SHA256), message.Audio.Voice
				case "video":
					parsed.MediaID, parsed.MimeType, parsed.SHA256, parsed.Caption = strings.TrimSpace(message.Video.ID), strings.TrimSpace(message.Video.MimeType), strings.TrimSpace(message.Video.SHA256), strings.TrimSpace(message.Video.Caption)
				default:
					continue
				}
				if parsed.MessageID == "" || parsed.From == "" || (parsed.Type != "text" && parsed.MediaID == "") {
					continue
				}
				result.Messages = append(result.Messages, parsed)
			}
			for _, status := range change.Value.Statuses {
				result.Statuses = append(result.Statuses, WhatsAppStatusUpdate{
					PhoneNumberID: phoneNumberID, MessageID: strings.TrimSpace(status.ID),
					RecipientID: strings.TrimSpace(status.RecipientID), Status: strings.TrimSpace(status.Status),
				})
			}
		}
	}
	return result, nil
}
