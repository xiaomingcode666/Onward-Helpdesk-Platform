package services

import (
	"fmt"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/servicecode"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

// QRTokenSuffixLength QR token 后缀长度
const QRTokenSuffixLength = 8

// BuildQrToken 构建带 QR 标识的 token（用于生成二维码内容）
func BuildQrToken(serviceCode string) string {
	return fmt.Sprintf("QR-%s-%s", serviceCode, utils.RandomSuffix(QRTokenSuffixLength))
}

// BuildQrUrl 构建扫码 URL（含 QR token）
func BuildQrUrl(baseURL, serviceCode string) string {
	if baseURL == "" {
		baseURL = "/c/"
	}
	return fmt.Sprintf("%s%s", baseURL, serviceCode)
}

func (s *serviceCodeManagementService) GenerateQRCodePNG(serviceCode string, size int) ([]byte, error) {
	return servicecode.GenerateQRCodePNG(serviceCode, size)
}

const (
	// DefaultServiceCodeExpiryDays 通用型服务码默认有效期天数
	DefaultServiceCodeExpiryDays = 180
	// MaxServiceCodeExpiryDays 通用型服务码最大有效期天数
	MaxServiceCodeExpiryDays = 365
)

var ServiceCodeManagementService = newServiceCodeManagementService()

func newServiceCodeManagementService() *serviceCodeManagementService {
	return &serviceCodeManagementService{}
}

type serviceCodeManagementService struct {
}

func (s *serviceCodeManagementService) Get(id int64) *models.ServiceCode {
	if id <= 0 {
		return nil
	}
	return repositories.ServiceCodeRepository.Get(sqls.DB(), id)
}

func (s *serviceCodeManagementService) Find(cnd *sqls.Cnd) []models.ServiceCode {
	return repositories.ServiceCodeRepository.Find(sqls.DB(), cnd)
}

func (s *serviceCodeManagementService) FindPageByCnd(cnd *sqls.Cnd) (list []models.ServiceCode, paging *sqls.Paging) {
	return repositories.ServiceCodeRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *serviceCodeManagementService) GenerateCodes(batchID int64, count int64, operator *dto.AuthPrincipal) ([]models.ServiceCode, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if batchID <= 0 {
		return nil, errorsx.InvalidParam("batchId is required")
	}
	if count <= 0 {
		return nil, errorsx.InvalidParam("count must be greater than 0")
	}

	codes := make([]models.ServiceCode, 0, count)
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		batch := repositories.ServiceCodeBatchRepository.Get(ctx.Tx, batchID)
		if batch == nil || batch.Status == enums.StatusDeleted {
			return errorsx.InvalidParam("service code batch not found")
		}
		if batch.Status != enums.StatusOk {
			return errorsx.InvalidParam("service code batch is disabled")
		}

		currentCount := repositories.ServiceCodeRepository.Count(ctx.Tx, sqls.NewCnd().Eq("batch_id", batch.ID))
		generatedCount := batch.GeneratedCount
		if currentCount > generatedCount {
			generatedCount = currentCount
		}
		if generatedCount+count > batch.Quantity {
			return errorsx.InvalidParam("service code quantity exceeded")
		}

		// 计算有效期
		now := time.Now()
		var effectiveStartAt, effectiveEndAt *time.Time
		effectiveStartAt = &now
		if batch.Mode == enums.ServiceCodeModeGeneral {
			endAt := now.AddDate(0, 0, DefaultServiceCodeExpiryDays)
			effectiveEndAt = &endAt
		}

		audit := utils.BuildAuditFields(operator)
		seq := generatedCount
		for int64(len(codes)) < count {
			seq++
			value := buildServiceCodeValue(batch.BatchNo, seq)
			if repositories.ServiceCodeRepository.GetByCode(ctx.Tx, value) != nil {
				continue
			}
			codes = append(codes, models.ServiceCode{
				TenantID:         batch.TenantID,
				BatchID:          batch.ID,
				ServiceCode:      value,
				Mode:             batch.Mode,
				ProductID:        batch.ProductID,
				ProductModelID:   batch.ProductModelID,
				Status:           enums.ServiceCodeStatusActive,
				EffectiveStartAt: effectiveStartAt,
				EffectiveEndAt:   effectiveEndAt,
				MetadataJSON:     "{}",
				AuditFields:      audit,
			})
		}
		if err := repositories.ServiceCodeRepository.BatchCreate(ctx.Tx, codes); err != nil {
			return err
		}
		return repositories.ServiceCodeBatchRepository.Updates(ctx.Tx, batch.ID, map[string]any{
			"generated_count":  generatedCount + int64(len(codes)),
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
			"updated_at":       now,
		})
	})
	if err != nil {
		return nil, err
	}
	return codes, nil
}

func (s *serviceCodeManagementService) RevokeCode(codeID int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if codeID <= 0 {
		return errorsx.InvalidParam("service code id is required")
	}

	code := repositories.ServiceCodeRepository.Get(sqls.DB(), codeID)
	if code == nil {
		return errorsx.BusinessError(1, "service code not found")
	}
	if code.Status == enums.ServiceCodeStatusRevoked {
		return nil
	}
	if !s.isRevocable(code.Status) {
		return errorsx.BusinessError(2, "service code cannot be revoked in its current status")
	}

	now := time.Now()
	rows, err := repositories.ServiceCodeRepository.UpdateStatusIf(sqls.DB(), codeID, string(enums.ServiceCodeStatusActive), map[string]any{
		"status":           enums.ServiceCodeStatusRevoked,
		"revoked_at":       &now,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       now,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		current := repositories.ServiceCodeRepository.Get(sqls.DB(), codeID)
		if current != nil && current.Status == enums.ServiceCodeStatusRevoked {
			return nil
		}
		return errorsx.BusinessError(2, "service code cannot be revoked in its current status")
	}
	return nil
}

// IsServiceCodeExpired 检查服务码是否已过期
func (s *serviceCodeManagementService) IsServiceCodeExpired(code *models.ServiceCode) bool {
	if code == nil {
		return true
	}
	if code.Status == enums.ServiceCodeStatusExpired {
		return true
	}
	if code.EffectiveEndAt != nil && code.EffectiveEndAt.Before(time.Now()) {
		return true
	}
	return false
}

// ServiceCodeExpiryJob 定时任务：扫描过期服务码并更新状态为 expired
// 建议每小时执行一次
func (s *serviceCodeManagementService) ServiceCodeExpiryJob() (int, error) {
	const batchSize = 1000
	batchDelay := 100 * time.Millisecond

	totalProcessed := 0
	offset := 0

	for {
		// 查询一批即将/已经过期的服务码
		now := time.Now()
		cnd := sqls.NewCnd().
			Eq("status", string(enums.ServiceCodeStatusActive)).
			Where("effective_end_at is not null and effective_end_at < ?", now).
			Limit(batchSize).
			Asc("id")

		// 使用 Page 分页替代 Offset
		cnd = cnd.Page(offset/batchSize+1, batchSize)

		list := repositories.ServiceCodeRepository.Find(sqls.DB(), cnd)
		if len(list) == 0 {
			break
		}

		for _, code := range list {
			if err := s.expireSingleCode(sqls.DB(), &code); err != nil {
				// 单个失败继续处理
				continue
			}
			totalProcessed++
		}

		if len(list) < batchSize {
			break
		}
		offset += batchSize
		time.Sleep(batchDelay)
	}

	return totalProcessed, nil
}

// expireSingleCode 将单条服务码标记为过期
func (s *serviceCodeManagementService) expireSingleCode(db *gorm.DB, code *models.ServiceCode) error {
	now := time.Now()
	return repositories.ServiceCodeRepository.Updates(db, code.ID, map[string]any{
		"status":           enums.ServiceCodeStatusExpired,
		"revoked_at":       &now,
		"update_user_id":   0,
		"update_user_name": "system",
		"updated_at":       now,
	})
}

// ValidateServiceCodeForScan 校验服务码是否可用于扫码
func (s *serviceCodeManagementService) ValidateServiceCodeForScan(code *models.ServiceCode) error {
	if code == nil {
		return errorsx.InvalidParam("service code not found")
	}
	if code.Status == enums.ServiceCodeStatusRevoked || code.Status == enums.ServiceCodeStatusDisabled {
		return errorsx.BusinessError(2, "service code has been revoked")
	}
	if s.IsServiceCodeExpired(code) {
		expiredDate := ""
		if code.EffectiveEndAt != nil {
			expiredDate = code.EffectiveEndAt.Format("2006-01-02")
		}
		return errorsx.BusinessError(3,
			fmt.Sprintf("此服务码已在 %s 过期，请联系客户支持", expiredDate))
	}
	if code.Status == enums.ServiceCodeStatusGenerated {
		return errorsx.BusinessError(4, "service code has not been activated yet")
	}
	return nil
}

// isRevocable 判断服务码是否可作废
func (s *serviceCodeManagementService) isRevocable(status enums.ServiceCodeStatus) bool {
	return status == enums.ServiceCodeStatusActive ||
		status == enums.ServiceCodeStatusGenerated ||
		status == enums.ServiceCodeStatusBound
}

func buildServiceCodeValue(batchNo string, seq int64) string {
	return fmt.Sprintf("SC%s-%06d", batchNo, seq)
}
