package services

import (
	"encoding/json"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"strings"
)

type imMessageAssetPayload struct {
	AssetID          string              `json:"assetId"`
	Provider         enums.AssetProvider `json:"provider,omitempty"`
	StorageKey       string              `json:"storageKey,omitempty"`
	Filename         string              `json:"filename,omitempty"`
	FileSize         int64               `json:"fileSize,omitempty"`
	MimeType         string              `json:"mimeType,omitempty"`
	URL              string              `json:"url,omitempty"`
	DurationSeconds  int                 `json:"durationSeconds,omitempty"`
	Source           string              `json:"source,omitempty"`
	CollaborationID  int64               `json:"collaborationId,omitempty"`
	PartnerAccountID int64               `json:"partnerAccountId,omitempty"`
	TicketProgressID int64               `json:"ticketProgressId,omitempty"`
}

func parseIMMessageAssetPayload(payload string) (*imMessageAssetPayload, error) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return nil, errorsx.InvalidParamI18n("error.e0346")
	}
	ret := &imMessageAssetPayload{}
	if err := json.Unmarshal([]byte(payload), ret); err != nil {
		return nil, errorsx.InvalidParamI18n("error.e0344")
	}
	ret.AssetID = strings.TrimSpace(ret.AssetID)
	ret.Provider = enums.AssetProvider(strings.TrimSpace(string(ret.Provider)))
	ret.StorageKey = strings.TrimSpace(ret.StorageKey)
	if ret.AssetID == "" {
		return nil, errorsx.InvalidParamI18n("error.e0345")
	}
	return ret, nil
}

func buildIMMessageAssetPayload(asset *models.Asset) (string, error) {
	if asset == nil {
		return "", errorsx.InvalidParamI18n("error.e0342")
	}
	payload, err := json.Marshal(imMessageAssetPayload{
		AssetID:    asset.AssetID,
		Provider:   asset.Provider,
		StorageKey: asset.StorageKey,
		Filename:   asset.Filename,
		FileSize:   asset.FileSize,
		MimeType:   asset.MimeType,
	})
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func buildIMMessageAssetPayloadForResponse(payload string) string {
	assetPayload, err := parseIMMessageAssetPayload(payload)
	if err != nil {
		return strings.TrimSpace(payload)
	}
	assetPayload = hydrateIMMessageAssetPayload(assetPayload)
	assetPayload.Provider = ""
	assetPayload.StorageKey = ""
	assetPayload.URL = ""
	data, err := json.Marshal(assetPayload)
	if err != nil {
		return strings.TrimSpace(payload)
	}
	return string(data)
}

func hydrateIMMessageAssetPayload(payload *imMessageAssetPayload) *imMessageAssetPayload {
	if payload == nil {
		return nil
	}
	if payload.Provider != "" && payload.StorageKey != "" {
		return payload
	}
	if payload.AssetID == "" {
		return payload
	}
	asset := AssetService.GetByAssetID(payload.AssetID)
	if asset == nil {
		return payload
	}
	if payload.Provider == "" {
		payload.Provider = asset.Provider
	}
	if payload.StorageKey == "" {
		payload.StorageKey = strings.TrimSpace(asset.StorageKey)
	}
	if payload.Filename == "" {
		payload.Filename = strings.TrimSpace(asset.Filename)
	}
	if payload.FileSize <= 0 {
		payload.FileSize = asset.FileSize
	}
	if payload.MimeType == "" {
		payload.MimeType = strings.TrimSpace(asset.MimeType)
	}
	return payload
}

func validateConversationAsset(asset *models.Asset, conversationID int64, messageType enums.IMMessageType) error {
	if asset == nil {
		return errorsx.InvalidParamI18n("error.e0342")
	}
	if asset.Status != enums.AssetStatusSuccess {
		return errorsx.InvalidParamI18n("error.e0343")
	}
	conversation := ConversationService.Get(conversationID)
	if conversation == nil {
		return errorsx.InvalidParamI18n("error.e0116")
	}
	if asset.TenantID != conversation.TenantID {
		return errorsx.Forbidden("message asset does not belong to the conversation tenant")
	}
	if asset.ConversationID > 0 && asset.ConversationID != conversationID {
		return errorsx.Forbidden("message asset does not belong to the conversation")
	}
	if asset.ConversationID == 0 && !conversationReferencesAssetExactly(conversationID, asset) {
		return errorsx.Forbidden("legacy message asset is not referenced by the conversation")
	}
	return validateMessageAssetType(asset, messageType)
}

func validateMessageAssetType(asset *models.Asset, messageType enums.IMMessageType) error {
	if asset == nil {
		return errorsx.InvalidParamI18n("error.e0342")
	}
	mimeType := strings.ToLower(strings.TrimSpace(asset.MimeType))
	if messageType == enums.IMMessageTypeImage && !strings.HasPrefix(mimeType, "image/") {
		return errorsx.InvalidParam("image message requires an image asset")
	}
	if messageType == enums.IMMessageTypeAudio && !strings.HasPrefix(mimeType, "audio/") {
		return errorsx.InvalidParam("audio message requires an audio asset")
	}
	return nil
}
