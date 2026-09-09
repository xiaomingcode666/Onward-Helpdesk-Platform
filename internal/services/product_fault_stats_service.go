package services

import (
	"fmt"
	"log/slog"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var ProductFaultStatsService = newProductFaultStatsService()

func newProductFaultStatsService() *productFaultStatsService {
	return &productFaultStatsService{}
}

type productFaultStatsService struct{}

func (s *productFaultStatsService) FindByProductAndDateRange(productID int64, startDate, endDate time.Time) []models.ProductFaultStatsDaily {
	return repositories.ProductFaultStatsRepository.FindByProductAndDateRange(sqls.DB(), productID, startDate, endDate)
}

func (s *productFaultStatsService) FindPageByCnd(cnd *sqls.Cnd) ([]models.ProductFaultStatsDaily, *sqls.Paging) {
	return repositories.ProductFaultStatsRepository.FindPageByCnd(sqls.DB(), cnd)
}

// ProjectEvent 将故障统计事件幂等投影到 ProductFaultStatsDaily（§6.2）。
// 以 event_id 做消费幂等，避免消息重试导致重复计数；
// Delta 为负数时产生补偿（重新打开、故障修订等）。
func (s *productFaultStatsService) ProjectEvent(event events.ProductFaultStatsEvent) error {
	if event.EventID == "" {
		return errorsx.InvalidParam("event id is required")
	}
	if event.TenantID <= 0 || event.ProductID <= 0 {
		// 无产品上下文的事件不参与产品故障统计。
		return nil
	}
	if repositories.FaultStatsEventInboxRepository.Exists(sqls.DB(), event.EventID) {
		return nil
	}
	delta := event.Delta
	if delta == 0 {
		delta = 1
	}
	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	bucketDate := time.Date(occurredAt.Year(), occurredAt.Month(), occurredAt.Day(), 0, 0, 0, 0, occurredAt.Location())

	stat := &models.ProductFaultStatsDaily{
		TenantID:       event.TenantID,
		ProductID:      event.ProductID,
		ProductModelID: event.ProductModelID,
		FaultCode:      event.FaultCode,
		FaultPart:      event.FaultPart,
		ModuleID:       event.ModuleID,
		BucketDate:     bucketDate,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	switch event.EventType {
	case events.FaultStatsEventTicketClosed, events.FaultStatsEventTicketReopened:
		stat.TicketCount = int64(delta)
		if event.DeviceID > 0 && delta > 0 {
			stat.AffectedDeviceCount = 1
		}
	case events.FaultStatsEventFaultConfirmed:
		stat.TicketCount = int64(delta)
	case events.FaultStatsEventCustomerRating:
		stat.LowScoreCount = int64(delta)
	case events.FaultStatsEventVideoMeeting:
		stat.MeetingCount = int64(delta)
	case events.FaultStatsEventRepairHistoryCreate:
		stat.RepeatCount = int64(delta)
	default:
		stat.TicketCount = int64(delta)
	}

	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.FaultStatsEventInboxRepository.Create(ctx.Tx, &models.FaultStatsEventInbox{
			EventID:        event.EventID,
			TenantID:       event.TenantID,
			EventType:      event.EventType,
			ProductID:      event.ProductID,
			ProductModelID: event.ProductModelID,
			DeviceID:       event.DeviceID,
			TicketID:       event.TicketID,
			PayloadJSON:    fmt.Sprintf(`{"fault_code":%q,"delta":%d}`, event.FaultCode, delta),
			ProcessedAt:    time.Now(),
			CreatedAt:      time.Now(),
		}); err != nil {
			return err
		}
		return repositories.ProductFaultStatsRepository.UpsertDaily(ctx.Tx, stat)
	})
}

// RequestRebuild 创建并异步执行故障统计历史重建任务（§6.3）。
func (s *productFaultStatsService) RequestRebuild(tenantID, productID int64, rangeValue string, operator *dto.AuthPrincipal) (*models.ProductFaultStatsRebuildJob, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	product := repositories.ProductRepository.GetByTenant(sqls.DB(), productID, tenantID)
	if product == nil {
		return nil, errorsx.InvalidParam("product not found")
	}
	days := 180
	switch rangeValue {
	case "30d":
		days = 30
	case "90d":
		days = 90
	case "180d", "":
		days = 180
	}
	now := time.Now()
	job := &models.ProductFaultStatsRebuildJob{
		TenantID:      tenantID,
		ProductID:     productID,
		Status:        "pending",
		RangeDays:     days,
		RequestedByID: operator.UserID,
		AuditFields: models.AuditFields{
			CreatedAt:      now,
			UpdatedAt:      now,
			CreateUserID:   operator.UserID,
			CreateUserName: operator.Username,
		},
	}
	if err := repositories.ProductFaultStatsRebuildJobRepository.Create(sqls.DB(), job); err != nil {
		return nil, err
	}
	go func(jobID int64) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("fault stats rebuild panic", "job_id", jobID, "recover", r)
			}
		}()
		if err := s.ExecuteRebuild(jobID); err != nil {
			slog.Error("fault stats rebuild failed", "job_id", jobID, "error", err)
		}
	}(job.ID)
	return job, nil
}

// ExecuteRebuild 从工单/维修事实表重建产品故障统计（先清空范围内投影再重算，天然幂等）。
func (s *productFaultStatsService) ExecuteRebuild(jobID int64) error {
	job := repositories.ProductFaultStatsRebuildJobRepository.Get(sqls.DB(), jobID)
	if job == nil {
		return errorsx.InvalidParam("rebuild job not found")
	}
	now := time.Now()
	_ = repositories.ProductFaultStatsRebuildJobRepository.Updates(sqls.DB(), jobID, map[string]any{
		"status":     "running",
		"started_at": now,
		"updated_at": now,
	})

	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -job.RangeDays)

	processed, err := s.recompute(job.TenantID, job.ProductID, startDate, endDate)
	finished := time.Now()
	if err != nil {
		_ = repositories.ProductFaultStatsRebuildJobRepository.Updates(sqls.DB(), jobID, map[string]any{
			"status":          "failed",
			"error_summary":   truncateErrorSummary(err.Error()),
			"processed_count": processed,
			"finished_at":     finished,
			"updated_at":      finished,
		})
		return err
	}
	_ = repositories.ProductFaultStatsRebuildJobRepository.Updates(sqls.DB(), jobID, map[string]any{
		"status":          "succeeded",
		"processed_count": processed,
		"error_summary":   "",
		"finished_at":     finished,
		"updated_at":      finished,
	})
	return nil
}

// recompute 清空范围内投影后，基于工单事实重新投影。
func (s *productFaultStatsService) recompute(tenantID, productID int64, startDate, endDate time.Time) (int64, error) {
	if err := repositories.ProductFaultStatsRepository.DeleteByTenantProductAndDateRange(sqls.DB(), tenantID, productID, startDate, endDate); err != nil {
		return 0, err
	}
	tickets := repositories.TicketRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Where("created_at >= ?", startDate).
		Where("created_at <= ?", endDate))
	var processed int64
	for _, ticket := range tickets {
		event := events.ProductFaultStatsEvent{
			EventID:        fmt.Sprintf("rebuild:%d:ticket:%d", productID, ticket.ID),
			EventType:      events.FaultStatsEventTicketClosed,
			TenantID:       tenantID,
			ProductID:      productID,
			ProductModelID: ticket.ProductModelID,
			DeviceID:       ticket.DeviceID,
			TicketID:       ticket.ID,
			FaultCode:      ticket.FaultCode,
			OccurredAt:     ticket.CreatedAt,
			Delta:          1,
		}
		if err := s.ProjectEvent(event); err != nil {
			return processed, err
		}
		processed++
	}
	// 维修记录维度补充 repeat_count。
	records := repositories.DeviceServiceRecordRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Where("occurred_at >= ?", startDate).
		Where("occurred_at <= ?", endDate))
	for _, record := range records {
		event := events.ProductFaultStatsEvent{
			EventID:        fmt.Sprintf("rebuild:%d:repair:%d", productID, record.ID),
			EventType:      events.FaultStatsEventRepairHistoryCreate,
			TenantID:       tenantID,
			ProductID:      productID,
			ProductModelID: record.ProductModelID,
			DeviceID:       record.DeviceID,
			TicketID:       record.TicketID,
			OccurredAt:     record.OccurredAt,
			Delta:          1,
		}
		if err := s.ProjectEvent(event); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

// GetRebuildJob 查询重建任务（租户+产品隔离）。
func (s *productFaultStatsService) GetRebuildJob(tenantID, productID, jobID int64) (*models.ProductFaultStatsRebuildJob, error) {
	job := repositories.ProductFaultStatsRebuildJobRepository.Get(sqls.DB(), jobID)
	if job == nil || job.TenantID != tenantID || job.ProductID != productID {
		return nil, errorsx.InvalidParam("rebuild job not found")
	}
	return job, nil
}

// BuildJobDTO 重建任务映射为 DTO。
func (s *productFaultStatsService) BuildJobDTO(job *models.ProductFaultStatsRebuildJob) dto.ProductFaultStatsRebuildJobDTO {
	if job == nil {
		return dto.ProductFaultStatsRebuildJobDTO{}
	}
	return dto.ProductFaultStatsRebuildJobDTO{
		ID:             job.ID,
		ProductID:      job.ProductID,
		Status:         job.Status,
		RangeDays:      job.RangeDays,
		ProcessedCount: job.ProcessedCount,
		ErrorSummary:   job.ErrorSummary,
		StartedAt:      formatEnterpriseTimePtr(job.StartedAt),
		FinishedAt:     formatEnterpriseTimePtr(job.FinishedAt),
		CreatedAt:      formatEnterpriseTime(job.CreatedAt),
	}
}

// LatestRebuildFailed 判断最近一次重建是否失败（供 data_status 使用）。
func (s *productFaultStatsService) LatestRebuildFailed(tenantID, productID int64) bool {
	jobs := repositories.ProductFaultStatsRebuildJobRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).Eq("product_id", productID).Desc("id").Limit(1))
	return len(jobs) > 0 && jobs[0].Status == "failed"
}

// CountTicketsInRange 统计范围内工单事实数量（供 data_status 区分 empty/building）。
func (s *productFaultStatsService) CountTicketsInRange(tenantID, productID int64, startDate, endDate time.Time) int64 {
	return repositories.TicketRepository.Count(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Where("created_at >= ?", startDate).
		Where("created_at <= ?", endDate))
}

// IncrementFaultStat creates or increments a daily fault stat counter.
func (s *productFaultStatsService) IncrementFaultStat(tenantID, productID, productModelID int64, faultCode, faultPart string, moduleID int64, bucketDate time.Time) error {
	stat := &models.ProductFaultStatsDaily{
		TenantID:       tenantID,
		ProductID:      productID,
		ProductModelID: productModelID,
		FaultCode:      faultCode,
		FaultPart:      faultPart,
		ModuleID:       moduleID,
		BucketDate:     bucketDate,
		TicketCount:    1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	return repositories.ProductFaultStatsRepository.UpsertDaily(sqls.DB(), stat)
}

// IncrementTicketCount increments the ticket count for a given fault stat.
func (s *productFaultStatsService) IncrementTicketCount(tenantID, productID, productModelID int64, faultCode string, occurredAt time.Time) error {
	bucketDate := time.Date(occurredAt.Year(), occurredAt.Month(), occurredAt.Day(), 0, 0, 0, 0, occurredAt.Location())
	stat := &models.ProductFaultStatsDaily{
		TenantID:       tenantID,
		ProductID:      productID,
		ProductModelID: productModelID,
		FaultCode:      faultCode,
		BucketDate:     bucketDate,
		TicketCount:    1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	return repositories.ProductFaultStatsRepository.UpsertDaily(sqls.DB(), stat)
}

// IncrementRepeatCount increments the repeat count for a given fault stat.
func (s *productFaultStatsService) IncrementRepeatCount(tenantID, productID, productModelID int64, faultCode string, occurredAt time.Time) error {
	bucketDate := time.Date(occurredAt.Year(), occurredAt.Month(), occurredAt.Day(), 0, 0, 0, 0, occurredAt.Location())
	stat := &models.ProductFaultStatsDaily{
		TenantID:       tenantID,
		ProductID:      productID,
		ProductModelID: productModelID,
		FaultCode:      faultCode,
		BucketDate:     bucketDate,
		RepeatCount:    1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	return repositories.ProductFaultStatsRepository.UpsertDaily(sqls.DB(), stat)
}

// IncrementLowScoreCount increments the low score count for a given fault stat.
func (s *productFaultStatsService) IncrementLowScoreCount(tenantID, productID, productModelID int64, faultCode string, occurredAt time.Time) error {
	bucketDate := time.Date(occurredAt.Year(), occurredAt.Month(), occurredAt.Day(), 0, 0, 0, 0, occurredAt.Location())
	stat := &models.ProductFaultStatsDaily{
		TenantID:       tenantID,
		ProductID:      productID,
		ProductModelID: productModelID,
		FaultCode:      faultCode,
		BucketDate:     bucketDate,
		LowScoreCount:  1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	return repositories.ProductFaultStatsRepository.UpsertDaily(sqls.DB(), stat)
}

// IncrementMeetingCount increments the meeting count for a given fault stat.
func (s *productFaultStatsService) IncrementMeetingCount(tenantID, productID, productModelID int64, faultCode string, occurredAt time.Time) error {
	bucketDate := time.Date(occurredAt.Year(), occurredAt.Month(), occurredAt.Day(), 0, 0, 0, 0, occurredAt.Location())
	stat := &models.ProductFaultStatsDaily{
		TenantID:       tenantID,
		ProductID:      productID,
		ProductModelID: productModelID,
		FaultCode:      faultCode,
		BucketDate:     bucketDate,
		MeetingCount:   1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	return repositories.ProductFaultStatsRepository.UpsertDaily(sqls.DB(), stat)
}
