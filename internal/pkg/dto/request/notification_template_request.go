package request

// SaveNotificationTemplateRequest 新增或修改通知模板。
type SaveNotificationTemplateRequest struct {
	Code            string   `json:"code"`
	Name            string   `json:"name"`
	Channel         string   `json:"channel"`
	Language        string   `json:"language"`
	TitleTemplate   string   `json:"titleTemplate"`
	ContentTemplate string   `json:"contentTemplate"`
	Variables       []string `json:"variables"`
}

// PreviewNotificationTemplateRequest 渲染模板并检查敏感信息，不需要先保存。
type PreviewNotificationTemplateRequest struct {
	Code            string            `json:"code"`
	Channel         string            `json:"channel"`
	Language        string            `json:"language"`
	TitleTemplate   string            `json:"titleTemplate"`
	ContentTemplate string            `json:"contentTemplate"`
	Variables       map[string]string `json:"variables"`
}
