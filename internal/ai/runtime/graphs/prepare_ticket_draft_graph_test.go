package graphs

import (
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
)

func TestBuildPrepareTicketDraftResult_UsesConversationFallbacks(t *testing.T) {
	conversation := models.Conversation{
		LastMessageSummary: "用户反馈企业微信扫码后页面空白，无法进入工作台",
	}
	messages := []models.Message{
		{SenderType: enums.IMSenderTypeCustomer, Content: "扫码登录后一直白屏"},
		{SenderType: enums.IMSenderTypeAI, Content: "请问是否有报错提示"},
	}

	got := buildPrepareTicketDraftResult(conversation, messages, PrepareTicketDraftInput{
		Impact:          "无法进入后台处理客户消息",
		ExpectedOutcome: "恢复正常登录",
	})

	if got.Title == "" {
		t.Fatalf("expected draft title to be generated")
	}
	if got.Description == "" {
		t.Fatalf("expected draft description to be generated")
	}
	if !got.Ready {
		t.Fatalf("expected conversation summary and recent messages to be enough, got %#v", got)
	}
	if len(got.ConversationFacts) == 0 {
		t.Fatalf("expected conversation facts to be populated")
	}
}

func TestBuildPrepareTicketDraftResult_ReadyWithExplicitIssue(t *testing.T) {
	conversation := models.Conversation{
		LastMessageSummary: "用户反馈连续支付失败",
	}

	got := buildPrepareTicketDraftResult(conversation, nil, PrepareTicketDraftInput{
		Issue:           "用户连续三次支付订单失败，页面提示网络异常。",
		ExpectedOutcome: "希望尽快恢复支付并完成下单。",
		CurrentAttempt:  "已尝试切换网络和刷新页面，问题仍存在。",
	})

	if !got.Ready {
		t.Fatalf("expected draft to be ready, got %#v", got)
	}
	if got.Title == "" || got.Description == "" {
		t.Fatalf("expected title and description to be populated, got %#v", got)
	}
}

func TestBuildPrepareTicketDraftResult_ActionOnlyRequestNeedsIssueDetails(t *testing.T) {
	conversation := models.Conversation{LastMessageSummary: "请帮我创建工单"}
	messages := []models.Message{
		{SenderType: enums.IMSenderTypeCustomer, Content: "请帮我创建工单"},
		{SenderType: enums.IMSenderTypeAI, Content: "可以，请确认。"},
	}

	got := buildPrepareTicketDraftResult(conversation, messages, PrepareTicketDraftInput{Issue: "请帮我创建工单"})

	if got.Ready {
		t.Fatalf("action-only ticket request must not be ready: %#v", got)
	}
	if got.Title != "" {
		t.Fatalf("action text must not become the ticket title: %q", got.Title)
	}
	if len(got.FollowUpQuestions) == 0 || !strings.Contains(got.FollowUpQuestions[len(got.FollowUpQuestions)-1], "故障现象") {
		t.Fatalf("expected a concrete issue follow-up, got %#v", got.FollowUpQuestions)
	}
}

func TestBuildPrepareTicketDraftResult_UsesEarlierCustomerIssue(t *testing.T) {
	conversation := models.Conversation{LastMessageSummary: "请帮我创建工单"}
	messages := []models.Message{
		{SenderType: enums.IMSenderTypeCustomer, Content: "设备无法启动，面板显示故障码 E42"},
		{SenderType: enums.IMSenderTypeCustomer, Content: "请帮我创建工单"},
	}

	got := buildPrepareTicketDraftResult(conversation, messages, PrepareTicketDraftInput{Issue: "请帮我创建工单"})

	if !got.Ready || !strings.Contains(got.Title, "e42") {
		t.Fatalf("expected earlier issue context to prepare the draft: %#v", got)
	}
}

func TestBuildPrepareTicketDraftResult_IgnoresUnrelatedEarlierMessage(t *testing.T) {
	conversation := models.Conversation{LastMessageSummary: "请帮我创建工单"}
	messages := []models.Message{
		{SenderType: enums.IMSenderTypeCustomer, Content: "我想了解一下你们的售后服务范围"},
		{SenderType: enums.IMSenderTypeCustomer, Content: "请帮我创建工单"},
	}

	got := buildPrepareTicketDraftResult(conversation, messages, PrepareTicketDraftInput{Issue: "请帮我创建工单"})

	if got.Ready || got.Title != "" {
		t.Fatalf("unrelated history must not satisfy ticket issue details: %#v", got)
	}
}

func TestTicketIssueDetail_AcceptsConciseFaultsAndServiceRequests(t *testing.T) {
	for _, issue := range []string{
		"E1",
		"机器漏水",
		"设备没反应",
		"控制器不开机",
		"设备需要年度保养",
		"故障现象：漏水；故障码：无；问题描述：底部持续渗水",
		"<p>请帮我创建工单。</p><p><strong>故障现象：</strong>机器漏水</p>",
	} {
		if got := ticketIssueDetail(issue); got == "" {
			t.Errorf("ticketIssueDetail(%q) should be accepted", issue)
		}
	}
}

func TestTicketIssueDetail_RejectsGenericOrUnrelatedText(t *testing.T) {
	for _, issue := range []string{
		"请帮我创建工单，这个事情比较着急希望尽快处理",
		"我想了解一下你们的售后服务范围",
		"故障现象：无；故障码：未知；问题描述：待补充",
	} {
		if got := ticketIssueDetail(issue); got != "" {
			t.Errorf("ticketIssueDetail(%q) = %q, want empty", issue, got)
		}
	}
}
