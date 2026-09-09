package repositories

import (
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var CustomerRepository = newCustomerRepository()

func newCustomerRepository() *customerRepository {
	return &customerRepository{}
}

type customerRepository struct {
}

type CompanyCustomerCount struct {
	CompanyID int64 `gorm:"column:company_id"`
	Count     int64 `gorm:"column:cnt"`
}

type TenantCustomerOption struct {
	CustomerID      int64
	CustomerUserID  int64
	CustomerOrgID   int64
	CustomerOrgName string
	DisplayName     string
	Email           string
	Phone           string
}

func (r *customerRepository) FindTenantCustomerOptions(db *gorm.DB, tenantID int64, search string, limit int) ([]TenantCustomerOption, error) {
	items := make([]TenantCustomerOption, 0)
	if db == nil || tenantID <= 0 {
		return items, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	customerTable := db.NamingStrategy.TableName("Customer")
	identityTable := db.NamingStrategy.TableName("CustomerIdentity")
	query := db.Table("customer_users AS cu").
		Select(`ci.customer_id AS customer_id, cu.id AS customer_user_id, cu.customer_org_id,
			co.name AS customer_org_name, COALESCE(NULLIF(cu.display_name, ''), c.name) AS display_name,
			COALESCE(NULLIF(cu.email, ''), c.primary_email) AS email,
			COALESCE(NULLIF(cu.phone, ''), c.primary_mobile) AS phone`).
		Joins("JOIN "+identityTable+" AS ci ON ci.external_source = ? AND ci.external_id = CAST(cu.user_id AS TEXT) AND ci.status = ?", enums.ExternalSourceUser, enums.StatusOk).
		Joins("JOIN "+customerTable+" AS c ON c.id = ci.customer_id AND c.status = ?", enums.StatusOk).
		Joins("LEFT JOIN customer_orgs AS co ON co.id = cu.customer_org_id AND co.tenant_id = cu.tenant_id AND co.status = ?", enums.StatusOk).
		Where("cu.tenant_id = ? AND cu.status = ?", tenantID, enums.StatusOk)
	if keyword := strings.TrimSpace(search); keyword != "" {
		pattern := "%" + strings.ToLower(keyword) + "%"
		query = query.Where(`(LOWER(cu.display_name) LIKE ? OR LOWER(cu.email) LIKE ? OR LOWER(cu.phone) LIKE ? OR LOWER(co.name) LIKE ?)`, pattern, pattern, pattern, pattern)
	}
	err := query.Order("cu.id DESC").Limit(limit).Scan(&items).Error
	return items, err
}

func (r *customerRepository) CountByCompanyIDs(db *gorm.DB, companyIDs []int64, excludeStatus int) map[int64]int64 {
	ret := make(map[int64]int64)
	if len(companyIDs) == 0 {
		return ret
	}

	rows := make([]CompanyCustomerCount, 0, len(companyIDs))
	db.Model(&models.Customer{}).
		Select("company_id, count(1) as cnt").
		Where("company_id in ?", companyIDs).
		Where("status <> ?", excludeStatus).
		Group("company_id").
		Scan(&rows)

	for _, row := range rows {
		ret[row.CompanyID] = row.Count
	}
	return ret
}

func (r *customerRepository) Get(db *gorm.DB, id int64) *models.Customer {
	ret := &models.Customer{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

// HasTenantAssociation reports whether a legacy global customer has already
// been used in tenant-owned business data for the given tenant.
func (r *customerRepository) HasTenantAssociation(db *gorm.DB, customerID, tenantID int64) bool {
	if customerID <= 0 || tenantID <= 0 {
		return false
	}
	return r.countTenantAssociations(db, customerID, "tenant_id = ?", tenantID) > 0
}

// HasOtherTenantAssociation prevents a customer that is already scoped by
// tenant-owned records from being attached across tenant boundaries.
func (r *customerRepository) HasOtherTenantAssociation(db *gorm.DB, customerID, tenantID int64) bool {
	if customerID <= 0 || tenantID <= 0 {
		return false
	}
	return r.countTenantAssociations(db, customerID, "tenant_id <> ?", tenantID) > 0
}

func (r *customerRepository) countTenantAssociations(db *gorm.DB, customerID int64, tenantWhere string, tenantID int64) int64 {
	var total int64
	var count int64
	db.Model(&models.Ticket{}).Where("customer_id = ?", customerID).Where(tenantWhere, tenantID).Count(&count)
	total += count
	count = 0
	db.Model(&models.Conversation{}).Where("customer_id = ?", customerID).Where(tenantWhere, tenantID).Count(&count)
	total += count
	count = 0
	db.Model(&models.CustomerDeviceBinding{}).Where("customer_org_id = ?", customerID).Where(tenantWhere, tenantID).Count(&count)
	return total + count
}

func (r *customerRepository) Take(db *gorm.DB, where ...interface{}) *models.Customer {
	ret := &models.Customer{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *customerRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.Customer) {
	cnd.Find(db, &list)
	return
}

func (r *customerRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.Customer {
	ret := &models.Customer{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *customerRepository) FindPageByParams(db *gorm.DB, params *params.QueryParams) (list []models.Customer, paging *sqls.Paging) {
	return r.FindPageByCnd(db, &params.Cnd)
}

func (r *customerRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.Customer, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.Customer{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *customerRepository) FindBySql(db *gorm.DB, sqlStr string, paramArr ...interface{}) (list []models.Customer) {
	db.Raw(sqlStr, paramArr...).Scan(&list)
	return
}

func (r *customerRepository) CountBySql(db *gorm.DB, sqlStr string, paramArr ...interface{}) (count int64) {
	db.Raw(sqlStr, paramArr...).Count(&count)
	return
}

func (r *customerRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.Customer{})
}

func (r *customerRepository) Create(db *gorm.DB, t *models.Customer) (err error) {
	err = db.Create(t).Error
	return
}

func (r *customerRepository) Update(db *gorm.DB, t *models.Customer) (err error) {
	err = db.Save(t).Error
	return
}

func (r *customerRepository) Updates(db *gorm.DB, id int64, columns map[string]interface{}) (err error) {
	err = db.Model(&models.Customer{}).Where("id = ?", id).Updates(columns).Error
	return
}

func (r *customerRepository) UpdateColumn(db *gorm.DB, id int64, name string, value interface{}) (err error) {
	err = db.Model(&models.Customer{}).Where("id = ?", id).UpdateColumn(name, value).Error
	return
}

func (r *customerRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.Customer{}, "id = ?", id)
}
