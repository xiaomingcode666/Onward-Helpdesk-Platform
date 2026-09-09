package capability

import (
	"strings"

	"remotehelpdesk/internal/ai/workflow/dsl"
	workflowregistry "remotehelpdesk/internal/ai/workflow/registry"
)

const SafeKnowledgeFallbackMessage = "当前知识不足以确认具体结论。我可以根据你的问题继续澄清；涉及保修、服务时间、服务区域或设备操作时，请以已发布资料和正式服务协议为准。"

// Set derives runtime capabilities from workflow nodes. Callers use the
// immutable definition instead of workflow names or product-specific codes.
type Set struct {
	HumanHandoff       bool
	TicketCreation     bool
	VideoMeeting       bool
	KnowledgeCandidate bool
}

func FromDefinition(definition dsl.Definition) Set {
	return Set{
		HumanHandoff:       definition.HasNodeType(workflowregistry.NodeTypeHandoffToHuman),
		TicketCreation:     definition.HasNodeType(workflowregistry.NodeTypeCreateTicket),
		VideoMeeting:       definition.HasNodeType(workflowregistry.NodeTypeCreateVideoMeeting),
		KnowledgeCandidate: definition.HasNodeType(workflowregistry.NodeTypeCreateKnowledgeCandidate),
	}
}

// SanitizeFallbackMessage prevents a release fallback from promising an action
// that its immutable workflow cannot execute.
func (c Set) SanitizeFallbackMessage(value string) string {
	message := strings.TrimSpace(value)
	if message == "" {
		return ""
	}
	lower := strings.ToLower(message)
	if !c.HumanHandoff && containsAny(lower,
		"转人工", "人工客服", "人工工程师", "真人客服", "human agent", "live agent", "handoff",
	) {
		return SafeKnowledgeFallbackMessage
	}
	if !c.TicketCreation && containsAny(lower,
		"创建工单", "提交工单", "生成工单", "新建工单", "create ticket", "open ticket", "submit ticket",
	) {
		return SafeKnowledgeFallbackMessage
	}
	if !c.VideoMeeting && containsAny(lower, "创建视频", "发起视频", "视频会议", "video meeting") {
		return SafeKnowledgeFallbackMessage
	}
	if !c.KnowledgeCandidate && containsAny(lower, "沉淀知识", "发布知识", "知识候选", "knowledge candidate") {
		return SafeKnowledgeFallbackMessage
	}
	return message
}

// RuntimeInstruction tells the model which side effects are unavailable. Tool
// blocking remains the enforcement layer; this instruction keeps replies from
// offering actions that the workflow cannot execute.
func (c Set) RuntimeInstruction() string {
	unavailable := make([]string, 0, 4)
	if !c.HumanHandoff {
		unavailable = append(unavailable, "转人工")
	}
	if !c.TicketCreation {
		unavailable = append(unavailable, "创建工单")
	}
	if !c.VideoMeeting {
		unavailable = append(unavailable, "发起视频会议")
	}
	if !c.KnowledgeCandidate {
		unavailable = append(unavailable, "创建知识候选")
	}
	if len(unavailable) == 0 {
		return ""
	}
	return "工作流能力边界：当前流程不提供" + strings.Join(unavailable, "、") + "。不得承诺、建议用户确认或声称已经执行这些动作；应继续在当前流程能力内提供帮助。"
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
