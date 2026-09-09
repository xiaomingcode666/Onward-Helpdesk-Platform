package builders

import (
	"encoding/json"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/services"
)

type TicketBuildContext struct {
	TagsByTicketID map[int64][]models.Tag
	Users          map[int64]*models.User
	Customers      map[int64]*models.Customer
}

type TicketDetailBuildContext struct {
	Users map[int64]*models.User
}

func BuildTicket(item *models.Ticket) *response.TicketResponse {
	return BuildTicketWithContext(item, nil)
}

func BuildTicketWithContext(item *models.Ticket, ctx *TicketBuildContext) *response.TicketResponse {
	if item == nil {
		return nil
	}
	ret := &response.TicketResponse{
		ID:                     item.ID,
		TicketNo:               item.TicketNo,
		Title:                  item.Title,
		Description:            item.Description,
		Source:                 item.Source,
		Channel:                item.Channel,
		TicketIntakeDTO:        services.BuildTicketIntakeDTO(item),
		CustomerID:             item.CustomerID,
		ConversationID:         item.ConversationID,
		Status:                 item.Status,
		CurrentAssigneeID:      item.CurrentAssigneeID,
		CreatedBy:              item.CreateUserID,
		CreatedByName:          item.CreateUserName,
		HandledAt:              utils.FormatTimePtr(item.HandledAt),
		CreatedAt:              utils.FormatTime(item.CreatedAt),
		UpdatedAt:              utils.FormatTime(item.UpdatedAt),
		TenantID:               item.TenantID,
		ProductID:              item.ProductID,
		ProductModelID:         item.ProductModelID,
		ProductModuleID:        item.ProductModuleID,
		DeviceID:               item.DeviceID,
		ServiceCodeID:          item.ServiceCodeID,
		CustomerEntrySessionID: item.CustomerEntrySessionID,
		ServiceRegion:          item.ServiceRegion,
		FaultCode:              item.FaultCode,
		SymptomSummary:         item.SymptomSummary,
		DiagnosisSummary:       item.DiagnosisSummary,
		SLADueAt:               utils.FormatTimePtr(item.SLADueAt),
		ResolvedAt:             utils.FormatTimePtr(item.ResolvedAt),
		AssignedAt:             utils.FormatTimePtr(item.AssignedAt),
		AcceptedAt:             utils.FormatTimePtr(item.AcceptedAt),
		AcceptDeadlineAt:       utils.FormatTimePtr(item.AcceptDeadlineAt),
		DispatchAttempts:       item.DispatchAttempts,
	}
	if ctx != nil && ctx.TagsByTicketID != nil {
		ret.Tags = BuildTagResponses(ctx.TagsByTicketID[item.ID])
	}
	if item.CurrentAssigneeID > 0 {
		if ctx != nil && ctx.Users != nil {
			ret.CurrentAssigneeName = buildTicketUserDisplayName(ctx.Users[item.CurrentAssigneeID])
		}
	}
	if item.CustomerID > 0 {
		if ctx != nil && ctx.Customers != nil {
			ret.Customer = BuildCustomer(ctx.Customers[item.CustomerID])
		}
	}
	return ret
}

func BuildTicketList(list []models.Ticket) []response.TicketResponse {
	return BuildTicketListWithContext(list, nil)
}

func BuildTicketListWithContext(list []models.Ticket, ctx *TicketBuildContext) []response.TicketResponse {
	if len(list) == 0 {
		return nil
	}
	results := make([]response.TicketResponse, 0, len(list))
	for i := range list {
		if item := BuildTicketWithContext(&list[i], ctx); item != nil {
			results = append(results, *item)
		}
	}
	return results
}

func BuildTicketProgress(item *models.TicketProgress) *response.TicketProgressResponse {
	return BuildTicketProgressWithContext(item, nil)
}

func BuildTicketProgressWithContext(item *models.TicketProgress, ctx *TicketDetailBuildContext) *response.TicketProgressResponse {
	if item == nil {
		return nil
	}
	ret := &response.TicketProgressResponse{
		ID:                item.ID,
		TicketID:          item.TicketID,
		Content:           item.Content,
		VisibleToCustomer: item.VisibleToCustomer,
		AuthorID:          item.AuthorID,
		CreatedAt:         utils.FormatTime(item.CreatedAt),
	}
	if item.AuthorID > 0 {
		if ctx != nil && ctx.Users != nil {
			ret.AuthorName = buildTicketUserDisplayName(ctx.Users[item.AuthorID])
		}
	}
	return ret
}

func BuildTicketProgressList(list []models.TicketProgress) []response.TicketProgressResponse {
	return BuildTicketProgressListWithContext(list, nil)
}

func BuildTicketProgressListWithContext(list []models.TicketProgress, ctx *TicketDetailBuildContext) []response.TicketProgressResponse {
	if len(list) == 0 {
		return nil
	}
	results := make([]response.TicketProgressResponse, 0, len(list))
	for i := range list {
		if item := BuildTicketProgressWithContext(&list[i], ctx); item != nil {
			results = append(results, *item)
		}
	}
	return results
}

func BuildTicketDetail(aggregate *services.TicketDetailAggregate) *response.TicketDetailResponse {
	if aggregate == nil || aggregate.Ticket == nil {
		return nil
	}
	ctx := &TicketBuildContext{
		TagsByTicketID: map[int64][]models.Tag{aggregate.Ticket.ID: aggregate.Tags},
		Users:          aggregate.Users,
		Customers:      map[int64]*models.Customer{},
	}
	if aggregate.Customer != nil {
		ctx.Customers[aggregate.Customer.ID] = aggregate.Customer
	}
	ret := &response.TicketDetailResponse{
		Ticket: *BuildTicketWithContext(aggregate.Ticket, ctx),
	}
	ret.Progresses = BuildTicketProgressListWithContext(aggregate.Progresses, &TicketDetailBuildContext{Users: aggregate.Users})
	return ret
}

func BuildTicketSummary(summary *services.TicketSummaryAggregate) *response.TicketSummaryResponse {
	if summary == nil {
		return nil
	}
	return &response.TicketSummaryResponse{
		All:        summary.All,
		Pending:    summary.Pending,
		InProgress: summary.InProgress,
		Done:       summary.Done,
		Unassigned: summary.Unassigned,
		Mine:       summary.Mine,
		Stale:      summary.Stale,
	}
}

func BuildTicketView(item *models.TicketView) *response.TicketViewResponse {
	if item == nil {
		return nil
	}
	ret := &response.TicketViewResponse{
		ID:     item.ID,
		Name:   item.Name,
		SortNo: item.SortNo,
	}
	if strings.TrimSpace(item.FiltersJSON) != "" {
		_ = json.Unmarshal([]byte(item.FiltersJSON), &ret.Filters)
	}
	return ret
}

func BuildTicketViewList(list []models.TicketView) []response.TicketViewResponse {
	if len(list) == 0 {
		return nil
	}
	results := make([]response.TicketViewResponse, 0, len(list))
	for i := range list {
		if item := BuildTicketView(&list[i]); item != nil {
			results = append(results, *item)
		}
	}
	return results
}

func buildTicketUserDisplayName(user *models.User) string {
	if user == nil {
		return ""
	}
	if user.Nickname != "" {
		return user.Nickname
	}
	return user.Username
}
