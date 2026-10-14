package services

import (
	"encoding/json"
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/i18nx"
)

// 站内信保存模板参数后，按读者当前语言重新渲染；系统文本参数随语言切换，用户内容原样保留。
func TestNotificationLocalizedContentFollowsReaderLocale(t *testing.T) {
	req := request.CreateNotificationRequest{
		TitleText:         i18nx.NewText("notificationText.sla.title", i18nx.Params{"ticketNo": "TK-1"}),
		ContentText:       i18nx.NewText("notificationText.sla.content", i18nx.Params{"target": i18nx.NewText("notificationText.sla.target.firstResponse", nil), "targetMinutes": 30, "actualMinutes": 22}),
		TemplateVariables: map[string]string{"TicketNo": "TK-1", "TicketTitle": "泵体异响"},
	}
	item := &models.Notification{
		TenantID:       1,
		TemplateCode:   "sla_warning",
		Title:          canonicalText(req.TitleText),
		Content:        canonicalText(req.ContentText),
		TemplateParams: buildNotificationTemplateParams(req, canonicalText(req.TitleText), canonicalText(req.ContentText)),
	}
	title, content := NotificationTemplateService.LocalizedContent(item, i18nx.LocaleEnUS)
	if title != "Ticket TK-1 is approaching its SLA" {
		t.Fatalf("en-US title = %q", title)
	}
	if !strings.HasPrefix(content, "First response target: 30 min. Time used: 22 min.") || strings.ContainsAny(content, "首目标") {
		t.Fatalf("en-US content = %q", content)
	}
	title, content = NotificationTemplateService.LocalizedContent(item, i18nx.LocaleZhCN)
	if title != "工单 TK-1 即将超时" || !strings.HasPrefix(content, "首次响应目标 30 分钟，当前已用 22 分钟。") {
		t.Fatalf("zh-CN notification = %q / %q", title, content)
	}

	legacy := &models.Notification{TemplateCode: "sla_warning", Title: "旧标题", Content: "旧内容"}
	if title, content := NotificationTemplateService.LocalizedContent(legacy, i18nx.LocaleEnUS); title != "旧标题" || content != "旧内容" {
		t.Fatalf("legacy notification should keep stored text, got %q / %q", title, content)
	}
}

func TestJoinTextLinesRendersEachLineInLocale(t *testing.T) {
	text := JoinTextLines(i18nx.NewText("notificationText.ticketAssigned.line", i18nx.Params{"ticketNo": "TK-2"}), "客户录入标题", i18nx.NewText("notificationText.ticketAssigned.reason", i18nx.Params{"reason": SystemReasonParam(dispatchReasonAutomatic)}))
	if got := text.Render(i18nx.LocaleEnUS); got != "Ticket TK-2 has been assigned to you\n客户录入标题\nReason: automatic assignment" {
		t.Fatalf("joined lines = %q", got)
	}
	long := JoinTextLines("1", "2", "3", "4", "5")
	if got := long.Render(i18nx.LocaleEnUS); got != "1\n2\n3\n4\n5" {
		t.Fatalf("five lines = %q", got)
	}
}

// 通知请求经队列序列化后，系统文本必须完整保留，否则消费端会因标题为空而拒绝创建。
func TestNotificationRequestTextSurvivesQueueSerialization(t *testing.T) {
	req := request.CreateNotificationRequest{
		TitleText:     i18nx.NewText("notificationText.ticketAssigned.title", nil),
		ContentText:   JoinTextLines(i18nx.NewText("notificationText.ticketAssigned.line", i18nx.Params{"ticketNo": "TK-3"}), "标题"),
		TextVariables: SystemReasonTextVariable("Reason", dispatchReasonAutomatic),
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var decoded request.CreateNotificationRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if got := decoded.TitleText.Render(i18nx.LocaleEnUS); got != "Ticket assignment" {
		t.Fatalf("decoded title = %q", got)
	}
	if got := decoded.ContentText.Render(i18nx.LocaleEnUS); got != "Ticket TK-3 has been assigned to you\n标题" {
		t.Fatalf("decoded content = %q", got)
	}
	if got := decoded.TextVariables["Reason"].Render(i18nx.LocaleEnUS); got != "automatic assignment" {
		t.Fatalf("decoded reason = %q", got)
	}
}
