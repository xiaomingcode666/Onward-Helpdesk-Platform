package builders

import (
	"encoding/json"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/utils"
)

func BuildTicketKnowledgeCandidates(items []models.KnowledgeCandidate) []dto.TicketKnowledgeCandidateDTO {
	result := make([]dto.TicketKnowledgeCandidateDTO, 0, len(items))
	for i := range items {
		result = append(result, *BuildTicketKnowledgeCandidate(&items[i], false))
	}
	return result
}

func BuildTicketKnowledgeCandidate(item *models.KnowledgeCandidate, created bool) *dto.TicketKnowledgeCandidateDTO {
	if item == nil {
		return nil
	}
	status := "pending"
	if item.Status == enums.StatusDeleted {
		status = "deleted"
	} else if item.KnowledgeEntryID > 0 {
		status = "linked"
	}
	flags := make([]string, 0)
	_ = json.Unmarshal([]byte(item.QualityFlagsJSON), &flags)
	return &dto.TicketKnowledgeCandidateDTO{
		ID:                    item.ID,
		TicketID:              item.TicketID,
		SourceType:            item.SourceType,
		Title:                 item.Title,
		Suggestion:            item.Suggestion,
		RootCauseSummary:      item.RootCauseSummary,
		SolutionSummary:       item.SolutionSummary,
		KnowledgeBaseID:       item.KnowledgeBaseID,
		KnowledgeEntryID:      item.KnowledgeEntryID,
		QualityScore:          item.QualityScore,
		ValueScore:            item.ValueScore,
		CandidateScore:        item.CandidateScore,
		QualityFlags:          flags,
		DeduplicationOverride: item.DeduplicationOverride,
		RecurrenceCount:       item.RecurrenceCount,
		AffectedDeviceCount:   item.AffectedDeviceCount,
		RequiresReassessment:  item.RequiresReassessment,
		ReviewEligible:        enums.IsKnowledgeCandidateReviewReady(item.ReviewStatus, item.QualityScore, item.ValueScore, item.MergedToCandidateID),
		Status:                status,
		ReviewStatus:          item.ReviewStatus,
		CreatedBy:             ticketKnowledgeCandidateCreatedBy(item),
		CreatedAt:             utils.FormatTime(item.CreatedAt),
		Created:               created,
	}
}

func ticketKnowledgeCandidateCreatedBy(item *models.KnowledgeCandidate) string {
	if item == nil {
		return ""
	}
	if item.SourceType == "ticket_repair" && item.CreateUserName == "客户访客" {
		return "客户确认触发"
	}
	if item.CreateUserName == "" {
		return "系统自动生成"
	}
	return item.CreateUserName
}
