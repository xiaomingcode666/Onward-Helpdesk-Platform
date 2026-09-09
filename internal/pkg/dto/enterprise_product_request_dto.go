package dto

// 企业端产品接口共享请求 DTO。
//
// 字段命名与前端 web/lib/api/types.ts 中的 CreateProductPayload /
// UpdateProductPayload 统一为 snake_case，Handler 不得再定义局部请求结构
// （见 docs/design/modules/18-product-center-knowledge-integration-remediation.md §10.1）。

// EnterpriseProductCreateRequest 企业端创建产品请求。
// product_line 为产品线名称，product_line_id 为产品线主数据 ID，二者同时提供时以 ID 优先。
type EnterpriseProductCreateRequest struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Category      string  `json:"category"`
	ProductLine   string  `json:"product_line"`
	ProductLineID int64   `json:"product_line_id"`
	OwnerMemberID int64   `json:"owner_member_id"`
	DefaultLocale string  `json:"default_locale"`
	AIQuotaLimit  float64 `json:"ai_quota_limit"`
}

// EnterpriseProductUpdateRequest 企业端更新产品请求（PATCH 语义）。
// 所有字段均为指针：nil 表示不修改，非 nil 表示按值写入（空字符串可清空对应字段）。
type EnterpriseProductUpdateRequest struct {
	Code          *string `json:"code"`
	Name          *string `json:"name"`
	Description   *string `json:"description"`
	Category      *string `json:"category"`
	ProductLine   *string `json:"product_line"`
	ProductLineID *int64  `json:"product_line_id"`
	OwnerMemberID *int64  `json:"owner_member_id"`
	DefaultLocale *string `json:"default_locale"`
	Status        *string `json:"status"`
}

// EnterpriseProductKnowledgeBaseCreateRequest 企业端按产品创建知识库请求。
// 用于 /products/:id/knowledge-base，创建 document 类型知识库并绑定到产品服务档案。
type EnterpriseProductKnowledgeBaseCreateRequest struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	RagflowDatasetID string   `json:"ragflow_dataset_id"`
	SupportLocales   []string `json:"support_locales"`
	SupportRegions   []string `json:"support_regions"`
}
