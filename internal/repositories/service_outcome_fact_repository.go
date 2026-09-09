package repositories

import (
	"strconv"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ServiceOutcomeFactRepository = newServiceOutcomeFactRepository()

func newServiceOutcomeFactRepository() *serviceOutcomeFactRepository {
	return &serviceOutcomeFactRepository{}
}

type serviceOutcomeFactRepository struct{}

type ServiceOutcomeFactSources struct {
	Ticket                  models.Ticket
	Conversation            *models.Conversation
	Diagnoses               []models.DiagnosisSession
	Repairs                 []models.TicketRepairRecord
	Feedback                *models.TicketFeedback
	Collaborations          []models.TicketSupplierCollaboration
	Meetings                []models.MeetingRoomJitsi
	Participants            []models.MeetingParticipant
	RetrieveLogs            []models.KnowledgeRetrieveLog
	RetrieveHits            []models.KnowledgeRetrieveHit
	Candidate               *models.KnowledgeCandidate
	ReopenCount             int64
	RepeatTicketCount30Days int64
}

func (r *serviceOutcomeFactRepository) LoadSources(db *gorm.DB, tenantID, ticketID int64) (*ServiceOutcomeFactSources, error) {
	var ticket models.Ticket
	if err := db.Where("tenant_id = ? AND id = ?", tenantID, ticketID).First(&ticket).Error; err != nil {
		return nil, err
	}

	sources := &ServiceOutcomeFactSources{Ticket: ticket}
	if ticket.ConversationID > 0 {
		var conversation models.Conversation
		if err := db.Where("tenant_id = ? AND id = ?", tenantID, ticket.ConversationID).First(&conversation).Error; err == nil {
			sources.Conversation = &conversation
		} else if err != gorm.ErrRecordNotFound {
			return nil, err
		}
	}

	if ticket.ConversationID > 0 {
		conversationID := strconv.FormatInt(ticket.ConversationID, 10)
		if err := db.Where("tenant_id = ? AND (conversation_ref_id = ? OR conversation_id = ?)", tenantID, ticket.ConversationID, conversationID).
			Order("created_at ASC").Find(&sources.Diagnoses).Error; err != nil {
			return nil, err
		}
	}
	if err := db.Where("tenant_id = ? AND ticket_id = ?", tenantID, ticketID).
		Order("created_at ASC").Find(&sources.Repairs).Error; err != nil {
		return nil, err
	}
	var feedback models.TicketFeedback
	if err := db.Where("tenant_id = ? AND ticket_id = ? AND status = ? AND rating BETWEEN 1 AND 5", tenantID, ticketID, "submitted").
		Order("submitted_at DESC").Order("id DESC").First(&feedback).Error; err == nil {
		sources.Feedback = &feedback
	} else if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	if err := db.Where("tenant_id = ? AND ticket_id = ? AND record_status <> ?", tenantID, ticketID, enums.StatusDeleted).
		Order("invited_at ASC").Find(&sources.Collaborations).Error; err != nil {
		return nil, err
	}

	ticketIDText := strconv.FormatInt(ticketID, 10)
	if err := db.Where("tenant_id = ? AND ticket_id = ?", tenantID, ticketIDText).
		Order("created_at ASC").Find(&sources.Meetings).Error; err != nil {
		return nil, err
	}
	meetingIDs := make([]string, 0, len(sources.Meetings))
	for i := range sources.Meetings {
		meetingIDs = append(meetingIDs, sources.Meetings[i].ID)
	}
	if len(meetingIDs) > 0 {
		if err := db.Where("meeting_id IN ?", meetingIDs).Order("created_at ASC").Find(&sources.Participants).Error; err != nil {
			return nil, err
		}
	}

	if ticket.ConversationID > 0 {
		if err := db.Where("tenant_id = ? AND conversation_id = ?", tenantID, ticket.ConversationID).
			Order("created_at ASC").Find(&sources.RetrieveLogs).Error; err != nil {
			return nil, err
		}
		logIDs := make([]int64, 0, len(sources.RetrieveLogs))
		for i := range sources.RetrieveLogs {
			logIDs = append(logIDs, sources.RetrieveLogs[i].ID)
		}
		if len(logIDs) > 0 {
			if err := db.Where("tenant_id = ? AND retrieve_log_id IN ?", tenantID, logIDs).
				Order("retrieve_log_id ASC").Order("rank_no ASC").Find(&sources.RetrieveHits).Error; err != nil {
				return nil, err
			}
		}
	}

	var candidate models.KnowledgeCandidate
	if err := db.Where("tenant_id = ? AND ticket_id = ? AND status <> ?", tenantID, ticketID, enums.StatusDeleted).
		Order("knowledge_entry_id DESC").Order("id DESC").First(&candidate).Error; err == nil {
		sources.Candidate = &candidate
	} else if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	if err := db.Model(&models.TicketProgress{}).
		Where("tenant_id = ? AND ticket_id = ? AND event_type = ?", tenantID, ticketID, enums.TicketProgressEventReopened).
		Count(&sources.ReopenCount).Error; err != nil {
		return nil, err
	}
	if ticket.DeviceID > 0 && ticket.ResolvedAt != nil {
		repeatQuery := db.Model(&models.Ticket{}).
			Where("tenant_id = ? AND device_id = ? AND id <> ? AND created_at > ? AND created_at <= ?",
				tenantID, ticket.DeviceID, ticketID, *ticket.ResolvedAt, ticket.ResolvedAt.AddDate(0, 0, 30))
		if faultCode := strings.TrimSpace(ticket.FaultCode); faultCode != "" {
			repeatQuery = repeatQuery.Where("fault_code = ?", faultCode)
		} else if ticket.ProductModelID > 0 {
			repeatQuery = repeatQuery.Where("product_model_id = ?", ticket.ProductModelID)
		} else {
			repeatQuery = nil
		}
		if repeatQuery != nil {
			if err := repeatQuery.Count(&sources.RepeatTicketCount30Days).Error; err != nil {
				return nil, err
			}
		}
	}
	return sources, nil
}

func (r *serviceOutcomeFactRepository) Find(db *gorm.DB, tenantID, ticketID int64, metricVersion string) (*models.TicketServiceOutcomeFact, error) {
	var fact models.TicketServiceOutcomeFact
	if err := db.Where("tenant_id = ? AND ticket_id = ? AND metric_version = ?", tenantID, ticketID, metricVersion).
		First(&fact).Error; err != nil {
		return nil, err
	}
	return &fact, nil
}

func (r *serviceOutcomeFactRepository) Upsert(db *gorm.DB, fact *models.TicketServiceOutcomeFact) error {
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "ticket_id"}, {Name: "metric_version"}},
		UpdateAll: true,
	}).Create(fact).Error
}
