package builders

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/services"
	"time"
)

func BuildEngineerBriefingResponse(result *services.EngineerBriefingResult) *dto.EngineerBriefingDTO {
	if result == nil {
		return nil
	}
	ret := &dto.EngineerBriefingDTO{
		IsEngineer:        result.IsEngineer,
		Teams:             make([]dto.EngineerTeamBriefDTO, 0, len(result.Teams)),
		UnassignedCount:   result.UnassignedTicketCount,
		MyOpenCount:       result.MyOpenTicketCount,
		UnassignedTickets: buildEngineerBriefingTickets(result.UnassignedTickets),
		MyOpenTickets:     buildEngineerBriefingTickets(result.MyOpenTickets),
	}
	if result.IsEngineer {
		ret.WorkStatus = dto.EngineerWorkStatusDTO{
			Status:            result.WorkStatus.Status,
			Note:              result.WorkStatus.Note,
			AvailableAt:       formatBriefingTime(result.WorkStatus.AvailableAt),
			ConfirmedAt:       formatBriefingTime(&result.WorkStatus.ConfirmedAt),
			StatusChangedAt:   formatBriefingTime(&result.WorkStatus.StatusChangedAt),
			NeedsConfirmation: result.NeedsConfirmation,
		}
	}
	for _, team := range result.Teams {
		ret.Teams = append(ret.Teams, dto.EngineerTeamBriefDTO{ID: team.ID, Name: team.Name, ProductID: team.ProductID})
	}
	return ret
}

func BuildEngineerWorkStatusResponse(item *models.AgentWorkStatus) dto.EngineerWorkStatusDTO {
	if item == nil {
		return dto.EngineerWorkStatusDTO{}
	}
	return dto.EngineerWorkStatusDTO{
		Status:          item.Status,
		Note:            item.Note,
		AvailableAt:     formatBriefingTime(item.AvailableAt),
		ConfirmedAt:     formatBriefingTime(&item.ConfirmedAt),
		StatusChangedAt: formatBriefingTime(&item.StatusChangedAt),
	}
}

func buildEngineerBriefingTickets(items []models.Ticket) []dto.EnterpriseTicketListItemDTO {
	ret := make([]dto.EnterpriseTicketListItemDTO, 0, len(items))
	for _, item := range items {
		ret = append(ret, services.EnterpriseTicketService.BuildListItem(item))
	}
	return ret
}

func formatBriefingTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}
