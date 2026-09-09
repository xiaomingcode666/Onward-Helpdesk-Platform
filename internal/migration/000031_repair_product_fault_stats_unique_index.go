package migration

import (
	"time"

	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const productFaultStatsUniqueIndex = "uk_product_fault_stats_daily_dimensions"

func init() {
	register(31, "repair product fault statistics unique index", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			return ensureProductFaultStatsUniqueIndex(ctx.Tx)
		})
	})
}

type productFaultStatsKey struct {
	tenantID       int64
	productID      int64
	productModelID int64
	faultCode      string
	faultPart      string
	moduleID       int64
	bucketDate     string
}

type productFaultStatsAggregate struct {
	id                  int64
	ticketCount         int64
	repeatCount         int64
	lowScoreCount       int64
	meetingCount        int64
	affectedDeviceCount int64
	hasDuplicates       bool
}

func ensureProductFaultStatsUniqueIndex(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.ProductFaultStatsDaily{}) {
		return nil
	}

	var rows []models.ProductFaultStatsDaily
	if err := db.Order("id ASC").Find(&rows).Error; err != nil {
		return err
	}

	aggregates := make(map[productFaultStatsKey]*productFaultStatsAggregate, len(rows))
	duplicateIDs := make([]int64, 0)
	for i := range rows {
		row := rows[i]
		key := productFaultStatsKey{
			tenantID:       row.TenantID,
			productID:      row.ProductID,
			productModelID: row.ProductModelID,
			faultCode:      row.FaultCode,
			faultPart:      row.FaultPart,
			moduleID:       row.ModuleID,
			bucketDate:     row.BucketDate.Format("2006-01-02"),
		}
		aggregate, exists := aggregates[key]
		if !exists {
			aggregates[key] = &productFaultStatsAggregate{
				id:                  row.ID,
				ticketCount:         row.TicketCount,
				repeatCount:         row.RepeatCount,
				lowScoreCount:       row.LowScoreCount,
				meetingCount:        row.MeetingCount,
				affectedDeviceCount: row.AffectedDeviceCount,
			}
			continue
		}
		aggregate.ticketCount += row.TicketCount
		aggregate.repeatCount += row.RepeatCount
		aggregate.lowScoreCount += row.LowScoreCount
		aggregate.meetingCount += row.MeetingCount
		aggregate.affectedDeviceCount += row.AffectedDeviceCount
		aggregate.hasDuplicates = true
		duplicateIDs = append(duplicateIDs, row.ID)
	}

	for _, aggregate := range aggregates {
		if !aggregate.hasDuplicates {
			continue
		}
		if err := db.Model(&models.ProductFaultStatsDaily{}).Where("id = ?", aggregate.id).Updates(map[string]any{
			"ticket_count":          aggregate.ticketCount,
			"repeat_count":          aggregate.repeatCount,
			"low_score_count":       aggregate.lowScoreCount,
			"meeting_count":         aggregate.meetingCount,
			"affected_device_count": aggregate.affectedDeviceCount,
			"updated_at":            time.Now(),
		}).Error; err != nil {
			return err
		}
	}
	if len(duplicateIDs) > 0 {
		if err := db.Delete(&models.ProductFaultStatsDaily{}, duplicateIDs).Error; err != nil {
			return err
		}
	}

	if db.Dialector.Name() == "postgres" || db.Dialector.Name() == "sqlite" {
		if err := db.Exec(`DROP INDEX IF EXISTS "idx_fault_stats_unique"`).Error; err != nil {
			return err
		}
		if err := db.Exec(`DROP INDEX IF EXISTS "uk_product_fault_stats_daily_dimensions"`).Error; err != nil {
			return err
		}
		return db.Exec(`CREATE UNIQUE INDEX "uk_product_fault_stats_daily_dimensions" ON "product_fault_stats_daily" ("tenant_id", "product_id", "product_model_id", "fault_code", "fault_part", "module_id", "bucket_date")`).Error
	}

	migrator := db.Migrator()
	if migrator.HasIndex(&models.ProductFaultStatsDaily{}, productFaultStatsUniqueIndex) {
		if err := migrator.DropIndex(&models.ProductFaultStatsDaily{}, productFaultStatsUniqueIndex); err != nil {
			return err
		}
	}
	return migrator.CreateIndex(&models.ProductFaultStatsDaily{}, productFaultStatsUniqueIndex)
}
