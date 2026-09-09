package runtime

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestReplyCommitStoresWorkflowRunIDOnAIMessage(t *testing.T) {
	db := setupReplyCommitTestDB(t)
	aiAgent := createReplyCommitTestAIAgent(t, db)
	conversation := createReplyCommitTestConversation(t, db, aiAgent)

	replyMessage, err := newReplyCommitService().CommitAIReply(replyCommitInput{
		Conversation:  *conversation,
		Message:       models.Message{ID: 101, RequestID: "trace-101"},
		AIAgent:       *aiAgent,
		ReplyText:     "AI reply",
		ClientPrefix:  "ai_reply",
		WorkflowRunID: 9988,
	})
	if err != nil {
		t.Fatalf("CommitAIReply() error = %v", err)
	}
	if replyMessage == nil {
		t.Fatalf("expected reply message")
	}
	if replyMessage.WorkflowRunID != 9988 {
		t.Fatalf("replyMessage.WorkflowRunID=%d want 9988", replyMessage.WorkflowRunID)
	}

	var stored models.Message
	if err := db.First(&stored, replyMessage.ID).Error; err != nil {
		t.Fatalf("find reply message: %v", err)
	}
	if stored.WorkflowRunID != 9988 {
		t.Fatalf("stored.WorkflowRunID=%d want 9988", stored.WorkflowRunID)
	}
}

func TestReplyCommitStoresKnowledgeCitationsOnAIMessage(t *testing.T) {
	db := setupReplyCommitTestDB(t)
	aiAgent := createReplyCommitTestAIAgent(t, db)
	conversation := createReplyCommitTestConversation(t, db, aiAgent)

	replyMessage, err := newReplyCommitService().CommitAIReply(replyCommitInput{
		Conversation: *conversation,
		Message:      models.Message{ID: 202, RequestID: "trace-202"},
		AIAgent:      *aiAgent,
		ReplyText:    "Calibrate the SI-42 every 90 days.",
		Citations: []dto.KnowledgeCitation{{
			DocumentID:    106,
			DocumentTitle: "odt-uat-knowledge-20260906.md",
			ChunkNo:       0,
			Snippet:       "The SI-42 calibration interval is 90 days.",
		}},
		ClientPrefix:  "ai_reply",
		WorkflowRunID: 944,
	})
	if err != nil {
		t.Fatalf("CommitAIReply() error = %v", err)
	}
	if replyMessage == nil {
		t.Fatal("expected reply message")
	}

	var payload struct {
		Kind               string                  `json:"kind"`
		KnowledgeCitations []dto.KnowledgeCitation `json:"knowledgeCitations"`
	}
	if err := json.Unmarshal([]byte(replyMessage.Payload), &payload); err != nil {
		t.Fatalf("unmarshal reply payload: %v", err)
	}
	if payload.Kind != "knowledge_answer" || len(payload.KnowledgeCitations) != 1 || payload.KnowledgeCitations[0].DocumentID != 106 {
		t.Fatalf("reply payload = %#v", payload)
	}

	var stored models.Message
	if err := db.First(&stored, replyMessage.ID).Error; err != nil {
		t.Fatalf("find reply message: %v", err)
	}
	if stored.Payload != replyMessage.Payload {
		t.Fatalf("stored payload = %q, want %q", stored.Payload, replyMessage.Payload)
	}
}

func TestReplyCommitStoresLateAIReplyAsSystemSummaryAfterHumanTakesOver(t *testing.T) {
	db := setupReplyCommitTestDB(t)
	aiAgent := createReplyCommitTestAIAgent(t, db)
	conversation := createReplyCommitTestConversation(t, db, aiAgent)
	staleConversation := *conversation
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
		"status":              enums.IMConversationStatusActive,
		"current_assignee_id": 42,
	}).Error; err != nil {
		t.Fatalf("mark conversation active: %v", err)
	}

	replyMessage, err := newReplyCommitService().CommitAIReply(replyCommitInput{
		Conversation:  staleConversation,
		Message:       models.Message{ID: 303, RequestID: "trace-303"},
		AIAgent:       *aiAgent,
		ReplyText:     "请停止重复上电，检查 48V 输入端子后联系工程师。",
		ClientPrefix:  "ai_reply",
		WorkflowRunID: 7788,
	})
	if err != nil {
		t.Fatalf("CommitAIReply() error = %v", err)
	}
	if replyMessage == nil {
		t.Fatalf("expected late summary message")
	}
	if replyMessage.SenderType != enums.IMSenderTypeSystem {
		t.Fatalf("senderType=%s want system", replyMessage.SenderType)
	}
	if replyMessage.WorkflowRunID != 7788 {
		t.Fatalf("workflowRunID=%d want 7788", replyMessage.WorkflowRunID)
	}
	if !strings.Contains(replyMessage.Content, "转人工前生成的 AI 诊断摘要") ||
		!strings.Contains(replyMessage.Content, "48V 输入端子") {
		t.Fatalf("late summary content = %q", replyMessage.Content)
	}

	var current models.Conversation
	if err := db.First(&current, conversation.ID).Error; err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	if current.AIReplyRounds != 1 {
		t.Fatalf("AIReplyRounds=%d want 1", current.AIReplyRounds)
	}
	var aiCount int64
	if err := db.Model(&models.Message{}).Where("conversation_id = ? AND sender_type = ?", conversation.ID, enums.IMSenderTypeAI).Count(&aiCount).Error; err != nil {
		t.Fatalf("count ai messages: %v", err)
	}
	if aiCount != 0 {
		t.Fatalf("expected no AI message after human takeover, got %d", aiCount)
	}
}

func TestReplyCommitSilentlyDropsLateAIReplyAfterConversationCloses(t *testing.T) {
	db := setupReplyCommitTestDB(t)
	aiAgent := createReplyCommitTestAIAgent(t, db)
	conversation := createReplyCommitTestConversation(t, db, aiAgent)
	staleConversation := *conversation
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Update(
		"status", enums.IMConversationStatusClosed,
	).Error; err != nil {
		t.Fatalf("close conversation: %v", err)
	}

	replyMessage, err := newReplyCommitService().CommitAIReply(replyCommitInput{
		Conversation:  staleConversation,
		Message:       models.Message{ID: 404, RequestID: "trace-404"},
		AIAgent:       *aiAgent,
		ReplyText:     "This reply completed after the customer transferred to human support.",
		ClientPrefix:  "ai_reply",
		WorkflowRunID: 8899,
	})
	if err != nil {
		t.Fatalf("CommitAIReply() error = %v", err)
	}
	if replyMessage != nil {
		t.Fatalf("expected closed conversation to drop late reply, got message %d", replyMessage.ID)
	}

	var messageCount int64
	if err := db.Model(&models.Message{}).Where("conversation_id = ?", conversation.ID).Count(&messageCount).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if messageCount != 0 {
		t.Fatalf("closed conversation received %d late messages", messageCount)
	}
	var current models.Conversation
	if err := db.First(&current, conversation.ID).Error; err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	if current.AIReplyRounds != 0 {
		t.Fatalf("AIReplyRounds=%d want 0", current.AIReplyRounds)
	}
}

func TestAIReplyFailureMessageIsIdempotent(t *testing.T) {
	db := setupReplyCommitTestDB(t)
	aiAgent := createReplyCommitTestAIAgent(t, db)
	conversation := createReplyCommitTestConversation(t, db, aiAgent)
	message := models.Message{ID: 202, RequestID: "trace-202"}
	service := newAIReplyService()

	service.commitAsyncFailure(*conversation, message, *aiAgent)
	service.commitAsyncFailure(*conversation, message, *aiAgent)

	var messages []models.Message
	if err := db.Where("conversation_id = ? AND sender_type = ?", conversation.ID, enums.IMSenderTypeAI).Find(&messages).Error; err != nil {
		t.Fatalf("find failure messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("failure message count = %d, want 1", len(messages))
	}
	if !strings.Contains(messages[0].Content, "转人工") {
		t.Fatalf("failure message = %q, want actionable handoff guidance", messages[0].Content)
	}
}

func TestAIReplyFailureMessageRespectsAIOnlyWorkflowCapabilities(t *testing.T) {
	db := setupReplyCommitTestDB(t)
	aiAgent := createReplyCommitTestAIAgent(t, db)
	if err := db.Model(aiAgent).Update("service_mode", enums.IMConversationServiceModeAIOnly).Error; err != nil {
		t.Fatalf("mark ai agent as ai-only: %v", err)
	}
	aiAgent.ServiceMode = enums.IMConversationServiceModeAIOnly
	conversation := createReplyCommitTestConversation(t, db, aiAgent)
	message := models.Message{ID: 212, RequestID: "trace-212"}
	service := newAIReplyService()

	service.commitAsyncFailure(*conversation, message, *aiAgent)

	var messages []models.Message
	if err := db.Where("conversation_id = ? AND sender_type = ?", conversation.ID, enums.IMSenderTypeAI).Find(&messages).Error; err != nil {
		t.Fatalf("find failure messages: %v", err)
	}
	if len(messages) != 1 || !strings.Contains(messages[0].Content, "当前流程不会创建工单或转人工") {
		t.Fatalf("ai-only failure message should stay inside workflow capabilities: %+v", messages)
	}
}

func TestAIReplyFailureMessageUsesEnglishForEnglishCustomerMessage(t *testing.T) {
	db := setupReplyCommitTestDB(t)
	aiAgent := createReplyCommitTestAIAgent(t, db)
	conversation := createReplyCommitTestConversation(t, db, aiAgent)
	message := models.Message{
		ID:          217,
		RequestID:   "trace-217",
		MessageType: enums.IMMessageTypeText,
		Content:     "Can I safely restart the device after this fault?",
	}
	service := newAIReplyService()

	service.commitAsyncFailure(*conversation, message, *aiAgent)

	var messages []models.Message
	if err := db.Where("conversation_id = ? AND sender_type = ?", conversation.ID, enums.IMSenderTypeAI).Find(&messages).Error; err != nil {
		t.Fatalf("find failure messages: %v", err)
	}
	if len(messages) != 1 || !strings.Contains(messages[0].Content, "AI support is temporarily unavailable") || strings.ContainsAny(messages[0].Content, "设备工单人工") {
		t.Fatalf("English AI failure message = %+v", messages)
	}
}

func TestAIReplyFailureMessageFallsBackToSystemAfterHumanTakeover(t *testing.T) {
	db := setupReplyCommitTestDB(t)
	aiAgent := createReplyCommitTestAIAgent(t, db)
	conversation := createReplyCommitTestConversation(t, db, aiAgent)
	if err := db.Model(&models.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
		"status":              enums.IMConversationStatusActive,
		"current_assignee_id": 42,
	}).Error; err != nil {
		t.Fatalf("mark conversation active: %v", err)
	}
	service := newAIReplyService()

	service.commitAsyncFailure(*conversation, models.Message{ID: 222, RequestID: "trace-222"}, *aiAgent)

	var messages []models.Message
	if err := db.Where("conversation_id = ?", conversation.ID).Find(&messages).Error; err != nil {
		t.Fatalf("find failure messages: %v", err)
	}
	if len(messages) != 1 || messages[0].SenderType != enums.IMSenderTypeSystem || !strings.Contains(messages[0].Content, "工程师继续处理") {
		t.Fatalf("late ai failure should be visible as a system message: %+v", messages)
	}
}

func TestAIReplyJobRetriesThenPublishesOneActionableFailure(t *testing.T) {
	db := setupReplyCommitTestDB(t)
	aiAgent := createReplyCommitTestAIAgent(t, db)
	conversation := createReplyCommitTestConversation(t, db, aiAgent)
	now := time.Now()
	customerMessage := &models.Message{
		ID: 303, ConversationID: conversation.ID, ClientMsgID: "customer-303", SenderType: enums.IMSenderTypeCustomer,
		MessageType: enums.IMMessageTypeText, Content: "设备仍然无法启动", SendStatus: enums.IMMessageStatusSent,
		SentAt: &now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(customerMessage).Error; err != nil {
		t.Fatalf("create customer message: %v", err)
	}
	job := &models.AIReplyJob{
		TenantID: conversation.TenantID, ConversationID: conversation.ID, MessageID: 303, AIAgentID: aiAgent.ID,
		Status: models.AIReplyJobStatusRunning, RetryCount: 0, MaxRetries: 2, LockOwner: "worker-1",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(job).Error; err != nil {
		t.Fatalf("create ai reply job: %v", err)
	}
	service := newAIReplyService()
	if err := service.retryOrFailReplyJob(job, errors.New("model provider unavailable")); err != nil {
		t.Fatalf("schedule ai reply retry: %v", err)
	}
	var waiting models.AIReplyJob
	if err := db.First(&waiting, job.ID).Error; err != nil {
		t.Fatalf("reload waiting ai reply job: %v", err)
	}
	if waiting.Status != models.AIReplyJobStatusWaitingRetry || waiting.RetryCount != 1 || waiting.NextAttemptAt == nil {
		t.Fatalf("unexpected waiting retry job: %+v", waiting)
	}
	if err := db.Model(&waiting).Updates(map[string]any{
		"status": models.AIReplyJobStatusRunning, "lock_owner": "worker-2", "next_attempt_at": nil,
	}).Error; err != nil {
		t.Fatalf("claim final ai reply attempt: %v", err)
	}
	waiting.Status = models.AIReplyJobStatusRunning
	waiting.LockOwner = "worker-2"
	if err := service.retryOrFailReplyJob(&waiting, errors.New("model provider still unavailable")); err != nil {
		t.Fatalf("finish failed ai reply job: %v", err)
	}
	var failed models.AIReplyJob
	if err := db.First(&failed, job.ID).Error; err != nil {
		t.Fatalf("reload failed ai reply job: %v", err)
	}
	if failed.Status != models.AIReplyJobStatusFailed || failed.RetryCount != 2 || failed.NextAttemptAt != nil {
		t.Fatalf("unexpected failed ai reply job: %+v", failed)
	}
	var messages []models.Message
	if err := db.Where("conversation_id = ? AND sender_type = ?", conversation.ID, enums.IMSenderTypeAI).Find(&messages).Error; err != nil {
		t.Fatalf("find ai failure message: %v", err)
	}
	if len(messages) != 1 || !strings.Contains(messages[0].Content, "转人工") {
		t.Fatalf("actionable ai failure messages = %+v", messages)
	}
}

func TestAIReplyJobLeaseHeartbeatPreventsPrematureRecovery(t *testing.T) {
	db := setupReplyCommitTestDB(t)
	now := time.Now()
	job := &models.AIReplyJob{
		TenantID: 1, ConversationID: 10, MessageID: 20, AIAgentID: 30,
		Status: models.AIReplyJobStatusRunning, LockOwner: "worker-live", LockedAt: ptrRuntimeTime(now.Add(-time.Minute)),
		CreatedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-time.Minute),
	}
	if err := db.Create(job).Error; err != nil {
		t.Fatalf("create running ai reply job: %v", err)
	}

	heartbeatAt := now
	updated, err := repositories.AIReplyJobRepository.Heartbeat(db, job.ID, job.LockOwner, heartbeatAt)
	if err != nil || !updated {
		t.Fatalf("heartbeat ai reply job: updated=%v err=%v", updated, err)
	}
	recovered, err := repositories.AIReplyJobRepository.RecoverExpiredRunningJobs(
		db,
		heartbeatAt.Add(-aiReplyJobLeaseTTL),
		heartbeatAt,
	)
	if err != nil || recovered != 0 {
		t.Fatalf("active heartbeat was recovered: count=%d err=%v", recovered, err)
	}

	staleAt := heartbeatAt.Add(-aiReplyJobLeaseTTL - time.Second)
	if err := db.Model(job).Updates(map[string]any{"locked_at": staleAt, "updated_at": staleAt}).Error; err != nil {
		t.Fatalf("expire ai reply job lease: %v", err)
	}
	recovered, err = repositories.AIReplyJobRepository.RecoverExpiredRunningJobs(
		db,
		heartbeatAt.Add(-aiReplyJobLeaseTTL),
		heartbeatAt,
	)
	if err != nil || recovered != 1 {
		t.Fatalf("stale heartbeat recovery: count=%d err=%v", recovered, err)
	}
	var recoveredJob models.AIReplyJob
	if err := db.First(&recoveredJob, job.ID).Error; err != nil {
		t.Fatalf("reload recovered ai reply job: %v", err)
	}
	if recoveredJob.Status != models.AIReplyJobStatusWaitingRetry || recoveredJob.LockedAt != nil || recoveredJob.LockOwner != "" {
		t.Fatalf("unexpected recovered ai reply job: %+v", recoveredJob)
	}
}

func ptrRuntimeTime(value time.Time) *time.Time {
	return &value
}

func setupReplyCommitTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := "reply_commit_test_" + strings.NewReplacer("/", "_").Replace(t.Name())
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
		&models.Channel{},
		&models.ChannelMessageOutbox{},
		&models.Conversation{},
		&models.ConversationReadState{},
		&models.ConversationEventLog{},
		&models.Message{},
		&models.AIReplyJob{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	return db
}

func createReplyCommitTestAIAgent(t *testing.T, db *gorm.DB) *models.AIAgent {
	t.Helper()
	now := time.Now()
	item := &models.AIAgent{
		TenantID: 7,
		Name:     "reply-agent",
		Status:   enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create ai agent: %v", err)
	}
	return item
}

func createReplyCommitTestConversation(t *testing.T, db *gorm.DB, aiAgent *models.AIAgent) *models.Conversation {
	t.Helper()
	now := time.Now()
	item := &models.Conversation{
		TenantID:     aiAgent.TenantID,
		CustomerID:   1,
		ChannelID:    11,
		AIAgentID:    aiAgent.ID,
		Status:       enums.IMConversationStatusAIServing,
		LastActiveAt: now,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	return item
}
