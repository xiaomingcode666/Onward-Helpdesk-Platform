package services

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestTicketSupportReadinessRecordsMissingAndGrantedKnowledgeAccess(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.KnowledgeBase{},
		&models.KnowledgeAccessGrant{},
		&models.Ticket{},
		&models.TicketProgress{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	now := time.Now().Truncate(time.Second)
	knowledgeBase := models.KnowledgeBase{
		TenantID: 1, Name: "售后知识库", KnowledgeType: "document", AccessScope: "tenant",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&knowledgeBase).Error; err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}

	restrictedTicket := models.Ticket{
		TenantID: 1, TicketNo: "SUPPORT-RESTRICTED", KnowledgeBaseID: knowledgeBase.ID,
		CurrentTeamID: 17, CurrentAssigneeID: 13, Status: enums.TicketStatusPendingAssigneeAccept,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&restrictedTicket).Error; err != nil {
		t.Fatalf("create restricted ticket: %v", err)
	}
	if err := EvaluateTicketSupportReadinessDB(db, &restrictedTicket, 17, 13, 0, now); err != nil {
		t.Fatalf("evaluate restricted ticket: %v", err)
	}
	if restrictedTicket.SupportStatus != SupportStatusRestricted ||
		restrictedTicket.SupportReasonCode != SupportReasonMissingAccess ||
		!strings.Contains(restrictedTicket.SupportReason, "售后知识库") {
		t.Fatalf("restricted readiness = %+v", restrictedTicket)
	}
	var progressCount int64
	if err := db.Model(&models.TicketProgress{}).
		Where("ticket_id = ? AND event_type = ?", restrictedTicket.ID, enums.TicketProgressEventSupportReadiness).
		Count(&progressCount).Error; err != nil || progressCount != 1 {
		t.Fatalf("support progress count=%d err=%v", progressCount, err)
	}

	grant := models.KnowledgeAccessGrant{
		TenantID: 1, KnowledgeBaseID: knowledgeBase.ID, SubjectType: KnowledgeAccessSubjectTeam,
		SubjectID: 17, AccessLevel: KnowledgeAccessLevelOperate, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&grant).Error; err != nil {
		t.Fatalf("create access grant: %v", err)
	}

	readyTicket := models.Ticket{
		TenantID: 1, TicketNo: "SUPPORT-READY", KnowledgeBaseID: knowledgeBase.ID,
		CurrentTeamID: 17, CurrentAssigneeID: 13, Status: enums.TicketStatusPendingAssigneeAccept,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&readyTicket).Error; err != nil {
		t.Fatalf("create ready ticket: %v", err)
	}
	if err := EvaluateTicketSupportReadinessDB(db, &readyTicket, 17, 13, 0, now); err != nil {
		t.Fatalf("evaluate ready ticket: %v", err)
	}
	if readyTicket.SupportStatus != SupportStatusReady || readyTicket.SupportReasonCode != "" {
		t.Fatalf("ready readiness = %+v", readyTicket)
	}
}

func TestDefaultTicketKnowledgeBaseFallsBackToTenantDefault(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.KnowledgeBase{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	item := models.KnowledgeBase{
		TenantID: 1, Name: "企业通用知识库", AccessScope: "tenant", Status: enums.StatusOk,
		Remark: tenantDefaultKnowledgeBaseRemark,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}
	if got := defaultTicketKnowledgeBaseDB(db, 1); got == nil || got.ID != item.ID {
		t.Fatalf("default knowledge base = %+v, want %d", got, item.ID)
	}
}
