package providers

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// InternalIndexStore 内部索引存储抽象，由 services 层基于 GORM 仓储注入，
// 使 Provider 不直接依赖 repositories（保持 providers 为基础设施层）。
type InternalIndexStore interface {
	// IndexDocument 将知识条目内容写入/刷新内部索引（chunk 或全文标记），返回命中预览文本。
	IndexDocument(ctx context.Context, req UpsertDocumentRequest) error
	// RemoveDocument 从内部索引移除条目。
	RemoveDocument(ctx context.Context, subjectType string, documentID int64) error
	// Search 内部检索（关键词/向量由实现决定）。
	Search(ctx context.Context, req KnowledgeQueryRequest) (*KnowledgeQueryResult, error)
}

// InternalKnowledgeIndexProvider 内部索引 Provider。
// Dataset 概念映射为租户知识库（external_dataset_id = "internal:<knowledgeBaseID>"）。
type InternalKnowledgeIndexProvider struct {
	store InternalIndexStore
}

func NewInternalKnowledgeIndexProvider(store InternalIndexStore) *InternalKnowledgeIndexProvider {
	return &InternalKnowledgeIndexProvider{store: store}
}

func (p *InternalKnowledgeIndexProvider) ProviderType() string { return "internal" }

func (p *InternalKnowledgeIndexProvider) EnsureDataset(ctx context.Context, req EnsureDatasetRequest) (*DatasetRef, error) {
	datasetID := req.ExternalDatasetID
	if datasetID == "" {
		datasetID = fmt.Sprintf("internal:tenant:%d:%s", req.TenantID, strings.TrimSpace(req.Name))
	}
	return &DatasetRef{ProviderType: "internal", ExternalDatasetID: datasetID}, nil
}

func (p *InternalKnowledgeIndexProvider) UpsertDocument(ctx context.Context, req UpsertDocumentRequest) (*IndexDocumentRef, error) {
	if p.store == nil {
		return &IndexDocumentRef{Status: "indexed"}, nil
	}
	if err := p.store.IndexDocument(ctx, req); err != nil {
		return nil, err
	}
	now := time.Now().Format(time.RFC3339)
	return &IndexDocumentRef{
		ExternalDocumentID: fmt.Sprintf("%s:%d", req.SubjectType, req.DocumentID),
		Status:             "indexed",
		ExternalJobID:      "internal-" + now,
	}, nil
}

func (p *InternalKnowledgeIndexProvider) DeleteDocument(ctx context.Context, req DeleteDocumentRequest) error {
	if p.store == nil {
		return nil
	}
	subjectType, idStr, found := strings.Cut(req.ExternalDocumentID, ":")
	if !found {
		return nil
	}
	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil || id <= 0 {
		return nil
	}
	return p.store.RemoveDocument(ctx, subjectType, id)
}

func (p *InternalKnowledgeIndexProvider) Query(ctx context.Context, req KnowledgeQueryRequest) (*KnowledgeQueryResult, error) {
	if p.store == nil {
		return nil, ErrProviderQueryUnsupported
	}
	return p.store.Search(ctx, req)
}

func (p *InternalKnowledgeIndexProvider) GetJob(ctx context.Context, jobID string) (*IndexJobStatus, error) {
	// 内部索引为同步执行，任务即完成。
	now := time.Now()
	return &IndexJobStatus{JobID: jobID, Status: "succeeded", FinishedAt: &now}, nil
}
