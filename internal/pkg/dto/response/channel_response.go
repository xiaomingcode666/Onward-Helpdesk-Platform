package response

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
)

type ChannelResponse struct {
	ID          int64        `json:"id"`
	ChannelType string       `json:"channelType"`
	ChannelID   string       `json:"channelId"`
	AIAgentID   int64        `json:"aiAgentId"`
	AIAgentName string       `json:"aiAgentName,omitempty"`
	Name        string       `json:"name"`
	ConfigJSON  string       `json:"configJson"`
	Status      enums.Status `json:"status"`
	Remark      string       `json:"remark"`
	// WhatsAppSecrets reports which Meta secrets are stored, keyed by request key.
	WhatsAppSecrets map[string]ChannelSecretStatusResponse `json:"whatsappSecrets,omitempty"`
}

// ChannelSecretStatusResponse never carries the secret or its ciphertext.
type ChannelSecretStatusResponse struct {
	Configured        bool   `json:"configured"`
	FingerprintSuffix string `json:"fingerprintSuffix,omitempty"`
	UpdatedAt         string `json:"updatedAt,omitempty"`
}

type WxWorkKFAccountResponse struct {
	OpenKfID        string `json:"openKfId"`
	Name            string `json:"name"`
	Avatar          string `json:"avatar"`
	ManagePrivilege bool   `json:"managePrivilege"`
}

func BuildChannelResponse(item *models.Channel) ChannelResponse {
	if item == nil {
		return ChannelResponse{}
	}
	return ChannelResponse{
		ID:          item.ID,
		ChannelType: item.ChannelType,
		ChannelID:   item.ChannelID,
		AIAgentID:   item.AIAgentID,
		Name:        item.Name,
		ConfigJSON:  item.ConfigJSON,
		Status:      item.Status,
		Remark:      item.Remark,
	}
}
