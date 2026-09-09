package builders

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/services"
)

var partnerEmailPattern = regexp.MustCompile(`(?i)([a-z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+)@([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+)`)

func maskPartnerEmails(value string) string {
	return partnerEmailPattern.ReplaceAllStringFunc(value, func(email string) string {
		parts := strings.SplitN(email, "@", 2)
		if len(parts) != 2 {
			return email
		}
		local := parts[0]
		if len(local) <= 1 {
			return "*@" + parts[1]
		}
		visible := 1
		if len(local) > 4 {
			visible = 2
		}
		return local[:visible] + strings.Repeat("*", len(local)-visible) + "@" + parts[1]
	})
}

func BuildTicketSupplierCollaboration(item services.TicketSupplierCollaborationAggregate) dto.TicketSupplierCollaborationDTO {
	ret := dto.TicketSupplierCollaborationDTO{}
	if item.Collaboration == nil {
		return ret
	}
	collaboration := item.Collaboration
	ret.ID = collaboration.ID
	ret.TicketID = collaboration.TicketID
	ret.ProductID = collaboration.ProductID
	ret.ProductModuleID = collaboration.ProductModuleID
	ret.PartnerCompanyID = collaboration.PartnerCompanyID
	ret.PartnerAccountID = collaboration.PartnerAccountID
	ret.Status = collaboration.Status
	ret.Reason = collaboration.Reason
	ret.Resolution = collaboration.Resolution
	ret.InvitedAt = utils.FormatTime(collaboration.InvitedAt)
	ret.ResolvedAt = utils.FormatTimePtr(collaboration.ResolvedAt)
	ret.AuthorizationEndsAt = utils.FormatTimePtr(collaboration.AuthorizationEnds)
	ret.AuthorizationActive = !services.IsSupplierCollaborationTerminalStatus(collaboration.Status) && (collaboration.AuthorizationEnds == nil || collaboration.AuthorizationEnds.After(time.Now()))
	ret.ParticipantCount = len(item.Participants)
	_ = json.Unmarshal([]byte(collaboration.VisibilityJSON), &ret.Visibility)
	if item.Ticket != nil {
		ret.ConversationID = item.Ticket.ConversationID
		ret.TicketNo = item.Ticket.TicketNo
		ret.TicketTitle = item.Ticket.Title
		ret.TicketStatus = string(item.Ticket.Status)
		ret.TicketUpdatedAt = utils.FormatTime(item.Ticket.UpdatedAt)
		ret.FaultCode = item.Ticket.FaultCode
		ret.SymptomSummary = item.Ticket.SymptomSummary
		ret.DiagnosisSummary = item.Ticket.DiagnosisSummary
	}
	if item.Module != nil {
		ret.ProductModuleName = item.Module.Name
	}
	if item.Company != nil {
		ret.PartnerCompanyName = item.Company.Name
	}
	if item.Account != nil {
		ret.PartnerAccountName = item.Account.DisplayName
	}
	return ret
}

func BuildTicketSupplierCollaborations(items []services.TicketSupplierCollaborationAggregate) []dto.TicketSupplierCollaborationDTO {
	result := make([]dto.TicketSupplierCollaborationDTO, 0, len(items))
	for i := range items {
		result = append(result, BuildTicketSupplierCollaboration(items[i]))
	}
	return result
}

func BuildTicketSupplierOptions(items []models.PartnerCompany) []dto.TicketSupplierOptionDTO {
	result := make([]dto.TicketSupplierOptionDTO, 0, len(items))
	for i := range items {
		result = append(result, dto.TicketSupplierOptionDTO{
			PartnerCompanyID: items[i].ID,
			Name:             items[i].Name,
		})
	}
	return result
}

func BuildPartnerTicketSupplierCollaboration(item services.TicketSupplierCollaborationAggregate) dto.TicketSupplierCollaborationDTO {
	ret := BuildTicketSupplierCollaboration(item)
	visibility := make(map[string]bool, len(ret.Visibility))
	for _, value := range ret.Visibility {
		visibility[value] = true
	}
	if !visibility["ticket_summary"] {
		ret.TicketNo = ""
		ret.TicketTitle = ""
		ret.TicketStatus = ""
		ret.TicketUpdatedAt = ""
	}
	ret.TicketTitle = maskPartnerEmails(ret.TicketTitle)
	if !visibility["diagnosis"] {
		ret.FaultCode = ""
		ret.SymptomSummary = ""
		ret.DiagnosisSummary = ""
	}
	return ret
}

func BuildPartnerTicketSupplierCollaborations(items []services.TicketSupplierCollaborationAggregate) []dto.TicketSupplierCollaborationDTO {
	result := make([]dto.TicketSupplierCollaborationDTO, 0, len(items))
	for i := range items {
		result = append(result, BuildPartnerTicketSupplierCollaboration(items[i]))
	}
	return result
}

func BuildPartnerPortalProfile(item services.TicketSupplierCollaborationAggregate) dto.PartnerPortalProfileDTO {
	ret := dto.PartnerPortalProfileDTO{}
	if item.Account == nil || item.Company == nil {
		return ret
	}
	ret.AccountID = item.Account.ID
	ret.UserID = item.Account.UserID
	ret.DisplayName = item.Account.DisplayName
	ret.Email = item.Account.Email
	ret.Phone = item.Account.Phone
	ret.CompanyID = item.Company.ID
	ret.CompanyName = item.Company.Name
	ret.PartnerNo = item.Company.PartnerNo
	ret.PartnerType = item.Company.PartnerType
	ret.CountryRegion = item.Company.CountryRegion
	ret.LanguagesJSON = item.Account.LanguagesJSON
	ret.LastActiveAt = utils.FormatTimePtr(item.Account.LastActiveAt)
	ret.AuthorizationOK = true
	ret.Roles = append([]string{}, item.Roles...)
	for _, role := range item.Roles {
		if role == services.PartnerRoleAdmin {
			ret.CanManageTeam = true
		}
		if role == services.PartnerRoleAdmin || role == services.PartnerRoleEngineer {
			ret.CanInviteTeam = true
		}
	}
	return ret
}

func BuildPartnerPortalAccounts(items []services.PartnerPortalAccountAggregate, currentAccountID int64) []dto.PartnerPortalAccountDTO {
	result := make([]dto.PartnerPortalAccountDTO, 0, len(items))
	for i := range items {
		item := items[i].Account
		username := ""
		if items[i].User != nil {
			username = items[i].User.Username
		}
		result = append(result, dto.PartnerPortalAccountDTO{
			ID:            item.ID,
			Username:      username,
			DisplayName:   item.DisplayName,
			Email:         item.Email,
			Phone:         item.Phone,
			LanguagesJSON: item.LanguagesJSON,
			Status:        int(item.Status),
			LastActiveAt:  utils.FormatTimePtr(item.LastActiveAt),
			UpdatedAt:     utils.FormatTime(item.UpdatedAt),
			IsCurrent:     item.ID == currentAccountID,
			Roles:         append([]string{}, items[i].Roles...),
		})
	}
	return result
}

func BuildPartnerPortalAccount(item services.PartnerPortalAccountAggregate, currentAccountID int64) dto.PartnerPortalAccountDTO {
	items := BuildPartnerPortalAccounts([]services.PartnerPortalAccountAggregate{item}, currentAccountID)
	if len(items) == 0 {
		return dto.PartnerPortalAccountDTO{}
	}
	return items[0]
}

func BuildPartnerPortalMeetings(items []services.PartnerPortalMeetingAggregate) dto.PartnerPortalMeetingListDTO {
	ret := dto.PartnerPortalMeetingListDTO{Items: make([]dto.PartnerPortalMeetingDTO, 0, len(items))}
	for i := range items {
		item := items[i]
		ret.Items = append(ret.Items, dto.PartnerPortalMeetingDTO{
			EnterpriseMeetingListItemDTO: item.Meeting,
			CollaborationID:              item.CollaborationID,
		})
		if item.Meeting.Status == "active" {
			ret.Summary.Active++
		} else if item.Meeting.Status == "waiting" || item.Meeting.Status == "scheduled" {
			ret.Summary.Waiting++
		} else if item.Meeting.Status == "finished" || item.Meeting.Status == "ended" {
			ret.Summary.Ended++
		}
		ret.Summary.Mine++
		ret.Summary.ParticipantsOnline += item.Meeting.ParticipantCount
	}
	return ret
}

func BuildPartnerTicketDetail(item *services.PartnerTicketDetailAggregate) dto.PartnerTicketDetailDTO {
	ret := dto.PartnerTicketDetailDTO{
		Participants: make([]dto.TicketSupplierParticipantDTO, 0),
		Progresses:   make([]dto.PartnerTicketProgressDTO, 0),
		Messages:     make([]dto.PartnerConversationMessageDTO, 0),
	}
	if item == nil {
		return ret
	}
	ret.Collaboration = BuildPartnerTicketSupplierCollaboration(item.Collaboration)
	for i := range item.Collaboration.Participants {
		participant := item.Collaboration.Participants[i]
		name := ""
		if participant.Account != nil {
			name = participant.Account.DisplayName
		}
		ret.Participants = append(ret.Participants, dto.TicketSupplierParticipantDTO{
			PartnerAccountID:   participant.Participant.PartnerAccountID,
			PartnerAccountName: name,
			Role:               participant.Participant.Role,
			Status:             int(participant.Participant.Status),
			JoinedAt:           utils.FormatTime(participant.Participant.JoinedAt),
			LeftAt:             utils.FormatTimePtr(participant.Participant.LeftAt),
		})
	}
	for i := range item.Progresses {
		progress := item.Progresses[i]
		ret.Progresses = append(ret.Progresses, dto.PartnerTicketProgressDTO{
			ID:         progress.Progress.ID,
			EventType:  string(progress.Progress.EventType),
			Content:    progress.Progress.Content,
			AuthorID:   progress.Progress.AuthorID,
			AuthorName: progress.AuthorName,
			CreatedAt:  utils.FormatTime(progress.Progress.CreatedAt),
		})
	}
	for i := range item.Messages {
		message := item.Messages[i]
		content, payload := utils.BuildRenderableMessage(&message.Message)
		ret.Messages = append(ret.Messages, dto.PartnerConversationMessageDTO{
			ID:          message.Message.ID,
			SenderID:    message.Message.SenderID,
			SenderType:  string(message.Message.SenderType),
			SenderName:  message.SenderName,
			MessageType: string(message.Message.MessageType),
			Content:     content,
			Payload:     payload,
			SentAt:      utils.FormatTimePtr(message.Message.SentAt),
		})
	}
	return ret
}
