package migration

import "github.com/mlogclub/simple/sqls"

func init() {
	register(91, "repair meeting transcript offsets using database timezone", func() error {
		return normalizeMeetingTranscriptOffsets(sqls.DB())
	})
}
