package rag

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/ai/rag/vectordb"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
)

func buildFAQChunkModel(knowledgeBase models.KnowledgeBase, faq models.KnowledgeFAQ, content string, target IndexTarget) (models.KnowledgeChunk, string) {
	chunkID := buildKnowledgeFAQChunkVectorID(knowledgeBase.ID, faq.ID, resolveFAQRevisionID(faq), target.IndexGenerationID, 0)
	now := time.Now()
	sectionPath := loadKnowledgeDirectoryPath(faq.DirectoryID)
	return models.KnowledgeChunk{
		TenantID:          faq.TenantID,
		KnowledgeBaseID:   knowledgeBase.ID,
		FaqID:             faq.ID,
		ChunkNo:           0,
		Title:             faq.Question,
		Content:           content,
		ContentHash:       buildChunkContentHash(content),
		CharCount:         len([]rune(content)),
		TokenCount:        len([]rune(content)) / 2,
		ChunkType:         string(enums.KnowledgeChunkTypeFAQ),
		SectionPath:       sectionPath,
		Provider:          string(enums.KnowledgeChunkProviderFAQ),
		RevisionID:        resolveFAQRevisionID(faq),
		Language:          faq.Language,
		Visibility:        "internal",
		ReviewStatus:      faq.ReviewStatus,
		ParserVersion:     string(enums.KnowledgeChunkProviderFAQ),
		IndexGenerationID: target.IndexGenerationID,
		VectorID:          chunkID,
		Status:            enums.StatusOk,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, chunkID
}

func (s *index) prepareFAQVector(ctx context.Context, knowledgeBase models.KnowledgeBase, faq models.KnowledgeFAQ, content string, target IndexTarget) (vectordb.Vector, models.KnowledgeChunk, int, error) {
	embeddingResult, err := ai.Embedding.GenerateEmbedding(ctx, content)
	if err != nil {
		return vectordb.Vector{}, models.KnowledgeChunk{}, 0, fmt.Errorf("failed to generate embedding for faq %d: %w", faq.ID, err)
	}

	chunkModel, chunkID := buildFAQChunkModel(knowledgeBase, faq, content, target)
	sectionPath := chunkModel.SectionPath
	entryScope := ResolveKnowledgeEntryProductScope(sqls.DB(), faq.TenantID, knowledgeBase.ID, faq.ID, "faq")
	vector := vectordb.Vector{
		ID:     chunkID,
		Vector: embeddingResult.Vector,
		Payload: vectordb.ChunkPayload{
			TenantKey:         strconv.FormatInt(faq.TenantID, 10),
			KnowledgeBaseID:   knowledgeBase.ID,
			EntryKey:          buildKnowledgeFAQEntryKey(faq.ID),
			EntryType:         "faq",
			FaqID:             faq.ID,
			FaqQuestion:       faq.Question,
			RevisionID:        resolveFAQRevisionID(faq),
			ReviewStatus:      faq.ReviewStatus,
			Language:          faq.Language,
			Visibility:        "internal",
			ScopeVersion:      1,
			ScopeKeys:         append([]string(nil), entryScope.ScopeKeys...),
			ProductIDs:        append([]int64(nil), entryScope.ProductIDs...),
			ProductModelIDs:   append([]int64(nil), entryScope.ProductModelIDs...),
			ChunkNo:           0,
			ChunkType:         string(enums.KnowledgeChunkTypeFAQ),
			SectionPath:       sectionPath,
			Content:           content,
			Title:             faq.Question,
			Provider:          string(enums.KnowledgeChunkProviderFAQ),
			ContentHash:       buildChunkContentHash(content),
			ParserVersion:     string(enums.KnowledgeChunkProviderFAQ),
			EmbeddingModel:    embeddingResult.ModelName,
			EmbeddingVersion:  embeddingResult.ModelName,
			IndexGenerationID: target.IndexGenerationID,
		},
	}
	chunkModel.EmbeddingModel = embeddingResult.ModelName
	chunkModel.EmbeddingVersion = embeddingResult.ModelName
	return vector, chunkModel, embeddingResult.Dimension, nil
}
