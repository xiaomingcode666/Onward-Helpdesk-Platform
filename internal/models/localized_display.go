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
