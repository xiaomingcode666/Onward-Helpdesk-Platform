package services

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/projectconfig"
)

func setupProjectRuntime(t *testing.T) (*gorm.DB, *dto.AuthPrincipal, projectconfig.Document) {
	t.Helper()
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "development")
	t.Setenv("RHD_PROJECT_CONFIG_FILE", "")
	t.Setenv("RHD_PROJECT_CONFIG_REQUIRED", "")
	db := setupSLATenantTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Tenant{}, &models.TenantBranding{}, &models.ProjectConfigurationState{}, &models.ProjectConfigurationVersion{}, &models.ProjectConfigurationActivation{}, &ServiceCalendar{}, &models.DataRegionPolicy{}, &models.TenantIntegrationConfig{}, &models.TenantMailSetting{}, &models.AuditLog{}, &models.Conversation{}, &models.ConversationEventLog{}, &models.Notification{}, &models.CustomerPrivacyConsent{}))
	require.NoError(t, db.Create(&models.Tenant{ID: 1, Name: "Runtime fixture", ServiceScene: "knowledge_support", DefaultLocale: "zh-CN", Timezone: "UTC", SupportedLocalesJSON: `["zh-CN"]`, Status: enums.StatusOk}).Error)
	op := &dto.AuthPrincipal{TenantID: 1, UserID: 7, Username: "fixture", Roles: []string{EnterpriseRoleAdmin}, Permissions: []string{constants.PermissionTicketUpdate.Code}}
	d, err := UpgradeProjectConfiguration(1)
	require.NoError(t, err)
	d.Runtime.Mail = projectconfig.Mail{RetryPolicy: "no_retry"}
	d.SecretRefs = []string{}
	d.Runtime.Targets = []projectconfig.Target{{ProjectKey: "*", Profile: "standard", Priority: "p2", CalendarKey: d.Runtime.Calendars[0].Key, ResponseMinutes: 30, AssignmentMinutes: 60, ResolutionMinutes: 120}}
	return db, op, *d
}
func activateRuntime(t *testing.T, doc projectconfig.Document, op *dto.AuthPrincipal) *ProjectConfigVersion {
	t.Helper()
	v, err := GetProjectConfiguration(doc.TenantID)
	require.NoError(t, err)
	d, err := SaveProjectConfigurationDraft(doc.TenantID, ProjectConfigDraft{Document: doc, BaseVersionID: v.ActiveVersionID, RequestKey: fmt.Sprintf("runtime-%d", time.Now().UnixNano()), Note: "Runtime acceptance"}, op)
	require.NoError(t, err)
	applied, err := ApplyProjectConfiguration(doc.TenantID, d.ID, op)
	require.NoError(t, err)
	return applied
}
func TestProjectRuntimeActivationRestoreAndPinnedSLA(t *testing.T) {
	db, op, doc := setupProjectRuntime(t)
	first := activateRuntime(t, doc, op)
	ticket := models.Ticket{TenantID: 1, TicketNo: "RUNTIME-1", PriorityCode: "p2", Channel: "manual", Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: time.Now().Add(-45 * time.Minute)}}
	require.NoError(t, TicketService.Create(&ticket))
	require.Equal(t, first.ID, ticket.ProjectConfigVersionID)
	require.WithinDuration(t, ticket.CreatedAt.Add(120*time.Minute), *ticket.SLADueAt, time.Second)
	blocked := models.Ticket{TenantID: 1, TicketNo: "BLOCKED", Channel: "email"}
	require.Error(t, TicketService.Create(&blocked))
	require.NoError(t, db.Create(&models.Tenant{ID: 2, Name: "Other", ServiceScene: "knowledge_support"}).Error)
	_, _, err := projectRuntimeDB(db, 2, first.ID)
	require.Error(t, err)
	doc.Runtime.Locale = "en-US"
	doc.Runtime.Locales = []string{"zh-CN", "en-US"}
	doc.Runtime.Targets = []projectconfig.Target{}
	second := activateRuntime(t, doc, op)
	require.NotEqual(t, first.ID, second.ID)
	var company models.Tenant
	require.NoError(t, db.First(&company, 1).Error)
	require.Equal(t, "en-US", company.DefaultLocale)
	// Removing the active target must not stop existing ticket clocks.
	violations, err := SLAService.CheckSLAViolations()
	require.NoError(t, err)
	require.Len(t, violations, 1)
	require.Equal(t, "frt", violations[0].ViolationType)
	restored := activateRuntime(t, first.Document, op)
	require.NotEqual(t, first.ID, restored.ID)
	require.NoError(t, db.First(&company, 1).Error)
	require.Equal(t, "zh-CN", company.DefaultLocale)
	require.NoError(t, db.First(&ticket, ticket.ID).Error)
	require.Equal(t, first.ID, ticket.ProjectConfigVersionID)
	_, err = SLAService.CreateSLAPolicy(CreateSLAPolicyInput{TenantID: "1", Name: "Bypass", Priority: "p1", FRTMinutes: 1})
	require.Error(t, err)
	_, err = MailSettingService.Save(1, request.UpdateMailSettingRequest{})
	require.Error(t, err)
	_, err = TicketAutoCloseService.UpdatePolicy(1, dto.TicketAutoClosePolicyDTO{Enabled: true, Days: 2}, op)
	require.Error(t, err)
	require.Error(t, DataRetentionService.SetRetentionPolicy(context.Background(), "1", "global", 10, 0, true, false, false))
	// Old schema cannot drop managed fields through a direct API draft.
	legacy := first.Document
	legacy.SchemaVersion = 1
	legacy.Runtime = nil
	v, err := SaveProjectConfigurationDraft(1, ProjectConfigDraft{Document: legacy, BaseVersionID: restored.ID, RequestKey: "reject-v1-downgrade", Note: "test"}, op)
	require.NoError(t, err)
	_, err = ApplyProjectConfiguration(1, v.ID, op)
	require.Error(t, err)
}
func TestProjectRuntimeMailResolutionAndMissingSecret(t *testing.T) {
	_, op, doc := setupProjectRuntime(t)
	root := t.TempDir()
	t.Setenv("RHD_PROJECT_SECRET_DIR", root)
	dir := filepath.Join(root, "1", "development")
	require.NoError(t, os.MkdirAll(dir, 0700))
	path := filepath.Join(dir, "smtp-password")
	require.NoError(t, os.WriteFile(path, []byte("fixture-password"), 0600))
	doc.SecretRefs = []string{"secret://smtp-password"}
	doc.Runtime.Mail = projectconfig.Mail{Enabled: true, Host: "smtp.example.test", Port: 465, Username: "fixture", PasswordRef: doc.SecretRefs[0], FromAddress: "fixture@example.test", UseTLS: true, ReplyTo: "reply@example.test", RetryPolicy: "retry_1_5m"}
	activateRuntime(t, doc, op)
	cfg, err := resolveProjectMailConfig(1)
	require.NoError(t, err)
	require.Equal(t, "fixture-password", cfg.Password)
	require.Equal(t, "reply@example.test", cfg.ReplyTo)
	retries, delay := notificationMailRetryPolicy(1)
	require.Equal(t, 1, retries)
	require.Equal(t, 5*time.Minute, delay)
	view, err := GetProjectConfiguration(1)
	require.NoError(t, err)
	encoded, err := json.Marshal(view)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "fixture-password")
	require.NoError(t, os.Remove(path))
	_, err = resolveProjectMailConfig(1)
	require.Error(t, err)
	// Only tenant-scoped files can satisfy the reference; no global fallback.
	_, err = resolveProjectMailConfig(1)
	require.Error(t, err)
}
func TestProjectRuntimeInvalidApplyRollsBack(t *testing.T) {
	db, op, doc := setupProjectRuntime(t)
	first := activateRuntime(t, doc, op)
	doc.Runtime.Locale = "en-US"
	doc.Runtime.Locales = []string{"en-US"}
	draft, err := SaveProjectConfigurationDraft(1, ProjectConfigDraft{Document: doc, BaseVersionID: first.ID, RequestKey: "rollback-failure", Note: "test"}, op)
	require.NoError(t, err)
	// Fail after tenant projection was written: transaction must undo everything.
	require.NoError(t, db.Migrator().DropTable(&models.DataRegionPolicy{}))
	_, err = ApplyProjectConfiguration(1, draft.ID, op)
	require.Error(t, err)
	var company models.Tenant
	require.NoError(t, db.First(&company, 1).Error)
	require.Equal(t, "zh-CN", company.DefaultLocale)
	view, err := GetProjectConfiguration(1)
	require.NoError(t, err)
	require.Equal(t, first.ID, view.ActiveVersionID)
}
func TestProjectRuntimeRetentionHoldAndUpgradeConflict(t *testing.T) {
	db, op, doc := setupProjectRuntime(t)
	doc.Runtime.Retention.LegalHold = true
	doc.Runtime.Retention.AutoDelete = true
	doc.Runtime.Retention.Days = 10
	doc.Runtime.Retention.ArchiveAfterDays = 0
	activateRuntime(t, doc, op)
	old := time.Now().AddDate(0, 0, -30)
	require.NoError(t, db.Create(&models.Notification{TenantID: 1, CreatedAt: old}).Error)
	require.NoError(t, DataRetentionService.CleanupExpiredData(context.Background()))
	var count int64
	require.NoError(t, db.Model(&models.Notification{}).Where("tenant_id = 1").Count(&count).Error)
	require.EqualValues(t, 1, count)
	doc.Runtime.Retention.LegalHold = false
	activateRuntime(t, doc, op)
	require.NoError(t, DataRetentionService.CleanupExpiredData(context.Background()))
	require.NoError(t, db.Model(&models.Notification{}).Where("tenant_id = 1").Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Create(&models.Tenant{ID: 2, Name: "Multiple policies", ServiceScene: "knowledge_support"}).Error)
	require.NoError(t, db.Create(&[]models.DataRegionPolicy{{ID: "region-a", TenantID: "2", DataRegion: "a", Status: "active"}, {ID: "region-b", TenantID: "2", DataRegion: "b", Status: "active"}}).Error)
	_, err := UpgradeProjectConfiguration(2)
	require.ErrorContains(t, err, "多条保存策略")
}

func TestProjectRuntimeSMTPUsesVersionedSender(t *testing.T) {
	_, op, doc := setupProjectRuntime(t)
	root := t.TempDir()
	t.Setenv("RHD_PROJECT_SECRET_DIR", root)
	dir := filepath.Join(root, "1", "development")
	require.NoError(t, os.MkdirAll(dir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "smtp"), []byte("local-fixture-only"), 0600))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	received := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
		reader := bufio.NewReader(conn)
		_, _ = fmt.Fprint(conn, "220 localhost fixture\r\n")
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO"):
				_, _ = fmt.Fprint(conn, "250-localhost\r\n250 AUTH PLAIN\r\n")
			case strings.HasPrefix(line, "AUTH"):
				_, _ = fmt.Fprint(conn, "235 authenticated\r\n")
			case strings.HasPrefix(line, "DATA"):
				_, _ = fmt.Fprint(conn, "354 data\r\n")
				var body strings.Builder
				for {
					part, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					if part == ".\r\n" {
						break
					}
					body.WriteString(part)
				}
				received <- body.String()
				_, _ = fmt.Fprint(conn, "250 received\r\n")
			case strings.HasPrefix(line, "QUIT"):
				_, _ = fmt.Fprint(conn, "221 bye\r\n")
				return
			default:
				_, _ = fmt.Fprint(conn, "250 ok\r\n")
			}
		}
	}()
	doc.SecretRefs = []string{"secret://smtp"}
	doc.Runtime.Mail = projectconfig.Mail{Enabled: true, Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, Username: "fixture", PasswordRef: "secret://smtp", FromAddress: "sender@example.test", ReplyTo: "reply@example.test", RetryPolicy: "no_retry"}
	activateRuntime(t, doc, op)
	require.NoError(t, EmailNotificationService.sendEmail(1, "recipient@example.test", "", "notification_generic", map[string]interface{}{"Title": "Local runtime acceptance", "Content": "Synthetic message"}))
	select {
	case body := <-received:
		require.Contains(t, body, "From: RemoteHelpDesk <sender@example.test>")
		require.Contains(t, body, "Reply-To: reply@example.test")
		require.Contains(t, body, "Synthetic message")
	case <-time.After(time.Second):
		t.Fatal("local SMTP received nothing")
	}
	// Closing the mail switch takes effect immediately without falling back.
	doc.Runtime.Mail.Enabled = false
	activateRuntime(t, doc, op)
	require.ErrorContains(t, EmailNotificationService.sendEmail(1, "recipient@example.test", "", "notification_generic", nil), "未启用")
}

func TestProjectRuntimePermissionsDraftSecretsAndDispatch(t *testing.T) {
	db, op, doc := setupProjectRuntime(t)
	engineer := *op
	engineer.Roles = []string{EnterpriseRoleEngineer}
	_, err := SaveProjectConfigurationDraft(1, ProjectConfigDraft{Document: doc, RequestKey: "engineer-runtime-draft", Note: "test"}, &engineer)
	require.Error(t, err)
	doc.Runtime.Mail.PasswordRef = "accidental-password"
	_, err = SaveProjectConfigurationDraft(1, ProjectConfigDraft{Document: doc, RequestKey: "plaintext-runtime-draft", Note: "test"}, op)
	require.ErrorContains(t, err, "不能保存密码")
	doc.Runtime.Mail.PasswordRef = ""
	doc.Runtime.Calendars[0] = projectconfig.Calendar{Key: "office", Timezone: "UTC", WorkDays: []int{1, 2, 3, 4, 5}, Start: "09:00", End: "18:00", Holidays: []string{}}
	doc.Runtime.Targets[0].CalendarKey = "office"
	first := activateRuntime(t, doc, op)
	start := time.Date(2026, 9, 11, 17, 30, 0, 0, time.UTC)
	ticket := models.Ticket{TenantID: 1, TicketNo: "PINNED-DISPATCH", Channel: "manual", PriorityCode: "p2", AuditFields: models.AuditFields{CreatedAt: start}}
	require.NoError(t, TicketService.Create(&ticket))
	require.Equal(t, first.ID, ticket.ProjectConfigVersionID)
	deadline, ok := ticketAssignmentSLADeadlineDB(db, &ticket)
	require.True(t, ok)
	require.Equal(t, time.Date(2026, 9, 14, 9, 30, 0, 0, time.UTC), deadline)
	doc.Runtime.Targets[0].AssignmentMinutes = 1
	activateRuntime(t, doc, op)
	after, ok := ticketAssignmentSLADeadlineDB(db, &ticket)
	require.True(t, ok)
	require.Equal(t, deadline, after)
}

func TestProjectRuntimeGlobalAuditCannotOverrideHold(t *testing.T) {
	db, op, doc := setupProjectRuntime(t)
	require.NoError(t, db.AutoMigrate(&models.AuthAuditLog{}, &models.SystemConfig{}))
	doc.Runtime.Retention.LegalHold = true
	doc.Runtime.Retention.AutoDelete = true
	activateRuntime(t, doc, op)
	old := time.Now().AddDate(0, 0, -400)
	require.NoError(t, db.Create(&[]models.AuthAuditLog{{TenantID: 1, OccurredAt: old}, {TenantID: 2, OccurredAt: old}}).Error)
	_, err := AuditRetentionService.CleanupExpiredLogs(context.Background())
	require.NoError(t, err)
	var rows []models.AuthAuditLog
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	require.EqualValues(t, 1, rows[0].TenantID)
}

func TestProjectRuntimeIntegrationProjectionAndRestore(t *testing.T) {
	db, op, doc := setupProjectRuntime(t)
	doc.Runtime.Integrations = []projectconfig.Integration{{Provider: "onepanel", Enabled: true, BaseURL: "https://console.example.test", AppID: "original", MetadataJSON: `{"displayName":"Original","embedMode":"external"}`}}
	first := activateRuntime(t, doc, op)
	var row models.TenantIntegrationConfig
	require.NoError(t, db.Where("tenant_id = ? AND provider = ?", 1, "onepanel").First(&row).Error)
	require.True(t, row.Enabled)
	require.Equal(t, doc.Runtime.Integrations[0].MetadataJSON, row.MetadataJSON)
	require.Error(t, TenantIntegrationConfigService.UpdateStatus(row.ID, int(enums.StatusDisabled), op))
	_, err := TenantPortalSettingsService.SaveOnePanelDB(db, 1, "Bypass", "https://other.example.test", "external", true, op)
	require.Error(t, err)
	doc.Runtime.Integrations = []projectconfig.Integration{}
	activateRuntime(t, doc, op)
	require.NoError(t, db.First(&row, row.ID).Error)
	require.False(t, row.Enabled)
	activateRuntime(t, first.Document, op)
	require.NoError(t, db.First(&row, row.ID).Error)
	require.True(t, row.Enabled)
	require.Equal(t, "original", row.AppID)
}
