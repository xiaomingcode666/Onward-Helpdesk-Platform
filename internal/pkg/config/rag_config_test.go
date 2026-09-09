package config

import "testing"

func TestRAGConfigNormalizeSetsDefaults(t *testing.T) {
	cfg := (RAGConfig{}).Normalized()
	if cfg.ActiveCollectionAlias != "knowledge_chunks_active" {
		t.Fatalf("ActiveCollectionAlias = %q", cfg.ActiveCollectionAlias)
	}
	if cfg.CollectionPrefix != "knowledge_chunks" {
		t.Fatalf("CollectionPrefix = %q", cfg.CollectionPrefix)
	}
	if cfg.TaskInterval.String() != "5s" {
		t.Fatalf("TaskInterval = %s", cfg.TaskInterval)
	}
	if cfg.ContextMaxTokens != 4000 {
		t.Fatalf("ContextMaxTokens = %d", cfg.ContextMaxTokens)
	}
	if cfg.RetrievalMode != "auto" {
		t.Fatalf("RetrievalMode = %q", cfg.RetrievalMode)
	}
	if cfg.LexicalTopK != 12 {
		t.Fatalf("LexicalTopK = %d", cfg.LexicalTopK)
	}
	if cfg.HybridDenseMinHits != 3 {
		t.Fatalf("HybridDenseMinHits = %d", cfg.HybridDenseMinHits)
	}
	if cfg.RRFK != 60 {
		t.Fatalf("RRFK = %d", cfg.RRFK)
	}
	if cfg.RerankMaxCandidates != 12 {
		t.Fatalf("RerankMaxCandidates = %d", cfg.RerankMaxCandidates)
	}
}

func TestRAGConfigValidateRejectsInvalidValues(t *testing.T) {
	cfg := RAGConfig{
		ActiveCollectionAlias: "",
		CollectionPrefix:      "knowledge_chunks",
		TaskInterval:          -1,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid config error")
	}
}
