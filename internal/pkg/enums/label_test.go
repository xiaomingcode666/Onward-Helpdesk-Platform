package enums

import (
	"reflect"
	"strings"
	"testing"

	"remotehelpdesk/internal/pkg/i18nx"
)

// 枚举名称以资源键保存，每个键在每种语言中都必须有译文，否则页面会显示资源键本身。
func TestEnumLabelKeysExistInEveryLocale(t *testing.T) {
	keys := []string{}
	for _, labels := range []any{
		aiAgentFallbackModeLabelMap, aiAgentHandoffModeLabelMap, aiModelTypeLabelMap, assetProviderLabelMap, assetStatusLabelMap,
		contactTypeLabelMap, externalSourceLabelMap, genderLabelMap, imAssignmentStatusLabelMap, imAssignmentTypeLabelMap,
		imConversationServiceModeLabelMap, imConversationStatusLabelMap, imEventTypeLabelMap, imMessageStatusLabelMap,
		imMessageTypeLabelMap, imParticipantTypeLabelMap, imSenderTypeLabelMap, knowledgeAnswerModeLabelMap,
		knowledgeAnswerStatusLabelMap, knowledgeBaseAccessScopeLabelMap, knowledgeBaseTypeLabelMap, knowledgeChunkProviderLabelMap,
		knowledgeChunkTypeLabelMap, knowledgeFeedbackTypeLabelMap, knowledgeRetrieveChannelLabelMap,
		knowledgeRetrieveSceneLabelMap, serviceStatusLabelMap, statusLabelMap, thirdProviderLabelMap, ticketStatusLabelMap,
	} {
		iter := reflect.ValueOf(labels).MapRange()
		for iter.Next() {
			if value := iter.Value().String(); strings.HasPrefix(value, "enum.") {
				keys = append(keys, value)
			}
		}
	}
	if len(keys) < 100 {
		t.Fatalf("collected %d enum keys, expected all label maps", len(keys))
	}
	for _, key := range keys {
		for _, locale := range i18nx.SupportedLocales {
			if got := enumLabel(locale, key); got == key || got == "" {
				t.Errorf("%s missing in %s", key, locale)
			}
		}
	}
	for _, status := range AIAgentReviewStatusValues {
		if got := GetAIAgentReviewStatusLabel(status, i18nx.LocaleEnUS); got == "" || strings.HasPrefix(got, "enum.") {
			t.Errorf("review status %s label = %q", status, got)
		}
	}
	if got := GetStatusLabel(StatusOk, i18nx.LocaleEnUS); got != "Enabled" {
		t.Fatalf("StatusOk en-US = %q", got)
	}
	if got := GetStatusLabel(StatusOk, i18nx.LocaleZhCN); got != "启用" {
		t.Fatalf("StatusOk zh-CN = %q", got)
	}
}
