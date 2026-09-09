package ai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestEmbeddingRetriesTransientProviderFailure(t *testing.T) {
	setupCapabilityRouterTestDB(t)
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/embeddings" {
			http.NotFound(writer, request)
			return
		}
		if attempts.Add(1) < 3 {
			http.Error(writer, `{"error":{"message":"temporarily unavailable"}}`, http.StatusServiceUnavailable)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(writer, `{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.25,0.75]}],"model":"text-embedding-v3","usage":{"prompt_tokens":2,"total_tokens":2}}`)
	}))
	t.Cleanup(server.Close)
	setEmbeddingTestEnvironment(t, server.URL, "2")

	result, err := Embedding.GenerateEmbedding(context.Background(), "设备故障")
	if err != nil {
		t.Fatalf("GenerateEmbedding() error = %v", err)
	}
	if attempts.Load() != 3 {
		t.Fatalf("provider attempts = %d, want 3", attempts.Load())
	}
	if result.Dimension != 2 || len(result.Vector) != 2 || result.ModelName != "text-embedding-v3" {
		t.Fatalf("unexpected embedding result: %#v", result)
	}
}

func TestEmbeddingRejectsProviderDimensionMismatch(t *testing.T) {
	setupCapabilityRouterTestDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(writer, `{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2,0.3]}],"model":"text-embedding-v3","usage":{"prompt_tokens":1,"total_tokens":1}}`)
	}))
	t.Cleanup(server.Close)
	setEmbeddingTestEnvironment(t, server.URL, "2")

	result, err := Embedding.GenerateEmbedding(context.Background(), "维度校验")
	if result != nil || err == nil || !strings.Contains(err.Error(), "embedding dimension mismatch") {
		t.Fatalf("GenerateEmbedding() = (%#v, %v), want dimension mismatch", result, err)
	}
}

func setEmbeddingTestEnvironment(t *testing.T, baseURL, dimension string) {
	t.Helper()
	t.Setenv(embeddingSourceEnv, embeddingSourceAliyun)
	t.Setenv(embeddingAPIKeyEnv, "test-key")
	t.Setenv(embeddingBaseURLEnv, baseURL+"/v1")
	t.Setenv(embeddingModelEnv, "text-embedding-v3")
	t.Setenv(embeddingDimEnv, dimension)
}
