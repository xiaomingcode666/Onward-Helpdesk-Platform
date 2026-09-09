package response

type ProductServiceProfileResponse struct {
	ID                     int64  `json:"id"`
	TenantID               int64  `json:"tenantId"`
	ProductID              int64  `json:"productId"`
	SupportLocalesJSON     string `json:"supportLocalesJson"`
	SupportRegionsJSON     string `json:"supportRegionsJson"`
	WarrantyPolicyJSON     string `json:"warrantyPolicyJson"`
	SafetyLevel            string `json:"safetyLevel"`
	DefaultFlowTemplateID  int64  `json:"defaultFlowTemplateId"`
	DefaultKnowledgeBaseID int64  `json:"defaultKnowledgeBaseId"`
	DefaultAIAgentID       int64  `json:"defaultAIAgentId"`
	MeetingEnabled         bool   `json:"meetingEnabled"`
	ServicePolicyJSON      string `json:"servicePolicyJson"`
	Status                 int    `json:"status"`
	CreatedAt              string `json:"createdAt"`
	UpdatedAt              string `json:"updatedAt"`
}
