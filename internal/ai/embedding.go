package ai

import (
	"context"
	"fmt"
	"math"

	openai "github.com/openai/openai-go/v3"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
)

type EmbeddingResult struct {
	Vector     []float32
	TokensUsed int
	ModelName  string
	Dimension  int
}

type embedding struct{}

var Embedding = &embedding{}

func (s *embedding) GetModel(ctx context.Context) (*models.AIConfig, error) {
	config, err := GetEnabledAIConfigWithContext(ctx, enums.AIModelTypeEmbedding)
	if err != nil {
		return nil, errorsx.BusinessErrorI18n(2001, "error.embeddingModel.noneEnabled")
	}
	return config, nil
}

func (s *embedding) GenerateEmbedding(ctx context.Context, text string) (*EmbeddingResult, error) {
	if text == "" {
		return nil, errorsx.InvalidParamI18n("error.e0215")
	}

	result, err := s.callEmbeddingAPI(ctx, text)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *embedding) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([]EmbeddingResult, error) {
	if len(texts) == 0 {
		return nil, errorsx.InvalidParamI18n("error.e0216")
	}

	results := make([]EmbeddingResult, 0, len(texts))
	for _, text := range texts {
		result, err := s.callEmbeddingAPI(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding for text: %w", err)
		}
		results = append(results, *result)
	}

	return results, nil
}

func (s *embedding) callEmbeddingAPI(ctx context.Context, text string) (*EmbeddingResult, error) {
	config, err := GetEnabledAIConfigWithContext(ctx, enums.AIModelTypeEmbedding)
	if err != nil {
		return nil, err
	}
	client := newOpenAIClient(*config)
	embeddingResp, err := client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(text),
		},
		Model: openai.EmbeddingModel(config.ModelName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to call embedding api: %w", err)
	}

	if len(embeddingResp.Data) == 0 {
		return nil, fmt.Errorf("no embedding data in response")
	}
	vector := make([]float32, 0, len(embeddingResp.Data[0].Embedding))
	for index, item := range embeddingResp.Data[0].Embedding {
		if math.IsNaN(item) || math.IsInf(item, 0) {
			return nil, fmt.Errorf("embedding response contains a non-finite value at index %d", index)
		}
		vector = append(vector, float32(item))
	}
	if len(vector) == 0 {
		return nil, fmt.Errorf("embedding response contains an empty vector")
	}
	if config.Dimension > 0 && len(vector) != config.Dimension {
		return nil, fmt.Errorf("embedding dimension mismatch: model %s returned %d dimensions, configured %d", config.ModelName, len(vector), config.Dimension)
	}

	return &EmbeddingResult{
		Vector:     vector,
		TokensUsed: int(embeddingResp.Usage.TotalTokens),
		ModelName:  embeddingResp.Model,
		Dimension:  len(vector),
	}, nil
}

func (s *embedding) GetDimension(ctx context.Context) (int, error) {
	model, err := s.GetModel(ctx)
	if err != nil {
		return 0, err
	}
	return model.Dimension, nil
}
