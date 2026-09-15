package migration

import (
	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/models"
)

func init() {
	register(136, "add ticket classification, priority assessments and relations", func() error {
		// Historical classification, assessments and SLA keys are not invented or rewritten.
		return sqls.DB().AutoMigrate(&models.Ticket{}, &models.TicketGovernanceOperation{}, &models.TicketPriorityProposal{}, &models.TicketRelation{})
	})
}
