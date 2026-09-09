package tag

import (
	"remotehelpdesk/cmd/testdata/seedlang"
	"remotehelpdesk/cmd/testdata/seeds"
	"regexp"
	"testing"
)

var hanTextPattern = regexp.MustCompile(`\p{Han}`)

func TestEnglishTagSeedsDoNotContainChineseText(t *testing.T) {
	for _, item := range seeds.TagSeeds(seedlang.English) {
		if hanTextPattern.MatchString(item.Name) {
			t.Fatalf("english tag seed contains Chinese text: %q", item.Name)
		}
	}
}
