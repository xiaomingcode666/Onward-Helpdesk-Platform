package request

type CreateProductKnowledgeBindingRequest struct {
	TenantID        int64  `json:"tenantId"`
	ProductID       int64  `json:"productId"`
	ProductModelID  int64  `json:"productModelId"`
	KnowledgeBaseID int64  `json:"knowledgeBaseId"`
	ScopeType       string `json:"scopeType"`
	Locale          string `json:"locale"`
	RegionCode      string `json:"regionCode"`
	SortNo          int    `json:"sortNo"`
}

type UpdateProductKnowledgeBindingRequest struct {
	ID int64 `json:"id"`
	CreateProductKnowledgeBindingRequest
}
type DeleteProductKnowledgeBindingRequest struct {
	ID int64 `json:"id"`
}
type UpdateProductKnowledgeBindingStatusRequest struct {
	ID     int64 `json:"id"`
	Status int   `json:"status"`
}

type ResolveProductKnowledgeBindingRequest struct {
	TenantID       int64  `json:"tenantId"`
	ProductID      int64  `json:"productId"`
	ProductModelID int64  `json:"productModelId"`
	Locale         string `json:"locale"`
	RegionCode     string `json:"regionCode"`
}
