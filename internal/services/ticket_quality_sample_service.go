package services

import (
	"encoding/json"
	"math/rand"
	"slices"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

const (
	ticketQualitySamplePending    = "pending"
	ticketQualitySampleInProgress = "in_progress"
	ticketQualitySampleCompleted  = "completed"
)

var TicketQualitySampleService = &ticketQualitySampleService{}

type ticketQualitySampleService struct{}

type ticketQualitySampleQuery struct {
	Page      int
	PageSize  int
	PeriodKey string
	Status    string
	Reason    string
	Search    string
}

// TicketQualitySampleQueryForHandler keeps HTTP parsing outside the service's
// internal query type while preserving tenant-scoped filtering.
type TicketQualitySampleQueryForHandler = ticketQualitySampleQuery

func (s *ticketQualitySampleService) Generate(tenantID int64, periodKey string, randomCount int, operator *dto.AuthPrincipal) (int, error) {
	if tenantID <= 0 {
		return 0, errorsx.InvalidParam("tenant is required")
	}
	periodKey = strings.TrimSpace(periodKey)
	if periodKey == "" {
		periodKey = time.Now().Format("2006-01-02")
	}
	if randomCount <= 0 {
		randomCount = 10
	}
	if randomCount > 500 {
		randomCount = 500
	}
	now := time.Now()
	var tickets []models.Ticket
	if err := sqls.DB().Where("tenant_id = ? AND status IN ?", tenantID, []string{
		string(enums.TicketStatusResolved), string(enums.TicketStatusClosed), string(enums.TicketStatusDone), string(enums.TicketStatusReopened),
	}).Order("updated_at DESC").Find(&tickets).Error; err != nil {
		return 0, err
	}
	if len(tickets) == 0 {
		return 0, nil
	}
	existing := make(map[int64]*models.TicketQualitySample)
	var current []models.TicketQualitySample
	if err := sqls.DB().Where("tenant_id = ? AND period_key = ?", tenantID, periodKey).Find(&current).Error; err != nil {
		return 0, err
	}
	for i := range current {
		existing[current[i].TicketID] = &current[i]
	}
	forced := make(map[int64][]string)
	var randomCandidates []models.Ticket
	for _, ticket := range tickets {
		reasons := s.ruleReasons(ticket, now)
		if len(reasons) > 0 {
			forced[ticket.ID] = reasons
		} else {
			randomCandidates = append(randomCandidates, ticket)
		}
	}
	r := rand.New(rand.NewSource(now.UnixNano()))
	r.Shuffle(len(randomCandidates), func(i, j int) { randomCandidates[i], randomCandidates[j] = randomCandidates[j], randomCandidates[i] })
	if randomCount > len(randomCandidates) {
		randomCount = len(randomCandidates)
	}
	for _, ticket := range randomCandidates[:randomCount] {
		forced[ticket.ID] = []string{"random"}
	}
	created := 0
	for ticketID, reasons := range forced {
		if currentSample := existing[ticketID]; currentSample != nil {
			merged := mergeQualityReasons(currentSample.ReasonsJSON, reasons)
			if err := sqls.DB().Model(&models.TicketQualitySample{}).Where("id = ?", currentSample.ID).Updates(map[string]any{"reasons_json": marshalQualityReasons(merged), "updated_at": now}).Error; err != nil {
				return created, err
			}
			continue
		}
		item := &models.TicketQualitySample{TenantID: tenantID, TicketID: ticketID, PeriodKey: periodKey, ReasonsJSON: marshalQualityReasons(reasons), Status: ticketQualitySamplePending, SampledAt: now, AuditFields: auditFieldsFor(operator)}
		if err := sqls.DB().Create(item).Error; err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

func (s *ticketQualitySampleService) ruleReasons(ticket models.Ticket, now time.Time) []string {
	reasons := make([]string, 0, 3)
	priority := strings.ToLower(strings.TrimSpace(ticket.PriorityLevel))
	if priority == "p0" || priority == "p1" || priority == "critical" || priority == "high" {
		reasons = append(reasons, "high_priority")
	}
	if ticket.Status == enums.TicketStatusReopened {
		reasons = append(reasons, "reopened")
	}
	text := strings.ToLower(ticket.Title + "\n" + ticket.Description)
	if strings.Contains(text, "投诉") || strings.Contains(text, "举报") || strings.Contains(text, "complaint") {
		reasons = append(reasons, "complaint")
	}
	if ticket.CurrentAssigneeID > 0 {
		if user := repositories.UserRepository.Get(sqls.DB(), ticket.CurrentAssigneeID); user != nil && user.CreatedAt.After(now.Add(-30*24*time.Hour)) {
			reasons = append(reasons, "new_agent")
		}
	}
	var clues []models.TicketQualityClue
	if err := sqls.DB().Where("tenant_id = ? AND ticket_id = ? AND status = ? AND severity IN ?", ticket.TenantID, ticket.ID, "open", []string{"warning", "critical"}).Find(&clues).Error; err == nil && len(clues) > 0 {
		reasons = append(reasons, "risk")
	}
	return uniqueQualityReasons(reasons)
}

func (s *ticketQualitySampleService) List(tenantID int64, q ticketQualitySampleQuery) (*dto.TicketQualitySamplePageDTO, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}
	db := sqls.DB().Model(&models.TicketQualitySample{}).Where("tenant_id = ?", tenantID)
	if q.PeriodKey != "" {
		db = db.Where("period_key = ?", q.PeriodKey)
	}
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	if q.Reason != "" {
		db = db.Where("reasons_json LIKE ?", "%\""+q.Reason+"\"%")
	}
	if q.Search != "" {
		db = db.Where("ticket_id IN (SELECT id FROM t_ticket WHERE tenant_id = ? AND (ticket_no LIKE ? OR title LIKE ?))", tenantID, "%"+q.Search+"%", "%"+q.Search+"%")
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []models.TicketQualitySample
	if err := db.Order("sampled_at DESC, id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	result := &dto.TicketQualitySamplePageDTO{Items: make([]dto.TicketQualitySampleDTO, 0, len(items)), Total: total, Page: q.Page, PageSize: q.PageSize, TotalPages: int((total + int64(q.PageSize) - 1) / int64(q.PageSize))}
	var summary []struct {
		Status string
		Count  int64
	}
	if err := sqls.DB().Model(&models.TicketQualitySample{}).Select("status, count(*) as count").Where("tenant_id = ?", tenantID).Group("status").Scan(&summary).Error; err == nil {
		for _, row := range summary {
			switch row.Status {
			case ticketQualitySamplePending:
				result.Summary.Pending = row.Count
			case ticketQualitySampleInProgress:
				result.Summary.InProgress = row.Count
			case ticketQualitySampleCompleted:
				result.Summary.Completed = row.Count
			}
		}
	}
	result.Summary.Total = result.Summary.Pending + result.Summary.InProgress + result.Summary.Completed
	for _, item := range items {
		result.Items = append(result.Items, s.buildDTO(item))
	}
	return result, nil
}

func (s *ticketQualitySampleService) Start(tenantID, id, reviewerID int64) error {
	return s.transition(tenantID, id, reviewerID, ticketQualitySampleInProgress, "")
}
func (s *ticketQualitySampleService) Complete(tenantID, id, reviewerID int64, note string) error {
	return s.transition(tenantID, id, reviewerID, ticketQualitySampleCompleted, note)
}
func (s *ticketQualitySampleService) transition(tenantID, id, reviewerID int64, status, note string) error {
	if tenantID <= 0 || id <= 0 {
		return errorsx.InvalidParam("sample is required")
	}
	now := time.Now()
	updates := map[string]any{"status": status, "reviewer_id": reviewerID, "updated_at": now}
	if status == ticketQualitySampleInProgress {
		updates["started_at"] = now
	}
	if status == ticketQualitySampleCompleted {
		updates["completed_at"] = now
		updates["review_note"] = strings.TrimSpace(note)
	}
	result := sqls.DB().Model(&models.TicketQualitySample{}).Where("id = ? AND tenant_id = ?", id, tenantID).Updates(updates)
	if result.RowsAffected == 0 {
		return errorsx.InvalidParam("quality sample not found")
	}
	return result.Error
}

func (s *ticketQualitySampleService) buildDTO(item models.TicketQualitySample) dto.TicketQualitySampleDTO {
	var reasons []string
	_ = json.Unmarshal([]byte(item.ReasonsJSON), &reasons)
	ticket := repositories.TicketRepository.Get(sqls.DB(), item.TicketID)
	ret := dto.TicketQualitySampleDTO{ID: item.ID, TicketID: item.TicketID, PeriodKey: item.PeriodKey, Reasons: reasons, Status: item.Status, ReviewerID: item.ReviewerID, ReviewNote: item.ReviewNote, SampledAt: item.SampledAt.Format(time.RFC3339), StartedAt: formatQualityTime(item.StartedAt), CompletedAt: formatQualityTime(item.CompletedAt)}
	if ticket != nil {
		ret.TicketNo = ticket.TicketNo
		ret.Title = ticket.Title
		ret.Priority = ticket.PriorityLevel
		ret.TicketStatus = string(ticket.Status)
		ret.AssigneeID = ticket.CurrentAssigneeID
	}
	if ret.AssigneeID > 0 {
		if user := repositories.UserRepository.Get(sqls.DB(), ret.AssigneeID); user != nil {
			ret.AssigneeName = user.Nickname
		}
	}
	return ret
}

func formatQualityTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339)
}
func auditFieldsFor(operator *dto.AuthPrincipal) models.AuditFields {
	name := "system"
	id := int64(0)
	if operator != nil {
		name = operator.Username
		id = operator.UserID
	}
	return models.AuditFields{CreateUserID: id, CreateUserName: name, UpdateUserID: id, UpdateUserName: name, CreatedAt: time.Now(), UpdatedAt: time.Now()}
}
func marshalQualityReasons(items []string) string {
	b, _ := json.Marshal(uniqueQualityReasons(items))
	return string(b)
}
func mergeQualityReasons(raw string, add []string) []string {
	var current []string
	_ = json.Unmarshal([]byte(raw), &current)
	return uniqueQualityReasons(append(current, add...))
}
func uniqueQualityReasons(items []string) []string {
	ret := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" && !slices.Contains(ret, item) {
			ret = append(ret, item)
		}
	}
	return ret
}
