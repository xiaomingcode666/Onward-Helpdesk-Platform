package repositories

import (
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var PlatformIAMRepository = &platformIAMRepository{}

type platformIAMRepository struct{}

type PlatformTenantFilter struct {
	Search    string
	Status    string
	Lifecycle string
	Billing   string
}

func (r *platformIAMRepository) LockTenant(db *gorm.DB, tenantID int64) *models.Tenant {
	if db == nil || tenantID <= 0 {
		return nil
	}
	ret := &models.Tenant{}
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(ret, "id = ?", tenantID).Error; err != nil {
		return nil
	}
	return ret
}

func (r *platformIAMRepository) FindTenantPage(db *gorm.DB, filter PlatformTenantFilter, page, limit int) ([]models.Tenant, *sqls.Paging, error) {
	tenantTable := db.NamingStrategy.TableName("Tenant")
	query := db.Model(&models.Tenant{}).Table(tenantTable + " AS tenants")
	if status := strings.TrimSpace(filter.Status); status != "" && status != "all" {
		query = query.Where("tenants.status = ?", status)
	}
	switch strings.TrimSpace(filter.Lifecycle) {
	case "active":
		query = query.Where("tenants.status = ?", enums.StatusOk)
	case "trial":
		query = query.Where("tenants.status = ? AND tenants.trial_ends_at IS NOT NULL AND tenants.trial_ends_at >= ?", enums.StatusOk, time.Now())
	case "expiring":
		now := time.Now()
		query = query.Where("tenants.status = ? AND tenants.trial_ends_at BETWEEN ? AND ?", enums.StatusOk, now, now.AddDate(0, 0, 30))
	case "frozen":
		query = query.Where("tenants.status = ? AND tenants.decommissioned_at IS NULL", enums.StatusDisabled)
	case "decommissioned":
		query = query.Where("tenants.decommissioned_at IS NOT NULL")
	default:
		query = query.Where("tenants.status <> ?", enums.StatusDeleted)
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		like := "%" + search + "%"
		query = query.Where("tenants.name LIKE ? OR tenants.country_region LIKE ? OR tenants.industry LIKE ?", like, like, like)
	}
	switch strings.ToLower(strings.TrimSpace(filter.Billing)) {
	case "paid":
		query = query.Where(`EXISTS (
			SELECT 1 FROM sub2api_tenant_accounts accounts
			WHERE accounts.tenant_id = tenants.id
				AND accounts.balance > 0
		)`)
	case "free":
		query = query.Where(`NOT EXISTS (
			SELECT 1 FROM sub2api_tenant_accounts accounts
			WHERE accounts.tenant_id = tenants.id
				AND accounts.balance > 0
		)`)
	}
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, nil, err
	}

	items := make([]models.Tenant, 0)
	err := query.Order("tenants.created_at DESC, tenants.id DESC").Offset((page - 1) * limit).Limit(limit).Find(&items).Error
	return items, &sqls.Paging{Page: page, Limit: limit, Total: total}, err
}

func (r *platformIAMRepository) GetTenant(db *gorm.DB, tenantID int64) *models.Tenant {
	if tenantID <= 0 {
		return nil
	}
	ret := &models.Tenant{}
	if err := db.First(ret, "id = ?", tenantID).Error; err != nil {
		return nil
	}
	return ret
}

func (r *platformIAMRepository) CreateTenant(db *gorm.DB, item *models.Tenant) error {
	return db.Create(item).Error
}

func (r *platformIAMRepository) UpdateTenant(db *gorm.DB, tenantID int64, columns map[string]any) error {
	return db.Model(&models.Tenant{}).Where("id = ?", tenantID).Updates(columns).Error
}

func (r *platformIAMRepository) GetTenantBranding(db *gorm.DB, tenantID int64) *models.TenantBranding {
	if db == nil || tenantID <= 0 {
		return nil
	}
	ret := &models.TenantBranding{}
	if err := db.First(ret, "tenant_id = ?", tenantID).Error; err != nil {
		return nil
	}
	return ret
}

func (r *platformIAMRepository) FindActiveTenantBrandingByLogoAssetID(db *gorm.DB, assetID int64) *models.TenantBranding {
	if db == nil || assetID <= 0 {
		return nil
	}
	ret := &models.TenantBranding{}
	if err := db.Where("logo_asset_id = ? AND status <> ?", assetID, enums.StatusDeleted).First(ret).Error; err != nil {
		return nil
	}
	return ret
}

func (r *platformIAMRepository) FindTenantBrandingsByTenantIDs(db *gorm.DB, tenantIDs []int64) (map[int64]*models.TenantBranding, error) {
	ret := make(map[int64]*models.TenantBranding, len(tenantIDs))
	if len(tenantIDs) == 0 {
		return ret, nil
	}
	items := make([]models.TenantBranding, 0, len(tenantIDs))
	if err := db.Where("tenant_id IN ? AND status <> ?", tenantIDs, enums.StatusDeleted).Find(&items).Error; err != nil {
		return nil, err
	}
	for i := range items {
		ret[items[i].TenantID] = &items[i]
	}
	return ret, nil
}

func (r *platformIAMRepository) CreateTenantBranding(db *gorm.DB, item *models.TenantBranding) error {
	return db.Create(item).Error
}

func (r *platformIAMRepository) UpdateTenantBranding(db *gorm.DB, tenantID int64, columns map[string]any) error {
	return db.Model(&models.TenantBranding{}).Where("tenant_id = ?", tenantID).Updates(columns).Error
}

func (r *platformIAMRepository) CountDevicesByTenantIDs(db *gorm.DB, tenantIDs []int64) (map[int64]int64, error) {
	return r.countByTenantIDs(db.Model(&models.Device{}), tenantIDs)
}

func (r *platformIAMRepository) CountMembersByTenantIDs(db *gorm.DB, tenantIDs []int64) (map[int64]int64, error) {
	return r.countByTenantIDs(db.Model(&models.TenantMember{}), tenantIDs)
}

func (r *platformIAMRepository) countByTenantIDs(query *gorm.DB, tenantIDs []int64) (map[int64]int64, error) {
	ret := make(map[int64]int64, len(tenantIDs))
	if len(tenantIDs) == 0 {
		return ret, nil
	}
	rows := make([]struct {
		TenantID int64
		Count    int64
	}, 0)
	if err := query.Select("tenant_id, COUNT(*) AS count").Where("tenant_id IN ?", tenantIDs).Group("tenant_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		ret[row.TenantID] = row.Count
	}
	return ret, nil
}

func (r *platformIAMRepository) FindActiveSubscriptions(db *gorm.DB, tenantIDs []int64) (map[int64]*models.TenantSubscription, error) {
	ret := make(map[int64]*models.TenantSubscription, len(tenantIDs))
	if len(tenantIDs) == 0 {
		return ret, nil
	}
	items := make([]models.TenantSubscription, 0)
	if err := db.Where("tenant_id IN ? AND status = ?", tenantIDs, enums.StatusOk).Order("id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	for i := range items {
		if _, exists := ret[items[i].TenantID]; !exists {
			item := items[i]
			ret[item.TenantID] = &item
		}
	}
	return ret, nil
}

func (r *platformIAMRepository) GetCurrentSubscription(db *gorm.DB, tenantID int64, now time.Time) *models.TenantSubscription {
	if db == nil || tenantID <= 0 {
		return nil
	}
	item := &models.TenantSubscription{}
	if err := db.Where("tenant_id = ? AND status = ? AND starts_at <= ? AND (ends_at IS NULL OR ends_at > ?)", tenantID, enums.StatusOk, now, now).
		Order("id DESC").
		First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *platformIAMRepository) FindPlansByIDs(db *gorm.DB, planIDs []int64) (map[int64]*models.TenantPlan, error) {
	ret := make(map[int64]*models.TenantPlan, len(planIDs))
	if len(planIDs) == 0 {
		return ret, nil
	}
	items := make([]models.TenantPlan, 0)
	if err := db.Where("id IN ?", planIDs).Find(&items).Error; err != nil {
		return nil, err
	}
	for i := range items {
		item := items[i]
		ret[item.ID] = &item
	}
	return ret, nil
}

func (r *platformIAMRepository) FindPlans(db *gorm.DB, status string) ([]models.TenantPlan, error) {
	items := make([]models.TenantPlan, 0)
	query := db.Model(&models.TenantPlan{})
	if status = strings.TrimSpace(status); status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}
	return items, query.Order("sort_no ASC, id ASC").Find(&items).Error
}

func (r *platformIAMRepository) GetPlan(db *gorm.DB, planID int64) *models.TenantPlan {
	if planID <= 0 {
		return nil
	}
	ret := &models.TenantPlan{}
	if err := db.First(ret, "id = ?", planID).Error; err != nil {
		return nil
	}
	return ret
}

func (r *platformIAMRepository) GetPlanByCode(db *gorm.DB, code string) *models.TenantPlan {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil
	}
	ret := &models.TenantPlan{}
	if err := db.First(ret, "code = ?", code).Error; err != nil {
		return nil
	}
	return ret
}

func (r *platformIAMRepository) FindPlanQuota(db *gorm.DB, planID int64, quotaKey, period string) *models.TenantPlanQuota {
	quotaKey = strings.TrimSpace(quotaKey)
	period = strings.TrimSpace(period)
	if db == nil || planID <= 0 || quotaKey == "" {
		return nil
	}
	item := &models.TenantPlanQuota{}
	query := db.Where("plan_id = ? AND quota_key = ? AND status = ?", planID, quotaKey, enums.StatusOk)
	if period != "" {
		query = query.Where("period = ?", period)
	}
	if err := query.Order("id DESC").First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *platformIAMRepository) CreatePlan(db *gorm.DB, item *models.TenantPlan) error {
	return db.Create(item).Error
}

func (r *platformIAMRepository) UpdatePlan(db *gorm.DB, planID int64, columns map[string]any) error {
	return db.Model(&models.TenantPlan{}).Where("id = ?", planID).Updates(columns).Error
}

func (r *platformIAMRepository) DisableActiveSubscriptions(db *gorm.DB, tenantID int64, now time.Time) error {
	return db.Model(&models.TenantSubscription{}).
		Where("tenant_id = ? AND status = ?", tenantID, enums.StatusOk).
		Updates(map[string]any{"status": enums.StatusDisabled, "updated_at": now}).Error
}

func (r *platformIAMRepository) CreateSubscription(db *gorm.DB, item *models.TenantSubscription) error {
	return db.Create(item).Error
}

func (r *platformIAMRepository) FindQuotaOverride(db *gorm.DB, tenantID, productID int64, quotaKey, period string) *models.TenantQuotaOverride {
	ret := &models.TenantQuotaOverride{}
	if err := db.First(ret, "tenant_id = ? AND product_id = ? AND quota_key = ? AND period = ?", tenantID, productID, quotaKey, period).Error; err != nil {
		return nil
	}
	return ret
}

func (r *platformIAMRepository) FindActiveQuotaOverride(db *gorm.DB, tenantID, productID int64, quotaKey, period string, now time.Time) *models.TenantQuotaOverride {
	quotaKey = strings.TrimSpace(quotaKey)
	period = strings.TrimSpace(period)
	if db == nil || tenantID <= 0 || quotaKey == "" {
		return nil
	}
	item := &models.TenantQuotaOverride{}
	query := db.Where("tenant_id = ? AND product_id = ? AND quota_key = ? AND status = ?", tenantID, productID, quotaKey, enums.StatusOk).
		Where("(effective_from IS NULL OR effective_from <= ?) AND (effective_to IS NULL OR effective_to > ?)", now, now)
	if period != "" {
		query = query.Where("period = ?", period)
	}
	if err := query.Order("id DESC").First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *platformIAMRepository) CreateQuotaOverride(db *gorm.DB, item *models.TenantQuotaOverride) error {
	return db.Create(item).Error
}

func (r *platformIAMRepository) UpdateQuotaOverride(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.TenantQuotaOverride{}).Where("id = ?", id).Updates(columns).Error
}

func (r *platformIAMRepository) FindSub2APIAccounts(db *gorm.DB, tenantID int64) ([]models.Sub2APITenantAccount, error) {
	items := make([]models.Sub2APITenantAccount, 0)
	query := db.Model(&models.Sub2APITenantAccount{})
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	return items, query.Order("id DESC").Find(&items).Error
}

func (r *platformIAMRepository) FindSub2APIAccountByTenantID(db *gorm.DB, tenantID int64) *models.Sub2APITenantAccount {
	if tenantID <= 0 {
		return nil
	}
	ret := &models.Sub2APITenantAccount{}
	if err := db.First(ret, "tenant_id = ?", tenantID).Error; err != nil {
		return nil
	}
	return ret
}

func (r *platformIAMRepository) HasPositiveSub2APIBalance(db *gorm.DB, tenantID int64) bool {
	if db == nil || tenantID <= 0 {
		return false
	}
	var count int64
	return db.Model(&models.Sub2APITenantAccount{}).
		Where("tenant_id = ? AND balance > 0", tenantID).
		Count(&count).Error == nil && count > 0
}

func (r *platformIAMRepository) FindSub2APIAccount(db *gorm.DB, tenantID int64, accountID string) *models.Sub2APITenantAccount {
	ret := &models.Sub2APITenantAccount{}
	if err := db.First(ret, "tenant_id = ? AND sub2_api_account_id = ?", tenantID, accountID).Error; err != nil {
		return nil
	}
	return ret
}

func (r *platformIAMRepository) GetSub2APIAccount(db *gorm.DB, id int64) *models.Sub2APITenantAccount {
	ret := &models.Sub2APITenantAccount{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *platformIAMRepository) CreateSub2APIAccount(db *gorm.DB, item *models.Sub2APITenantAccount) error {
	return db.Create(item).Error
}

func (r *platformIAMRepository) UpdateSub2APIAccount(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.Sub2APITenantAccount{}).Where("id = ?", id).Updates(columns).Error
}

func (r *platformIAMRepository) ClaimSub2APIAccountProvision(db *gorm.DB, id int64, staleBefore time.Time, columns map[string]any) (bool, error) {
	result := db.Model(&models.Sub2APITenantAccount{}).
		Where("id = ? AND (provision_status NOT IN ? OR updated_at < ?)", id, []string{"provisioning_user", "provisioning_key"}, staleBefore).
		Updates(columns)
	return result.RowsAffected == 1, result.Error
}

func (r *platformIAMRepository) CreateSub2APIRechargeRecord(db *gorm.DB, item *models.Sub2APIRechargeRecord) error {
	return db.Create(item).Error
}

func (r *platformIAMRepository) FindSub2APIRechargeRecords(db *gorm.DB, tenantID int64, limit int) ([]models.Sub2APIRechargeRecord, error) {
	items := make([]models.Sub2APIRechargeRecord, 0)
	query := db.Model(&models.Sub2APIRechargeRecord{})
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return items, query.Order("occurred_at DESC, id DESC").Limit(limit).Find(&items).Error
}

func (r *platformIAMRepository) FindPlatformStaff(db *gorm.DB, status string) ([]models.PlatformStaffProfile, error) {
	items := make([]models.PlatformStaffProfile, 0)
	query := db.Model(&models.PlatformStaffProfile{})
	if status = strings.TrimSpace(status); status != "" && status != "all" {
		query = query.Where("status = ?", status)
	} else {
		query = query.Where("status <> ?", enums.StatusDeleted)
	}
	return items, query.Order("id DESC").Find(&items).Error
}

func (r *platformIAMRepository) GetPlatformStaff(db *gorm.DB, id int64) *models.PlatformStaffProfile {
	ret := &models.PlatformStaffProfile{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *platformIAMRepository) FindPlatformStaffByUserID(db *gorm.DB, userID int64) *models.PlatformStaffProfile {
	ret := &models.PlatformStaffProfile{}
	if err := db.First(ret, "user_id = ?", userID).Error; err != nil {
		return nil
	}
	return ret
}

func (r *platformIAMRepository) CreatePlatformStaff(db *gorm.DB, item *models.PlatformStaffProfile) error {
	return db.Create(item).Error
}

func (r *platformIAMRepository) UpdatePlatformStaff(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.PlatformStaffProfile{}).Where("id = ?", id).Updates(columns).Error
}

func (r *platformIAMRepository) CreatePlatformTenantGrant(db *gorm.DB, item *models.PlatformTenantGrant) error {
	return db.Create(item).Error
}

func (r *platformIAMRepository) GetActivePlatformTenantGrant(db *gorm.DB, id, tenantID int64) *models.PlatformTenantGrant {
	if id <= 0 || tenantID <= 0 {
		return nil
	}
	item := &models.PlatformTenantGrant{}
	if err := db.Where(
		"id = ? AND tenant_id = ? AND status = ? AND revoked_at IS NULL AND expired_at > ?",
		id, tenantID, models.AccessGrantStatusActive, time.Now(),
	).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *platformIAMRepository) CreateTemporaryAccessGrant(db *gorm.DB, item *models.TemporaryAccessGrant) error {
	return db.Create(item).Error
}

func (r *platformIAMRepository) GetActiveTemporaryAccessGrant(db *gorm.DB, id, tenantID int64, resourceType, resourceID string) *models.TemporaryAccessGrant {
	if id <= 0 || tenantID <= 0 || strings.TrimSpace(resourceType) == "" || strings.TrimSpace(resourceID) == "" {
		return nil
	}
	item := &models.TemporaryAccessGrant{}
	if err := db.Where(
		"id = ? AND tenant_id = ? AND resource_type = ? AND resource_id = ? AND status = ? AND revoked_at IS NULL AND expired_at > ?",
		id, tenantID, strings.TrimSpace(resourceType), strings.TrimSpace(resourceID), models.AccessGrantStatusActive, time.Now(),
	).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *platformIAMRepository) FindAuthRoles(db *gorm.DB, tenantID int64, domainType string) ([]models.AuthRole, error) {
	items := make([]models.AuthRole, 0)
	query := db.Model(&models.AuthRole{})
	if tenantID >= 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if domainType = strings.TrimSpace(domainType); domainType != "" && domainType != "all" {
		query = query.Where("domain_type = ?", domainType)
	}
	query = query.Where("status <> ?", enums.StatusDeleted)
	return items, query.Order("sort_no ASC, id ASC").Find(&items).Error
}

func (r *platformIAMRepository) GetAuthRole(db *gorm.DB, id int64) *models.AuthRole {
	ret := &models.AuthRole{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *platformIAMRepository) FindAuthRolesByIDs(db *gorm.DB, roleIDs []int64) (map[int64]*models.AuthRole, error) {
	ret := make(map[int64]*models.AuthRole, len(roleIDs))
	if len(roleIDs) == 0 {
		return ret, nil
	}
	items := make([]models.AuthRole, 0)
	if err := db.Where("id IN ?", roleIDs).Find(&items).Error; err != nil {
		return nil, err
	}
	for i := range items {
		item := items[i]
		ret[item.ID] = &item
	}
	return ret, nil
}

func (r *platformIAMRepository) FindAuthRoleByCode(db *gorm.DB, tenantID int64, domainType, code string) *models.AuthRole {
	ret := &models.AuthRole{}
	if err := db.First(ret, "tenant_id = ? AND domain_type = ? AND code = ?", tenantID, domainType, code).Error; err != nil {
		return nil
	}
	return ret
}

func (r *platformIAMRepository) CreateAuthRole(db *gorm.DB, item *models.AuthRole) error {
	return db.Create(item).Error
}

func (r *platformIAMRepository) UpdateAuthRole(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.AuthRole{}).Where("id = ?", id).Updates(columns).Error
}

func (r *platformIAMRepository) UpdateRoleBindingsByRole(db *gorm.DB, tenantID int64, domainType string, roleID int64, columns map[string]any) error {
	return db.Model(&models.AuthRoleBinding{}).
		Where("tenant_id = ? AND domain_type = ? AND role_id = ?", tenantID, domainType, roleID).
		Updates(columns).Error
}

func (r *platformIAMRepository) UpdateRoleBindingsBySubject(db *gorm.DB, tenantID int64, domainType, subjectType string, subjectID int64, columns map[string]any) error {
	return db.Model(&models.AuthRoleBinding{}).
		Where("tenant_id = ? AND domain_type = ? AND subject_type = ? AND subject_id = ?", tenantID, domainType, subjectType, subjectID).
		Updates(columns).Error
}

func (r *platformIAMRepository) UpdateRolePermissionsByRole(db *gorm.DB, tenantID int64, roleID int64, columns map[string]any) error {
	return db.Model(&models.AuthRolePermission{}).
		Where("tenant_id = ? AND role_id = ?", tenantID, roleID).
		Updates(columns).Error
}

func (r *platformIAMRepository) ReplaceAuthRolePermissions(db *gorm.DB, tenantID, roleID int64, permissions []models.AuthRolePermission) error {
	if err := db.Where("tenant_id = ? AND role_id = ?", tenantID, roleID).Delete(&models.AuthRolePermission{}).Error; err != nil {
		return err
	}
	if len(permissions) == 0 {
		return nil
	}
	return db.Create(&permissions).Error
}

func (r *platformIAMRepository) FindPermissionsByRoleIDs(db *gorm.DB, roleIDs []int64) (map[int64][]string, error) {
	ret := make(map[int64][]string, len(roleIDs))
	if len(roleIDs) == 0 {
		return ret, nil
	}
	rows := make([]models.AuthRolePermission, 0)
	if err := db.Where("role_id IN ? AND status = ?", roleIDs, enums.StatusOk).Order("permission_code ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.Effect != "deny" {
			ret[row.RoleID] = append(ret[row.RoleID], row.PermissionCode)
		}
	}
	return ret, nil
}

func (r *platformIAMRepository) FindRoleBindings(db *gorm.DB, tenantID int64, domainType, subjectType string, subjectID int64) ([]models.AuthRoleBinding, error) {
	items := make([]models.AuthRoleBinding, 0)
	now := time.Now()
	err := db.Where("tenant_id = ? AND domain_type = ? AND subject_type = ? AND subject_id = ? AND status = ?", tenantID, domainType, subjectType, subjectID, enums.StatusOk).
		Where("(effective_at IS NULL OR effective_at <= ?) AND (expired_at IS NULL OR expired_at > ?)", now, now).
		Find(&items).Error
	return items, err
}

func (r *platformIAMRepository) ReplaceRoleBinding(db *gorm.DB, item *models.AuthRoleBinding) error {
	if err := db.Where(
		"tenant_id = ? AND domain_type = ? AND subject_type = ? AND subject_id = ?",
		item.TenantID, item.DomainType, item.SubjectType, item.SubjectID,
	).Delete(&models.AuthRoleBinding{}).Error; err != nil {
		return err
	}
	return db.Create(item).Error
}

func (r *platformIAMRepository) ReplaceRoleBindings(db *gorm.DB, tenantID int64, domainType, subjectType string, subjectID int64, items []models.AuthRoleBinding) error {
	if err := db.Where(
		"tenant_id = ? AND domain_type = ? AND subject_type = ? AND subject_id = ?",
		tenantID, domainType, subjectType, subjectID,
	).Delete(&models.AuthRoleBinding{}).Error; err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	return db.Create(&items).Error
}

func (r *platformIAMRepository) FindAuthRolePermissionRows(db *gorm.DB, tenantID int64, roleIDs []int64) ([]models.AuthRolePermission, error) {
	items := make([]models.AuthRolePermission, 0)
	if len(roleIDs) == 0 {
		return items, nil
	}
	err := db.Where("tenant_id = ? AND role_id IN ? AND status = ?", tenantID, roleIDs, enums.StatusOk).
		Order("id ASC").Find(&items).Error
	return items, err
}

func (r *platformIAMRepository) FindSubjectPermissionOverrides(
	db *gorm.DB,
	tenantID int64,
	domainType string,
	subjectType string,
	subjectID int64,
) ([]models.AuthSubjectPermissionOverride, error) {
	items := make([]models.AuthSubjectPermissionOverride, 0)
	if tenantID <= 0 || subjectID <= 0 || !db.Migrator().HasTable(&models.AuthSubjectPermissionOverride{}) {
		return items, nil
	}
	now := time.Now()
	err := db.Where(
		"tenant_id = ? AND domain_type = ? AND subject_type = ? AND subject_id = ? AND status = ?",
		tenantID,
		domainType,
		subjectType,
		subjectID,
		enums.StatusOk,
	).
		Where("expired_at IS NULL OR expired_at > ?", now).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

func (r *platformIAMRepository) FindRolePermissions(db *gorm.DB, tenantID int64, roleIDs []int64, permissionCode string) ([]models.AuthRolePermission, error) {
	items := make([]models.AuthRolePermission, 0)
	if len(roleIDs) == 0 {
		return items, nil
	}
	err := db.Where("tenant_id = ? AND role_id IN ? AND permission_code = ? AND status = ?", tenantID, roleIDs, permissionCode, enums.StatusOk).Find(&items).Error
	return items, err
}

func (r *platformIAMRepository) CreateAuthAuditLog(db *gorm.DB, item *models.AuthAuditLog) error {
	return db.Create(item).Error
}

type AuditLogFilter struct {
	TenantID   int64
	Action     string
	RiskLevel  string
	Status     string
	TargetType string
	Search     string
	ActorID    int64
	StartAt    *time.Time
	EndAt      *time.Time
}

func (r *platformIAMRepository) FindAuthAuditLogs(db *gorm.DB, filter AuditLogFilter, page, limit int) ([]models.AuthAuditLog, *sqls.Paging, error) {
	query := applyAuthAuditLogFilter(db.Model(&models.AuthAuditLog{}), filter)
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, nil, err
	}
	items := make([]models.AuthAuditLog, 0)
	err := query.Order("occurred_at DESC, id DESC").Offset((page - 1) * limit).Limit(limit).Find(&items).Error
	return items, &sqls.Paging{Page: page, Limit: limit, Total: total}, err
}

func (r *platformIAMRepository) FindAuthAuditLogHead(db *gorm.DB, filter AuditLogFilter, limit int) ([]models.AuthAuditLog, int64, error) {
	query := applyAuthAuditLogFilter(db.Model(&models.AuthAuditLog{}), filter)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]models.AuthAuditLog, 0)
	err := query.Order("occurred_at DESC, id DESC").Limit(limit).Find(&items).Error
	return items, total, err
}

func (r *platformIAMRepository) FindBusinessAuditLogHead(db *gorm.DB, filter AuditLogFilter, limit int) ([]models.AuditLog, int64, error) {
	query := applyBusinessAuditLogFilter(db.Model(&models.AuditLog{}), filter)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]models.AuditLog, 0)
	err := query.Order("created_at DESC, id DESC").Limit(limit).Find(&items).Error
	return items, total, err
}

func (r *platformIAMRepository) FindBusinessAuditLogs(db *gorm.DB, filter AuditLogFilter, page, limit int) ([]models.AuditLog, *sqls.Paging, error) {
	query := applyBusinessAuditLogFilter(db.Model(&models.AuditLog{}), filter)
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, nil, err
	}
	items := make([]models.AuditLog, 0)
	err := query.Order("created_at DESC, id DESC").Offset((page - 1) * limit).Limit(limit).Find(&items).Error
	return items, &sqls.Paging{Page: page, Limit: limit, Total: total}, err
}

func (r *platformIAMRepository) DeleteAuthAuditLogsBefore(db *gorm.DB, cutoff time.Time, limit int) (int64, error) {
	if cutoff.IsZero() {
		return 0, nil
	}
	if limit <= 0 {
		limit = 1000
	}
	var ids []int64
	if err := db.Model(&models.AuthAuditLog{}).
		Where("occurred_at < ?", cutoff).
		Order("occurred_at ASC, id ASC").
		Limit(limit).
		Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := db.Where("id IN ?", ids).Delete(&models.AuthAuditLog{})
	return result.RowsAffected, result.Error
}

func (r *platformIAMRepository) DeleteBusinessAuditLogsBefore(db *gorm.DB, cutoff time.Time, limit int) (int64, error) {
	if cutoff.IsZero() {
		return 0, nil
	}
	if limit <= 0 {
		limit = 1000
	}
	var ids []int64
	if err := db.Model(&models.AuditLog{}).
		Where("created_at < ?", cutoff).
		Order("created_at ASC, id ASC").
		Limit(limit).
		Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := db.Where("id IN ?", ids).Delete(&models.AuditLog{})
	return result.RowsAffected, result.Error
}

func (r *platformIAMRepository) DeleteBusinessAuditArchiveLogsBefore(db *gorm.DB, cutoff time.Time, limit int) (int64, error) {
	const tableName = "audit_logs_archive"
	if cutoff.IsZero() || !db.Migrator().HasTable(tableName) {
		return 0, nil
	}
	if limit <= 0 {
		limit = 1000
	}
	var ids []int64
	if err := db.Table(tableName).
		Where("created_at < ?", cutoff).
		Order("created_at ASC, id ASC").
		Limit(limit).
		Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := db.Exec("DELETE FROM audit_logs_archive WHERE id IN ?", ids)
	return result.RowsAffected, result.Error
}

func applyAuthAuditLogFilter(query *gorm.DB, filter AuditLogFilter) *gorm.DB {
	if filter.TenantID > 0 {
		query = query.Where("tenant_id = ?", filter.TenantID)
	}
	if action := strings.TrimSpace(filter.Action); action != "" {
		query = query.Where("action = ?", action)
	}
	query = applyAuthAuditSearch(query, filter)
	query = applyAuditCommonFilter(query, filter, "actor_user_id", "target_type", "occurred_at")
	return query
}

func applyBusinessAuditLogFilter(query *gorm.DB, filter AuditLogFilter) *gorm.DB {
	if filter.TenantID > 0 {
		query = query.Where("tenant_id = ?", filter.TenantID)
	}
	if action := strings.TrimSpace(filter.Action); action != "" {
		query = query.Where("action = ?", action)
	}
	if filter.ActorID > 0 {
		query = query.Where("actor_id = ?", strconv.FormatInt(filter.ActorID, 10))
	}
	filter.ActorID = 0
	query = applyBusinessAuditSearch(query, filter)
	query = applyAuditCommonFilter(query, filter, "", "resource_type", "created_at")
	return query
}

func applyAuthAuditSearch(query *gorm.DB, filter AuditLogFilter) *gorm.DB {
	search := strings.ToLower(strings.TrimSpace(filter.Search))
	if search == "" {
		return query
	}
	pattern := "%" + search + "%"
	return query.Where(`(
		LOWER(action) LIKE ? OR LOWER(target_type) LIKE ? OR LOWER(target_id) LIKE ? OR
		LOWER(request_id) LIKE ? OR LOWER(before_state_json) LIKE ? OR LOWER(after_state_json) LIKE ? OR
		(target_type = ? AND EXISTS (
			SELECT 1 FROM tickets
			WHERE tickets.tenant_id = auth_audit_logs.tenant_id
				AND CAST(tickets.id AS TEXT) = auth_audit_logs.target_id
				AND LOWER(tickets.ticket_no) LIKE ?
		))
	)`, pattern, pattern, pattern, pattern, pattern, pattern, "ticket", pattern)
}

func applyBusinessAuditSearch(query *gorm.DB, filter AuditLogFilter) *gorm.DB {
	search := strings.ToLower(strings.TrimSpace(filter.Search))
	if search == "" {
		return query
	}
	pattern := "%" + search + "%"
	return query.Where(`(
		LOWER(action) LIKE ? OR LOWER(resource_type) LIKE ? OR LOWER(resource_id) LIKE ? OR
		LOWER(request_id) LIKE ? OR LOWER(before_state) LIKE ? OR LOWER(after_state) LIKE ? OR
		(resource_type = ? AND EXISTS (
			SELECT 1 FROM tickets
			WHERE tickets.tenant_id = audit_logs.tenant_id
				AND CAST(tickets.id AS TEXT) = audit_logs.resource_id
				AND LOWER(tickets.ticket_no) LIKE ?
		))
	)`, pattern, pattern, pattern, pattern, pattern, pattern, "ticket", pattern)
}

func applyAuditCommonFilter(query *gorm.DB, filter AuditLogFilter, actorColumn, targetColumn, occurredColumn string) *gorm.DB {
	if riskLevel := strings.TrimSpace(filter.RiskLevel); riskLevel != "" {
		query = query.Where("risk_level = ?", riskLevel)
	}
	switch status := strings.TrimSpace(filter.Status); status {
	case "failed":
		query = query.Where("status IN ?", []string{"error", "failed", "failure"})
	case "success", "blocked":
		query = query.Where("status = ?", status)
	}
	if targetType := strings.TrimSpace(filter.TargetType); targetType != "" {
		switch targetType {
		case "message":
			query = query.Where(targetColumn+" IN ?", []string{"message", "conversation_message"})
		case "support":
			query = query.Where("support_grant_id > 0")
		default:
			query = query.Where(targetColumn+" = ?", targetType)
		}
	}
	if actorColumn != "" && filter.ActorID > 0 {
		query = query.Where(actorColumn+" = ?", filter.ActorID)
	}
	if filter.StartAt != nil {
		query = query.Where(occurredColumn+" >= ?", *filter.StartAt)
	}
	if filter.EndAt != nil {
		query = query.Where(occurredColumn+" <= ?", *filter.EndAt)
	}
	return query
}
