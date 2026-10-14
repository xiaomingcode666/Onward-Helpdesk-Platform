package services

import (
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/i18nx"
)

// 动态拼接的资源键（状态、派单失败原因等）必须覆盖全部取值，否则页面会显示资源键本身。
func TestDynamicSystemTextKeysExist(t *testing.T) {
	keys := []string{}
	for _, status := range append(append([]enums.TicketStatus{}, enums.TicketStatusValues...), enums.AfterSalesTicketStatusValues...) {
		keys = append(keys, ticketStatusText(status).Key)
	}
	for _, status := range []string{"", "new", "acknowledged", "in_triage", "assigned", "waiting", "restored", "resolved", "closure_pending", "closed", "cancelled"} {
		keys = append(keys, caseStatusText(status).Key)
	}
	for status := range caseProgressPublicStatuses {
		keys = append(keys, "caseProgress.public."+status)
	}
	for _, prefix := range []string{"ticketProgress.takeover", "ticketProgress.takeoverExpiredAcceptance", "ticketProgress.takeoverUnansweredCustomer", "ticketProgress.poolSelfAssign", "ticketProgress.returnedToPool"} {
		keys = append(keys, prefix+".product", prefix+".support")
	}
	for _, key := range systemReasonTexts {
		keys = append(keys, key)
	}
	for _, messageType := range []enums.IMMessageType{enums.IMMessageTypeImage, enums.IMMessageTypeAudio, enums.IMMessageTypeAttachment} {
		keys = append(keys, messageSummaryText(messageType, "").Key, messageSummaryText(messageType, "a.png").Key)
	}
	for _, status := range []string{SupportStatusReady, SupportStatusRestricted, SupportStatusUnknown} {
		keys = append(keys, supportStatusText(status).Key)
	}
	for _, key := range keys {
		for _, locale := range i18nx.SupportedLocales {
			if got := i18nx.NewText(key, nil).Render(locale); got == key || strings.TrimSpace(got) == "" {
				t.Errorf("%s missing in %s", key, locale)
			}
		}
	}
}

// 系统进度同时写入中文规范值和可翻译文本，切换语言后按当前语言展示。
func TestTicketProgressRendersInRequestLocale(t *testing.T) {
	content := withReasonText(i18nx.NewText("ticketProgress.autoAssignedWithConversation", nil), dispatchReasonAutomatic)
	progress := models.TicketProgress{Content: canonicalText(content), ContentText: content}
	if progress.Content != "工单随会话自动分配，原因：自动分配" {
		t.Fatalf("canonical content = %q", progress.Content)
	}
	if got := progress.DisplayContent(i18nx.LocaleEnUS); got != "Ticket assigned automatically with the conversation. Reason: automatic assignment" {
		t.Fatalf("en-US content = %q", got)
	}
	userReason := withReasonText(i18nx.NewText("ticketProgress.reopened", nil), "客户反馈仍有异响")
	if got := userReason.Render(i18nx.LocaleEnUS); got != "Ticket reopened. Reason: 客户反馈仍有异响" {
		t.Fatalf("user reason should stay as entered, got %q", got)
	}
	legacy := models.TicketProgress{Content: "客户自己写的进度"}
	if got := legacy.DisplayContent(i18nx.LocaleEnUS); got != "客户自己写的进度" {
		t.Fatalf("user-entered progress should stay as entered, got %q", got)
	}
}

func TestDispatchFailureReasonTextKeepsUnknownCodes(t *testing.T) {
	if text, ok := dispatchFailureReasonText("no_matched_profile").(i18nx.Text); !ok || text.Render(i18nx.LocaleEnUS) != "No matched profile" {
		t.Fatalf("known code should be translated, got %#v", dispatchFailureReasonText("no_matched_profile"))
	}
	if got := dispatchFailureReasonText("custom_code"); got != "custom_code" {
		t.Fatalf("unknown code should stay as is, got %#v", got)
	}
}
