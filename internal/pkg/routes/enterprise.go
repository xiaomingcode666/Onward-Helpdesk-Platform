package routes

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func EnterpriseTicketWorkbenchPath(ticketID int64) string {
	if ticketID <= 0 {
		return "/enterprise/ticket-workbench"
	}
	return fmt.Sprintf("/enterprise/ticket-workbench?ticket_id=%d", ticketID)
}

func EnterpriseConversationWorkbenchPath(conversationID int64) string {
	if conversationID <= 0 {
		return "/enterprise/ticket-workbench"
	}
	return fmt.Sprintf("/enterprise/ticket-workbench?conversationId=%d", conversationID)
}

func NormalizeEnterpriseActionURL(actionURL, bizType string, bizID int64) string {
	if normalized, ok := normalizeLegacyEnterpriseTicketsURL(actionURL); ok {
		if normalized == "/enterprise/ticket-workbench" && isTicketBizType(bizType) && bizID > 0 {
			return EnterpriseTicketWorkbenchPath(bizID)
		}
		return normalized
	}
	if value := strings.TrimSpace(actionURL); value != "" {
		return value
	}
	switch strings.ToLower(strings.TrimSpace(bizType)) {
	case "ticket":
		return EnterpriseTicketWorkbenchPath(bizID)
	case "conversation":
		return EnterpriseConversationWorkbenchPath(bizID)
	default:
		return ""
	}
}

func isTicketBizType(bizType string) bool {
	return strings.ToLower(strings.TrimSpace(bizType)) == "ticket"
}

func normalizeLegacyEnterpriseTicketsURL(actionURL string) (string, bool) {
	value := strings.TrimSpace(actionURL)
	if value == "" {
		return "", false
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", false
	}
	path := strings.TrimRight(parsed.Path, "/")
	const prefix = "/enterprise/tickets/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	ticketSegment := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if ticketSegment == "" {
		return "/enterprise/ticket-workbench", true
	}
	if index := strings.Index(ticketSegment, "/"); index >= 0 {
		ticketSegment = ticketSegment[:index]
	}
	ticketID, err := strconv.ParseInt(ticketSegment, 10, 64)
	if err != nil || ticketID <= 0 {
		return "/enterprise/ticket-workbench", true
	}
	return EnterpriseTicketWorkbenchPath(ticketID), true
}
