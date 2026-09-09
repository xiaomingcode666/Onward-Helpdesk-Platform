package rag

import (
	"context"
	"fmt"
	"log/slog"

	"remotehelpdesk/internal/ai/rag/vectordb"
)

func (s *index) ensureCollection(ctx context.Context, provider vectordb.Provider, collectionName string, dimension int) error {
	if dimension <= 0 {
		return fmt.Errorf("invalid embedding dimension: %d", dimension)
	}
	collectionInfo, err := provider.GetCollection(ctx, collectionName)
	if err == nil && collectionInfo != nil {
		if collectionInfo.Dimension != dimension {
			return fmt.Errorf("collection %s dimension mismatch: existing %d, embedding %d", collectionName, collectionInfo.Dimension, dimension)
		}
		return nil
	}
	if err := provider.CreateCollection(ctx, collectionName, dimension); err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}
	collectionInfo, err = provider.GetCollection(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("failed to verify collection %s: %w", collectionName, err)
	}
	if collectionInfo == nil || collectionInfo.Dimension != dimension {
		actualDimension := 0
		if collectionInfo != nil {
			actualDimension = collectionInfo.Dimension
		}
		return fmt.Errorf("collection %s dimension mismatch after creation: actual %d, embedding %d", collectionName, actualDimension, dimension)
	}
	slog.Info("Created collection for knowledge base", "collection", collectionName, "dimension", dimension)
	return nil
}
