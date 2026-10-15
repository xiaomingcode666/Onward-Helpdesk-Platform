package builtin

import (
	"encoding/json"
	"testing"

	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/pkg/i18nx"

	"github.com/stretchr/testify/require"
)

func TestEveryBuiltinNodeAndBranchHasLocalizedNames(t *testing.T) {
	items, err := Load()
	require.NoError(t, err)
	for _, item := range items {
		for _, node := range item.Definition.Nodes {
			key := NodeNameKey(item.Code, node.ID)
			require.True(t, i18nx.HasKey(key), key)
			require.Equal(t, node.Name, i18nx.TLocale(i18nx.LocaleZhCN, key), key)
			for _, locale := range i18nx.SupportedLocales {
				require.NotEqual(t, key, i18nx.TLocale(locale, key), "%s missing in %s", key, locale)
			}
			for _, branch := range definitionBranches(t, node) {
				if branch.Name == "" {
					continue
				}
				branchKey := BranchNameKey(item.Code, node.ID, branch.ID)
				require.True(t, i18nx.HasKey(branchKey), branchKey)
				require.Equal(t, branch.Name, i18nx.TLocale(i18nx.LocaleZhCN, branchKey), branchKey)
			}
		}
	}
}

func TestLocalizeDefinitionRendersBuiltinNamesOnly(t *testing.T) {
	manifest, ok, err := Get("aftersales_customer_service_default")
	require.NoError(t, err)
	require.True(t, ok)
	definition := manifest.Definition
	original := definition.Nodes[0].Name

	renamed := definition
	renamed.Nodes = append([]dsl.Node(nil), definition.Nodes...)
	renamed.Nodes[1].Name = "Custom intake step"

	localized := LocalizeDefinition(renamed, i18nx.LocaleEnUS)
	require.Equal(t, "Customer starts conversation", localized.Nodes[0].Name)
	require.Equal(t, "Custom intake step", localized.Nodes[1].Name)
	require.Equal(t, original, definition.Nodes[0].Name, "input definition must not be modified")

	route := findNode(t, localized, "service_access_route_1")
	names := map[string]string{}
	for _, branch := range definitionBranches(t, route) {
		names[branch.ID] = branch.Name
	}
	require.Equal(t, "Human support only", names["human_only"])

	// Names saved from another locale are still recognized as built-in names.
	back := LocalizeDefinition(localized, i18nx.LocaleZhCN)
	require.Equal(t, original, back.Nodes[0].Name)
}

func TestLocalizeNodeNameKeepsUnknownNodes(t *testing.T) {
	require.Equal(t, "Customer starts conversation", LocalizeNodeName("start_1", i18nx.TLocale(i18nx.LocaleZhCN, NodeNameKey("aftersales_customer_service_default", "start_1")), i18nx.LocaleEnUS))
	require.Equal(t, "My node", LocalizeNodeName("start_1", "My node", i18nx.LocaleEnUS))
	require.Equal(t, "", LocalizeNodeName("start_1", "", i18nx.LocaleEnUS))
}

func definitionBranches(t *testing.T, node dsl.Node) []dsl.ConditionBranch {
	t.Helper()
	if len(node.Config) == 0 {
		return nil
	}
	var config dsl.ConditionConfig
	require.NoError(t, json.Unmarshal(node.Config, &config))
	return config.Branches
}

func findNode(t *testing.T, definition dsl.Definition, id string) dsl.Node {
	t.Helper()
	for _, node := range definition.Nodes {
		if node.ID == id {
			return node
		}
	}
	t.Fatalf("node %s not found", id)
	return dsl.Node{}
}

func TestCanonicalizeDefinitionRestoresManifestNames(t *testing.T) {
	manifest, ok, err := Get("knowledge_customer_service_default")
	require.NoError(t, err)
	require.True(t, ok)
	english := LocalizeDefinition(manifest.Definition, i18nx.LocaleEnUS)
	require.NotEqual(t, manifest.Definition.Nodes[0].Name, english.Nodes[0].Name)

	canonical := CanonicalizeDefinition(english)
	left, err := json.Marshal(canonical)
	require.NoError(t, err)
	right, err := json.Marshal(manifest.Definition)
	require.NoError(t, err)
	require.JSONEq(t, string(right), string(left))
}
