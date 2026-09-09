package seeds

import (
	"remotehelpdesk/cmd/testdata/seedlang"
	"remotehelpdesk/internal/pkg/enums"
)

type AIAgentSeed struct {
	Name                string
	Description         string
	ServiceMode         enums.IMConversationServiceMode
	SystemPrompt        string
	WelcomeMessage      string
	ReplyTimeoutSeconds int
	HandoffMode         enums.AIAgentHandoffMode
	FallbackMode        enums.AIAgentFallbackMode
	FallbackMessage     string
	SortNo              int
}

func AIAgentSeeds(lang seedlang.Language) []AIAgentSeed {
	if lang == seedlang.English {
		return []AIAgentSeed{
			{
				Name:        "Test AI Support Agent",
				Description: "Local test AI support agent",
				ServiceMode: enums.IMConversationServiceModeAIFirst,
				SystemPrompt: `You are working in a customer support system with explicit engineering constraints.
During execution, strictly follow the injected Agent rules and skill rules.
If tool allowlist restrictions exist, call only the currently allowed tools. Ask follow-up questions when information is insufficient; do not fabricate facts or skip required confirmations.
Do not promise processing times, completion times, callbacks, or contact times unless they have been confirmed by system context, tool results, human confirmation, or knowledge base facts.
Do not make commitments on behalf of the human team, technical team, or after-sales team unless the current context contains explicit tool results, human confirmation, or knowledge base facts.
When the user only says that they have sent materials, an email, screenshots, or attachments, only acknowledge the current message or suggest waiting for human confirmation. Do not invent internal handling processes, SLAs, or follow-up arrangements.`,
				WelcomeMessage:      "Hello, how can I help you?",
				ReplyTimeoutSeconds: 180,
				HandoffMode:         enums.AIAgentHandoffModeWaitPool,
				FallbackMode:        enums.AIAgentFallbackModeSuggestRetry,
				FallbackMessage:     "I could not find enough accurate information yet. Please add more details and I will keep checking.",
				SortNo:              10,
			},
		}
	}
	return []AIAgentSeed{
		{
			Name:        "测试AI客服",
			Description: "本地测试 AI 客服 Agent",
			ServiceMode: enums.IMConversationServiceModeAIFirst,
			SystemPrompt: `你正在一个有明确工程约束的客服系统中工作。
执行时必须严格遵守当前注入的 Agent 规则和技能规则。
如果存在工具白名单限制，只能调用当前允许的工具；信息不足时优先追问，不要伪造事实或跳过必要确认。
禁止承诺未经系统确认的处理时效、完成时间、回访时间或联系时间。
禁止代表人工团队、技术团队、售后团队承诺后续动作，除非当前上下文已有明确的工具结果、人工确认或知识库事实支持。
当用户只表示已发送资料、邮件、截图或附件时，只能确认已收到当前消息或建议等待人工确认，不能自行补充内部处理流程、SLA 或跟进安排。`,
			WelcomeMessage:      "您好，有什么可以帮助您的？",
			ReplyTimeoutSeconds: 180,
			HandoffMode:         enums.AIAgentHandoffModeWaitPool,
			FallbackMode:        enums.AIAgentFallbackModeSuggestRetry,
			FallbackMessage:     "我暂时没有找到足够准确的信息。你可以补充具体的问题，我再继续帮你查。",
			SortNo:              10,
		},
	}
}
