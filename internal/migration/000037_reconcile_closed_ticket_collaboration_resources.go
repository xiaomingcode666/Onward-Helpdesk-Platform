package migration

import (
	"strconv"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(37, "reconcile closed ticket collaboration resources", func() error {
		return reconcileClosedTicketCollaborationResources(sqls.DB())
	})
}

func reconcileClosedTicketCollaborationResources(db *gorm.DB) error {
	var tickets []models.Ticket
	if err := db.Where("status IN ?", []enums.TicketStatus{enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled}).Find(&tickets).Error; err != nil {
		return err
	}
	for i := range tickets {
		closedAt := tickets[i].UpdatedAt
		if tickets[i].HandledAt != nil {
			closedAt = *tickets[i].HandledAt
		}
		if closedAt.IsZero() {
			closedAt = time.Now()
		}
		if _, err := repositories.MeetingRoomRepository.EndActiveJitsiByTicket(db, tickets[i].TenantID, strconv.FormatInt(tickets[i].ID, 10), closedAt); err != nil {
			return err
		}
		if err := repositories.TicketSupplierCollaborationRepository.ResolveActiveByTicket(db, tickets[i].TenantID, tickets[i].ID, "工单已关闭，系统自动结束供应商协作", closedAt, 0, "system"); err != nil {
			return err
		}
		if err := repositories.TicketSupplierCollaborationRepository.DisableAllTicketAuthorizationScopes(db, tickets[i].TenantID, tickets[i].ID, closedAt); err != nil {
			return err
		}
	}
	return nil
}
