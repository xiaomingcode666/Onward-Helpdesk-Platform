package i18nx

import (
	"encoding/json"
	"strconv"
	"strings"
)

// ConversationSystemText is stored alongside event metadata in Message.Payload.
// The rendered Content remains a compatibility value, never the translation source.
type ConversationSystemText struct {
	Content Text `json:"content"`
	Summary Text `json:"summary"`
}

// ConversationSystemTextFromPayload also handles historical events without a
// systemText field. Only known event types and their business metadata are used;
// arbitrary translation keys supplied in the payload are never trusted.
func ConversationSystemTextFromPayload(payload string) *ConversationSystemText {
	var event map[string]json.RawMessage
	if json.Unmarshal([]byte(payload), &event) != nil || event == nil {
		return nil
	}
	stringValue := func(name string) string {
		var value string
		_ = json.Unmarshal(event[name], &value)
		return strings.TrimSpace(value)
	}
	var content, summary Text
	switch stringValue("eventType") {
	case "ticket_created":
		ticketNo := stringValue("ticketNo")
		if ticketNo == "" {
			var id int64
			if json.Unmarshal(event["ticketId"], &id) != nil || id <= 0 {
				return nil
			}
			ticketNo = "#" + strconv.FormatInt(id, 10)
		}
		key := "ticketCreated"
		var device bool
		_ = json.Unmarshal(event["hasDeviceContext"], &device)
		if device {
			key = "ticketCreatedWithDevice"
		}
		params := Params{"ticketNo": ticketNo}
		content = NewText("conversationSystem."+key, params)
		summary = NewText("conversationSystem.ticketCreatedSummary", params)
	case "supplier_collaboration_invited", "supplier_collaboration_accepted":
		company := stringValue("partnerCompanyName")
		if company == "" {
			if stringValue("eventType") == "supplier_collaboration_accepted" {
				return &ConversationSystemText{
					Content: NewText("conversationSystem.supplierJoinedGeneric", nil),
					Summary: NewText("conversationSystem.supplierJoinedSummaryGeneric", nil),
				}
			}
			return nil
		}
		params := Params{"company": company}
		if stringValue("eventType") == "supplier_collaboration_accepted" {
			content = NewText("conversationSystem.supplierJoined", params)
			summary = NewText("conversationSystem.supplierJoinedSummary", params)
		} else {
			key := "supplierInvited"
			if module := stringValue("productModuleName"); module != "" {
				key = "supplierInvitedWithModule"
				params["module"] = module
			}
			content = NewText("conversationSystem."+key, params)
			summary = content
		}
	default:
		return nil
	}
	return &ConversationSystemText{Content: content, Summary: summary}
}

// WithConversationSystemText preserves event fields while enriching a trusted
// system event for clients that switch languages without fetching it again.
func WithConversationSystemText(payload string) (string, *ConversationSystemText) {
	text := ConversationSystemTextFromPayload(payload)
	if text == nil {
		return payload, nil
	}
	var event map[string]json.RawMessage
	if json.Unmarshal([]byte(payload), &event) != nil {
		return payload, nil
	}
	event["systemText"], _ = json.Marshal(text)
	encoded, err := json.Marshal(event)
	if err != nil {
		return payload, nil
	}
	return string(encoded), text
}
