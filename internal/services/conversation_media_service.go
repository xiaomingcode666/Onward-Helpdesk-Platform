package services

import (
	"io"
	"net/url"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/openidentity"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"golang.org/x/net/html"
)

var ConversationMediaService = newConversationMediaService()

type conversationMediaService struct{}

func newConversationMediaService() *conversationMediaService {
	return &conversationMediaService{}
}

func (s *conversationMediaService) OpenForOperator(assetID string, operator *dto.AuthPrincipal) (*models.Asset, io.ReadCloser, error) {
	asset, err := s.AuthorizeForOperator(assetID, operator)
	if err != nil {
		return nil, nil, err
	}
	reader, err := AssetService.OpenReader(asset)
	return asset, reader, err
}

func (s *conversationMediaService) AuthorizeForOperator(assetID string, operator *dto.AuthPrincipal) (*models.Asset, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	return s.authorize(assetID, func(conversation *models.Conversation) bool {
		return ConversationService.CanAccessConversation(conversation, operator)
	})
}

func (s *conversationMediaService) OpenForCustomer(assetID string, external *openidentity.ExternalUser) (*models.Asset, io.ReadCloser, error) {
	asset, err := s.AuthorizeForCustomer(assetID, external)
	if err != nil {
		return nil, nil, err
	}
	reader, err := AssetService.OpenReader(asset)
	return asset, reader, err
}

func (s *conversationMediaService) AuthorizeForCustomer(assetID string, external *openidentity.ExternalUser) (*models.Asset, error) {
	if external == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	return s.authorize(assetID, func(conversation *models.Conversation) bool {
		return ConversationService.IsCustomerConversationOwner(conversation, *external)
	})
}

func (s *conversationMediaService) authorize(assetID string, canAccess func(*models.Conversation) bool) (*models.Asset, error) {
	asset := AssetService.GetByAssetID(strings.TrimSpace(assetID))
	if asset == nil {
		return nil, errorsx.InvalidParamI18n("error.e0214")
	}
	if asset.Status != enums.AssetStatusSuccess {
		return nil, errorsx.InvalidParamI18n("error.e0213")
	}

	conversationIDs := make([]int64, 0, 2)
	if asset.ConversationID > 0 {
		conversationIDs = append(conversationIDs, asset.ConversationID)
	} else {
		conversationIDs = exactConversationAssetReferences(asset)
	}
	for _, conversationID := range conversationIDs {
		conversation := ConversationService.Get(conversationID)
		if conversation == nil || conversation.TenantID != asset.TenantID || !canAccess(conversation) {
			continue
		}
		return asset, nil
	}
	return nil, errorsx.Forbidden("conversation media access denied")
}

func exactConversationAssetReferences(asset *models.Asset) []int64 {
	if asset == nil {
		return nil
	}
	candidates := repositories.MessageRepository.FindAssetReferenceCandidates(sqls.DB(), asset.AssetID, asset.StorageKey)
	seen := make(map[int64]struct{}, len(candidates))
	conversationIDs := make([]int64, 0, len(candidates))
	for i := range candidates {
		message := &candidates[i]
		if !messageReferencesAssetExactly(message, asset) {
			continue
		}
		if _, exists := seen[message.ConversationID]; exists {
			continue
		}
		seen[message.ConversationID] = struct{}{}
		conversationIDs = append(conversationIDs, message.ConversationID)
	}
	return conversationIDs
}

func conversationReferencesAssetExactly(conversationID int64, asset *models.Asset) bool {
	if conversationID <= 0 || asset == nil {
		return false
	}
	for _, referencedConversationID := range exactConversationAssetReferences(asset) {
		if referencedConversationID == conversationID {
			return true
		}
	}
	return false
}

func messageReferencesAssetExactly(message *models.Message, asset *models.Asset) bool {
	if message == nil || asset == nil {
		return false
	}
	if payload, err := parseIMMessageAssetPayload(message.Payload); err == nil {
		if payload.AssetID == strings.TrimSpace(asset.AssetID) ||
			payload.StorageKey != "" && payload.StorageKey == strings.TrimSpace(asset.StorageKey) {
			return true
		}
	}
	for _, assetID := range utils.ExtractMessageHTMLAssetIDs(message.Content) {
		if strings.TrimSpace(assetID) == strings.TrimSpace(asset.AssetID) {
			return true
		}
	}
	return htmlReferencesStorageKey(message.Content, asset.StorageKey)
}

func htmlReferencesStorageKey(content, storageKey string) bool {
	storageKey = strings.TrimLeft(strings.TrimSpace(storageKey), "/")
	content = strings.TrimSpace(content)
	if storageKey == "" || content == "" {
		return false
	}
	if content == storageKey || content == "/"+storageKey {
		return true
	}
	document, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return false
	}
	var visit func(*html.Node) bool
	visit = func(node *html.Node) bool {
		if node.Type == html.ElementNode {
			for _, attribute := range node.Attr {
				if attribute.Key != "src" && attribute.Key != "href" {
					continue
				}
				parsed, err := url.Parse(strings.TrimSpace(attribute.Val))
				if err == nil {
					path := strings.TrimLeft(parsed.Path, "/")
					if path == storageKey || strings.HasSuffix(path, "/"+storageKey) {
						return true
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if visit(child) {
				return true
			}
		}
		return false
	}
	return visit(document)
}
