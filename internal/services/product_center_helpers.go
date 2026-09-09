package services

import (
	"encoding/json"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func normalizeCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func normalizeJSON(value string, fallback string) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return fallback, nil
	}
	if !json.Valid([]byte(normalized)) {
		return "", errorsx.InvalidParam("invalid json")
	}
	return normalized, nil
}

func requireActiveTenant(tenantID int64) (*models.Tenant, error) {
	return requireActiveTenantDB(sqls.DB(), tenantID)
}

func requireActiveTenantDB(db *gorm.DB, tenantID int64) (*models.Tenant, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenantId is required")
	}
	if db == nil {
		db = sqls.DB()
	}
	tenant := repositories.TenantRepository.Get(db, tenantID)
	if tenant == nil || tenant.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("tenant not found")
	}
	if tenant.Status == enums.StatusDisabled {
		return nil, errorsx.InvalidParam("tenant is disabled")
	}
	return tenant, nil
}

func isMutableStatus(status int) bool {
	return status == int(enums.StatusOk) || status == int(enums.StatusDisabled)
}
