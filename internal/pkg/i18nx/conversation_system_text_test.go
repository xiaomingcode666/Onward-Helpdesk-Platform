package i18nx

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConversationSystemTextUsesEventMetadata(t *testing.T) {
	for _, device := range []bool{false, true} {
		payload, _ := json.Marshal(map[string]any{"eventType": "ticket_created", "ticketId": 42, "hasDeviceContext": device})
		text := ConversationSystemTextFromPayload(string(payload))
		if text == nil || text.Summary.Render(LocaleEnUS) != "Service ticket #42 created" {
			t.Fatalf("missing ticket descriptor: %#v", text)
		}
		if !strings.Contains(text.Content.Render(LocaleEnUS), "#42") || !strings.Contains(text.Content.Render(LocaleZhCN), "已生成服务工单") {
			t.Fatal("one event must render in both languages")
		}
		if strings.Contains(text.Content.Render(LocaleEnUS), "device context") != device {
			t.Fatal("device context variant lost")
		}
	}
}

func TestConversationSystemTextPreservesBusinessValues(t *testing.T) {
	payload := `{"eventType":"supplier_collaboration_invited","partnerCompanyName":"公司 {ticketNo}","productModuleName":"模块 A"}`
	text := ConversationSystemTextFromPayload(payload)
	if text == nil || text.Content.Render(LocaleEnUS) != "Supplier collaboration invited: 公司 {ticketNo} · 模块 A" {
		t.Fatalf("business values were changed: %#v", text)
	}
	for _, locale := range SupportedLocales {
		if got := text.Summary.Render(locale); got == text.Summary.Key || strings.Contains(got, "{company}") {
			t.Fatalf("missing translation for %s: %s", locale, got)
		}
	}
}

func TestConversationSystemTextMissingCompanyUsesLocalizedSystemLabel(t *testing.T) {
	text := ConversationSystemTextFromPayload(`{"eventType":"supplier_collaboration_accepted"}`)
	if text == nil || text.Summary.Render(LocaleEnUS) != "Supplier joined the collaboration conversation" || text.Summary.Render(LocaleZhCN) != "供应商已加入协作会话" {
		t.Fatalf("supplier fallback must follow the viewer locale: %#v", text)
	}
}

func TestConversationSystemTextRejectsUnknownAndMalformedEvents(t *testing.T) {
	for _, payload := range []string{"", "[]", "null", "{}", `{"eventType":"custom"}`, `{"eventType":"ticket_created"}`, `{"eventType":"supplier_collaboration_invited","partnerCompanyName":42}`, `{"systemText":{"content":{"key":"error.e0245"}}}`} {
		if got := ConversationSystemTextFromPayload(payload); got != nil {
			t.Fatalf("unexpected descriptor for %s: %#v", payload, got)
		}
	}
}
