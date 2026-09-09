package repositories

import (
	"errors"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var EnterpriseIAMRepository = &enterpriseIAMRepository{}

type enterpriseIAMRepository struct{}

type EnterpriseIAMFilter struct {
	Search string
	Status string
}

func (r *enterpriseIAMRepository) FindActiveTenantMemberByUserID(db *gorm.DB, userID int64) *models.TenantMember {
	if userID <= 0 {
		return nil
	}
	item := &models.TenantMember{}
	if err := db.Where("user_id = ? AND status = ?", userID, enums.StatusOk).Order("id ASC").First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *enterpriseIAMRepository) FindTenantMemberByUserID(db *gorm.DB, tenantID, userID int64) *models.TenantMember {
	item := &models.TenantMember{}
	if err := db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *enterpriseIAMRepository) FindActiveTenantMembers(db *gorm.DB, tenantID int64) ([]models.TenantMember, error) {
	items := make([]models.TenantMember, 0)
	if db == nil || tenantID <= 0 || !db.Migrator().HasTable(&models.TenantMember{}) || !db.Migrator().HasTable(&models.User{}) {
		return items, nil
	}
	err := db.Where("tenant_id = ? AND status = ?", tenantID, enums.StatusOk).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

func (r *enterpriseIAMRepository) GetTenantMember(db *gorm.DB, tenantID, memberID int64) *models.TenantMember {
	if tenantID <= 0 || memberID <= 0 {
		return nil
	}
	item := &models.TenantMember{}
	if err := db.Where("tenant_id = ? AND id = ? AND status = ?", tenantID, memberID, enums.StatusOk).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *enterpriseIAMRepository) GetTenantMemberAnyStatus(db *gorm.DB, tenantID, memberID int64) *models.TenantMember {
	if tenantID <= 0 || memberID <= 0 {
		return nil
	}
	item := &models.TenantMember{}
	if err := db.Where("tenant_id = ? AND id = ? AND status <> ?", tenantID, memberID, enums.StatusDeleted).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *enterpriseIAMRepository) FindTenantAdministrator(db *gorm.DB, tenantID int64) *models.TenantMember {
	if tenantID <= 0 {
		return nil
	}
	now := time.Now()
	item := &models.TenantMember{}
	err := db.Table("tenant_members AS tm").
		Select("tm.*").
		Joins("JOIN auth_role_bindings AS arb ON arb.tenant_id = tm.tenant_id AND arb.domain_type = ? AND arb.subject_type = ? AND arb.subject_id = tm.id AND arb.status = ?", models.DomainTypeEnterprise, models.SubjectTypeTenantMember, enums.StatusOk).
		Joins("JOIN auth_roles AS ar ON ar.id = arb.role_id AND ar.status = ?", enums.StatusOk).
		Where("tm.tenant_id = ? AND tm.status = ?", tenantID, enums.StatusOk).
		Where("ar.code IN ?", []string{"tenant_owner", "tenant_admin"}).
		Where("(arb.effective_at IS NULL OR arb.effective_at <= ?) AND (arb.expired_at IS NULL OR arb.expired_at > ?)", now, now).
		Order("CASE ar.code WHEN 'tenant_owner' THEN 0 ELSE 1 END, tm.id ASC").
		First(item).Error
	if err == nil {
		return item
	}
	if err := db.Where("tenant_id = ? AND status = ?", tenantID, enums.StatusOk).Order("id ASC").First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *enterpriseIAMRepository) FindTenantAdministrators(db *gorm.DB, tenantIDs []int64) (map[int64]*models.TenantMember, error) {
	ret := make(map[int64]*models.TenantMember)
	if len(tenantIDs) == 0 {
		return ret, nil
	}

	now := time.Now()
	var items []models.TenantMember
	err := db.Table("tenant_members AS tm").
		Select("tm.*").
		Joins("JOIN auth_role_bindings AS arb ON arb.tenant_id = tm.tenant_id AND arb.domain_type = ? AND arb.subject_type = ? AND arb.subject_id = tm.id AND arb.status = ?", models.DomainTypeEnterprise, models.SubjectTypeTenantMember, enums.StatusOk).
		Joins("JOIN auth_roles AS ar ON ar.id = arb.role_id AND ar.status = ?", enums.StatusOk).
		Where("tm.tenant_id IN ? AND tm.status = ?", tenantIDs, enums.StatusOk).
		Where("ar.code IN ?", []string{"tenant_owner", "tenant_admin"}).
		Where("(arb.effective_at IS NULL OR arb.effective_at <= ?) AND (arb.expired_at IS NULL OR arb.expired_at > ?)", now, now).
		Order("tm.tenant_id ASC, CASE ar.code WHEN 'tenant_owner' THEN 0 ELSE 1 END, tm.id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	for i := range items {
		if _, exists := ret[items[i].TenantID]; !exists {
			ret[items[i].TenantID] = &items[i]
		}
	}

	var fallbackItems []models.TenantMember
	if err := db.Where("tenant_id IN ? AND status = ?", tenantIDs, enums.StatusOk).
		Order("tenant_id ASC, id ASC").
		Find(&fallbackItems).Error; err != nil {
		return nil, err
	}
	for i := range fallbackItems {
		if _, exists := ret[fallbackItems[i].TenantID]; !exists {
			ret[fallbackItems[i].TenantID] = &fallbackItems[i]
		}
	}
	return ret, nil
}

func (r *enterpriseIAMRepository) CreateTenantMember(db *gorm.DB, item *models.TenantMember) error {
	return db.Create(item).Error
}

func (r *enterpriseIAMRepository) UpdateTenantMember(db *gorm.DB, tenantID, memberID int64, updates map[string]any) error {
	return db.Model(&models.TenantMember{}).
		Where("tenant_id = ? AND id = ?", tenantID, memberID).
		Updates(updates).Error
}

func (r *enterpriseIAMRepository) UpdateTenantMemberDepartmentByUserID(db *gorm.DB, tenantID, userID, departmentID int64) error {
	if tenantID <= 0 || userID <= 0 || departmentID <= 0 {
		return nil
	}
	return db.Model(&models.TenantMember{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Updates(map[string]any{"department_id": departmentID, "updated_at": time.Now()}).Error
}

func (r *enterpriseIAMRepository) CreateEngineerProfile(db *gorm.DB, item *models.EngineerProfile) error {
	return db.Create(item).Error
}

func (r *enterpriseIAMRepository) UpdateEngineerProfile(db *gorm.DB, tenantID, memberID int64, updates map[string]any) error {
	return db.Model(&models.EngineerProfile{}).
		Where("tenant_id = ? AND member_id = ?", tenantID, memberID).
		Updates(updates).Error
}

func (r *enterpriseIAMRepository) GetDepartment(db *gorm.DB, tenantID, id int64) *models.Department {
	item := &models.Department{}
	if err := db.Where("tenant_id = ? AND id = ?", tenantID, id).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *enterpriseIAMRepository) GetDepartmentByCode(db *gorm.DB, tenantID int64, code string) *models.Department {
	item := &models.Department{}
	if err := db.Where("tenant_id = ? AND department_code = ?", tenantID, strings.TrimSpace(code)).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *enterpriseIAMRepository) FindDepartmentByParentAndName(db *gorm.DB, tenantID, parentID int64, name string) *models.Department {
	item := &models.Department{}
	if err := db.Where("tenant_id = ? AND parent_id = ? AND name = ? AND status <> ?", tenantID, parentID, strings.TrimSpace(name), enums.StatusDeleted).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *enterpriseIAMRepository) CreateDepartment(db *gorm.DB, item *models.Department) error {
	return db.Create(item).Error
}

func (r *enterpriseIAMRepository) FindRootDepartment(db *gorm.DB, tenantID int64) *models.Department {
	item := &models.Department{}
	if err := db.Where("tenant_id = ? AND (department_code = ? OR depth = ?)", tenantID, "root", 0).Order("id ASC").First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *enterpriseIAMRepository) GetCustomerOrg(db *gorm.DB, tenantID, id int64) *models.CustomerOrg {
	item := &models.CustomerOrg{}
	if err := db.Where("tenant_id = ? AND id = ?", tenantID, id).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *enterpriseIAMRepository) FindCustomerOrgByName(db *gorm.DB, tenantID int64, name string) *models.CustomerOrg {
	item := &models.CustomerOrg{}
	if err := db.Where("tenant_id = ? AND name = ?", tenantID, strings.TrimSpace(name)).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *enterpriseIAMRepository) CreateCustomerOrg(db *gorm.DB, item *models.CustomerOrg) error {
	return db.Create(item).Error
}

func (r *enterpriseIAMRepository) CreateCustomerUser(db *gorm.DB, item *models.CustomerUser) error {
	return db.Create(item).Error
}

func (r *enterpriseIAMRepository) UpdateCustomerOrg(db *gorm.DB, tenantID, orgID int64, updates map[string]any) error {
	return db.Model(&models.CustomerOrg{}).
		Where("tenant_id = ? AND id = ?", tenantID, orgID).
		Updates(updates).Error
}

func (r *enterpriseIAMRepository) UpdateCustomerUser(db *gorm.DB, tenantID, customerUserID int64, updates map[string]any) error {
	return db.Model(&models.CustomerUser{}).
		Where("tenant_id = ? AND id = ?", tenantID, customerUserID).
		Updates(updates).Error
}

func (r *enterpriseIAMRepository) GetCustomerUser(db *gorm.DB, tenantID, id int64) *models.CustomerUser {
	item := &models.CustomerUser{}
	if err := db.Where("tenant_id = ? AND id = ?", tenantID, id).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *enterpriseIAMRepository) GetPartnerCompany(db *gorm.DB, tenantID, id int64) *models.PartnerCompany {
	item := &models.PartnerCompany{}
	if err := db.Where("tenant_id = ? AND id = ?", tenantID, id).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *enterpriseIAMRepository) CreatePartnerCompany(db *gorm.DB, item *models.PartnerCompany) error {
	return db.Create(item).Error
}

func (r *enterpriseIAMRepository) UpdatePartnerCompany(db *gorm.DB, tenantID, partnerCompanyID int64, updates map[string]any) error {
	return db.Model(&models.PartnerCompany{}).
		Where("tenant_id = ? AND id = ?", tenantID, partnerCompanyID).
		Updates(updates).Error
}

func (r *enterpriseIAMRepository) CreatePartnerAccount(db *gorm.DB, item *models.PartnerAccount) error {
	return db.Create(item).Error
}

func (r *enterpriseIAMRepository) FindPartnerAccountByUser(db *gorm.DB, tenantID, companyID, userID int64) *models.PartnerAccount {
	item := &models.PartnerAccount{}
	if err := db.Where("tenant_id = ? AND partner_company_id = ? AND user_id = ? AND status <> ?", tenantID, companyID, userID, enums.StatusDeleted).
		Order("id ASC").First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *enterpriseIAMRepository) UpdatePartnerAccount(db *gorm.DB, tenantID, companyID, accountID int64, updates map[string]any) error {
	return db.Model(&models.PartnerAccount{}).
		Where("tenant_id = ? AND partner_company_id = ? AND id = ?", tenantID, companyID, accountID).
		Updates(updates).Error
}

func (r *enterpriseIAMRepository) FindPartnerAccountsByCompany(db *gorm.DB, tenantID, companyID int64) ([]models.PartnerAccount, error) {
	items := make([]models.PartnerAccount, 0)
	err := db.Where("tenant_id = ? AND partner_company_id = ? AND status <> ?", tenantID, companyID, enums.StatusDeleted).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

func (r *enterpriseIAMRepository) UpdatePartnerAccountsByCompany(db *gorm.DB, tenantID, companyID int64, updates map[string]any) error {
	return db.Model(&models.PartnerAccount{}).
		Where("tenant_id = ? AND partner_company_id = ? AND status <> ?", tenantID, companyID, enums.StatusDeleted).
		Updates(updates).Error
}

func (r *enterpriseIAMRepository) UpdatePartnerAuthorizationScopesByCompany(db *gorm.DB, tenantID, companyID int64, updates map[string]any) error {
	sub := db.Model(&models.PartnerAccount{}).
		Select("id").
		Where("tenant_id = ? AND partner_company_id = ?", tenantID, companyID)
	return db.Model(&models.PartnerAuthorizationScope{}).
		Where("tenant_id = ? AND partner_account_id IN (?) AND status <> ?", tenantID, sub, enums.StatusDeleted).
		Updates(updates).Error
}

func (r *enterpriseIAMRepository) FindTenantMembers(db *gorm.DB, tenantID int64, filter EnterpriseIAMFilter, page, limit int) ([]models.TenantMember, *sqls.Paging, error) {
	items := make([]models.TenantMember, 0)
	query := db.Model(&models.TenantMember{}).Where("tenant_id = ?", tenantID)
	query = applyEnterpriseIAMFilter(query, filter, []string{"display_name", "member_no", "job_title", "member_type"})
	return findEnterpriseIAMPage(query.Order("id DESC"), page, limit, &items)
}

func (r *enterpriseIAMRepository) FindTenantMembersByIDs(db *gorm.DB, tenantID int64, ids []int64) (map[int64]*models.TenantMember, error) {
	ret := make(map[int64]*models.TenantMember, len(ids))
	if len(ids) == 0 {
		return ret, nil
	}
	items := make([]models.TenantMember, 0)
	if err := db.Where("tenant_id = ? AND id IN ?", tenantID, uniqueRepositoryInt64s(ids)).Find(&items).Error; err != nil {
		return nil, err
	}
	for i := range items {
		item := items[i]
		ret[item.ID] = &item
	}
	return ret, nil
}

func (r *enterpriseIAMRepository) FindTenantMembersByUserIDs(db *gorm.DB, tenantID int64, userIDs []int64) (map[int64]*models.TenantMember, error) {
	ret := make(map[int64]*models.TenantMember, len(userIDs))
	if db == nil || tenantID <= 0 || len(userIDs) == 0 || !db.Migrator().HasTable(&models.TenantMember{}) {
		return ret, nil
	}
	items := make([]models.TenantMember, 0)
	if err := db.Where("tenant_id = ? AND user_id IN ? AND status = ?", tenantID, uniqueRepositoryInt64s(userIDs), enums.StatusOk).Find(&items).Error; err != nil {
		return nil, err
	}
	for i := range items {
		item := items[i]
		ret[item.UserID] = &item
	}
	return ret, nil
}

func (r *enterpriseIAMRepository) FindEngineerProfilesByMemberIDs(db *gorm.DB, tenantID int64, memberIDs []int64) (map[int64]*models.EngineerProfile, error) {
	ret := make(map[int64]*models.EngineerProfile, len(memberIDs))
	if len(memberIDs) == 0 {
		return ret, nil
	}
	items := make([]models.EngineerProfile, 0)
	if err := db.Where("tenant_id = ? AND member_id IN ?", tenantID, memberIDs).Find(&items).Error; err != nil {
		return nil, err
	}
	for i := range items {
		item := items[i]
		ret[item.MemberID] = &item
	}
	return ret, nil
}

func (r *enterpriseIAMRepository) FindEngineerProfileByMemberID(db *gorm.DB, tenantID, memberID int64) (*models.EngineerProfile, error) {
	if db == nil || tenantID <= 0 || memberID <= 0 || !db.Migrator().HasTable(&models.EngineerProfile{}) {
		return nil, nil
	}
	var item models.EngineerProfile
	err := db.Where("tenant_id = ? AND member_id = ?", tenantID, memberID).First(&item).Error
	switch {
	case err == nil:
		return &item, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, nil
	default:
		return nil, err
	}
}

func (r *enterpriseIAMRepository) FindDepartments(db *gorm.DB, tenantID int64, filter EnterpriseIAMFilter, page, limit int) ([]models.Department, *sqls.Paging, error) {
	items := make([]models.Department, 0)
	query := db.Model(&models.Department{}).Where("tenant_id = ?", tenantID)
	query = applyEnterpriseIAMFilter(query, filter, []string{"department_code", "name", "path", "region_code"})
	return findEnterpriseIAMPage(query.Order("sort_no ASC, id ASC"), page, limit, &items)
}

func (r *enterpriseIAMRepository) FindDepartmentsByIDs(db *gorm.DB, tenantID int64, ids []int64) (map[int64]*models.Department, error) {
	ret := make(map[int64]*models.Department, len(ids))
	if len(ids) == 0 {
		return ret, nil
	}
	items := make([]models.Department, 0)
	if err := db.Where("tenant_id = ? AND id IN ?", tenantID, uniqueRepositoryInt64s(ids)).Find(&items).Error; err != nil {
		return nil, err
	}
	for i := range items {
		item := items[i]
		ret[item.ID] = &item
	}
	return ret, nil
}

func (r *enterpriseIAMRepository) CountMembersByDepartmentIDs(db *gorm.DB, tenantID int64, departmentIDs []int64) (map[int64]int64, error) {
	ret := make(map[int64]int64, len(departmentIDs))
	if len(departmentIDs) == 0 {
		return ret, nil
	}
	var rows []struct {
		DepartmentID int64
		Count        int64
	}
	err := db.Model(&models.TenantMember{}).
		Select("department_id, COUNT(*) AS count").
		Where("tenant_id = ? AND department_id IN ?", tenantID, uniqueRepositoryInt64s(departmentIDs)).
		Group("department_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		ret[row.DepartmentID] = row.Count
	}
	return ret, nil
}

func (r *enterpriseIAMRepository) FindCustomerUsers(db *gorm.DB, tenantID int64, filter EnterpriseIAMFilter, page, limit int) ([]models.CustomerUser, *sqls.Paging, error) {
	items := make([]models.CustomerUser, 0)
	query := db.Model(&models.CustomerUser{}).Where("tenant_id = ?", tenantID)
	query = applyEnterpriseIAMFilter(query, filter, []string{"display_name", "email", "phone", "locale"})
	return findEnterpriseIAMPage(query.Order("id DESC"), page, limit, &items)
}

func (r *enterpriseIAMRepository) FindCustomerOrgsByIDs(db *gorm.DB, tenantID int64, ids []int64) (map[int64]*models.CustomerOrg, error) {
	ret := make(map[int64]*models.CustomerOrg, len(ids))
	if len(ids) == 0 {
		return ret, nil
	}
	items := make([]models.CustomerOrg, 0)
	if err := db.Where("tenant_id = ? AND id IN ?", tenantID, uniqueRepositoryInt64s(ids)).Find(&items).Error; err != nil {
		return nil, err
	}
	for i := range items {
		item := items[i]
		ret[item.ID] = &item
	}
	return ret, nil
}

func (r *enterpriseIAMRepository) FindPartnerCompanies(db *gorm.DB, tenantID int64, filter EnterpriseIAMFilter, page, limit int) ([]models.PartnerCompany, *sqls.Paging, error) {
	items := make([]models.PartnerCompany, 0)
	query := db.Model(&models.PartnerCompany{}).Where("tenant_id = ?", tenantID)
	query = applyEnterpriseIAMFilter(query, filter, []string{"partner_no", "name", "partner_type", "country_region", "contact_name"})
	return findEnterpriseIAMPage(query.Order("id DESC"), page, limit, &items)
}

func (r *enterpriseIAMRepository) FindPartnerCompaniesByIDs(db *gorm.DB, tenantID int64, ids []int64) (map[int64]*models.PartnerCompany, error) {
	ret := make(map[int64]*models.PartnerCompany, len(ids))
	if tenantID <= 0 || len(ids) == 0 {
		return ret, nil
	}
	items := make([]models.PartnerCompany, 0)
	if err := db.Where("tenant_id = ? AND id IN ?", tenantID, uniqueRepositoryInt64s(ids)).Find(&items).Error; err != nil {
		return nil, err
	}
	for i := range items {
		item := items[i]
		ret[item.ID] = &item
	}
	return ret, nil
}

func (r *enterpriseIAMRepository) CountPartnerAccountsByCompanyIDs(db *gorm.DB, tenantID int64, companyIDs []int64) (map[int64]int64, error) {
	return countEnterpriseIAMByCompanyIDs(db.Model(&models.PartnerAccount{}), tenantID, companyIDs)
}

func (r *enterpriseIAMRepository) CountPartnerContractsByCompanyIDs(db *gorm.DB, tenantID int64, companyIDs []int64) (map[int64]int64, error) {
	return countEnterpriseIAMByCompanyIDs(db.Model(&models.PartnerContract{}), tenantID, companyIDs)
}

func (r *enterpriseIAMRepository) FindRoleBindingsBySubjects(db *gorm.DB, tenantID int64, domainType, subjectType string, subjectIDs []int64) ([]models.AuthRoleBinding, error) {
	items := make([]models.AuthRoleBinding, 0)
	if len(subjectIDs) == 0 {
		return items, nil
	}
	now := time.Now()
	err := db.Where("tenant_id = ? AND domain_type = ? AND subject_type = ? AND subject_id IN ? AND status = ?", tenantID, domainType, subjectType, uniqueRepositoryInt64s(subjectIDs), enums.StatusOk).
		Where("(effective_at IS NULL OR effective_at <= ?) AND (expired_at IS NULL OR expired_at > ?)", now, now).
		Find(&items).Error
	return items, err
}

func applyEnterpriseIAMFilter(query *gorm.DB, filter EnterpriseIAMFilter, searchColumns []string) *gorm.DB {
	if status := strings.TrimSpace(filter.Status); status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}
	search := strings.TrimSpace(filter.Search)
	if search == "" || len(searchColumns) == 0 {
		return query
	}
	like := "%" + search + "%"
	parts := make([]string, 0, len(searchColumns))
	args := make([]any, 0, len(searchColumns))
	for _, column := range searchColumns {
		parts = append(parts, column+" LIKE ?")
		args = append(args, like)
	}
	return query.Where("("+strings.Join(parts, " OR ")+")", args...)
}

func findEnterpriseIAMPage[T any](query *gorm.DB, page, limit int, dest *[]T) ([]T, *sqls.Paging, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, nil, err
	}
	if err := query.Offset((page - 1) * limit).Limit(limit).Find(dest).Error; err != nil {
		return nil, nil, err
	}
	return *dest, &sqls.Paging{Page: page, Limit: limit, Total: total}, nil
}

func countEnterpriseIAMByCompanyIDs(query *gorm.DB, tenantID int64, companyIDs []int64) (map[int64]int64, error) {
	ret := make(map[int64]int64, len(companyIDs))
	if len(companyIDs) == 0 {
		return ret, nil
	}
	var rows []struct {
		PartnerCompanyID int64
		Count            int64
	}
	err := query.
		Select("partner_company_id, COUNT(*) AS count").
		Where("tenant_id = ? AND partner_company_id IN ?", tenantID, uniqueRepositoryInt64s(companyIDs)).
		Group("partner_company_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		ret[row.PartnerCompanyID] = row.Count
	}
	return ret, nil
}

func uniqueRepositoryInt64s(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	ret := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		ret = append(ret, value)
	}
	return ret
}
