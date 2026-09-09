package response

type ProductKnowledgeBindingResponse struct {
	ID              int64  `json:"id"`
	TenantID        int64  `json:"tenantId"`
	ProductID       int64  `json:"productId"`
	ProductModelID  int64  `json:"productModelId"`
	KnowledgeBaseID int64  `json:"knowledgeBaseId"`
	ScopeType       string `json:"scopeType"`
	Locale          string `json:"locale"`
	RegionCode      string `json:"regionCode"`
	SortNo          int    `json:"sortNo"`
	Status          int    `json:"status"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

type ResolvedKnowledgeBaseResponse struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	KnowledgeType string `json:"knowledgeType"`
}
