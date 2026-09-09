package rag

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/ai/rag/vectordb"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

func (s *index) runDocumentIndex(ctx context.Context, document models.KnowledgeDocument, knowledgeBase models.KnowledgeBase, target IndexTarget) ([]vectordb.Vector, int, error) {
	ctx = ai.WithCapabilityScope(ctx, ai.CapabilityScope{
		TenantID:        document.TenantID,
		KnowledgeBaseID: document.KnowledgeBaseID,
	})
	existingChunks := repositories.KnowledgeChunkRepository.FindByDocumentIDAndGeneration(sqls.DB(), document.ID, target.IndexGenerationID)
	chunks, err := s.buildDocumentChunks(ctx, document, knowledgeBase)
	if err != nil {
		return nil, 0, err
	}

	collectionName := resolveIndexTargetCollectionName(target.CollectionName, s.getCollectionName())
	provider := vectordb.GetProvider()
	if provider == nil {
		return nil, 0, fmt.Errorf("vectordb provider not initialized")
	}
	if _, err := ai.Embedding.GetModel(ctx); err != nil {
		return nil, 0, fmt.Errorf("failed to get embedding model: %w", err)
	}

	vectors, chunkModels, dimension, err := s.prepareDocumentVectors(ctx, knowledgeBase, document, chunks, target)
	if err != nil {
		return nil, 0, err
	}
	if err := s.ensureCollection(ctx, provider, collectionName, dimension); err != nil {
		return nil, 0, err
	}
	if err := provider.UpsertVectors(ctx, collectionName, vectors); err != nil {
		return nil, 0, fmt.Errorf("failed to upsert vectors: %w", err)
	}
	if err := repositories.KnowledgeChunkRepository.ReplaceByDocumentIDAndGeneration(sqls.DB(), document.ID, target.IndexGenerationID, chunkModels); err != nil {
		return nil, 0, fmt.Errorf("failed to save chunks: %w", err)
	}
	if staleVectorIDs := s.collectStaleVectorIDs(existingChunks, vectors); len(staleVectorIDs) > 0 {
		if err := provider.DeleteVectors(ctx, collectionName, staleVectorIDs); err != nil {
			slog.Error("Failed to delete stale document vectors", "document_id", document.ID, "error", err)
		}
	}
	return vectors, len(chunks), nil
}

func (s *index) runFAQIndex(ctx context.Context, faq models.KnowledgeFAQ, knowledgeBase models.KnowledgeBase, target IndexTarget) error {
	ctx = ai.WithCapabilityScope(ctx, ai.CapabilityScope{
		TenantID:        faq.TenantID,
		KnowledgeBaseID: faq.KnowledgeBaseID,
	})
	existingChunks := repositories.KnowledgeChunkRepository.FindByFaqIDAndGeneration(sqls.DB(), faq.ID, target.IndexGenerationID)
	content := buildFAQChunkContent(faq)
	if content == "" {
		return fmt.Errorf("faq content is empty")
	}

	provider := vectordb.GetProvider()
	if provider == nil {
		return fmt.Errorf("vectordb provider not initialized")
	}
	if _, err := ai.Embedding.GetModel(ctx); err != nil {
		return fmt.Errorf("failed to get embedding model: %w", err)
	}
	vector, chunkModel, dimension, err := s.prepareFAQVector(ctx, knowledgeBase, faq, content, target)
	if err != nil {
		return err
	}

	collectionName := resolveIndexTargetCollectionName(target.CollectionName, s.getCollectionName())
	if err := s.ensureCollection(ctx, provider, collectionName, dimension); err != nil {
		return err
	}

	if err := provider.UpsertVectors(ctx, collectionName, []vectordb.Vector{vector}); err != nil {
		return fmt.Errorf("failed to upsert vectors: %w", err)
	}
	if err := repositories.KnowledgeChunkRepository.ReplaceByFaqIDAndGeneration(sqls.DB(), faq.ID, target.IndexGenerationID, &chunkModel); err != nil {
		return fmt.Errorf("failed to save faq chunk: %w", err)
	}
	if staleVectorIDs := s.collectStaleVectorIDs(existingChunks, []vectordb.Vector{vector}); len(staleVectorIDs) > 0 {
		if err := provider.DeleteVectors(ctx, collectionName, staleVectorIDs); err != nil {
			slog.Error("Failed to delete stale faq vectors", "faq_id", faq.ID, "error", err)
		}
	}
	return nil
}

func (s *index) collectStaleVectorIDs(existingChunks []models.KnowledgeChunk, currentVectors []vectordb.Vector) []string {
	currentVectorIDs := make(map[string]struct{}, len(currentVectors))
	for _, vector := range currentVectors {
		if vector.ID == "" {
			continue
		}
		currentVectorIDs[vector.ID] = struct{}{}
	}
	staleVectorIDs := make([]string, 0, len(existingChunks))
	for _, chunk := range existingChunks {
		if chunk.VectorID == "" {
			continue
		}
		if _, ok := currentVectorIDs[chunk.VectorID]; ok {
			continue
		}
		staleVectorIDs = append(staleVectorIDs, chunk.VectorID)
	}
	return staleVectorIDs
}

func resolveIndexTargetCollectionName(target string, fallback string) string {
	if strings.TrimSpace(target) != "" {
		return strings.TrimSpace(target)
	}
	return fallback
}
