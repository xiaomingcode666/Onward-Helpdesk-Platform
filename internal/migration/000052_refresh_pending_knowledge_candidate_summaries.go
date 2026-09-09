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
	register(52, "refresh pending ticket knowledge candidate summaries", func() error {
		return refreshPendingKnowledgeCandidateSummaries(sqls.DB())
	})
}

func refreshPendingKnowledgeCandidateSummaries(db *gorm.DB) error {
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
		var repairs []models.TicketRepairRecord
		if err := db.Where("tenant_id = ? AND ticket_id = ?", candidate.TenantID, candidate.TicketID).
			Order("id ASC").
			Find(&repairs).Error; err != nil {
			return err
		}
		if len(repairs) == 0 {
			continue
		}
		title := candidate.Title
		if isGenericKnowledgeCandidateTitle(candidate.Title, ticket) {
			title = migrationKnowledgeCandidateTitle(ticket, repairs)
		}
		rootCauseSummary := migrationRootCauseSummary(repairs)
		solutionSummary := migrationSolutionSummary(repairs)
		updates := map[string]any{}
		if title != "" && title != candidate.Title {
			updates["title"] = title
		}
		if rootCauseSummary != "" && rootCauseSummary != candidate.RootCauseSummary {
			updates["root_cause_summary"] = rootCauseSummary
		}
		if solutionSummary != "" && solutionSummary != candidate.SolutionSummary {
			updates["solution_summary"] = solutionSummary
		}
		if len(updates) == 0 {
			continue
		}
		updates["updated_at"] = now
		updates["update_user_name"] = "knowledge-candidate-refresh"
		if err := db.Model(&models.KnowledgeCandidate{}).Where("id = ?", candidate.ID).Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrationRootCauseSummary(repairs []models.TicketRepairRecord) string {
	repair := migrationLatestVerifiedKnowledgeRepair(repairs)
	if repair == nil {
		return ""
	}
	return strings.TrimSpace(repair.RootCause)
}

func migrationSolutionSummary(repairs []models.TicketRepairRecord) string {
	repair := migrationLatestVerifiedKnowledgeRepair(repairs)
	if repair == nil {
		return ""
	}
	solutions := make([]string, 0, 2)
	if solution := strings.TrimSpace(repair.Solution); solution != "" {
		solutions = append(solutions, solution)
	}
	conclusion := strings.TrimSpace(repair.Conclusion)
	if conclusion != "" && !migrationSummaryContains(solutions, conclusion) {
		solutions = append(solutions, "维修结论："+conclusion)
	}
	return strings.Join(solutions, "; ")
}

func migrationLatestVerifiedKnowledgeRepair(repairs []models.TicketRepairRecord) *models.TicketRepairRecord {
	for i := len(repairs) - 1; i >= 0; i-- {
		if strings.EqualFold(strings.TrimSpace(repairs[i].TestResult), "passed") {
			return &repairs[i]
		}
	}
	for i := len(repairs) - 1; i >= 0; i-- {
		if strings.TrimSpace(repairs[i].RootCause) != "" || strings.TrimSpace(repairs[i].Solution) != "" || strings.TrimSpace(repairs[i].Conclusion) != "" {
			return &repairs[i]
		}
	}
	return nil
}

func migrationSummaryContains(parts []string, value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	for _, part := range parts {
		if strings.Contains(part, value) {
			return true
		}
	}
	return false
}
