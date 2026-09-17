package services_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
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

func TestNotificationTemplateServiceUsesBuiltInTemplateWithoutDatabaseRows(t *testing.T) {
	db := setupNotificationTemplateTestDB(t)
	seedNotificationTemplateUser(t, db, 1, 911, "zh-CN")

	item := newNotificationTemplateCase("ticket_assigned", "原文标题", "原文内容", 911)
	if err := services.NotificationTemplateService.ApplyToNotification(item, 911, map[string]string{
		"TicketNo":    "TK-911",
		"TicketTitle": "打印机故障",
		"Reason":      "请及时处理。",
	}); err != nil {
		t.Fatalf("apply built-in template: %v", err)
	}
	if item.TemplateID != 0 {
		t.Fatalf("built-in template should not depend on a database id, got %d", item.TemplateID)
	}
	if item.TemplateCode != "ticket_assigned" || item.Language != "zh-CN" {
		t.Fatalf("unexpected built-in template metadata: %+v", item)
	}
	if item.Title != "工单 TK-911 已分配给你" || !strings.Contains(item.Content, "打印机故障") {
		t.Fatalf("unexpected built-in rendering: %q / %q", item.Title, item.Content)
	}
}

func TestNotificationTemplateServiceRendersRecipientLanguage(t *testing.T) {
	db := setupNotificationTemplateTestDB(t)
	seedNotificationTemplateUser(t, db, 1, 601, "en-US")
	seedNotificationTemplateUser(t, db, 1, 602, "zh-CN")

	english := newNotificationTemplateCase("ticket_assigned", "原文", "原文", 601)
	if err := services.NotificationTemplateService.ApplyToNotification(english, 601, map[string]string{
		"TicketNo": "TK-7",
	}); err != nil {
		t.Fatalf("apply english template: %v", err)
	}
	if english.Title != "Ticket TK-7 assigned to you" || english.Language != "en-US" {
		t.Fatalf("expected english rendering, got %q (%s)", english.Title, english.Language)
	}

	chinese := newNotificationTemplateCase("ticket_assigned", "原文", "原文", 602)
	if err := services.NotificationTemplateService.ApplyToNotification(chinese, 602, map[string]string{
		"TicketNo": "TK-7",
	}); err != nil {
		t.Fatalf("apply chinese template: %v", err)
	}
	if chinese.Title != "工单 TK-7 已分配给你" {
		t.Fatalf("expected chinese rendering, got %q", chinese.Title)
	}
}

func TestNotificationTemplateServiceFallsBackToBuiltInGenericTemplate(t *testing.T) {
	db := setupNotificationTemplateTestDB(t)
	seedNotificationTemplateUser(t, db, 1, 801, "zh-CN")

	item := newNotificationTemplateCase("device_alarm_raised", "设备告警", "设备离线 15 分钟", 801)
	if err := services.NotificationTemplateService.ApplyToNotification(item, 801, nil); err != nil {
		t.Fatalf("apply generic template: %v", err)
	}
	if item.TemplateID != 0 || item.TemplateCode != services.NotificationTemplateCodeGeneric {
		t.Fatalf("expected built-in generic template to be used, got %+v", item)
	}
	if item.Title != "设备告警" || item.Content != "设备离线 15 分钟" {
		t.Fatalf("unexpected generic rendering: %q / %q", item.Title, item.Content)
	}
}

func TestBuiltInNotificationTemplateCatalogCoversAllSupportedLocales(t *testing.T) {
	codes := []string{
		"ticket_created",
		"ticket_assigned",
		"ticket_closed",
		"sla_warning",
		"ticket_created_assigned",
		"ticket_assigned_transferred",
		"ticket_assigned_accepted",
		"ticket_assigned_cancelled",
		"ticket_assigned_recovered",
		services.NotificationTemplateCodeGeneric,
	}
	locales := []string{"zh-CN", "en-US", "es-ES"}

	for _, code := range codes {
		for _, channel := range []string{
			services.NotificationTemplateChannelInApp,
			services.NotificationTemplateChannelEmail,
		} {
			if code == "ticket_created_assigned" ||
				code == "ticket_assigned_transferred" ||
				code == "ticket_assigned_accepted" ||
				code == "ticket_assigned_cancelled" ||
				code == "ticket_assigned_recovered" {
				if channel == services.NotificationTemplateChannelEmail {
					continue
				}
			}
			for _, locale := range locales {
				if template := services.NotificationTemplateService.ResolveApproved(0, code, channel, locale); template == nil {
					t.Fatalf("missing built-in template code=%s channel=%s locale=%s", code, channel, locale)
				}
			}
		}
	}
}

func TestNotificationTemplateServiceFallsBackToDefaultLanguage(t *testing.T) {
	template := services.NotificationTemplateService.ResolveApproved(
		0,
		"ticket_closed",
		services.NotificationTemplateChannelInApp,
		"fr-FR",
	)
	if template == nil {
		t.Fatal("expected unsupported locale to fall back to the default language")
	}
	if template.Language != "zh-CN" {
		t.Fatalf("fallback language = %q, want zh-CN", template.Language)
	}
}

func TestNotificationTemplateServiceBlocksSensitiveContent(t *testing.T) {
	db := setupNotificationTemplateTestDB(t)
	seedNotificationTemplateUser(t, db, 1, 701, "zh-CN")

	item := newNotificationTemplateCase("ticket_closed", "原文标题", "客户电话 13800138000，请回访。", 701)
	if err := services.NotificationTemplateService.ApplyToNotification(item, 701, map[string]string{
		"TicketNo": "TK-3",
	}); err == nil {
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
