package migration

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestReconcileAssignedTicketNotifications(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "assigned-ticket-notifications.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.TenantMember{}, &models.Ticket{}, &models.Notification{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	now := time.Now()
	user := &models.User{
		Username: "dispatch-engineer",
		Nickname: "派单工程师",
		Status:   enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&models.TenantMember{
		TenantID:    7,
		UserID:      user.ID,
		DisplayName: "产品组值班工程师",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}
	assignedTicket := &models.Ticket{
		TenantID:          7,
		TicketNo:          "TK-ASSIGNED",
		Title:             "设备断电后仍无法恢复",
		Status:            enums.TicketStatusAssigned,
		CurrentAssigneeID: user.ID,
		AuditFields:       models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	unassignedTicket := &models.Ticket{
		TenantID:    7,
		TicketNo:    "TK-UNASSIGNED",
		Title:       "设备等待派工",
		Status:      enums.TicketStatusPending,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(assignedTicket).Error; err != nil {
		t.Fatalf("create assigned ticket: %v", err)
	}
	if err := db.Create(unassignedTicket).Error; err != nil {
		t.Fatalf("create unassigned ticket: %v", err)
	}
	assignedNotification := &models.Notification{
		TenantID: 7, RecipientUserID: user.ID, Title: "转人工工单 TK-ASSIGNED 待处理",
		Content:          "设备断电后仍无法恢复\n当前未指派处理人，请及时派工。",
		NotificationType: "ticket_created", BizType: "ticket", BizID: assignedTicket.ID,
		Level: "warning", Status: int(enums.StatusOk), CreatedAt: now,
	}
	unassignedNotification := &models.Notification{
		TenantID: 7, RecipientUserID: user.ID, Title: "转人工工单 TK-UNASSIGNED 待处理",
		Content:          "设备等待派工\n当前未指派处理人，请及时派工。",
		NotificationType: "ticket_created", BizType: "ticket", BizID: unassignedTicket.ID,
		Level: "warning", Status: int(enums.StatusOk), CreatedAt: now,
	}
	if err := db.Create(assignedNotification).Error; err != nil {
		t.Fatalf("create assigned notification: %v", err)
	}
	if err := db.Create(unassignedNotification).Error; err != nil {
		t.Fatalf("create unassigned notification: %v", err)
	}

	if err := reconcileAssignedTicketNotifications(db); err != nil {
		t.Fatalf("reconcileAssignedTicketNotifications() error = %v", err)
	}
	if err := reconcileAssignedTicketNotifications(db); err != nil {
		t.Fatalf("idempotent reconcileAssignedTicketNotifications() error = %v", err)
	}

	var reconciled models.Notification
	if err := db.First(&reconciled, assignedNotification.ID).Error; err != nil {
		t.Fatalf("load reconciled notification: %v", err)
	}
	if reconciled.Level != "info" || !strings.Contains(reconciled.Title, "已分配") || strings.Contains(reconciled.Content, "未指派") {
		t.Fatalf("assigned notification not reconciled: %+v", reconciled)
	}
	if !strings.Contains(reconciled.Content, "产品组值班工程师") {
		t.Fatalf("assignee name missing from reconciled notification: %q", reconciled.Content)
	}

	var untouched models.Notification
	if err := db.First(&untouched, unassignedNotification.ID).Error; err != nil {
		t.Fatalf("load unassigned notification: %v", err)
	}
	if untouched.Level != "warning" || !strings.Contains(untouched.Content, "当前未指派处理人") {
		t.Fatalf("unassigned notification should stay actionable: %+v", untouched)
	}
}
