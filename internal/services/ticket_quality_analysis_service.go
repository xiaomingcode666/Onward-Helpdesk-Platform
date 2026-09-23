package services

import (
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

type TicketQualityAnalysisFilter struct {
	Project  string
	TeamID   int64
	AgentID  int64
	Category string
	Channel  string
	From     time.Time
	To       time.Time
}

type TicketQualityAnalysisBucket struct {
	Value    string  `json:"value"`
	Label    string  `json:"label"`
	Reviews  int64   `json:"reviews"`
	Average  float64 `json:"average"`
	PassRate float64 `json:"pass_rate"`
}

type TicketQualityAnalysis struct {
	From          string                        `json:"from"`
	To            string                        `json:"to"`
	CanViewAgents bool                          `json:"can_view_agents"`
	Summary       TicketQualityAnalysisBucket   `json:"summary"`
	ByProject     []TicketQualityAnalysisBucket `json:"by_project"`
	ByTeam        []TicketQualityAnalysisBucket `json:"by_team"`
	ByAgent       []TicketQualityAnalysisBucket `json:"by_agent,omitempty"`
	ByCategory    []TicketQualityAnalysisBucket `json:"by_category"`
	ByChannel     []TicketQualityAnalysisBucket `json:"by_channel"`
}

func AnalyzeTicketQuality(tenantID int64, filter TicketQualityAnalysisFilter, operator *dto.AuthPrincipal) (*TicketQualityAnalysis, error) {
	if operator == nil || tenantID <= 0 || operator.EffectiveTenantID() != tenantID || !operator.HasPermission(constants.PermissionTicketView.Code) {
		return nil, errorsx.Forbidden("无质量分析查看权限")
	}
	canViewAgents := operator.HasRole(EnterpriseRoleOwner) || operator.HasRole(EnterpriseRoleAdmin) || operator.HasRole(EnterpriseRoleServiceManager) || operator.HasPermission(constants.PermissionTicketAssign.Code)
	var reviews []models.TicketQualityReview
	query := sqls.DB().Where("tenant_id = ?", tenantID)
	if !filter.From.IsZero() {
		query = query.Where("reviewed_at >= ?", filter.From)
	}
	if !filter.To.IsZero() {
		query = query.Where("reviewed_at < ?", filter.To)
	}
	if err := query.Order("reviewed_at DESC, id DESC").Find(&reviews).Error; err != nil {
		return nil, err
	}
	if len(reviews) == 0 {
		return emptyTicketQualityAnalysis(filter, canViewAgents), nil
	}

	ticketIDs := make([]int64, 0, len(reviews))
	seen := map[int64]bool{}
	for _, review := range reviews {
		if !seen[review.TicketID] {
			ticketIDs = append(ticketIDs, review.TicketID)
			seen[review.TicketID] = true
		}
	}
	var tickets []models.Ticket
	if err := sqls.DB().Where("tenant_id = ? AND id IN ?", tenantID, ticketIDs).Find(&tickets).Error; err != nil {
		return nil, err
	}
	ticketByID := make(map[int64]models.Ticket, len(tickets))
	for _, ticket := range tickets {
		ticketByID[ticket.ID] = ticket
	}
	teamNames := map[int64]string{}
	var teams []models.AgentTeam
	if err := sqls.DB().Where("tenant_id = ?", tenantID).Find(&teams).Error; err == nil {
		for _, team := range teams {
			teamNames[team.ID] = team.Name
		}
	}

	type dimensions struct{ project, team, agent, category, channel string }
	buckets := map[string]map[string]*TicketQualityAnalysisBucket{"project": {}, "team": {}, "agent": {}, "category": {}, "channel": {}}
	summary := &TicketQualityAnalysisBucket{Value: "all", Label: "全部"}
	for _, review := range reviews {
		ticket, ok := ticketByID[review.TicketID]
		if !ok {
			continue
		}
		if filter.Project != "" && ticket.ProjectKey != filter.Project {
			continue
		}
		if filter.TeamID > 0 && ticket.CurrentTeamID != filter.TeamID {
			continue
		}
		if filter.AgentID > 0 && ticket.CurrentAssigneeID != filter.AgentID {
			continue
		}
		if filter.Category != "" && ticket.CaseType != filter.Category {
			continue
		}
		if filter.Channel != "" && ticket.Channel != filter.Channel {
			continue
		}
		d := dimensions{project: ticket.ProjectKey, team: teamNames[ticket.CurrentTeamID], agent: "", category: ticket.CaseType, channel: ticket.Channel}
		if d.project == "" {
			d.project = "未设置"
		}
		if d.team == "" {
			d.team = "未分组"
		}
		if d.category == "" {
			d.category = "未分类"
		}
		if d.channel == "" {
			d.channel = "未设置"
		}
		if canViewAgents {
			if user := repositories.UserRepository.Get(sqls.DB(), ticket.CurrentAssigneeID); user != nil {
				d.agent = user.Nickname
			}
			if d.agent == "" {
				d.agent = "未分配"
			}
		}
		summary.Reviews++
		summary.Average += float64(review.TotalScore)
		if review.Result == "pass" {
			summary.PassRate++
		}
		addQualityBucket(buckets["project"], d.project, d.project, review)
		addQualityBucket(buckets["team"], teamKey(ticket.CurrentTeamID, d.team), d.team, review)
		if canViewAgents {
			addQualityBucket(buckets["agent"], agentKey(ticket.CurrentAssigneeID, d.agent), d.agent, review)
		}
		addQualityBucket(buckets["category"], d.category, d.category, review)
		addQualityBucket(buckets["channel"], d.channel, d.channel, review)
	}
	finalizeQualityBucket(summary)
	result := &TicketQualityAnalysis{From: formatQualityAnalysisTime(filter.From), To: formatQualityAnalysisTime(filter.To), CanViewAgents: canViewAgents, Summary: *summary, ByProject: bucketValues(buckets["project"]), ByTeam: bucketValues(buckets["team"]), ByCategory: bucketValues(buckets["category"]), ByChannel: bucketValues(buckets["channel"])}
	if canViewAgents {
		result.ByAgent = bucketValues(buckets["agent"])
	}
	return result, nil
}

func addQualityBucket(target map[string]*TicketQualityAnalysisBucket, value, label string, review models.TicketQualityReview) {
	b := target[value]
	if b == nil {
		b = &TicketQualityAnalysisBucket{Value: value, Label: label}
		target[value] = b
	}
	b.Reviews++
	b.Average += float64(review.TotalScore)
	if review.Result == "pass" {
		b.PassRate++
	}
}
func finalizeQualityBucket(b *TicketQualityAnalysisBucket) {
	if b.Reviews > 0 {
		b.Average /= float64(b.Reviews)
		b.PassRate = b.PassRate / float64(b.Reviews) * 100
	}
}
func bucketValues(values map[string]*TicketQualityAnalysisBucket) []TicketQualityAnalysisBucket {
	result := make([]TicketQualityAnalysisBucket, 0, len(values))
	for _, b := range values {
		finalizeQualityBucket(b)
		result = append(result, *b)
	}
	slicesSortQualityBuckets(result)
	return result
}
func slicesSortQualityBuckets(values []TicketQualityAnalysisBucket) {
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if values[j].Reviews > values[i].Reviews || (values[j].Reviews == values[i].Reviews && strings.Compare(values[j].Label, values[i].Label) < 0) {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}
func teamKey(id int64, label string) string {
	if id > 0 {
		return "team:" + strconv.FormatInt(id, 10)
	}
	return label
}
func agentKey(id int64, label string) string {
	if id > 0 {
		return "agent:" + strconv.FormatInt(id, 10)
	}
	return label
}
func formatQualityAnalysisTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}
func emptyTicketQualityAnalysis(filter TicketQualityAnalysisFilter, canViewAgents bool) *TicketQualityAnalysis {
	return &TicketQualityAnalysis{From: formatQualityAnalysisTime(filter.From), To: formatQualityAnalysisTime(filter.To), CanViewAgents: canViewAgents, Summary: TicketQualityAnalysisBucket{Value: "all", Label: "全部"}, ByProject: []TicketQualityAnalysisBucket{}, ByTeam: []TicketQualityAnalysisBucket{}, ByCategory: []TicketQualityAnalysisBucket{}, ByChannel: []TicketQualityAnalysisBucket{}, ByAgent: []TicketQualityAnalysisBucket{}}
}
