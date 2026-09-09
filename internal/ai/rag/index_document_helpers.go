package rag

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	ragchunk "remotehelpdesk/internal/ai/rag/chunk"
	"remotehelpdesk/internal/ai/rag/vectordb"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/ai"
)

func (s *index) buildDocumentChunkRequest(document models.KnowledgeDocument, knowledgeBase models.KnowledgeBase) *ragchunk.ChunkRequest {
	return &ragchunk.ChunkRequest{
		KnowledgeBaseID: document.KnowledgeBaseID,
		DocumentID:      document.ID,
		DocumentTitle:   document.Title,
		ContentType:     document.ContentType,
		Content:         document.Content,
		PlainText:       ExtractPlainText(document.Content, document.ContentType),
		Options: ragchunk.ChunkOptions{
			Provider:       firstNonEmptyString(knowledgeBase.ChunkProvider, s.chunkConfig.Provider),
			TargetTokens:   firstPositiveInt(knowledgeBase.ChunkTargetTokens, s.chunkConfig.TargetTokens),
			MaxTokens:      firstPositiveInt(knowledgeBase.ChunkMaxTokens, s.chunkConfig.MaxTokens),
			OverlapTokens:  firstPositiveInt(knowledgeBase.ChunkOverlapTokens, s.chunkConfig.OverlapTokens),
			EnableFallback: s.chunkConfig.EnableFallback,
		},
	}
}

func (s *index) buildDocumentChunks(ctx context.Context, document models.KnowledgeDocument, knowledgeBase models.KnowledgeBase) ([]ragchunk.ChunkResult, error) {
	chunks, err := s.registry.Chunk(ctx, s.buildDocumentChunkRequest(document, knowledgeBase))
	if err != nil {
		return nil, fmt.Errorf("failed to chunk document: %w", err)
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks generated from document")
	}
	return chunks, nil
}

func (s *index) prepareDocumentVectors(ctx context.Context, knowledgeBase models.KnowledgeBase, document models.KnowledgeDocument, chunks []ragchunk.ChunkResult, target IndexTarget) ([]vectordb.Vector, []models.KnowledgeChunk, int, error) {
	vectors := make([]vectordb.Vector, 0, len(chunks))
	chunkModels := make([]models.KnowledgeChunk, 0, len(chunks))
	dimension := 0
	directoryPath := loadKnowledgeDirectoryPath(document.DirectoryID)
	revisionID := resolveDocumentRevisionID(document)
	entryKey := buildKnowledgeDocumentEntryKey(document.ID)
	entryScope := ResolveKnowledgeEntryProductScope(sqls.DB(), document.TenantID, knowledgeBase.ID, document.ID, "document")

	for i, chunk := range chunks {
		embeddingResult, err := ai.Embedding.GenerateEmbedding(ctx, chunk.Content)
		if err != nil {
			slog.Error("Failed to generate embedding for chunk", "document_id", document.ID, "chunk_index", i, "error", err)
			return nil, nil, 0, fmt.Errorf("failed to generate embedding for chunk %d: %w", i, err)
		}
		if dimension == 0 {
			dimension = embeddingResult.Dimension
		}

		chunkID := buildKnowledgeChunkVectorID(knowledgeBase.ID, document.ID, revisionID, target.IndexGenerationID, chunk.ChunkNo)
		providerName := ""
		if chunk.Metadata != nil {
			if value, ok := chunk.Metadata["provider"].(string); ok {
				providerName = value
			}
		}
		sectionPath := joinKnowledgeSectionPath(directoryPath, chunk.SectionPath)
		now := time.Now()
		chunkModels = append(chunkModels, models.KnowledgeChunk{
			TenantID:          document.TenantID,
			KnowledgeBaseID:   knowledgeBase.ID,
			DocumentID:        document.ID,
			ChunkNo:           chunk.ChunkNo,
			Title:             chunk.Title,
			Content:           chunk.Content,
			ContentHash:       buildChunkContentHash(chunk.Content),
			CharCount:         chunk.CharCount,
			TokenCount:        chunk.TokenCount,
			ChunkType:         string(chunk.ChunkType),
			SectionPath:       sectionPath,
			Provider:          providerName,
			RevisionID:        revisionID,
			Language:          document.Language,
			Visibility:        "internal",
			ReviewStatus:      document.ReviewStatus,
			ParserVersion:     providerName,
			EmbeddingModel:    embeddingResult.ModelName,
			EmbeddingVersion:  embeddingResult.ModelName,
			IndexGenerationID: target.IndexGenerationID,
			VectorID:          chunkID,
			Status:            enums.StatusOk,
			CreatedAt:         now,
			UpdatedAt:         now,
		})

		vectors = append(vectors, vectordb.Vector{
			ID:     chunkID,
			Vector: embeddingResult.Vector,
			Payload: vectordb.ChunkPayload{
				TenantKey:         strconv.FormatInt(document.TenantID, 10),
				KnowledgeBaseID:   knowledgeBase.ID,
				EntryKey:          entryKey,
				EntryType:         "document",
				DocumentID:        document.ID,
				DocumentTitle:     document.Title,
				RevisionID:        revisionID,
				ReviewStatus:      document.ReviewStatus,
				Language:          document.Language,
				Visibility:        "internal",
				ScopeVersion:      1,
				ScopeKeys:         append([]string(nil), entryScope.ScopeKeys...),
				ProductIDs:        append([]int64(nil), entryScope.ProductIDs...),
				ProductModelIDs:   append([]int64(nil), entryScope.ProductModelIDs...),
				ChunkNo:           chunk.ChunkNo,
				ChunkType:         string(chunk.ChunkType),
				SectionPath:       sectionPath,
				Content:           chunk.Content,
				Title:             chunk.Title,
				Provider:          providerName,
				ContentHash:       buildChunkContentHash(chunk.Content),
				ParserVersion:     providerName,
				EmbeddingModel:    embeddingResult.ModelName,
				EmbeddingVersion:  embeddingResult.ModelName,
				IndexGenerationID: target.IndexGenerationID,
			},
		})
	}

	if len(vectors) == 0 {
		return nil, nil, 0, fmt.Errorf("no vectors generated")
	}
	return vectors, chunkModels, dimension, nil
}
