package request

// PlatformSystemPolicyUpdateRequest 平台系统级默认策略更新请求。
// 0 表示不设置平台默认上限，由套餐/租户覆盖或现有默认策略决定。
type PlatformSystemPolicyUpdateRequest struct {
	FreeTenantDocumentLimit int64 `json:"freeTenantDocumentLimit"`
	PaidTenantDocumentLimit int64 `json:"paidTenantDocumentLimit"`
	KnowledgeDocumentMaxMB  int64 `json:"knowledgeDocumentMaxMB"`
}
