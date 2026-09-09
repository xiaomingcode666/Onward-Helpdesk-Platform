package aiagent

import (
	"remotehelpdesk/cmd/testdata/seedlang"
	"remotehelpdesk/cmd/testdata/seeds"
	"regexp"
	"testing"
)

var hanTextPattern = regexp.MustCompile(`\p{Han}`)

func TestEnglishAIAgentSeedDoesNotContainChineseText(t *testing.T) {
	for _, item := range seeds.AIAgentSeeds(seedlang.English) {
		values := []string{item.Name, item.Description, item.SystemPrompt, item.WelcomeMessage, item.FallbackMessage}
		for _, value := range values {
			if hanTextPattern.MatchString(value) {
				t.Fatalf("english AI agent seed contains Chinese text: %q", value)
			}
		}
	}
}
