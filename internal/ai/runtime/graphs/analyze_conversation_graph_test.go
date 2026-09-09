package graphs

import (
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
)

func TestBuildAnalyzeConversationResult_RecommendsHandoffForComplaint(t *testing.T) {
	conversation := models.Conversation{
		LastMessageSummary: "用户反馈被重复扣费，并要求人工处理",
	}
	messages := []models.Message{
		{SenderType: enums.IMSenderTypeCustomer, Content: "你们重复扣费了，我要投诉并转人工"},
	}

	got := buildAnalyzeConversationResult(conversation, messages, AnalyzeConversationInput{
		NeedHumanHandoff: true,
	})

	if got.UserIntent != "handoff_request" {
		t.Fatalf("expected handoff_request, got %q", got.UserIntent)
	}
	if got.RiskLevel != "high" {
		t.Fatalf("expected high risk, got %q", got.RiskLevel)
	}
	if got.RecommendedNextAction != "handoff_to_human" {
		t.Fatalf("expected handoff_to_human, got %q", got.RecommendedNextAction)
	}
}

func TestBuildAnalyzeConversationResult_RecommendsPrepareTicket(t *testing.T) {
	conversation := models.Conversation{
		LastMessageSummary: "用户要求登记问题并尽快处理",
	}
	messages := []models.Message{
		{SenderType: enums.IMSenderTypeCustomer, Content: "麻烦帮我建个工单，订单一直支付失败"},
	}

	got := buildAnalyzeConversationResult(conversation, messages, AnalyzeConversationInput{
		NeedTicket: true,
	})

	if got.UserIntent != "ticket_request" {
		t.Fatalf("expected ticket_request, got %q", got.UserIntent)
	}
	if got.RecommendedNextAction != "prepare_ticket" {
		t.Fatalf("expected prepare_ticket, got %q", got.RecommendedNextAction)
	}
}

func TestBuildAnalyzeConversationResultDoesNotReuseNegatedOrHistoricalHandoff(t *testing.T) {
	conversation := models.Conversation{
		LastMessageSummary: "AI 曾提示人工服务队列",
	}
	messages := []models.Message{
		{SenderType: enums.IMSenderTypeAI, Content: "如有需要可以转人工客服"},
		{SenderType: enums.IMSenderTypeCustomer, Content: "暂时不要转人工，请继续告诉我复位步骤"},
	}

	got := buildAnalyzeConversationResult(conversation, messages, AnalyzeConversationInput{})
	if got.UserIntent != "general_support" {
		t.Fatalf("expected general_support, got %q", got.UserIntent)
	}
	if got.RecommendedNextAction != "continue_answering" {
		t.Fatalf("expected continue_answering, got %q", got.RecommendedNextAction)
	}
	for _, signal := range got.RiskSignals {
		if signal == "handoff_requested" {
			t.Fatalf("historical or negated handoff leaked into risk signals: %#v", got.RiskSignals)
		}
	}
}
