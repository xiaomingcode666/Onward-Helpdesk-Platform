package migration

import (
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(36, "repair pending ticket knowledge candidate titles", func() error {
		return repairPendingTicketKnowledgeCandidateTitles(sqls.DB())
	})
}

func repairPendingTicketKnowledgeCandidateTitles(db *gorm.DB) error {
	var candidates []models.KnowledgeCandidate
	if err := db.Where("source_type = ? AND review_status = ? AND status <> ?", "ticket_repair", "pending", enums.StatusDeleted).
		Find(&candidates).Error; err != nil {
		return err
	}
	for i := range candidates {
		var ticket models.Ticket
		if err := db.First(&ticket, candidates[i].TicketID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			return err
		}
		faultType := strings.TrimSpace(ticket.FaultCode)
		currentTitle := strings.TrimSpace(candidates[i].Title)
		if faultType == "" || (currentTitle != "" && currentTitle != strings.TrimSpace(ticket.Title)) {
			continue
		}
		if err := db.Model(&models.KnowledgeCandidate{}).Where("id = ?", candidates[i].ID).Updates(map[string]any{
			"title":            faultType,
			"updated_at":       time.Now(),
			"update_user_name": "system",
		}).Error; err != nil {
			return err
		}
	}
	return nil
}
