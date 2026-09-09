package services

import (
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var QrLabelService = newQrLabelService()

func newQrLabelService() *qrLabelService {
	return &qrLabelService{}
}

type qrLabelService struct{}

// ---------- QrLabelTemplate CRUD ----------

func (s *qrLabelService) GetTemplate(id int64) *models.QrLabelTemplate {
	if id <= 0 {
		return nil
	}
	return repositories.QrLabelTemplateRepository.Get(sqls.DB(), id)
}

func (s *qrLabelService) FindTemplates(cnd *sqls.Cnd) []models.QrLabelTemplate {
	return repositories.QrLabelTemplateRepository.Find(sqls.DB(), cnd)
}

func (s *qrLabelService) FindTemplatePage(cnd *sqls.Cnd) ([]models.QrLabelTemplate, *sqls.Paging) {
	return repositories.QrLabelTemplateRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *qrLabelService) CreateTemplate(t *models.QrLabelTemplate, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	t.AuditFields = utils.BuildAuditFields(operator)
	return repositories.QrLabelTemplateRepository.Create(sqls.DB(), t)
}

func (s *qrLabelService) UpdateTemplate(id int64, columns map[string]any, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	columns["updated_at"] = time.Now()
	columns["update_user_id"] = operator.UserID
	columns["update_user_name"] = operator.Username
	return repositories.QrLabelTemplateRepository.Updates(sqls.DB(), id, columns)
}

func (s *qrLabelService) DeleteTemplate(id int64) {
	repositories.QrLabelTemplateRepository.Delete(sqls.DB(), id)
}

// ---------- QrLabelExportJob ----------

func (s *qrLabelService) CreateExportJob(t *models.QrLabelExportJob, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	t.AuditFields = utils.BuildAuditFields(operator)
	return repositories.QrLabelExportJobRepository.Create(sqls.DB(), t)
}

func (s *qrLabelService) GetExportJob(id int64) *models.QrLabelExportJob {
	if id <= 0 {
		return nil
	}
	return repositories.QrLabelExportJobRepository.Get(sqls.DB(), id)
}

func (s *qrLabelService) FindExportJobPage(cnd *sqls.Cnd) ([]models.QrLabelExportJob, *sqls.Paging) {
	return repositories.QrLabelExportJobRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *qrLabelService) UpdateExportJob(id int64, columns map[string]any) error {
	return repositories.QrLabelExportJobRepository.Updates(sqls.DB(), id, columns)
}

// ---------- QrScanLog ----------

func (s *qrLabelService) CreateScanLog(log *models.QrScanLog) error {
	if log == nil {
		return errorsx.InvalidParam("scan log is required")
	}
	return repositories.QrScanLogRepository.Create(sqls.DB(), log)
}

func (s *qrLabelService) FindScanLogPage(cnd *sqls.Cnd) ([]models.QrScanLog, *sqls.Paging) {
	return repositories.QrScanLogRepository.FindPageByCnd(sqls.DB(), cnd)
}

// GetAbnormalScanLogs 查询异常扫码记录（失败扫码）
func (s *qrLabelService) GetAbnormalScanLogs(tenantID int64, since time.Time, limit int) []models.QrScanLog {
	cnd := sqls.NewCnd()
	if tenantID > 0 {
		cnd.Eq("tenant_id", tenantID)
	}
	cnd.Where("created_at >= ?", since).
		NotEq("result", "success").
		Desc("created_at").
		Limit(limit)
	return repositories.QrScanLogRepository.Find(sqls.DB(), cnd)
}

// CountScanLogsByDate 统计每日扫码量
func (s *qrLabelService) CountScanLogsByDate(tenantID int64, date string) int64 {
	cnd := sqls.NewCnd().Eq("result", "success")
	if tenantID > 0 {
		cnd.Eq("tenant_id", tenantID)
	}
	cnd.Where("created_at >= ? AND created_at < ?", date+" 00:00:00", date+" 23:59:59")
	return repositories.QrScanLogRepository.Count(sqls.DB(), cnd)
}
