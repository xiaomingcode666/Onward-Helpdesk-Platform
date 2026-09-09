package repositories

import (
	"strconv"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var TicketSupplierCollaborationRepository = newTicketSupplierCollaborationRepository()

type ticketSupplierCollaborationRepository struct{}

func newTicketSupplierCollaborationRepository() *ticketSupplierCollaborationRepository {
	return &ticketSupplierCollaborationRepository{}
}

func (r *ticketSupplierCollaborationRepository) Get(db *gorm.DB, id int64) *models.TicketSupplierCollaboration {
	item := &models.TicketSupplierCollaboration{}
	if err := db.First(item, "id = ?", id).Error; err != nil {
		return nil
	}
	return item
}

func (r *ticketSupplierCollaborationRepository) GetForUpdate(db *gorm.DB, id int64) *models.TicketSupplierCollaboration {
	item := &models.TicketSupplierCollaboration{}
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(item, "id = ?", id).Error; err != nil {
		return nil
	}
	return item
}

func (r *ticketSupplierCollaborationRepository) FindByTicket(db *gorm.DB, tenantID, ticketID int64) []models.TicketSupplierCollaboration {
	items := make([]models.TicketSupplierCollaboration, 0)
	db.Where("tenant_id = ? AND ticket_id = ? AND record_status <> ?", tenantID, ticketID, enums.StatusDeleted).
		Order("id DESC").Find(&items)
	return items
}

func (r *ticketSupplierCollaborationRepository) FindByPartnerAccount(db *gorm.DB, tenantID, partnerAccountID int64) []models.TicketSupplierCollaboration {
	items := make([]models.TicketSupplierCollaboration, 0)
	db.Where("tenant_id = ? AND partner_account_id = ? AND record_status <> ?", tenantID, partnerAccountID, enums.StatusDeleted).
		Order("id DESC").Find(&items)
	return items
}

func (r *ticketSupplierCollaborationRepository) FindByPartnerCompany(db *gorm.DB, tenantID, partnerCompanyID int64) []models.TicketSupplierCollaboration {
	items := make([]models.TicketSupplierCollaboration, 0)
	db.Where("tenant_id = ? AND partner_company_id = ? AND record_status <> ?", tenantID, partnerCompanyID, enums.StatusDeleted).
		Order("id DESC").Find(&items)
	return items
}

func (r *ticketSupplierCollaborationRepository) FindForPerformance(db *gorm.DB, tenantID int64, startDate, endDate time.Time) ([]models.TicketSupplierCollaboration, error) {
	items := make([]models.TicketSupplierCollaboration, 0)
	err := db.Where("tenant_id = ? AND invited_at BETWEEN ? AND ? AND record_status <> ?", tenantID, startDate, endDate, enums.StatusDeleted).
		Order("partner_company_id ASC, ticket_id ASC, invited_at ASC, id ASC").
		Find(&items).Error
	return items, err
}

func (r *ticketSupplierCollaborationRepository) FindInvitedBefore(db *gorm.DB, invitedBefore time.Time, limit int) ([]models.TicketSupplierCollaboration, error) {
	items := make([]models.TicketSupplierCollaboration, 0)
	if db == nil {
		return items, nil
	}
	if limit <= 0 {
		limit = 100
	}
	err := db.Where("status = ? AND invited_at <= ? AND accepted_at IS NULL AND record_status <> ?", "invited", invitedBefore, enums.StatusDeleted).
		Order("invited_at ASC, id ASC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

func (r *ticketSupplierCollaborationRepository) FindAuthorizationExpiredBefore(db *gorm.DB, expiredBefore time.Time, limit int) ([]models.TicketSupplierCollaboration, error) {
	items := make([]models.TicketSupplierCollaboration, 0)
	if db == nil {
		return items, nil
	}
	if limit <= 0 {
		limit = 100
	}
	err := db.Where("status IN ? AND authorization_ends IS NOT NULL AND authorization_ends <= ? AND record_status <> ?",
		[]string{"invited", "accepted", "processing"}, expiredBefore, enums.StatusDeleted).
		Order("authorization_ends ASC, id ASC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

func (r *ticketSupplierCollaborationRepository) FindByParticipantAccount(db *gorm.DB, tenantID, partnerAccountID int64) []models.TicketSupplierCollaboration {
	items := make([]models.TicketSupplierCollaboration, 0)
	db.Model(&models.TicketSupplierCollaboration{}).
		Distinct("ticket_supplier_collaborations.*").
		Joins("JOIN ticket_supplier_collaboration_participants ON ticket_supplier_collaboration_participants.collaboration_id = ticket_supplier_collaborations.id").
		Where("ticket_supplier_collaborations.tenant_id = ? AND ticket_supplier_collaboration_participants.tenant_id = ? AND ticket_supplier_collaboration_participants.partner_account_id = ? AND ticket_supplier_collaboration_participants.status = ? AND ticket_supplier_collaborations.record_status <> ?",
			tenantID, tenantID, partnerAccountID, enums.StatusOk, enums.StatusDeleted).
		Order("ticket_supplier_collaborations.id DESC").Find(&items)
	return items
}

func (r *ticketSupplierCollaborationRepository) FindByParticipantAccountAnyStatus(db *gorm.DB, tenantID, partnerAccountID int64) []models.TicketSupplierCollaboration {
	items := make([]models.TicketSupplierCollaboration, 0)
	db.Model(&models.TicketSupplierCollaboration{}).
		Distinct("ticket_supplier_collaborations.*").
		Joins("JOIN ticket_supplier_collaboration_participants ON ticket_supplier_collaboration_participants.collaboration_id = ticket_supplier_collaborations.id").
		Where("ticket_supplier_collaborations.tenant_id = ? AND ticket_supplier_collaboration_participants.tenant_id = ? AND ticket_supplier_collaboration_participants.partner_account_id = ? AND ticket_supplier_collaboration_participants.status <> ? AND ticket_supplier_collaborations.record_status <> ?",
			tenantID, tenantID, partnerAccountID, enums.StatusDeleted, enums.StatusDeleted).
		Order("ticket_supplier_collaborations.id DESC").Find(&items)
	return items
}

func (r *ticketSupplierCollaborationRepository) FindParticipants(db *gorm.DB, tenantID, collaborationID int64) []models.TicketSupplierCollaborationParticipant {
	items := make([]models.TicketSupplierCollaborationParticipant, 0)
	db.Where("tenant_id = ? AND collaboration_id = ? AND status <> ?", tenantID, collaborationID, enums.StatusDeleted).
		Order("CASE WHEN role = 'owner' THEN 0 ELSE 1 END, joined_at ASC, id ASC").Find(&items)
	return items
}

func (r *ticketSupplierCollaborationRepository) FindActiveParticipant(db *gorm.DB, tenantID, collaborationID, partnerAccountID int64) *models.TicketSupplierCollaborationParticipant {
	item := &models.TicketSupplierCollaborationParticipant{}
	if err := db.Where("tenant_id = ? AND collaboration_id = ? AND partner_account_id = ? AND status = ?", tenantID, collaborationID, partnerAccountID, enums.StatusOk).
		First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *ticketSupplierCollaborationRepository) FindParticipantAnyStatus(db *gorm.DB, tenantID, collaborationID, partnerAccountID int64) *models.TicketSupplierCollaborationParticipant {
	item := &models.TicketSupplierCollaborationParticipant{}
	if err := db.Where("tenant_id = ? AND collaboration_id = ? AND partner_account_id = ? AND status <> ?", tenantID, collaborationID, partnerAccountID, enums.StatusDeleted).
		First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *ticketSupplierCollaborationRepository) CreateParticipant(db *gorm.DB, item *models.TicketSupplierCollaborationParticipant) error {
	return db.Create(item).Error
}

func (r *ticketSupplierCollaborationRepository) UpdateParticipant(db *gorm.DB, id int64, updates map[string]any) error {
	return db.Model(&models.TicketSupplierCollaborationParticipant{}).Where("id = ?", id).Updates(updates).Error
}

func (r *ticketSupplierCollaborationRepository) UpdateParticipants(db *gorm.DB, tenantID, collaborationID int64, updates map[string]any) error {
	return db.Model(&models.TicketSupplierCollaborationParticipant{}).
		Where("tenant_id = ? AND collaboration_id = ? AND status <> ?", tenantID, collaborationID, enums.StatusDeleted).
		Updates(updates).Error
}

func (r *ticketSupplierCollaborationRepository) DisableActiveParticipants(db *gorm.DB, tenantID, collaborationID int64, leftAt time.Time, userID int64, username string) error {
	return db.Model(&models.TicketSupplierCollaborationParticipant{}).
		Where("tenant_id = ? AND collaboration_id = ? AND status = ?", tenantID, collaborationID, enums.StatusOk).
		Updates(map[string]any{
			"left_at":          leftAt,
			"status":           enums.StatusDisabled,
			"updated_at":       leftAt,
			"update_user_id":   userID,
			"update_user_name": username,
		}).Error
}

func (r *ticketSupplierCollaborationRepository) DisableParticipantsByTicket(db *gorm.DB, tenantID, ticketID int64, leftAt time.Time, userID int64, username string) error {
	activeCollaborations := db.Model(&models.TicketSupplierCollaboration{}).
		Select("id").Where("tenant_id = ? AND ticket_id = ? AND record_status <> ?", tenantID, ticketID, enums.StatusDeleted)
	return db.Model(&models.TicketSupplierCollaborationParticipant{}).
		Where("tenant_id = ? AND collaboration_id IN (?) AND status = ?", tenantID, activeCollaborations, enums.StatusOk).
		Updates(map[string]any{
			"left_at":          leftAt,
			"status":           enums.StatusDisabled,
			"updated_at":       leftAt,
			"update_user_id":   userID,
			"update_user_name": username,
		}).Error
}

func (r *ticketSupplierCollaborationRepository) CountActiveParticipantByTicket(db *gorm.DB, tenantID, ticketID, partnerAccountID int64) int64 {
	var count int64
	db.Model(&models.TicketSupplierCollaborationParticipant{}).
		Joins("JOIN ticket_supplier_collaborations ON ticket_supplier_collaborations.id = ticket_supplier_collaboration_participants.collaboration_id").
		Where("ticket_supplier_collaboration_participants.tenant_id = ? AND ticket_supplier_collaboration_participants.partner_account_id = ? AND ticket_supplier_collaboration_participants.status = ? AND ticket_supplier_collaborations.ticket_id = ? AND ticket_supplier_collaborations.status IN ? AND ticket_supplier_collaborations.record_status <> ?",
			tenantID, partnerAccountID, enums.StatusOk, ticketID, []string{"invited", "accepted", "processing"}, enums.StatusDeleted).
		Count(&count)
	return count
}

func (r *ticketSupplierCollaborationRepository) FindActive(db *gorm.DB, tenantID, ticketID, moduleID, partnerCompanyID int64) *models.TicketSupplierCollaboration {
	item := &models.TicketSupplierCollaboration{}
	err := db.Where("tenant_id = ? AND ticket_id = ? AND product_module_id = ? AND partner_company_id = ? AND status IN ? AND record_status <> ?",
		tenantID, ticketID, moduleID, partnerCompanyID, []string{"invited", "accepted", "processing"}, enums.StatusDeleted).
		First(item).Error
	if err != nil {
		return nil
	}
	return item
}

func (r *ticketSupplierCollaborationRepository) CountActiveByTicket(db *gorm.DB, tenantID, ticketID int64) int64 {
	var count int64
	db.Model(&models.TicketSupplierCollaboration{}).
		Where("tenant_id = ? AND ticket_id = ? AND status IN ? AND record_status <> ?", tenantID, ticketID, []string{"invited", "accepted", "processing"}, enums.StatusDeleted).
		Count(&count)
	return count
}

func (r *ticketSupplierCollaborationRepository) CountActiveByPartnerTicket(db *gorm.DB, tenantID, ticketID, partnerAccountID int64) int64 {
	var count int64
	db.Model(&models.TicketSupplierCollaboration{}).
		Where("tenant_id = ? AND ticket_id = ? AND partner_account_id = ? AND status IN ? AND record_status <> ?",
			tenantID, ticketID, partnerAccountID, []string{"invited", "accepted", "processing"}, enums.StatusDeleted).
		Count(&count)
	return count
}

func (r *ticketSupplierCollaborationRepository) Create(db *gorm.DB, item *models.TicketSupplierCollaboration) error {
	return db.Create(item).Error
}

func (r *ticketSupplierCollaborationRepository) Updates(db *gorm.DB, id int64, updates map[string]any) error {
	return db.Model(&models.TicketSupplierCollaboration{}).Where("id = ?", id).Updates(updates).Error
}

func (r *ticketSupplierCollaborationRepository) GetPartnerCompany(db *gorm.DB, tenantID, id int64) *models.PartnerCompany {
	item := &models.PartnerCompany{}
	if err := db.First(item, "id = ? AND tenant_id = ? AND status = ?", id, tenantID, enums.StatusOk).Error; err != nil {
		return nil
	}
	return item
}

func (r *ticketSupplierCollaborationRepository) GetPartnerAccount(db *gorm.DB, tenantID, companyID, accountID int64) *models.PartnerAccount {
	item := &models.PartnerAccount{}
	query := db.Where("tenant_id = ? AND status = ?", tenantID, enums.StatusOk)
	if companyID > 0 {
		query = query.Where("partner_company_id = ?", companyID)
	}
	if accountID > 0 {
		query = query.Where("id = ?", accountID)
	}
	if err := query.Order("id ASC").First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *ticketSupplierCollaborationRepository) FindAvailablePartnerCompanies(db *gorm.DB, tenantID int64) ([]models.PartnerCompany, error) {
	items := make([]models.PartnerCompany, 0)
	err := db.Model(&models.PartnerCompany{}).
		Select("DISTINCT partner_companies.*").
		Joins("JOIN partner_accounts ON partner_accounts.partner_company_id = partner_companies.id AND partner_accounts.tenant_id = partner_companies.tenant_id").
		Where("partner_companies.tenant_id = ? AND partner_companies.status = ? AND partner_accounts.status = ?", tenantID, enums.StatusOk, enums.StatusOk).
		Order("partner_companies.name ASC, partner_companies.id ASC").
		Find(&items).Error
	return items, err
}

func (r *ticketSupplierCollaborationRepository) GetPartnerAccountAnyStatus(db *gorm.DB, tenantID, companyID, accountID int64) *models.PartnerAccount {
	item := &models.PartnerAccount{}
	if err := db.First(item, "id = ? AND tenant_id = ? AND partner_company_id = ? AND status <> ?", accountID, tenantID, companyID, enums.StatusDeleted).Error; err != nil {
		return nil
	}
	return item
}

func (r *ticketSupplierCollaborationRepository) FindPartnerAccountsByCompany(db *gorm.DB, tenantID, companyID int64) []models.PartnerAccount {
	items := make([]models.PartnerAccount, 0)
	db.Where("tenant_id = ? AND partner_company_id = ? AND status <> ?", tenantID, companyID, enums.StatusDeleted).
		Order("status ASC, display_name ASC, id ASC").Find(&items)
	return items
}

func (r *ticketSupplierCollaborationRepository) CreatePartnerAccount(db *gorm.DB, item *models.PartnerAccount) error {
	return db.Create(item).Error
}

func (r *ticketSupplierCollaborationRepository) UpdatePartnerAccount(db *gorm.DB, tenantID, companyID, accountID int64, updates map[string]any) error {
	return db.Model(&models.PartnerAccount{}).
		Where("id = ? AND tenant_id = ? AND partner_company_id = ?", accountID, tenantID, companyID).
		Updates(updates).Error
}

func (r *ticketSupplierCollaborationRepository) FindActivePartnerAccountByUser(db *gorm.DB, userID int64) *models.PartnerAccount {
	item := &models.PartnerAccount{}
	if err := db.Where("user_id = ? AND status = ?", userID, enums.StatusOk).
		Order("id ASC").First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *ticketSupplierCollaborationRepository) FindPartnerAccountByUserAnyStatus(db *gorm.DB, userID int64) *models.PartnerAccount {
	item := &models.PartnerAccount{}
	if err := db.Where("user_id = ? AND status <> ?", userID, enums.StatusDeleted).
		Order("id ASC").First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *ticketSupplierCollaborationRepository) CreateAuthorizationScope(db *gorm.DB, item *models.PartnerAuthorizationScope) error {
	return db.Create(item).Error
}

func (r *ticketSupplierCollaborationRepository) FindAuthorizationScope(db *gorm.DB, tenantID, ticketID, collaborationID, partnerAccountID int64) *models.PartnerAuthorizationScope {
	item := &models.PartnerAuthorizationScope{}
	if err := db.Where("tenant_id = ? AND partner_account_id = ? AND resource_type = ? AND resource_id = ? AND scope_rule_json LIKE ?",
		tenantID, partnerAccountID, "ticket", strconv.FormatInt(ticketID, 10), "%\"collaboration_id\":"+strconv.FormatInt(collaborationID, 10)+"%").
		Order("id DESC").First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *ticketSupplierCollaborationRepository) UpdateAuthorizationScope(db *gorm.DB, id int64, updates map[string]any) error {
	return db.Model(&models.PartnerAuthorizationScope{}).Where("id = ?", id).Updates(updates).Error
}

func (r *ticketSupplierCollaborationRepository) DisableTicketAuthorizationScopes(db *gorm.DB, tenantID, ticketID, partnerAccountID int64, expiredAt time.Time) error {
	return db.Model(&models.PartnerAuthorizationScope{}).
		Where("tenant_id = ? AND partner_account_id = ? AND resource_type = ? AND resource_id = ? AND status = ?",
			tenantID, partnerAccountID, "ticket", strconv.FormatInt(ticketID, 10), enums.StatusOk).
		Updates(map[string]any{"status": enums.StatusDisabled, "expired_at": expiredAt, "updated_at": expiredAt}).Error
}

func (r *ticketSupplierCollaborationRepository) DisableCollaborationAuthorizationScopes(db *gorm.DB, tenantID, ticketID, collaborationID int64, partnerAccountIDs []int64, expiredAt time.Time) error {
	if len(partnerAccountIDs) == 0 {
		return nil
	}
	return db.Model(&models.PartnerAuthorizationScope{}).
		Where("tenant_id = ? AND partner_account_id IN ? AND resource_type = ? AND resource_id = ? AND scope_rule_json LIKE ? AND status = ?",
			tenantID, partnerAccountIDs, "ticket", strconv.FormatInt(ticketID, 10), "%\"collaboration_id\":"+strconv.FormatInt(collaborationID, 10)+"%", enums.StatusOk).
		Updates(map[string]any{"status": enums.StatusDisabled, "expired_at": expiredAt, "updated_at": expiredAt}).Error
}

func (r *ticketSupplierCollaborationRepository) ResolveActiveByTicket(db *gorm.DB, tenantID, ticketID int64, resolution string, resolvedAt time.Time, userID int64, username string) error {
	return db.Model(&models.TicketSupplierCollaboration{}).
		Where("tenant_id = ? AND ticket_id = ? AND status IN ? AND record_status <> ?", tenantID, ticketID, []string{"invited", "accepted", "processing"}, enums.StatusDeleted).
		Updates(map[string]any{
			"status": "resolved", "resolution": resolution, "resolved_at": resolvedAt,
			"updated_at": resolvedAt, "update_user_id": userID, "update_user_name": username,
		}).Error
}

func (r *ticketSupplierCollaborationRepository) DisableAllTicketAuthorizationScopes(db *gorm.DB, tenantID, ticketID int64, expiredAt time.Time) error {
	return db.Model(&models.PartnerAuthorizationScope{}).
		Where("tenant_id = ? AND resource_type = ? AND resource_id = ? AND status = ?", tenantID, "ticket", strconv.FormatInt(ticketID, 10), enums.StatusOk).
		Updates(map[string]any{"status": enums.StatusDisabled, "expired_at": expiredAt, "updated_at": expiredAt}).Error
}
