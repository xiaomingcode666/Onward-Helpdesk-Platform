package services_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
)

func TestTicketSupplierCollaborationConcurrentInviteReusesActiveCollaboration(t *testing.T) {
	setupTicketTestDBWithMaxOpenConns(t, 1)
	operator := createTestOperator(t, "supplier-concurrent")
	tenant, product, productModel, device, serviceCode := createTicketAfterSalesFixture(t, "supplier-concurrent", enums.StatusOk)
	operator.TenantID = tenant.ID
	ensureTestProductRepairEngineer(t, tenant.ID, product.ID, operator.UserID)
	ticket, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:             "并发供应商协作",
		Description:       "重复邀请必须复用活动协作",
		TenantID:          tenant.ID,
		ProductID:         product.ID,
		ProductModelID:    productModel.ID,
		DeviceID:          device.ID,
		ServiceCodeID:     serviceCode.ID,
		CurrentAssigneeID: operator.UserID,
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	now := time.Now()
	conversation := &models.Conversation{
		TenantID: tenant.ID, ProductID: product.ID, ProductModelID: productModel.ID, DeviceID: device.ID,
		Status: enums.IMConversationStatusActive, CurrentAssigneeID: operator.UserID,
		LastMessageAt: now, LastActiveAt: now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := sqls.DB().Create(conversation).Error; err != nil {
		t.Fatalf("create linked conversation: %v", err)
	}
	if err := repositories.TicketRepository.Updates(sqls.DB(), ticket.ID, map[string]any{
		"status": enums.TicketStatusProcessing, "conversation_id": conversation.ID,
	}); err != nil {
		t.Fatalf("seed ticket status: %v", err)
	}
	company := &models.PartnerCompany{
		TenantID: tenant.ID, PartnerNo: "SUP-CONCURRENT", Name: "并发测试供应商", PartnerType: "module_supplier", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := sqls.DB().Create(company).Error; err != nil {
		t.Fatalf("create partner company: %v", err)
	}
	account := &models.PartnerAccount{
		TenantID: tenant.ID, PartnerCompanyID: company.ID, UserID: createTestUser(t, "supplier-concurrent-engineer"),
		DisplayName: "供应商并发工程师", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := sqls.DB().Create(account).Error; err != nil {
		t.Fatalf("create partner account: %v", err)
	}
	module := &models.ProductModule{
		TenantID: tenant.ID, ProductID: product.ID, ModuleCode: "CONCURRENT", Name: "并发模块",
		DefaultSupplierID: company.ID, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := repositories.ProductModuleRepository.Create(sqls.DB(), module); err != nil {
		t.Fatalf("create product module: %v", err)
	}

	const requestCount = 4
	results := make([]*services.TicketSupplierCollaborationAggregate, requestCount)
	errs := make([]error, requestCount)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			results[index], errs[index] = services.TicketSupplierCollaborationService.Invite(ticket.ID, dto.TicketSupplierInviteRequest{
				ProductModuleID: module.ID,
				Reason:          "需要供应商确认并发故障",
			}, operator)
		}(i)
	}
	close(start)
	wg.Wait()

	for i := range errs {
		if errs[i] != nil || results[i] == nil || results[i].Collaboration == nil {
			t.Fatalf("Invite()[%d] result=%+v error=%v", i, results[i], errs[i])
		}
		if results[i].Collaboration.ID != results[0].Collaboration.ID {
			t.Fatalf("Invite()[%d] collaboration=%d, want %d", i, results[i].Collaboration.ID, results[0].Collaboration.ID)
		}
	}
	var collaborationCount int64
	if err := sqls.DB().Model(&models.TicketSupplierCollaboration{}).
		Where("tenant_id = ? AND ticket_id = ? AND status IN ?", tenant.ID, ticket.ID, []string{
			services.SupplierCollaborationInvited, services.SupplierCollaborationAccepted, services.SupplierCollaborationProcessing,
		}).Count(&collaborationCount).Error; err != nil || collaborationCount != 1 {
		t.Fatalf("active collaboration count=%d err=%v", collaborationCount, err)
	}
	var scopeCount int64
	if err := sqls.DB().Model(&models.PartnerAuthorizationScope{}).
		Where("tenant_id = ? AND partner_account_id = ? AND resource_type = ? AND resource_id = ?", tenant.ID, account.ID, "ticket", ticket.ID).
		Count(&scopeCount).Error; err != nil || scopeCount != 1 {
		t.Fatalf("authorization scope count=%d err=%v", scopeCount, err)
	}
	var progressCount int64
	if err := sqls.DB().Model(&models.TicketProgress{}).
		Where("ticket_id = ? AND content LIKE ?", ticket.ID, "升级供应商协作：%").
		Count(&progressCount).Error; err != nil || progressCount != 1 {
		t.Fatalf("supplier progress count=%d err=%v", progressCount, err)
	}
	var eventCount int64
	if err := sqls.DB().Model(&models.Message{}).
		Where("conversation_id = ? AND sender_type = ? AND payload LIKE ?", conversation.ID, enums.IMSenderTypeSystem, "%supplier_collaboration_invited%").
		Count(&eventCount).Error; err != nil || eventCount != 1 {
		t.Fatalf("supplier conversation event count=%d err=%v", eventCount, err)
	}

	partner := &dto.AuthPrincipal{
		UserID:           account.UserID,
		TenantID:         tenant.ID,
		DomainType:       models.DomainTypePartner,
		SubjectType:      models.SubjectTypePartnerAccount,
		PartnerAccountID: account.ID,
	}
	acceptResults := make([]*services.TicketSupplierCollaborationAggregate, requestCount)
	acceptErrs := make([]error, requestCount)
	startAccept := make(chan struct{})
	wg = sync.WaitGroup{}
	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-startAccept
			acceptResults[index], acceptErrs[index] = services.TicketSupplierCollaborationService.Accept(results[0].Collaboration.ID, partner)
		}(i)
	}
	close(startAccept)
	wg.Wait()
	for i := range acceptErrs {
		if acceptErrs[i] != nil || acceptResults[i] == nil || acceptResults[i].Collaboration.ID != results[0].Collaboration.ID {
			t.Fatalf("Accept()[%d] result=%+v error=%v", i, acceptResults[i], acceptErrs[i])
		}
	}
	var acceptedProgressCount int64
	if err := sqls.DB().Model(&models.TicketProgress{}).
		Where("ticket_id = ? AND content = ?", ticket.ID, "供应商已接单").
		Count(&acceptedProgressCount).Error; err != nil || acceptedProgressCount != 1 {
		t.Fatalf("supplier accepted progress count=%d err=%v", acceptedProgressCount, err)
	}
	var acceptedEventCount int64
	if err := sqls.DB().Model(&models.Message{}).
		Where("conversation_id = ? AND sender_type = ? AND payload LIKE ?", conversation.ID, enums.IMSenderTypeSystem, "%supplier_collaboration_accepted%").
		Count(&acceptedEventCount).Error; err != nil || acceptedEventCount != 1 {
		t.Fatalf("supplier accepted event count=%d err=%v", acceptedEventCount, err)
	}

	resolveResults := make([]*services.TicketSupplierCollaborationAggregate, requestCount)
	resolveErrs := make([]error, requestCount)
	startResolve := make(chan struct{})
	wg = sync.WaitGroup{}
	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-startResolve
			resolveResults[index], resolveErrs[index] = services.TicketSupplierCollaborationService.Resolve(results[0].Collaboration.ID, "并发处理完成", partner)
		}(i)
	}
	close(startResolve)
	wg.Wait()
	resolveSuccesses := 0
	for i := range resolveErrs {
		if resolveErrs[i] != nil {
			if resolveResults[i] != nil {
				t.Fatalf("Resolve()[%d] returned both result=%+v and error=%v", i, resolveResults[i], resolveErrs[i])
			}
			continue
		}
		if resolveResults[i] == nil || resolveResults[i].Collaboration.Status != services.SupplierCollaborationResolved {
			t.Fatalf("Resolve()[%d] result=%+v", i, resolveResults[i])
		}
		resolveSuccesses++
	}
	if resolveSuccesses != 1 {
		t.Fatalf("concurrent Resolve() successes=%d, want 1", resolveSuccesses)
	}
	var resolvedProgressCount int64
	if err := sqls.DB().Model(&models.TicketProgress{}).
		Where("ticket_id = ? AND content = ?", ticket.ID, "供应商处理完成：并发处理完成").
		Count(&resolvedProgressCount).Error; err != nil || resolvedProgressCount != 1 {
		t.Fatalf("supplier resolved progress count=%d err=%v", resolvedProgressCount, err)
	}
	var resolvedMessageCount int64
	if err := sqls.DB().Model(&models.Message{}).
		Where("conversation_id = ? AND sender_type = ? AND content = ?", conversation.ID, enums.IMSenderTypePartner, "供应商处理完成：并发处理完成").
		Count(&resolvedMessageCount).Error; err != nil || resolvedMessageCount != 1 {
		t.Fatalf("supplier resolved message count=%d err=%v", resolvedMessageCount, err)
	}
}

func TestTicketSupplierCollaborationInviteSupportsProductlessKnowledgeTenant(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "supplier-knowledge-support")
	tenant, _, _, _, _ := createTicketAfterSalesFixture(t, "supplier-knowledge-support", enums.StatusOk)
	operator.TenantID = tenant.ID
	if err := repositories.TenantRepository.Updates(sqls.DB(), tenant.ID, map[string]any{
		"service_scene": models.TenantServiceSceneKnowledgeSupport,
	}); err != nil {
		t.Fatalf("configure knowledge-support tenant: %v", err)
	}

	now := time.Now()
	ticket := &models.Ticket{
		TicketNo:          "TK-KNOWLEDGE-SUPPLIER",
		Title:             "Knowledge support supplier review",
		Description:       "A productless ticket that requires external technical review",
		Source:            enums.TicketSourceManual,
		Status:            enums.TicketStatusProcessing,
		CurrentAssigneeID: operator.UserID,
		TenantID:          tenant.ID,
		AuditFields:       models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := repositories.TicketRepository.Create(sqls.DB(), ticket); err != nil {
		t.Fatalf("create productless ticket: %v", err)
	}
	company := &models.PartnerCompany{
		TenantID: tenant.ID, PartnerNo: "SUP-KNOWLEDGE", Name: "Knowledge Support Partner", PartnerType: "service_supplier", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := sqls.DB().Create(company).Error; err != nil {
		t.Fatalf("create partner company: %v", err)
	}
	account := &models.PartnerAccount{
		TenantID: tenant.ID, PartnerCompanyID: company.ID, UserID: createTestUser(t, "supplier-knowledge-support-account"),
		DisplayName: "Knowledge Support Partner Engineer", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := sqls.DB().Create(account).Error; err != nil {
		t.Fatalf("create partner account: %v", err)
	}

	result, err := services.TicketSupplierCollaborationService.Invite(ticket.ID, dto.TicketSupplierInviteRequest{
		PartnerCompanyID: company.ID,
		Reason:           "Review the cross-document diagnosis",
	}, operator)
	if err != nil {
		t.Fatalf("Invite() error = %v", err)
	}
	if result == nil || result.Collaboration == nil {
		t.Fatal("Invite() returned no collaboration")
	}
	if result.Collaboration.ProductID != 0 || result.Collaboration.ProductModuleID != 0 {
		t.Fatalf("productless collaboration gained product context: %+v", result.Collaboration)
	}
	if result.Collaboration.PartnerCompanyID != company.ID || result.Collaboration.PartnerAccountID != account.ID {
		t.Fatalf("unexpected supplier routing: %+v", result.Collaboration)
	}
	current := services.TicketService.Get(ticket.ID)
	if current == nil || current.Status != enums.TicketStatusSupplierSupport || current.ProductModuleID != 0 {
		t.Fatalf("unexpected ticket state after supplier invitation: %+v", current)
	}
	var progress models.TicketProgress
	if err := sqls.DB().Where("ticket_id = ? AND event_type = ?", ticket.ID, enums.TicketProgressEventEscalated).First(&progress).Error; err != nil {
		t.Fatalf("load supplier escalation progress: %v", err)
	}
	if progress.Content != "升级供应商协作："+company.Name {
		t.Fatalf("productless supplier progress = %q", progress.Content)
	}

	if err := repositories.TenantRepository.Updates(sqls.DB(), tenant.ID, map[string]any{
		"service_scene": models.TenantServiceSceneEquipmentAfterSales,
	}); err != nil {
		t.Fatalf("restore equipment after-sales scene: %v", err)
	}
	equipmentTicket := &models.Ticket{
		TicketNo:          "TK-EQUIPMENT-NO-PRODUCT-SUPPLIER",
		Title:             "Equipment ticket without product context",
		Source:            enums.TicketSourceManual,
		Status:            enums.TicketStatusProcessing,
		CurrentAssigneeID: operator.UserID,
		TenantID:          tenant.ID,
		AuditFields:       models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := repositories.TicketRepository.Create(sqls.DB(), equipmentTicket); err != nil {
		t.Fatalf("create equipment ticket without product: %v", err)
	}
	if _, err := services.TicketSupplierCollaborationService.Invite(equipmentTicket.ID, dto.TicketSupplierInviteRequest{
		PartnerCompanyID: company.ID,
		Reason:           "Must not bypass product-module routing",
	}, operator); err == nil {
		t.Fatal("equipment after-sales tenant bypassed product-module supplier routing")
	}
}

func TestSupplierCollaborationTimeoutEscalatesToSupervisorAndAllowsReinvite(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "supplier-timeout-engineer")
	tenant, product, productModel, device, serviceCode := createTicketAfterSalesFixture(t, "supplier-timeout", enums.StatusOk)
	operator.TenantID = tenant.ID
	teamID := ensureTestProductRepairEngineer(t, tenant.ID, product.ID, operator.UserID)
	supervisorID := createTestUser(t, "supplier-timeout-supervisor")
	ensureTestProductRepairEngineer(t, tenant.ID, product.ID, supervisorID)
	if err := repositories.AgentTeamRepository.Updates(sqls.DB(), teamID, map[string]any{"leader_user_id": supervisorID}); err != nil {
		t.Fatalf("set product repair supervisor: %v", err)
	}

	ticket, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:             "供应商未响应超时",
		Description:       "供应商没有在响应窗口内接单",
		TenantID:          tenant.ID,
		ProductID:         product.ID,
		ProductModelID:    productModel.ID,
		DeviceID:          device.ID,
		ServiceCodeID:     serviceCode.ID,
		CurrentAssigneeID: operator.UserID,
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	now := time.Now()
	conversation := &models.Conversation{
		TenantID: tenant.ID, ProductID: product.ID, ProductModelID: productModel.ID, DeviceID: device.ID,
		Status: enums.IMConversationStatusActive, CurrentAssigneeID: operator.UserID,
		LastMessageAt: now, LastActiveAt: now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := sqls.DB().Create(conversation).Error; err != nil {
		t.Fatalf("create linked conversation: %v", err)
	}
	if err := repositories.TicketRepository.Updates(sqls.DB(), ticket.ID, map[string]any{
		"status": enums.TicketStatusProcessing, "conversation_id": conversation.ID, "current_team_id": teamID,
	}); err != nil {
		t.Fatalf("seed ticket status: %v", err)
	}
	company := &models.PartnerCompany{
		TenantID: tenant.ID, PartnerNo: "SUP-TIMEOUT", Name: "超时测试供应商", PartnerType: "module_supplier", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := sqls.DB().Create(company).Error; err != nil {
		t.Fatalf("create partner company: %v", err)
	}
	account := &models.PartnerAccount{
		TenantID: tenant.ID, PartnerCompanyID: company.ID, UserID: createTestUser(t, "supplier-timeout-account"),
		DisplayName: "超时供应商工程师", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := sqls.DB().Create(account).Error; err != nil {
		t.Fatalf("create partner account: %v", err)
	}
	module := &models.ProductModule{
		TenantID: tenant.ID, ProductID: product.ID, ModuleCode: "TIMEOUT", Name: "超时模块",
		DefaultSupplierID: company.ID, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := repositories.ProductModuleRepository.Create(sqls.DB(), module); err != nil {
		t.Fatalf("create product module: %v", err)
	}
	collaboration, err := services.TicketSupplierCollaborationService.Invite(ticket.ID, dto.TicketSupplierInviteRequest{
		ProductModuleID: module.ID,
		Reason:          "需要供应商在响应窗口内确认",
	}, operator)
	if err != nil {
		t.Fatalf("Invite() error = %v", err)
	}
	oldInviteAt := now.Add(-2 * time.Hour)
	if err := sqls.DB().Model(&models.TicketSupplierCollaboration{}).
		Where("id = ?", collaboration.Collaboration.ID).
		Update("invited_at", oldInviteAt).Error; err != nil {
		t.Fatalf("age supplier collaboration: %v", err)
	}

	handled, err := services.TicketSupplierCollaborationService.EscalateUnresponsiveInvitations(time.Hour, 10)
	if err != nil || handled != 1 {
		t.Fatalf("EscalateUnresponsiveInvitations() = (%d, %v), want one handled", handled, err)
	}
	handled, err = services.TicketSupplierCollaborationService.EscalateUnresponsiveInvitations(time.Hour, 10)
	if err != nil || handled != 0 {
		t.Fatalf("idempotent EscalateUnresponsiveInvitations() = (%d, %v), want none", handled, err)
	}

	closed := repositories.TicketSupplierCollaborationRepository.Get(sqls.DB(), collaboration.Collaboration.ID)
	if closed == nil || closed.Status != services.SupplierCollaborationTimeout || closed.ResolvedAt == nil {
		t.Fatalf("supplier collaboration was not timed out: %+v", closed)
	}
	current := services.TicketService.Get(ticket.ID)
	if current == nil || current.CurrentAssigneeID != supervisorID || current.Status != enums.TicketStatusProcessing || current.AcceptedAt == nil {
		t.Fatalf("ticket was not escalated to product repair supervisor: %+v", current)
	}
	var timeoutProgressCount int64
	if err := sqls.DB().Model(&models.TicketProgress{}).
		Where("ticket_id = ? AND metadata_json LIKE ?", ticket.ID, "%supplier_timeout%").
		Count(&timeoutProgressCount).Error; err != nil || timeoutProgressCount != 1 {
		t.Fatalf("supplier timeout progress count=%d err=%v", timeoutProgressCount, err)
	}
	var timeoutMessageCount int64
	if err := sqls.DB().Model(&models.Message{}).
		Where("conversation_id = ? AND sender_type = ? AND payload LIKE ?", conversation.ID, enums.IMSenderTypeSystem, "%supplier_collaboration_timeout%").
		Count(&timeoutMessageCount).Error; err != nil || timeoutMessageCount != 1 {
		t.Fatalf("supplier timeout customer-visible message count=%d err=%v", timeoutMessageCount, err)
	}
	participant := repositories.ConversationParticipantRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("conversation_id", conversation.ID).
		Eq("participant_type", enums.IMParticipantTypePartner).
		Eq("participant_id", account.UserID))
	if participant == nil || participant.Status != enums.StatusDisabled || participant.LeftAt == nil {
		t.Fatalf("supplier participant should be disabled after timeout: %+v", participant)
	}

	reinvited, err := services.TicketSupplierCollaborationService.Invite(ticket.ID, dto.TicketSupplierInviteRequest{
		ProductModuleID:  module.ID,
		PartnerAccountID: account.ID,
		Reason:           "超时后重新邀请供应商确认",
	}, operator)
	if err != nil {
		t.Fatalf("reinvite after timeout error = %v", err)
	}
	if reinvited.Collaboration.ID == collaboration.Collaboration.ID {
		t.Fatalf("reinvite reused timed out collaboration: %+v", reinvited.Collaboration)
	}
}

func TestTicketSupplierCollaborationInviteIsScopedAndIdempotent(t *testing.T) {
	config.SetCurrent(&config.Config{Storage: config.StorageConfig{
		Default: enums.AssetProviderLocal,
		Local: config.LocalStorageConfig{
			Root:    t.TempDir(),
			BaseURL: "https://files.example.test",
		},
	}})
	setupTicketTestDB(t)
	operator := createTestOperator(t, "supplier-invite")
	tenant, product, productModel, device, serviceCode := createTicketAfterSalesFixture(t, "supplier-invite", enums.StatusOk)
	operator.TenantID = tenant.ID

	ticket, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:          "电源模块故障",
		Description:    "现场工程师需要供应商协助",
		TenantID:       tenant.ID,
		ProductID:      product.ID,
		ProductModelID: productModel.ID,
		DeviceID:       device.ID,
		ServiceCodeID:  serviceCode.ID,
		FaultCode:      "PWR-001",
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	now := time.Now()
	conversation := &models.Conversation{
		TenantID: tenant.ID, ProductID: product.ID, ProductModelID: productModel.ID, DeviceID: device.ID,
		Status: enums.IMConversationStatusActive, CurrentAssigneeID: operator.UserID,
		LastMessageAt: now, LastActiveAt: now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := sqls.DB().Create(conversation).Error; err != nil {
		t.Fatalf("create linked conversation: %v", err)
	}
	if err := repositories.TicketRepository.Updates(sqls.DB(), ticket.ID, map[string]any{"status": enums.TicketStatusProcessing, "conversation_id": conversation.ID}); err != nil {
		t.Fatalf("seed ticket status: %v", err)
	}

	company := &models.PartnerCompany{
		TenantID:    tenant.ID,
		PartnerNo:   "SUP-PWR",
		Name:        "电源模块供应商",
		PartnerType: "module_supplier",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := sqls.DB().Create(company).Error; err != nil {
		t.Fatalf("create partner company: %v", err)
	}
	account := &models.PartnerAccount{
		TenantID:         tenant.ID,
		PartnerCompanyID: company.ID,
		UserID:           createTestUser(t, "supplier-engineer"),
		DisplayName:      "供应商工程师",
		Status:           enums.StatusOk,
		AuditFields:      models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := sqls.DB().Create(account).Error; err != nil {
		t.Fatalf("create partner account: %v", err)
	}
	module := &models.ProductModule{
		TenantID:          tenant.ID,
		ProductID:         product.ID,
		ModuleCode:        "PWR",
		Name:              "电源模块",
		DefaultSupplierID: company.ID,
		Status:            enums.StatusOk,
		AuditFields:       models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := repositories.ProductModuleRepository.Create(sqls.DB(), module); err != nil {
		t.Fatalf("create product module: %v", err)
	}

	first, err := services.TicketSupplierCollaborationService.Invite(ticket.ID, dto.TicketSupplierInviteRequest{
		ProductModuleID: module.ID,
		Reason:          "需要原厂确认电源板故障",
		Visibility:      []string{"repair_progress"},
	}, operator)
	if err != nil {
		t.Fatalf("Invite() error = %v", err)
	}
	second, err := services.TicketSupplierCollaborationService.Invite(ticket.ID, dto.TicketSupplierInviteRequest{
		ProductModuleID: module.ID,
		Reason:          "重复请求不应重复邀请",
		Visibility:      []string{"meeting"},
	}, operator)
	if err != nil {
		t.Fatalf("duplicate Invite() error = %v", err)
	}
	if first.Collaboration.ID <= 0 || first.Collaboration.ID != second.Collaboration.ID {
		t.Fatalf("expected idempotent collaboration, first=%+v second=%+v", first.Collaboration, second.Collaboration)
	}
	if first.Collaboration.PartnerCompanyID != company.ID || first.Collaboration.PartnerAccountID != account.ID {
		t.Fatalf("unexpected supplier routing: %+v", first.Collaboration)
	}
	var supplierInvitedMessageCount int64
	if err := sqls.DB().Model(&models.Message{}).
		Where("conversation_id = ? AND sender_type = ? AND payload LIKE ?", conversation.ID, enums.IMSenderTypeSystem, "%supplier_collaboration_invited%").
		Count(&supplierInvitedMessageCount).Error; err != nil || supplierInvitedMessageCount != 1 {
		t.Fatalf("supplier invitation conversation event count=%d err=%v", supplierInvitedMessageCount, err)
	}
	if current := services.TicketService.Get(ticket.ID); current == nil || current.Status != enums.TicketStatusSupplierSupport {
		t.Fatalf("expected supplier_support ticket, got %+v", current)
	}
	initialParticipant := repositories.ConversationParticipantRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("conversation_id", conversation.ID).
		Eq("participant_type", enums.IMParticipantTypePartner).
		Eq("participant_id", account.UserID))
	if initialParticipant == nil || initialParticipant.Status != enums.StatusOk {
		t.Fatalf("expected invited supplier in conversation, got %+v", initialParticipant)
	}
	initialCollaborationParticipant := repositories.TicketSupplierCollaborationRepository.FindActiveParticipant(sqls.DB(), tenant.ID, first.Collaboration.ID, account.ID)
	if initialCollaborationParticipant == nil || initialCollaborationParticipant.Role != services.SupplierParticipantRoleOwner {
		t.Fatalf("expected invited supplier to be collaboration owner, got %+v", initialCollaborationParticipant)
	}
	var scope models.PartnerAuthorizationScope
	if err := sqls.DB().Where("partner_account_id = ? AND resource_type = ? AND resource_id = ?", account.ID, "ticket", ticket.ID).First(&scope).Error; err != nil {
		t.Fatalf("expected scoped partner authorization: %v", err)
	}
	mergedCollaboration := repositories.TicketSupplierCollaborationRepository.Get(sqls.DB(), first.Collaboration.ID)
	if mergedCollaboration == nil || !strings.Contains(mergedCollaboration.VisibilityJSON, `"repair_progress"`) || !strings.Contains(mergedCollaboration.VisibilityJSON, `"meeting"`) {
		t.Fatalf("duplicate invitation should merge supplier visibility, got %+v", mergedCollaboration)
	}
	if !strings.Contains(scope.ScopeRuleJSON, `"repair_progress"`) || !strings.Contains(scope.ScopeRuleJSON, `"meeting"`) {
		t.Fatalf("duplicate invitation should refresh supplier authorization scope, got %s", scope.ScopeRuleJSON)
	}

	partner := &dto.AuthPrincipal{
		UserID:           account.UserID,
		TenantID:         tenant.ID,
		DomainType:       models.DomainTypePartner,
		SubjectType:      models.SubjectTypePartnerAccount,
		PartnerAccountID: account.ID,
		Permissions: []string{
			constants.PermissionPartnerMemberView.Code,
			constants.PermissionPartnerMemberInvite.Code,
			constants.PermissionPartnerMemberUpdate.Code,
		},
	}
	profile, err := services.TicketSupplierCollaborationService.PartnerProfile(partner)
	if err != nil {
		t.Fatalf("PartnerProfile() error = %v", err)
	}
	if profile.Company == nil || profile.Company.ID != company.ID || profile.Account == nil || profile.Account.ID != account.ID {
		t.Fatalf("unexpected partner profile: %+v", profile)
	}
	accounts, accountCompany, err := services.TicketSupplierCollaborationService.ListPartnerCompanyAccounts(partner)
	if err != nil {
		t.Fatalf("ListPartnerCompanyAccounts() error = %v", err)
	}
	if accountCompany == nil || accountCompany.ID != company.ID || len(accounts) != 1 || accounts[0].Account.ID != account.ID {
		t.Fatalf("unexpected partner account scope: company=%+v accounts=%+v", accountCompany, accounts)
	}
	createdAccount, err := services.TicketSupplierCollaborationService.CreatePartnerAccount(dto.PartnerAccountCreateRequest{
		Username:    "supplier-engineer-2",
		DisplayName: "供应商工程师二号",
		Email:       "supplier-2@example.com",
		Languages:   []string{"zh-CN", "en-US"},
		RoleCode:    services.PartnerRoleEngineer,
	}, partner)
	if err != nil {
		t.Fatalf("CreatePartnerAccount() error = %v", err)
	}
	if createdAccount.InitialPassword == "" || createdAccount.Account.Account.PartnerCompanyID != company.ID || len(createdAccount.Account.Roles) != 1 || createdAccount.Account.Roles[0] != services.PartnerRoleEngineer {
		t.Fatalf("unexpected created partner account: %+v", createdAccount)
	}
	firstPartnerLogin, err := services.AuthService.Login(request.LoginRequest{
		Username:   createdAccount.Account.User.Username,
		Password:   createdAccount.InitialPassword,
		DomainType: models.DomainTypePartner,
	}, config.AuthConfig{TokenTTLHours: 2}, "127.0.0.1", "go-test")
	if err != nil {
		t.Fatalf("create first active partner login: %v", err)
	}
	secondPartnerLogin, err := services.AuthService.Login(request.LoginRequest{
		Username:   createdAccount.Account.User.Username,
		Password:   createdAccount.InitialPassword,
		DomainType: models.DomainTypePartner,
	}, config.AuthConfig{TokenTTLHours: 2}, "127.0.0.1", "go-test")
	if err != nil {
		t.Fatalf("create second active partner login: %v", err)
	}
	activeSessionExpiry := now.Add(2 * time.Hour)
	expiredSessionExpiry := now.Add(-time.Hour)
	activeSessions := []models.LoginSession{
		{
			UserID: createdAccount.Account.Account.UserID, Token: "ak_supplier_expired", ClientType: constants.ClientTypeAdminWeb,
			DomainType: models.DomainTypePartner, TenantID: tenant.ID, SubjectType: models.SubjectTypePartnerAccount,
			SubjectID: createdAccount.Account.Account.ID, ExpiredAt: expiredSessionExpiry,
			AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
		},
		{
			UserID: account.UserID, Token: "ak_supplier_admin_active", ClientType: constants.ClientTypeAdminWeb,
			DomainType: models.DomainTypePartner, TenantID: tenant.ID, SubjectType: models.SubjectTypePartnerAccount,
			SubjectID: account.ID, ExpiredAt: activeSessionExpiry,
			AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
		},
	}
	if err := sqls.DB().Create(&activeSessions).Error; err != nil {
		t.Fatalf("create partner login sessions: %v", err)
	}
	updatedAccount, err := services.TicketSupplierCollaborationService.UpdatePartnerAccount(createdAccount.Account.Account.ID, dto.PartnerAccountUpdateRequest{
		DisplayName: "供应商工程师二号",
		Email:       "supplier-2@example.com",
		Languages:   []string{"en-US"},
		RoleCode:    services.PartnerRoleEngineer,
		Status:      int(enums.StatusDisabled),
	}, partner)
	if err != nil || updatedAccount.Account.Status != enums.StatusDisabled {
		t.Fatalf("UpdatePartnerAccount() = %+v, %v", updatedAccount, err)
	}
	for _, token := range []string{firstPartnerLogin.AccessToken, secondPartnerLogin.AccessToken} {
		var session models.LoginSession
		if err := sqls.DB().First(&session, "token = ?", token).Error; err != nil || session.RevokedAt == nil {
			t.Fatalf("active partner session %q was not revoked: session=%+v err=%v", token, session, err)
		}
	}
	for _, token := range []string{"ak_supplier_expired", "ak_supplier_admin_active"} {
		var session models.LoginSession
		if err := sqls.DB().First(&session, "token = ?", token).Error; err != nil || session.RevokedAt != nil {
			t.Fatalf("unrelated or expired session %q was unexpectedly revoked: session=%+v err=%v", token, session, err)
		}
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/partner/v1/profile", nil)
	ctx.Request.Header.Set("Authorization", "Bearer "+firstPartnerLogin.AccessToken)
	if _, err := services.AuthService.Authenticate(ctx); err == nil {
		t.Fatal("revoked partner token remained usable after account disable")
	}
	var sessionCountBeforeRelogin int64
	if err := sqls.DB().Model(&models.LoginSession{}).Where("user_id = ?", createdAccount.Account.Account.UserID).Count(&sessionCountBeforeRelogin).Error; err != nil {
		t.Fatalf("count disabled partner sessions before relogin: %v", err)
	}
	if _, err := services.AuthService.Login(request.LoginRequest{
		Username:   createdAccount.Account.User.Username,
		Password:   createdAccount.InitialPassword,
		DomainType: models.DomainTypePartner,
	}, config.AuthConfig{TokenTTLHours: 2}, "127.0.0.1", "go-test"); err == nil {
		t.Fatal("disabled partner account logged in again through the partner portal")
	}
	var sessionCountAfterRelogin int64
	if err := sqls.DB().Model(&models.LoginSession{}).Where("user_id = ?", createdAccount.Account.Account.UserID).Count(&sessionCountAfterRelogin).Error; err != nil {
		t.Fatalf("count disabled partner sessions after relogin: %v", err)
	}
	if sessionCountAfterRelogin != sessionCountBeforeRelogin {
		t.Fatalf("disabled partner relogin created a session: before=%d after=%d", sessionCountBeforeRelogin, sessionCountAfterRelogin)
	}
	if _, err := services.TicketSupplierCollaborationService.UpdatePartnerAccount(account.ID, dto.PartnerAccountUpdateRequest{
		DisplayName: account.DisplayName,
		RoleCode:    services.PartnerRoleAdmin,
		Status:      int(enums.StatusDisabled),
	}, partner); err == nil {
		t.Fatal("expected current supplier administrator self-disable to be rejected")
	}
	updatedAccount, err = services.TicketSupplierCollaborationService.UpdatePartnerAccount(createdAccount.Account.Account.ID, dto.PartnerAccountUpdateRequest{
		DisplayName: "供应商工程师二号",
		Email:       "supplier-2@example.com",
		Languages:   []string{"en-US"},
		RoleCode:    services.PartnerRoleEngineer,
		Status:      int(enums.StatusOk),
	}, partner)
	if err != nil || updatedAccount.Account.Status != enums.StatusOk {
		t.Fatalf("re-enable partner account = %+v, %v", updatedAccount, err)
	}
	assigned, err := services.TicketSupplierCollaborationService.Assign(first.Collaboration.ID, updatedAccount.Account.ID, partner)
	if err != nil || assigned.Collaboration.PartnerAccountID != updatedAccount.Account.ID {
		t.Fatalf("Assign() = %+v, %v", assigned, err)
	}
	if err := sqls.DB().First(initialParticipant, initialParticipant.ID).Error; err != nil || initialParticipant.Status != enums.StatusOk {
		t.Fatalf("previous supplier owner should remain an active conversation participant: participant=%+v err=%v", initialParticipant, err)
	}
	previousOwner := repositories.TicketSupplierCollaborationRepository.FindActiveParticipant(sqls.DB(), tenant.ID, first.Collaboration.ID, account.ID)
	newOwner := repositories.TicketSupplierCollaborationRepository.FindActiveParticipant(sqls.DB(), tenant.ID, first.Collaboration.ID, updatedAccount.Account.ID)
	if previousOwner == nil || previousOwner.Role != services.SupplierParticipantRoleMember || newOwner == nil || newOwner.Role != services.SupplierParticipantRoleOwner {
		t.Fatalf("unexpected collaboration ownership after assignment: previous=%+v new=%+v", previousOwner, newOwner)
	}
	engineerPartner := &dto.AuthPrincipal{
		UserID:           updatedAccount.Account.UserID,
		TenantID:         tenant.ID,
		DomainType:       models.DomainTypePartner,
		SubjectType:      models.SubjectTypePartnerAccount,
		PartnerAccountID: updatedAccount.Account.ID,
		Permissions: []string{
			constants.PermissionPartnerMemberView.Code,
			constants.PermissionPartnerMemberInvite.Code,
		},
	}
	if err := repositories.TicketSupplierCollaborationRepository.UpdateParticipant(sqls.DB(), newOwner.ID, map[string]any{
		"status":     enums.StatusDisabled,
		"updated_at": now,
	}); err != nil {
		t.Fatalf("temporarily disable owner participant: %v", err)
	}
	ownerItems, err := services.TicketSupplierCollaborationService.ListForPartner(engineerPartner)
	if err != nil {
		t.Fatalf("list collaborations for owner without active participant: %v", err)
	}
	ownerVisible := false
	for i := range ownerItems {
		if ownerItems[i].Collaboration.ID == first.Collaboration.ID {
			ownerVisible = true
			break
		}
	}
	if !ownerVisible {
		t.Fatal("current supplier owner lost list access without an active participant row")
	}
	if _, err := services.TicketSupplierCollaborationService.PartnerTicketDetail(first.Collaboration.ID, engineerPartner); err != nil {
		t.Fatalf("current supplier owner lost detail access without an active participant row: %v", err)
	}
	if err := repositories.TicketSupplierCollaborationRepository.UpdateParticipant(sqls.DB(), newOwner.ID, map[string]any{
		"status":     enums.StatusOk,
		"updated_at": now,
	}); err != nil {
		t.Fatalf("restore owner participant: %v", err)
	}
	invitedByEngineer, err := services.TicketSupplierCollaborationService.CreatePartnerAccount(dto.PartnerAccountCreateRequest{
		Username:    "supplier-engineer-3",
		DisplayName: "供应商工程师三号",
		Email:       "supplier-3@example.com",
		Languages:   []string{"zh-CN"},
		RoleCode:    services.PartnerRoleEngineer,
	}, engineerPartner)
	if err != nil {
		t.Fatalf("engineer CreatePartnerAccount() error = %v", err)
	}
	if invitedByEngineer.InitialPassword == "" || invitedByEngineer.Account.Account.PartnerCompanyID != company.ID || len(invitedByEngineer.Account.Roles) != 1 || invitedByEngineer.Account.Roles[0] != services.PartnerRoleEngineer {
		t.Fatalf("unexpected engineer-created partner account: %+v", invitedByEngineer)
	}
	if _, err := services.TicketSupplierCollaborationService.CreatePartnerAccount(dto.PartnerAccountCreateRequest{
		Username:    "supplier-admin-by-engineer",
		DisplayName: "越权供应商管理员",
		RoleCode:    services.PartnerRoleAdmin,
	}, engineerPartner); err == nil {
		t.Fatal("expected supplier engineer creating an administrator to be rejected")
	}
	if _, err := services.TicketSupplierCollaborationService.UpdatePartnerAccount(invitedByEngineer.Account.Account.ID, dto.PartnerAccountUpdateRequest{
		DisplayName: invitedByEngineer.Account.Account.DisplayName,
		RoleCode:    services.PartnerRoleEngineer,
		Status:      int(enums.StatusDisabled),
	}, engineerPartner); err == nil {
		t.Fatal("expected supplier engineer updating another account to be rejected")
	}
	if _, err := services.TicketSupplierCollaborationService.AddPartnerProgress(first.Collaboration.ID, "尚未接单不应写进展", engineerPartner); err == nil {
		t.Fatal("expected progress before acceptance to be rejected")
	}
	if _, err := services.TicketSupplierCollaborationService.Accept(first.Collaboration.ID, engineerPartner); err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	if _, err := services.TicketSupplierCollaborationService.Accept(first.Collaboration.ID, engineerPartner); err != nil {
		t.Fatalf("idempotent Accept() error = %v", err)
	}
	var supplierAcceptedMessageCount int64
	if err := sqls.DB().Model(&models.Message{}).
		Where("conversation_id = ? AND sender_type = ? AND payload LIKE ?", conversation.ID, enums.IMSenderTypeSystem, "%supplier_collaboration_accepted%").
		Count(&supplierAcceptedMessageCount).Error; err != nil || supplierAcceptedMessageCount != 1 {
		t.Fatalf("supplier accepted conversation event count=%d err=%v", supplierAcceptedMessageCount, err)
	}
	thirdPartner := &dto.AuthPrincipal{
		UserID:           invitedByEngineer.Account.Account.UserID,
		TenantID:         tenant.ID,
		DomainType:       models.DomainTypePartner,
		SubjectType:      models.SubjectTypePartnerAccount,
		PartnerAccountID: invitedByEngineer.Account.Account.ID,
	}
	companyItemsBeforeJoin, err := services.TicketSupplierCollaborationService.ListForPartner(thirdPartner)
	if err != nil || len(companyItemsBeforeJoin) != 0 {
		t.Fatalf("same-company supplier ticket list before participant join = %+v, %v", companyItemsBeforeJoin, err)
	}
	if companyDetailBeforeJoin, err := services.TicketSupplierCollaborationService.PartnerTicketDetail(first.Collaboration.ID, thirdPartner); err == nil {
		t.Fatalf("same-company supplier detail before participant join = %+v, want forbidden", companyDetailBeforeJoin)
	}
	if _, err := services.TicketSupplierCollaborationService.AddPartnerProgress(first.Collaboration.ID, "未加入成员不应写进展", thirdPartner); err == nil {
		t.Fatal("expected same-company supplier without participant access to be unable to add progress")
	}
	participantDetail, err := services.TicketSupplierCollaborationService.AddParticipant(first.Collaboration.ID, thirdPartner.PartnerAccountID, partner)
	if err != nil {
		t.Fatalf("AddParticipant() error = %v", err)
	}
	if participantDetail.Collaboration.Collaboration.ID != first.Collaboration.ID || len(participantDetail.Collaboration.Participants) != 3 {
		t.Fatalf("expected three supplier collaboration participants, got %+v", participantDetail.Collaboration.Participants)
	}
	thirdItems, err := services.TicketSupplierCollaborationService.ListForPartner(thirdPartner)
	if err != nil || len(thirdItems) != 1 || thirdItems[0].Collaboration.ID != first.Collaboration.ID {
		t.Fatalf("supplier collaboration member ticket list = %+v, %v", thirdItems, err)
	}
	if _, err := services.TicketSupplierCollaborationService.AddPartnerProgress(first.Collaboration.ID, "三号工程师：已核对原厂测试记录", thirdPartner); err != nil {
		t.Fatalf("supplier collaboration member AddPartnerProgress() error = %v", err)
	}
	detail, err := services.TicketSupplierCollaborationService.AddPartnerProgress(first.Collaboration.ID, "已完成电源板交叉验证，准备复测", engineerPartner)
	if err != nil {
		t.Fatalf("AddPartnerProgress() error = %v", err)
	}
	if detail.Collaboration.Collaboration.ID != first.Collaboration.ID || len(detail.Progresses) < 4 {
		t.Fatalf("unexpected supplier detail after progress: %+v", detail)
	}
	partnerMessage := repositories.MessageRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("conversation_id", conversation.ID).
		Eq("sender_type", enums.IMSenderTypePartner).
		Desc("id"))
	if partnerMessage == nil || partnerMessage.SenderID != engineerPartner.UserID || partnerMessage.Content != "已完成电源板交叉验证，准备复测" {
		t.Fatalf("supplier progress was not synchronized to conversation: %+v", partnerMessage)
	}
	conversation = repositories.ConversationRepository.Get(sqls.DB(), conversation.ID)
	if conversation == nil || conversation.AgentUnreadCount != 2 || conversation.CustomerUnreadCount != 2 {
		t.Fatalf("supplier reply should be unread for engineer and customer: %+v", conversation)
	}
	assignedParticipant := repositories.ConversationParticipantRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("conversation_id", conversation.ID).
		Eq("participant_type", enums.IMParticipantTypePartner).
		Eq("participant_id", engineerPartner.UserID))
	if assignedParticipant == nil || assignedParticipant.Status != enums.StatusOk {
		t.Fatalf("expected assigned supplier participant to be active, got %+v", assignedParticipant)
	}
	if len(detail.Messages) < 4 || detail.Messages[len(detail.Messages)-1].Message.ID != partnerMessage.ID {
		t.Fatalf("supplier detail should expose the scoped collaboration conversation: %+v", detail.Messages)
	}
	audioAsset := &models.Asset{
		TenantID:   tenant.ID,
		AssetID:    "partner_audio_" + time.Now().Format("150405.000000000"),
		Provider:   enums.AssetProviderLocal,
		StorageKey: "audio/partner-voice.webm",
		Filename:   "partner-voice.webm",
		FileSize:   1024,
		MimeType:   "audio/webm",
		Status:     enums.AssetStatusSuccess,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := repositories.AssetRepository.Create(sqls.DB(), audioAsset); err != nil {
		t.Fatalf("create supplier audio asset: %v", err)
	}
	audioDetail, err := services.TicketSupplierCollaborationService.AddPartnerMessage(first.Collaboration.ID, dto.PartnerTicketProgressCreateRequest{
		MessageType:     string(enums.IMMessageTypeAudio),
		AssetID:         audioAsset.AssetID,
		DurationSeconds: 17,
	}, engineerPartner)
	if err != nil {
		t.Fatalf("AddPartnerMessage(audio) error = %v", err)
	}
	audioMessage := audioDetail.Messages[len(audioDetail.Messages)-1].Message
	if audioMessage.MessageType != enums.IMMessageTypeAudio || !strings.Contains(audioMessage.Payload, `"assetId":"`+audioAsset.AssetID+`"`) || !strings.Contains(audioMessage.Payload, `"durationSeconds":17`) {
		t.Fatalf("supplier audio message was not synchronized correctly: %+v", audioMessage)
	}
	audioAsset = repositories.AssetRepository.Get(sqls.DB(), audioAsset.ID)
	if audioAsset == nil || audioAsset.ConversationID != conversation.ID {
		t.Fatalf("legacy supplier audio asset was not bound to the collaboration conversation: %+v", audioAsset)
	}
	detail = audioDetail

	foreignAudio := &models.Asset{
		TenantID:   tenant.ID + 1,
		AssetID:    "foreign_partner_audio_" + time.Now().Format("150405.000000000"),
		Provider:   enums.AssetProviderLocal,
		StorageKey: "audio/foreign-partner-voice.webm",
		Filename:   "foreign-partner-voice.webm",
		FileSize:   1024,
		MimeType:   "audio/webm",
		Status:     enums.AssetStatusSuccess,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := repositories.AssetRepository.Create(sqls.DB(), foreignAudio); err != nil {
		t.Fatalf("create foreign supplier audio asset: %v", err)
	}
	if _, err := services.TicketSupplierCollaborationService.AddPartnerMessage(first.Collaboration.ID, dto.PartnerTicketProgressCreateRequest{
		MessageType: string(enums.IMMessageTypeAudio),
		AssetID:     foreignAudio.AssetID,
	}, engineerPartner); err == nil {
		t.Fatal("cross-tenant supplier audio asset unexpectedly accepted")
	}
	assignedCollaborationParticipant := repositories.TicketSupplierCollaborationRepository.FindActiveParticipant(
		sqls.DB(), tenant.ID, first.Collaboration.ID, engineerPartner.PartnerAccountID,
	)
	if assignedCollaborationParticipant == nil {
		t.Fatal("expected assigned supplier collaboration participant")
	}
	assignedJoinedAt := assignedCollaborationParticipant.JoinedAt
	removedDetail, err := services.TicketSupplierCollaborationService.RemoveParticipant(first.Collaboration.ID, thirdPartner.PartnerAccountID, partner)
	if err != nil {
		t.Fatalf("RemoveParticipant() error = %v", err)
	}
	if len(removedDetail.Collaboration.Participants) != 3 {
		t.Fatalf("removed participant history should be retained: %+v", removedDetail.Collaboration.Participants)
	}
	removedParticipant := repositories.TicketSupplierCollaborationRepository.FindParticipantAnyStatus(
		sqls.DB(), tenant.ID, first.Collaboration.ID, thirdPartner.PartnerAccountID,
	)
	if removedParticipant == nil || removedParticipant.Status != enums.StatusDisabled || removedParticipant.LeftAt == nil {
		t.Fatalf("removed supplier member should retain exit state: %+v", removedParticipant)
	}
	removedAt := *removedParticipant.LeftAt
	visibleAfterRemoval, err := services.TicketSupplierCollaborationService.ListForPartner(thirdPartner)
	if err != nil {
		t.Fatalf("list collaborations for removed supplier member: %v", err)
	}
	removedMemberVisible := false
	for i := range visibleAfterRemoval {
		if visibleAfterRemoval[i].Collaboration.ID == first.Collaboration.ID {
			removedMemberVisible = true
			break
		}
	}
	if removedMemberVisible {
		t.Fatalf("same-company supplier retained list access after participant removal: %+v", visibleAfterRemoval)
	}
	if removedMemberDetail, err := services.TicketSupplierCollaborationService.PartnerTicketDetail(first.Collaboration.ID, thirdPartner); err == nil {
		t.Fatalf("same-company supplier retained detail access after participant removal: %+v", removedMemberDetail)
	}
	if _, err := services.TicketSupplierCollaborationService.AddPartnerProgress(first.Collaboration.ID, "退出后不应继续回复", thirdPartner); err == nil {
		t.Fatal("expected removed supplier member reply to be rejected")
	}
	if conversation, err := services.TicketSupplierCollaborationService.ResolvePartnerMessageUploadConversation(first.Collaboration.ID, thirdPartner); err == nil {
		t.Fatalf("removed supplier member retained upload access: %+v", conversation)
	}
	adminItemsAfterRemoval, err := services.TicketSupplierCollaborationService.ListForPartner(partner)
	if err != nil {
		t.Fatalf("list collaborations for supplier administrator after member removal: %v", err)
	}
	adminVisibleAfterRemoval := false
	for i := range adminItemsAfterRemoval {
		if adminItemsAfterRemoval[i].Collaboration.ID == first.Collaboration.ID {
			adminVisibleAfterRemoval = true
			break
		}
	}
	if !adminVisibleAfterRemoval {
		t.Fatal("supplier administrator lost participant-scoped list access after member removal")
	}
	enterpriseDetail, err := services.TicketSupplierCollaborationService.AddEnterpriseProgress(
		ticket.ID,
		first.Collaboration.ID,
		"企业工程师：请补充复测时的输入电压范围",
		operator,
	)
	if err != nil {
		t.Fatalf("AddEnterpriseProgress() error = %v", err)
	}
	if len(enterpriseDetail.Progresses) != len(removedDetail.Progresses)+1 || enterpriseDetail.Progresses[len(enterpriseDetail.Progresses)-1].Progress.Content != "企业工程师：请补充复测时的输入电压范围" {
		t.Fatalf("unexpected enterprise supplier collaboration detail: %+v", enterpriseDetail)
	}
	partnerDetail, err := services.TicketSupplierCollaborationService.PartnerTicketDetail(first.Collaboration.ID, engineerPartner)
	if err != nil || len(partnerDetail.Progresses) != len(enterpriseDetail.Progresses) {
		t.Fatalf("supplier did not receive enterprise collaboration message: %+v, %v", partnerDetail, err)
	}
	adminDetail, err := services.TicketSupplierCollaborationService.PartnerTicketDetail(first.Collaboration.ID, partner)
	if err != nil || adminDetail.Collaboration.Collaboration.PartnerCompanyID != company.ID {
		t.Fatalf("supplier administrator participant-scoped detail = %+v, %v", adminDetail, err)
	}
	partnerConversations, err := services.TicketSupplierCollaborationService.ListConversationsForPartner(partner)
	if err != nil || len(partnerConversations) != 1 || partnerConversations[0].Collaboration.ID != first.Collaboration.ID {
		t.Fatalf("supplier administrator participant-scoped conversations = %+v, %v", partnerConversations, err)
	}
	meeting := &models.MeetingRoomJitsi{
		ID:        "supplier-scoped-meeting",
		TenantID:  tenant.ID,
		TicketID:  strconv.FormatInt(ticket.ID, 10),
		RoomName:  "supplier-scoped-room",
		Status:    "active",
		CreatedBy: strconv.FormatInt(operator.UserID, 10),
		BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
	if err := sqls.DB().Create(meeting).Error; err != nil {
		t.Fatalf("create supplier scoped meeting: %v", err)
	}
	partnerMeetings, err := services.TicketSupplierCollaborationService.ListPortalMeetings(nil, "active", partner)
	if err != nil || len(partnerMeetings) != 1 || partnerMeetings[0].CollaborationID != first.Collaboration.ID {
		t.Fatalf("supplier administrator participant-scoped meetings = %+v, %v", partnerMeetings, err)
	}
	otherAdmin, err := services.TicketSupplierCollaborationService.CreatePartnerAccount(dto.PartnerAccountCreateRequest{
		Username:    "supplier-admin-2",
		DisplayName: "供应商管理员二号",
		Email:       "supplier-admin-2@example.com",
		Languages:   []string{"zh-CN"},
		RoleCode:    services.PartnerRoleAdmin,
	}, partner)
	if err != nil {
		t.Fatalf("CreatePartnerAccount(admin) error = %v", err)
	}
	otherAdminPartner := &dto.AuthPrincipal{
		UserID:           otherAdmin.Account.Account.UserID,
		TenantID:         tenant.ID,
		DomainType:       models.DomainTypePartner,
		SubjectType:      models.SubjectTypePartnerAccount,
		PartnerAccountID: otherAdmin.Account.Account.ID,
		Permissions:      []string{constants.PermissionTicketView.Code, constants.PermissionMeetingView.Code},
	}
	otherAdminItems, err := services.TicketSupplierCollaborationService.ListForPartner(otherAdminPartner)
	if err != nil || len(otherAdminItems) != 1 || otherAdminItems[0].Collaboration.ID != first.Collaboration.ID {
		t.Fatalf("same-company supplier administrator ticket list = %+v, %v", otherAdminItems, err)
	}
	if otherAdminDetail, err := services.TicketSupplierCollaborationService.PartnerTicketDetail(first.Collaboration.ID, otherAdminPartner); err != nil || otherAdminDetail.Collaboration.Collaboration.ID != first.Collaboration.ID {
		t.Fatalf("same-company supplier administrator detail access = %+v, %v", otherAdminDetail, err)
	}
	if _, err := services.TicketSupplierCollaborationService.Assign(first.Collaboration.ID, account.ID, otherAdminPartner); err != nil {
		t.Fatalf("same-company supplier administrator assign access = %v", err)
	}
	otherAdminConversations, err := services.TicketSupplierCollaborationService.ListConversationsForPartner(otherAdminPartner)
	if err != nil || len(otherAdminConversations) != 1 || otherAdminConversations[0].Collaboration.ID != first.Collaboration.ID {
		t.Fatalf("same-company supplier administrator conversations = %+v, %v", otherAdminConversations, err)
	}
	otherAdminMeetings, err := services.TicketSupplierCollaborationService.ListPortalMeetings(nil, "active", otherAdminPartner)
	if err != nil || len(otherAdminMeetings) != 1 || otherAdminMeetings[0].CollaborationID != first.Collaboration.ID {
		t.Fatalf("same-company supplier administrator meetings = %+v, %v", otherAdminMeetings, err)
	}
	outsiderCompany := &models.PartnerCompany{
		TenantID:    tenant.ID,
		PartnerNo:   "SUP-OUTSIDE",
		Name:        "其他供应商",
		PartnerType: "module_supplier",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := sqls.DB().Create(outsiderCompany).Error; err != nil {
		t.Fatalf("create outside partner company: %v", err)
	}
	outsiderAccount := &models.PartnerAccount{
		TenantID:         tenant.ID,
		PartnerCompanyID: outsiderCompany.ID,
		UserID:           createTestUser(t, "outside-supplier-admin"),
		DisplayName:      "其他供应商管理员",
		Status:           enums.StatusOk,
		AuditFields:      models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := sqls.DB().Create(outsiderAccount).Error; err != nil {
		t.Fatalf("create outside partner account: %v", err)
	}
	outsider := &dto.AuthPrincipal{
		UserID:           outsiderAccount.UserID,
		TenantID:         tenant.ID,
		DomainType:       models.DomainTypePartner,
		SubjectType:      models.SubjectTypePartnerAccount,
		PartnerAccountID: outsiderAccount.ID,
	}
	if _, err := services.TicketSupplierCollaborationService.PartnerTicketDetail(first.Collaboration.ID, outsider); err == nil {
		t.Fatal("expected another supplier company to be denied")
	}
	if _, err := services.TicketSupplierCollaborationService.Resolve(first.Collaboration.ID, "确认更换电源板并完成复测", engineerPartner); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	resolvedDetail, err := services.TicketSupplierCollaborationService.PartnerTicketDetail(first.Collaboration.ID, engineerPartner)
	if err != nil || resolvedDetail.Collaboration.Collaboration.ID != first.Collaboration.ID {
		t.Fatalf("resolved supplier detail should remain readable as history: %+v, %v", resolvedDetail, err)
	}
	visibleAfterResolve, err := services.TicketSupplierCollaborationService.ListForPartner(engineerPartner)
	if err != nil {
		t.Fatalf("list supplier tickets after resolve: %v", err)
	}
	resolvedVisible := false
	for i := range visibleAfterResolve {
		if visibleAfterResolve[i].Collaboration.ID == first.Collaboration.ID {
			resolvedVisible = true
			break
		}
	}
	if !resolvedVisible {
		t.Fatalf("resolved supplier collaboration should remain visible as history: %+v", visibleAfterResolve)
	}
	conversationsAfterResolve, err := services.TicketSupplierCollaborationService.ListConversationsForPartner(engineerPartner)
	if err != nil {
		t.Fatalf("list supplier conversations after resolve: %v", err)
	}
	resolvedConversationVisible := false
	for i := range conversationsAfterResolve {
		if conversationsAfterResolve[i].Collaboration.ID == first.Collaboration.ID {
			resolvedConversationVisible = true
			break
		}
	}
	if !resolvedConversationVisible {
		t.Fatalf("resolved supplier collaboration should remain visible in conversation history: %+v", conversationsAfterResolve)
	}
	meetingsAfterResolve, err := services.TicketSupplierCollaborationService.ListPortalMeetings(nil, "all", engineerPartner)
	if err != nil {
		t.Fatalf("list supplier meetings after resolve: %v", err)
	}
	resolvedMeetingVisible := false
	for i := range meetingsAfterResolve {
		if meetingsAfterResolve[i].CollaborationID == first.Collaboration.ID {
			resolvedMeetingVisible = true
			break
		}
	}
	if !resolvedMeetingVisible {
		t.Fatalf("resolved supplier collaboration should retain meeting history: %+v", meetingsAfterResolve)
	}
	if meetingStatus, err := services.TicketSupplierCollaborationService.GetMeetingStatus(nil, first.Collaboration.ID, meeting.ID, engineerPartner); err != nil || meetingStatus.Status != meeting.Status {
		t.Fatalf("resolved supplier collaboration should retain read-only meeting status: %+v, %v", meetingStatus, err)
	}
	if _, err := services.TicketSupplierCollaborationService.JoinMeeting(nil, first.Collaboration.ID, meeting.ID, engineerPartner); err == nil {
		t.Fatal("resolved supplier collaboration must not rejoin historical meetings")
	}
	if _, err := services.TicketSupplierCollaborationService.AddPartnerProgress(first.Collaboration.ID, "结案后不应继续回复", engineerPartner); err == nil {
		t.Fatal("resolved supplier collaboration must not allow ticket updates")
	}
	resolutionMessage := repositories.MessageRepository.FindLastValidByConversationIDAndSenderType(sqls.DB(), conversation.ID, enums.IMSenderTypePartner)
	if resolutionMessage == nil || resolutionMessage.Content != "供应商处理完成：确认更换电源板并完成复测" || !strings.Contains(resolutionMessage.Payload, "partner") {
		t.Fatalf("supplier final resolution was not synchronized to conversation: %+v", resolutionMessage)
	}
	if current := services.TicketService.Get(ticket.ID); current == nil || current.Status != enums.TicketStatusProcessing {
		t.Fatalf("expected ticket to return to processing, got %+v", current)
	}
	if _, err := services.TicketSupplierCollaborationService.ResolveForTicket(ticket.ID+1000, first.Collaboration.ID, "重复结论", operator); err == nil {
		t.Fatal("expected collaboration route mismatch to be rejected")
	}
	if err := sqls.DB().First(&scope, scope.ID).Error; err != nil {
		t.Fatalf("reload authorization scope: %v", err)
	}
	if scope.Status != enums.StatusDisabled {
		t.Fatalf("expected supplier ticket authorization to be disabled, got %v", scope.Status)
	}
	if err := sqls.DB().First(assignedParticipant, assignedParticipant.ID).Error; err != nil || assignedParticipant.Status != enums.StatusDisabled || assignedParticipant.LeftAt == nil {
		t.Fatalf("resolved supplier participant should leave conversation: participant=%+v err=%v", assignedParticipant, err)
	}
	if err := sqls.DB().First(initialParticipant, initialParticipant.ID).Error; err != nil || initialParticipant.Status != enums.StatusDisabled || initialParticipant.LeftAt == nil {
		t.Fatalf("all supplier co-workers should leave realtime conversation after resolution: participant=%+v err=%v", initialParticipant, err)
	}
	if err := sqls.DB().First(assignedCollaborationParticipant, assignedCollaborationParticipant.ID).Error; err != nil || !assignedCollaborationParticipant.JoinedAt.Equal(assignedJoinedAt) {
		t.Fatalf("supplier replies and resolution should preserve the first joined time: participant=%+v err=%v", assignedCollaborationParticipant, err)
	}
	if err := sqls.DB().First(removedParticipant, removedParticipant.ID).Error; err != nil || removedParticipant.LeftAt == nil || !removedParticipant.LeftAt.Equal(removedAt) {
		t.Fatalf("resolving collaboration should not overwrite an earlier member exit time: participant=%+v err=%v", removedParticipant, err)
	}
	repeated, err := services.TicketSupplierCollaborationService.Invite(ticket.ID, dto.TicketSupplierInviteRequest{
		ProductModuleID:  module.ID,
		PartnerAccountID: engineerPartner.PartnerAccountID,
		Reason:           "复测后故障复发，需要原厂再次确认",
	}, operator)
	if err != nil {
		t.Fatalf("repeated Invite() after resolution error = %v", err)
	}
	if repeated.Collaboration.ID == first.Collaboration.ID {
		t.Fatalf("repeated invitation reused resolved collaboration: %+v", repeated.Collaboration)
	}
	if _, err := services.TicketSupplierCollaborationService.Accept(repeated.Collaboration.ID, partner); err != nil {
		t.Fatalf("same-company supplier administrator Accept() repeated collaboration error = %v", err)
	}
	if _, err := services.TicketSupplierCollaborationService.Accept(repeated.Collaboration.ID, engineerPartner); err != nil {
		t.Fatalf("assigned supplier Accept() repeated collaboration error = %v", err)
	}
	if _, err := services.TicketSupplierCollaborationService.Resolve(repeated.Collaboration.ID, "复发原因已确认并完成二次复测", engineerPartner); err != nil {
		t.Fatalf("Resolve() repeated collaboration error = %v", err)
	}
	var resolvedCount int64
	if err := sqls.DB().Model(&models.TicketSupplierCollaboration{}).
		Where("tenant_id = ? AND ticket_id = ? AND product_module_id = ? AND partner_company_id = ? AND status = ? AND record_status <> ?",
			tenant.ID, ticket.ID, module.ID, company.ID, services.SupplierCollaborationResolved, enums.StatusDeleted).
		Count(&resolvedCount).Error; err != nil {
		t.Fatalf("count resolved collaborations: %v", err)
	}
	if resolvedCount != 2 {
		t.Fatalf("resolved collaboration count = %d, want 2", resolvedCount)
	}
	expiring, err := services.TicketSupplierCollaborationService.Invite(ticket.ID, dto.TicketSupplierInviteRequest{
		ProductModuleID:     module.ID,
		PartnerAccountID:    engineerPartner.PartnerAccountID,
		Reason:              "验证供应商临时授权到期回收",
		AuthorizationEndsAt: time.Now().Add(250 * time.Millisecond).Format(time.RFC3339Nano),
	}, operator)
	if err != nil {
		t.Fatalf("create expiring collaboration: %v", err)
	}
	if expiring.Collaboration.AuthorizationEnds == nil || expiring.Collaboration.AuthorizationEnds.Sub(time.Now()) > time.Second {
		t.Fatalf("explicit supplier authorization end was not persisted: %+v", expiring.Collaboration.AuthorizationEnds)
	}
	if _, err := services.TicketSupplierCollaborationService.Accept(expiring.Collaboration.ID, engineerPartner); err != nil {
		t.Fatalf("accept expiring collaboration: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	expiredCount, err := services.TicketSupplierCollaborationService.ExpireAuthorizations(100)
	if err != nil {
		t.Fatalf("expire supplier authorizations: %v", err)
	}
	if expiredCount != 1 {
		t.Fatalf("expired supplier authorization count = %d, want 1", expiredCount)
	}
	expiredCollaboration := repositories.TicketSupplierCollaborationRepository.Get(sqls.DB(), expiring.Collaboration.ID)
	if expiredCollaboration == nil || expiredCollaboration.Status != services.SupplierCollaborationExpired || expiredCollaboration.ResolvedAt == nil {
		t.Fatalf("supplier collaboration was not finalized after authorization expiry: %+v", expiredCollaboration)
	}
	visibleAfterExpiry, err := services.TicketSupplierCollaborationService.ListForPartner(engineerPartner)
	if err != nil {
		t.Fatalf("list supplier tickets after expiry: %v", err)
	}
	expiredVisible := false
	for i := range visibleAfterExpiry {
		if visibleAfterExpiry[i].Collaboration.ID == expiring.Collaboration.ID {
			expiredVisible = true
			break
		}
	}
	if !expiredVisible {
		t.Fatalf("expired supplier authorization should remain visible as read-only history: %+v", visibleAfterExpiry)
	}
	if expiredDetail, err := services.TicketSupplierCollaborationService.PartnerTicketDetail(expiring.Collaboration.ID, engineerPartner); err != nil || expiredDetail.Collaboration.Collaboration.ID != expiring.Collaboration.ID {
		t.Fatalf("expired supplier authorization should allow read-only detail history: %+v, %v", expiredDetail, err)
	}
	if _, err := services.TicketSupplierCollaborationService.AddPartnerProgress(expiring.Collaboration.ID, "到期后不应写入", engineerPartner); err == nil {
		t.Fatal("expired supplier authorization must not allow ticket updates")
	}
	expiredParticipant := repositories.TicketSupplierCollaborationRepository.FindParticipantAnyStatus(sqls.DB(), tenant.ID, expiring.Collaboration.ID, engineerPartner.PartnerAccountID)
	if expiredParticipant == nil || expiredParticipant.Status != enums.StatusDisabled || expiredParticipant.LeftAt == nil {
		t.Fatalf("expired supplier participant was not revoked: %+v", expiredParticipant)
	}
	expiredScope := repositories.TicketSupplierCollaborationRepository.FindAuthorizationScope(sqls.DB(), tenant.ID, ticket.ID, expiring.Collaboration.ID, engineerPartner.PartnerAccountID)
	if expiredScope == nil || expiredScope.Status != enums.StatusDisabled || expiredScope.ExpiredAt == nil {
		t.Fatalf("expired supplier scope was not revoked: %+v", expiredScope)
	}
	if current := services.TicketService.Get(ticket.ID); current == nil || current.Status != enums.TicketStatusProcessing {
		t.Fatalf("ticket did not return to processing after supplier authorization expiry: %+v", current)
	}
	var expiryAuditCount int64
	if err := sqls.DB().Model(&models.AuditLog{}).
		Where("tenant_id = ? AND resource_type = ? AND resource_id = ? AND action = ?", tenant.ID, "supplier_collaboration", strconv.FormatInt(expiring.Collaboration.ID, 10), "ticket.supplier_collaboration.authorization_expired").
		Count(&expiryAuditCount).Error; err != nil || expiryAuditCount != 1 {
		t.Fatalf("supplier authorization expiry audit count=%d err=%v", expiryAuditCount, err)
	}
	if _, err := services.TicketSupplierCollaborationService.Invite(ticket.ID, dto.TicketSupplierInviteRequest{
		ProductModuleID:     module.ID,
		PartnerAccountID:    engineerPartner.PartnerAccountID,
		Reason:              "过去时间不得创建授权",
		AuthorizationEndsAt: time.Now().Add(-time.Minute).Format(time.RFC3339Nano),
	}, operator); err == nil {
		t.Fatal("expected a past supplier authorization end to fail")
	}
	if _, err := services.TicketSupplierCollaborationService.Invite(ticket.ID, dto.TicketSupplierInviteRequest{
		ProductModuleID:     module.ID,
		PartnerAccountID:    engineerPartner.PartnerAccountID,
		Reason:              "超过九十天不得创建授权",
		AuthorizationEndsAt: time.Now().AddDate(0, 0, 91).Format(time.RFC3339Nano),
	}, operator); err == nil {
		t.Fatal("expected an overlong supplier authorization to fail")
	}
}
