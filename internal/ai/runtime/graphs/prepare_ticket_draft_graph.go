package graphs

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"

	runtimeintent "remotehelpdesk/internal/ai/runtime/intent"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"
)

var ticketIssueCodePattern = regexp.MustCompile(`(?i)\b[a-z]{1,10}[-_]?\d+\b`)
var ticketIssueHTMLPattern = regexp.MustCompile(`<[^>]+>`)
var ticketIssueFieldPattern = regexp.MustCompile(`(?i)(故障现象|故障码|问题描述|fault symptom|fault code|issue description)\s*[:：]\s*`)

type PrepareTicketDraftInput struct {
	Title           string `json:"title"`
	Description     string `json:"description"`
	Issue           string `json:"issue"`
	Impact          string `json:"impact"`
	ExpectedOutcome string `json:"expectedOutcome"`
	CurrentAttempt  string `json:"currentAttempt"`
}

type PrepareTicketDraftResult struct {
	Ready             bool     `json:"ready"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	MissingFields     []string `json:"missingFields,omitempty"`
	FollowUpQuestions []string `json:"followUpQuestions,omitempty"`
	ConversationFacts []string `json:"conversationFacts,omitempty"`
}

type PrepareTicketDraftGraph struct {
	conversation models.Conversation
}

func NewPrepareTicketDraftGraph(conversation models.Conversation) *PrepareTicketDraftGraph {
	return &PrepareTicketDraftGraph{conversation: conversation}
}

func (g *PrepareTicketDraftGraph) Run(_ context.Context, argumentsInJSON string) (string, error) {
	input, err := g.parseInput(argumentsInJSON)
	if err != nil {
		return "", err
	}
	messages, _, _ := services.MessageService.FindByConversationIDCursor(g.conversation.ID, 0, 6, "", "")
	result := buildPrepareTicketDraftResult(g.conversation, messages, input)
	buf, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func (g *PrepareTicketDraftGraph) parseInput(argumentsInJSON string) (PrepareTicketDraftInput, error) {
	var input PrepareTicketDraftInput
	if strings.TrimSpace(argumentsInJSON) == "" {
		return input, nil
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return input, fmt.Errorf("invalid prepare ticket draft arguments: %w", err)
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	input.Issue = strings.TrimSpace(input.Issue)
	input.Impact = strings.TrimSpace(input.Impact)
	input.ExpectedOutcome = strings.TrimSpace(input.ExpectedOutcome)
	input.CurrentAttempt = strings.TrimSpace(input.CurrentAttempt)
	return input, nil
}

func buildPrepareTicketDraftResult(conversation models.Conversation, messages []models.Message, input PrepareTicketDraftInput) PrepareTicketDraftResult {
	result := PrepareTicketDraftResult{
		MissingFields:     make([]string, 0, 2),
		FollowUpQuestions: make([]string, 0, 2),
		ConversationFacts: buildConversationFacts(conversation, messages),
	}
	result.Title = buildDraftTitle(conversation, messages, input)
	result.Description = buildDraftDescription(conversation, messages, input)
	if strings.TrimSpace(result.Title) == "" {
		result.MissingFields = append(result.MissingFields, "title")
		result.FollowUpQuestions = append(result.FollowUpQuestions, "Please provide a concise ticket title that clearly summarizes the issue.")
	}
	if !hasSufficientIssueContext(conversation, messages, input) {
		result.MissingFields = append(result.MissingFields, "issue")
		result.FollowUpQuestions = append(result.FollowUpQuestions, "请补充故障现象、故障码（如有）或具体问题描述。")
	}
	result.Ready = result.Title != "" && result.Description != "" && len(result.MissingFields) == 0
	return result
}

func buildDraftTitle(conversation models.Conversation, messages []models.Message, input PrepareTicketDraftInput) string {
	switch {
	case input.Title != "":
		return limitText(input.Title, 80)
	case ticketIssueDetail(input.Issue) != "":
		return limitText(ticketIssueDetail(input.Issue), 80)
	case firstCustomerTicketIssue(messages) != "":
		return limitText(firstCustomerTicketIssue(messages), 80)
	case ticketIssueDetail(conversation.LastMessageSummary) != "":
		return limitText(ticketIssueDetail(conversation.LastMessageSummary), 80)
	default:
		return ""
	}
}

func buildDraftDescription(conversation models.Conversation, messages []models.Message, input PrepareTicketDraftInput) string {
	if input.Description != "" {
		return input.Description
	}
	parts := make([]string, 0, 6)
	if issue := ticketIssueDetail(input.Issue); issue != "" {
		parts = append(parts, "问题描述："+issue)
	}
	if input.Impact != "" {
		parts = append(parts, "Impact: "+input.Impact)
	}
	if input.ExpectedOutcome != "" {
		parts = append(parts, "Requested outcome: "+input.ExpectedOutcome)
	}
	if input.CurrentAttempt != "" {
		parts = append(parts, "Attempts so far: "+input.CurrentAttempt)
	}
	if strings.TrimSpace(conversation.LastMessageSummary) != "" {
		parts = append(parts, "Conversation summary: "+strings.TrimSpace(conversation.LastMessageSummary))
	}
	if recent := buildRecentMessageDigest(messages); recent != "" {
		parts = append(parts, "Recent messages: "+recent)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func hasSufficientIssueContext(conversation models.Conversation, messages []models.Message, input PrepareTicketDraftInput) bool {
	if ticketIssueDetail(input.Issue) != "" || ticketIssueDetail(input.Description) != "" {
		return true
	}
	if firstCustomerTicketIssue(messages) != "" {
		return true
	}
	return ticketIssueDetail(conversation.LastMessageSummary) != ""
}

func HasSufficientTicketIssueContext(value string) bool {
	return ticketIssueDetail(value) != ""
}

func firstCustomerTicketIssue(messages []models.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].SenderType != enums.IMSenderTypeCustomer {
			continue
		}
		if detail := ticketIssueDetail(messages[i].Content); detail != "" {
			return detail
		}
	}
	return ""
}

func ticketIssueDetail(value string) string {
	value = ticketIssueHTMLPattern.ReplaceAllString(strings.TrimSpace(value), " ")
	value = html.UnescapeString(value)
	value = strings.Join(strings.Fields(value), " ")
	if detail, structured := structuredTicketIssueDetail(value); structured {
		return detail
	}
	detail := runtimeintent.TicketRequestIssueDetail(value)
	if detail == "" || isGenericTicketIssueDetail(detail) {
		return ""
	}
	lower := strings.ToLower(detail)
	if ticketIssueCodePattern.MatchString(lower) || containsTicketIssueSignal(lower) {
		return detail
	}
	return ""
}

func structuredTicketIssueDetail(value string) (string, bool) {
	matches := ticketIssueFieldPattern.FindAllStringIndex(value, -1)
	if len(matches) == 0 {
		return "", false
	}
	parts := make([]string, 0, len(matches))
	for i, match := range matches {
		end := len(value)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		part := strings.Trim(value[match[1]:end], " \t\r\n，,。.!！?？;；:：~～")
		part = runtimeintent.TicketRequestIssueDetail(part)
		if part == "" || isGenericTicketIssueDetail(part) {
			continue
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "；"), true
}

func isGenericTicketIssueDetail(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, generic := range []string{
		"无", "没有", "暂无", "未知", "不清楚", "不知道", "待补充",
		"有问题", "一个问题", "这个问题", "帮我处理", "需要处理",
		"这个事情比较着急希望尽快处理", "售后", "售后服务",
	} {
		if value == generic {
			return true
		}
	}
	return false
}

func containsTicketIssueSignal(value string) bool {
	for _, signal := range []string{
		"故障", "故障码", "报警", "告警", "报错", "错误", "异常", "失败",
		"无法", "不能", "不工作", "不运行", "不启动", "不开机", "开不了机", "启动不了",
		"坏了", "没反应", "无响应", "没声音", "无声音", "无显示", "红灯", "黄灯",
		"白屏", "黑屏", "花屏", "异响", "噪音", "过热", "漏液", "漏油", "漏水", "渗水",
		"停机", "关机", "掉电", "断电", "重启", "死机", "卡住", "卡顿", "堵塞",
		"冒烟", "异味", "烧焦", "破损", "裂纹", "松动", "抖动", "震动", "离线", "断连",
		"不制冷", "不加热", "不通电", "不出水", "不充电", "不识别",
		"维修", "检修", "保养", "维护", "巡检", "校准", "更换", "安装", "调试", "退换",
	} {
		if strings.Contains(value, signal) {
			return true
		}
	}
	return false
}

func buildConversationFacts(conversation models.Conversation, messages []models.Message) []string {
	facts := make([]string, 0, 4)
	if strings.TrimSpace(conversation.LastMessageSummary) != "" {
		facts = append(facts, "Recent summary: "+strings.TrimSpace(conversation.LastMessageSummary))
	}
	if digest := buildRecentMessageDigest(messages); digest != "" {
		facts = append(facts, "Recent messages: "+digest)
	}
	return facts
}

func buildRecentMessageDigest(messages []models.Message) string {
	if len(messages) == 0 {
		return ""
	}
	parts := make([]string, 0, len(messages))
	for i := range messages {
		content := strings.TrimSpace(messages[i].Content)
		if content == "" {
			continue
		}
		parts = append(parts, messageSenderLabel(messages[i].SenderType)+"："+limitText(content, 60))
	}
	return strings.Join(parts, " | ")
}

func messageSenderLabel(senderType enums.IMSenderType) string {
	switch senderType {
	case enums.IMSenderTypeCustomer:
		return "Customer"
	case enums.IMSenderTypeAgent:
		return "Agent"
	case enums.IMSenderTypeAI:
		return "AI"
	default:
		return "Message"
	}
}

func limitText(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	if max <= 3 {
		return strings.TrimSpace(string(runes[:max]))
	}
	return strings.TrimSpace(string(runes[:max-3])) + "..."
}
