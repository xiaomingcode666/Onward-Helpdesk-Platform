package services_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupNotificationTemplateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(
		&models.User{},
		&models.Notification{},
		&models.NotificationTemplate{},
		&models.DeliveryLog{},
		&models.NotificationDeliveryAttempt{},
	); err != nil {
		t.Fatalf("migrate notification template models: %v", err)
	}
	sqls.SetDB(db)
	return db
}

func seedNotificationTemplateUser(t *testing.T, db *gorm.DB, tenantID, userID int64, locale string) {
	t.Helper()
	if err := db.Create(&models.User{
		ID:       userID,
		Username: fmt.Sprintf("template-user-%d", userID),
		Nickname: fmt.Sprintf("User %d", userID),
		Locale:   locale,
		Status:   enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func newNotificationTemplateCase(notificationType, title, content string, recipientUserID int64) *models.Notification {
	return &models.Notification{
		TenantID:         1,
		RecipientUserID:  recipientUserID,
		Title:            title,
		Content:          content,
		NotificationType: notificationType,
		Channels:         "in_app",
		Status:           int(enums.StatusOk),
		CreatedAt:        time.Now(),
	}
}

func TestNotificationTemplateServiceUsesOnlyApprovedTemplate(t *testing.T) {
	db := setupNotificationTemplateTestDB(t)
	seedNotificationTemplateUser(t, db, 1, 501, "zh-CN")

	created, err := services.NotificationTemplateService.Create(1, 9, request.SaveNotificationTemplateRequest{
		Code:            "ticket_assigned",
		Name:            "工单分配提醒",
		Channel:         "in_app",
		Language:        "zh-CN",
		TitleTemplate:   "工单 {{TicketNo}} 已分配",
		ContentTemplate: "请处理 {{TicketTitle}}",
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	if created.ApprovalStatus != services.NotificationTemplateStatusDraft {
		t.Fatalf("new template must start as draft, got %q", created.ApprovalStatus)
	}

	variables := map[string]string{"TicketNo": "TK-9", "TicketTitle": "打印机故障"}
	draft := newNotificationTemplateCase("ticket_assigned", "原文标题", "原文内容", 501)
	// 只有一个草稿模板等于“没有可用的已批准模板”，此时不允许直接发原文。
	if err := services.NotificationTemplateService.ApplyToNotification(draft, 501, variables); err == nil {
		t.Fatal("expected missing approved template to block sending")
	} else if !strings.Contains(err.Error(), "通知缺少已批准的模板") {
		t.Fatalf("unexpected error: %v", err)
	}
	if draft.TemplateID != 0 || draft.Title != "原文标题" {
		t.Fatalf("draft template must not be used for sending: %+v", draft)
	}
	var blockedAttempts []models.NotificationDeliveryAttempt
	if err := db.Find(&blockedAttempts).Error; err != nil {
		t.Fatalf("load attempts: %v", err)
	}
	if len(blockedAttempts) != 1 || blockedAttempts[0].Reason != "no_approved_template" {
		t.Fatalf("expected one no_approved_template attempt, got %+v", blockedAttempts)
	}

	if _, err := services.NotificationTemplateService.Approve(1, 9, created.ID); err != nil {
		t.Fatalf("approve template: %v", err)
	}
	approved := newNotificationTemplateCase("ticket_assigned", "原文标题", "原文内容", 501)
	if err := services.NotificationTemplateService.ApplyToNotification(approved, 501, variables); err != nil {
		t.Fatalf("apply approved template: %v", err)
	}
	if approved.TemplateID != created.ID {
		t.Fatalf("expected approved template %d to be used, got %d", created.ID, approved.TemplateID)
	}
	if approved.Title != "工单 TK-9 已分配" || approved.Content != "请处理 打印机故障" {
		t.Fatalf("unexpected rendered content: %q / %q", approved.Title, approved.Content)
	}
}

func TestNotificationTemplateServiceRendersRecipientLanguage(t *testing.T) {
	db := setupNotificationTemplateTestDB(t)
	seedNotificationTemplateUser(t, db, 1, 601, "en-US")
	seedNotificationTemplateUser(t, db, 1, 602, "zh-CN")

	zh, err := services.NotificationTemplateService.Create(1, 9, request.SaveNotificationTemplateRequest{
		Code: "ticket_assigned", Name: "分配提醒", Channel: "in_app", Language: "zh-CN",
		TitleTemplate: "工单 {{TicketNo}} 已分配",
	})
	if err != nil {
		t.Fatalf("create zh template: %v", err)
	}
	if _, err := services.NotificationTemplateService.Approve(1, 9, zh.ID); err != nil {
		t.Fatalf("approve zh template: %v", err)
	}
	en, err := services.NotificationTemplateService.Create(1, 9, request.SaveNotificationTemplateRequest{
		Code: "ticket_assigned", Name: "Assignment", Channel: "in_app", Language: "en-US",
		TitleTemplate: "Ticket {{TicketNo}} assigned to you",
	})
	if err != nil {
		t.Fatalf("create en template: %v", err)
	}
	if _, err := services.NotificationTemplateService.Approve(1, 9, en.ID); err != nil {
		t.Fatalf("approve en template: %v", err)
	}

	english := newNotificationTemplateCase("ticket_assigned", "原文", "原文", 601)
	if err := services.NotificationTemplateService.ApplyToNotification(english, 601, map[string]string{"TicketNo": "TK-7"}); err != nil {
		t.Fatalf("apply english template: %v", err)
	}
	if english.Title != "Ticket TK-7 assigned to you" || english.Language != "en-US" {
		t.Fatalf("expected english rendering, got %q (%s)", english.Title, english.Language)
	}

	chinese := newNotificationTemplateCase("ticket_assigned", "原文", "原文", 602)
	if err := services.NotificationTemplateService.ApplyToNotification(chinese, 602, map[string]string{"TicketNo": "TK-7"}); err != nil {
		t.Fatalf("apply chinese template: %v", err)
	}
	if chinese.Title != "工单 TK-7 已分配" {
		t.Fatalf("expected chinese rendering, got %q", chinese.Title)
	}
}

func TestNotificationTemplateServiceFallsBackToApprovedGenericTemplate(t *testing.T) {
	db := setupNotificationTemplateTestDB(t)
	seedNotificationTemplateUser(t, db, 1, 801, "zh-CN")

	created, err := services.NotificationTemplateService.Create(1, 9, request.SaveNotificationTemplateRequest{
		Code: services.NotificationTemplateCodeGeneric, Name: "通用通知", Channel: "in_app", Language: "zh-CN",
		TitleTemplate: "{{Title}}", ContentTemplate: "{{Content}}",
	})
	if err != nil {
		t.Fatalf("create generic template: %v", err)
	}
	if _, err := services.NotificationTemplateService.Approve(1, 9, created.ID); err != nil {
		t.Fatalf("approve generic template: %v", err)
	}

	// 没有专属模板的通知类型，必须回退到已批准的通用模板，而不是直接发原文。
	item := newNotificationTemplateCase("device_alarm_raised", "设备告警", "设备离线 15 分钟", 801)
	if err := services.NotificationTemplateService.ApplyToNotification(item, 801, nil); err != nil {
		t.Fatalf("apply generic template: %v", err)
	}
	if item.TemplateID != created.ID || item.TemplateCode != services.NotificationTemplateCodeGeneric {
		t.Fatalf("expected generic template to be used, got %+v", item)
	}
	if item.Title != "设备告警" || item.Content != "设备离线 15 分钟" {
		t.Fatalf("unexpected generic rendering: %q / %q", item.Title, item.Content)
	}
}

func TestEnsurePlatformDefaultsDBSeedsApprovedTemplates(t *testing.T) {
	db := setupNotificationTemplateTestDB(t)
	seedNotificationTemplateUser(t, db, 42, 901, "zh-CN")

	created, err := services.NotificationTemplateService.EnsurePlatformDefaultsDB(db)
	if err != nil {
		t.Fatalf("ensure platform defaults: %v", err)
	}
	if created != 45 {
		t.Fatalf("expected 45 platform baseline templates, got %d", created)
	}
	again, err := services.NotificationTemplateService.EnsurePlatformDefaultsDB(db)
	if err != nil {
		t.Fatalf("re-run ensure platform defaults: %v", err)
	}
	if again != 0 {
		t.Fatalf("expected idempotent seeding, got %d new rows", again)
	}

	var templates []models.NotificationTemplate
	if err := db.Find(&templates).Error; err != nil {
		t.Fatalf("load templates: %v", err)
	}
	for i := range templates {
		if templates[i].ApprovalStatus != services.NotificationTemplateStatusApproved || templates[i].TenantID != 0 {
			t.Fatalf("platform baseline must be approved and platform-scoped: %+v", templates[i])
		}
	}

	// 平台基线让“没有自定义模板的租户”也能正常发通知。
	item := newNotificationTemplateCase("ticket_closed", "工单已关闭", "客户主动结束工单", 901)
	if err := services.NotificationTemplateService.ApplyToNotification(item, 901, map[string]string{"TicketNo": "TK-5"}); err != nil {
		t.Fatalf("apply platform baseline template: %v", err)
	}
	if item.TemplateCode != "ticket_closed" {
		t.Fatalf("expected ticket_closed template, got %q", item.TemplateCode)
	}
	if item.Title != "工单 TK-5 已关闭" {
		t.Fatalf("unexpected baseline rendering: %q", item.Title)
	}

	// 没有专属模板的通知类型仍然能发出，只是走通用基线模板。
	fallback := newNotificationTemplateCase("security_alert", "安全告警", "检测到异常登录", 901)
	if err := services.NotificationTemplateService.ApplyToNotification(fallback, 901, nil); err != nil {
		t.Fatalf("apply generic baseline template: %v", err)
	}
	if fallback.TemplateCode != services.NotificationTemplateCodeGeneric || fallback.Title != "安全告警" {
		t.Fatalf("unexpected generic fallback rendering: %+v", fallback)
	}
}

func TestNotificationTemplateServiceBlocksSensitiveContent(t *testing.T) {
	db := setupNotificationTemplateTestDB(t)
	seedNotificationTemplateUser(t, db, 1, 701, "zh-CN")

	created, err := services.NotificationTemplateService.Create(1, 9, request.SaveNotificationTemplateRequest{
		Code: "ticket_closed", Name: "关闭提醒", Channel: "in_app", Language: "zh-CN",
		TitleTemplate: "工单 {{TicketNo}} 已关闭", ContentTemplate: "客户电话 13800138000，请回访。",
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	if _, err := services.NotificationTemplateService.Approve(1, 9, created.ID); err != nil {
		t.Fatalf("approve template: %v", err)
	}

	item := newNotificationTemplateCase("ticket_closed", "原文标题", "原文内容", 701)
	if err := services.NotificationTemplateService.ApplyToNotification(item, 701, map[string]string{"TicketNo": "TK-3"}); err == nil {
		t.Fatal("expected sensitive content to be blocked")
	}

	var attempts []models.NotificationDeliveryAttempt
	if err := db.Find(&attempts).Error; err != nil {
		t.Fatalf("load attempts: %v", err)
	}
	if len(attempts) != 1 {
		t.Fatalf("expected 1 blocked attempt, got %d", len(attempts))
	}
	if attempts[0].Status != services.NotificationDeliveryAttemptStatusBlocked {
		t.Fatalf("expected blocked attempt, got %q", attempts[0].Status)
	}
	if !strings.Contains(attempts[0].Reason, "phone_cn") {
		t.Fatalf("expected phone rule reason, got %q", attempts[0].Reason)
	}
}

func TestNotificationTemplateServicePreviewReportsSensitiveRule(t *testing.T) {
	setupNotificationTemplateTestDB(t)

	result, err := services.NotificationTemplateService.Preview(1, request.PreviewNotificationTemplateRequest{
		Code: "ticket_closed", Channel: "in_app", Language: "zh-CN",
		TitleTemplate: "工单 {{TicketNo}} 已关闭", ContentTemplate: "请联系 admin@example.com 复核。",
		Variables: map[string]string{"TicketNo": "TK-11"},
	})
	if err != nil {
		t.Fatalf("preview template: %v", err)
	}
	if !result.Blocked || result.RuleCode != "email" {
		t.Fatalf("expected email rule hit, got %+v", result)
	}
	if result.Title != "工单 TK-11 已关闭" {
		t.Fatalf("unexpected preview title %q", result.Title)
	}
}

func TestNotificationTemplateServiceSeedDraftsIsIdempotent(t *testing.T) {
	setupNotificationTemplateTestDB(t)

	created, err := services.NotificationTemplateService.SeedDrafts(1, 9)
	if err != nil {
		t.Fatalf("seed drafts: %v", err)
	}
	if created == 0 {
		t.Fatal("expected seeded drafts")
	}
	again, err := services.NotificationTemplateService.SeedDrafts(1, 9)
	if err != nil {
		t.Fatalf("seed drafts again: %v", err)
	}
	if again != 0 {
		t.Fatalf("expected seeds to be idempotent, created %d on second run", again)
	}
}

func TestScanNotificationSensitiveContent(t *testing.T) {
	cases := []struct {
		text string
		code string
	}{
		{"请联系 13800138000 处理工单", "phone_cn"},
		{"邮箱 admin@example.com 可以联系", "email"},
		{"身份证 110101199003074219 需要核验", "id_card_cn"},
		{"账号 6222021234567890123 已登记", "bank_card"},
		{"工单 TK-20260916 已创建，无敏感信息", ""},
	}
	for _, item := range cases {
		finding := services.ScanNotificationSensitiveContent(item.text)
		if item.code == "" {
			if finding != nil {
				t.Fatalf("expected no rule hit for %q, got %+v", item.text, finding)
			}
			continue
		}
		if finding == nil || finding.Code != item.code {
			t.Fatalf("expected rule %q for %q, got %+v", item.code, item.text, finding)
		}
	}
}

func TestRenderNotificationTemplateTextKeepsUnknownVariables(t *testing.T) {
	rendered := services.RenderNotificationTemplateText("工单 {{TicketNo}} 由 {{Unknown}} 处理", map[string]string{"TicketNo": "TK-1"})
	if rendered != "工单 TK-1 由 {{Unknown}} 处理" {
		t.Fatalf("unexpected render result %q", rendered)
	}
	if got := services.RenderNotificationTemplateText("", map[string]string{}); got != "" {
		t.Fatalf("expected empty render, got %q", got)
	}
	if rendered := services.RenderNotificationTemplateText("工单 {{ TicketNo }} 完成", map[string]string{"TicketNo": "TK-2"}); rendered != "工单 TK-2 完成" {
		t.Fatalf("expected trimmed variable render, got %q", rendered)
	}
}
