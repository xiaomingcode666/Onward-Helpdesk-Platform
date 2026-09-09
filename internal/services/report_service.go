package services

import (
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var ReportService = newReportService()

func newReportService() *reportService {
	return &reportService{}
}

type reportService struct{}

// ProductReport 产品维度报表指标
type ProductReport struct {
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"`

	// 工单指标
	TotalTickets       int64   `json:"total_tickets"`
	OpenTickets        int64   `json:"open_tickets"`
	ClosedTickets      int64   `json:"closed_tickets"`
	AvgResolutionHours float64 `json:"avg_resolution_hours"`

	// AI 指标
	AISessions     int64   `json:"ai_sessions"`
	AIResolveRate  float64 `json:"ai_resolve_rate"` // AI 自助解决率
	AvgConfidence  float64 `json:"avg_confidence"`
	EscalationRate float64 `json:"escalation_rate"` // 转人工率

	// 知识指标
	KnowledgeEntries int64   `json:"knowledge_entries"`
	KnowledgeHitRate float64 `json:"knowledge_hit_rate"` // 知识命中率

	// 会议指标
	TotalMeetings       int64 `json:"total_meetings"`
	TotalMeetingMinutes int64 `json:"total_meeting_minutes"`

	// 满意度指标
	AvgSatisfaction float64 `json:"avg_satisfaction"`
	LowScoreRate    float64 `json:"low_score_rate"` // 低分率（<3分）

	// 成本指标
	TotalTokensIn  int64   `json:"total_tokens_in"`
	TotalTokensOut int64   `json:"total_tokens_out"`
	EstimatedCost  float64 `json:"estimated_cost"`

	// 采购决策指标（来自当前版本的工单服务结果事实）
	ExpertInterventionRate    *float64 `json:"expert_intervention_rate"`
	AvoidedTrips              *int64   `json:"avoided_trips"`
	FirstTimeFixRate          *float64 `json:"first_time_fix_rate"`
	AvgDowntimeMinutes        *float64 `json:"avg_downtime_minutes"`
	KnowledgeReuseRate        *float64 `json:"knowledge_reuse_rate"`
	RemoteResolutionRate      *float64 `json:"remote_resolution_rate"`
	OutcomeMetricCoverageRate *float64 `json:"outcome_metric_coverage_rate"`
	OutcomeMetricSampleSize   int64    `json:"outcome_metric_sample_size"`
}

// DashboardOverview 运营概览
type DashboardOverview struct {
	TotalProducts             int64         `json:"total_products"`
	TotalDevices              int64         `json:"total_devices"`
	TotalTickets              int64         `json:"total_tickets"`
	PendingTickets            int64         `json:"pending_tickets"`
	InProgressTickets         int64         `json:"in_progress_tickets"`
	SLAAtRisk                 int64         `json:"sla_at_risk"`
	NewToday                  int64         `json:"new_today"`
	AISessions                int64         `json:"ai_sessions"`
	AIResolveRate             float64       `json:"ai_resolve_rate"`
	ExpertInterventionRate    *float64      `json:"expert_intervention_rate"`
	AvoidedTrips              *int64        `json:"avoided_trips"`
	FirstTimeFixRate          *float64      `json:"first_time_fix_rate"`
	AvgDowntimeMinutes        *float64      `json:"avg_downtime_minutes"`
	KnowledgeReuseRate        *float64      `json:"knowledge_reuse_rate"`
	RemoteResolutionRate      *float64      `json:"remote_resolution_rate"`
	OutcomeMetricCoverageRate *float64      `json:"outcome_metric_coverage_rate"`
	OutcomeMetricSampleSize   int64         `json:"outcome_metric_sample_size"`
	RecentActivities          []Activity    `json:"recent_activities"`
	QueueTickets              []QueueTicket `json:"queue_tickets"`
}

// Activity 最近活动记录
type Activity struct {
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// QueueTicket 工作台待办队列工单
type QueueTicket struct {
	TicketNo string `json:"ticket_no"`
	Customer string `json:"customer"`
	Summary  string `json:"summary"`
	Level    string `json:"level"`
	State    string `json:"state"`
	Owner    string `json:"owner"`
	Label    string `json:"label"`
}

// TrendPoint 趋势数据点
type TrendPoint struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
	Label string  `json:"label,omitempty"`
}

// ProductFailureRank 产品故障排行
type ProductFailureRank struct {
	ProductID        string  `json:"product_id"`
	ProductName      string  `json:"product_name"`
	FailureCount     int64   `json:"failure_count"`
	EscalationRate   float64 `json:"escalation_rate"`
	AvgResolutionHrs float64 `json:"avg_resolution_hours"`
	Trend            string  `json:"trend"` // up / down / stable
}

// AgentPerformance 坐席绩效
type AgentPerformance struct {
	AgentID         string  `json:"agent_id"`
	AgentName       string  `json:"agent_name"`
	TicketsResolved int64   `json:"tickets_resolved"`
	TicketsAssigned int64   `json:"tickets_assigned"`
	AvgResponseMin  float64 `json:"avg_response_minutes"`
	AvgHandleMin    float64 `json:"avg_handle_minutes"`
	Satisfaction    float64 `json:"satisfaction"`
	MeetingsHeld    int64   `json:"meetings_held"`
}

type SupplierPerformanceAggregate struct {
	SupplierID         int64
	SupplierName       string
	SupplierNo         string
	TicketsAssigned    int64
	TicketsProcessing  int64
	TicketsCompleted   int64
	TicketsResponded   int64
	AvgResponseMinutes float64
	CompletionRate     float64
	AverageRating      float64
	RatingCount        int64
}

type supplierTicketPerformance struct {
	SupplierID      int64
	TicketID        int64
	InvitedAt       time.Time
	AcceptedAt      *time.Time
	HasActiveWork   bool
	HasResolvedWork bool
}

type supplierPerformanceAccumulator struct {
	Aggregate      SupplierPerformanceAggregate
	ResponseMinute float64
	RatingTotal    float64
}

// GetProductReport 获取产品维度报表
func (s *reportService) GetProductReport(ctx interface{}, tenantID string, productID string, startDate, endDate time.Time) (*ProductReport, error) {
	db := sqls.DB()

	report := &ProductReport{
		ProductID: productID,
	}

	// 查询产品名称
	var product models.Product
	if err := db.Select("name").Where("tenant_id = ? AND id = ?", tenantID, productID).First(&product).Error; err == nil {
		report.ProductName = product.Name
	}

	// 工单统计
	var ticketStats struct {
		Total    int64
		Open     int64
		Closed   int64
		AvgHours float64
	}
	db.Model(&models.Ticket{}).
		Select("COUNT(*) as total, "+
			"SUM(CASE WHEN status NOT IN ('closed','resolved') THEN 1 ELSE 0 END) as open, "+
			"SUM(CASE WHEN status IN ('closed','resolved') THEN 1 ELSE 0 END) as closed, "+
			"COALESCE(AVG(EXTRACT(EPOCH FROM (COALESCE(resolved_at, 'now') - created_at)))/3600, 0) as avg_hours").
		Where("tenant_id = ? AND product_id = ? AND created_at BETWEEN ? AND ?", tenantID, productID, startDate, endDate).
		Scan(&ticketStats)
	report.TotalTickets = ticketStats.Total
	report.OpenTickets = ticketStats.Open
	report.ClosedTickets = ticketStats.Closed
	if ticketStats.Total > 0 {
		report.AvgResolutionHours = ticketStats.AvgHours
	}

	// AI 诊断会话统计
	var aiStats struct {
		Sessions      int64
		Resolved      int64
		Escalated     int64
		AvgConfidence float64
	}
	db.Model(&models.DiagnosisSession{}).
		Select("COUNT(*) as sessions, "+
			"SUM(CASE WHEN status = 'resolved' THEN 1 ELSE 0 END) as resolved, "+
			"SUM(CASE WHEN status = 'escalated' THEN 1 ELSE 0 END) as escalated, "+
			"COALESCE(AVG(confidence_score), 0) as avg_confidence").
		Where("tenant_id = ? AND product_id = ? AND created_at BETWEEN ? AND ?", tenantID, productID, startDate, endDate).
		Scan(&aiStats)
	report.AISessions = aiStats.Sessions
	if aiStats.Sessions > 0 {
		report.AIResolveRate = float64(aiStats.Resolved) / float64(aiStats.Sessions) * 100
		report.EscalationRate = float64(aiStats.Escalated) / float64(aiStats.Sessions) * 100
	} else {
		report.EscalationRate = float64(aiStats.Escalated) / float64(aiStats.Sessions+aiStats.Resolved+1) * 100
	}
	report.AvgConfidence = aiStats.AvgConfidence * 100

	// 知识条目数只统计当前租户明确绑定到当前产品的知识库。
	knowledgeBaseIDs, err := reportProductKnowledgeBaseIDs(db, tenantID, productID)
	if err != nil {
		return nil, fmt.Errorf("load product knowledge scope: %w", err)
	}
	if len(knowledgeBaseIDs) > 0 {
		var docCount, faqCount int64
		if err := db.Model(&models.KnowledgeDocument{}).
			Where("tenant_id = ? AND knowledge_base_id IN ? AND status = ?", tenantID, knowledgeBaseIDs, enums.StatusOk).
			Count(&docCount).Error; err != nil {
			return nil, fmt.Errorf("count product knowledge documents: %w", err)
		}
		if err := db.Model(&models.KnowledgeFAQ{}).
			Where("tenant_id = ? AND knowledge_base_id IN ? AND status = ?", tenantID, knowledgeBaseIDs, enums.StatusOk).
			Count(&faqCount).Error; err != nil {
			return nil, fmt.Errorf("count product knowledge faqs: %w", err)
		}
		report.KnowledgeEntries = docCount + faqCount
	}

	// 知识命中率通过产品工单/诊断会话反查，避免共享知识库造成跨产品污染。
	var hitStats struct {
		Total int64
		Hits  int64
	}
	conversationIDs, err := reportProductConversationIDs(db, tenantID, productID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("load product conversation scope: %w", err)
	}
	if len(conversationIDs) > 0 {
		if err := db.Model(&models.KnowledgeRetrieveLog{}).
			Select("COUNT(*) as total, "+
				"SUM(CASE WHEN hit_count > 0 THEN 1 ELSE 0 END) as hits").
			Where("tenant_id = ? AND conversation_id IN ? AND created_at BETWEEN ? AND ?", tenantID, conversationIDs, startDate, endDate).
			Scan(&hitStats).Error; err != nil {
			return nil, fmt.Errorf("aggregate product knowledge retrieval: %w", err)
		}
	}
	if hitStats.Total > 0 {
		report.KnowledgeHitRate = float64(hitStats.Hits) / float64(hitStats.Total) * 100
	}

	// 会议统计
	report.TotalMeetings = s.countProductMeetings(db, tenantID, productID, startDate, endDate)
	report.TotalMeetingMinutes = s.sumMeetingMinutes(db, tenantID, productID, startDate, endDate)

	// 用量统计
	var usageStats struct {
		TokensIn  int64
		TokensOut int64
		Cost      float64
	}
	db.Model(&models.UsageEvent{}).
		Select("COALESCE(SUM(tokens_in), 0) as tokens_in, "+
			"COALESCE(SUM(tokens_out), 0) as tokens_out, "+
			"COALESCE(SUM(cost), 0) as cost").
		Where("tenant_id = ? AND product_id = ? AND created_at BETWEEN ? AND ?", tenantID, productID, startDate, endDate).
		Scan(&usageStats)
	report.TotalTokensIn = usageStats.TokensIn
	report.TotalTokensOut = usageStats.TokensOut
	report.EstimatedCost = usageStats.Cost

	purchasingMetrics, err := loadPurchasingMetricAggregate(db, tenantID, productID, &startDate, &endDate)
	if err != nil {
		return nil, fmt.Errorf("aggregate product service outcomes: %w", err)
	}
	applyPurchasingMetricsToProductReport(report, purchasingMetrics)

	return report, nil
}

// GetDashboardOverview 获取运营概览
func (s *reportService) GetDashboardOverview(ctx interface{}, tenantID string) (*DashboardOverview, error) {
	db := sqls.DB()
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekAgo := todayStart.AddDate(0, 0, -7)

	overview := &DashboardOverview{}

	// 并行聚合,避免 12+ 条顺序 COUNT 串行叠加
	var (
		resolvedSessions int64
		totalSessions    int64
		recentTickets    []models.Ticket
		queueTickets     []models.Ticket
	)
	var wg sync.WaitGroup
	run := func(fn func()) {
		wg.Add(1)
		go func() { defer wg.Done(); fn() }()
	}

	run(func() {
		purchasingMetrics, err := loadPurchasingMetricAggregate(db, tenantID, "", nil, nil)
		if err != nil {
			slog.Warn("dashboard overview purchasing metrics", "tenant_id", tenantID, "error", err)
			return
		}
		applyPurchasingMetricsToDashboardOverview(overview, purchasingMetrics)
	})
	run(func() { // 产品数
		db.Model(&models.Product{}).Where("tenant_id = ?", tenantID).Count(&overview.TotalProducts)
	})
	run(func() { // 设备数
		db.Model(&models.Device{}).Where("tenant_id = ?", tenantID).Count(&overview.TotalDevices)
	})
	run(func() { // 工单总数
		db.Model(&models.Ticket{}).Where("tenant_id = ?", tenantID).Count(&overview.TotalTickets)
	})
	run(func() { // 待处理工单
		db.Model(&models.Ticket{}).Where("tenant_id = ? AND status IN ?", tenantID, enterpriseStatusFilterToDB("pending")).Count(&overview.PendingTickets)
	})
	run(func() { // 处理中工单
		db.Model(&models.Ticket{}).Where("tenant_id = ? AND status IN ?", tenantID, enterpriseStatusFilterToDB("processing")).Count(&overview.InProgressTickets)
	})
	run(func() { // SLA 风险工单
		db.Model(&models.Ticket{}).Where("tenant_id = ? AND sla_due_at IS NOT NULL AND sla_due_at < ? AND status NOT IN ?", tenantID, now, enterpriseTicketCompletedStatuses()).Count(&overview.SLAAtRisk)
	})
	run(func() { // 今日新增工单
		db.Model(&models.Ticket{}).Where("tenant_id = ? AND created_at >= ?", tenantID, todayStart).Count(&overview.NewToday)
	})
	run(func() { // AI 会话数
		db.Model(&models.DiagnosisSession{}).Where("tenant_id = ? AND created_at >= ?", tenantID, todayStart).Count(&overview.AISessions)
	})
	run(func() { // AI 会话总数
		db.Model(&models.DiagnosisSession{}).Where("tenant_id = ? AND created_at >= ?", tenantID, todayStart).Count(&totalSessions)
	})
	run(func() { // AI 已解决会话数
		db.Model(&models.DiagnosisSession{}).Where("tenant_id = ? AND status = 'resolved' AND created_at >= ?", tenantID, todayStart).Count(&resolvedSessions)
	})
	run(func() { // 最近活动（近 7 天工单）
		db.Where("tenant_id = ? AND created_at >= ?", tenantID, weekAgo).
			Order("created_at desc").
			Limit(10).
			Find(&recentTickets)
	})
	run(func() { // 待办队列工单（Top 6）
		db.Where("tenant_id = ? AND status NOT IN ?", tenantID, enterpriseTicketCompletedStatuses()).
			Order("created_at desc").
			Limit(6).
			Find(&queueTickets)
	})
	wg.Wait()

	if totalSessions > 0 {
		overview.AIResolveRate = float64(resolvedSessions) / float64(totalSessions) * 100
	}

	overview.RecentActivities = make([]Activity, 0, len(recentTickets))
	for _, t := range recentTickets {
		overview.RecentActivities = append(overview.RecentActivities, Activity{
			Type:      "ticket",
			Message:   fmt.Sprintf("新工单 %s %s", t.TicketNo, t.Title),
			CreatedAt: t.CreatedAt,
		})
	}

	overview.QueueTickets = make([]QueueTicket, 0, len(queueTickets))

	// 批量预取客户与负责人名称,避免逐条 N+1 查询
	customerIDs := make([]int64, 0, len(queueTickets))
	assigneeIDs := make([]int64, 0, len(queueTickets))
	for _, t := range queueTickets {
		if t.CustomerID > 0 {
			customerIDs = append(customerIDs, t.CustomerID)
		}
		if t.CurrentAssigneeID > 0 {
			assigneeIDs = append(assigneeIDs, t.CurrentAssigneeID)
		}
	}
	customerNames := map[int64]string{}
	if len(customerIDs) > 0 {
		var customers []models.Customer
		db.Select("id", "name").Where("id IN ?", customerIDs).Find(&customers)
		for _, c := range customers {
			customerNames[c.ID] = c.Name
		}
	}
	assigneeNames := map[int64]string{}
	if len(assigneeIDs) > 0 {
		var users []models.User
		db.Select("id", "nickname").Where("id IN ?", assigneeIDs).Find(&users)
		for _, u := range users {
			assigneeNames[u.ID] = u.Nickname
		}
	}

	for _, t := range queueTickets {
		qt := QueueTicket{
			TicketNo: t.TicketNo,
			Summary:  t.Title,
			State:    string(t.Status),
		}
		// 判断紧急级别
		if t.SLADueAt != nil && t.SLADueAt.Before(now.Add(2*time.Hour)) {
			qt.Level = "紧急"
		}
		// 映射状态文本
		switch t.Status {
		case "pending":
			qt.State = "待处理"
			qt.Label = "处理"
		case "in_progress":
			qt.State = "处理中"
			qt.Label = "进入"
		case "assigned":
			qt.State = "待派单"
			qt.Label = "派单"
		case "waiting_customer":
			qt.State = "处理中"
			qt.Label = "跟进"
		default:
			qt.State = "处理中"
			qt.Label = "进入"
		}
		if name, ok := customerNames[t.CustomerID]; ok {
			qt.Customer = name
		}
		if name, ok := assigneeNames[t.CurrentAssigneeID]; ok {
			qt.Owner = name
		}
		overview.QueueTickets = append(overview.QueueTickets, qt)
	}

	return overview, nil
}

// GetTrendData 获取趋势数据（按天）
func (s *reportService) GetTrendData(ctx interface{}, tenantID, productID, metric string, startDate, endDate time.Time) ([]TrendPoint, error) {
	db := sqls.DB()

	series := make(map[string]*TrendPoint)
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		series[key] = &TrendPoint{Date: key}
	}

	switch metric {
	case "tickets":
		type row struct {
			DateKey string
			Count   int64
		}
		var rows []row
		query := db.Model(&models.Ticket{}).
			Select("DATE(created_at) as date_key, COUNT(*) as count").
			Where("tenant_id = ? AND created_at BETWEEN ? AND ?", tenantID, startDate, endDate)
		if productID != "" {
			query = query.Where("product_id = ?", productID)
		}
		query.Group("DATE(created_at)").Order("date_key asc").Scan(&rows)
		for _, r := range rows {
			if p, ok := series[r.DateKey]; ok {
				p.Value = float64(r.Count)
				p.Label = "工单数"
			}
		}

	case "ai_sessions":
		type row struct {
			DateKey string
			Count   int64
		}
		var rows []row
		query := db.Model(&models.DiagnosisSession{}).
			Select("DATE(created_at) as date_key, COUNT(*) as count").
			Where("tenant_id = ? AND created_at BETWEEN ? AND ?", tenantID, startDate, endDate)
		if productID != "" {
			query = query.Where("product_id = ?", productID)
		}
		query.Group("DATE(created_at)").Order("date_key asc").Scan(&rows)
		for _, r := range rows {
			if p, ok := series[r.DateKey]; ok {
				p.Value = float64(r.Count)
				p.Label = "AI诊断会话数"
			}
		}

	case "tokens":
		type row struct {
			DateKey string
			Tokens  int64
		}
		var rows []row
		query := db.Model(&models.UsageEvent{}).
			Select("DATE(created_at) as date_key, COALESCE(SUM(tokens_in + tokens_out), 0) as tokens").
			Where("tenant_id = ? AND created_at BETWEEN ? AND ?", tenantID, startDate, endDate)
		if productID != "" {
			query = query.Where("product_id = ?", productID)
		}
		query.Group("DATE(created_at)").Order("date_key asc").Scan(&rows)
		for _, r := range rows {
			if p, ok := series[r.DateKey]; ok {
				p.Value = float64(r.Tokens)
				p.Label = "Token用量"
			}
		}

	case "satisfaction":
		// 使用工单关闭率作为满意度代理指标
		type row struct {
			DateKey  string
			Resolved int64
			Total    int64
		}
		var rows []row
		query := db.Model(&models.Ticket{}).
			Select("DATE(created_at) as date_key, "+
				"SUM(CASE WHEN status IN ('closed','resolved') THEN 1 ELSE 0 END) as resolved, "+
				"COUNT(*) as total").
			Where("tenant_id = ? AND created_at BETWEEN ? AND ?", tenantID, startDate, endDate)
		if productID != "" {
			query = query.Where("product_id = ?", productID)
		}
		query.Group("DATE(created_at)").Order("date_key asc").Scan(&rows)
		for _, r := range rows {
			if p, ok := series[r.DateKey]; ok {
				if r.Total > 0 {
					p.Value = float64(r.Resolved) / float64(r.Total) * 100
				}
				p.Label = "解决率(%)"
			}
		}

	default:
		// 默认返回工单趋势
		type row struct {
			DateKey string
			Count   int64
		}
		var rows []row
		query := db.Model(&models.Ticket{}).
			Select("DATE(created_at) as date_key, COUNT(*) as count").
			Where("tenant_id = ? AND created_at BETWEEN ? AND ?", tenantID, startDate, endDate)
		if productID != "" {
			query = query.Where("product_id = ?", productID)
		}
		query.Group("DATE(created_at)").Order("date_key asc").Scan(&rows)
		for _, r := range rows {
			if p, ok := series[r.DateKey]; ok {
				p.Value = float64(r.Count)
				p.Label = "工单数"
			}
		}
	}

	result := make([]TrendPoint, 0, len(series))
	keys := make([]string, 0, len(series))
	for k := range series {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		result = append(result, *series[k])
	}
	return result, nil
}

// GetTopFailureProducts 获取故障最多的产品排行
func (s *reportService) GetTopFailureProducts(ctx interface{}, tenantID string, limit int) ([]ProductFailureRank, error) {
	db := sqls.DB()
	if limit <= 0 {
		limit = 10
	}

	type row struct {
		ProductID      string
		FailureCount   int64
		EscalatedCount int64
	}
	var rows []row
	db.Model(&models.Ticket{}).
		Select("product_id, COUNT(*) as failure_count, "+
			"SUM(CASE WHEN source = 'ai_escalation' OR source = 'manual' THEN 1 ELSE 0 END) as escalated_count").
		Where("tenant_id = ?", tenantID).
		Group("product_id").
		Order("failure_count desc").
		Limit(limit).
		Scan(&rows)

	result := make([]ProductFailureRank, 0, len(rows))
	for _, r := range rows {
		var product models.Product
		productName := ""
		if err := db.Select("name").Where("id = ?", r.ProductID).First(&product).Error; err == nil {
			productName = product.Name
		}

		rank := ProductFailureRank{
			ProductID:    r.ProductID,
			ProductName:  productName,
			FailureCount: r.FailureCount,
		}
		if r.FailureCount > 0 {
			rank.EscalationRate = float64(r.EscalatedCount) / float64(r.FailureCount) * 100
		}

		// 计算平均解决时长
		var avgHours float64
		db.Model(&models.Ticket{}).
			Select("COALESCE(AVG(EXTRACT(EPOCH FROM (COALESCE(resolved_at, 'now') - created_at)))/3600, 0)").
			Where("tenant_id = ? AND product_id = ? AND resolved_at IS NOT NULL", tenantID, r.ProductID).
			Scan(&avgHours)
		rank.AvgResolutionHrs = math.Round(avgHours*100) / 100

		result = append(result, rank)
	}

	return result, nil
}

// GetTeamPerformance 获取坐席绩效
func (s *reportService) GetTeamPerformance(ctx interface{}, tenantID string, startDate, endDate time.Time) ([]AgentPerformance, error) {
	db := sqls.DB()

	type agentStat struct {
		AssigneeID int64
		Resolved   int64
		Assigned   int64
	}
	var stats []agentStat
	db.Model(&models.Ticket{}).
		Select("current_assignee_id as assignee_id, "+
			"SUM(CASE WHEN status IN ('closed','resolved') THEN 1 ELSE 0 END) as resolved, "+
			"COUNT(*) as assigned").
		Where("tenant_id = ? AND created_at BETWEEN ? AND ?", tenantID, startDate, endDate).
		Where("current_assignee_id > 0").
		Group("current_assignee_id").
		Order("resolved desc").
		Scan(&stats)

	result := make([]AgentPerformance, 0, len(stats))
	for _, st := range stats {
		perf := AgentPerformance{
			AgentID:         int642Str(st.AssigneeID),
			TicketsResolved: st.Resolved,
			TicketsAssigned: st.Assigned,
		}

		// 获取客服名称
		var user models.User
		if err := db.Select("nickname").Where("id = ?", st.AssigneeID).First(&user).Error; err == nil {
			perf.AgentName = user.Nickname
		}

		perf.AvgResponseMin, perf.AvgHandleMin, perf.Satisfaction, perf.MeetingsHeld = reportAssigneeMetrics(db, tenantID, st.AssigneeID, startDate, endDate)

		result = append(result, perf)
	}

	return result, nil
}

func (s *reportService) GetSupplierPerformance(tenantID int64, startDate, endDate time.Time) ([]SupplierPerformanceAggregate, error) {
	db := sqls.DB()
	collaborations, err := repositories.TicketSupplierCollaborationRepository.FindForPerformance(db, tenantID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	if len(collaborations) == 0 {
		return make([]SupplierPerformanceAggregate, 0), nil
	}

	supplierIDs := make([]int64, 0, len(collaborations))
	ticketIDs := make([]int64, 0, len(collaborations))
	assignments := make(map[[2]int64]*supplierTicketPerformance)
	for i := range collaborations {
		item := collaborations[i]
		supplierIDs = append(supplierIDs, item.PartnerCompanyID)
		ticketIDs = append(ticketIDs, item.TicketID)
		key := [2]int64{item.PartnerCompanyID, item.TicketID}
		assignment := assignments[key]
		if assignment == nil {
			assignment = &supplierTicketPerformance{
				SupplierID: item.PartnerCompanyID,
				TicketID:   item.TicketID,
				InvitedAt:  item.InvitedAt,
			}
			assignments[key] = assignment
		}
		if item.InvitedAt.Before(assignment.InvitedAt) {
			assignment.InvitedAt = item.InvitedAt
		}
		if item.AcceptedAt != nil && (assignment.AcceptedAt == nil || item.AcceptedAt.Before(*assignment.AcceptedAt)) {
			acceptedAt := *item.AcceptedAt
			assignment.AcceptedAt = &acceptedAt
		}
		if item.Status == SupplierCollaborationResolved {
			assignment.HasResolvedWork = true
		} else {
			assignment.HasActiveWork = true
		}
	}

	companies, err := repositories.EnterpriseIAMRepository.FindPartnerCompaniesByIDs(db, tenantID, supplierIDs)
	if err != nil {
		return nil, err
	}
	feedbackItems, err := repositories.TicketFeedbackRepository.FindByTicketIDs(db, tenantID, ticketIDs)
	if err != nil {
		return nil, err
	}
	latestRatingByTicket := make(map[int64]int)
	for i := range feedbackItems {
		feedback := feedbackItems[i]
		if _, exists := latestRatingByTicket[feedback.TicketID]; !exists {
			latestRatingByTicket[feedback.TicketID] = feedback.Rating
		}
	}

	bySupplier := make(map[int64]*supplierPerformanceAccumulator)
	for _, assignment := range assignments {
		acc := bySupplier[assignment.SupplierID]
		if acc == nil {
			acc = &supplierPerformanceAccumulator{Aggregate: SupplierPerformanceAggregate{SupplierID: assignment.SupplierID}}
			if company := companies[assignment.SupplierID]; company != nil {
				acc.Aggregate.SupplierName = company.Name
				acc.Aggregate.SupplierNo = company.PartnerNo
			}
			bySupplier[assignment.SupplierID] = acc
		}
		acc.Aggregate.TicketsAssigned++
		if assignment.HasActiveWork {
			acc.Aggregate.TicketsProcessing++
		} else if assignment.HasResolvedWork {
			acc.Aggregate.TicketsCompleted++
		}
		if assignment.AcceptedAt != nil && !assignment.AcceptedAt.Before(assignment.InvitedAt) {
			acc.Aggregate.TicketsResponded++
			acc.ResponseMinute += assignment.AcceptedAt.Sub(assignment.InvitedAt).Minutes()
		}
		if rating, ok := latestRatingByTicket[assignment.TicketID]; ok && rating >= 1 && rating <= 5 {
			acc.Aggregate.RatingCount++
			acc.RatingTotal += float64(rating)
		}
	}

	result := make([]SupplierPerformanceAggregate, 0, len(bySupplier))
	for _, acc := range bySupplier {
		if acc.Aggregate.TicketsResponded > 0 {
			acc.Aggregate.AvgResponseMinutes = roundReportValue(acc.ResponseMinute / float64(acc.Aggregate.TicketsResponded))
		}
		if acc.Aggregate.TicketsAssigned > 0 {
			acc.Aggregate.CompletionRate = roundReportValue(float64(acc.Aggregate.TicketsCompleted) / float64(acc.Aggregate.TicketsAssigned) * 100)
		}
		if acc.Aggregate.RatingCount > 0 {
			acc.Aggregate.AverageRating = roundReportValue(acc.RatingTotal / float64(acc.Aggregate.RatingCount))
		}
		result = append(result, acc.Aggregate)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].CompletionRate != result[j].CompletionRate {
			return result[i].CompletionRate > result[j].CompletionRate
		}
		if result[i].AverageRating != result[j].AverageRating {
			return result[i].AverageRating > result[j].AverageRating
		}
		if result[i].TicketsAssigned != result[j].TicketsAssigned {
			return result[i].TicketsAssigned > result[j].TicketsAssigned
		}
		return result[i].SupplierName < result[j].SupplierName
	})
	return result, nil
}

func roundReportValue(value float64) float64 {
	return math.Round(value*100) / 100
}

type purchasingMetricAggregate struct {
	EligibleTickets      int64 `gorm:"-"`
	TotalFacts           int64 `gorm:"column:total_facts"`
	ExpertKnown          int64 `gorm:"column:expert_known"`
	ExpertInterventions  int64 `gorm:"column:expert_interventions"`
	AvoidedTripsKnown    int64 `gorm:"column:avoided_trips_known"`
	AvoidedTrips         int64 `gorm:"column:avoided_trips"`
	FirstTimeFixEligible int64 `gorm:"column:first_time_fix_eligible"`
	FirstTimeFixes       int64 `gorm:"column:first_time_fixes"`
	DowntimeKnown        int64 `gorm:"column:downtime_known"`
	DowntimeMinutes      int64 `gorm:"column:downtime_minutes"`
	KnowledgeKnown       int64 `gorm:"column:knowledge_known"`
	KnowledgeReused      int64 `gorm:"column:knowledge_reused"`
	ResolutionKnown      int64 `gorm:"column:resolution_known"`
	RemoteResolutions    int64 `gorm:"column:remote_resolutions"`
}

func loadPurchasingMetricAggregate(db *gorm.DB, tenantID, productID string, startDate, endDate *time.Time) (purchasingMetricAggregate, error) {
	var aggregate purchasingMetricAggregate
	eligibleTicketScope := func() *gorm.DB {
		query := db.Model(&models.Ticket{}).
			Where("tenant_id = ? AND status IN ?", tenantID, []enums.TicketStatus{
				enums.TicketStatusResolved,
				enums.TicketStatusClosed,
				enums.TicketStatusDone,
			})
		if productID != "" {
			query = query.Where("product_id = ?", productID)
		}
		if startDate != nil && endDate != nil {
			query = query.Where("created_at BETWEEN ? AND ?", *startDate, *endDate)
		}
		return query
	}
	query := db.Model(&models.TicketServiceOutcomeFact{}).
		Select("COUNT(*) AS total_facts, "+
			"COALESCE(SUM(CASE WHEN expert_intervention_known THEN 1 ELSE 0 END), 0) AS expert_known, "+
			"COALESCE(SUM(CASE WHEN expert_intervention_known AND expert_intervened THEN 1 ELSE 0 END), 0) AS expert_interventions, "+
			"COALESCE(SUM(CASE WHEN avoided_onsite_visit_known THEN 1 ELSE 0 END), 0) AS avoided_trips_known, "+
			"COALESCE(SUM(CASE WHEN avoided_onsite_visit_known AND avoided_onsite_visit THEN 1 ELSE 0 END), 0) AS avoided_trips, "+
			"COALESCE(SUM(CASE WHEN first_time_fix_known THEN 1 ELSE 0 END), 0) AS first_time_fix_eligible, "+
			"COALESCE(SUM(CASE WHEN first_time_fix_known AND first_time_fix THEN 1 ELSE 0 END), 0) AS first_time_fixes, "+
			"COALESCE(SUM(CASE WHEN downtime_known THEN 1 ELSE 0 END), 0) AS downtime_known, "+
			"COALESCE(SUM(CASE WHEN downtime_known THEN downtime_minutes ELSE 0 END), 0) AS downtime_minutes, "+
			"COALESCE(SUM(CASE WHEN knowledge_reuse_known THEN 1 ELSE 0 END), 0) AS knowledge_known, "+
			"COALESCE(SUM(CASE WHEN knowledge_reuse_known AND knowledge_reused THEN 1 ELSE 0 END), 0) AS knowledge_reused, "+
			"COALESCE(SUM(CASE WHEN resolution_known THEN 1 ELSE 0 END), 0) AS resolution_known, "+
			"COALESCE(SUM(CASE WHEN resolution_known AND remote_resolved THEN 1 ELSE 0 END), 0) AS remote_resolutions").
		Where("tenant_id = ? AND metric_version = ?", tenantID, models.ServiceOutcomeMetricVersionV1).
		Where("ticket_id IN (?)", eligibleTicketScope().Select("id"))
	if productID != "" {
		query = query.Where("product_id = ?", productID)
	}
	if startDate != nil && endDate != nil {
		query = query.Where("ticket_created_at BETWEEN ? AND ?", *startDate, *endDate)
	}
	if err := query.Scan(&aggregate).Error; err != nil {
		return purchasingMetricAggregate{}, err
	}
	if err := eligibleTicketScope().Count(&aggregate.EligibleTickets).Error; err != nil {
		return purchasingMetricAggregate{}, err
	}
	return aggregate, nil
}

func applyPurchasingMetricsToProductReport(report *ProductReport, aggregate purchasingMetricAggregate) {
	if report == nil {
		return
	}
	report.OutcomeMetricSampleSize = aggregate.TotalFacts
	if aggregate.EligibleTickets > 0 {
		report.OutcomeMetricCoverageRate = reportFloat64(roundReportValue(float64(aggregate.TotalFacts) / float64(aggregate.EligibleTickets) * 100))
	}
	if aggregate.ExpertKnown > 0 {
		report.ExpertInterventionRate = reportFloat64(roundReportValue(float64(aggregate.ExpertInterventions) / float64(aggregate.ExpertKnown) * 100))
	}
	if aggregate.AvoidedTripsKnown > 0 {
		report.AvoidedTrips = reportInt64(aggregate.AvoidedTrips)
	}
	if aggregate.KnowledgeKnown > 0 {
		report.KnowledgeReuseRate = reportFloat64(roundReportValue(float64(aggregate.KnowledgeReused) / float64(aggregate.KnowledgeKnown) * 100))
	}
	if aggregate.FirstTimeFixEligible > 0 {
		report.FirstTimeFixRate = reportFloat64(roundReportValue(float64(aggregate.FirstTimeFixes) / float64(aggregate.FirstTimeFixEligible) * 100))
	}
	if aggregate.DowntimeKnown > 0 {
		report.AvgDowntimeMinutes = reportFloat64(roundReportValue(float64(aggregate.DowntimeMinutes) / float64(aggregate.DowntimeKnown)))
	}
	if aggregate.ResolutionKnown > 0 {
		report.RemoteResolutionRate = reportFloat64(roundReportValue(float64(aggregate.RemoteResolutions) / float64(aggregate.ResolutionKnown) * 100))
	}
}

func applyPurchasingMetricsToDashboardOverview(overview *DashboardOverview, aggregate purchasingMetricAggregate) {
	if overview == nil {
		return
	}
	overview.OutcomeMetricSampleSize = aggregate.TotalFacts
	if aggregate.EligibleTickets > 0 {
		overview.OutcomeMetricCoverageRate = reportFloat64(roundReportValue(float64(aggregate.TotalFacts) / float64(aggregate.EligibleTickets) * 100))
	}
	if aggregate.ExpertKnown > 0 {
		overview.ExpertInterventionRate = reportFloat64(roundReportValue(float64(aggregate.ExpertInterventions) / float64(aggregate.ExpertKnown) * 100))
	}
	if aggregate.AvoidedTripsKnown > 0 {
		overview.AvoidedTrips = reportInt64(aggregate.AvoidedTrips)
	}
	if aggregate.KnowledgeKnown > 0 {
		overview.KnowledgeReuseRate = reportFloat64(roundReportValue(float64(aggregate.KnowledgeReused) / float64(aggregate.KnowledgeKnown) * 100))
	}
	if aggregate.FirstTimeFixEligible > 0 {
		overview.FirstTimeFixRate = reportFloat64(roundReportValue(float64(aggregate.FirstTimeFixes) / float64(aggregate.FirstTimeFixEligible) * 100))
	}
	if aggregate.DowntimeKnown > 0 {
		overview.AvgDowntimeMinutes = reportFloat64(roundReportValue(float64(aggregate.DowntimeMinutes) / float64(aggregate.DowntimeKnown)))
	}
	if aggregate.ResolutionKnown > 0 {
		overview.RemoteResolutionRate = reportFloat64(roundReportValue(float64(aggregate.RemoteResolutions) / float64(aggregate.ResolutionKnown) * 100))
	}
}

func reportFloat64(value float64) *float64 {
	return &value
}

func reportInt64(value int64) *int64 {
	return &value
}

func reportProductKnowledgeBaseIDs(db *gorm.DB, tenantID, productID string) ([]int64, error) {
	idSet := make(map[int64]struct{})
	var bindings []models.ProductKnowledgeBinding
	if err := db.Select("knowledge_base_id").
		Where("tenant_id = ? AND product_id = ? AND status = ?", tenantID, productID, enums.StatusOk).
		Find(&bindings).Error; err != nil {
		return nil, err
	}
	for i := range bindings {
		if bindings[i].KnowledgeBaseID > 0 {
			idSet[bindings[i].KnowledgeBaseID] = struct{}{}
		}
	}
	var profiles []models.ProductServiceProfile
	if err := db.Select("default_knowledge_base_id").
		Where("tenant_id = ? AND product_id = ? AND status = ?", tenantID, productID, enums.StatusOk).
		Find(&profiles).Error; err != nil {
		return nil, err
	}
	for i := range profiles {
		if profiles[i].DefaultKnowledgeBaseID > 0 {
			idSet[profiles[i].DefaultKnowledgeBaseID] = struct{}{}
		}
	}
	var links []models.ProductKnowledgeLink
	if err := db.Select("knowledge_base_id").
		Where("tenant_id = ? AND product_id = ? AND status = ?", tenantID, productID, enums.StatusOk).
		Find(&links).Error; err != nil {
		return nil, err
	}
	for i := range links {
		if links[i].KnowledgeBaseID > 0 {
			idSet[links[i].KnowledgeBaseID] = struct{}{}
		}
	}
	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func reportProductConversationIDs(db *gorm.DB, tenantID, productID string, startDate, endDate time.Time) ([]int64, error) {
	idSet := make(map[int64]struct{})
	var ticketConversationIDs []int64
	if err := db.Model(&models.Ticket{}).Where(
		"tenant_id = ? AND product_id = ? AND conversation_id > 0 AND created_at BETWEEN ? AND ?",
		tenantID, productID, startDate, endDate,
	).Pluck("conversation_id", &ticketConversationIDs).Error; err != nil {
		return nil, err
	}
	for _, id := range ticketConversationIDs {
		idSet[id] = struct{}{}
	}
	var sessions []models.DiagnosisSession
	if err := db.Select("conversation_ref_id", "conversation_id").Where(
		"tenant_id = ? AND (product_ref_id = ? OR product_id = ?) AND created_at BETWEEN ? AND ?",
		tenantID, productID, productID, startDate, endDate,
	).Find(&sessions).Error; err != nil {
		return nil, err
	}
	for i := range sessions {
		if sessions[i].ConversationRefID > 0 {
			idSet[sessions[i].ConversationRefID] = struct{}{}
			continue
		}
		if id, err := strconv.ParseInt(strings.TrimSpace(sessions[i].ConversationID), 10, 64); err == nil && id > 0 {
			idSet[id] = struct{}{}
		}
	}
	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

// countProductMeetings 统计产品相关会议数
func reportAssigneeMetrics(db *gorm.DB, tenantID string, assigneeID int64, startDate, endDate time.Time) (float64, float64, float64, int64) {
	var tickets []models.Ticket
	if err := db.Select("id", "created_at", "handled_at", "resolved_at").
		Where("tenant_id = ? AND current_assignee_id = ? AND created_at BETWEEN ? AND ?", tenantID, assigneeID, startDate, endDate).
		Find(&tickets).Error; err != nil {
		return 0, 0, 0, 0
	}
	var responseMinutes, handleMinutes float64
	responseCount := 0
	handleCount := 0
	ticketIDs := make([]int64, 0, len(tickets))
	meetingTicketIDs := make([]string, 0, len(tickets))
	for i := range tickets {
		ticketIDs = append(ticketIDs, tickets[i].ID)
		meetingTicketIDs = append(meetingTicketIDs, strconv.FormatInt(tickets[i].ID, 10))
		if tickets[i].HandledAt != nil && !tickets[i].HandledAt.Before(tickets[i].CreatedAt) {
			responseMinutes += tickets[i].HandledAt.Sub(tickets[i].CreatedAt).Minutes()
			responseCount++
		}
		if tickets[i].ResolvedAt != nil && !tickets[i].ResolvedAt.Before(tickets[i].CreatedAt) {
			handleMinutes += tickets[i].ResolvedAt.Sub(tickets[i].CreatedAt).Minutes()
			handleCount++
		}
	}
	avgResponse := float64(0)
	if responseCount > 0 {
		avgResponse = roundReportValue(responseMinutes / float64(responseCount))
	}
	avgHandle := float64(0)
	if handleCount > 0 {
		avgHandle = roundReportValue(handleMinutes / float64(handleCount))
	}
	avgSatisfaction := float64(0)
	tenantIDValue, parseErr := strconv.ParseInt(tenantID, 10, 64)
	if parseErr == nil {
		feedbackItems, feedbackErr := repositories.TicketFeedbackRepository.FindByTicketIDs(db, tenantIDValue, ticketIDs)
		if feedbackErr == nil {
			latestRatingByTicket := make(map[int64]int, len(ticketIDs))
			for i := range feedbackItems {
				feedback := feedbackItems[i]
				if _, exists := latestRatingByTicket[feedback.TicketID]; !exists && feedback.Rating >= 1 && feedback.Rating <= 5 {
					latestRatingByTicket[feedback.TicketID] = feedback.Rating
				}
			}
			if len(latestRatingByTicket) > 0 {
				var ratingTotal int
				for _, rating := range latestRatingByTicket {
					ratingTotal += rating
				}
				avgSatisfaction = roundReportValue(float64(ratingTotal) / float64(len(latestRatingByTicket)))
			}
		}
	}
	meetingsHeld := int64(0)
	if len(meetingTicketIDs) > 0 {
		_ = db.Model(&models.MeetingRoomJitsi{}).
			Where("tenant_id = ? AND ticket_id IN ?", tenantID, meetingTicketIDs).
			Count(&meetingsHeld).Error
	}
	return avgResponse, avgHandle, avgSatisfaction, meetingsHeld
}

func reportProductTicketIDs(db *gorm.DB, tenantID, productID string) []string {
	var tickets []models.Ticket
	if err := db.Select("id").Where("tenant_id = ? AND product_id = ?", tenantID, productID).Find(&tickets).Error; err != nil {
		return nil
	}
	ids := make([]string, 0, len(tickets))
	for i := range tickets {
		ids = append(ids, strconv.FormatInt(tickets[i].ID, 10))
	}
	return ids
}

func (s *reportService) countProductMeetings(db *gorm.DB, tenantID, productID string, startDate, endDate time.Time) int64 {
	ticketIDs := reportProductTicketIDs(db, tenantID, productID)
	if len(ticketIDs) == 0 {
		return 0
	}
	var count int64
	db.Model(&models.MeetingRoomJitsi{}).
		Where("tenant_id = ? AND ticket_id IN ? AND started_at BETWEEN ? AND ?", tenantID, ticketIDs, startDate, endDate).
		Count(&count)
	return count
}

// sumMeetingMinutes 统计会议总时长（分钟）
func (s *reportService) sumMeetingMinutes(db *gorm.DB, tenantID, productID string, startDate, endDate time.Time) int64 {
	ticketIDs := reportProductTicketIDs(db, tenantID, productID)
	if len(ticketIDs) == 0 {
		return 0
	}
	var meetings []models.MeetingRoomJitsi
	if err := db.Select("id").Where("tenant_id = ? AND ticket_id IN ?", tenantID, ticketIDs).Find(&meetings).Error; err != nil || len(meetings) == 0 {
		return 0
	}
	meetingIDs := make([]string, 0, len(meetings))
	for i := range meetings {
		meetingIDs = append(meetingIDs, meetings[i].ID)
	}
	type row struct {
		Total int64
	}
	var r row
	db.Model(&models.MeetingParticipant{}).
		Select("COALESCE(SUM(duration), 0) as total").
		Where("meeting_id IN ? AND joined_at BETWEEN ? AND ?", meetingIDs, startDate, endDate).
		Scan(&r)
	return r.Total / 60
}

func int642Str(id int64) string {
	return fmt.Sprintf("%d", id)
}
