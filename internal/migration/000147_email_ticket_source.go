package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
)

// Email ingestion originally reused the manual source. Correct existing rows so
// the ticket shows how it really arrived instead of claiming manual entry.
func init() {
	register(147, "correct email ticket source", func() error {
		return sqls.DB().Model(&models.Ticket{}).
			Where("channel = ? AND source = ?", "email", "manual").
			Update("source", "email").Error
	})
}
