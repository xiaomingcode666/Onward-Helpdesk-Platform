package services

import (
	"path/filepath"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestConversationAccessFollowsProductTeamScope(t *testing.T) {
	db := setupConversationAccessScopeDB(t)
	now := time.Now()
	teamA := models.AgentTeam{TenantID: 1, ProductID: 11, Name: "产品 A 维修组", TeamType: AgentTeamTypeProductRepair, Status: enums.StatusOk}
	teamB := models.AgentTeam{TenantID: 1, ProductID: 22, Name: "产品 B 维修组", TeamType: AgentTeamTypeProductRepair, Status: enums.StatusOk}
	if err := db.Create(&[]*models.AgentTeam{&teamA, &teamB}).Error; err != nil {
		t.Fatalf("create product teams: %v", err)
	}
	engineer := models.User{Username: "product-a-engineer", Status: enums.StatusOk}
	if err := db.Create(&engineer).Error; err != nil {
		t.Fatalf("create engineer: %v", err)
	}
	profile := models.AgentProfile{TenantID: 1, UserID: engineer.ID, TeamID: teamA.ID, AgentCode: "ENG-A", DisplayName: "产品 A 工程师", Status: enums.StatusOk}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create engineer profile: %v", err)
	}
	membership := models.AgentTeamMember{
		TenantID: 1, TeamID: teamA.ID, UserID: engineer.ID,
		DispatchWeight: 1, DispatchEnabled: true, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&membership).Error; err != nil {
		t.Fatalf("create engineer team membership: %v", err)
	}
	conversations := []models.Conversation{
		{TenantID: 1, ProductID: 11, CurrentTeamID: teamA.ID, Status: enums.IMConversationStatusPending, LastActiveAt: now},
		{TenantID: 1, ProductID: 22, CurrentTeamID: teamB.ID, Status: enums.IMConversationStatusPending, LastActiveAt: now.Add(time.Second)},
		{TenantID: 2, ProductID: 11, CurrentTeamID: teamA.ID, Status: enums.IMConversationStatusPending, LastActiveAt: now.Add(2 * time.Second)},
	}
	if err := db.Create(&conversations).Error; err != nil {
		t.Fatalf("create conversations: %v", err)
	}
	operator := &dto.AuthPrincipal{
		TenantID: 1, UserID: engineer.ID, Username: engineer.Username,
		DomainType: models.DomainTypeEnterprise, Roles: []string{EnterpriseRoleEngineer},
	}
	if !ConversationService.CanAccessConversation(&conversations[0], operator) {
		t.Fatal("engineer must access a conversation in their product repair team")
	}
	if ConversationService.CanAccessConversation(&conversations[1], operator) {
		t.Fatal("engineer must not access another product repair team's conversation")
	}
	if ConversationService.CanAccessConversation(&conversations[2], operator) {
		t.Fatal("engineer must not access another tenant's conversation")
	}
	list, paging, err := ConversationService.ListConversations(
		1, engineer.ID, request.AgentConversationFilterPending, "", &sqls.Paging{Page: 1, Limit: 20}, operator,
	)
	if err != nil || paging.Total != 1 || len(list) != 1 || list[0].ID != conversations[0].ID {
		t.Fatalf("scoped pending conversation list=%+v paging=%+v err=%v", list, paging, err)
	}
	if _, err := MessageService.ValidateConversationSender(conversations[1].ID, enums.IMSenderTypeAgent, operator, nil); err == nil {
		t.Fatal("engineer must not send to another product repair team's conversation")
	}
	productBUser := models.User{Username: "product-b-engineer", Status: enums.StatusOk}
	if err := db.Create(&productBUser).Error; err != nil {
		t.Fatalf("create product B engineer: %v", err)
	}
	productBProfile := models.AgentProfile{
		TenantID: 1, UserID: productBUser.ID, TeamID: teamB.ID, AgentCode: "ENG-B", DisplayName: "产品 B 工程师", Status: enums.StatusOk,
	}
	if err := db.Create(&productBProfile).Error; err != nil {
		t.Fatalf("create product B profile: %v", err)
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID: 1, TeamID: teamB.ID, UserID: productBUser.ID, DispatchWeight: 1, DispatchEnabled: true, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create product B membership: %v", err)
	}
	if _, err := ConversationService.resolveAssignmentTeam(db, &conversations[0], &productBProfile); err == nil {
		t.Fatal("product A conversation must not be assigned to a product B engineer")
	}
	foreignProfile := productBProfile
	foreignProfile.TenantID = 2
	if _, err := ConversationService.resolveAssignmentTeam(db, &conversations[0], &foreignProfile); err == nil {
		t.Fatal("conversation must not be assigned across tenants")
	}
	if teamID, err := ConversationService.resolveAssignmentTeam(db, &conversations[0], &profile); err != nil || teamID != teamA.ID {
		t.Fatalf("valid product team assignment team=%d err=%v", teamID, err)
	}
}

func TestConversationAssignmentDoesNotInferTeamFromLegacyProfileTeamID(t *testing.T) {
	db := setupConversationAccessScopeDB(t)
	profile := models.AgentProfile{
		TenantID: 0, UserID: 101, TeamID: 99, AgentCode: "LEGACY-ENG", DisplayName: "旧档案工程师", Status: enums.StatusOk,
	}
	conversation := models.Conversation{TenantID: 0, CurrentTeamID: 0, Status: enums.IMConversationStatusPending}

	teamID, err := ConversationService.resolveAssignmentTeam(db, &conversation, &profile)
	if err != nil {
		t.Fatalf("resolveAssignmentTeam() error = %v", err)
	}
	if teamID != 0 {
		t.Fatalf("legacy AgentProfile.TeamID must not grant runtime team scope, got team=%d", teamID)
	}
}

func TestPartnerRealtimeAccessExpiresWithSupplierAuthorization(t *testing.T) {
	db := setupConversationAccessScopeDB(t)
	now := time.Now()
	conversation := models.Conversation{TenantID: 1, ProductID: 11, Status: enums.IMConversationStatusActive, LastActiveAt: now}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	ticket := models.Ticket{
		TenantID: 1, ProductID: 11, ConversationID: conversation.ID, TicketNo: "PARTNER-WS-1", Title: "供应商实时授权", Status: enums.TicketStatusSupplierSupport,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	company := models.PartnerCompany{TenantID: 1, PartnerNo: "SUP-WS", Name: "实时测试供应商", PartnerType: "module_supplier", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&company).Error; err != nil {
		t.Fatalf("create partner company: %v", err)
	}
	user := models.User{Username: "partner-ws-engineer", Status: enums.StatusOk}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create partner user: %v", err)
	}
	account := models.PartnerAccount{TenantID: 1, PartnerCompanyID: company.ID, UserID: user.ID, DisplayName: "供应商工程师", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create partner account: %v", err)
	}
	expiresAt := now.Add(time.Hour)
	collaboration := models.TicketSupplierCollaboration{
		TenantID: 1, TicketID: ticket.ID, ProductID: 11, ProductModuleID: 1, PartnerCompanyID: company.ID, PartnerAccountID: account.ID,
		Status: SupplierCollaborationProcessing, AuthorizationEnds: &expiresAt, InvitedAt: now, RecordStatus: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&collaboration).Error; err != nil {
		t.Fatalf("create supplier collaboration: %v", err)
	}
	joinedAt := now
	collaborationParticipant := models.TicketSupplierCollaborationParticipant{
		TenantID: 1, CollaborationID: collaboration.ID, PartnerCompanyID: company.ID, PartnerAccountID: account.ID, Role: SupplierParticipantRoleOwner,
		JoinedAt: joinedAt, Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&collaborationParticipant).Error; err != nil {
		t.Fatalf("create collaboration participant: %v", err)
	}
	conversationParticipant := models.ConversationParticipant{
		ConversationID: conversation.ID, ParticipantType: string(enums.IMParticipantTypePartner), ParticipantID: user.ID,
		ExternalParticipantID: "partner-account:1", JoinedAt: &joinedAt, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&conversationParticipant).Error; err != nil {
		t.Fatalf("create conversation participant: %v", err)
	}
	operator := &dto.AuthPrincipal{
		TenantID: 1, UserID: user.ID, Username: user.Username, DomainType: models.DomainTypePartner,
		SubjectType: models.SubjectTypePartnerAccount, PartnerAccountID: account.ID,
	}
	session := &ClientSession{Role: realtimeRoleAdmin, Principal: operator}
	if !ConversationService.CanAccessConversation(&conversation, operator) || !newWsService().canSubscribeConversation(session, conversation.ID) {
		t.Fatal("active supplier participant must access the collaboration conversation")
	}
	expiredAt := now.Add(-time.Minute)
	if err := db.Model(&collaboration).Update("authorization_ends", expiredAt).Error; err != nil {
		t.Fatalf("expire supplier authorization: %v", err)
	}
	if ConversationService.CanAccessConversation(&conversation, operator) {
		t.Fatal("expired supplier authorization must revoke conversation access")
	}
	if newWsService().canSubscribeConversation(session, conversation.ID) {
		t.Fatal("expired supplier authorization must revoke realtime subscription")
	}
}

func setupConversationAccessScopeDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "conversation-access.db")
	db, err := gorm.Open(sqlite.Open("file:"+dbPath+"?_busy_timeout=5000"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{}, &models.AgentTeam{}, &models.AgentProfile{}, &models.AgentTeamMember{},
		&models.Conversation{}, &models.ConversationParticipant{}, &models.Ticket{},
		&models.PartnerCompany{}, &models.PartnerAccount{}, &models.TicketSupplierCollaboration{},
		&models.TicketSupplierCollaborationParticipant{},
	); err != nil {
		t.Fatalf("migrate conversation access tables: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}
