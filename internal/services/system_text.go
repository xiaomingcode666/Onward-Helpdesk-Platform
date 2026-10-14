package services

import (
	"strconv"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/i18nx"
)

// systemText 生成系统文本：返回简体中文渲染结果和可按语言渲染的 Text。
// 中文结果写入原有文本列，保持检索、导出和既有比较逻辑不变；展示时使用 Text 按当前语言渲染。
func systemText(key string, params i18nx.Params) (string, i18nx.Text) {
	text := i18nx.NewText(key, params)
	return text.Render(i18nx.LocaleZhCN), text
}

// canonicalText 返回 Text 的简体中文规范值，用于写入原有文本列。
func canonicalText(text i18nx.Text) string {
	return text.Render(i18nx.LocaleZhCN)
}

// withReasonText 在系统文本后追加原因；原因为空时返回原文本。
func withReasonText(content i18nx.Text, reason string) i18nx.Text {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return content
	}
	return i18nx.NewText("ticketProgress.withReason", i18nx.Params{"content": content, "reason": systemReasonParam(reason)})
}

// systemReasonTexts 是代码内置的原因常量，展示时按语言渲染；其余原因由用户填写，原样展示。
var systemReasonTexts = map[string]string{
	dispatchReasonAutomatic: "ticketProgress.reason.automatic",
}

// systemReasonParam 把代码内置的原因常量转为可翻译文本，其余原因原样保留。
func systemReasonParam(reason string) any {
	reason = strings.TrimSpace(reason)
	if key, ok := systemReasonTexts[reason]; ok {
		return i18nx.NewText(key, nil)
	}
	return reason
}

// ticketStatusText 返回工单状态的可翻译名称。
func ticketStatusText(status enums.TicketStatus) i18nx.Text {
	return i18nx.NewText("ticketStatus."+string(status), nil)
}

// caseStatusText 返回工单主状态（案件阶段）的可翻译名称。
func caseStatusText(status string) i18nx.Text {
	return i18nx.NewText(caseStatusKey(status), nil)
}

// caseStatusKey 返回主状态资源键；历史工单没有主状态时使用 caseStatus.none。
func caseStatusKey(status string) string {
	if strings.TrimSpace(status) == "" {
		return "caseStatus.none"
	}
	return "caseStatus." + status
}

// dispatchFailureReasonText 把派单失败原因码转为可翻译文本；未登记的原因码原样展示。
func dispatchFailureReasonText(code string) any {
	code = strings.TrimSpace(code)
	key := "dispatchFailure." + code
	if code != "" && i18nx.HasKey(key) {
		return i18nx.NewText(key, nil)
	}
	return code
}

// messageSummaryText 返回图片、语音、附件消息的可翻译摘要；其他消息类型返回空文本，由调用方使用原内容。
func messageSummaryText(messageType enums.IMMessageType, filename string) i18nx.Text {
	var key string
	switch messageType {
	case enums.IMMessageTypeImage:
		key = "messageSummary.image"
	case enums.IMMessageTypeAudio:
		key = "messageSummary.audio"
	case enums.IMMessageTypeAttachment:
		key = "messageSummary.attachment"
	default:
		return i18nx.Text{}
	}
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return i18nx.NewText(key, nil)
	}
	return i18nx.NewText(key+"WithName", i18nx.Params{"name": filename})
}

// knowledgeBaseNameParam 返回知识库名称参数：内置知识库按语言渲染，用户创建的知识库原样展示。
func knowledgeBaseNameParam(knowledgeBase *models.KnowledgeBase) any {
	if knowledgeBase == nil {
		return ""
	}
	if !knowledgeBase.NameText.IsZero() {
		return knowledgeBase.NameText
	}
	return knowledgeBase.Name
}

// SystemReasonParam 把代码内置的原因常量转为可翻译文本，其余原因原样保留。
func SystemReasonParam(reason string) any {
	return systemReasonParam(reason)
}

// SystemReasonTextVariable 在原因为代码内置常量时返回可翻译的模板变量，用户填写的原因返回 nil。
func SystemReasonTextVariable(name, reason string) map[string]i18nx.Text {
	if text, ok := systemReasonParam(reason).(i18nx.Text); ok {
		return map[string]i18nx.Text{name: text}
	}
	return nil
}

// maxJoinedTextLines 是 text.lines.N 资源支持的最大行数。
const maxJoinedTextLines = 4

// JoinTextLines 把多行系统文本或用户内容按行拼接为一个可翻译文本，最多 4 行，多余的行合并到最后一行。
func JoinTextLines(lines ...any) i18nx.Text {
	if len(lines) > maxJoinedTextLines {
		tail := JoinTextLines(lines[maxJoinedTextLines-1:]...)
		lines = append(append([]any{}, lines[:maxJoinedTextLines-1]...), tail)
	}
	params := make(i18nx.Params, len(lines))
	for index, line := range lines {
		params["l"+strconv.Itoa(index+1)] = line
	}
	return i18nx.NewText("text.lines."+strconv.Itoa(len(lines)), params)
}

// ProductAgentAutoDeployReviewText 是产品创建时自动部署机器人的审核备注。
var ProductAgentAutoDeployReviewText = i18nx.NewText("aiAgentRelease.reviewComment.productAutoDeploy", nil)

// systemComments 是系统写入备注列的固定文本；存储的是中文规范值，展示时按语言渲染。
var systemComments = []i18nx.Text{ProductAgentAutoDeployReviewText}

// LocalizeSystemComment 按语言显示系统写入的固定备注，用户填写的备注原样返回。
func LocalizeSystemComment(comment, locale string) string {
	for _, text := range systemComments {
		if canonicalText(text) == comment {
			return text.Render(locale)
		}
	}
	return comment
}
