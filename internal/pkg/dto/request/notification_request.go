package request

type CreateNotificationRequest struct {
	TenantID         int64  `json:"tenantId"`
	RecipientUserID  int64  `json:"recipientUserId"`
	RecipientName    string `json:"recipientName"`
	Title            string `json:"title"`
	Content          string `json:"content"`
	NotificationType string `json:"notificationType"`
	BizType          string `json:"bizType"`
	BizID            int64  `json:"bizId"`
	ActionURL        string `json:"actionUrl"`
	Category         string `json:"category"`
	Level            string `json:"level"`
	Channels         string `json:"channels"`
	IdempotencyKey   string `json:"idempotencyKey"`
	// TemplateCode 指定使用的通知模板；留空时按 NotificationType 查找已批准模板。
	TemplateCode string `json:"templateCode"`
	// TemplateVariables 是模板占位符的取值，例如 {"TicketNo": "TK-1"}。
	TemplateVariables map[string]string `json:"templateVariables"`
}

type MarkNotificationReadRequest struct {
	ID int64 `json:"id"`
}

// UpdateMailSettingRequest 保存租户邮箱发送配置。Password 留空表示沿用已保存的密码。
type UpdateMailSettingRequest struct {
	FromAddress  string `json:"fromAddress"`
	FromName     string `json:"fromName"`
	SMTPHost     string `json:"smtpHost"`
	SMTPPort     int    `json:"smtpPort"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	ReplyTo      string `json:"replyTo"`
	RetryPolicy  string `json:"retryPolicy"`
	UseTLS       bool   `json:"useTls"`
	IMAPHost     string `json:"imapHost"`
	IMAPPort     int    `json:"imapPort"`
	IMAPUsername string `json:"imapUsername"`
	IMAPPassword string `json:"imapPassword"`
	IMAPUseTLS   bool   `json:"imapUseTls"`
	IMAPEnabled  bool   `json:"imapEnabled"`
}

// TestMailRequest 发送测试邮件。
type TestMailRequest struct {
	To string `json:"to"`
}

type UpdateNotificationRecipientSettingRequest struct {
	Email           string `json:"email"`
	UseProfileEmail bool   `json:"useProfileEmail"`
	EmailEnabled    bool   `json:"emailEnabled"`
}

type RegisterMobilePushTokenRequest struct {
	Platform   string `json:"platform"`
	Token      string `json:"token"`
	DeviceID   string `json:"deviceId"`
	AppVersion string `json:"appVersion"`
}
