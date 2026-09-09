package dto

// 产品中心管理接口响应/请求 DTO（企业端）。
// Handler 不得返回 GORM 模型或 map[string]any，模型到 DTO 的映射统一在
// internal/builders/enterprise_product_builder.go 完成。
// 见 docs/design/modules/18-product-center-knowledge-integration-remediation.md §10、§11.5。

// EnterpriseProductDetailDTO 产品详情。
type EnterpriseProductDetailDTO struct {
	ID              int64  `json:"id"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	ProductLine     string `json:"product_line"`
	ProductLineID   int64  `json:"product_line_id"`
	Status          string `json:"status"`
	Category        string `json:"category"`
	OwnerMemberID   int64  `json:"owner_member_id"`
	OwnerUserID     int64  `json:"owner_user_id"`
	OwnerName       string `json:"owner_name"`
	DefaultLocale   string `json:"default_locale"`
	Description     string `json:"description"`
	DeviceCount     int64  `json:"device_count"`
	TicketCount     int64  `json:"ticket_count"`
	MeetingEnabled  bool   `json:"meeting_enabled"`
	SafetyLevel     string `json:"safety_level"`
	KnowledgeBaseID int64  `json:"knowledge_base_id"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// EnterpriseProductModelDTO 产品型号主数据。
type EnterpriseProductModelDTO struct {
	ID              int64  `json:"id"`
	TenantID        int64  `json:"tenant_id"`
	ProductID       int64  `json:"product_id"`
	ModelCode       string `json:"model_code"`
	Name            string `json:"name"`
	VersionPolicy   string `json:"version_policy"`
	RegionScopeJSON string `json:"region_scope_json"`
	Status          string `json:"status"`
	DeviceCount     int64  `json:"device_count"`
	TicketCount     int64  `json:"ticket_count"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// EnterpriseProductModuleDTO 产品模块。
type EnterpriseProductModuleDTO struct {
	ID                int64    `json:"id"`
	TenantID          int64    `json:"tenant_id"`
	ProductID         int64    `json:"product_id"`
	ModuleCode        string   `json:"module_code"`
	Name              string   `json:"name"`
	Type              string   `json:"type"`
	DefaultSupplierID int64    `json:"default_supplier_id"`
	IsSafetyCritical  bool     `json:"is_safety_critical"`
	Status            string   `json:"status"`
	ModelIDs          []int64  `json:"model_ids"`
	ModelNames        []string `json:"model_names"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
}

// EnterpriseProductManualDTO 产品手册/知识挂载。
type EnterpriseProductManualDTO struct {
	ID                int64  `json:"id"`
	ProductID         int64  `json:"product_id"`
	ProductModelID    int64  `json:"product_model_id"`
	ProductModelName  string `json:"product_model_name"`
	KnowledgeBaseID   int64  `json:"knowledge_base_id"`
	KnowledgeBaseName string `json:"knowledge_base_name"`
	KnowledgeEntryID  int64  `json:"knowledge_entry_id"`
	EntryReviewStatus string `json:"entry_review_status"`
	Title             string `json:"title"`
	LinkType          string `json:"link_type"`
	Language          string `json:"language"`
	Version           string `json:"version"`
	Visibility        string `json:"visibility"`
	PublishStatus     string `json:"publish_status"`
	SortNo            int    `json:"sort_no"`
	IndexStatus       string `json:"index_status"`
	Status            string `json:"status"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

// EnterpriseProductManualFileDTO 产品手册文件。
type EnterpriseProductManualFileDTO struct {
	ID                  int64  `json:"id"`
	ProductID           int64  `json:"product_id"`
	AssetID             int64  `json:"asset_id"`
	AssetToken          string `json:"asset_token"`
	Title               string `json:"title"`
	Language            string `json:"language"`
	Version             string `json:"version"`
	Visibility          string `json:"visibility"`
	Filename            string `json:"filename"`
	FileSize            int64  `json:"file_size"`
	MimeType            string `json:"mime_type"`
	Provider            string `json:"provider"`
	URL                 string `json:"url"`
	KnowledgeBaseID     int64  `json:"knowledge_base_id"`
	KnowledgeDocumentID int64  `json:"knowledge_document_id"`
	RAGSyncStatus       string `json:"rag_sync_status"`
	RAGSyncError        string `json:"rag_sync_error"`
	SyncedAt            string `json:"synced_at"`
	Status              string `json:"status"`
	UploadedAt          string `json:"uploaded_at"`
	UploadedBy          string `json:"uploaded_by"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
}

// EnterpriseProductKnowledgeDocumentDTO 产品知识库上传文档及索引状态。
type EnterpriseProductKnowledgeDocumentDTO struct {
	ID                int64  `json:"id"`
	KnowledgeEntryID  int64  `json:"knowledge_entry_id"`
	ProductID         int64  `json:"product_id"`
	KnowledgeBaseID   int64  `json:"knowledge_base_id"`
	KnowledgeLinkID   int64  `json:"knowledge_link_id"`
	SourceAssetID     int64  `json:"source_asset_id"`
	SourceType        string `json:"source_type"`
	SourceReferenceID int64  `json:"source_reference_id"`
	Title             string `json:"title"`
	Filename          string `json:"filename"`
	FileSize          int64  `json:"file_size"`
	MimeType          string `json:"mime_type"`
	Provider          string `json:"provider"`
	URL               string `json:"url"`
	IndexStatus       string `json:"index_status"`
	ReviewStatus      string `json:"review_status"`
	Deletable         bool   `json:"deletable"`
	IndexError        string `json:"index_error"`
	IndexedAt         string `json:"indexed_at"`
	UploadedAt        string `json:"uploaded_at"`
	UploadedBy        string `json:"uploaded_by"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

// EnterpriseKnowledgeUploadQuotaDTO 当前账号知识文档上传额度与使用量。
type EnterpriseKnowledgeUploadQuotaDTO struct {
	DocumentLimit            int64 `json:"document_limit"`
	DocumentUsed             int64 `json:"document_used"`
	DocumentRemaining        int64 `json:"document_remaining"`
	TenantDocumentLimit      int64 `json:"tenant_document_limit"`
	TenantDocumentUsed       int64 `json:"tenant_document_used"`
	TenantDocumentRemaining  int64 `json:"tenant_document_remaining"`
	ProductDocumentLimit     int64 `json:"product_document_limit"`
	ProductDocumentUsed      int64 `json:"product_document_used"`
	ProductDocumentRemaining int64 `json:"product_document_remaining"`
	TotalFileSize            int64 `json:"total_file_size"`
	SingleFileSizeLimit      int64 `json:"single_file_size_limit"`
	UnlimitedDocuments       bool  `json:"unlimited_documents"`
}

// EnterpriseProductManualFileMutationRequest 产品手册业务属性。
type EnterpriseProductManualFileMutationRequest struct {
	Title      string `json:"title"`
	Language   string `json:"language"`
	Version    string `json:"version"`
	Visibility string `json:"visibility"`
}

// EnterpriseModuleModelsRequest 模块适用型号整体替换请求（PUT 语义）。
type EnterpriseModuleModelsRequest struct {
	ModelIDs []int64 `json:"model_ids"`
}

// EnterpriseProductModelRequest 型号创建/更新请求（snake_case，与前端统一）。
type EnterpriseProductModelRequest struct {
	ModelCode       string `json:"model_code"`
	Name            string `json:"name"`
	VersionPolicy   string `json:"version_policy"`
	RegionScopeJSON string `json:"region_scope_json"`
}

// EnterpriseProductModelUpdateRequest 型号 PATCH 请求。
type EnterpriseProductModelUpdateRequest struct {
	ModelCode       *string `json:"model_code"`
	Name            *string `json:"name"`
	VersionPolicy   *string `json:"version_policy"`
	RegionScopeJSON *string `json:"region_scope_json"`
}

// EnterpriseProductModuleRequest 模块创建/更新请求。
type EnterpriseProductModuleRequest struct {
	ModuleCode        string `json:"module_code"`
	Name              string `json:"name"`
	DefaultSupplierID int64  `json:"default_supplier_id"`
	IsSafetyCritical  bool   `json:"is_safety_critical"`
}

// EnterpriseProductModuleUpdateRequest 模块 PATCH 请求。
type EnterpriseProductModuleUpdateRequest struct {
	ModuleCode        *string `json:"module_code"`
	Name              *string `json:"name"`
	DefaultSupplierID *int64  `json:"default_supplier_id"`
	IsSafetyCritical  *bool   `json:"is_safety_critical"`
}

// EnterpriseManualLinkRequest 手册挂载请求。知识条目必须由条目选择器提交，
// 后端仍会做完整归属校验（§10.4）。
type EnterpriseManualLinkRequest struct {
	KnowledgeBaseID  int64  `json:"knowledge_base_id"`
	KnowledgeEntryID int64  `json:"knowledge_entry_id"`
	ProductModelID   int64  `json:"product_model_id"`
	LinkType         string `json:"link_type"`
	Language         string `json:"language"`
	Version          string `json:"version"`
	Visibility       string `json:"visibility"`
	SortNo           int    `json:"sort_no"`
}

// EnterpriseManualUpdateRequest 手册链接更新请求（PATCH 语义，指针为 nil 表示不修改）。
type EnterpriseManualUpdateRequest struct {
	ProductModelID *int64  `json:"product_model_id"`
	Language       *string `json:"language"`
	Version        *string `json:"version"`
	Visibility     *string `json:"visibility"`
	SortNo         *int    `json:"sort_no"`
}
