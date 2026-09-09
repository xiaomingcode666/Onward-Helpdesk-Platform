package services

import (
	"encoding/base64"
	"fmt"
	"io"
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
	"remotehelpdesk/internal/pkg/openidentity"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestAllowAIMessageOnPendingHandoff(t *testing.T) {
	conversation := &models.Conversation{
		Status:            enums.IMConversationStatusPending,
		CurrentAssigneeID: 0,
		HandoffAt:         ptrTime(time.Now()),
	}
	if !MessageService.allowAIMessageOnPendingHandoff(conversation) {
		t.Fatalf("expected pending handoff conversation to allow ai handoff notice")
	}

	conversation.Status = enums.IMConversationStatusAIServing
	if MessageService.allowAIMessageOnPendingHandoff(conversation) {
		t.Fatalf("expected ai serving conversation not to use pending handoff allowance")
	}
}

func TestCustomerSenderAuthorizationPrecedesClosedStateValidation(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	now := time.Now()
	customers := []models.Customer{
		{Name: "Customer One", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{Name: "Customer Two", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	for index := range customers {
		if err := db.Create(&customers[index]).Error; err != nil {
			t.Fatalf("create customer: %v", err)
		}
	}
	external := openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     "tenant-one-customer",
		ExternalName:   "Tenant One Customer",
	}
	if err := db.Create(&models.CustomerIdentity{
		CustomerID:     customers[0].ID,
		ExternalSource: external.ExternalSource,
		ExternalID:     external.ExternalID,
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create customer identity: %v", err)
	}
	foreignConversation := &models.Conversation{
		TenantID:     2,
		CustomerID:   customers[1].ID,
		Status:       enums.IMConversationStatusClosed,
		LastActiveAt: now,
		AuditFields:  models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(foreignConversation).Error; err != nil {
		t.Fatalf("create foreign conversation: %v", err)
	}

	_, err := MessageService.ValidateConversationSender(foreignConversation.ID, enums.IMSenderTypeCustomer, nil, &external)
	if err == nil {
		t.Fatal("foreign customer unexpectedly passed sender validation")
	}
	if strings.Contains(err.Error(), "会话已关闭") {
		t.Fatalf("foreign customer learned conversation state before authorization: %v", err)
	}
}

func TestValidateConversationAssetEnforcesTenantAndMediaType(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	now := time.Now()
	conversation := &models.Conversation{
		TenantID:     21,
		Status:       enums.IMConversationStatusActive,
		LastActiveAt: now,
		AuditFields:  models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	audio := &models.Asset{TenantID: 21, ConversationID: conversation.ID, Status: enums.AssetStatusSuccess, MimeType: "audio/webm"}
	if err := validateConversationAsset(audio, conversation.ID, enums.IMMessageTypeAudio); err != nil {
		t.Fatalf("same-tenant audio asset rejected: %v", err)
	}

	otherConversationAsset := &models.Asset{TenantID: 21, ConversationID: conversation.ID + 1, Status: enums.AssetStatusSuccess, MimeType: "audio/webm"}
	if err := validateConversationAsset(otherConversationAsset, conversation.ID, enums.IMMessageTypeAudio); err == nil {
		t.Fatal("asset belonging to another conversation unexpectedly passed validation")
	}

	foreignAudio := &models.Asset{TenantID: 22, Status: enums.AssetStatusSuccess, MimeType: "audio/webm"}
	if err := validateConversationAsset(foreignAudio, conversation.ID, enums.IMMessageTypeAudio); err == nil {
		t.Fatal("cross-tenant audio asset unexpectedly accepted")
	}

	notAudio := &models.Asset{TenantID: 21, Status: enums.AssetStatusSuccess, MimeType: "image/png"}
	if err := validateConversationAsset(notAudio, conversation.ID, enums.IMMessageTypeAudio); err == nil {
		t.Fatal("non-audio asset unexpectedly accepted as an audio message")
	}
}

func TestForwardAgentImageClonesIntoTargetConversationAndIsIdempotent(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	now := time.Now()
	operator := &dto.AuthPrincipal{
		TenantID: 31, UserID: 501, Username: "forward-engineer", DomainType: models.DomainTypeEnterprise,
	}
	conversations := []models.Conversation{
		{TenantID: 31, Status: enums.IMConversationStatusActive, CurrentAssigneeID: operator.UserID, LastActiveAt: now},
		{TenantID: 31, Status: enums.IMConversationStatusActive, CurrentAssigneeID: operator.UserID, LastActiveAt: now},
		{TenantID: 32, Status: enums.IMConversationStatusActive, CurrentAssigneeID: operator.UserID, LastActiveAt: now},
	}
	if err := db.Create(&conversations).Error; err != nil {
		t.Fatalf("create conversations: %v", err)
	}

	storageRoot := t.TempDir()
	previousConfig := config.CurrentOrDefault()
	config.SetCurrent(&config.Config{Storage: config.StorageConfig{
		Default: enums.AssetProviderLocal,
		Local:   config.LocalStorageConfig{Root: storageRoot},
	}})
	t.Cleanup(func() { config.SetCurrent(&previousConfig) })
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatalf("decode png fixture: %v", err)
	}
	writeMediaTestFile(t, storageRoot, "images/source.png", string(png))
	sourceAsset := &models.Asset{
		TenantID: 31, ConversationID: conversations[0].ID, AssetID: "forward-source-asset",
		Provider: enums.AssetProviderLocal, StorageKey: "images/source.png", Filename: "source.png",
		FileSize: int64(len(png)), MimeType: "image/png", Status: enums.AssetStatusSuccess,
	}
	if err := db.Create(sourceAsset).Error; err != nil {
		t.Fatalf("create source asset: %v", err)
	}
	sourceMessage := &models.Message{
		ConversationID: conversations[0].ID, ClientMsgID: "forward-source-message",
		SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeImage,
		Content: sourceAsset.Filename, Payload: `{"assetId":"forward-source-asset"}`,
		SendStatus: enums.IMMessageStatusSent, SentAt: &now,
	}
	sourceMessage.Payload, err = buildIMMessageAssetPayload(sourceAsset)
	if err != nil {
		t.Fatalf("build source message payload: %v", err)
	}
	if err := db.Create(sourceMessage).Error; err != nil {
		t.Fatalf("create source message: %v", err)
	}

	forwarded, err := MessageService.ForwardAgentImage(
		sourceMessage.ID, conversations[1].ID, "forward-target-message", operator, "forward-request",
	)
	if err != nil {
		t.Fatalf("forward image: %v", err)
	}
	if forwarded == nil || forwarded.ConversationID != conversations[1].ID || forwarded.MessageType != enums.IMMessageTypeImage {
		t.Fatalf("forwarded message=%+v", forwarded)
	}
	forwardedPayload, err := parseIMMessageAssetPayload(forwarded.Payload)
	if err != nil {
		t.Fatalf("parse forwarded payload: %v", err)
	}
	if forwardedPayload.AssetID == sourceAsset.AssetID {
		t.Fatal("cross-conversation forward reused the source asset")
	}
	forwardedAsset := AssetService.GetByAssetID(forwardedPayload.AssetID)
	if forwardedAsset == nil || forwardedAsset.ConversationID != conversations[1].ID || forwardedAsset.TenantID != operator.TenantID {
		t.Fatalf("forwarded asset=%+v", forwardedAsset)
	}
	reader, err := AssetService.OpenReader(forwardedAsset)
	if err != nil {
		t.Fatalf("open forwarded asset: %v", err)
	}
	forwardedBytes, readErr := io.ReadAll(reader)
	_ = reader.Close()
	if readErr != nil || string(forwardedBytes) != string(png) {
		t.Fatalf("forwarded bytes match=%v readErr=%v", string(forwardedBytes) == string(png), readErr)
	}

	retry, err := MessageService.ForwardAgentImage(
		sourceMessage.ID, conversations[1].ID, "forward-target-message", operator, "forward-retry",
	)
	if err != nil || retry == nil || retry.ID != forwarded.ID {
		t.Fatalf("idempotent retry message=%+v err=%v", retry, err)
	}
	unauthorizedOperator := &dto.AuthPrincipal{
		TenantID: 31, UserID: 777, Username: "unassigned-engineer", DomainType: models.DomainTypeEnterprise,
	}
	if leaked, err := MessageService.ForwardAgentImage(
		sourceMessage.ID, conversations[1].ID, "forward-target-message", unauthorizedOperator, "unauthorized-retry",
	); err == nil || leaked != nil {
		t.Fatalf("unauthorized idempotent retry leaked message=%+v err=%v", leaked, err)
	}
	var assetCount int64
	if err := db.Model(&models.Asset{}).Count(&assetCount).Error; err != nil || assetCount != 2 {
		t.Fatalf("asset count=%d err=%v, want 2", assetCount, err)
	}
	if _, err := MessageService.ForwardAgentImage(
		sourceMessage.ID, conversations[2].ID, "forward-cross-tenant", operator, "forward-cross-tenant",
	); err == nil {
		t.Fatal("cross-tenant image forward unexpectedly succeeded")
	}
	if err := AssetService.DeleteAsset(sourceAsset.ID, &dto.AuthPrincipal{TenantID: 32, DomainType: models.DomainTypeEnterprise}); err == nil {
		t.Fatal("cross-tenant asset deletion unexpectedly succeeded")
	}
}

func ptrTime(v time.Time) *time.Time {
	return &v
}

func setupMessageWelcomeTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := "message_welcome_test_" + strings.NewReplacer("/", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",
			SingularTable: true,
		},
	})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite db: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("close sqlite db: %v", err)
		}
	})
	if err := db.AutoMigrate(
		&models.AIAgent{},
		&models.AIAgentRelease{},
		&models.AIWorkflow{},
		&models.AIWorkflowVersion{},
		&models.Asset{},
		&models.Channel{},
		&models.ChannelMessageOutbox{},
		&models.Customer{},
		&models.CustomerIdentity{},
		&models.CustomerOrg{},
		&models.CustomerUser{},
		&models.CustomerEntrySession{},
		&models.CustomerPrivacyConsent{},
		&models.Conversation{},
		&models.ConversationParticipant{},
		&models.ConversationReadState{},
		&models.ConversationEventLog{},
		&models.Device{},
		&models.ProductServiceProfile{},
		&models.ServiceCode{},
		&models.CustomerDeviceBinding{},
		&models.Message{},
		&models.AIReplyJob{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	return db
}

func TestNormalizeHTMLMessageRejectsAssetFromAnotherConversation(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	now := time.Now()
	conversations := []models.Conversation{
		{TenantID: 21, LastActiveAt: now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 21, LastActiveAt: now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&conversations).Error; err != nil {
		t.Fatalf("create conversations: %v", err)
	}
	asset := &models.Asset{
		TenantID: 21, ConversationID: conversations[1].ID, AssetID: "other-conversation-image",
		Provider: enums.AssetProviderLocal, StorageKey: "images/other.png", Filename: "other.png",
		FileSize: 10, MimeType: "image/png", Status: enums.AssetStatusSuccess,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}

	_, _, _, err := MessageService.normalizeMessageContent(
		conversations[0].ID,
		enums.IMMessageTypeHTML,
		`<p><img data-asset-id="other-conversation-image" alt="other"></p>`,
		"",
	)
	if err == nil {
		t.Fatal("html image belonging to another conversation unexpectedly passed validation")
	}

	if err := db.Model(asset).Update("conversation_id", conversations[0].ID).Error; err != nil {
		t.Fatalf("move asset into current conversation: %v", err)
	}
	if _, _, _, err := MessageService.normalizeMessageContent(
		conversations[0].ID,
		enums.IMMessageTypeHTML,
		`<p><img data-asset-id="other-conversation-image" alt="current"></p>`,
		"",
	); err != nil {
		t.Fatalf("html image belonging to current conversation was rejected: %v", err)
	}
}

func createWelcomeTestAIAgent(t *testing.T, db *gorm.DB, welcomeMessage string) *models.AIAgent {
	t.Helper()

	now := time.Now()
	aiAgent := &models.AIAgent{
		Name:           "welcome-test-agent",
		Status:         enums.StatusOk,
		ServiceMode:    enums.IMConversationServiceModeAIOnly,
		WelcomeMessage: welcomeMessage,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(aiAgent).Error; err != nil {
		t.Fatalf("create ai agent: %v", err)
	}
	return aiAgent
}

func createWelcomeTestActiveRelease(t *testing.T, db *gorm.DB, agent *models.AIAgent) *models.AIAgentRelease {
	t.Helper()
	if agent == nil || agent.WorkflowID <= 0 || agent.WorkflowVersionID <= 0 {
		t.Fatalf("agent must have a workflow before deployment: %+v", agent)
	}
	now := time.Now()
	agentSnapshot, agentHash, err := buildAgentReleaseConfigSnapshot(agent)
	if err != nil {
		t.Fatalf("build active AI agent snapshot: %v", err)
	}
	knowledgeSnapshot, knowledgeHash, err := marshalSnapshot(dto.AIAgentReleaseKnowledgeScopeSnapshot{
		SchemaVersion: 1,
		TenantID:      agent.TenantID,
		ProductID:     agent.ProductID,
		Resolution:    "test_release_scope",
		Bindings:      []dto.AIAgentReleaseKnowledgeBindingSnapshot{},
		Revisions:     []dto.AIAgentReleaseKnowledgeRevisionSnapshot{},
	})
	if err != nil {
		t.Fatalf("build active AI knowledge snapshot: %v", err)
	}
	workflow := repositories.AIWorkflowRepository.Get(db, agent.WorkflowID)
	if workflow == nil {
		workflow = &models.AIWorkflow{
			ID: agent.WorkflowID, TenantID: agent.TenantID, Code: fmt.Sprintf("welcome-test-%d", agent.WorkflowID),
			Scope: models.AIWorkflowScopeTenant, Name: "Welcome test workflow", Status: enums.StatusOk,
			AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
		}
		if err := db.Create(workflow).Error; err != nil {
			t.Fatalf("create active AI workflow: %v", err)
		}
	}
	version := repositories.AIWorkflowVersionRepository.Get(db, agent.WorkflowVersionID)
	if version == nil {
		definition := `{"schemaVersion":"1.0","nodes":[],"edges":[]}`
		version = &models.AIWorkflowVersion{
			ID: agent.WorkflowVersionID, WorkflowID: agent.WorkflowID, Version: 1,
			Definition: definition, DefinitionHash: hashString(definition),
			ReleaseChannel: models.AIWorkflowReleaseChannelStable, Status: enums.StatusOk,
			AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
		}
		if err := db.Create(version).Error; err != nil {
			t.Fatalf("create active AI workflow version: %v", err)
		}
		if err := db.Model(workflow).Update("current_stable_version_id", version.ID).Error; err != nil {
			t.Fatalf("activate AI workflow version: %v", err)
		}
	}
	release := &models.AIAgentRelease{
		TenantID:               agent.TenantID,
		ProductID:              agent.ProductID,
		AgentID:                agent.ID,
		ReleaseNo:              1,
		WorkflowID:             agent.WorkflowID,
		WorkflowVersionID:      agent.WorkflowVersionID,
		WorkflowDefinitionHash: version.DefinitionHash,
		AgentConfigSnapshot:    agentSnapshot,
		AgentConfigHash:        agentHash,
		KnowledgeScopeSnapshot: knowledgeSnapshot,
		KnowledgeScopeHash:     knowledgeHash,
		ReviewStatus:           enums.AIAgentReviewStatusApproved,
		DeploymentStatus:       models.AIAgentReleaseDeploymentActive,
		Status:                 enums.StatusOk,
		AuditFields:            models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(release).Error; err != nil {
		t.Fatalf("create active AI agent release: %v", err)
	}
	if err := db.Model(agent).Update("active_release_id", release.ID).Error; err != nil {
		t.Fatalf("activate AI agent release: %v", err)
	}
	agent.ActiveReleaseID = release.ID
	return release
}

func welcomeTestExternalUser(id string) openidentity.ExternalUser {
	return openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     id,
		ExternalName:   "访客" + id,
	}
}

func createMessageTestConversation(t *testing.T, db *gorm.DB, aiAgentID int64) *models.Conversation {
	t.Helper()
	now := time.Now()
	conversation := &models.Conversation{
		CustomerID:   1,
		ChannelID:    11,
		AIAgentID:    aiAgentID,
		Status:       enums.IMConversationStatusAIServing,
		LastActiveAt: now,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	return conversation
}

func workflowTestAIPrincipal() *dto.AuthPrincipal {
	return &dto.AuthPrincipal{UserID: 0, Username: "AI", Nickname: "AI"}
}

func TestAgentConversationParticipantCanReplyWithoutBecomingPrimaryAssignee(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	now := time.Now()
	conversation := &models.Conversation{
		TenantID:          9,
		Status:            enums.IMConversationStatusActive,
		CurrentAssigneeID: 101,
		LastActiveAt:      now,
		AuditFields:       models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	primary := &dto.AuthPrincipal{TenantID: 9, UserID: 101, Username: "primary-engineer"}
	coworker := &dto.AuthPrincipal{TenantID: 9, UserID: 202, Username: "support-engineer"}
	if err := ConversationService.ensureAgentParticipantTx(db, conversation.ID, primary.UserID, now, primary); err != nil {
		t.Fatalf("ensure primary participant: %v", err)
	}
	if err := ConversationService.ensureAgentParticipantTx(db, conversation.ID, coworker.UserID, now, primary); err != nil {
		t.Fatalf("ensure co-worker participant: %v", err)
	}
	message, err := MessageService.SendAgentMessage(conversation.ID, coworker.UserID, "agent-coworker-1", enums.IMMessageTypeText, "协同工程师补充排查结果", "", coworker)
	if err != nil {
		t.Fatalf("co-worker SendAgentMessage() error = %v", err)
	}
	if message == nil || message.SenderID != coworker.UserID || message.Content != "协同工程师补充排查结果" {
		t.Fatalf("unexpected co-worker message: %+v", message)
	}
	outsider := &dto.AuthPrincipal{TenantID: 9, UserID: 303, Username: "outside-engineer"}
	if _, err := MessageService.SendAgentMessage(conversation.ID, outsider.UserID, "agent-outsider-1", enums.IMMessageTypeText, "不应发送", "", outsider); err == nil {
		t.Fatal("expected a non-participant engineer reply to be rejected")
	}
}

func TestConversationCreateCreatesAIWelcomeMessage(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	aiAgent := createWelcomeTestAIAgent(t, db, "  您好，请问有什么可以帮您？  ")

	conversation, err := ConversationService.Create(welcomeTestExternalUser("welcome-1"), 11, aiAgent.ID)
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if conversation == nil {
		t.Fatalf("expected conversation")
	}

	var messages []models.Message
	if err := db.Find(&messages).Error; err != nil {
		t.Fatalf("find messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected exactly one welcome message, got %d", len(messages))
	}
	message := messages[0]
	if message.ConversationID != conversation.ID {
		t.Fatalf("expected conversation_id %d, got %d", conversation.ID, message.ConversationID)
	}
	if message.SenderType != enums.IMSenderTypeAI {
		t.Fatalf("expected sender type ai, got %q", message.SenderType)
	}
	if message.SenderID != aiAgent.ID {
		t.Fatalf("expected sender id %d, got %d", aiAgent.ID, message.SenderID)
	}
	if message.MessageType != enums.IMMessageTypeText {
		t.Fatalf("expected message type text, got %q", message.MessageType)
	}
	if message.Content != "您好，请问有什么可以帮您？" {
		t.Fatalf("expected trimmed welcome content, got %q", message.Content)
	}
	if message.SendStatus != enums.IMMessageStatusSent {
		t.Fatalf("expected sent status, got %d", message.SendStatus)
	}

	var updated models.Conversation
	if err := db.First(&updated, conversation.ID).Error; err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	if updated.LastMessageID != message.ID {
		t.Fatalf("expected last message id %d, got %d", message.ID, updated.LastMessageID)
	}
	if updated.LastMessageSummary != "您好，请问有什么可以帮您？" {
		t.Fatalf("expected last message summary, got %q", updated.LastMessageSummary)
	}
	if updated.CustomerUnreadCount != 1 {
		t.Fatalf("expected customer unread count 1, got %d", updated.CustomerUnreadCount)
	}
	if updated.AgentUnreadCount != 0 {
		t.Fatalf("expected agent unread count 0, got %d", updated.AgentUnreadCount)
	}
}

func TestConversationCreateWithDeviceContextKeepsDeviceScopedSessions(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	aiAgent := createWelcomeTestAIAgent(t, db, "")
	now := time.Now()
	productAgent := createWelcomeTestAIAgent(t, db, "产品设备诊断已开始。")
	if err := db.Model(productAgent).Updates(map[string]any{
		"tenant_id":           9,
		"product_id":          101,
		"review_status":       enums.AIAgentReviewStatusApproved,
		"workflow_id":         1,
		"workflow_version_id": 1,
	}).Error; err != nil {
		t.Fatalf("scope product agent: %v", err)
	}
	productAgent = repositories.AIAgentRepository.Get(db, productAgent.ID)
	createWelcomeTestActiveRelease(t, db, productAgent)
	if err := db.Model(aiAgent).Updates(map[string]any{
		"tenant_id":           9,
		"product_id":          102,
		"review_status":       enums.AIAgentReviewStatusApproved,
		"workflow_id":         2,
		"workflow_version_id": 2,
	}).Error; err != nil {
		t.Fatalf("scope second product agent: %v", err)
	}
	aiAgent = repositories.AIAgentRepository.Get(db, aiAgent.ID)
	createWelcomeTestActiveRelease(t, db, aiAgent)
	profiles := []models.ProductServiceProfile{
		{TenantID: 9, ProductID: 101, DefaultAIAgentID: productAgent.ID, Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 9, ProductID: 102, DefaultAIAgentID: aiAgent.ID, Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&profiles).Error; err != nil {
		t.Fatalf("create product service profiles: %v", err)
	}

	deviceA := &models.Device{
		TenantID:       9,
		DeviceNo:       "DEV-A",
		ProductID:      101,
		ProductModelID: 1001,
		Status:         enums.StatusOk,
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	deviceB := &models.Device{
		TenantID:       9,
		DeviceNo:       "DEV-B",
		ProductID:      102,
		ProductModelID: 1002,
		Status:         enums.StatusOk,
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(deviceA).Error; err != nil {
		t.Fatalf("create deviceA: %v", err)
	}
	if err := db.Create(deviceB).Error; err != nil {
		t.Fatalf("create deviceB: %v", err)
	}
	serviceCodeA := &models.ServiceCode{
		TenantID:       9,
		ServiceCode:    "SC-DEV-A",
		DeviceID:       deviceA.ID,
		ProductID:      deviceA.ProductID,
		ProductModelID: deviceA.ProductModelID,
		Status:         enums.ServiceCodeStatusActive,
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(serviceCodeA).Error; err != nil {
		t.Fatalf("create service code: %v", err)
	}
	for _, deviceID := range []int64{deviceA.ID, deviceB.ID} {
		binding := &models.CustomerDeviceBinding{
			TenantID:       9,
			DeviceID:       deviceID,
			CustomerUserID: 1001,
			Status:         enums.StatusOk,
			ConfirmedAt:    &now,
			AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
		}
		if err := db.Create(binding).Error; err != nil {
			t.Fatalf("create binding for device %d: %v", deviceID, err)
		}
	}

	external := openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     "1001",
		ExternalName:   "设备客户",
	}

	conversationA, err := ConversationService.CreateWithContext(external, 11, aiAgent.ID, request.CreateOrMatchConversationRequest{
		DeviceID: deviceA.ID,
	})
	if err != nil {
		t.Fatalf("create deviceA conversation: %v", err)
	}
	if conversationA.DeviceID != deviceA.ID || conversationA.TenantID != deviceA.TenantID {
		t.Fatalf("conversationA context mismatch: %#v", conversationA)
	}
	if conversationA.ProductID != deviceA.ProductID || conversationA.ProductModelID != deviceA.ProductModelID {
		t.Fatalf("conversationA product context mismatch: %#v", conversationA)
	}
	if conversationA.AIAgentID != productAgent.ID {
		t.Fatalf("device-bound conversation agent = %d, want product agent %d", conversationA.AIAgentID, productAgent.ID)
	}
	if conversationA.ServiceCodeID != serviceCodeA.ID {
		t.Fatalf("conversationA service code = %d, want %d", conversationA.ServiceCodeID, serviceCodeA.ID)
	}

	matchedA, err := ConversationService.CreateWithContext(external, 11, aiAgent.ID, request.CreateOrMatchConversationRequest{
		DeviceID: deviceA.ID,
	})
	if err != nil {
		t.Fatalf("match deviceA conversation: %v", err)
	}
	if matchedA.ID != conversationA.ID {
		t.Fatalf("expected same device to reuse conversation %d, got %d", conversationA.ID, matchedA.ID)
	}

	forcedConversationA, err := ConversationService.CreateWithContext(external, 11, aiAgent.ID, request.CreateOrMatchConversationRequest{
		DeviceID: deviceA.ID,
		ForceNew: true,
	})
	if err != nil {
		t.Fatalf("force new deviceA conversation: %v", err)
	}
	if forcedConversationA.ID == conversationA.ID {
		t.Fatalf("forceNew must create a separate device conversation, reused %d", conversationA.ID)
	}
	if forcedConversationA.DeviceID != deviceA.ID || forcedConversationA.TenantID != deviceA.TenantID {
		t.Fatalf("forced conversation context mismatch: %#v", forcedConversationA)
	}

	closedAt := time.Now()
	if err := repositories.ConversationRepository.Updates(db, conversationA.ID, map[string]any{
		"status":     enums.IMConversationStatusClosed,
		"closed_at":  &closedAt,
		"updated_at": closedAt,
	}); err != nil {
		t.Fatalf("close deviceA conversation: %v", err)
	}
	reopenedConversationA, err := ConversationService.CreateWithContext(external, 11, aiAgent.ID, request.CreateOrMatchConversationRequest{
		DeviceID: deviceA.ID,
		ForceNew: true,
	})
	if err != nil {
		t.Fatalf("reopen deviceA conversation after close: %v", err)
	}
	if reopenedConversationA.ID == conversationA.ID {
		t.Fatalf("closed conversation should permit a new session, reused %d", conversationA.ID)
	}
	if reopenedConversationA.DeviceID != deviceA.ID || reopenedConversationA.TenantID != deviceA.TenantID {
		t.Fatalf("reopened conversation context mismatch: %#v", reopenedConversationA)
	}

	conversationB, err := ConversationService.CreateWithContext(external, 11, aiAgent.ID, request.CreateOrMatchConversationRequest{
		DeviceID: deviceB.ID,
	})
	if err != nil {
		t.Fatalf("create deviceB conversation: %v", err)
	}
	if conversationB.ID == conversationA.ID {
		t.Fatalf("different devices should not share the same conversation")
	}
	if conversationB.DeviceID != deviceB.ID || conversationB.ServiceCodeID != 0 {
		t.Fatalf("conversationB context mismatch: %#v", conversationB)
	}
	if conversationB.AIAgentID != aiAgent.ID {
		t.Fatalf("deviceB conversation agent = %d, want product agent %d", conversationB.AIAgentID, aiAgent.ID)
	}
}

func TestConversationCreateWithDeviceRejectsAnotherCustomerAccount(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	now := time.Now()
	productAgent := createWelcomeTestAIAgent(t, db, "产品设备诊断已开始。")
	if err := db.Model(productAgent).Updates(map[string]any{
		"tenant_id":           9,
		"product_id":          101,
		"review_status":       enums.AIAgentReviewStatusApproved,
		"workflow_id":         1,
		"workflow_version_id": 1,
	}).Error; err != nil {
		t.Fatalf("scope product agent: %v", err)
	}
	productAgent = repositories.AIAgentRepository.Get(db, productAgent.ID)
	createWelcomeTestActiveRelease(t, db, productAgent)
	if err := db.Create(&models.ProductServiceProfile{
		TenantID: 9, ProductID: 101, DefaultAIAgentID: productAgent.ID,
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create product service profile: %v", err)
	}
	device := &models.Device{
		TenantID: 9, DeviceNo: "DEV-PRIVATE", ProductID: 101, ProductModelID: 1001,
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(device).Error; err != nil {
		t.Fatalf("create private device: %v", err)
	}
	if err := db.Create(&models.CustomerDeviceBinding{
		TenantID: 9, DeviceID: device.ID, CustomerUserID: 1001, Status: enums.StatusOk,
		ConfirmedAt: &now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create owner binding: %v", err)
	}

	_, err := ConversationService.CreateWithContext(openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     "2002",
		ExternalName:   "未绑定客户",
	}, 11, productAgent.ID, request.CreateOrMatchConversationRequest{DeviceID: device.ID})
	if err == nil || !strings.Contains(err.Error(), "该设备未绑定到当前客户账号") {
		t.Fatalf("foreign customer device conversation error = %v", err)
	}
	var conversationCount int64
	if err := db.Model(&models.Conversation{}).Count(&conversationCount).Error; err != nil {
		t.Fatalf("count conversations: %v", err)
	}
	if conversationCount != 0 {
		t.Fatalf("foreign customer created %d conversations", conversationCount)
	}
}

func TestConversationCreateWithDeviceAcceptsOrganizationWideBinding(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	now := time.Now()
	productAgent := createWelcomeTestAIAgent(t, db, "产品设备诊断已开始。")
	if err := db.Model(productAgent).Updates(map[string]any{
		"tenant_id":           9,
		"product_id":          101,
		"review_status":       enums.AIAgentReviewStatusApproved,
		"workflow_id":         1,
		"workflow_version_id": 1,
	}).Error; err != nil {
		t.Fatalf("scope product agent: %v", err)
	}
	productAgent = repositories.AIAgentRepository.Get(db, productAgent.ID)
	createWelcomeTestActiveRelease(t, db, productAgent)
	if err := db.Create(&models.ProductServiceProfile{
		TenantID: 9, ProductID: 101, DefaultAIAgentID: productAgent.ID,
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create product service profile: %v", err)
	}
	customer := &models.Customer{
		Name: "华东工厂", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	if err := db.Create(&models.CustomerIdentity{
		CustomerID: customer.ID, ExternalSource: enums.ExternalSourceUser, ExternalID: "2001",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create customer identity: %v", err)
	}
	org := &models.CustomerOrg{
		TenantID: 9, CustomerNo: "CUST-EAST", Name: "华东工厂",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(org).Error; err != nil {
		t.Fatalf("create customer org: %v", err)
	}
	user := &models.CustomerUser{
		TenantID: 9, CustomerOrgID: org.ID, UserID: 2001, DisplayName: "华东工厂设备主管",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create customer user: %v", err)
	}
	device := &models.Device{
		TenantID: 9, DeviceNo: "DEV-ORG", ProductID: 101, ProductModelID: 1001,
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(device).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err := db.Create(&models.CustomerDeviceBinding{
		TenantID: 9, DeviceID: device.ID, CustomerOrgID: org.ID, CustomerUserID: 0,
		Status: enums.StatusOk, ConfirmedAt: &now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create organization binding: %v", err)
	}

	conversation, err := ConversationService.CreateWithContext(openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     "2001",
		ExternalName:   "华东工厂设备主管",
	}, 11, productAgent.ID, request.CreateOrMatchConversationRequest{DeviceID: device.ID, ForceNew: true})
	if err != nil {
		t.Fatalf("organization-wide device conversation rejected: %v", err)
	}
	if conversation.DeviceID != device.ID || conversation.ProductID != device.ProductID || conversation.AIAgentID != productAgent.ID {
		t.Fatalf("conversation context mismatch: %+v", conversation)
	}
}

func TestConversationCreateWithDeviceAcceptsAnotherUserBindingVisibleToSameOrganization(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	now := time.Now()
	productAgent := createWelcomeTestAIAgent(t, db, "产品设备诊断已开始。")
	if err := db.Model(productAgent).Updates(map[string]any{
		"tenant_id":           9,
		"product_id":          101,
		"review_status":       enums.AIAgentReviewStatusApproved,
		"workflow_id":         1,
		"workflow_version_id": 1,
	}).Error; err != nil {
		t.Fatalf("scope product agent: %v", err)
	}
	productAgent = repositories.AIAgentRepository.Get(db, productAgent.ID)
	createWelcomeTestActiveRelease(t, db, productAgent)
	if err := db.Create(&models.ProductServiceProfile{
		TenantID: 9, ProductID: 101, DefaultAIAgentID: productAgent.ID,
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create product service profile: %v", err)
	}
	customer := &models.Customer{
		ID: 777, Name: "华东工厂", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	if err := db.Create(&models.CustomerIdentity{
		CustomerID: customer.ID, ExternalSource: enums.ExternalSourceUser, ExternalID: "legacy-customer",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create customer identity: %v", err)
	}
	device := &models.Device{
		TenantID: 9, DeviceNo: "DEV-USER-BOUND", ProductID: 101, ProductModelID: 1001,
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(device).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err := db.Create(&models.CustomerDeviceBinding{
		TenantID: 9, DeviceID: device.ID, CustomerOrgID: customer.ID, CustomerUserID: 888,
		Status: enums.StatusOk, ConfirmedAt: &now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create user-scoped binding: %v", err)
	}

	conversation, err := ConversationService.CreateWithContext(openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     "legacy-customer",
		ExternalName:   "华东工厂设备主管",
	}, customer.ID, productAgent.ID, request.CreateOrMatchConversationRequest{DeviceID: device.ID, ForceNew: true})
	if err != nil {
		t.Fatalf("same-organization visible device conversation rejected: %v", err)
	}
	if conversation.DeviceID != device.ID || conversation.ProductID != device.ProductID || conversation.AIAgentID != productAgent.ID {
		t.Fatalf("conversation context mismatch: %+v", conversation)
	}
}

func TestConversationCreateConcurrentSameIdempotencyKeyReusesOneActiveConversation(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	now := time.Now()
	productAgent := createWelcomeTestAIAgent(t, db, "产品设备诊断已开始。")
	if err := db.Model(productAgent).Updates(map[string]any{
		"tenant_id":           9,
		"product_id":          101,
		"review_status":       enums.AIAgentReviewStatusApproved,
		"workflow_id":         1,
		"workflow_version_id": 1,
	}).Error; err != nil {
		t.Fatalf("scope product agent: %v", err)
	}
	productAgent = repositories.AIAgentRepository.Get(db, productAgent.ID)
	createWelcomeTestActiveRelease(t, db, productAgent)
	if err := db.Create(&models.ProductServiceProfile{
		TenantID: 9, ProductID: 101, DefaultAIAgentID: productAgent.ID,
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create product service profile: %v", err)
	}
	device := &models.Device{
		TenantID: 9, DeviceNo: "DEV-CONCURRENT", ProductID: 101, ProductModelID: 1001,
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(device).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	customer := &models.Customer{
		ID: 1001, Name: "并发设备客户", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	if err := db.Create(&models.CustomerIdentity{
		CustomerID: customer.ID, ExternalSource: enums.ExternalSourceUser, ExternalID: "1001",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create customer identity: %v", err)
	}
	if err := db.Create(&models.CustomerDeviceBinding{
		TenantID: 9, DeviceID: device.ID, CustomerUserID: 1001, Status: enums.StatusOk,
		ConfirmedAt: &now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create device binding: %v", err)
	}

	external := openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     "1001",
		ExternalName:   "设备客户",
	}
	const idempotencyKey = "conversation-create-concurrent-retry"
	start := make(chan struct{})
	results := make(chan *models.Conversation, 8)
	errorsFound := make(chan error, 8)
	var wait sync.WaitGroup
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			conversation, err := ConversationService.CreateWithContext(external, 11, productAgent.ID, request.CreateOrMatchConversationRequest{
				DeviceID:       device.ID,
				ForceNew:       true,
				IdempotencyKey: idempotencyKey,
			})
			results <- conversation
			errorsFound <- err
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(errorsFound)

	for err := range errorsFound {
		if err != nil {
			t.Fatalf("concurrent create conversation returned error: %v", err)
		}
	}
	var conversationID int64
	for conversation := range results {
		if conversation == nil || conversation.ID <= 0 {
			t.Fatalf("concurrent create returned empty conversation: %+v", conversation)
		}
		if conversationID == 0 {
			conversationID = conversation.ID
		} else if conversation.ID != conversationID {
			t.Fatalf("concurrent create returned different conversations: first=%d current=%d", conversationID, conversation.ID)
		}
	}
	var activeCount int64
	if err := db.Model(&models.Conversation{}).
		Where("tenant_id = ? AND customer_id = ? AND device_id = ? AND status IN ?", 9, customer.ID, device.ID, []enums.IMConversationStatus{
			enums.IMConversationStatusAIServing,
			enums.IMConversationStatusPending,
			enums.IMConversationStatusActive,
		}).
		Count(&activeCount).Error; err != nil {
		t.Fatalf("count active conversations: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("concurrent create persisted %d active conversations, want 1", activeCount)
	}
	var welcomeCount int64
	if err := db.Model(&models.Message{}).Where("conversation_id = ? AND sender_type = ?", conversationID, enums.IMSenderTypeAI).Count(&welcomeCount).Error; err != nil {
		t.Fatalf("count welcome messages: %v", err)
	}
	if welcomeCount != 1 {
		t.Fatalf("concurrent create persisted %d welcome messages, want 1", welcomeCount)
	}
	var createEventCount int64
	if err := db.Model(&models.ConversationEventLog{}).
		Where("conversation_id = ? AND event_type = ? AND request_id = ?", conversationID, enums.IMEventTypeCreate, idempotencyKey).
		Count(&createEventCount).Error; err != nil {
		t.Fatalf("count idempotent create events: %v", err)
	}
	if createEventCount != 1 {
		t.Fatalf("concurrent create persisted %d create events, want 1", createEventCount)
	}
}

func TestConversationCreateWithDeviceRejectsMissingProductAgent(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	fallbackAgent := createWelcomeTestAIAgent(t, db, "不应使用渠道机器人。")
	now := time.Now()
	device := &models.Device{
		TenantID:    9,
		DeviceNo:    "DEV-NO-AGENT",
		ProductID:   103,
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(device).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err := db.Create(&models.CustomerDeviceBinding{
		TenantID:       9,
		DeviceID:       device.ID,
		CustomerUserID: 1001,
		Status:         enums.StatusOk,
		ConfirmedAt:    &now,
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create device binding: %v", err)
	}

	_, err := ConversationService.CreateWithContext(openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     "1001",
		ExternalName:   "设备客户",
	}, 11, fallbackAgent.ID, request.CreateOrMatchConversationRequest{DeviceID: device.ID})
	if err == nil || !strings.Contains(err.Error(), "product AI agent is not configured") {
		t.Fatalf("missing product agent error = %v", err)
	}
	var conversationCount int64
	if err := db.Model(&models.Conversation{}).Count(&conversationCount).Error; err != nil {
		t.Fatalf("count conversations: %v", err)
	}
	if conversationCount != 0 {
		t.Fatalf("missing product agent created %d conversations", conversationCount)
	}
}

func TestConversationCreateWithoutDeviceUsesTenantGeneralAgent(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	generalAgent := createWelcomeTestAIAgent(t, db, "请描述需要协助的问题。")
	now := time.Now()
	workflow := &models.AIWorkflow{
		Code: PlatformDeviceAIOnlyWorkflowCode, Scope: models.AIWorkflowScopePlatform, Name: "AI diagnosis service",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(workflow).Error; err != nil {
		t.Fatalf("create platform diagnosis workflow: %v", err)
	}
	version := &models.AIWorkflowVersion{
		WorkflowID: workflow.ID, Version: 1, Status: enums.StatusOk, ReleaseChannel: models.AIWorkflowReleaseChannelStable,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(version).Error; err != nil {
		t.Fatalf("create platform diagnosis workflow version: %v", err)
	}
	if err := db.Model(workflow).Update("current_stable_version_id", version.ID).Error; err != nil {
		t.Fatalf("publish platform diagnosis workflow: %v", err)
	}
	if err := db.Model(generalAgent).Updates(map[string]any{
		"tenant_id":           9,
		"product_id":          0,
		"source":              TenantDefaultAIAgentSource,
		"service_mode":        enums.IMConversationServiceModeAIOnly,
		"workflow_id":         workflow.ID,
		"workflow_version_id": version.ID,
		"review_status":       enums.AIAgentReviewStatusApproved,
		"sort_no":             1,
	}).Error; err != nil {
		t.Fatalf("scope general agent: %v", err)
	}
	generalAgent = repositories.AIAgentRepository.Get(db, generalAgent.ID)
	createWelcomeTestActiveRelease(t, db, generalAgent)
	draftVersion := &models.AIWorkflowVersion{
		WorkflowID: workflow.ID, Version: 2, Status: enums.StatusOk, ReleaseChannel: models.AIWorkflowReleaseChannelStable,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(draftVersion).Error; err != nil {
		t.Fatalf("create newer platform workflow version: %v", err)
	}
	if err := db.Model(workflow).Update("current_stable_version_id", draftVersion.ID).Error; err != nil {
		t.Fatalf("advance platform stable workflow: %v", err)
	}
	if err := db.Model(generalAgent).Updates(map[string]any{
		"workflow_version_id": draftVersion.ID,
		"welcome_message":     "这条草稿欢迎语不应提前生效。",
		"review_status":       enums.AIAgentReviewStatusUnreviewed,
	}).Error; err != nil {
		t.Fatalf("update tenant default agent draft: %v", err)
	}
	channelFallback := createWelcomeTestAIAgent(t, db, "不应使用渠道机器人。")

	external := welcomeTestExternalUser("general-consultation")
	conversation, err := ConversationService.CreateWithContext(external, 0, channelFallback.ID, request.CreateOrMatchConversationRequest{
		ContextTenantID: 9,
		ForceNew:        true,
	})
	if err != nil {
		t.Fatalf("create general conversation: %v", err)
	}
	if conversation.DeviceID != 0 || conversation.ProductID != 0 {
		t.Fatalf("general conversation unexpectedly bound after-sales context: %#v", conversation)
	}
	if conversation.TenantID != 9 || conversation.AIAgentID != generalAgent.ID {
		t.Fatalf("general conversation scope mismatch: %#v", conversation)
	}
	var welcome models.Message
	if err := db.Where("conversation_id = ?", conversation.ID).Order("id ASC").First(&welcome).Error; err != nil {
		t.Fatalf("load general conversation welcome: %v", err)
	}
	if welcome.Content != "请描述需要协助的问题。" {
		t.Fatalf("general conversation used draft instead of active release: %q", welcome.Content)
	}

	expiresAt := now.Add(time.Hour)
	entrySession := &models.CustomerEntrySession{
		TenantID: 9, ProductID: 101, ProductModelID: 1001, DeviceID: 501,
		EntryType: "qr", VisitorID: "general-entry-visitor", State: "active",
		EntryContextJSON: `{"tenantId":9,"productId":101,"productModelId":1001}`,
		ExpiresAt:        &expiresAt, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(entrySession).Error; err != nil {
		t.Fatalf("create device-scoped customer entry session: %v", err)
	}
	if err := db.Create(&models.CustomerPrivacyConsent{
		TenantID: 9, ProductID: entrySession.ProductID, EntrySessionID: entrySession.ID,
		VisitorID: entrySession.VisitorID, PolicyVersion: constants.CustomerPrivacyPolicyVersion,
		RequiredAccepted: true, ReceiptHash: strings.Repeat("a", 64),
		ConsentedAt: now, CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create customer entry privacy consent: %v", err)
	}
	entryExternal := openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceGuest,
		ExternalID:     entrySessionExternalID(entrySession.ID, entrySession.VisitorID),
		ExternalName:   "入口通用咨询客户",
	}
	entryGeneral, err := ConversationService.CreateWithContext(entryExternal, 0, channelFallback.ID, request.CreateOrMatchConversationRequest{
		CustomerEntrySessionID: entrySession.ID,
		ContextTenantID:        9,
		ContextProductID:       101,
		General:                true,
		ForceNew:               true,
	})
	if err != nil {
		t.Fatalf("create explicit general conversation from device entry: %v", err)
	}
	if entryGeneral.TenantID != 9 || entryGeneral.ProductID != 0 || entryGeneral.ProductModelID != 0 || entryGeneral.DeviceID != 0 || entryGeneral.ServiceCodeID != 0 || entryGeneral.CustomerEntrySessionID != 0 {
		t.Fatalf("explicit general conversation inherited device entry context: %+v", entryGeneral)
	}
	if entryGeneral.AIAgentID != generalAgent.ID {
		t.Fatalf("explicit general conversation agent = %d, want tenant default %d", entryGeneral.AIAgentID, generalAgent.ID)
	}
}

func TestSendCustomerMessageStoresRequestIDOnMessageAndEvent(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	aiAgent := createWelcomeTestAIAgent(t, db, "")
	external := welcomeTestExternalUser("trace-user")
	conversation, err := ConversationService.Create(external, 11, aiAgent.ID)
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	message, err := MessageService.SendCustomerMessageWithRequestID(
		conversation.ID,
		"client-msg-trace",
		enums.IMMessageTypeText,
		"hello",
		"",
		external,
		"trace-123",
	)
	if err != nil {
		t.Fatalf("SendCustomerMessageWithRequestID() error = %v", err)
	}
	if message.RequestID != "trace-123" {
		t.Fatalf("message.RequestID=%q want %q", message.RequestID, "trace-123")
	}

	var event models.ConversationEventLog
	if err := db.Where("conversation_id = ?", conversation.ID).Order("id DESC").First(&event).Error; err != nil {
		t.Fatalf("find event: %v", err)
	}
	if event.RequestID != "trace-123" {
		t.Fatalf("event.RequestID=%q want %q", event.RequestID, "trace-123")
	}
}

func TestSendCustomerMessagesConcurrentlyAssignsUniqueIDs(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	aiAgent := createWelcomeTestAIAgent(t, db, "")
	external := welcomeTestExternalUser("concurrent-user")
	conversation, err := ConversationService.Create(external, 11, aiAgent.ID)
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	const messageCount = 10
	var wg sync.WaitGroup
	errCh := make(chan error, messageCount)
	for i := 0; i < messageCount; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := MessageService.SendCustomerMessageWithRequestID(
				conversation.ID,
				fmt.Sprintf("client-msg-concurrent-%d", i),
				enums.IMMessageTypeText,
				"hello concurrent",
				"",
				external,
				"trace-concurrent",
			)
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("SendCustomerMessageWithRequestID() concurrent error = %v", err)
		}
	}

	var messages []models.Message
	if err := db.
		Where("conversation_id = ? AND sender_type = ?", conversation.ID, enums.IMSenderTypeCustomer).
		Order("id ASC").
		Find(&messages).Error; err != nil {
		t.Fatalf("find messages: %v", err)
	}
	if len(messages) != messageCount {
		t.Fatalf("expected %d customer messages, got %d", messageCount, len(messages))
	}
	seen := make(map[int64]struct{}, messageCount)
	for _, message := range messages {
		if message.ID <= 0 {
			t.Fatalf("expected persisted message id, got %d", message.ID)
		}
		if _, ok := seen[message.ID]; ok {
			t.Fatalf("duplicate message id %d", message.ID)
		}
		seen[message.ID] = struct{}{}
	}
}

func TestSendCustomerMessageRetryReturnsOriginalMessageAndJob(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	aiAgent := createWelcomeTestAIAgent(t, db, "")
	external := welcomeTestExternalUser("idempotent-user")
	conversation, err := ConversationService.Create(external, 11, aiAgent.ID)
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	first, err := MessageService.SendCustomerMessageWithRequestID(
		conversation.ID,
		"same-client-message",
		enums.IMMessageTypeText,
		"同一个客户端请求",
		"",
		external,
		"trace-first",
	)
	if err != nil {
		t.Fatalf("first send: %v", err)
	}
	retried, err := MessageService.SendCustomerMessageWithRequestID(
		conversation.ID,
		"same-client-message",
		enums.IMMessageTypeText,
		"同一个客户端请求",
		"",
		external,
		"trace-retry",
	)
	if err != nil {
		t.Fatalf("retry send: %v", err)
	}
	if first == nil || retried == nil || first.ID != retried.ID {
		t.Fatalf("retry did not return original message: first=%+v retried=%+v", first, retried)
	}

	var messageCount int64
	if err := db.Model(&models.Message{}).
		Where("conversation_id = ? AND client_msg_id = ?", conversation.ID, "same-client-message").
		Count(&messageCount).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	var jobCount int64
	if err := db.Model(&models.AIReplyJob{}).Where("message_id = ?", first.ID).Count(&jobCount).Error; err != nil {
		t.Fatalf("count ai reply jobs: %v", err)
	}
	if messageCount != 1 || jobCount != 1 {
		t.Fatalf("idempotent retry created duplicates: messages=%d jobs=%d", messageCount, jobCount)
	}
}

func TestSendSystemMessageConcurrentSameClientMsgIDIsIdempotent(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	aiAgent := createWelcomeTestAIAgent(t, db, "")
	conversation := createMessageTestConversation(t, db, aiAgent.ID)

	const senderCount = 8
	start := make(chan struct{})
	errCh := make(chan error, senderCount)
	idCh := make(chan int64, senderCount)
	var wg sync.WaitGroup
	for i := 0; i < senderCount; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			message, err := MessageService.SendSystemMessageWithRequestID(
				conversation.ID,
				"system-handoff-request",
				"客户请求人工支持",
				`{"source":"test"}`,
				fmt.Sprintf("trace-system-%d", i),
			)
			if err != nil {
				errCh <- err
				return
			}
			idCh <- message.ID
		}()
	}
	close(start)
	wg.Wait()
	close(errCh)
	close(idCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("SendSystemMessageWithRequestID() concurrent error = %v", err)
		}
	}

	var firstID int64
	for id := range idCh {
		if id <= 0 {
			t.Fatalf("expected persisted message id, got %d", id)
		}
		if firstID == 0 {
			firstID = id
			continue
		}
		if id != firstID {
			t.Fatalf("concurrent retry returned different message ids: first=%d current=%d", firstID, id)
		}
	}

	var messageCount int64
	if err := db.Model(&models.Message{}).
		Where("conversation_id = ? AND client_msg_id = ?", conversation.ID, "system-handoff-request").
		Count(&messageCount).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	var eventCount int64
	if err := db.Model(&models.ConversationEventLog{}).
		Where("conversation_id = ? AND event_type = ? AND operator_type = ?", conversation.ID, enums.IMEventTypeMessageSend, enums.IMSenderTypeSystem).
		Count(&eventCount).Error; err != nil {
		t.Fatalf("count events: %v", err)
	}
	if messageCount != 1 || eventCount != 1 {
		t.Fatalf("idempotent concurrent system message created duplicates: messages=%d events=%d", messageCount, eventCount)
	}
}

func TestSendCustomerMessageGeneratesClientMsgIDWhenMissing(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	aiAgent := createWelcomeTestAIAgent(t, db, "")
	external := welcomeTestExternalUser("missing-client-id-user")
	conversation, err := ConversationService.Create(external, 11, aiAgent.ID)
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	first, err := MessageService.SendCustomerMessage(conversation.ID, "", enums.IMMessageTypeText, "first", "", external)
	if err != nil {
		t.Fatalf("first send: %v", err)
	}
	second, err := MessageService.SendCustomerMessage(conversation.ID, "", enums.IMMessageTypeText, "second", "", external)
	if err != nil {
		t.Fatalf("second send: %v", err)
	}
	if first.ClientMsgID == "" || second.ClientMsgID == "" {
		t.Fatalf("missing clientMsgId should be filled: first=%q second=%q", first.ClientMsgID, second.ClientMsgID)
	}
	if first.ClientMsgID == second.ClientMsgID {
		t.Fatalf("server-generated clientMsgId must be unique, got %q", first.ClientMsgID)
	}

	var messageCount int64
	if err := db.Model(&models.Message{}).
		Where("conversation_id = ? AND sender_type = ?", conversation.ID, enums.IMSenderTypeCustomer).
		Count(&messageCount).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if messageCount != 2 {
		t.Fatalf("blank clientMsgId sends should not collide, messages=%d", messageCount)
	}
}

func TestUnreadCountUsesLastReadMessageID(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	aiAgent := createWelcomeTestAIAgent(t, db, "")
	conversation := createMessageTestConversation(t, db, aiAgent.ID)
	now := time.Now()

	messages := []models.Message{
		{
			ConversationID: conversation.ID,
			ClientMsgID:    "read-message",
			SenderType:     enums.IMSenderTypeCustomer,
			MessageType:    enums.IMMessageTypeText,
			Content:        "read",
			SendStatus:     enums.IMMessageStatusSent,
			SentAt:         &now,
			AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
		},
		{
			ConversationID: conversation.ID,
			ClientMsgID:    "unread-message-1",
			SenderType:     enums.IMSenderTypeCustomer,
			MessageType:    enums.IMMessageTypeText,
			Content:        "unread 1",
			SendStatus:     enums.IMMessageStatusSent,
			SentAt:         &now,
			AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
		},
		{
			ConversationID: conversation.ID,
			ClientMsgID:    "unread-message-2",
			SenderType:     enums.IMSenderTypeCustomer,
			MessageType:    enums.IMMessageTypeText,
			Content:        "unread 2",
			SendStatus:     enums.IMMessageStatusSent,
			SentAt:         &now,
			AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
		},
	}
	if err := db.Create(&messages).Error; err != nil {
		t.Fatalf("create messages: %v", err)
	}

	readState := &models.ConversationReadState{
		ConversationID:    conversation.ID,
		ReaderType:        enums.IMSenderTypeAgent,
		ReaderID:          1,
		LastReadMessageID: messages[0].ID,
		LastReadAt:        &now,
		AuditFields:       models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(readState).Error; err != nil {
		t.Fatalf("create read state: %v", err)
	}

	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		count, err := ConversationService.countUnreadByState(ctx, conversation.ID, readState, enums.IMSenderTypeCustomer)
		if err != nil {
			return err
		}
		if count != 2 {
			t.Fatalf("unread count=%d want 2", count)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("count unread: %v", err)
	}
}

func TestSendAIMessageStoresWorkflowRunID(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	aiAgent := createWelcomeTestAIAgent(t, db, "")
	conversation := createMessageTestConversation(t, db, aiAgent.ID)

	message, err := MessageService.SendAIMessageWithRequestIDAndWorkflowRunID(
		conversation.ID,
		aiAgent.ID,
		"ai-reply-workflow-1",
		enums.IMMessageTypeText,
		"AI reply",
		"",
		workflowTestAIPrincipal(),
		"trace-workflow-1",
		9988,
	)
	if err != nil {
		t.Fatalf("SendAIMessageWithRequestIDAndWorkflowRunID() error = %v", err)
	}
	if message.WorkflowRunID != 9988 {
		t.Fatalf("message.WorkflowRunID=%d want 9988", message.WorkflowRunID)
	}

	var stored models.Message
	if err := db.First(&stored, message.ID).Error; err != nil {
		t.Fatalf("find message: %v", err)
	}
	if stored.WorkflowRunID != 9988 {
		t.Fatalf("stored.WorkflowRunID=%d want 9988", stored.WorkflowRunID)
	}
}

func TestConversationCreateDoesNotDuplicateWelcomeMessageForExistingConversation(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	aiAgent := createWelcomeTestAIAgent(t, db, "欢迎咨询")
	external := welcomeTestExternalUser("u-2")

	first, err := ConversationService.Create(external, 11, aiAgent.ID)
	if err != nil {
		t.Fatalf("create first conversation: %v", err)
	}
	second, err := ConversationService.Create(external, 11, aiAgent.ID)
	if err != nil {
		t.Fatalf("create second conversation: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected existing conversation id %d, got %d", first.ID, second.ID)
	}

	var count int64
	if err := db.Model(&models.Message{}).Where("conversation_id = ?", first.ID).Count(&count).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one welcome message, got %d", count)
	}
}

func TestConversationCreateSkipsBlankWelcomeMessage(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	aiAgent := createWelcomeTestAIAgent(t, db, "   ")

	conversation, err := ConversationService.Create(welcomeTestExternalUser("blank-welcome-1"), 11, aiAgent.ID)
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if conversation == nil {
		t.Fatalf("expected conversation")
	}

	var count int64
	if err := db.Model(&models.Message{}).Where("conversation_id = ?", conversation.ID).Count(&count).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no welcome messages, got %d", count)
	}

	var updated models.Conversation
	if err := db.First(&updated, conversation.ID).Error; err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	if updated.LastMessageID != 0 {
		t.Fatalf("expected last message id 0, got %d", updated.LastMessageID)
	}
	if updated.CustomerUnreadCount != 0 {
		t.Fatalf("expected customer unread count 0, got %d", updated.CustomerUnreadCount)
	}
}

func TestConversationCreateWelcomeMessageDoesNotTriggerAIReplyHook(t *testing.T) {
	db := setupMessageWelcomeTestDB(t)
	aiAgent := createWelcomeTestAIAgent(t, db, "欢迎咨询")

	previousHook := TriggerAIReplyAsyncHook
	called := false
	TriggerAIReplyAsyncHook = func(conversation models.Conversation, message models.Message) {
		called = true
	}
	t.Cleanup(func() {
		TriggerAIReplyAsyncHook = previousHook
	})

	if _, err := ConversationService.Create(welcomeTestExternalUser("hook-welcome-1"), 11, aiAgent.ID); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if called {
		t.Fatalf("expected welcome message not to trigger ai reply hook")
	}
}
