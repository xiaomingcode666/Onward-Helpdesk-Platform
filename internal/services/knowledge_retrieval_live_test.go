package services_test

import (
	"context"
	"os"
	"testing"

	"remotehelpdesk/internal/ai/rag/vectordb"
	"remotehelpdesk/internal/bootstrap"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/services"
)

func TestLiveKnowledgeRetrievalThroughUnifiedEmbeddingRoute(t *testing.T) {
	if os.Getenv("RUN_KNOWLEDGE_LIVE_TEST") != "1" {
		t.Skip("set RUN_KNOWLEDGE_LIVE_TEST=1 to query the configured knowledge index")
	}
	cfg, err := config.Load("../../config/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	config.SetCurrent(cfg)
	if _, err := bootstrap.InitDB(cfg.DB); err != nil {
		t.Fatal(err)
	}
	if err := vectordb.Init(&cfg.VectorDB); err != nil {
		t.Fatal(err)
	}

	result, err := services.KnowledgeRetrievalService.Retrieve(context.Background(), services.KnowledgeRetrievalRequest{
		Scope: dto.KnowledgeScopeContext{
			TenantID:  1,
			ProductID: 1,
			Locale:    "en",
			Audience:  "engineer",
		},
		AllowedKnowledgeBaseIDs: []int64{1},
		Query:                   "How do I reset fault code RHD-FLOW-ALPHA-7742?",
		TopK:                    5,
		ScoreThreshold:          0.3,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, hit := range result.RerankedHits {
		if hit.DocumentID == 2 {
			t.Logf("retrieved uploaded document id=2 score=%.4f model=%s", hit.Score, result.EmbeddingModel)
			return
		}
	}
	t.Fatalf("uploaded document id=2 was not retrieved: %+v", result.RerankedHits)
}

func TestLiveKnowledgeRetrievalIsolatesProducts(t *testing.T) {
	if os.Getenv("RUN_KNOWLEDGE_LIVE_TEST") != "1" {
		t.Skip("set RUN_KNOWLEDGE_LIVE_TEST=1 to query the configured knowledge index")
	}
	cfg, err := config.Load("../../config/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	config.SetCurrent(cfg)
	if _, err := bootstrap.InitDB(cfg.DB); err != nil {
		t.Fatal(err)
	}
	if err := vectordb.Init(&cfg.VectorDB); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name              string
		productID         int64
		query             string
		wantKnowledgeID   int64
		forbidKnowledgeID int64
	}{
		{
			name:              "product one cannot retrieve product three rules",
			productID:         1,
			query:             "How do I use MEASURE DISTANCE entity1 entity2?",
			wantKnowledgeID:   1,
			forbidKnowledgeID: 3,
		},
		{
			name:              "product three cannot retrieve product one fault code",
			productID:         3,
			query:             "How do I reset fault code RHD-FLOW-ALPHA-7742?",
			wantKnowledgeID:   3,
			forbidKnowledgeID: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := services.KnowledgeRetrievalService.Retrieve(context.Background(), services.KnowledgeRetrievalRequest{
				Scope: dto.KnowledgeScopeContext{
					TenantID:  1,
					ProductID: tt.productID,
					Locale:    "en",
					Audience:  "customer",
				},
				AllowedKnowledgeBaseIDs: []int64{1, 3},
				Query:                   tt.query,
				TopK:                    12,
				ScoreThreshold:          0.01,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(result.ResolvedScope.KnowledgeBaseIDs) != 1 || result.ResolvedScope.KnowledgeBaseIDs[0] != tt.wantKnowledgeID {
				t.Fatalf("resolved knowledge bases = %v, want [%d]", result.ResolvedScope.KnowledgeBaseIDs, tt.wantKnowledgeID)
			}
			for _, hit := range result.RerankedHits {
				if hit.KnowledgeBaseID == tt.forbidKnowledgeID {
					t.Fatalf("cross-product knowledge leak: product=%d hit=%+v", tt.productID, hit)
				}
			}
		})
	}
}
