package services_test

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestConversationListAndSenderAreTenantScoped(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Conversation{}); err != nil {
		t.Fatalf("migrate conversations: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	now := time.Now()
	for _, conversation := range []models.Conversation{
		{TenantID: 101, CustomerName: "Tenant 101", Status: enums.IMConversationStatusActive, CurrentAssigneeID: 7, LastActiveAt: now},
		{TenantID: 202, CustomerName: "Tenant 202", Status: enums.IMConversationStatusActive, CurrentAssigneeID: 7, LastActiveAt: now},
	} {
		if err := db.Create(&conversation).Error; err != nil {
			t.Fatalf("create conversation: %v", err)
		}
	}

	list, paging, err := services.ConversationService.ListConversations(
		101,
		7,
		request.AgentConversationFilterActive,
		"",
		&sqls.Paging{Page: 1, Limit: 20},
		&dto.AuthPrincipal{TenantID: 101, UserID: 7},
	)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if paging.Total != 1 || len(list) != 1 || list[0].TenantID != 101 {
		t.Fatalf("tenant list = total %d rows %+v, want tenant 101 only", paging.Total, list)
	}

	var otherTenantConversation models.Conversation
	if err := db.Where("tenant_id = ?", 202).First(&otherTenantConversation).Error; err != nil {
		t.Fatalf("find other tenant conversation: %v", err)
	}
	operator := &dto.AuthPrincipal{TenantID: 101, UserID: 7}
	if _, err := services.MessageService.ValidateConversationSender(otherTenantConversation.ID, enums.IMSenderTypeAgent, operator, nil); err == nil {
		t.Fatal("expected cross-tenant sender validation to fail")
	}
}
