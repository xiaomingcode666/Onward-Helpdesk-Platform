package migration

import (
	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(60, "refresh pending knowledge candidates from latest verified repair", func() error {
		return refreshPendingKnowledgeCandidateSummaries(sqls.DB())
	})
}
