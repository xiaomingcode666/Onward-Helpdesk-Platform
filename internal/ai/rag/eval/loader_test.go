package eval

import (
	"path/filepath"
	"testing"
)

func TestLoadSuite(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "rag-eval", "customer_robot_seed.yaml")
	suite, err := LoadSuite(path)
	if err != nil {
		t.Fatalf("LoadSuite() error = %v", err)
	}

	if suite.Name != "customer_robot_seed" {
		t.Fatalf("suite.Name = %q, want customer_robot_seed", suite.Name)
	}
	if len(suite.Cases) != 20 {
		t.Fatalf("len(suite.Cases) = %d, want 20", len(suite.Cases))
	}

	normalized := suite.NormalizedCases()
	if normalized[0].Enabled == nil || !*normalized[0].Enabled {
		t.Fatalf("first case enabled default not applied")
	}
	if normalized[0].Scope.Locale != "en-US" {
		t.Fatalf("normalized locale = %q, want en-US", normalized[0].Scope.Locale)
	}
	if normalized[15].AnswerExpectation.MinCitationCount != 2 {
		t.Fatalf("case 16 minCitationCount = %d, want 2", normalized[15].AnswerExpectation.MinCitationCount)
	}
}

func TestLoadRefsAndResolveEntryKeys(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "rag-eval", "refs.local.example.yaml")
	refs, err := LoadRefs(path)
	if err != nil {
		t.Fatalf("LoadRefs() error = %v", err)
	}

	keys, err := refs.ResolveEntryKeys([]string{
		"manual.startup_precheck",
		"faq.alarm_p101_reset",
	})
	if err != nil {
		t.Fatalf("ResolveEntryKeys() error = %v", err)
	}

	if len(keys) != 2 {
		t.Fatalf("len(keys) = %d, want 2", len(keys))
	}
	if keys[0] != "document:101" {
		t.Fatalf("keys[0] = %q, want document:101", keys[0])
	}
	if keys[1] != "faq:201" {
		t.Fatalf("keys[1] = %q, want faq:201", keys[1])
	}
}
