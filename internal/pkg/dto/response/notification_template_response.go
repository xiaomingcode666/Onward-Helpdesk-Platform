package response

type NotificationTemplateResponse struct {
	ID              int64    `json:"id"`
	TenantID        int64    `json:"tenant_id"`
	Code            string   `json:"code"`
	Name            string   `json:"name"`
	Channel         string   `json:"channel"`
	Language        string   `json:"language"`
	TitleTemplate   string   `json:"title_template"`
	ContentTemplate string   `json:"content_template"`
	Variables       []string `json:"variables"`
	ApprovalStatus  string   `json:"approval_status"`
	ApprovedBy      int64    `json:"approved_by"`
	ApprovedAt      string   `json:"approved_at"`
	Source          string   `json:"source"`
	Editable        bool     `json:"editable"`
	UpdatedAt       string   `json:"updated_at"`
}

type NotificationTemplateListResponse struct {
	Items    []NotificationTemplateResponse `json:"items"`
	Total    int64                          `json:"total"`
	Page     int                            `json:"page"`
	PageSize int                            `json:"page_size"`
}

type NotificationTemplatePreviewResponse struct {
	Title     string `json:"title"`
	Content   string `json:"content"`
	Blocked   bool   `json:"blocked"`
	RuleCode  string `json:"rule_code"`
	RuleLabel string `json:"rule_label"`
}

type NotificationDeliveryAttemptResponse struct {
	ID             int64  `json:"id"`
	TenantID       int64  `json:"tenant_id"`
	DeliveryID     int64  `json:"delivery_id"`
	NotificationID int64  `json:"notification_id"`
	AttemptNo      int    `json:"attempt_no"`
	Channel        string `json:"channel"`
	Status         string `json:"status"`
	Reason         string `json:"reason"`
	Detail         string `json:"detail"`
	TemplateCode   string `json:"template_code"`
	Language       string `json:"language"`
	RecipientID    string `json:"recipient_id"`
	CreatedAt      string `json:"created_at"`
}

type NotificationDeliveryAttemptListResponse struct {
	Items    []NotificationDeliveryAttemptResponse `json:"items"`
	Total    int64                                 `json:"total"`
	Page     int                                   `json:"page"`
	PageSize int                                   `json:"page_size"`
}
