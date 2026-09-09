package registry

import "remotehelpdesk/internal/ai/workflow/dsl"

const (
	NodeTypeStart                     = "start"
	NodeTypeEntryContext              = "entry_context"
	NodeTypeServiceAccessPolicy       = "service_access_policy"
	NodeTypeConversationUnderstanding = "conversation_understanding"
	NodeTypeReplyPolicy               = "reply_policy"
	NodeTypeKnowledgeRetrieve         = "knowledge_retrieve"
	NodeTypeKnowledgeMerge            = "knowledge_merge"
	NodeTypeAnswerabilityGate         = "answerability_gate"
	NodeTypeLLMReply                  = "llm_reply"
	NodeTypeCondition                 = "condition"
	NodeTypeAnalyzeConversation       = "analyze_conversation"
	NodeTypePrepareTicketDraft        = "prepare_ticket_draft"
	NodeTypeHumanConfirm              = "human_confirm"
	NodeTypeCreateTicket              = "create_ticket"
	NodeTypeCreateVideoMeeting        = "create_video_meeting"
	NodeTypeCreateKnowledgeCandidate  = "create_knowledge_candidate"
	NodeTypeHandoffToHuman            = "handoff_to_human"
	NodeTypeSendReply                 = "send_reply"
	NodeTypeSubflow                   = "subflow"
	NodeTypeLoop                      = "loop"
	NodeTypeEnd                       = "end"
)

func DefaultRegistry() *Registry {
	return NewRegistry(
		NodeSpec{
			Type:        NodeTypeStart,
			Title:       "开始",
			Description: "会话流程入口。",
			RiskLevel:   NodeRiskLevelLow,
			OutputSchema: []VariableSpec{
				output("tenantId", VariableTypeInteger, "当前租户 ID。"),
				output("productId", VariableTypeInteger, "当前产品 ID。"),
				output("productModelId", VariableTypeInteger, "当前产品型号 ID。"),
				output("deviceId", VariableTypeInteger, "当前设备 ID。"),
				output("serviceCodeId", VariableTypeInteger, "当前服务码 ID。"),
				output("customerEntrySessionId", VariableTypeInteger, "当前客户入口会话 ID。"),
				output("conversationId", VariableTypeInteger, "当前会话 ID。"),
				output("conversationServiceMode", VariableTypeString, "当前会话服务模式。"),
				output("messageId", VariableTypeInteger, "当前用户消息 ID。"),
				output("aiAgentId", VariableTypeInteger, "当前 AI 机器人 ID。"),
				output("agentReleaseId", VariableTypeInteger, "当前生产 Release ID。"),
				output("userMessage", VariableTypeString, "当前用户消息内容。"),
				output("locale", VariableTypeString, "当前会话语言。"),
				output("regionCode", VariableTypeString, "当前服务区域。"),
				output("audience", VariableTypeString, "当前知识可见受众。"),
			},
		},
		NodeSpec{
			Type:        NodeTypeEntryContext,
			Title:       "入口与设备识别",
			Description: "识别快速问答、服务码和设备绑定上下文，输出稳定的接入结果。",
			RiskLevel:   NodeRiskLevelLow,
			InputSchema: []VariableSpec{
				optionalInput("productId", VariableTypeInteger, "当前产品 ID。"),
				optionalInput("productModelId", VariableTypeInteger, "当前产品型号 ID。"),
				optionalInput("deviceId", VariableTypeInteger, "当前设备 ID。"),
				optionalInput("serviceCodeId", VariableTypeInteger, "当前服务码 ID。"),
				optionalInput("customerEntrySessionId", VariableTypeInteger, "当前客户入口会话 ID。"),
			},
			OutputSchema: []VariableSpec{
				output("entryMode", VariableTypeString, "识别出的入口模式。"),
				output("contextLevel", VariableTypeString, "当前售后上下文完整度。"),
				output("deviceBound", VariableTypeBoolean, "是否已绑定到具体设备。"),
				output("productBound", VariableTypeBoolean, "是否已识别产品。"),
				output("requiresContextCollection", VariableTypeBoolean, "是否需要客户补录产品或设备信息。"),
				output("reason", VariableTypeString, "入口识别原因。"),
			},
		},
		NodeSpec{
			Type:        NodeTypeServiceAccessPolicy,
			Title:       "企业服务策略",
			Description: "按设备上下文和企业规则决定 AI、人工、工单及客户侧入口权限。",
			RiskLevel:   NodeRiskLevelLow,
			InputSchema: []VariableSpec{
				requiredInput("contextLevel", VariableTypeString, "入口识别得到的上下文完整度。"),
				requiredInput("deviceBound", VariableTypeBoolean, "是否已绑定到具体设备。"),
				optionalInput("conversationServiceMode", VariableTypeString, "机器人或会话当前服务模式。"),
			},
			OutputSchema: []VariableSpec{
				output("serviceMode", VariableTypeString, "本次会话实际采用的服务模式。"),
				output("allowHumanHandoff", VariableTypeBoolean, "本次会话是否允许转人工。"),
				output("allowTicketCreation", VariableTypeBoolean, "本次会话是否允许创建工单。"),
				output("allowVideoMeeting", VariableTypeBoolean, "本次会话是否允许创建视频会议。"),
				output("showHumanEntry", VariableTypeBoolean, "客户侧是否显示人工入口。"),
				output("showTicketEntry", VariableTypeBoolean, "客户侧是否显示工单入口。"),
				output("showDeviceEntry", VariableTypeBoolean, "客户侧是否显示设备入口。"),
				output("conversationTag", VariableTypeString, "本次会话的初始业务标签。"),
				output("reason", VariableTypeString, "服务策略判断原因。"),
			},
		},
		NodeSpec{
			Type:        NodeTypeConversationUnderstanding,
			Title:       "会话理解",
			Description: "识别客户消息意图和回答范围，供检索与分流使用。",
			RiskLevel:   NodeRiskLevelLow,
			InputSchema: []VariableSpec{
				requiredInput("userMessage", VariableTypeString, "当前用户消息内容。"),
			},
			OutputSchema: []VariableSpec{
				output("normalizedMessage", VariableTypeString, "标准化后的客户消息。"),
				output("messageIntent", VariableTypeString, "识别出的客户消息意图。"),
				output("answerScope", VariableTypeString, "建议的回答范围。"),
				output("confidence", VariableTypeNumber, "意图识别置信度。"),
				output("riskSignals", VariableTypeStringArray, "识别出的风险信号。"),
				output("reason", VariableTypeString, "判断原因。"),
			},
			DefaultInputs: map[string]dsl.VariableSelector{
				"userMessage": {NodeID: "start_1", Field: "userMessage"},
			},
		},
		NodeSpec{
			Type:        NodeTypeReplyPolicy,
			Title:       "回复策略",
			Description: "结合会话理解和机器人策略决定下一步客服动作。",
			RiskLevel:   NodeRiskLevelLow,
			InputSchema: []VariableSpec{
				requiredInput("messageIntent", VariableTypeString, "识别出的客户消息意图。"),
				requiredInput("answerScope", VariableTypeString, "建议的回答范围。"),
				optionalInput("userMessage", VariableTypeString, "当前用户消息内容。"),
				optionalInput("riskSignals", VariableTypeStringArray, "识别出的风险信号。"),
				optionalInput("answerability", VariableTypeString, "知识可回答性判断结果。"),
				optionalInput("allowHumanHandoff", VariableTypeBoolean, "当前服务策略是否允许转人工。"),
				optionalInput("allowTicketCreation", VariableTypeBoolean, "当前服务策略是否允许创建工单。"),
			},
			OutputSchema: []VariableSpec{
				output("action", VariableTypeString, "选定的策略动作。"),
				output("replyText", VariableTypeString, "策略可直接回答时返回的客户可见内容。"),
				output("reason", VariableTypeString, "策略判断原因。"),
				output("requiresFlow", VariableTypeBoolean, "是否继续执行后续流程动作。"),
				output("targetFlow", VariableTypeString, "建议进入的目标流程。"),
				output("finalReplySource", VariableTypeString, "最终回复的来源类别。"),
			},
		},
		NodeSpec{
			Type:        NodeTypeKnowledgeRetrieve,
			Title:       "知识检索",
			Description: "按当前用户消息检索产品知识库。",
			RiskLevel:   NodeRiskLevelLow,
			InputSchema: []VariableSpec{
				requiredInput("query", VariableTypeString, "知识检索问题。"),
			},
			OutputSchema: []VariableSpec{
				output("items", VariableTypeObjectArray, "检索到的知识条目。"),
				output("citations", VariableTypeObjectArray, "客户可见的知识来源。"),
				output("summary", VariableTypeString, "知识检索摘要。"),
				output("knowledgeBaseIds", VariableTypeIntegerArray, "当前检索节点使用的知识库 ID。"),
			},
			DefaultInputs: map[string]dsl.VariableSelector{
				"query": {NodeID: "start_1", Field: "userMessage"},
			},
		},
		NodeSpec{
			Type:        NodeTypeKnowledgeMerge,
			Title:       "知识汇聚",
			Description: "合并并去重多个并行知识检索结果。",
			RiskLevel:   NodeRiskLevelLow,
			OutputSchema: []VariableSpec{
				output("items", VariableTypeObjectArray, "合并后的知识条目。"),
				output("summary", VariableTypeString, "合并后的知识上下文。"),
				output("sourceCount", VariableTypeInteger, "已配置的检索来源数量。"),
			},
		},
		NodeSpec{
			Type:        NodeTypeAnswerabilityGate,
			Title:       "可回答判断",
			Description: "判断已检索知识是否足以支撑本次回答。",
			RiskLevel:   NodeRiskLevelLow,
			InputSchema: []VariableSpec{
				requiredInput("userMessage", VariableTypeString, "当前用户消息内容。"),
				requiredInput("knowledgeItems", VariableTypeObjectArray, "已检索到的知识条目。"),
			},
			OutputSchema: []VariableSpec{
				output("answerability", VariableTypeString, "可回答性判断结果。"),
				output("reason", VariableTypeString, "判断原因。"),
				output("confidence", VariableTypeNumber, "可回答性置信度。"),
				output("bestScore", VariableTypeNumber, "最高知识检索得分。"),
				output("matchedTermCount", VariableTypeInteger, "问题与知识匹配的有效词数量。"),
				output("matchedItemCount", VariableTypeInteger, "与问题匹配的知识条目数量。"),
				output("evidenceItemCount", VariableTypeInteger, "可用于判断的知识条目数量。"),
			},
		},
		NodeSpec{
			Type:        NodeTypeLLMReply,
			Title:       "模型回复",
			Description: "使用当前配置的对话模型生成回复或结构化分析。",
			RiskLevel:   NodeRiskLevelMedium,
			InputSchema: []VariableSpec{
				requiredInput("userMessage", VariableTypeString, "当前用户消息内容。"),
				optionalInput("knowledgeItems", VariableTypeObjectArray, "已检索到的知识条目。"),
			},
			OutputSchema: []VariableSpec{
				output("replyText", VariableTypeString, "生成的回复内容。"),
			},
		},
		NodeSpec{
			Type:        NodeTypeCondition,
			Title:       "条件分流",
			Description: "按受控流程变量将会话路由到不同节点。",
			RiskLevel:   NodeRiskLevelLow,
			OutputSchema: []VariableSpec{
				output("matched", VariableTypeBoolean, "条件是否命中。"),
			},
		},
		NodeSpec{
			Type:        NodeTypeAnalyzeConversation,
			Title:       "会话分析",
			Description: "分析用户意图、风险和建议的下一步动作。",
			RiskLevel:   NodeRiskLevelLow,
			InputSchema: []VariableSpec{
				requiredInput("userMessage", VariableTypeString, "当前用户消息内容。"),
			},
			OutputSchema: []VariableSpec{
				output("intent", VariableTypeString, "识别出的用户意图。"),
				output("riskLevel", VariableTypeString, "识别出的风险等级。"),
				output("needTicket", VariableTypeBoolean, "是否建议创建工单。"),
				output("needHumanHandoff", VariableTypeBoolean, "是否建议转人工。"),
			},
		},
		NodeSpec{
			Type:        NodeTypePrepareTicketDraft,
			Title:       "整理工单草稿",
			Description: "根据会话上下文生成工单草稿。",
			RiskLevel:   NodeRiskLevelMedium,
			InputSchema: []VariableSpec{
				requiredInput("issue", VariableTypeString, "问题摘要。"),
			},
			OutputSchema: []VariableSpec{
				output("ticketDraft", VariableTypeObject, "待确认的工单草稿。"),
			},
		},
		NodeSpec{
			Type:          NodeTypeHumanConfirm,
			Title:         "人工确认",
			Description:   "中断流程并等待用户明确确认。",
			RiskLevel:     NodeRiskLevelMedium,
			Interruptible: true,
			InputSchema: []VariableSpec{
				requiredInput("prompt", VariableTypeString, "需要向用户展示的确认提示。"),
			},
			OutputSchema: []VariableSpec{
				output("confirmed", VariableTypeBoolean, "用户是否确认。"),
				output("responseText", VariableTypeString, "用户的确认回复。"),
			},
		},
		NodeSpec{
			Type:                            NodeTypeCreateTicket,
			Title:                           "创建工单",
			Description:                     "根据已确认的工单草稿创建工单。",
			RiskLevel:                       NodeRiskLevelHigh,
			RequiresConfirmationPredecessor: true,
			InputSchema: []VariableSpec{
				requiredInput("ticketDraft", VariableTypeObject, "已确认的工单草稿。"),
				requiredInput("confirmed", VariableTypeBoolean, "用户确认结果。"),
			},
			OutputSchema: []VariableSpec{
				output("ticketId", VariableTypeInteger, "已创建的工单 ID。"),
				output("ticketNo", VariableTypeString, "已创建的工单号。"),
				output("created", VariableTypeBoolean, "工单是否创建成功。"),
				output("message", VariableTypeString, "客户可见的建单结果。"),
			},
		},
		NodeSpec{
			Type:                            NodeTypeCreateVideoMeeting,
			Title:                           "创建视频会议",
			Description:                     "为已分配并处于可协作状态的工单创建或复用视频会议。",
			RiskLevel:                       NodeRiskLevelHigh,
			RequiresConfirmationPredecessor: true,
			InputSchema: []VariableSpec{
				requiredInput("ticketId", VariableTypeInteger, "已分配工程师的工单 ID。"),
				requiredInput("confirmed", VariableTypeBoolean, "用户或工程师确认结果。"),
			},
			OutputSchema: []VariableSpec{
				output("meetingId", VariableTypeString, "已创建或复用的会议 ID。"),
				output("roomName", VariableTypeString, "会议房间名。"),
				output("created", VariableTypeBoolean, "会议是否创建或复用成功。"),
				output("message", VariableTypeString, "客户可见的会议创建结果。"),
			},
		},
		NodeSpec{
			Type:                            NodeTypeCreateKnowledgeCandidate,
			Title:                           "创建知识候选",
			Description:                     "把已处理工单沉淀为待审核的产品知识候选。",
			RiskLevel:                       NodeRiskLevelHigh,
			RequiresConfirmationPredecessor: true,
			InputSchema: []VariableSpec{
				requiredInput("ticketId", VariableTypeInteger, "需要沉淀知识的工单 ID。"),
				requiredInput("confirmed", VariableTypeBoolean, "用户或工程师确认结果。"),
				optionalInput("knowledgeBaseId", VariableTypeInteger, "目标知识库 ID；未提供时由产品知识审核流程决定。"),
				optionalInput("title", VariableTypeString, "知识候选标题。"),
				optionalInput("suggestion", VariableTypeString, "知识建议内容。"),
				optionalInput("rootCauseSummary", VariableTypeString, "根因摘要。"),
				optionalInput("solutionSummary", VariableTypeString, "解决方案摘要。"),
			},
			OutputSchema: []VariableSpec{
				output("candidateId", VariableTypeInteger, "知识候选 ID。"),
				output("created", VariableTypeBoolean, "是否创建了新的候选；false 表示复用已有候选或已取消。"),
				output("reviewStatus", VariableTypeString, "知识候选审核状态。"),
				output("message", VariableTypeString, "客户或工程师可见的知识沉淀结果。"),
			},
		},
		NodeSpec{
			Type:        NodeTypeHandoffToHuman,
			Title:       "转人工",
			Description: "将当前会话转交给人工服务团队。",
			RiskLevel:   NodeRiskLevelHigh,
			InputSchema: []VariableSpec{
				requiredInput("reason", VariableTypeString, "转人工原因。"),
				optionalInput("confirmed", VariableTypeBoolean, "用户确认结果。"),
			},
			OutputSchema: []VariableSpec{
				output("handoffId", VariableTypeInteger, "转人工操作 ID。"),
				output("reason", VariableTypeString, "转人工原因。"),
				output("decision", VariableTypeString, "转人工分配结果。"),
				output("teamId", VariableTypeInteger, "已分配或待接入的团队 ID。"),
				output("assigneeId", VariableTypeInteger, "已分配的客服人员 ID。"),
				output("ticketId", VariableTypeInteger, "幂等转人工关联的工单 ID。"),
				output("ticketNo", VariableTypeString, "幂等转人工关联的工单号。"),
				output("ticketCreated", VariableTypeBoolean, "本次转人工是否创建了工单。"),
				output("message", VariableTypeString, "客户可见的转人工提示。"),
			},
		},
		NodeSpec{
			Type:        NodeTypeSendReply,
			Title:       "发送回复",
			Description: "返回或发送客户可见的回复内容。",
			RiskLevel:   NodeRiskLevelLow,
			InputSchema: []VariableSpec{
				requiredInput("replyText", VariableTypeString, "客户可见的回复内容。"),
				optionalInput("citations", VariableTypeObjectArray, "随回复展示的知识来源。"),
			},
			OutputSchema: []VariableSpec{
				output("sent", VariableTypeBoolean, "回复是否发送成功。"),
				output("replyMessageId", VariableTypeInteger, "回复消息 ID。"),
			},
		},
		NodeSpec{
			Type:        NodeTypeSubflow,
			Title:       "子流程",
			Description: "在同一租户和产品范围内执行不可变的已发布流程版本。",
			RiskLevel:   NodeRiskLevelMedium,
			OutputSchema: []VariableSpec{
				output("status", VariableTypeString, "子流程执行状态。"),
				output("replyText", VariableTypeString, "子流程回复内容。"),
				output("nodePath", VariableTypeStringArray, "子流程已执行的节点路径。"),
			},
		},
		NodeSpec{
			Type:        NodeTypeLoop,
			Title:       "循环排障",
			Description: "按固定版本循环执行诊断子流程，直到满足退出条件或达到次数上限。",
			RiskLevel:   NodeRiskLevelMedium,
			OutputSchema: []VariableSpec{
				output("status", VariableTypeString, "最后一次子流程状态。"),
				output("replyText", VariableTypeString, "最后一次子流程回复内容。"),
				output("iteration", VariableTypeInteger, "已完成的循环次数。"),
				output("completed", VariableTypeBoolean, "是否满足退出条件。"),
			},
		},
		NodeSpec{
			Type:        NodeTypeEnd,
			Title:       "结束",
			Description: "结束本次流程执行。",
			RiskLevel:   NodeRiskLevelLow,
			OutputSchema: []VariableSpec{
				output("status", VariableTypeString, "流程最终状态。"),
			},
		},
	)
}

func requiredInput(name string, variableType VariableType, description string) VariableSpec {
	return VariableSpec{Name: name, Type: variableType, Required: true, Description: description}
}

func optionalInput(name string, variableType VariableType, description string) VariableSpec {
	return VariableSpec{Name: name, Type: variableType, Description: description}
}

func output(name string, variableType VariableType, description string) VariableSpec {
	return VariableSpec{Name: name, Type: variableType, Description: description}
}
