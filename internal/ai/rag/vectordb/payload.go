package vectordb

import (
	"github.com/mlogclub/simple/common/structs"
	"github.com/spf13/cast"
)

type ChunkPayload struct {
	TenantKey         string   `json:"tenant_id"`
	KnowledgeBaseID   int64    `json:"knowledge_base_id"`
	EntryKey          string   `json:"entry_key"`
	EntryType         string   `json:"entry_type"`
	DocumentID        int64    `json:"document_id"`
	DocumentTitle     string   `json:"document_title"`
	FaqID             int64    `json:"faq_id"`
	FaqQuestion       string   `json:"faq_question"`
	RevisionID        int64    `json:"revision_id"`
	ReviewStatus      string   `json:"review_status"`
	Language          string   `json:"language"`
	Visibility        string   `json:"visibility"`
	ScopeVersion      int      `json:"scope_version"`
	ScopeKeys         []string `json:"scope_keys"`
	ProductIDs        []int64  `json:"product_ids"`
	ProductModelIDs   []int64  `json:"product_model_ids"`
	FaultCodes        []string `json:"fault_codes"`
	ChunkNo           int      `json:"chunk_no"`
	ChunkType         string   `json:"chunk_type"`
	SectionPath       string   `json:"section_path"`
	Title             string   `json:"title"`
	Content           string   `json:"content"`
	Provider          string   `json:"provider"`
	ContentHash       string   `json:"content_hash"`
	ParserVersion     string   `json:"parser_version"`
	EmbeddingModel    string   `json:"embedding_model"`
	EmbeddingVersion  string   `json:"embedding_version"`
	IndexGenerationID int64    `json:"index_generation_id"`
}

func (p ChunkPayload) ToMap() map[string]any {
	return structs.StructToMap(p)
}

func ChunkPayloadFromMap(data map[string]any) ChunkPayload {
	if data == nil {
		return ChunkPayload{}
	}
	return ChunkPayload{
		TenantKey:         cast.ToString(data["tenant_id"]),
		KnowledgeBaseID:   cast.ToInt64(data["knowledge_base_id"]),
		EntryKey:          cast.ToString(data["entry_key"]),
		EntryType:         cast.ToString(data["entry_type"]),
		DocumentID:        cast.ToInt64(data["document_id"]),
		DocumentTitle:     cast.ToString(data["document_title"]),
		FaqID:             cast.ToInt64(data["faq_id"]),
		FaqQuestion:       cast.ToString(data["faq_question"]),
		RevisionID:        cast.ToInt64(data["revision_id"]),
		ReviewStatus:      cast.ToString(data["review_status"]),
		Language:          cast.ToString(data["language"]),
		Visibility:        cast.ToString(data["visibility"]),
		ScopeVersion:      cast.ToInt(data["scope_version"]),
		ScopeKeys:         toStringSlice(data["scope_keys"]),
		ProductIDs:        toInt64Slice(data["product_ids"]),
		ProductModelIDs:   toInt64Slice(data["product_model_ids"]),
		FaultCodes:        toStringSlice(data["fault_codes"]),
		ChunkNo:           cast.ToInt(data["chunk_no"]),
		ChunkType:         cast.ToString(data["chunk_type"]),
		SectionPath:       cast.ToString(data["section_path"]),
		Title:             cast.ToString(data["title"]),
		Content:           cast.ToString(data["content"]),
		Provider:          cast.ToString(data["provider"]),
		ContentHash:       cast.ToString(data["content_hash"]),
		ParserVersion:     cast.ToString(data["parser_version"]),
		EmbeddingModel:    cast.ToString(data["embedding_model"]),
		EmbeddingVersion:  cast.ToString(data["embedding_version"]),
		IndexGenerationID: cast.ToInt64(data["index_generation_id"]),
	}
}

func toInt64Slice(value any) []int64 {
	raw := cast.ToSlice(value)
	if len(raw) == 0 {
		return nil
	}
	items := make([]int64, 0, len(raw))
	for _, item := range raw {
		items = append(items, cast.ToInt64(item))
	}
	return items
}

func toStringSlice(value any) []string {
	raw := cast.ToSlice(value)
	if len(raw) == 0 {
		return nil
	}
	items := make([]string, 0, len(raw))
	for _, item := range raw {
		items = append(items, cast.ToString(item))
	}
	return items
}
