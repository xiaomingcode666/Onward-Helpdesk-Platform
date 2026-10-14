package i18nx

import (
	"encoding/json"
	"testing"
)

func fakeLookup(values map[string]map[string]string) func(locale, key string) string {
	return func(locale, key string) string {
		if value := values[locale][key]; value != "" {
			return value
		}
		return key
	}
}

var textTestMessages = map[string]map[string]string{
	LocaleZhCN: {
		"notification.knowledgeCandidate.title": "新的知识候选待审核：{title}",
		"notification.ticketStatus.content":     "工单 {ticketNo} 状态变为 {status}",
		"enum.ticketStatus.accepted":            "已受理",
	},
	LocaleEnUS: {
		"notification.knowledgeCandidate.title": "New knowledge candidate pending review: {title}",
		"notification.ticketStatus.content":     "Ticket {ticketNo} changed to {status}",
		"enum.ticketStatus.accepted":            "Accepted",
	},
}

func TestTextRendersNamedParamsPerLocale(t *testing.T) {
	text := NewText("notification.knowledgeCandidate.title", Params{"title": "Pump noise"})
	lookup := fakeLookup(textTestMessages)

	if got := text.render(LocaleEnUS, lookup); got != "New knowledge candidate pending review: Pump noise" {
		t.Fatalf("en-US render = %q", got)
	}
	if got := text.render(LocaleZhCN, lookup); got != "新的知识候选待审核：Pump noise" {
		t.Fatalf("zh-CN render = %q", got)
	}
}

func TestTextRendersNestedTextAndScalarParams(t *testing.T) {
	text := NewText("notification.ticketStatus.content", Params{
		"ticketNo": 1024,
		"status":   NewText("enum.ticketStatus.accepted", nil),
	})

	if got := text.render(LocaleEnUS, fakeLookup(textTestMessages)); got != "Ticket 1024 changed to Accepted" {
		t.Fatalf("render = %q", got)
	}
}

func TestTextKeepsUnknownPlaceholdersAndMissingKeys(t *testing.T) {
	lookup := fakeLookup(textTestMessages)

	partial := NewText("notification.ticketStatus.content", Params{"ticketNo": "T-1"})
	if got := partial.render(LocaleEnUS, lookup); got != "Ticket T-1 changed to {status}" {
		t.Fatalf("partial render = %q", got)
	}
	missing := NewText("notification.unknown", nil)
	if got := missing.render(LocaleEnUS, lookup); got != "notification.unknown" {
		t.Fatalf("missing key render = %q", got)
	}
	if got := (Text{}).render(LocaleEnUS, lookup); got != "" {
		t.Fatalf("zero text render = %q", got)
	}
}

func TestTextJSONRoundTripPreservesNestedText(t *testing.T) {
	original := NewText("notification.ticketStatus.content", Params{
		"ticketNo": "T-1",
		"status":   NewText("enum.ticketStatus.accepted", nil),
	})

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Text
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got := decoded.render(LocaleEnUS, fakeLookup(textTestMessages)); got != "Ticket T-1 changed to Accepted" {
		t.Fatalf("decoded render = %q (json %s)", got, data)
	}
}

func TestTextDatabaseValueAndScan(t *testing.T) {
	original := NewText("enum.ticketStatus.accepted", Params{"by": "Alex"})

	value, err := original.Value()
	if err != nil {
		t.Fatalf("value: %v", err)
	}
	var scanned Text
	if err := scanned.Scan(value); err != nil {
		t.Fatalf("scan string: %v", err)
	}
	if scanned.Key != original.Key || scanned.Params["by"] != "Alex" {
		t.Fatalf("scanned = %#v", scanned)
	}

	var fromBytes Text
	if err := fromBytes.Scan([]byte(value.(string))); err != nil {
		t.Fatalf("scan bytes: %v", err)
	}
	if fromBytes.Key != original.Key {
		t.Fatalf("scanned from bytes = %#v", fromBytes)
	}

	zeroValue, err := (Text{}).Value()
	if err != nil || zeroValue != "" {
		t.Fatalf("zero value = %#v, %v", zeroValue, err)
	}
	var empty Text
	for _, input := range []any{nil, "", []byte("")} {
		if err := empty.Scan(input); err != nil || !empty.IsZero() {
			t.Fatalf("Scan(%#v) = %#v, %v", input, empty, err)
		}
	}
	if err := empty.Scan(42); err == nil {
		t.Fatal("Scan(int) succeeded, want error")
	}
}

func TestTextRenderUsesLocaleResources(t *testing.T) {
	text := NewText("error.e0273", nil)

	if got := text.Render(LocaleEnUS); got != "Directory not found." {
		t.Fatalf("Render(en-US) = %q", got)
	}
	if got := text.Render(LocaleZhCN); got != "目录不存在" {
		t.Fatalf("Render(zh-CN) = %q", got)
	}
}
