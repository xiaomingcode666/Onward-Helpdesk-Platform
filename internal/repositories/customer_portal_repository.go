package repositories

import (
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"gorm.io/gorm"
)

var CustomerPortalRepository = newCustomerPortalRepository()

type customerPortalRepository struct{}

func newCustomerPortalRepository() *customerPortalRepository {
	return &customerPortalRepository{}
}

func (r *customerPortalRepository) FindActiveBindings(db *gorm.DB, customerUserID, customerOrgID int64) []models.CustomerDeviceBinding {
	if customerUserID <= 0 && customerOrgID <= 0 {
		return nil
	}
	var list []models.CustomerDeviceBinding
	visibleCustomerDeviceBindings(db, customerUserID, customerOrgID).
		Order("confirmed_at DESC").Order("id DESC").Find(&list)
	return list
}

func (r *customerPortalRepository) FindCustomerUserByUserID(db *gorm.DB, userID int64) *models.CustomerUser {
	if userID <= 0 || !db.Migrator().HasTable(&models.CustomerUser{}) {
		return nil
	}
	var item models.CustomerUser
	if err := db.Where("user_id = ? AND status = ?", userID, enums.StatusOk).Order("id DESC").First(&item).Error; err != nil {
		return nil
	}
	return &item
}

func (r *customerPortalRepository) FindUserByCustomerEmail(db *gorm.DB, email string) *models.User {
	if db == nil || strings.TrimSpace(email) == "" || !db.Migrator().HasTable(&models.CustomerUser{}) {
		return nil
	}
	var item models.User
	if err := db.Model(&models.User{}).
		Joins("JOIN customer_users ON customer_users.user_id = users.id").
		Joins("LEFT JOIN customer_identities ON customer_identities.external_source = ? AND customer_identities.external_id = CAST(users.id AS TEXT) AND customer_identities.status = ?", enums.ExternalSourceUser, enums.StatusOk).
		Joins("LEFT JOIN customers ON customers.id = customer_identities.customer_id").
		Where("((LOWER(customer_users.email) = LOWER(?) AND customer_users.status = ?) OR (LOWER(customers.primary_email) = LOWER(?) AND customer_identities.id IS NOT NULL)) AND users.status = ?", strings.TrimSpace(email), enums.StatusOk, strings.TrimSpace(email), enums.StatusOk).
		Order("customer_users.id DESC").First(&item).Error; err != nil {
		return nil
	}
	return &item
}

func (r *customerPortalRepository) FindCustomerUserByTenantAndUserID(db *gorm.DB, tenantID, userID int64) *models.CustomerUser {
	if tenantID <= 0 || userID <= 0 || !db.Migrator().HasTable(&models.CustomerUser{}) {
		return nil
	}
	var item models.CustomerUser
	if err := db.Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, enums.StatusOk).
		Order("id DESC").First(&item).Error; err != nil {
		return nil
	}
	return &item
}

func (r *customerPortalRepository) FindCustomerUserByTenantOrgAndUserID(db *gorm.DB, tenantID, customerOrgID, userID int64) *models.CustomerUser {
	if tenantID <= 0 || customerOrgID <= 0 || userID <= 0 || !db.Migrator().HasTable(&models.CustomerUser{}) {
		return nil
	}
	var item models.CustomerUser
	if err := db.Where("tenant_id = ? AND customer_org_id = ? AND user_id = ?", tenantID, customerOrgID, userID).
		Order("id DESC").First(&item).Error; err != nil {
		return nil
	}
	return &item
}

func (r *customerPortalRepository) FindCustomerUsersByIDs(db *gorm.DB, ids []int64) ([]models.CustomerUser, error) {
	items := make([]models.CustomerUser, 0)
	ids = uniqueRepositoryInt64s(ids)
	if db == nil || len(ids) == 0 || !db.Migrator().HasTable(&models.CustomerUser{}) {
		return items, nil
	}
	err := db.Where("id IN ? AND status = ?", ids, enums.StatusOk).Find(&items).Error
	return items, err
}

func (r *customerPortalRepository) FindCustomerUsersByTenantIDsAndUserIDs(db *gorm.DB, tenantIDs, userIDs []int64) ([]models.CustomerUser, error) {
	items := make([]models.CustomerUser, 0)
	tenantIDs = uniqueRepositoryInt64s(tenantIDs)
	userIDs = uniqueRepositoryInt64s(userIDs)
	if db == nil || len(tenantIDs) == 0 || len(userIDs) == 0 || !db.Migrator().HasTable(&models.CustomerUser{}) {
		return items, nil
	}
	err := db.Where("tenant_id IN ? AND user_id IN ? AND status = ?", tenantIDs, userIDs, enums.StatusOk).
		Order("id DESC").
		Find(&items).Error
	return items, err
}

func (r *customerPortalRepository) TouchCustomerUserLastSeenAt(db *gorm.DB, tenantID, customerUserID, userID int64, seenAt time.Time) (bool, error) {
	if db == nil || tenantID <= 0 || customerUserID <= 0 || userID <= 0 || seenAt.IsZero() || !db.Migrator().HasTable(&models.CustomerUser{}) {
		return false, nil
	}
	result := db.Model(&models.CustomerUser{}).
		Where("tenant_id = ? AND id = ? AND user_id = ? AND status = ?", tenantID, customerUserID, userID, enums.StatusOk).
		UpdateColumn("last_seen_at", seenAt)
	return result.RowsAffected > 0, result.Error
}

func (r *customerPortalRepository) FindCustomerOrgByCustomerNo(db *gorm.DB, tenantID int64, customerNo string) *models.CustomerOrg {
	if tenantID <= 0 || customerNo == "" || !db.Migrator().HasTable(&models.CustomerOrg{}) {
		return nil
	}
	var item models.CustomerOrg
	if err := db.Where("tenant_id = ? AND customer_no = ?", tenantID, customerNo).First(&item).Error; err != nil {
		return nil
	}
	return &item
}

func (r *customerPortalRepository) GetActiveCustomerOrg(db *gorm.DB, customerOrgID int64) *models.CustomerOrg {
	if customerOrgID <= 0 || !db.Migrator().HasTable(&models.CustomerOrg{}) {
		return nil
	}
	var item models.CustomerOrg
	if err := db.Where("id = ? AND status = ?", customerOrgID, enums.StatusOk).First(&item).Error; err != nil {
		return nil
	}
	return &item
}

func (r *customerPortalRepository) FindDevicesByTenantAndIDs(db *gorm.DB, tenantIDs []int64, deviceIDs []int64) []models.Device {
	if len(tenantIDs) == 0 || len(deviceIDs) == 0 {
		return nil
	}
	var list []models.Device
	db.Where("tenant_id IN ? AND id IN ?", tenantIDs, deviceIDs).Order("updated_at DESC").Find(&list)
	return list
}

func (r *customerPortalRepository) FindVisibleTickets(db *gorm.DB, tenantIDs, unscopedTenantIDs []int64, customerID int64, deviceIDs, conversationIDs []int64) []models.Ticket {
	if customerID <= 0 || len(tenantIDs) == 0 {
		return nil
	}
	var list []models.Ticket
	query := db.Where("tenant_id IN ? AND customer_id = ?", tenantIDs, customerID)
	switch {
	case len(deviceIDs) > 0 && len(conversationIDs) > 0 && len(unscopedTenantIDs) > 0:
		query = query.Where("(device_id IN ? OR conversation_id IN ? OR (tenant_id IN ? AND product_id = 0 AND device_id = 0 AND conversation_id = 0))", deviceIDs, conversationIDs, unscopedTenantIDs)
	case len(deviceIDs) > 0 && len(conversationIDs) > 0:
		query = query.Where("(device_id IN ? OR conversation_id IN ?)", deviceIDs, conversationIDs)
	case len(deviceIDs) > 0 && len(unscopedTenantIDs) > 0:
		query = query.Where("(device_id IN ? OR (tenant_id IN ? AND product_id = 0 AND device_id = 0 AND conversation_id = 0))", deviceIDs, unscopedTenantIDs)
	case len(deviceIDs) > 0:
		query = query.Where("device_id IN ?", deviceIDs)
	case len(conversationIDs) > 0 && len(unscopedTenantIDs) > 0:
		query = query.Where("(conversation_id IN ? OR (tenant_id IN ? AND product_id = 0 AND device_id = 0 AND conversation_id = 0))", conversationIDs, unscopedTenantIDs)
	case len(conversationIDs) > 0:
		query = query.Where("conversation_id IN ?", conversationIDs)
	case len(unscopedTenantIDs) > 0:
		query = query.Where("tenant_id IN ? AND product_id = 0 AND device_id = 0 AND conversation_id = 0", unscopedTenantIDs)
	default:
		return nil
	}
	query.
		Order("updated_at DESC").
		Order("id DESC").
		Find(&list)
	return list
}

func (r *customerPortalRepository) FindVisibleConversations(db *gorm.DB, tenantIDs []int64, customerID int64, externalParticipantIDs []string) []models.Conversation {
	if customerID <= 0 || (len(tenantIDs) == 0 && len(externalParticipantIDs) == 0) {
		return nil
	}
	conversationIDs := make([]int64, 0)
	if len(externalParticipantIDs) > 0 {
		db.Model(&models.ConversationParticipant{}).
			Where("participant_type = ? AND status = ? AND external_participant_id IN ?", enums.IMParticipantTypeCustomer, enums.StatusOk, externalParticipantIDs).
			Pluck("conversation_id", &conversationIDs)
	}
	var list []models.Conversation
	query := db.Where("customer_id = ?", customerID)
	switch {
	case len(tenantIDs) > 0 && len(conversationIDs) > 0:
		query = query.Where("(tenant_id IN ? OR id IN ?)", tenantIDs, conversationIDs)
	case len(tenantIDs) > 0:
		query = query.Where("tenant_id IN ?", tenantIDs)
	default:
		query = query.Where("id IN ?", conversationIDs)
	}
	query.
		Order("last_active_at DESC").
		Order("id DESC").
		Find(&list)
	return list
}

func (r *customerPortalRepository) FindVisibleRepairRecords(db *gorm.DB, tenantIDs []int64, deviceIDs []int64) []models.DeviceServiceRecord {
	if len(tenantIDs) == 0 || len(deviceIDs) == 0 {
		return nil
	}
	var list []models.DeviceServiceRecord
	db.Where("tenant_id IN ? AND device_id IN ? AND visible_to_customer = ?", tenantIDs, deviceIDs, true).
		Order("occurred_at DESC").
		Order("id DESC").
		Find(&list)
	return list
}

func (r *customerPortalRepository) FindVisibleMeetings(db *gorm.DB, tenantIDs []int64, ticketIDs []string) []models.MeetingRoomJitsi {
	if len(tenantIDs) == 0 || len(ticketIDs) == 0 {
		return nil
	}
	var list []models.MeetingRoomJitsi
	db.Where("tenant_id IN ? AND ticket_id IN ?", tenantIDs, ticketIDs).Order("created_at DESC").Find(&list)
	return list
}
