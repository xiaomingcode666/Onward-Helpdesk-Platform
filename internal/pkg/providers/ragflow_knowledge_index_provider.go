package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RagflowKnowledgeIndexProvider RAGFlow 索引 Provider。
// 凭证（BaseURL / API Key）通过 AccessConnector 传入，Provider 不持久化凭证（§8.2）。
type RagflowKnowledgeIndexProvider struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewRagflowKnowledgeIndexProvider(baseURL, apiKey string) *RagflowKnowledgeIndexProvider {
	return &RagflowKnowledgeIndexProvider{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *RagflowKnowledgeIndexProvider) ProviderType() string { return "ragflow" }

// ragflowEnvelope RAGFlow API 通用响应包。
type ragflowEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"message"`
	Data json.RawMessage `json:"data"`
}

func (p *RagflowKnowledgeIndexProvider) do(ctx context.Context, method, path string, body io.Reader, contentType string) (*ragflowEnvelope, error) {
	if p.baseURL == "" || p.apiKey == "" {
		return nil, fmt.Errorf("ragflow connector is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	} else if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ragflow api %s %s failed: %s %s", method, path, resp.Status, truncateText(string(raw), 300))
	}
	var envelope ragflowEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("ragflow api invalid response: %w", err)
	}
	if envelope.Code != 0 {
		return nil, fmt.Errorf("ragflow api error code=%d: %s", envelope.Code, envelope.Msg)
	}
	return &envelope, nil
}

// EnsureDataset 创建或校验 RAGFlow Dataset。
func (p *RagflowKnowledgeIndexProvider) EnsureDataset(ctx context.Context, req EnsureDatasetRequest) (*DatasetRef, error) {
	if req.ExternalDatasetID != "" {
		// 已绑定：尝试列出以校验可访问性。
		_, err := p.do(ctx, http.MethodGet, "/api/v1/datasets?id="+url.QueryEscape(req.ExternalDatasetID), nil, "")
		if err != nil {
			return nil, err
		}
		return &DatasetRef{ProviderType: "ragflow", ExternalDatasetID: req.ExternalDatasetID}, nil
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("dataset name is required")
	}
	payload, _ := json.Marshal(map[string]any{"name": name})
	envelope, err := p.do(ctx, http.MethodPost, "/api/v1/datasets", bytes.NewReader(payload), "")
	if err != nil {
		return nil, err
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(envelope.Data, &created); err != nil || created.ID == "" {
		return nil, fmt.Errorf("ragflow dataset create returned no id")
	}
	return &DatasetRef{ProviderType: "ragflow", ExternalDatasetID: created.ID}, nil
}

// UpsertDocument 上传文档并触发解析。内容以 Markdown 文件形式上传。
func (p *RagflowKnowledgeIndexProvider) UpsertDocument(ctx context.Context, req UpsertDocumentRequest) (*IndexDocumentRef, error) {
	if req.ExternalDatasetID == "" {
		return nil, fmt.Errorf("external dataset id is required")
	}
	fileName := fmt.Sprintf("%s-%d.md", req.SubjectType, req.DocumentID)
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, err
	}
	content := req.Content
	if strings.TrimSpace(content) == "" {
		content = req.Title
	}
	if _, err := io.WriteString(part, content); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	envelope, err := p.do(ctx, http.MethodPost, "/api/v1/datasets/"+url.PathEscape(req.ExternalDatasetID)+"/documents", &buf, writer.FormDataContentType())
	if err != nil {
		return nil, err
	}
	var docs []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(envelope.Data, &docs); err != nil || len(docs) == 0 || docs[0].ID == "" {
		return nil, fmt.Errorf("ragflow document upload returned no id")
	}
	docID := docs[0].ID
	// 触发解析（异步任务）。
	parsePayload, _ := json.Marshal(map[string]any{"document_ids": []string{docID}})
	if _, err := p.do(ctx, http.MethodPost, "/api/v1/datasets/"+url.PathEscape(req.ExternalDatasetID)+"/chunks", bytes.NewReader(parsePayload), ""); err != nil {
		return &IndexDocumentRef{ExternalDocumentID: docID, Status: "pending"}, err
	}
	return &IndexDocumentRef{ExternalDocumentID: docID, Status: "running", ExternalJobID: docID}, nil
}

// DeleteDocument 删除 RAGFlow 文档。
func (p *RagflowKnowledgeIndexProvider) DeleteDocument(ctx context.Context, req DeleteDocumentRequest) error {
	if req.ExternalDatasetID == "" || req.ExternalDocumentID == "" {
		return nil
	}
	payload, _ := json.Marshal(map[string]any{"ids": []string{req.ExternalDocumentID}})
	_, err := p.do(ctx, http.MethodDelete, "/api/v1/datasets/"+url.PathEscape(req.ExternalDatasetID)+"/documents", bytes.NewReader(payload), "")
	return err
}

// Query 调用 RAGFlow 检索接口。
func (p *RagflowKnowledgeIndexProvider) Query(ctx context.Context, req KnowledgeQueryRequest) (*KnowledgeQueryResult, error) {
	if len(req.ExternalDatasetIDs) == 0 {
		return &KnowledgeQueryResult{Hits: []KnowledgeQueryHit{}}, nil
	}
	topK := req.TopK
	if topK <= 0 {
		topK = 5
	}
	payload, _ := json.Marshal(map[string]any{
		"question":             req.Question,
		"dataset_ids":          req.ExternalDatasetIDs,
		"top_k":                topK,
		"similarity_threshold": req.ScoreThreshold,
	})
	envelope, err := p.do(ctx, http.MethodPost, "/api/v1/retrieval", bytes.NewReader(payload), "")
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Chunks []struct {
			DocumentID   string  `json:"document_id"`
			DocumentName string  `json:"document_name"`
			Content      string  `json:"content"`
			Score        float64 `json:"similarity"`
		} `json:"chunks"`
	}
	if err := json.Unmarshal(envelope.Data, &parsed); err != nil {
		return nil, fmt.Errorf("ragflow retrieval invalid response: %w", err)
	}
	hits := make([]KnowledgeQueryHit, 0, len(parsed.Chunks))
	for _, chunk := range parsed.Chunks {
		hits = append(hits, KnowledgeQueryHit{
			DocumentID: chunk.DocumentID,
			Title:      chunk.DocumentName,
			Content:    chunk.Content,
			Score:      chunk.Score,
		})
	}
	return &KnowledgeQueryResult{Hits: hits}, nil
}

// GetJob 查询文档解析任务状态（以文档状态近似任务状态）。
func (p *RagflowKnowledgeIndexProvider) GetJob(ctx context.Context, jobID string) (*IndexJobStatus, error) {
	if jobID == "" {
		return nil, fmt.Errorf("job id is required")
	}
	// jobID 采用 "<datasetID>:<documentID>" 形式。
	datasetID, docID, found := strings.Cut(jobID, ":")
	if !found {
		return &IndexJobStatus{JobID: jobID, Status: "succeeded"}, nil
	}
	envelope, err := p.do(ctx, http.MethodGet, "/api/v1/datasets/"+url.PathEscape(datasetID)+"/documents?id="+url.QueryEscape(docID), nil, "")
	if err != nil {
		return nil, err
	}
	var docs []struct {
		RunStatus string `json:"run"` // UNSTART/RUNNING/DONE/FAIL
	}
	if err := json.Unmarshal(envelope.Data, &docs); err != nil || len(docs) == 0 {
		return &IndexJobStatus{JobID: jobID, Status: "pending"}, nil
	}
	status := "pending"
	switch strings.ToUpper(docs[0].RunStatus) {
	case "DONE":
		status = "succeeded"
	case "FAIL":
		status = "failed"
	case "RUNNING":
		status = "running"
	}
	result := &IndexJobStatus{JobID: jobID, Status: status}
	if status == "succeeded" || status == "failed" {
		now := time.Now()
		result.FinishedAt = &now
	}
	return result, nil
}

func truncateText(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
