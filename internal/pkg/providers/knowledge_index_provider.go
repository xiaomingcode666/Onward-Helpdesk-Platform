package providers

import (
	"context"
	"errors"
	"time"
)

// 知识索引 Provider 边界（§8.1）：
// Service 层不允许直接拼接 RAGFlow HTTP 请求，必须通过本接口访问索引后端。
// 首期实现：InternalKnowledgeIndexProvider（本地索引）与 RagflowKnowledgeIndexProvider。

// ErrProviderQueryUnsupported 表示当前 Provider 不支持检索（由调用方回退到原生检索链路）。
var ErrProviderQueryUnsupported = errors.New("knowledge index provider does not support query")

// EnsureDatasetRequest 确保外部 Dataset 存在的请求。
type EnsureDatasetRequest struct {
	TenantID          int64
	Name              string
	ExternalDatasetID string // 已绑定时传入，Provider 校验可用性
	Locale            string
}

// DatasetRef 外部 Dataset 引用。
type DatasetRef struct {
	ProviderType      string `json:"provider_type"`
	ExternalDatasetID string `json:"external_dataset_id"`
}

// UpsertDocumentRequest 上传/更新索引文档。
type UpsertDocumentRequest struct {
	TenantID          int64
	ExternalDatasetID string
	DocumentID        int64  // 内部知识条目 ID（document 或 faq）
	SubjectType       string // knowledge_document / knowledge_faq
	Title             string
	Content           string
	Language          string
	Version           string
}

// IndexDocumentRef 索引文档引用与异步任务信息。
type IndexDocumentRef struct {
	ExternalDocumentID string `json:"external_document_id"`
	ExternalJobID      string `json:"external_job_id"`
	Status             string `json:"status"` // pending/running/indexed/failed
}

// DeleteDocumentRequest 从索引中删除文档。
type DeleteDocumentRequest struct {
	TenantID           int64
	ExternalDatasetID  string
	ExternalDocumentID string
}

// KnowledgeQueryRequest 索引检索请求。
type KnowledgeQueryRequest struct {
	TenantID           int64
	ExternalDatasetIDs []string
	Question           string
	TopK               int
	ScoreThreshold     float64
	Locale             string
}

// KnowledgeQueryHit 单条检索命中。
type KnowledgeQueryHit struct {
	DocumentID string         `json:"document_id"`
	Title      string         `json:"title"`
	Content    string         `json:"content"`
	Score      float64        `json:"score"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// KnowledgeQueryResult 检索结果。
type KnowledgeQueryResult struct {
	Hits []KnowledgeQueryHit `json:"hits"`
}

// IndexJobStatus 外部索引任务状态。
type IndexJobStatus struct {
	JobID        string     `json:"job_id"`
	Status       string     `json:"status"` // pending/running/succeeded/failed
	ErrorSummary string     `json:"error_summary"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
}

// KnowledgeIndexProvider 知识索引后端统一接口。
type KnowledgeIndexProvider interface {
	// ProviderType 返回实现标识：internal / ragflow。
	ProviderType() string
	EnsureDataset(ctx context.Context, req EnsureDatasetRequest) (*DatasetRef, error)
	UpsertDocument(ctx context.Context, req UpsertDocumentRequest) (*IndexDocumentRef, error)
	DeleteDocument(ctx context.Context, req DeleteDocumentRequest) error
	Query(ctx context.Context, req KnowledgeQueryRequest) (*KnowledgeQueryResult, error)
	GetJob(ctx context.Context, jobID string) (*IndexJobStatus, error)
}
