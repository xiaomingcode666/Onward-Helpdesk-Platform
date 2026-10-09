package request

type CreateChannelRequest struct {
	ChannelType string `json:"channelType"`
	AIAgentID   int64  `json:"aiAgentId"`
	Name        string `json:"name"`
	ConfigJSON  string `json:"configJson"`
	Status      int    `json:"status"`
	Remark      string `json:"remark"`
	// WhatsAppSecrets is write-only and only accepted for WhatsApp channels.
	WhatsAppSecrets *WhatsAppSecretsInput `json:"whatsappSecrets,omitempty"`
}

// WhatsAppSecretsInput carries write-only Meta credentials. Empty values keep
// the stored secret; Clear lists interface keys whose stored secret is removed.
type WhatsAppSecretsInput struct {
	MetaAccessToken    string   `json:"metaAccessToken"`
	MetaAppSecret      string   `json:"metaAppSecret"`
	WebhookVerifyToken string   `json:"webhookVerifyToken"`
	Clear              []string `json:"clear"`
}

type UpdateChannelRequest struct {
	ID int64 `json:"id"`
	CreateChannelRequest
}

type UpdateChannelStatusRequest struct {
	ID     int64 `json:"id"`
	Status int   `json:"status"`
}

type DeleteChannelRequest struct {
	ID int64 `json:"id"`
}

type ResetChannelUserTokenSecretRequest struct {
	ID int64 `json:"id"`
}
