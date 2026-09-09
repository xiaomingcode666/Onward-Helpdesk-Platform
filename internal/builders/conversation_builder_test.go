package builders

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/i18nx"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestLocalizeConversationSummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		locale  string
		summary string
		want    string
	}{
		{
			name:    "image summary in english",
			locale:  i18nx.LocaleEnUS,
			summary: "[图片]",
			want:    "[Image]",
		},
		{
			name:    "attachment summary in english",
			locale:  i18nx.LocaleEnUS,
			summary: "[附件] spec.pdf",
			want:    "[Attachment] spec.pdf",
		},
		{
			name:    "recalled message in english",
			locale:  i18nx.LocaleEnUS,
			summary: "该消息已撤回",
			want:    "This message was recalled.",
		},
		{
			name:    "business text is not translated",
			locale:  i18nx.LocaleEnUS,
			summary: "客户反馈无法登录",
			want:    "客户反馈无法登录",
		},
		{
			name:    "chinese locale keeps existing summary",
			locale:  i18nx.LocaleZhCN,
			summary: "[图片]",
			want:    "[图片]",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := localizeConversationSummary(tt.locale, tt.summary); got != tt.want {
				t.Fatalf("localizeConversationSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLocalizeRenderableMessageContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		locale  string
		content string
		want    string
	}{
		{
			name:    "recalled message in english",
			locale:  i18nx.LocaleEnUS,
			content: "该消息已撤回",
			want:    "This message was recalled.",
		},
		{
			name:    "normal customer message is not translated",
			locale:  i18nx.LocaleEnUS,
			content: "客户反馈无法登录",
			want:    "客户反馈无法登录",
		},
		{
			name:    "chinese locale keeps content",
			locale:  i18nx.LocaleZhCN,
			content: "该消息已撤回",
			want:    "该消息已撤回",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := localizeRenderableMessageContent(tt.locale, tt.content); got != tt.want {
				t.Fatalf("localizeRenderableMessageContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildMessageIncludesWorkflowRunID(t *testing.T) {
	resp := BuildMessageWithReadStatesAndLocale(&models.Message{
		ID:             1,
		ConversationID: 2,
		SenderType:     enums.IMSenderTypeAI,
		MessageType:    enums.IMMessageTypeText,
		Content:        "AI reply",
		WorkflowRunID:  9988,
	}, nil, nil, map[int64]string{88: "AI"}, nil, nil, i18nx.DefaultLocale)

	if resp.WorkflowRunID != 9988 {
		t.Fatalf("resp.WorkflowRunID=%d want 9988", resp.WorkflowRunID)
	}
}

func TestBuildCustomerMessageOmitsInternalRoutingAndSenderIDs(t *testing.T) {
	resp := BuildMessageWithReadStatesAndLocale(&models.Message{
		ID:             1,
		ConversationID: 2,
		RequestID:      "trace-internal",
		SenderType:     enums.IMSenderTypeAI,
		SenderID:       88,
		MessageType:    enums.IMMessageTypeText,
		Content:        "AI reply",
		WorkflowRunID:  9988,
	}, nil, nil, map[int64]string{88: "AI"}, nil, nil, i18nx.DefaultLocale)
	sanitizeCustomerMessageResponse(&resp)

	if resp.WorkflowRunID != 0 {
		t.Fatalf("customer resp.WorkflowRunID=%d want 0", resp.WorkflowRunID)
	}
	if resp.SenderID != 0 {
		t.Fatalf("customer resp.SenderID=%d want 0", resp.SenderID)
	}
	if resp.RequestID != "" {
		t.Fatalf("customer resp.RequestID=%q want empty", resp.RequestID)
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal customer message: %v", err)
	}
	for _, forbidden := range []string{"workflowRunId", "senderId", "requestId"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("customer message exposed %s in %s", forbidden, data)
		}
	}
}

func TestBuildCustomerConversationDetailOmitsInternalIdentityIDs(t *testing.T) {
	dbName := "customer_conversation_detail_" + strings.NewReplacer("/", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.ConversationReadState{}, &models.ConversationParticipant{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqls.SetDB(db)

	joinedAt := time.Now().Add(-time.Minute)
	if err := db.Create(&models.ConversationParticipant{
		ConversationID:        42,
		ParticipantType:       string(enums.IMParticipantTypeAgent),
		ParticipantID:         88,
		ExternalParticipantID: "user:88",
		JoinedAt:              &joinedAt,
		Status:                enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create participant: %v", err)
	}
	resp := BuildCustomerConversationDetailWithLocale(&models.Conversation{
		ID:                 42,
		AIAgentID:          99,
		ChannelID:          7,
		CustomerID:         123,
		TenantID:           456,
		CurrentAssigneeID:  88,
		CurrentTeamID:      66,
		Status:             enums.IMConversationStatusActive,
		ServiceMode:        enums.IMConversationServiceModeHumanOnly,
		Priority:           2,
		LastMessageID:      1001,
		LastMessageAt:      time.Now(),
		LastActiveAt:       time.Now(),
		LastMessageSummary: "排查中",
	}, i18nx.DefaultLocale)
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal customer conversation detail: %v", err)
	}
	for _, forbidden := range []string{
		"aiAgentId",
		"channelId",
		"customerId",
		"tenantId",
		"currentAssigneeId",
		"currentTeamId",
		"participantId",
		"externalParticipantId",
	} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("customer conversation detail exposed %s: %s", forbidden, raw)
		}
	}
}

func TestBuildMessageJSONDoesNotExposeSeqNo(t *testing.T) {
	resp := BuildMessageWithReadStatesAndLocale(&models.Message{
		ID:             1,
		ConversationID: 2,
		SenderType:     enums.IMSenderTypeCustomer,
		MessageType:    enums.IMMessageTypeText,
		Content:        "hello",
	}, nil, nil, nil, nil, nil, i18nx.DefaultLocale)

	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal message response: %v", err)
	}
	if strings.Contains(string(raw), "seqNo") {
		t.Fatalf("message response should not expose seqNo, got %s", raw)
	}
}
