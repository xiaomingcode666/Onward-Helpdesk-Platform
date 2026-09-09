package builders

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/utils"
)

func BuildProductKnowledgeBinding(item *models.ProductKnowledgeBinding) *response.ProductKnowledgeBindingResponse {
	if item == nil {
		return nil
	}
	return &response.ProductKnowledgeBindingResponse{ID: item.ID, TenantID: item.TenantID, ProductID: item.ProductID, ProductModelID: item.ProductModelID, KnowledgeBaseID: item.KnowledgeBaseID, ScopeType: item.ScopeType, Locale: item.Locale, RegionCode: item.RegionCode, SortNo: item.SortNo, Status: int(item.Status), CreatedAt: utils.FormatTime(item.CreatedAt), UpdatedAt: utils.FormatTime(item.UpdatedAt)}
}

func BuildProductKnowledgeBindingList(list []models.ProductKnowledgeBinding) []response.ProductKnowledgeBindingResponse {
	ret := make([]response.ProductKnowledgeBindingResponse, 0, len(list))
	for i := range list {
		ret = append(ret, *BuildProductKnowledgeBinding(&list[i]))
	}
	return ret
}

func BuildResolvedKnowledgeBaseList(list []models.KnowledgeBase) []response.ResolvedKnowledgeBaseResponse {
	ret := make([]response.ResolvedKnowledgeBaseResponse, 0, len(list))
	for _, item := range list {
		ret = append(ret, response.ResolvedKnowledgeBaseResponse{ID: item.ID, Name: item.Name, KnowledgeType: item.KnowledgeType})
	}
	return ret
}
