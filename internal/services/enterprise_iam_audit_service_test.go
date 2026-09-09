package services

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestEnterpriseAuditListMergesAuthAndBusinessLogs(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AuthAuditLog{}, &models.AuditLog{}); err != nil {
		t.Fatalf("migrate audit logs: %v", err)
	}
	sqls.SetDB(db)
	now := time.Now().UTC()
	auth := models.AuthAuditLog{
		TenantID: 1, DomainType: models.DomainTypeEnterprise, ActorUserID: 7,
		ActorSubjectType: models.SubjectTypeTenantMember, ActorSubjectID: 9,
		TargetType: "tenant_member", TargetID: "10", Action: "tenant_member.invited",
		RiskLevel: models.RiskLevelMedium, Status: models.AuditStatusSuccess, OccurredAt: now.Add(-time.Minute),
	}
	business := models.AuditLog{
		TenantID: 1, ActorID: "7", ActorType: models.SubjectTypeTenantMember,
		Domain: "meeting", ResourceType: "meeting", ResourceID: "meeting-1", Action: "meeting.created",
		SupportGrantID: 85, RiskLevel: models.RiskLevelLow, Status: models.AuditStatusSuccess, CreatedAt: now,
	}
	otherTenant := models.AuditLog{
		TenantID: 2, ActorID: "8", ActorType: "user", Domain: "meeting",
		ResourceType: "meeting", ResourceID: "meeting-2", Action: "meeting.created",
		RiskLevel: models.RiskLevelLow, Status: models.AuditStatusSuccess, CreatedAt: now,
	}
	if err := db.Create(&auth).Error; err != nil {
		t.Fatalf("create auth audit: %v", err)
	}
	if err := db.Create(&[]models.AuditLog{business, otherTenant}).Error; err != nil {
		t.Fatalf("create business audits: %v", err)
	}

	items, paging, err := EnterpriseIAMService.ListAuditLogs(1, AuditLogQuery{Page: 1, Limit: 20})
	if err != nil {
		t.Fatalf("list enterprise audits: %v", err)
	}
	if paging.Total != 2 || len(items) != 2 {
		t.Fatalf("unexpected merged audit result: paging=%+v items=%+v", paging, items)
	}
	if items[0].ID >= 0 || items[0].Action != "meeting.created" || items[0].SupportGrantID != 85 {
		t.Fatalf("business audit mapping mismatch: %+v", items[0])
	}
	filtered, filteredPaging, err := EnterpriseIAMService.ListAuditLogs(1, AuditLogQuery{
		Action: "meeting.created", RiskLevel: "low", Status: "success", TargetType: "meeting", ActorID: 7, Period: "24h", Page: 1, Limit: 20,
	})
	if err != nil {
		t.Fatalf("filter enterprise audits: %v", err)
	}
	if filteredPaging.Total != 1 || len(filtered) != 1 || filtered[0].Action != "meeting.created" {
		t.Fatalf("unexpected filtered audit result: paging=%+v items=%+v", filteredPaging, filtered)
	}
}

func TestPlatformAuditSearchFindsBusinessTicketLogByTicketNo(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AuthAuditLog{}, &models.AuditLog{}, &models.Ticket{}); err != nil {
		t.Fatalf("migrate audit logs: %v", err)
	}
	sqls.SetDB(db)

	ticket := models.Ticket{
		TenantID: 5,
		TicketNo: "TK-AUDIT-SEARCH-001",
		Title:    "audit searchable ticket",
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	audit := models.AuditLog{
		TenantID:     ticket.TenantID,
		ActorID:      "7",
		ActorType:    models.SubjectTypeTenantMember,
		Domain:       "ticket",
		ResourceType: "ticket",
		ResourceID:   strconv.FormatInt(ticket.ID, 10),
		Action:       "ticket.progress.created",
		AfterState:   `{"ticketId":` + strconv.FormatInt(ticket.ID, 10) + `}`,
		RiskLevel:    models.RiskLevelLow,
		Status:       models.AuditStatusSuccess,
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.Create(&audit).Error; err != nil {
		t.Fatalf("create business audit: %v", err)
	}

	items, paging, err := PlatformIAMService.ListAuditLogs(0, AuditLogQuery{
		Search: "TK-AUDIT-SEARCH-001", TargetType: "ticket", Page: 1, Limit: 20,
	})
	if err != nil {
		t.Fatalf("search platform audits: %v", err)
	}
	if paging.Total != 1 || len(items) != 1 || items[0].Action != "ticket.progress.created" {
		t.Fatalf("unexpected ticket audit search result: paging=%+v items=%+v", paging, items)
	}
	if !strings.Contains(items[0].AfterStateJSON, `"ticketNo":"TK-AUDIT-SEARCH-001"`) {
		t.Fatalf("ticket number was not enriched into audit metadata: %+v", items[0])
	}
}
