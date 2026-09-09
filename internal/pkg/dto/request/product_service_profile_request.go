package request

type CreateProductServiceProfileRequest struct {
	TenantID               int64  `json:"tenantId"`
	ProductID              int64  `json:"productId"`
	SupportLocalesJSON     string `json:"supportLocalesJson"`
	SupportRegionsJSON     string `json:"supportRegionsJson"`
	WarrantyPolicyJSON     string `json:"warrantyPolicyJson"`
	SafetyLevel            string `json:"safetyLevel"`
	DefaultFlowTemplateID  int64  `json:"defaultFlowTemplateId"`
	DefaultKnowledgeBaseID int64  `json:"defaultKnowledgeBaseId"`
	MeetingEnabled         bool   `json:"meetingEnabled"`
	ServicePolicyJSON      string `json:"servicePolicyJson"`
}

type UpdateProductServiceProfileRequest struct {
	ID int64 `json:"id"`
	CreateProductServiceProfileRequest
}

type DeleteProductServiceProfileRequest struct {
	ID int64 `json:"id"`
}

type UpdateProductServiceProfileStatusRequest struct {
	ID     int64 `json:"id"`
	Status int   `json:"status"`
}
