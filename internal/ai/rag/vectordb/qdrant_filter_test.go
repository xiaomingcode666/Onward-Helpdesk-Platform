package vectordb

import (
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/protobuf/proto"
)

func TestQdrantBuildFilterRequiresTenantAndRevisionScope(t *testing.T) {
	provider := &QdrantProvider{}
	_, err := provider.buildFilter(&SearchFilter{
		EntryKeys:        []string{"document:1"},
		RevisionIDs:      []int64{101},
		ReviewStatus:     "published",
		KnowledgeBaseIDs: []int64{1},
	})
	if err == nil {
		t.Fatal("buildFilter() error = nil, want tenant validation error")
	}
}

func TestQdrantBuildFilterIncludesTenantKeyword(t *testing.T) {
	provider := &QdrantProvider{}
	filter, err := provider.buildFilter(&SearchFilter{
		TenantID:         42,
		EntryKeys:        []string{"document:1"},
		KnowledgeBaseIDs: []int64{9},
		Languages:        []string{"en-US", "default"},
		ReviewStatus:     "published",
		RevisionIDs:      []int64{101},
	})
	if err != nil {
		t.Fatalf("buildFilter() error = %v", err)
	}
	if filter == nil || len(filter.Must) == 0 {
		t.Fatal("buildFilter() returned empty filter")
	}
	foundTenant := false
	for _, cond := range filter.Must {
		field := cond.GetField()
		if field == nil || field.Key != "tenant_id" {
			continue
		}
		foundTenant = true
		break
	}
	if !foundTenant {
		t.Fatal("tenant_id condition missing from filter")
	}
}

func TestQdrantRuntimeFilterRequiresKnowledgeScope(t *testing.T) {
	err := validateRuntimeSearchFilter(&SearchFilter{
		TenantID:     42,
		EntryKeys:    []string{"document:1"},
		ReviewStatus: "published",
		RevisionIDs:  []int64{101},
	})
	if err == nil {
		t.Fatal("validateRuntimeSearchFilter() error = nil, want knowledge scope validation error")
	}
}

func TestQdrantTenantGeneralFilterPreservesIsolation(t *testing.T) {
	for _, allowLegacy := range []bool{false, true} {
		input := &SearchFilter{
			TenantID:          14,
			EntryKeys:         []string{"document:27"},
			KnowledgeBaseIDs:  []int64{16},
			ReviewStatus:      "published",
			RevisionIDs:       []int64{26},
			ScopeKeys:         []string{"tenant:14"},
			AllowLegacyScope:  allowLegacy,
			IndexGenerationID: 1,
		}
		if err := validateRuntimeSearchFilter(input); err != nil {
			t.Fatalf("tenant general runtime filter rejected: %v", err)
		}
		filter, err := (&QdrantProvider{}).buildFilter(input)
		if err != nil {
			t.Fatalf("buildFilter() error = %v", err)
		}
		scopeConditions := []*qdrant.Condition{qdrant.NewMatchKeywords("scope_keys", "tenant:14")}
		if allowLegacy {
			scopeConditions = append(scopeConditions, qdrant.NewIsEmpty("scope_version"))
		}
		want := &qdrant.Filter{Must: []*qdrant.Condition{
			qdrant.NewMatchKeyword("tenant_id", "14"),
			qdrant.NewMatchKeyword("review_status", "published"),
			qdrant.NewMatchKeywords("entry_key", "document:27"),
			qdrant.NewMatchInts("revision_id", 26),
			qdrant.NewFilterAsCondition(&qdrant.Filter{Should: scopeConditions}),
			qdrant.NewMatchInts("knowledge_base_id", 16),
			qdrant.NewMatchInt("index_generation_id", 1),
		}}
		if !proto.Equal(filter, want) {
			t.Fatalf("tenant general filter loses isolation (allowLegacy=%v): got %v, want %v", allowLegacy, filter, want)
		}
	}
}

func TestQdrantBuildFilterIncludesProductAndTenantSharedScope(t *testing.T) {
	provider := &QdrantProvider{}
	filter, err := provider.buildFilter(&SearchFilter{
		TenantID:         42,
		EntryKeys:        []string{"document:1"},
		KnowledgeBaseIDs: []int64{9},
		ReviewStatus:     "published",
		RevisionIDs:      []int64{101},
		ScopeKeys:        []string{"tenant:42", "product:88"},
		AllowLegacyScope: true,
	})
	if err != nil {
		t.Fatalf("buildFilter() error = %v", err)
	}
	foundScope := false
	foundLegacyFallback := false
	for _, condition := range filter.Must {
		nestedFilter := condition.GetFilter()
		if nestedFilter == nil {
			continue
		}
		for _, should := range nestedFilter.Should {
			if field := should.GetField(); field != nil && field.Key == "scope_keys" {
				foundScope = true
			}
			if empty := should.GetIsEmpty(); empty != nil && empty.Key == "scope_version" {
				foundLegacyFallback = true
			}
		}
	}
	if !foundScope {
		t.Fatal("scope_keys condition missing from filter")
	}
	if !foundLegacyFallback {
		t.Fatal("legacy scope compatibility condition missing from filter")
	}
}
