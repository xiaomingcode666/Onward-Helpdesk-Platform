package enums

import "remotehelpdesk/internal/pkg/i18nx"

// enumLabel 按语言渲染枚举名称；key 为空表示没有对应名称。
func enumLabel(locale, key string) string {
	if key == "" {
		return ""
	}
	return i18nx.NewText(key, nil).Render(locale)
}
