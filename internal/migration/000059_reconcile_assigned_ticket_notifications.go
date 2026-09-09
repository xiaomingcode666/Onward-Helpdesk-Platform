package migration

import (
	"fmt"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(59, "reconcile stale unassigned ticket notifications", func() error {
		return reconcileAssignedTicketNotifications(sqls.DB())
	})
}

func reconcileAssignedTicketNotifications(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.Notification{}) || !db.Migrator().HasTable(&models.Ticket{}) {
		return nil
	}
	var notifications []models.Notification
	if err := db.Where(
		"notification_type = ? AND biz_type = ? AND status = ? AND content LIKE ?",
		"ticket_created",
		"ticket",
		enums.StatusOk,
		"%当前未指派处理人，请及时派工。%",
	).Order("id ASC").Find(&notifications).Error; err != nil {
		return err
	}
	for i := range notifications {
		var ticket models.Ticket
		if err := db.Select("id", "tenant_id", "ticket_no", "title", "current_assignee_id").
			First(&ticket, "id = ? AND tenant_id = ?", notifications[i].BizID, notifications[i].TenantID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			return err
		}
		if ticket.CurrentAssigneeID <= 0 {
			continue
		}
		ticketNo := strings.TrimSpace(ticket.TicketNo)
		if ticketNo == "" {
			ticketNo = fmt.Sprintf("#%d", ticket.ID)
		}
		assigneeName := notificationRecipientName(db, ticket.TenantID, ticket.CurrentAssigneeID)
		if assigneeName == "" {
			assigneeName = "当前负责人"
		}
		content := strings.TrimSpace(strings.ReplaceAll(
			notifications[i].Content,
			"当前未指派处理人，请及时派工。",
			fmt.Sprintf("已分配给 %s，请按当前负责人继续处理。", assigneeName),
		))
		if err := db.Model(&models.Notification{}).Where("id = ?", notifications[i].ID).Updates(map[string]any{
			"title":   fmt.Sprintf("工单 %s 已分配", ticketNo),
			"content": content,
			"level":   "info",
		}).Error; err != nil {
			return err
		}
	}
	return nil
}
