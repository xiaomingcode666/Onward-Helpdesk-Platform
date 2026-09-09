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
	register(51, "refresh generic ticket knowledge candidate titles", func() error {
		return refreshGenericKnowledgeCandidateTitles(sqls.DB())
	})
}

func refreshGenericKnowledgeCandidateTitles(db *gorm.DB) error {
	if db == nil ||
		!db.Migrator().HasTable(&models.KnowledgeCandidate{}) ||
		!db.Migrator().HasTable(&models.Ticket{}) ||
		!db.Migrator().HasTable(&models.TicketRepairRecord{}) {
		return nil
	}
	var candidates []models.KnowledgeCandidate
	if err := db.Where("status <> ? AND knowledge_entry_id = 0 AND review_status = ?", enums.StatusDeleted, "pending").
		Order("id ASC").
		Find(&candidates).Error; err != nil {
		return err
	}
	now := time.Now()
	for i := range candidates {
		candidate := candidates[i]
		ticket := &models.Ticket{}
		if err := db.First(ticket, "id = ? AND tenant_id = ?", candidate.TicketID, candidate.TenantID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			return err
		}
		if !isGenericKnowledgeCandidateTitle(candidate.Title, ticket) {
			continue
		}
		var repairs []models.TicketRepairRecord
		if err := db.Where("tenant_id = ? AND ticket_id = ?", candidate.TenantID, candidate.TicketID).
			Order("id ASC").
			Find(&repairs).Error; err != nil {
			return err
		}
		title := migrationKnowledgeCandidateTitle(ticket, repairs)
		if title == "" || title == strings.TrimSpace(candidate.Title) {
			continue
		}
		if err := db.Model(&models.KnowledgeCandidate{}).Where("id = ?", candidate.ID).Updates(map[string]any{
			"title":            title,
			"updated_at":       now,
			"update_user_name": "migration-51",
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func isGenericKnowledgeCandidateTitle(title string, ticket *models.Ticket) bool {
	title = strings.TrimSpace(title)
	if title == "" || title == "维修知识候选" {
		return true
	}
	if ticket == nil {
		return false
	}
	faultCode := strings.TrimSpace(ticket.FaultCode)
	ticketTitle := strings.TrimSpace(ticket.Title)
	return (faultCode != "" && strings.EqualFold(title, faultCode)) ||
		(ticketTitle != "" && strings.EqualFold(title, ticketTitle))
}

func migrationKnowledgeCandidateTitle(ticket *models.Ticket, repairs []models.TicketRepairRecord) string {
	faultCode := ""
	ticketTitle := ""
	if ticket != nil {
		faultCode = migrationCompactKnowledgeCandidateTitle(ticket.FaultCode)
		ticketTitle = migrationCompactKnowledgeCandidateTitle(ticket.Title)
	}
	detail := migrationCompactKnowledgeCandidateTitle(migrationLatestRepairDigest(repairs))
	if faultCode != "" {
		if detail != "" {
			return migrationTrimKnowledgeCandidateTitle(strings.TrimSpace(faultCode + " " + strings.TrimPrefix(detail, faultCode)))
		}
		if ticketTitle != "" && !strings.EqualFold(ticketTitle, faultCode) {
			return migrationTrimKnowledgeCandidateTitle(strings.TrimSpace(faultCode + " " + strings.TrimPrefix(ticketTitle, faultCode)))
		}
		return faultCode
	}
	if ticketTitle != "" {
		if detail != "" && !strings.Contains(ticketTitle, detail) {
			return migrationTrimKnowledgeCandidateTitle(ticketTitle + " " + detail)
		}
		return ticketTitle
	}
	if detail != "" {
		return detail
	}
	return "维修知识候选"
}

func migrationLatestRepairDigest(repairs []models.TicketRepairRecord) string {
	for i := len(repairs) - 1; i >= 0; i-- {
		repair := repairs[i]
		for _, value := range []string{repair.RootCause, repair.Solution, repair.Conclusion} {
			if strings.TrimSpace(value) != "" {
				return value
			}
		}
	}
	return ""
}

func migrationCompactKnowledgeCandidateTitle(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer("\r", " ", "\n", " ", "\t", " ", "；", "，", ";", "，", "。", "，")
	value = replacer.Replace(value)
	if fields := strings.Fields(value); len(fields) > 0 {
		value = strings.Join(fields, " ")
	}
	if idx := strings.Index(value, "，"); idx > 0 {
		value = value[:idx]
	}
	return migrationTrimKnowledgeCandidateTitle(strings.TrimSpace(value))
}

func migrationTrimKnowledgeCandidateTitle(value string) string {
	const maxRunes = 56
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:maxRunes]))
}
