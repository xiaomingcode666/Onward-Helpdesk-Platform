package response

// PlatformSystemPolicyResponse 平台系统级默认策略。
type PlatformSystemPolicyResponse struct {
	CanUpdate   bool                              `json:"canUpdate"`
	Configured  bool                              `json:"configured"`
	UpdatedAt   string                            `json:"updatedAt"`
	TenantQuota PlatformTenantQuotaPolicyResponse `json:"tenantQuota"`
	UploadLimit PlatformUploadLimitPolicyResponse `json:"uploadLimit"`
}

type PlatformTenantQuotaPolicyResponse struct {
	FreeTenantDocumentLimit int64 `json:"freeTenantDocumentLimit"`
	PaidTenantDocumentLimit int64 `json:"paidTenantDocumentLimit"`
}

type PlatformUploadLimitPolicyResponse struct {
	KnowledgeDocumentMaxMB int64 `json:"knowledgeDocumentMaxMB"`
}
