package ai

import (
	"reflect"
	"testing"

	"remotehelpdesk/internal/pkg/providers"
)

func TestNormalizeCapabilityModelNamesKeepsDefaultFirstAndDeduplicates(t *testing.T) {
	actual := normalizeCapabilityModelNames("gpt-5.4-mini", []providers.Sub2APIModel{
		{ID: "gpt-5.6"},
		{ID: "gpt-5.4-mini"},
		{ID: " gpt-5.2 "},
		{ID: "gpt-5.6"},
		{ID: ""},
	})
	want := []string{"gpt-5.4-mini", "gpt-5.2", "gpt-5.6"}
	if !reflect.DeepEqual(actual, want) {
		t.Fatalf("normalizeCapabilityModelNames() = %#v, want %#v", actual, want)
	}
}

func TestSub2APIProviderBaseURLRemovesOpenAICompatibilitySuffix(t *testing.T) {
	if actual := sub2APIProviderBaseURL("https://example.test/compatible-mode/v1/"); actual != "https://example.test/compatible-mode" {
		t.Fatalf("sub2APIProviderBaseURL() = %q", actual)
	}
}
