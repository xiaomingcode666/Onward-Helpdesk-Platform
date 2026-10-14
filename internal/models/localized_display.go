package models

import "remotehelpdesk/internal/pkg/i18nx"

// displayText 优先按语言渲染系统生成的文本；为空时表示内容由用户定义，原样返回。
func displayText(text i18nx.Text, stored string, locale string) string {
	if text.IsZero() {
		return stored
	}
	return text.Render(locale)
}

func (m *KnowledgeBase) DisplayName(locale string) string {
	return displayText(m.NameText, m.Name, locale)
}

func (m *KnowledgeBase) DisplayDescription(locale string) string {
	return displayText(m.DescriptionText, m.Description, locale)
}

func (m *AIAgent) DisplayName(locale string) string {
	return displayText(m.NameText, m.Name, locale)
}

func (m *AIAgent) DisplayDescription(locale string) string {
	return displayText(m.DescriptionText, m.Description, locale)
}

func (m *AgentTeam) DisplayName(locale string) string {
	return displayText(m.NameText, m.Name, locale)
}

func (m *AgentTeam) DisplayDescription(locale string) string {
	return displayText(m.DescriptionText, m.Description, locale)
}

func (m *Department) DisplayName(locale string) string {
	return displayText(m.NameText, m.Name, locale)
}

func (m *TicketProgress) DisplayContent(locale string) string {
	return displayText(m.ContentText, m.Content, locale)
}

func (m *Ticket) DisplaySupportReason(locale string) string {
	return displayText(m.SupportReasonText, m.SupportReason, locale)
}

func (m *TicketGovernance) DisplayPriorityExplanation(locale string) string {
	return displayText(m.PriorityExplanationText, m.PriorityExplanation, locale)
}

// builtinWorkflowText 平台内置流程按编码提供多语言名称和描述；内容被修改后按存储值显示。
func (m *AIWorkflow) builtinWorkflowText(field, stored string) (i18nx.Text, bool) {
	if m.Scope != AIWorkflowScopePlatform || m.Code == "" {
		return i18nx.Text{}, false
	}
	key := "builtin.aiWorkflow." + m.Code + "." + field
	if !i18nx.HasKey(key) {
		return i18nx.Text{}, false
	}
	text := i18nx.NewText(key, nil)
	return text, text.MatchesAnyLocale(stored)
}

func (m *AIWorkflow) DisplayName(locale string) string {
	if text, ok := m.builtinWorkflowText("name", m.Name); ok {
		return text.Render(locale)
	}
	return m.Name
}

func (m *AIWorkflow) DisplayDescription(locale string) string {
	if text, ok := m.builtinWorkflowText("description", m.Description); ok {
		return text.Render(locale)
	}
	return m.Description
}
