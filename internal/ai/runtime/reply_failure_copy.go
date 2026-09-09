package runtime

import (
	"strings"
	"unicode"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/utils"
	svc "remotehelpdesk/internal/services"
)

const (
	aiReplyFailureKindRuntime = "runtime"
	aiReplyFailureKindQuota   = "quota"
)

func buildAIReplyFailureText(conversation models.Conversation, message models.Message, aiAgent models.AIAgent, kind string) string {
	english := aiReplyMessageLooksEnglish(message)
	prefix := "AI 服务暂时不可用，请稍后重试。"
	if kind == aiReplyFailureKindQuota {
		prefix = "本产品的 AI 服务额度已用完，已停止继续调用模型。"
	}
	if english {
		prefix = "AI support is temporarily unavailable. Please try again later."
		if kind == aiReplyFailureKindQuota {
			prefix = "The AI service quota for this product has been exhausted, so no further model calls will be made."
		}
	}
	return strings.TrimSpace(prefix + " " + buildAIReplyFailureActionText(conversation, aiAgent, english))
}

func buildAIReplyFailureActionText(conversation models.Conversation, aiAgent models.AIAgent, english bool) string {
	allowsHuman := svc.CustomerConversationAllowsHumanHandoff(&conversation, &aiAgent)
	allowsTicket := svc.CustomerConversationAllowsTicketCreation(&conversation, &aiAgent)
	if english {
		switch {
		case allowsHuman && allowsTicket:
			return "Use the 'Human support' button or submit a ticket so a technical engineer can continue."
		case allowsHuman:
			return "Use the 'Human support' button so a technical engineer can continue."
		case allowsTicket:
			return "Submit a ticket so a technical engineer can continue."
		default:
			return "This workflow will not create a ticket or transfer to a human. Add the symptoms, fault code, and troubleshooting already attempted, then try again."
		}
	}
	switch {
	case allowsHuman && allowsTicket:
		return "你可以点击“转人工”或提交工单，由技术工程师继续处理。"
	case allowsHuman:
		return "你可以点击“转人工”，由技术工程师继续处理。"
	case allowsTicket:
		return "你可以提交工单，由技术工程师继续处理。"
	default:
		return "当前流程不会创建工单或转人工，请补充故障现象、故障码和已尝试步骤后再试。"
	}
}

func buildLateAIReplyFailureSystemText(message models.Message) string {
	if aiReplyMessageLooksEnglish(message) {
		return "Automatic AI handling is temporarily unavailable. An engineer is now handling this conversation. You can continue providing the symptoms, fault code, and troubleshooting already attempted."
	}
	return "AI 自动处理暂时不可用，当前会话已由工程师继续处理。你可以继续补充故障现象、故障码和已尝试步骤。"
}

func aiReplyMessageLooksEnglish(message models.Message) bool {
	value := strings.ToLower(utils.BuildRuntimeMessageText(message.MessageType, message.Content))
	if value == "" {
		return false
	}
	latinCount := 0
	for _, character := range value {
		if unicode.Is(unicode.Han, character) {
			return false
		}
		if unicode.Is(unicode.Latin, character) {
			latinCount++
		}
	}
	if latinCount < 2 {
		return false
	}
	markers := map[string]struct{}{
		"a": {}, "an": {}, "and": {}, "are": {}, "can": {}, "could": {}, "device": {}, "equipment": {},
		"hello": {}, "help": {}, "how": {}, "i": {}, "is": {}, "it": {}, "machine": {}, "my": {},
		"please": {}, "reset": {}, "restart": {}, "safe": {}, "should": {}, "support": {}, "the": {},
		"this": {}, "to": {}, "warranty": {}, "what": {}, "when": {}, "where": {}, "why": {}, "you": {},
	}
	for _, word := range strings.FieldsFunc(value, func(character rune) bool {
		return !unicode.IsLetter(character)
	}) {
		if _, ok := markers[word]; ok {
			return true
		}
	}
	return false
}
