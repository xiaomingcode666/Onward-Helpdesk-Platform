package intent

import "strings"

var handoffNegationPhrases = []string{
	"不要转人工", "别转人工", "不用转人工", "无需转人工", "不需要转人工", "不转人工",
	"暂时不要转人工", "暂不转人工", "先不要转人工", "先不转人工",
	"不要人工", "不用人工", "无需人工", "不需要人工",
	"别找人工", "不用找人工", "别联系人工", "先别联系人工", "不要联系人工", "不用联系人工", "无需联系人工",
	"donottransfer", "donttransfer", "don'ttransfer", "nohumanagent",
}

var handoffRequestPhrases = []string{
	"转人工", "接人工", "找人工", "联系人工", "请求人工", "呼叫人工", "安排人工",
	"需要人工", "真人客服",
	"humanagent", "liveagent", "transferme",
}

var handoffRolePhrases = []string{"人工客服", "人工服务", "人工处理"}

var handoffRequestVerbs = []string{"我要", "我想", "帮我", "需要", "联系", "转接", "接入", "安排", "请求", "呼叫"}

var ticketNegationPhrases = []string{
	"不要创建工单", "别创建工单", "不用创建工单", "无需创建工单", "不需要创建工单",
	"不要建工单", "别建工单", "不用建工单", "无需建工单", "不建工单",
	"不要建单", "别建单", "不用建单", "无需建单", "不建单",
	"暂时不要建单", "暂不建单", "先不要建单", "先不建单",
	"不要报障", "不用报障", "无需报障",
	"donotcreateticket", "dontcreateticket", "don'tcreateticket",
}

var ticketRequestPhrases = []string{
	"创建工单", "新建工单", "提交工单", "发起工单", "建工单", "开工单",
	"我要建单", "帮我建单", "请建单", "登记工单", "记录工单", "登记问题", "记录问题", "我要报障", "提交报障",
	"createticket", "openticket", "submitticket",
}

func IsExplicitHandoffRequest(value string) bool {
	compact := compactActionText(value)
	if compact == "" || containsActionPhrase(compact, handoffNegationPhrases) || containsActionPhrase(compactActionModifiers(compact), handoffNegationPhrases) {
		return false
	}
	if compact == "人工" || compact == "客服" || compact == "真人" {
		return true
	}
	if containsActionPhrase(compact, handoffRequestPhrases) {
		return true
	}
	if containsActionPhrase(compact, handoffRolePhrases) {
		return compact == "人工客服" || compact == "人工服务" || compact == "人工处理" || containsActionPhrase(compact, handoffRequestVerbs)
	}
	return false
}

func IsExplicitTicketRequest(value string) bool {
	compact := compactActionText(value)
	if compact == "" || containsActionPhrase(compact, ticketNegationPhrases) || containsActionPhrase(compactActionModifiers(compact), ticketNegationPhrases) {
		return false
	}
	if compact == "工单" || compact == "报障" || compact == "建单" {
		return true
	}
	return containsActionPhrase(compact, ticketRequestPhrases)
}

// TicketRequestIssueDetail removes the ticket action itself so callers can
// distinguish a real issue description from an action-only request.
func TicketRequestIssueDetail(value string) string {
	detail := compactActionText(value)
	if detail == "" {
		return ""
	}
	for _, phrase := range append(append([]string{}, ticketRequestPhrases...), "工单", "报障", "建单") {
		detail = strings.ReplaceAll(detail, phrase, "")
	}
	detail = compactActionModifiers(detail)
	for {
		before := detail
		for _, prefix := range []string{"请", "麻烦", "劳驾", "帮我", "给我", "替我", "为我", "我要", "我想", "我需要", "需要", "想要", "协助", "处理"} {
			detail = strings.TrimPrefix(detail, prefix)
		}
		for _, suffix := range []string{"请帮我", "麻烦了", "一下", "可以吗", "好吗", "行吗", "谢谢", "感谢", "吧"} {
			detail = strings.TrimSuffix(detail, suffix)
		}
		detail = strings.Trim(detail, " \t\r\n，,。.!！?？;；:：~～")
		if detail == before {
			break
		}
	}
	return detail
}

func compactActionModifiers(value string) string {
	return strings.NewReplacer(
		"自动", "",
		"直接", "",
		"立即", "",
		"立刻", "",
		"马上", "",
		"现在", "",
	).Replace(value)
}

func compactActionText(value string) string {
	return strings.NewReplacer(
		" ", "",
		"\t", "",
		"\n", "",
		"\r", "",
		"，", "",
		",", "",
	).Replace(strings.ToLower(strings.TrimSpace(value)))
}

func containsActionPhrase(value string, phrases []string) bool {
	for _, phrase := range phrases {
		if phrase != "" && strings.Contains(value, phrase) {
			return true
		}
	}
	return false
}
