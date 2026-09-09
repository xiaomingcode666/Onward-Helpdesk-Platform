package services_test

import (
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/openidentity"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestConversationHumanDispatchAIHoldKeepsAIServingAndNotifiesRepairTeam(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "1")
	aiAgent.HandoffMode = enums.AIAgentHandoffModeAIHoldAndNotify
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusAIServing)

	result, err := services.ConversationHumanDispatchService.HandoffByAI(conversation.ID, aiAgent, "用户要求转人工")
	if err != nil {
		t.Fatalf("HandoffByAI() error = %v", err)
	}
	if result == nil || result.Decision != services.HandoffDecisionAIHold {
		t.Fatalf("expected ai_hold decision, got %+v", result)
	}

	current := services.ConversationService.Get(conversation.ID)
	if current.Status != enums.IMConversationStatusAIServing {
		t.Fatalf("expected conversation to stay AI serving, got status=%d", current.Status)
	}
	if current.HandoffAt == nil {
		t.Fatalf("expected handoffAt to be recorded")
	}

	message := services.MessageService.FindOne(sqls.NewCnd().Eq("conversation_id", conversation.ID).Desc("id"))
	if message == nil {
		t.Fatalf("expected off-hours notice message")
	}
	if message.SenderType != enums.IMSenderTypeAI || message.Content != services.HandoffAIHoldMessage {
		t.Fatalf("unexpected AI hold message: %+v", message)
	}
}

func TestConversationHumanDispatchRejectsTenantlessLegacyHandoff(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	aiAgent := models.AIAgent{
		Name:        "旧入口AI",
		ServiceMode: enums.IMConversationServiceModeAIFirst,
		TeamIDs:     "1",
		HandoffMode: enums.AIAgentHandoffModeDefaultTeamPool,
		Status:      enums.StatusOk,
	}
	if err := db.Create(&aiAgent).Error; err != nil {
		t.Fatalf("create legacy ai agent error = %v", err)
	}
	conversation := models.Conversation{
		AIAgentID:     aiAgent.ID,
		ChannelID:     1,
		CustomerID:    1,
		CustomerName:  "旧访客",
		Status:        enums.IMConversationStatusAIServing,
		ServiceMode:   enums.IMConversationServiceModeAIFirst,
		LastMessageAt: time.Now(),
		LastActiveAt:  time.Now(),
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create legacy conversation error = %v", err)
	}

	result, err := services.ConversationHumanDispatchService.HandoffByAI(conversation.ID, aiAgent, "旧入口转人工")
	if err == nil || !strings.Contains(err.Error(), "tenant") {
		t.Fatalf("expected tenant guard error, got result=%+v err=%v", result, err)
	}
	current := services.ConversationService.Get(conversation.ID)
	if current == nil || current.HandoffAt != nil || current.Status != enums.IMConversationStatusAIServing {
		t.Fatalf("tenantless handoff must not mutate old conversation path: %+v", current)
	}
}

func TestConversationHumanDispatchCustomerRequestIgnoresHistoricalScheduleAndAssignsAvailableMember(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	product := models.Product{
		TenantID: 1,
		Code:     "LEGACY-SCHEDULE-PRODUCT",
		Name:     "历史时间表产品",
		Status:   enums.StatusOk,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product error = %v", err)
	}
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "1")
	if err := db.Model(&models.AIAgent{}).Where("id = ?", aiAgent.ID).Update("product_id", product.ID).Error; err != nil {
		t.Fatalf("scope ai agent to product error = %v", err)
	}
	aiAgent.ProductID = product.ID
	createHumanDispatchTeam(t, db, 1, "售后支持组")
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Updates(map[string]any{
		"product_id":        product.ID,
		"team_type":         services.AgentTeamTypeProductRepair,
		"schedule_enforced": true,
	}).Error; err != nil {
		t.Fatalf("enable published schedule enforcement: %v", err)
	}
	createHumanDispatchAgentProfile(t, db, 101, 1, enums.ServiceStatusIdle, 3, true, enums.StatusOk)
	if err := db.Create(&models.AgentTeamSchedule{
		TenantID:      1,
		TeamID:        1,
		UserID:        101,
		PublishStatus: services.AgentTeamSchedulePublishPublished,
		StartAt:       time.Now().Add(-2 * time.Hour),
		EndAt:         time.Now().Add(-time.Hour),
		Status:        enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create expired schedule error = %v", err)
	}
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusAIServing)
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Update("product_id", product.ID).Error; err != nil {
		t.Fatalf("scope conversation to product error = %v", err)
	}
	external := openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceGuest,
		ExternalID:     "guest-customer-off-hours",
		ExternalName:   "测试访客",
	}
	ensureHumanDispatchCustomer(t, db, conversation.CustomerID, external.ExternalName)
	if err := db.Create(&models.CustomerIdentity{CustomerID: conversation.CustomerID, ExternalSource: external.ExternalSource, ExternalID: external.ExternalID}).Error; err != nil {
		t.Fatalf("create customer identity error = %v", err)
	}

	if err := services.ConversationHumanDispatchService.RequestByCustomer(conversation.ID, external, "客户明确请求人工", "request-off-hours"); err != nil {
		t.Fatalf("RequestByCustomer() error = %v", err)
	}
	current := services.ConversationService.Get(conversation.ID)
	if current.Status != enums.IMConversationStatusPending || current.CurrentTeamID != 1 || current.CurrentAssigneeID != 101 {
		t.Fatalf("expected direct pending assignment from available member, got status=%d team=%d assignee=%d", current.Status, current.CurrentTeamID, current.CurrentAssigneeID)
	}
	if current.HandoffAt == nil || current.HandoffReason != "客户明确请求人工" {
		t.Fatalf("expected handoff metadata, got at=%v reason=%q", current.HandoffAt, current.HandoffReason)
	}
	ticket := services.TicketService.FindOne(sqls.NewCnd().Eq("conversation_id", conversation.ID))
	if ticket == nil {
		t.Fatal("customer handoff should create a linked ticket")
	}
	if ticket.ProductID != product.ID || ticket.CurrentTeamID != 1 || ticket.CurrentAssigneeID != 101 {
		t.Errorf("product ticket lost its product team assignment: %+v", ticket)
	}
	if ticket.Status != enums.TicketStatusAssigned || ticket.DispatchDeferredUntil != nil {
		t.Errorf("product ticket should be synced to the assigned engineer: %+v", ticket)
	}
	if ticket.LastDispatchFailureReason != "" {
		t.Errorf("product ticket kept stale dispatch failure reason: %+v", ticket)
	}
	var attempts []models.TicketDispatchAttempt
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id ASC").Find(&attempts).Error; err != nil {
		t.Fatalf("find dispatch attempts error = %v", err)
	}
	if len(attempts) != 1 {
		t.Errorf("dispatch attempt count = %d, want 1 pending attempt: %+v", len(attempts), attempts)
	} else if attempts[0].TeamID != 1 || attempts[0].AssigneeID != 101 || attempts[0].Outcome != "pending" || attempts[0].Reason != "自动分配" || attempts[0].EndedAt != nil {
		t.Errorf("member assignment was not audited completely: %+v", attempts[0])
	}
	message := services.MessageService.FindOne(sqls.NewCnd().Eq("conversation_id", conversation.ID).Desc("id"))
	if message == nil || message.Content != services.HandoffWaitingMessage {
		t.Fatalf("expected waiting notice, got %+v", message)
	}
	messages := services.MessageService.Find(sqls.NewCnd().Eq("conversation_id", conversation.ID).Asc("id"))
	if len(messages) != 2 || messages[0].SenderType != enums.IMSenderTypeSystem || messages[0].Content != services.HandoffCustomerRequestedMessageForLocale("zh-CN") {
		t.Fatalf("expected visible customer handoff request before queue result, got %+v", messages)
	}
	if err := services.ConversationHumanDispatchService.RequestByCustomer(conversation.ID, external, "重复请求人工", "request-off-hours-retry"); err != nil {
		t.Fatalf("repeated RequestByCustomer() error = %v", err)
	}
	clientMsgID := "customer-handoff-request:" + strconv.FormatInt(conversation.ID, 10)
	if count := services.MessageService.Count(sqls.NewCnd().Eq("conversation_id", conversation.ID).Eq("client_msg_id", clientMsgID)); count != 1 {
		t.Fatalf("customer handoff request notice should be idempotent, got %d", count)
	}
	var attemptCount int64
	if err := db.Model(&models.TicketDispatchAttempt{}).Where("ticket_id = ?", ticket.ID).Count(&attemptCount).Error; err != nil {
		t.Fatalf("count repeated dispatch attempts error = %v", err)
	} else if attemptCount != 1 {
		t.Fatalf("repeated customer handoff should keep one pending dispatch attempt, got %d", attemptCount)
	}
}

func TestConversationCreateExistingConversationEntersHumanQueueWhenTenantAIDisabled(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "")
	if err := db.Model(&models.Tenant{}).Where("id = ?", 1).Update("ai_enabled", false).Error; err != nil {
		t.Fatalf("disable tenant AI error = %v", err)
	}
	external := openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     "existing-ai-customer",
		ExternalName:   "已有 AI 客户",
	}
	if err := db.Create(&models.CustomerIdentity{
		CustomerID: 1, ExternalSource: external.ExternalSource, ExternalID: external.ExternalID, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create customer identity error = %v", err)
	}
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusAIServing)
	if err := db.Create(&models.ConversationParticipant{
		ConversationID:        conversation.ID,
		ParticipantType:       string(enums.IMParticipantTypeCustomer),
		ExternalParticipantID: "user:" + external.ExternalID,
		Status:                enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create conversation participant error = %v", err)
	}

	matched, err := services.ConversationService.CreateWithContext(external, 0, aiAgent.ID, request.CreateOrMatchConversationRequest{
		ContextTenantID: 1,
	})
	if err != nil {
		t.Fatalf("reuse conversation with tenant AI disabled error = %v", err)
	}
	if matched == nil || matched.ID != conversation.ID {
		t.Fatalf("expected existing conversation to be reused, got %+v", matched)
	}
	current := services.ConversationService.Get(conversation.ID)
	if current == nil || current.Status != enums.IMConversationStatusPending || current.HandoffAt == nil {
		t.Fatalf("existing conversation did not enter human queue: %+v", current)
	}
	if ticket := services.TicketService.FindOne(sqls.NewCnd().Eq("conversation_id", conversation.ID)); ticket == nil {
		t.Fatal("existing conversation should receive a human-service ticket")
	}
}

func TestCustomerMessageEntersHumanQueueWhenTenantAIDisabled(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "")
	if err := db.Model(&models.Tenant{}).Where("id = ?", 1).Update("ai_enabled", false).Error; err != nil {
		t.Fatalf("disable tenant AI error = %v", err)
	}
	external := openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     "direct-message-customer",
		ExternalName:   "直接发消息客户",
	}
	if err := db.Create(&models.CustomerIdentity{
		CustomerID: 1, ExternalSource: external.ExternalSource, ExternalID: external.ExternalID, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create customer identity error = %v", err)
	}
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusAIServing)
	if err := db.Create(&models.ConversationParticipant{
		ConversationID:        conversation.ID,
		ParticipantType:       string(enums.IMParticipantTypeCustomer),
		ExternalParticipantID: "user:" + external.ExternalID,
		Status:                enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create conversation participant error = %v", err)
	}

	message, err := services.MessageService.SendCustomerMessage(
		conversation.ID,
		"direct-message-when-ai-disabled",
		enums.IMMessageTypeText,
		"设备还是无法启动",
		"",
		external,
	)
	if err != nil {
		t.Fatalf("send customer message with tenant AI disabled error = %v", err)
	}
	if message == nil || message.Content != "设备还是无法启动" {
		t.Fatalf("customer message was not persisted: %+v", message)
	}
	current := services.ConversationService.Get(conversation.ID)
	if current == nil || current.Status != enums.IMConversationStatusPending || current.HandoffAt == nil {
		t.Fatalf("customer message did not enter human queue: %+v", current)
	}
	if ticket := services.TicketService.FindOne(sqls.NewCnd().Eq("conversation_id", conversation.ID)); ticket == nil {
		t.Fatal("customer message should create a human-service ticket")
	}
}

func TestConversationHumanDispatchCustomerRequestRejectsAIOnlyWorkflow(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	if err := db.AutoMigrate(&models.AIWorkflow{}, &models.AIWorkflowVersion{}); err != nil {
		t.Fatalf("auto migrate workflows error = %v", err)
	}
	workflow, version, err := services.AIWorkflowService.EnsurePlatformDeviceAIOnlyWorkflowDB(db)
	if err != nil {
		t.Fatalf("EnsurePlatformDeviceAIOnlyWorkflowDB() error = %v", err)
	}
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIOnly, "")
	if err := db.Model(&models.AIAgent{}).Where("id = ?", aiAgent.ID).Updates(map[string]any{
		"workflow_id":         workflow.ID,
		"workflow_version_id": version.ID,
	}).Error; err != nil {
		t.Fatalf("bind AI-only workflow error = %v", err)
	}
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusAIServing)
	external := openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceGuest,
		ExternalID:     "guest-ai-only",
		ExternalName:   "AI 专属客户",
	}
	ensureHumanDispatchCustomer(t, db, conversation.CustomerID, external.ExternalName)
	if err := db.Create(&models.CustomerIdentity{CustomerID: conversation.CustomerID, ExternalSource: external.ExternalSource, ExternalID: external.ExternalID}).Error; err != nil {
		t.Fatalf("create customer identity error = %v", err)
	}

	err = services.ConversationHumanDispatchService.RequestByCustomer(conversation.ID, external, "请求人工", "request-ai-only")
	if err == nil || !strings.Contains(err.Error(), "仅提供 AI 客服") {
		t.Fatalf("expected AI-only handoff rejection, got %v", err)
	}
	if _, err = services.ConversationHumanDispatchService.HandoffByAIWithRequestID(conversation.ID, aiAgent, "AI 尝试转人工", "ai-handoff-ai-only"); err == nil || !strings.Contains(err.Error(), "仅提供 AI 客服") {
		t.Fatalf("expected direct AI handoff rejection, got %v", err)
	}
	if _, err = services.ConversationHumanDispatchService.TryOffHoursHandoffByAIWithRequestID(conversation.ID, aiAgent, "AI 尝试非工作时间转人工", "ai-off-hours-ai-only"); err == nil || !strings.Contains(err.Error(), "仅提供 AI 客服") {
		t.Fatalf("expected off-hours AI handoff rejection, got %v", err)
	}
	current := services.ConversationService.Get(conversation.ID)
	if current.Status != enums.IMConversationStatusAIServing || current.HandoffAt != nil {
		t.Fatalf("AI-only rejection must keep the conversation in AI service: %+v", current)
	}
	if count := services.MessageService.Count(sqls.NewCnd().Eq("conversation_id", conversation.ID)); count != 0 {
		t.Fatalf("AI-only rejection must not emit handoff messages, got %d", count)
	}
}

func TestConversationHumanDispatchRejectsTenantDefaultGeneralHandoffEvenWhenMisconfigured(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "1")
	if err := db.Model(&models.AIAgent{}).Where("id = ?", aiAgent.ID).Updates(map[string]any{
		"tenant_id":  int64(1),
		"product_id": int64(0),
		"source":     services.TenantDefaultAIAgentSource,
	}).Error; err != nil {
		t.Fatalf("mark tenant default agent: %v", err)
	}
	aiAgent = *services.AIAgentService.Get(aiAgent.ID)
	if !services.AIWorkflowService.AgentAllowsHumanHandoff(&aiAgent) {
		t.Fatal("test setup expects the misconfigured default agent to appear handoff-capable")
	}
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusAIServing)
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
		"tenant_id":                 int64(1),
		"product_id":                int64(0),
		"device_id":                 int64(0),
		"service_code_id":           int64(0),
		"customer_entry_session_id": int64(0),
	}).Error; err != nil {
		t.Fatalf("mark general conversation: %v", err)
	}
	conversation = *services.ConversationService.Get(conversation.ID)
	external := openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     "general-handoff-customer",
		ExternalName:   "通用咨询客户",
	}
	ensureHumanDispatchCustomer(t, db, conversation.CustomerID, external.ExternalName)
	if err := db.Create(&models.CustomerIdentity{CustomerID: conversation.CustomerID, ExternalSource: external.ExternalSource, ExternalID: external.ExternalID}).Error; err != nil {
		t.Fatalf("create customer identity error = %v", err)
	}

	err := services.ConversationHumanDispatchService.RequestByCustomer(conversation.ID, external, "我要转人工", "general-customer-handoff")
	if err == nil || !strings.Contains(err.Error(), "通用咨询仅支持 AI 自助") {
		t.Fatalf("expected customer general handoff rejection, got %v", err)
	}
	if _, err := services.ConversationHumanDispatchService.HandoffByAIWithRequestID(conversation.ID, aiAgent, "AI 自动转人工", "general-ai-handoff"); err == nil || !strings.Contains(err.Error(), "通用咨询仅支持 AI 自助") {
		t.Fatalf("expected direct AI general handoff rejection, got %v", err)
	}
	if handled, err := services.ConversationHumanDispatchService.TryOffHoursHandoffByAIWithRequestID(conversation.ID, aiAgent, "非服务时间转人工", "general-off-hours"); err == nil || handled || !strings.Contains(err.Error(), "通用咨询仅支持 AI 自助") {
		t.Fatalf("expected off-hours general handoff rejection, handled=%v err=%v", handled, err)
	}
	current := services.ConversationService.Get(conversation.ID)
	if current.Status != enums.IMConversationStatusAIServing || current.HandoffAt != nil || current.CurrentTeamID != 0 || current.CurrentAssigneeID != 0 {
		t.Fatalf("general rejection must keep AI serving and unassigned: %+v", current)
	}
	if count := services.TicketService.Count(sqls.NewCnd().Eq("conversation_id", conversation.ID)); count != 0 {
		t.Fatalf("general handoff rejection must not create tickets, got %d", count)
	}
	if count := services.MessageService.Count(sqls.NewCnd().Eq("conversation_id", conversation.ID)); count != 0 {
		t.Fatalf("general handoff rejection must not emit handoff messages, got %d", count)
	}
}

func TestConversationHumanDispatchAllowsKnowledgeTenantDefaultGeneralHandoff(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	if err := db.Model(&models.Tenant{}).Where("id = ?", 1).Update("service_scene", models.TenantServiceSceneKnowledgeSupport).Error; err != nil {
		t.Fatalf("mark knowledge support tenant: %v", err)
	}
	createHumanDispatchTeam(t, db, 1, services.TechnicalMaintenanceTeamName)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Updates(map[string]any{
		"team_type":  services.AgentTeamTypeTechnicalRepair,
		"product_id": 0,
	}).Error; err != nil {
		t.Fatalf("configure knowledge maintenance team: %v", err)
	}
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "1")
	aiAgent.Source = services.TenantDefaultAIAgentSource
	aiAgent.ProductID = 0
	aiAgent.HandoffMode = enums.AIAgentHandoffModeDefaultTeamPool
	if err := db.Model(&models.AIAgent{}).Where("id = ?", aiAgent.ID).Updates(map[string]any{
		"source":       aiAgent.Source,
		"product_id":   aiAgent.ProductID,
		"team_ids":     aiAgent.TeamIDs,
		"handoff_mode": aiAgent.HandoffMode,
	}).Error; err != nil {
		t.Fatalf("configure knowledge tenant agent: %v", err)
	}
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusAIServing)
	external := openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     "knowledge-handoff-customer",
		ExternalName:   "知识服务客户",
	}
	ensureHumanDispatchCustomer(t, db, conversation.CustomerID, external.ExternalName)
	if err := db.Create(&models.CustomerIdentity{CustomerID: conversation.CustomerID, ExternalSource: external.ExternalSource, ExternalID: external.ExternalID}).Error; err != nil {
		t.Fatalf("create knowledge customer identity: %v", err)
	}
	if !services.CustomerConversationAllowsHumanHandoff(&conversation, &aiAgent) {
		t.Fatal("knowledge support conversation must expose customer handoff")
	}
	if err := services.ConversationHumanDispatchService.RequestByCustomer(conversation.ID, external, "我要转人工", "knowledge-customer-handoff"); err != nil {
		t.Fatalf("RequestByCustomer() error = %v", err)
	}
	current := services.ConversationService.Get(conversation.ID)
	if current == nil || current.Status != enums.IMConversationStatusPending || current.HandoffAt == nil || current.CurrentTeamID != 1 {
		t.Fatalf("knowledge conversation did not enter human queue: %+v", current)
	}
	ticket := services.TicketService.FindOne(sqls.NewCnd().Eq("conversation_id", conversation.ID))
	if ticket == nil || ticket.ProductID != 0 || ticket.DeviceID != 0 || ticket.ServiceCodeID != 0 || ticket.CurrentTeamID != 1 {
		t.Fatalf("knowledge handoff ticket retained equipment context: %+v", ticket)
	}
	createHumanDispatchAgentProfile(t, db, 101, 1, enums.ServiceStatusIdle, 3, true, enums.StatusOk)
	if err := services.TicketLifecycleService.Takeover(ticket.ID, &dto.AuthPrincipal{TenantID: 1, UserID: 101, Username: "knowledge-engineer", Nickname: "知识服务工程师"}); err != nil {
		t.Fatalf("knowledge support ticket takeover error = %v", err)
	}
	var progress models.TicketProgress
	if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, enums.TicketProgressEventAccepted).Order("id DESC").First(&progress).Error; err != nil {
		t.Fatalf("load knowledge support acceptance progress: %v", err)
	}
	if progress.Content != "技术支持组成员从待派单池转派给自己并受理" {
		t.Fatalf("knowledge support acceptance retained product terminology: %q", progress.Content)
	}
}

func TestConversationHumanDispatchNoScheduleUsesDefaultAutoAssignment(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "1")
	createHumanDispatchTeam(t, db, 1, "默认全天候产品组")
	createHumanDispatchAgentProfile(t, db, 101, 1, enums.ServiceStatusIdle, 3, true, enums.StatusOk)
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusAIServing)

	result, err := services.ConversationHumanDispatchService.HandoffByAI(conversation.ID, aiAgent, "客户要求转人工")
	if err != nil {
		t.Fatalf("HandoffByAI() error = %v", err)
	}
	if result == nil || result.Decision != services.HandoffDecisionAssigned || result.AssigneeID != 101 {
		t.Fatalf("team without schedules should use default auto assignment, got %+v", result)
	}
	current := services.ConversationService.Get(conversation.ID)
	if current == nil || current.CurrentTeamID != 1 || current.CurrentAssigneeID != 101 {
		t.Fatalf("unexpected default assignment: %+v", current)
	}
}

func TestConversationDispatchCountsLinkedConversationAndTicketOnce(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "1")
	createHumanDispatchTeam(t, db, 1, "容量去重产品组")
	createHumanDispatchAgentProfile(t, db, 101, 1, enums.ServiceStatusIdle, 2, true, enums.StatusOk)
	existing := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusActive)
	if err := db.Model(&models.Conversation{}).Where("id = ?", existing.ID).Updates(map[string]any{
		"current_team_id":     1,
		"current_assignee_id": 101,
	}).Error; err != nil {
		t.Fatalf("assign existing conversation: %v", err)
	}
	now := time.Now()
	if err := db.Create(&models.Ticket{
		TicketNo:          "LINKED-WORK-1",
		Title:             "已关联会话的在办事项",
		Status:            enums.TicketStatusAssigned,
		ConversationID:    existing.ID,
		CurrentTeamID:     1,
		CurrentAssigneeID: 101,
		AuditFields:       models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create linked ticket: %v", err)
	}
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusAIServing)

	result, err := services.ConversationHumanDispatchService.HandoffByAI(conversation.ID, aiAgent, "第二个事项请求人工")
	if err != nil {
		t.Fatalf("HandoffByAI() error = %v", err)
	}
	if result == nil || result.Decision != services.HandoffDecisionAssigned {
		t.Fatalf("linked conversation and ticket should count once, got %+v", result)
	}
}

func TestConversationDispatchDoesNotCountResolvedLinkedConversationAsActiveWork(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "1")
	createHumanDispatchTeam(t, db, 1, "待确认不占容量产品组")
	createHumanDispatchAgentProfile(t, db, 101, 1, enums.ServiceStatusIdle, 1, true, enums.StatusOk)
	existing := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusActive)
	if err := db.Model(&models.Conversation{}).Where("id = ?", existing.ID).Updates(map[string]any{
		"current_team_id":     1,
		"current_assignee_id": 101,
	}).Error; err != nil {
		t.Fatalf("assign resolved conversation: %v", err)
	}
	now := time.Now()
	if err := db.Create(&models.Ticket{
		TicketNo:          "LINKED-RESOLVED-1",
		Title:             "等待客户确认的历史事项",
		Status:            enums.TicketStatusResolved,
		ConversationID:    existing.ID,
		CurrentTeamID:     1,
		CurrentAssigneeID: 101,
		ResolvedAt:        &now,
		AuditFields:       models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create resolved linked ticket: %v", err)
	}
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusAIServing)

	result, err := services.ConversationHumanDispatchService.HandoffByAI(conversation.ID, aiAgent, "新的客户事项请求人工")
	if err != nil {
		t.Fatalf("HandoffByAI() error = %v", err)
	}
	if result == nil || result.Decision != services.HandoffDecisionAssigned || result.AssigneeID != 101 {
		t.Fatalf("resolved linked conversation must not consume active capacity, got %+v", result)
	}
}

func TestConversationHumanDispatchAIHandoffAssignsAvailableAgent(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "1")
	createHumanDispatchTeam(t, db, 1, "售后支持组")
	createHumanDispatchActiveSchedule(t, db, 1)
	createHumanDispatchAgentProfile(t, db, 101, 1, enums.ServiceStatusIdle, 3, true, enums.StatusOk)
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusAIServing)

	result, err := services.ConversationHumanDispatchService.HandoffByAI(conversation.ID, aiAgent, "用户要求转人工")
	if err != nil {
		t.Fatalf("HandoffByAI() error = %v", err)
	}
	if result == nil || result.Decision != services.HandoffDecisionAssigned {
		t.Fatalf("expected assigned decision, got %+v", result)
	}

	current := services.ConversationService.Get(conversation.ID)
	if current.Status != enums.IMConversationStatusPending {
		t.Fatalf("expected assigned conversation to wait for acceptance, got status=%d", current.Status)
	}
	if current.CurrentAssigneeID != 101 || current.CurrentTeamID != 1 {
		t.Fatalf("unexpected assignment: assignee=%d team=%d", current.CurrentAssigneeID, current.CurrentTeamID)
	}
	if current.HandoffAt == nil || current.HandoffReason != "用户要求转人工" {
		t.Fatalf("expected handoff metadata, got at=%v reason=%q", current.HandoffAt, current.HandoffReason)
	}
}

func TestConversationHumanDispatchWeightedProductTeamPrefersLowerWeightedLoad(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "1")
	createHumanDispatchTeam(t, db, 1, "高压泵维修组")
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("assignment_mode", services.AgentTeamAssignmentModeWeighted).Error; err != nil {
		t.Fatalf("set weighted assignment mode error = %v", err)
	}
	createHumanDispatchActiveSchedule(t, db, 1)
	createHumanDispatchAgentProfile(t, db, 101, 1, enums.ServiceStatusIdle, 10, true, enums.StatusOk)
	createHumanDispatchAgentProfile(t, db, 102, 1, enums.ServiceStatusIdle, 10, true, enums.StatusOk)
	if err := db.Model(&models.AgentTeamMember{}).Where("team_id = ? AND user_id = ?", 1, 101).
		Updates(map[string]any{"dispatch_enabled": true, "dispatch_weight": 1}).Error; err != nil {
		t.Fatalf("update low-weight member error = %v", err)
	}
	if err := db.Model(&models.AgentTeamMember{}).Where("team_id = ? AND user_id = ?", 1, 102).
		Updates(map[string]any{"dispatch_enabled": true, "dispatch_weight": 3}).Error; err != nil {
		t.Fatalf("update high-weight member error = %v", err)
	}
	now := time.Now()
	if err := db.Create(&[]models.Ticket{
		{TicketNo: "W-101-1", Title: "低权重已有 1 单", Status: enums.TicketStatusProcessing, CurrentAssigneeID: 101, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TicketNo: "W-102-1", Title: "高权重已有 1 单", Status: enums.TicketStatusProcessing, CurrentAssigneeID: 102, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TicketNo: "W-102-2", Title: "高权重已有 2 单", Status: enums.TicketStatusProcessing, CurrentAssigneeID: 102, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}).Error; err != nil {
		t.Fatalf("create workload tickets error = %v", err)
	}
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusAIServing)

	result, err := services.ConversationHumanDispatchService.HandoffByAI(conversation.ID, aiAgent, "客户要求转人工")
	if err != nil {
		t.Fatalf("HandoffByAI() error = %v", err)
	}
	if result == nil || result.Decision != services.HandoffDecisionAssigned {
		t.Fatalf("expected assigned decision, got %+v", result)
	}
	current := services.ConversationService.Get(conversation.ID)
	if current.CurrentAssigneeID != 102 || current.CurrentTeamID != 1 {
		t.Fatalf("weighted dispatch should choose user 102, got assignee=%d team=%d", current.CurrentAssigneeID, current.CurrentTeamID)
	}
}

func TestConversationHumanDispatchWaitPoolDoesNotAutoAssignProductTeam(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "1")
	aiAgent.HandoffMode = enums.AIAgentHandoffModeWaitPool
	createHumanDispatchTeam(t, db, 1, "售后支持组")
	createHumanDispatchActiveSchedule(t, db, 1)
	createHumanDispatchAgentProfile(t, db, 101, 1, enums.ServiceStatusIdle, 3, true, enums.StatusOk)
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusAIServing)

	result, err := services.ConversationHumanDispatchService.HandoffByAI(conversation.ID, aiAgent, "进入企业公共队列")
	if err != nil {
		t.Fatalf("HandoffByAI() error = %v", err)
	}
	if result == nil || result.Decision != services.HandoffDecisionGlobalPool {
		t.Fatalf("expected global_pool decision, got %+v", result)
	}
	current := services.ConversationService.Get(conversation.ID)
	if current.Status != enums.IMConversationStatusPending || current.CurrentTeamID != 0 || current.CurrentAssigneeID != 0 {
		t.Fatalf("expected global pending pool, got %+v", current)
	}
}

func TestConversationHumanDispatchProductHandoffSynchronizesTicketAssignment(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	tenant := models.Tenant{Name: "产品维修租户", Status: enums.StatusOk}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatalf("create tenant error = %v", err)
	}
	ensureHumanDispatchEnterpriseWorkTime(t, db, tenant.ID, time.Now())
	product := models.Product{TenantID: tenant.ID, Code: "REPAIR-PRODUCT", Name: "维修产品", Status: enums.StatusOk}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product error = %v", err)
	}
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "1")
	if err := db.Model(&models.AIAgent{}).Where("id = ?", aiAgent.ID).Updates(map[string]any{
		"tenant_id":  tenant.ID,
		"product_id": product.ID,
	}).Error; err != nil {
		t.Fatalf("scope ai agent error = %v", err)
	}
	aiAgent.TenantID = tenant.ID
	aiAgent.ProductID = product.ID

	createHumanDispatchTeam(t, db, 1, product.Name)
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Updates(map[string]any{
		"tenant_id":  tenant.ID,
		"product_id": product.ID,
		"team_type":  services.AgentTeamTypeProductRepair,
	}).Error; err != nil {
		t.Fatalf("scope product team error = %v", err)
	}
	createHumanDispatchActiveSchedule(t, db, 1)
	if err := db.Model(&models.AgentTeamSchedule{}).Where("team_id = ?", 1).Update("tenant_id", tenant.ID).Error; err != nil {
		t.Fatalf("scope schedule error = %v", err)
	}
	createHumanDispatchAgentProfile(t, db, 101, 1, enums.ServiceStatusIdle, 3, true, enums.StatusOk)
	if err := db.Model(&models.AgentProfile{}).Where("user_id = ?", 101).Update("tenant_id", tenant.ID).Error; err != nil {
		t.Fatalf("scope agent profile error = %v", err)
	}

	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusAIServing)
	ensureHumanDispatchCustomer(t, db, conversation.CustomerID, "测试访客")
	entrySession := models.CustomerEntrySession{
		TenantID: tenant.ID, ProductID: product.ID, VisitorID: "product-handoff-visitor",
		VisitorTokenHash: "product-handoff-token", State: "active", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.Create(&entrySession).Error; err != nil {
		t.Fatalf("create entry session error = %v", err)
	}
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
		"tenant_id": tenant.ID, "product_id": product.ID, "customer_entry_session_id": entrySession.ID,
	}).Error; err != nil {
		t.Fatalf("scope conversation error = %v", err)
	}

	result, err := services.ConversationHumanDispatchService.HandoffByAI(conversation.ID, aiAgent, "需要工程师处理")
	if err != nil {
		t.Fatalf("HandoffByAI() error = %v", err)
	}
	if result.Decision != services.HandoffDecisionAssigned || result.TicketID <= 0 {
		t.Fatalf("unexpected handoff result: %+v", result)
	}
	current := services.ConversationService.Get(conversation.ID)
	if current.CurrentTeamID != 1 || current.CurrentAssigneeID != 101 {
		t.Fatalf("conversation assignment = team:%d assignee:%d", current.CurrentTeamID, current.CurrentAssigneeID)
	}
	ticket := services.TicketService.Get(result.TicketID)
	if ticket == nil || ticket.CurrentTeamID != 1 || ticket.CurrentAssigneeID != 101 || ticket.ProductID != product.ID {
		t.Fatalf("ticket assignment was not synchronized: %+v", ticket)
	}
}

func TestConversationManualAssignmentSynchronizesLinkedTicket(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	createHumanDispatchTeam(t, db, 1, "产品维修组")
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("tenant_id", 1).Error; err != nil {
		t.Fatalf("scope team error = %v", err)
	}
	createHumanDispatchAgentProfile(t, db, 101, 1, enums.ServiceStatusIdle, 3, true, enums.StatusOk)
	conversation := createHumanDispatchConversation(t, db, 1, enums.IMConversationStatusPending)
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Update("tenant_id", 1).Error; err != nil {
		t.Fatalf("scope conversation error = %v", err)
	}
	ticket := models.Ticket{
		TicketNo:       "TK-MANUAL-ASSIGN",
		Title:          "手动分配同步测试",
		TenantID:       1,
		ConversationID: conversation.ID,
		Status:         enums.TicketStatusPending,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket error = %v", err)
	}

	operator := testHumanDispatchOperator()
	operator.TenantID = 1
	err := services.ConversationService.AssignConversation(request.AssignConversationRequest{
		ConversationID: conversation.ID,
		AssigneeID:     101,
		Reason:         "人工调度",
	}, operator)
	if err != nil {
		t.Fatalf("AssignConversation() error = %v", err)
	}

	current := services.ConversationService.Get(conversation.ID)
	if current.Status != enums.IMConversationStatusPending || current.CurrentTeamID != 1 || current.CurrentAssigneeID != 101 {
		t.Fatalf("conversation assignment was not synchronized: %+v", current)
	}
	currentTicket := services.TicketService.Get(ticket.ID)
	if currentTicket.Status != enums.TicketStatusAssigned || currentTicket.CurrentTeamID != 1 || currentTicket.CurrentAssigneeID != 101 {
		t.Fatalf("ticket assignment was not synchronized: %+v", currentTicket)
	}
	var progressCount int64
	if err := db.Model(&models.TicketProgress{}).Where("ticket_id = ? AND event_type = ?", ticket.ID, enums.TicketProgressEventAssigned).Count(&progressCount).Error; err != nil {
		t.Fatalf("count ticket progress error = %v", err)
	}
	if progressCount != 1 {
		t.Fatalf("expected one ticket assignment progress, got %d", progressCount)
	}
}

func TestConversationManualAssignmentAllowsDispatchDisabledTeamMember(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	createHumanDispatchTeam(t, db, 1, "产品维修组")
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("tenant_id", 1).Error; err != nil {
		t.Fatalf("scope team: %v", err)
	}
	createHumanDispatchAgentProfile(t, db, 101, 1, enums.ServiceStatusIdle, 3, false, enums.StatusOk)
	conversation := createHumanDispatchConversation(t, db, 1, enums.IMConversationStatusPending)
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
		"tenant_id":           int64(1),
		"current_team_id":     int64(1),
		"current_assignee_id": int64(0),
	}).Error; err != nil {
		t.Fatalf("scope conversation: %v", err)
	}
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager", Roles: []string{services.EnterpriseRoleServiceManager}}

	err := services.ConversationService.AssignConversation(request.AssignConversationRequest{
		ConversationID: conversation.ID,
		AssigneeID:     101,
		Reason:         "本组停派成员",
	}, manager)
	if err != nil {
		t.Fatalf("manual conversation assignment should override the automatic-dispatch toggle: %v", err)
	}
	current := services.ConversationService.Get(conversation.ID)
	if current.CurrentAssigneeID != 101 || current.CurrentTeamID != 1 || current.Status != enums.IMConversationStatusPending {
		t.Fatalf("manual conversation assignment was not persisted: %+v", current)
	}
}

func TestConversationManualAssignmentOverridesCapacityButRequiresLinkedTicketCapability(t *testing.T) {
	manager := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager", Roles: []string{services.EnterpriseRoleServiceManager}}
	cases := []struct {
		name   string
		reject bool
		mutate func(t *testing.T, db *gorm.DB, conversation models.Conversation)
	}{
		{
			name: "full capacity",
			mutate: func(t *testing.T, db *gorm.DB, conversation models.Conversation) {
				t.Helper()
				if err := db.Model(&models.AgentProfile{}).Where("tenant_id = ? AND user_id = ?", 1, 101).
					Update("max_concurrent_count", 1).Error; err != nil {
					t.Fatal(err)
				}
				now := time.Now()
				if err := db.Create(&models.Ticket{
					TicketNo: "CONV-MANUAL-CAPACITY-EXISTING", Title: "已有在办工单", TenantID: 1,
					CurrentTeamID: 1, CurrentAssigneeID: 101, Status: enums.TicketStatusProcessing,
					AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
				}).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:   "linked ticket capability mismatch",
			reject: true,
			mutate: func(t *testing.T, db *gorm.DB, conversation models.Conversation) {
				t.Helper()
				if err := db.AutoMigrate(&models.TenantMember{}, &models.EngineerProfile{}); err != nil {
					t.Fatal(err)
				}
				createConversationDispatchEngineerCapability(t, db, 101, `["hydraulic"]`, `["emea"]`, true)
				now := time.Now()
				if err := db.Create(&models.Ticket{
					TicketNo: "CONV-MANUAL-CAPABILITY", Title: "关联工单能力不匹配", TenantID: 1, ProductID: 701,
					ConversationID: conversation.ID, CurrentTeamID: 1, FaultCode: "power", ServiceRegion: "apac",
					Status:      enums.TicketStatusPendingDispatch,
					AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
				}).Error; err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := setupConversationHumanDispatchTestDB(t)
			now := time.Now()
			ensureHumanDispatchTenant(t, db, 1, "手动会话派单租户")
			if err := db.Create(&models.Product{
				ID: 701, TenantID: 1, Code: "POWER", Name: "电源系统", Status: enums.StatusOk,
				AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
			}).Error; err != nil {
				t.Fatal(err)
			}
			createHumanDispatchTeam(t, db, 1, "产品维修组")
			if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Updates(map[string]any{
				"tenant_id":  int64(1),
				"product_id": int64(701),
				"team_type":  services.AgentTeamTypeProductRepair,
			}).Error; err != nil {
				t.Fatal(err)
			}
			createHumanDispatchAgentProfile(t, db, 101, 1, enums.ServiceStatusIdle, 3, true, enums.StatusOk)
			aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "1")
			conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusPending)
			if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
				"tenant_id":           int64(1),
				"product_id":          int64(701),
				"current_team_id":     int64(1),
				"current_assignee_id": int64(0),
			}).Error; err != nil {
				t.Fatal(err)
			}
			conversation = *services.ConversationService.Get(conversation.ID)
			tc.mutate(t, db, conversation)

			err := services.ConversationService.AssignConversation(request.AssignConversationRequest{
				ConversationID: conversation.ID,
				AssigneeID:     101,
				Reason:         tc.name,
			}, manager)
			current := services.ConversationService.Get(conversation.ID)
			if tc.reject {
				if err == nil {
					t.Fatalf("manual conversation assignment should reject %s", tc.name)
				}
				if current.CurrentAssigneeID != 0 || current.CurrentTeamID != 1 || current.Status != enums.IMConversationStatusPending {
					t.Fatalf("rejected manual conversation assignment mutated conversation: %+v", current)
				}
				return
			}
			if err != nil {
				t.Fatalf("manual conversation assignment should override %s: %v", tc.name, err)
			}
			if current.CurrentAssigneeID != 101 || current.CurrentTeamID != 1 || current.Status != enums.IMConversationStatusPending {
				t.Fatalf("manual conversation assignment was not persisted: %+v", current)
			}
		})
	}
}

func TestConversationTransferSynchronizesLinkedTicket(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	createHumanDispatchTeam(t, db, 1, "一线维修组")
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("tenant_id", 1).Error; err != nil {
		t.Fatalf("scope source team error = %v", err)
	}
	createHumanDispatchAgentProfile(t, db, 101, 1, enums.ServiceStatusIdle, 3, true, enums.StatusOk)
	createHumanDispatchTeam(t, db, 2, "二线维修组")
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 2).Update("tenant_id", 1).Error; err != nil {
		t.Fatalf("scope target team error = %v", err)
	}
	createHumanDispatchAgentProfile(t, db, 102, 2, enums.ServiceStatusIdle, 3, true, enums.StatusOk)
	conversation := createHumanDispatchConversation(t, db, 1, enums.IMConversationStatusActive)
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
		"tenant_id":           int64(1),
		"current_team_id":     1,
		"current_assignee_id": 101,
	}).Error; err != nil {
		t.Fatalf("seed conversation assignment error = %v", err)
	}
	ticket := models.Ticket{
		TicketNo:          "TK-MANUAL-TRANSFER",
		Title:             "转接同步测试",
		TenantID:          1,
		ConversationID:    conversation.ID,
		Status:            enums.TicketStatusAssigned,
		CurrentTeamID:     1,
		CurrentAssigneeID: 101,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket error = %v", err)
	}

	err := services.ConversationService.TransferConversation(conversation.ID, 102, "升级二线", &dto.AuthPrincipal{
		TenantID: 1,
		UserID:   101,
		Username: "first-line-agent",
	})
	if err != nil {
		t.Fatalf("TransferConversation() error = %v", err)
	}

	current := services.ConversationService.Get(conversation.ID)
	if current.CurrentTeamID != 2 || current.CurrentAssigneeID != 102 {
		t.Fatalf("conversation transfer was not synchronized: %+v", current)
	}
	currentTicket := services.TicketService.Get(ticket.ID)
	if currentTicket.CurrentTeamID != 2 || currentTicket.CurrentAssigneeID != 102 {
		t.Fatalf("ticket transfer was not synchronized: %+v", currentTicket)
	}
}

func TestConversationTransferUsesCurrentProductTeamWithoutCapabilityFiltering(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	createHumanDispatchTeam(t, db, 1, "产品 A 维修组")
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Updates(map[string]any{
		"tenant_id":  int64(1),
		"product_id": int64(11),
		"team_type":  services.AgentTeamTypeProductRepair,
	}).Error; err != nil {
		t.Fatalf("scope team 1: %v", err)
	}
	createHumanDispatchTeam(t, db, 2, "旧主组")
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 2).Updates(map[string]any{
		"tenant_id":  int64(1),
		"product_id": int64(22),
	}).Error; err != nil {
		t.Fatalf("scope team 2: %v", err)
	}
	createHumanDispatchAgentProfile(t, db, 101, 1, enums.ServiceStatusIdle, 3, true, enums.StatusOk)
	createHumanDispatchAgentProfile(t, db, 102, 2, enums.ServiceStatusIdle, 3, true, enums.StatusOk)
	if err := db.AutoMigrate(&models.TenantMember{}, &models.EngineerProfile{}); err != nil {
		t.Fatalf("migrate engineer capability tables: %v", err)
	}
	createConversationDispatchEngineerCapability(t, db, 102, `["hydraulic"]`, `["emea"]`, false)
	if err := db.Create(&models.AgentTeamMember{
		TenantID: 1, TeamID: 1, UserID: 102,
		DispatchEnabled: true, DispatchWeight: 3, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("add target to current team: %v", err)
	}
	conversation := createHumanDispatchConversation(t, db, 1, enums.IMConversationStatusActive)
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
		"tenant_id":           int64(1),
		"product_id":          int64(11),
		"current_team_id":     int64(1),
		"current_assignee_id": int64(101),
	}).Error; err != nil {
		t.Fatalf("seed active conversation: %v", err)
	}
	ticket := models.Ticket{
		TenantID: 1, ProductID: 11, TicketNo: "TK-CURRENT-TEAM-TRANSFER", Title: "保留当前产品组转派",
		ConversationID: conversation.ID, Status: enums.TicketStatusAssigned,
		CurrentTeamID: 1, CurrentAssigneeID: 101, ServiceRegion: "apac",
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	err := services.ConversationService.TransferConversation(conversation.ID, 102, "同组二线处理", &dto.AuthPrincipal{
		TenantID: 1, UserID: 101, Username: "first-line-agent",
	})
	if err != nil {
		t.Fatalf("TransferConversation() error = %v", err)
	}

	current := services.ConversationService.Get(conversation.ID)
	if current.CurrentTeamID != 1 || current.CurrentAssigneeID != 102 {
		t.Fatalf("conversation transfer should keep current product team: %+v", current)
	}
	currentTicket := services.TicketService.Get(ticket.ID)
	if currentTicket.CurrentTeamID != 1 || currentTicket.CurrentAssigneeID != 102 {
		t.Fatalf("ticket transfer should keep current product team: %+v", currentTicket)
	}
}

func TestConversationTransferRejectsTargetInAnotherProductRepairTeam(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	createHumanDispatchTeam(t, db, 1, "产品 A 维修组")
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Updates(map[string]any{
		"tenant_id":  int64(1),
		"product_id": int64(11),
		"team_type":  services.AgentTeamTypeProductRepair,
	}).Error; err != nil {
		t.Fatalf("scope product team: %v", err)
	}
	createHumanDispatchAgentProfile(t, db, 101, 1, enums.ServiceStatusIdle, 3, true, enums.StatusOk)
	createHumanDispatchTeam(t, db, 2, "产品 B 维修组")
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 2).Updates(map[string]any{
		"tenant_id":  int64(1),
		"product_id": int64(22),
		"team_type":  services.AgentTeamTypeProductRepair,
	}).Error; err != nil {
		t.Fatalf("scope other product team: %v", err)
	}
	createHumanDispatchAgentProfile(t, db, 102, 2, enums.ServiceStatusIdle, 3, true, enums.StatusOk)
	conversation := createHumanDispatchConversation(t, db, 1, enums.IMConversationStatusActive)
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
		"tenant_id":           int64(1),
		"product_id":          int64(11),
		"current_team_id":     int64(1),
		"current_assignee_id": int64(101),
	}).Error; err != nil {
		t.Fatalf("seed product conversation: %v", err)
	}
	err := services.ConversationService.TransferConversation(conversation.ID, 102, "尝试转到其他产品组", &dto.AuthPrincipal{
		TenantID: 1, UserID: 101, Username: "product-agent",
	})
	if err == nil {
		t.Fatal("product conversation transfer should reject a target in another product repair team")
	}
	current := services.ConversationService.Get(conversation.ID)
	if current.CurrentTeamID != 1 || current.CurrentAssigneeID != 101 {
		t.Fatalf("rejected cross-product transfer mutated conversation: %+v", current)
	}
}

func TestConversationTransferAllowsExplicitUnavailableEngineerSelection(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	createHumanDispatchTeam(t, db, 1, "一线维修组")
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("tenant_id", 1).Error; err != nil {
		t.Fatalf("scope team 1: %v", err)
	}
	createHumanDispatchAgentProfile(t, db, 101, 1, enums.ServiceStatusIdle, 3, true, enums.StatusOk)
	createHumanDispatchTeam(t, db, 2, "二线维修组")
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 2).Update("tenant_id", 1).Error; err != nil {
		t.Fatalf("scope team 2: %v", err)
	}
	createHumanDispatchAgentProfile(t, db, 102, 2, enums.ServiceStatusIdle, 3, true, enums.StatusOk)
	now := time.Now()
	if err := db.Model(&models.AgentWorkStatus{}).Where("tenant_id = ? AND user_id = ?", 1, 102).Updates(map[string]any{
		"status": services.AgentWorkStatusLeave, "confirmed_at": now, "status_changed_at": now,
	}).Error; err != nil {
		t.Fatalf("mark target leave: %v", err)
	}
	conversation := createHumanDispatchConversation(t, db, 1, enums.IMConversationStatusActive)
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
		"tenant_id":           int64(1),
		"current_team_id":     int64(1),
		"current_assignee_id": int64(101),
	}).Error; err != nil {
		t.Fatalf("seed active conversation: %v", err)
	}
	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 101, Username: "first-line-agent"}

	err := services.ConversationService.TransferConversation(conversation.ID, 102, "转接给请假工程师", operator)
	if err != nil {
		t.Fatalf("explicit conversation transfer should ignore display-only work status: %v", err)
	}
	current := services.ConversationService.Get(conversation.ID)
	if current.CurrentTeamID != 2 || current.CurrentAssigneeID != 102 || current.Status != enums.IMConversationStatusActive {
		t.Fatalf("explicit conversation transfer was not persisted: %+v", current)
	}
}

func TestConversationTeamPoolClearsLinkedTicketAssigneeState(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	createHumanDispatchTeam(t, db, 1, "产品维修组")
	conversation := createHumanDispatchConversation(t, db, 1, enums.IMConversationStatusPending)
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
		"current_team_id":     int64(1),
		"current_assignee_id": int64(101),
	}).Error; err != nil {
		t.Fatalf("seed conversation assignment error = %v", err)
	}
	now := time.Now()
	assignedAt := now.Add(-20 * time.Minute)
	deadlineAt := now.Add(10 * time.Minute)
	deferredUntil := now.Add(5 * time.Minute)
	ticket := models.Ticket{
		TicketNo: "TK-TEAM-POOL-RESET", Title: "回到团队池同步测试", TenantID: 1,
		ConversationID: conversation.ID, Status: enums.TicketStatusPendingAssigneeAccept,
		CurrentTeamID: 1, CurrentAssigneeID: 101,
		AssignedAt: &assignedAt, AcceptDeadlineAt: &deadlineAt,
		DispatchDeferredUntil: &deferredUntil, LastDispatchFailureReason: "accept_timeout_redispatching",
		AuditFields: models.AuditFields{CreatedAt: assignedAt, UpdatedAt: assignedAt},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket error = %v", err)
	}

	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		return services.TicketService.SyncConversationDispatchTx(ctx.Tx, conversation.ID, 1, 0, "重新进入团队池", testHumanDispatchOperator())
	}); err != nil {
		t.Fatalf("SyncConversationDispatchTx() error = %v", err)
	}

	currentTicket := services.TicketService.Get(ticket.ID)
	if currentTicket.Status != enums.TicketStatusPendingDispatch || currentTicket.CurrentTeamID != 1 || currentTicket.CurrentAssigneeID != 0 {
		t.Fatalf("linked ticket did not return to the team dispatch pool: %+v", currentTicket)
	}
	if currentTicket.AssignedAt != nil || currentTicket.AcceptedAt != nil || currentTicket.AcceptDeadlineAt != nil ||
		currentTicket.DispatchDeferredUntil != nil || currentTicket.LastDispatchFailureReason != "" {
		t.Fatalf("linked ticket kept stale assignee tracking after returning to team pool: %+v", currentTicket)
	}
	var progressCount int64
	if err := db.Model(&models.TicketProgress{}).
		Where("ticket_id = ? AND event_type = ?", ticket.ID, enums.TicketProgressEventAssigned).
		Count(&progressCount).Error; err != nil {
		t.Fatalf("count ticket progress error = %v", err)
	}
	if progressCount != 1 {
		t.Fatalf("expected one ticket progress for team-pool reset, got %d", progressCount)
	}
}

func TestConversationHumanDispatchAIHandoffFallsBackToFirstEligibleTeam(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "3,1,2")
	createHumanDispatchTeam(t, db, 1, "售后支持组")
	createHumanDispatchTeam(t, db, 2, "VIP支持组")
	createHumanDispatchTeam(t, db, 3, "非值班组")
	createHumanDispatchActiveSchedule(t, db, 1)
	createHumanDispatchActiveSchedule(t, db, 2)
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusAIServing)

	result, err := services.ConversationHumanDispatchService.HandoffByAI(conversation.ID, aiAgent, "用户要求转人工")
	if err != nil {
		t.Fatalf("HandoffByAI() error = %v", err)
	}
	if result == nil || result.Decision != services.HandoffDecisionTeamPool {
		t.Fatalf("expected team_pool decision, got %+v", result)
	}

	current := services.ConversationService.Get(conversation.ID)
	if current.Status != enums.IMConversationStatusPending {
		t.Fatalf("expected pending conversation, got status=%d", current.Status)
	}
	if current.CurrentTeamID != 3 || current.CurrentAssigneeID != 0 {
		t.Fatalf("expected first unrestricted team 3 with no assignee, got team=%d assignee=%d", current.CurrentTeamID, current.CurrentAssigneeID)
	}
}

func TestConversationHumanDispatchAIHandoffCreatesOneTicketPerConversation(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "1")
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusAIServing)
	conversation.TenantID = 77
	if err := db.Create(&models.Tenant{ID: conversation.TenantID, Name: "测试租户", Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create tenant error = %v", err)
	}
	ensureHumanDispatchCustomer(t, db, conversation.CustomerID, "测试访客")
	entrySession := models.CustomerEntrySession{
		TenantID: conversation.TenantID, VisitorID: "ticket-context-visitor", VisitorTokenHash: "ticket-context-token-hash",
		State: "active", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.Create(&entrySession).Error; err != nil {
		t.Fatalf("create customer entry session error = %v", err)
	}
	conversation.CustomerEntrySessionID = entrySession.ID
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
		"tenant_id":                 conversation.TenantID,
		"customer_entry_session_id": entrySession.ID,
		"last_message_summary":      "请补充具体的产品、场景和报错信息。",
	}).Error; err != nil {
		t.Fatalf("set conversation tenant error = %v", err)
	}
	if err := db.Create(&models.Message{
		ConversationID: conversation.ID,
		ClientMsgID:    "customer-fault-description",
		SenderType:     enums.IMSenderTypeCustomer,
		MessageType:    enums.IMMessageTypeText,
		Content:        "设备 RHD-FLOW-ALPHA-7742 显示故障码 PWR-001，复位后仍然红色。暂时不要转人工，也不要创建工单。",
		SendStatus:     enums.IMMessageStatusSent,
	}).Error; err != nil {
		t.Fatalf("create customer message error = %v", err)
	}
	if err := db.Create(&models.Message{
		ConversationID: conversation.ID,
		ClientMsgID:    "ai-fallback-answer",
		SenderType:     enums.IMSenderTypeAI,
		MessageType:    enums.IMMessageTypeText,
		Content:        "请补充具体的产品、场景和报错信息。",
		SendStatus:     enums.IMMessageStatusSent,
	}).Error; err != nil {
		t.Fatalf("create AI fallback message error = %v", err)
	}
	if err := db.Create(&models.Message{
		ConversationID: conversation.ID,
		ClientMsgID:    "customer-handoff-control",
		SenderType:     enums.IMSenderTypeCustomer,
		MessageType:    enums.IMMessageTypeText,
		Content:        "PROD-E2E-20260728-1346：请转人工工程师处理",
		SendStatus:     enums.IMMessageStatusSent,
	}).Error; err != nil {
		t.Fatalf("create customer handoff control message error = %v", err)
	}

	first, err := services.ConversationHumanDispatchService.HandoffByAI(conversation.ID, aiAgent, "知识库未找到可靠答案")
	if err != nil {
		t.Fatalf("first HandoffByAI() error = %v", err)
	}
	second, err := services.ConversationHumanDispatchService.HandoffByAI(conversation.ID, aiAgent, "客户仍需人工支持")
	if err != nil {
		t.Fatalf("second HandoffByAI() error = %v", err)
	}
	if first.TicketID <= 0 || first.TicketID != second.TicketID {
		t.Fatalf("expected one idempotent ticket, first=%+v second=%+v", first, second)
	}
	if !first.TicketCreated || second.TicketCreated {
		t.Fatalf("unexpected creation flags, first=%+v second=%+v", first, second)
	}
	if count := services.TicketService.Count(sqls.NewCnd().Eq("conversation_id", conversation.ID)); count != 1 {
		t.Fatalf("expected one ticket, got %d", count)
	}
	ticket := services.TicketService.Get(first.TicketID)
	if ticket == nil || ticket.CustomerEntrySessionID != entrySession.ID {
		t.Fatalf("handoff ticket lost entry session context: %+v", ticket)
	}
	if ticket.Title != "设备 RHD-FLOW-ALPHA-7742 显示故障码 PWR-001，复位后仍然红色" {
		t.Fatalf("handoff ticket should use the latest issue instead of action controls, got %q", ticket.Title)
	}
	if ticket.FaultCode != "PWR-001" {
		t.Fatalf("handoff ticket should preserve the reported fault code, got %q", ticket.FaultCode)
	}
	if !strings.Contains(ticket.SymptomSummary, "复位后仍然红色") {
		t.Fatalf("handoff ticket should preserve the customer symptom, got %q", ticket.SymptomSummary)
	}
	if ticket.DiagnosisSummary != "请补充具体的产品、场景和报错信息。" {
		t.Fatalf("handoff ticket should preserve the latest AI diagnosis, got %q", ticket.DiagnosisSummary)
	}
}

func TestCreateFromConversationConcurrentWithoutIdempotencyCreatesOneTicket(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	tenant := models.Tenant{ID: 88, Name: "并发建单租户", Status: enums.StatusOk}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	ensureHumanDispatchCustomer(t, db, 1, "并发建单客户")
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "1")
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusAIServing)
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
		"tenant_id":            tenant.ID,
		"last_message_summary": "设备无法启动，需要工程师协助。",
	}).Error; err != nil {
		t.Fatalf("scope conversation: %v", err)
	}

	operator := &dto.AuthPrincipal{TenantID: tenant.ID, UserID: 9, Username: "dispatcher", Nickname: "调度员"}
	start := make(chan struct{})
	results := make(chan *models.Ticket, 8)
	errorsFound := make(chan error, 8)
	var wait sync.WaitGroup
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			ticket, err := services.TicketService.CreateFromConversation(request.CreateTicketFromConversationRequest{
				ConversationID: conversation.ID,
				Title:          "设备无法启动",
				Description:    "客户从会话入口并发提交工单。",
			}, operator)
			results <- ticket
			errorsFound <- err
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(errorsFound)

	for err := range errorsFound {
		if err != nil {
			t.Fatalf("concurrent CreateFromConversation returned error: %v", err)
		}
	}
	var ticketID int64
	for ticket := range results {
		if ticket == nil || ticket.ID <= 0 {
			t.Fatalf("concurrent CreateFromConversation returned empty ticket: %+v", ticket)
		}
		if ticketID == 0 {
			ticketID = ticket.ID
		} else if ticket.ID != ticketID {
			t.Fatalf("concurrent CreateFromConversation returned different tickets: first=%d current=%d", ticketID, ticket.ID)
		}
	}
	if count := services.TicketService.Count(sqls.NewCnd().Eq("conversation_id", conversation.ID)); count != 1 {
		t.Fatalf("concurrent CreateFromConversation persisted %d tickets, want 1", count)
	}
	ticket := services.TicketService.Get(ticketID)
	if ticket == nil || ticket.IdempotencyKey == nil || *ticket.IdempotencyKey != "conversation-ticket:"+strconv.FormatInt(conversation.ID, 10) {
		t.Fatalf("conversation ticket idempotency key mismatch: %+v", ticket)
	}
}

func TestConversationHumanDispatchAIHandoffReusesExistingOpenTicket(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	tenant := models.Tenant{Name: "已有工单转人工租户", Status: enums.StatusOk}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	ensureHumanDispatchCustomer(t, db, 1, "已有工单客户")
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "")
	aiAgent.TenantID = tenant.ID
	if err := db.Model(&models.AIAgent{}).Where("id = ?", aiAgent.ID).Update("tenant_id", tenant.ID).Error; err != nil {
		t.Fatalf("scope AI agent: %v", err)
	}
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusAIServing)
	conversation.TenantID = tenant.ID
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Update("tenant_id", tenant.ID).Error; err != nil {
		t.Fatalf("scope conversation: %v", err)
	}
	idempotencyKey := "workflow-ticket:existing"
	existingTicket := models.Ticket{
		TenantID: tenant.ID, CustomerID: conversation.CustomerID, ConversationID: conversation.ID,
		TicketNo: "TK-EXISTING-1", Title: "客户已确认的服务问题", IdempotencyKey: &idempotencyKey,
		Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	if err := db.Create(&existingTicket).Error; err != nil {
		t.Fatalf("create existing ticket: %v", err)
	}

	result, err := services.ConversationHumanDispatchService.HandoffByAI(conversation.ID, aiAgent, "客户要求人工继续处理")
	if err != nil {
		t.Fatalf("HandoffByAI() error = %v", err)
	}
	if result == nil || result.TicketID != existingTicket.ID || result.TicketNo != existingTicket.TicketNo || result.TicketCreated {
		t.Fatalf("handoff did not reuse existing ticket: %+v", result)
	}
	if count := services.TicketService.Count(sqls.NewCnd().Eq("conversation_id", conversation.ID)); count != 1 {
		t.Fatalf("handoff persisted %d tickets, want one existing ticket", count)
	}
}

func TestConversationHumanDispatchConcurrentRequestsClaimHandoffOnce(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	rawDB, err := db.DB()
	if err != nil {
		t.Fatalf("read sql database: %v", err)
	}
	rawDB.SetMaxOpenConns(1)
	tenant := models.Tenant{Name: "并发转人工租户", Status: enums.StatusOk}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	ensureHumanDispatchEnterpriseWorkTime(t, db, tenant.ID, time.Now())
	ensureHumanDispatchCustomer(t, db, 1, "并发转人工客户")
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "1")
	aiAgent.TenantID = tenant.ID
	if err := db.Model(&models.AIAgent{}).Where("id = ?", aiAgent.ID).Update("tenant_id", tenant.ID).Error; err != nil {
		t.Fatalf("scope ai agent: %v", err)
	}
	createHumanDispatchTeam(t, db, 1, "并发转人工产品组")
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("tenant_id", tenant.ID).Error; err != nil {
		t.Fatalf("scope repair team: %v", err)
	}
	createHumanDispatchActiveSchedule(t, db, 1)
	createHumanDispatchAgentProfile(t, db, 101, 1, enums.ServiceStatusIdle, 3, true, enums.StatusOk)
	if err := db.Model(&models.AgentProfile{}).Where("user_id = ?", 101).Update("tenant_id", tenant.ID).Error; err != nil {
		t.Fatalf("scope agent profile: %v", err)
	}
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusAIServing)
	conversation.TenantID = tenant.ID
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Update("tenant_id", tenant.ID).Error; err != nil {
		t.Fatalf("scope conversation: %v", err)
	}

	start := make(chan struct{})
	results := make(chan *services.HandoffDecisionResult, 2)
	errorsFound := make(chan error, 2)
	var wait sync.WaitGroup
	for index := range 2 {
		wait.Add(1)
		go func(requestIndex int) {
			defer wait.Done()
			<-start
			result, handoffErr := services.ConversationHumanDispatchService.HandoffByAIWithRequestID(
				conversation.ID,
				aiAgent,
				"客户并发请求人工",
				"concurrent-handoff-"+strconv.Itoa(requestIndex),
			)
			results <- result
			errorsFound <- handoffErr
		}(index)
	}
	close(start)
	wait.Wait()
	close(results)
	close(errorsFound)

	for handoffErr := range errorsFound {
		if handoffErr != nil {
			t.Fatalf("concurrent handoff returned error: %v", handoffErr)
		}
	}
	var ticketID int64
	createdCount := 0
	for result := range results {
		if result == nil || result.TicketID <= 0 {
			t.Fatalf("concurrent handoff lost ticket result: %+v", result)
		}
		if ticketID == 0 {
			ticketID = result.TicketID
		} else if result.TicketID != ticketID {
			t.Fatalf("concurrent handoff created different tickets: first=%d current=%d", ticketID, result.TicketID)
		}
		if result.TicketCreated {
			createdCount++
		}
	}
	if createdCount != 1 {
		t.Fatalf("exactly one request must create the handoff ticket, got %d", createdCount)
	}
	if count := services.TicketService.Count(sqls.NewCnd().Eq("conversation_id", conversation.ID)); count != 1 {
		t.Fatalf("concurrent handoff persisted %d tickets, want 1", count)
	}
	current := services.ConversationService.Get(conversation.ID)
	if current == nil || current.CurrentAssigneeID != 101 || current.CurrentTeamID != 1 {
		t.Fatalf("concurrent handoff lost final assignment: %+v", current)
	}
	if count := services.ConversationEventLogService.Count(sqls.NewCnd().
		Eq("conversation_id", conversation.ID).
		Eq("event_type", enums.IMEventTypeTransfer).
		Eq("content", "AI转人工")); count != 1 {
		t.Fatalf("concurrent handoff persisted %d transition claims, want 1", count)
	}
	if count := services.MessageService.Count(sqls.NewCnd().
		Eq("conversation_id", conversation.ID).
		Eq("sender_type", enums.IMSenderTypeAI).
		Eq("content", services.HandoffWaitingMessage)); count != 1 {
		t.Fatalf("concurrent handoff sent %d waiting notices, want 1", count)
	}
}

func TestConversationHumanDispatchConcurrentHandoffAndTicketCreateShareActiveTicket(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	rawDB, err := db.DB()
	if err != nil {
		t.Fatalf("read sql database: %v", err)
	}
	rawDB.SetMaxOpenConns(1)
	tenant := models.Tenant{Name: "转人工建单并发租户", Status: enums.StatusOk}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	ensureHumanDispatchEnterpriseWorkTime(t, db, tenant.ID, time.Now())
	ensureHumanDispatchCustomer(t, db, 1, "转人工建单客户")
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "1")
	aiAgent.TenantID = tenant.ID
	if err := db.Model(&models.AIAgent{}).Where("id = ?", aiAgent.ID).Update("tenant_id", tenant.ID).Error; err != nil {
		t.Fatalf("scope ai agent: %v", err)
	}
	createHumanDispatchTeam(t, db, 1, "转人工建单组")
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Update("tenant_id", tenant.ID).Error; err != nil {
		t.Fatalf("scope repair team: %v", err)
	}
	createHumanDispatchActiveSchedule(t, db, 1)
	createHumanDispatchAgentProfile(t, db, 101, 1, enums.ServiceStatusIdle, 3, true, enums.StatusOk)
	if err := db.Model(&models.AgentProfile{}).Where("user_id = ?", 101).Update("tenant_id", tenant.ID).Error; err != nil {
		t.Fatalf("scope agent profile: %v", err)
	}
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusAIServing)
	conversation.TenantID = tenant.ID
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
		"tenant_id":            tenant.ID,
		"last_message_summary": "设备报错 E42，客户要求工程师协助。",
	}).Error; err != nil {
		t.Fatalf("scope conversation: %v", err)
	}
	operator := &dto.AuthPrincipal{TenantID: tenant.ID, UserID: 9, Username: "customer", Nickname: "客户"}

	start := make(chan struct{})
	ticketIDs := make(chan int64, 2)
	errorsFound := make(chan error, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		result, handoffErr := services.ConversationHumanDispatchService.HandoffByAIWithRequestID(
			conversation.ID,
			aiAgent,
			"AI 无法确认故障，需要人工处理",
			"concurrent-ai-handoff",
		)
		if result != nil {
			ticketIDs <- result.TicketID
		} else {
			ticketIDs <- 0
		}
		errorsFound <- handoffErr
	}()
	go func() {
		defer wait.Done()
		<-start
		ticket, createErr := services.TicketService.CreateFromConversation(request.CreateTicketFromConversationRequest{
			ConversationID: conversation.ID,
			Title:          "设备报错 E42",
			Description:    "客户同时点击提交工单。",
		}, operator)
		if ticket != nil {
			ticketIDs <- ticket.ID
		} else {
			ticketIDs <- 0
		}
		errorsFound <- createErr
	}()
	close(start)
	wait.Wait()
	close(ticketIDs)
	close(errorsFound)

	for err := range errorsFound {
		if err != nil {
			t.Fatalf("concurrent handoff/ticket create returned error: %v", err)
		}
	}
	var ticketID int64
	for id := range ticketIDs {
		if id <= 0 {
			t.Fatalf("concurrent handoff/ticket create returned empty ticket id")
		}
		if ticketID == 0 {
			ticketID = id
		} else if id != ticketID {
			t.Fatalf("concurrent handoff/ticket create returned different tickets: first=%d current=%d", ticketID, id)
		}
	}
	if count := services.TicketService.Count(sqls.NewCnd().Eq("conversation_id", conversation.ID)); count != 1 {
		t.Fatalf("concurrent handoff/ticket create persisted %d tickets, want 1", count)
	}
	current := services.ConversationService.Get(conversation.ID)
	if current == nil || current.HandoffAt == nil || current.CurrentAssigneeID != 101 || current.CurrentTeamID != 1 {
		t.Fatalf("concurrent handoff/ticket create lost handoff assignment: %+v", current)
	}
}

func TestConversationHumanDispatchHumanOnlyCreateOffHoursUsesGlobalPendingPool(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeHumanOnly, "1")
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusPending)

	result, err := services.ConversationHumanDispatchService.ApplyHumanOnlyCreate(conversation.ID, aiAgent)
	if err != nil {
		t.Fatalf("ApplyHumanOnlyCreate() error = %v", err)
	}
	if result == nil || result.Decision != services.HandoffDecisionGlobalPool {
		t.Fatalf("expected global pool decision, got %+v", result)
	}
	current := services.ConversationService.Get(conversation.ID)
	if current.Status != enums.IMConversationStatusPending {
		t.Fatalf("expected pending conversation, got status=%d", current.Status)
	}
	if current.CurrentTeamID != 0 || current.CurrentAssigneeID != 0 {
		t.Fatalf("expected global pending pool, got team=%d assignee=%d", current.CurrentTeamID, current.CurrentAssigneeID)
	}

	message := services.MessageService.FindOne(sqls.NewCnd().Eq("conversation_id", conversation.ID).Desc("id"))
	if message == nil || message.Content != services.HandoffWaitingMessage {
		t.Fatalf("expected waiting message, got %+v", message)
	}
}

func TestConversationHumanDispatchHumanOnlyCreateRejectsTenantlessLegacyEntry(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	aiAgent := models.AIAgent{
		Name:        "旧人工入口AI",
		ServiceMode: enums.IMConversationServiceModeHumanOnly,
		TeamIDs:     "1",
		HandoffMode: enums.AIAgentHandoffModeDefaultTeamPool,
		Status:      enums.StatusOk,
	}
	if err := db.Create(&aiAgent).Error; err != nil {
		t.Fatalf("create legacy human-only ai agent error = %v", err)
	}

	conversation, err := services.ConversationService.Create(openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceGuest,
		ExternalID:     "guest-human-only-legacy",
		ExternalName:   "旧入口访客",
	}, 1, aiAgent.ID)
	if err == nil || !strings.Contains(err.Error(), "tenant") {
		t.Fatalf("expected tenant context error, got conversation=%+v err=%v", conversation, err)
	}
	var count int64
	if err := db.Model(&models.Conversation{}).Count(&count).Error; err != nil {
		t.Fatalf("count conversations error = %v", err)
	}
	if count != 0 {
		t.Fatalf("tenantless human-only create should not persist conversations, got %d", count)
	}
}

func TestConversationHumanDispatchHumanOnlyCreateAssignsAvailableAgent(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeHumanOnly, "1")
	createHumanDispatchTeam(t, db, 1, "售后支持组")
	createHumanDispatchActiveSchedule(t, db, 1)
	createHumanDispatchAgentProfile(t, db, 101, 1, enums.ServiceStatusIdle, 3, true, enums.StatusOk)
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusPending)

	result, err := services.ConversationHumanDispatchService.ApplyHumanOnlyCreate(conversation.ID, aiAgent)
	if err != nil {
		t.Fatalf("ApplyHumanOnlyCreate() error = %v", err)
	}
	if result == nil || result.Decision != services.HandoffDecisionAssigned || result.AssigneeID != 101 {
		t.Fatalf("expected assigned decision for user 101, got %+v", result)
	}
	current := services.ConversationService.Get(conversation.ID)
	if current.Status != enums.IMConversationStatusPending {
		t.Fatalf("expected assigned conversation to wait for acceptance, got status=%d", current.Status)
	}
	if current.CurrentAssigneeID != 101 || current.CurrentTeamID != 1 {
		t.Fatalf("unexpected assignment: assignee=%d team=%d", current.CurrentAssigneeID, current.CurrentTeamID)
	}
}

func TestConversationHumanDispatchHumanOnlyDoesNotUseForeignTenantTeam(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeHumanOnly, "1")
	product := models.Product{TenantID: 1, Code: "TENANT-TEAM-ISOLATION", Name: "租户隔离产品", Status: enums.StatusOk}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create tenant product: %v", err)
	}
	if err := db.Model(&models.AIAgent{}).Where("id = ?", aiAgent.ID).Updates(map[string]any{
		"tenant_id":  1,
		"product_id": product.ID,
	}).Error; err != nil {
		t.Fatalf("scope ai agent: %v", err)
	}
	aiAgent.TenantID = 1
	aiAgent.ProductID = product.ID
	createHumanDispatchTeam(t, db, 1, "外租户产品组")
	if err := db.Model(&models.AgentTeam{}).Where("id = ?", 1).Updates(map[string]any{
		"tenant_id":  2,
		"product_id": product.ID,
	}).Error; err != nil {
		t.Fatalf("scope foreign team: %v", err)
	}
	createHumanDispatchActiveSchedule(t, db, 1)
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusPending)
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
		"tenant_id":  1,
		"product_id": product.ID,
	}).Error; err != nil {
		t.Fatalf("scope conversation: %v", err)
	}

	result, err := services.ConversationHumanDispatchService.ApplyHumanOnlyCreate(conversation.ID, aiAgent)
	if err != nil {
		t.Fatalf("ApplyHumanOnlyCreate() error = %v", err)
	}
	if result == nil || result.Decision != services.HandoffDecisionGlobalPool {
		t.Fatalf("foreign tenant team must fall back to the tenant global pool, got %+v", result)
	}
	current := services.ConversationService.Get(conversation.ID)
	if current.CurrentTeamID != 0 || current.CurrentAssigneeID != 0 {
		t.Fatalf("foreign tenant team leaked into conversation assignment: %+v", current)
	}
}

func TestConversationAutoAssignManualDispatchOffHoursReturnsBusinessMessage(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "1")
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusPending)

	err := services.ConversationService.AutoAssignConversation(conversation.ID, testHumanDispatchOperator())
	if err == nil {
		t.Fatalf("expected off-hours manual dispatch to fail")
	}
	if !strings.Contains(err.Error(), "当前暂不在人工客服服务时间内") {
		t.Fatalf("expected off-hours error, got %v", err)
	}
}

func TestConversationAutoAssignManualDispatchFallsBackToTeamPool(t *testing.T) {
	db := setupConversationHumanDispatchTestDB(t)
	aiAgent := createHumanDispatchAIAgent(t, db, enums.IMConversationServiceModeAIFirst, "1")
	createHumanDispatchTeam(t, db, 1, "售后支持组")
	createHumanDispatchAgentProfile(t, db, 101, 1, enums.ServiceStatusIdle, 3, false, enums.StatusOk)
	createHumanDispatchActiveSchedule(t, db, 1)
	conversation := createHumanDispatchConversation(t, db, aiAgent.ID, enums.IMConversationStatusPending)

	err := services.ConversationService.AutoAssignConversation(conversation.ID, testHumanDispatchOperator())
	if err != nil {
		t.Fatalf("AutoAssignConversation() error = %v", err)
	}
	current := services.ConversationService.Get(conversation.ID)
	if current.Status != enums.IMConversationStatusPending || current.CurrentTeamID != 1 || current.CurrentAssigneeID != 0 {
		t.Fatalf("expected team-pool pending conversation, got %+v", current)
	}
}

func setupConversationHumanDispatchTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",
			SingularTable: true,
		},
	})
	if err != nil {
		t.Fatalf("open sqlite error = %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(
		&models.Tenant{},
		&models.Product{},
		&models.User{},
		&models.Customer{},
		&models.CustomerIdentity{},
		&models.CustomerDeviceBinding{},
		&models.CustomerEntrySession{},
		&models.AIAgent{},
		&models.AgentTeam{},
		&models.AgentTeamMember{},
		&models.AgentTeamSchedule{},
		&models.AgentTeamScheduleTemplate{},
		&models.AgentProfile{},
		&models.AgentWorkStatus{},
		&models.Conversation{},
		&models.ConversationParticipant{},
		&models.ConversationAssignment{},
		&models.ConversationEventLog{},
		&models.ConversationReadState{},
		&models.Message{},
		&models.Channel{},
		&models.ChannelMessageOutbox{},
		&models.Ticket{},
		&models.TicketTag{},
		&models.TicketProgress{},
		&models.TicketDispatchAttempt{},
		&models.TicketContextSnapshot{},
		&models.TicketNoSequence{},
		&models.DomainEvent{},
		&models.OutboxRecord{},
	); err != nil {
		t.Fatalf("auto migrate error = %v", err)
	}
	sqls.SetDB(db)
	ensureHumanDispatchTenant(t, db, 1, "测试租户")
	ensureHumanDispatchEnterpriseWorkTime(t, db, 1, time.Now())
	return db
}

func createHumanDispatchAIAgent(t *testing.T, db *gorm.DB, mode enums.IMConversationServiceMode, teamIDs string) models.AIAgent {
	t.Helper()
	item := models.AIAgent{
		TenantID:    1,
		Name:        "测试AI",
		ServiceMode: mode,
		TeamIDs:     teamIDs,
		HandoffMode: enums.AIAgentHandoffModeDefaultTeamPool,
		Status:      enums.StatusOk,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create ai agent error = %v", err)
	}
	return item
}

func createHumanDispatchTeam(t *testing.T, db *gorm.DB, id int64, name string) {
	t.Helper()
	if err := db.Create(&models.AgentTeam{ID: id, TenantID: 1, Name: name, Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create team error = %v", err)
	}
}

func createHumanDispatchActiveSchedule(t *testing.T, db *gorm.DB, teamID int64) {
	t.Helper()
	now := time.Now()
	minute := humanDispatchScheduleMinute(now)
	if err := db.Create(&models.AgentTeamSchedule{
		TenantID:      1,
		TeamID:        teamID,
		RepeatType:    services.AgentTeamScheduleRepeatOnce,
		DayType:       services.AgentTeamScheduleDayTypeWork,
		Weekday:       humanDispatchScheduleWeekday(now),
		StartMinute:   max(0, minute-1),
		EndMinute:     min(24*60, minute+30),
		Timezone:      "Asia/Shanghai",
		PublishStatus: services.AgentTeamSchedulePublishPublished,
		StartAt:       now.Add(-time.Hour),
		EndAt:         now.Add(time.Hour),
		Status:        enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create schedule error = %v", err)
	}
}

func humanDispatchScheduleWeekday(at time.Time) int {
	weekday := int(at.In(time.Local).Weekday())
	if weekday == 0 {
		return 7
	}
	return weekday
}

func humanDispatchScheduleMinute(at time.Time) int {
	local := at.In(time.Local)
	return local.Hour()*60 + local.Minute()
}

func ensureHumanDispatchTenant(t *testing.T, db *gorm.DB, id int64, name string) {
	t.Helper()
	if strings.TrimSpace(name) == "" {
		name = "测试租户"
	}
	now := time.Now()
	item := models.Tenant{
		ID:     id,
		Name:   name,
		Status: enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Where("id = ?", id).FirstOrCreate(&item).Error; err != nil {
		t.Fatalf("ensure tenant error = %v", err)
	}
	if item.Name != name || item.Status != enums.StatusOk {
		if err := db.Model(&models.Tenant{}).Where("id = ?", id).Updates(map[string]any{
			"name":       name,
			"status":     enums.StatusOk,
			"updated_at": now,
		}).Error; err != nil {
			t.Fatalf("update tenant fixture error = %v", err)
		}
	}
}

func ensureHumanDispatchEnterpriseWorkTime(t *testing.T, db *gorm.DB, tenantID int64, at time.Time) {
	t.Helper()
	local := at.In(time.FixedZone("CST", 8*60*60))
	weekday := int(local.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	minute := local.Hour()*60 + local.Minute()
	startMinute := minute - 1
	if startMinute < 0 {
		startMinute = 0
	}
	endMinute := minute + 30
	if endMinute > 24*60 {
		endMinute = 24 * 60
	}
	if endMinute <= startMinute {
		startMinute = 0
		endMinute = 24 * 60
	}
	if err := db.Create(&models.AgentTeamScheduleTemplate{
		TenantID:    tenantID,
		Workdays:    "[" + strconv.Itoa(weekday) + "]",
		StartMinute: startMinute,
		EndMinute:   endMinute,
		Timezone:    services.EngineerScheduleTimezone,
		Status:      enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create enterprise work-time template: %v", err)
	}
}

func ensureHumanDispatchCustomer(t *testing.T, db *gorm.DB, id int64, name string) {
	t.Helper()
	if strings.TrimSpace(name) == "" {
		name = "测试访客"
	}
	now := time.Now()
	item := models.Customer{
		ID:     id,
		Name:   name,
		Status: enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Where("id = ?", id).FirstOrCreate(&item).Error; err != nil {
		t.Fatalf("ensure customer error = %v", err)
	}
	if item.Name != name || item.Status != enums.StatusOk {
		if err := db.Model(&models.Customer{}).Where("id = ?", id).Updates(map[string]any{
			"name":       name,
			"status":     enums.StatusOk,
			"updated_at": now,
		}).Error; err != nil {
			t.Fatalf("update customer fixture error = %v", err)
		}
	}
}

func createHumanDispatchAgentProfile(t *testing.T, db *gorm.DB, userID, teamID int64, serviceStatus enums.ServiceStatus, maxConcurrent int, autoAssign bool, status enums.Status) {
	t.Helper()
	now := time.Now()
	var team models.AgentTeam
	if err := db.First(&team, teamID).Error; err != nil {
		t.Fatalf("load profile team error = %v", err)
	}
	if err := db.Create(&models.User{
		ID:       userID,
		Username: "agent-" + strconv.FormatInt(userID, 10),
		Nickname: "客服",
		Status:   enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create user error = %v", err)
	}
	if err := db.Create(&models.AgentProfile{
		TenantID:           team.TenantID,
		UserID:             userID,
		TeamID:             teamID,
		AgentCode:          "A" + strconv.FormatInt(userID, 10),
		DisplayName:        "客服",
		ServiceStatus:      serviceStatus,
		MaxConcurrentCount: maxConcurrent,
		AutoAssignEnabled:  autoAssign,
		LastOnlineAt:       &now,
		Status:             status,
	}).Error; err != nil {
		t.Fatalf("create profile error = %v", err)
	}
	if status == enums.StatusOk && teamID > 0 {
		if err := db.Create(&models.AgentTeamMember{
			TenantID:        team.TenantID,
			TeamID:          teamID,
			UserID:          userID,
			DispatchEnabled: autoAssign,
			DispatchWeight:  1,
			Status:          enums.StatusOk,
		}).Error; err != nil {
			t.Fatalf("create team member error = %v", err)
		}
	}
	if err := db.Create(&models.AgentWorkStatus{
		TenantID: team.TenantID, UserID: userID, Status: services.AgentWorkStatusAvailable,
		ConfirmedAt: now, StatusChangedAt: now,
	}).Error; err != nil {
		t.Fatalf("create profile work status error = %v", err)
	}
}

func createConversationDispatchEngineerCapability(t *testing.T, db *gorm.DB, userID int64, skillTagsJSON, regionsJSON string, enabled bool) {
	t.Helper()
	now := time.Now()
	member := models.TenantMember{
		TenantID: 1, UserID: userID, MemberNo: "M" + strconv.FormatInt(userID, 10),
		DisplayName: "工程师", MemberType: "employee", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&member).Error; err != nil {
		t.Fatalf("create tenant member: %v", err)
	}
	engineer := models.EngineerProfile{
		TenantID: 1, MemberID: member.ID, SkillTagsJSON: skillTagsJSON, ServiceRegionsJSON: regionsJSON,
		LanguagesJSON: "[]", Timezone: "UTC", MaxTicketLoad: 3, DispatchEnabled: enabled, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&engineer).Error; err != nil {
		t.Fatalf("create engineer profile: %v", err)
	}
}

func createHumanDispatchConversation(t *testing.T, db *gorm.DB, aiAgentID int64, status enums.IMConversationStatus) models.Conversation {
	t.Helper()
	now := time.Now()
	ensureHumanDispatchCustomer(t, db, 1, "测试访客")
	item := models.Conversation{
		TenantID:      1,
		AIAgentID:     aiAgentID,
		ChannelID:     1,
		CustomerID:    1,
		CustomerName:  "测试访客",
		Status:        status,
		ServiceMode:   enums.IMConversationServiceModeAIFirst,
		LastMessageAt: now,
		LastActiveAt:  now,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create conversation error = %v", err)
	}
	return item
}

func testHumanDispatchOperator() *dto.AuthPrincipal {
	return &dto.AuthPrincipal{TenantID: 1, UserID: 9, Username: "dispatcher", Nickname: "调度员"}
}
