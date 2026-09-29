package meta

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestValidateWhatsAppSignatureRejectsTampering(t *testing.T) {
	body := []byte(`{"object":"whatsapp_business_account"}`)
	mac := hmac.New(sha256.New, []byte("app-secret"))
	_, _ = mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !ValidateWhatsAppSignature("app-secret", body, signature) {
		t.Fatal("valid Meta signature was rejected")
	}
	if ValidateWhatsAppSignature("app-secret", append(body, ' '), signature) {
		t.Fatal("tampered payload was accepted")
	}
	if ValidateWhatsAppSignature("app-secret", body, "sha256=bad") {
		t.Fatal("malformed signature was accepted")
	}
}

func TestParseWhatsAppWebhookExtractsTextAndStatus(t *testing.T) {
	body := []byte(`{
  "object":"whatsapp_business_account",
  "entry":[{"id":"waba-1","changes":[{"field":"messages","value":{
    "metadata":{"display_phone_number":"15551905900","phone_number_id":"phone-1"},
    "contacts":[{"profile":{"name":"Test Customer"},"wa_id":"8613800000000"}],
    "messages":[{"from":"8613800000000","id":"wamid.inbound-1","timestamp":"1710000000","type":"text","text":{"body":"hello helpdesk"}}],
    "statuses":[{"id":"wamid.outbound-1","status":"read","timestamp":"1710000001","recipient_id":"8613800000000"}]
  }}]}]
}`)

	event, err := ParseWhatsAppWebhook(body)
	if err != nil {
		t.Fatalf("ParseWhatsAppWebhook() error = %v", err)
	}
	if len(event.Messages) != 1 {
		t.Fatalf("message count = %d, want 1", len(event.Messages))
	}
	message := event.Messages[0]
	if message.PhoneNumberID != "phone-1" || message.WABAID != "waba-1" || message.MessageID != "wamid.inbound-1" || message.From != "8613800000000" || message.ProfileName != "Test Customer" || message.Text != "hello helpdesk" {
		t.Fatalf("unexpected inbound message: %+v", message)
	}
	if len(event.Statuses) != 1 {
		t.Fatalf("status count = %d, want 1", len(event.Statuses))
	}
	status := event.Statuses[0]
	if status.PhoneNumberID != "phone-1" || status.MessageID != "wamid.outbound-1" || status.Status != "read" || status.RecipientID != "8613800000000" {
		t.Fatalf("unexpected status update: %+v", status)
	}
}

func TestParseWhatsAppWebhookExtractsSupportedMediaMessages(t *testing.T) {
	body := []byte(`{"object":"whatsapp_business_account","entry":[{"id":"waba-1","changes":[{"field":"messages","value":{"metadata":{"phone_number_id":"phone-1"},"contacts":[{"profile":{"name":"Media Customer"},"wa_id":"1"}],"messages":[
{"from":"1","id":"wamid.image","type":"image","image":{"id":"media-image","mime_type":"image/jpeg","sha256":"image-sha","caption":"damage photo"}},
{"from":"1","id":"wamid.document","type":"document","document":{"id":"media-document","mime_type":"application/pdf","sha256":"document-sha","filename":"report.pdf","caption":"service report"}},
{"from":"1","id":"wamid.audio","type":"audio","audio":{"id":"media-audio","mime_type":"audio/ogg","sha256":"audio-sha","voice":true}},
{"from":"1","id":"wamid.video","type":"video","video":{"id":"media-video","mime_type":"video/mp4","sha256":"video-sha","caption":"fault video"}},
{"from":"1","id":"wamid.location","type":"location","location":{"latitude":1,"longitude":2}}
]}}]}]}`)

	event, err := ParseWhatsAppWebhook(body)
	if err != nil {
		t.Fatalf("ParseWhatsAppWebhook() error = %v", err)
	}
	if len(event.Messages) != 4 {
		t.Fatalf("message count = %d, want 4: %+v", len(event.Messages), event.Messages)
	}
	wants := []WhatsAppInboundMessage{
		{WABAID: "waba-1", PhoneNumberID: "phone-1", MessageID: "wamid.image", From: "1", ProfileName: "Media Customer", Type: "image", MediaID: "media-image", MimeType: "image/jpeg", SHA256: "image-sha", Caption: "damage photo"},
		{WABAID: "waba-1", PhoneNumberID: "phone-1", MessageID: "wamid.document", From: "1", ProfileName: "Media Customer", Type: "document", MediaID: "media-document", MimeType: "application/pdf", SHA256: "document-sha", Filename: "report.pdf", Caption: "service report"},
		{WABAID: "waba-1", PhoneNumberID: "phone-1", MessageID: "wamid.audio", From: "1", ProfileName: "Media Customer", Type: "audio", MediaID: "media-audio", MimeType: "audio/ogg", SHA256: "audio-sha", Voice: true},
		{WABAID: "waba-1", PhoneNumberID: "phone-1", MessageID: "wamid.video", From: "1", ProfileName: "Media Customer", Type: "video", MediaID: "media-video", MimeType: "video/mp4", SHA256: "video-sha", Caption: "fault video"},
	}
	for i := range wants {
		if event.Messages[i] != wants[i] {
			t.Fatalf("message[%d] = %+v, want %+v", i, event.Messages[i], wants[i])
		}
	}
}
