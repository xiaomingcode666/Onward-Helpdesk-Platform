package runtime_test

import (
	"context"
	"os"
	"testing"
	"time"

	"remotehelpdesk/internal/ai/rag/vectordb"
	airuntime "remotehelpdesk/internal/ai/runtime"
	"remotehelpdesk/internal/bootstrap"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"

	"github.com/mlogclub/simple/sqls"
)

func TestLiveCustomerMessageRunsEinoWorkflow(t *testing.T) {
	if os.Getenv("RUN_AI_REPLY_LIVE_TEST") != "1" {
		t.Skip("set RUN_AI_REPLY_LIVE_TEST=1 to process the latest pending customer message")
	}
	cfg, err := config.Load("../../../config/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	config.SetCurrent(cfg)
	if _, err := bootstrap.InitDB(cfg.DB); err != nil {
		t.Fatal(err)
	}
	if err := vectordb.Init(&cfg.VectorDB); err != nil {
		t.Fatal(err)
	}

	var message models.Message
	if err := sqls.DB().
		Where("sender_type = ? AND workflow_run_id = 0", "customer").
		Order("id DESC").
		First(&message).Error; err != nil {
		t.Fatal(err)
	}
	var conversation models.Conversation
	if err := sqls.DB().First(&conversation, message.ConversationID).Error; err != nil {
		t.Fatal(err)
	}
	var agent models.AIAgent
	if err := sqls.DB().First(&agent, conversation.AIAgentID).Error; err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := airuntime.AIReplyService.TriggerReply(ctx, conversation, message, agent); err != nil {
		t.Fatal(err)
	}

	var reply models.Message
	if err := sqls.DB().
		Where("conversation_id = ? AND sender_type = ? AND workflow_run_id > 0", conversation.ID, "ai").
		Order("id DESC").
		First(&reply).Error; err != nil {
		t.Fatal(err)
	}
	t.Logf("conversation=%d customer_message=%d ai_message=%d workflow_run=%d", conversation.ID, message.ID, reply.ID, reply.WorkflowRunID)
}
