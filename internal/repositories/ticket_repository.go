package repositories

import (
	"strconv"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var TicketRepository = newTicketRepository()

func newTicketRepository() *ticketRepository {
	return &ticketRepository{}
}

func (r *ticketRepository) FindDueForAutoClose(db *gorm.DB, tenantID int64, cutoff time.Time, limit int) []models.Ticket {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	items := make([]models.Ticket, 0)
	db.Where("tenant_id = ? AND status IN ? AND resolved_at IS NOT NULL AND resolved_at <= ?",
		tenantID, []enums.TicketStatus{enums.TicketStatusResolved, enums.TicketStatusPendingCustomerConfirm}, cutoff).
		Order("resolved_at ASC").Limit(limit).Find(&items)
	return items
}

func (r *ticketRepository) FindUnclaimedLinkedConversationTickets(db *gorm.DB, statuses []enums.TicketStatus, createdBefore time.Time, limit int) ([]models.Ticket, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	conversationIDs := db.Model(&models.Conversation{}).
		Select("id").
		Where("status = ?", enums.IMConversationStatusAIServing).
		Where("handoff_at IS NULL").
		Where("current_assignee_id = 0")
	items := make([]models.Ticket, 0)
	err := db.Where("status IN ?", statuses).
		Where("current_assignee_id = 0").
		Where("conversation_id > 0").
		Where("created_at <= ?", createdBefore).
		Where("conversation_id IN (?)", conversationIDs).
		Order("created_at ASC").
		Order("id ASC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

func (r *ticketRepository) HasActiveMeeting(db *gorm.DB, ticketID int64) bool {
	var count int64
	db.Model(&models.MeetingRoomJitsi{}).
		Where("ticket_id = ? AND status = ?", strconv.FormatInt(ticketID, 10), "active").Count(&count)
	return count > 0
}

func (r *ticketRepository) HasUnfinishedMeeting(db *gorm.DB, ticketID int64) bool {
	var count int64
	db.Model(&models.MeetingRoomJitsi{}).
		Where("ticket_id = ? AND status IN ?", strconv.FormatInt(ticketID, 10), []string{"waiting", "scheduled", "active"}).Count(&count)
	return count > 0
}

type ticketRepository struct {
}

func (r *ticketRepository) Get(db *gorm.DB, id int64) *models.Ticket {
	ret := &models.Ticket{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *ticketRepository) Take(db *gorm.DB, where ...interface{}) *models.Ticket {
	ret := &models.Ticket{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *ticketRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.Ticket) {
	cnd.Find(db, &list)
	return
}

func (r *ticketRepository) FindDraftsByCustomerRegistrationGrantID(db *gorm.DB, tenantID, grantID int64) ([]models.Ticket, error) {
	items := make([]models.Ticket, 0)
	if db == nil || tenantID <= 0 || grantID <= 0 {
		return items, nil
	}
	err := db.Where("tenant_id = ? AND customer_registration_grant_id = ? AND status = ?", tenantID, grantID, enums.TicketStatusDraft).
		Order("id ASC").Find(&items).Error
	return items, err
}

func (r *ticketRepository) MoveInvitationDrafts(db *gorm.DB, tenantID int64, fromGrantIDs []int64, toGrantID int64) error {
	if db == nil || tenantID <= 0 || len(fromGrantIDs) == 0 || toGrantID <= 0 {
		return nil
	}
	return db.Model(&models.Ticket{}).
		Where("tenant_id = ? AND customer_registration_grant_id IN ? AND status = ?", tenantID, fromGrantIDs, enums.TicketStatusDraft).
		Update("customer_registration_grant_id", toGrantID).Error
}

func (r *ticketRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.Ticket {
	ret := &models.Ticket{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *ticketRepository) FindPageByParams(db *gorm.DB, params *params.QueryParams) (list []models.Ticket, paging *sqls.Paging) {
	return r.FindPageByCnd(db, &params.Cnd)
}

func (r *ticketRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.Ticket, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.Ticket{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *ticketRepository) FindBySql(db *gorm.DB, sqlStr string, paramArr ...interface{}) (list []models.Ticket) {
	db.Raw(sqlStr, paramArr...).Scan(&list)
	return
}

func (r *ticketRepository) CountBySql(db *gorm.DB, sqlStr string, paramArr ...interface{}) (count int64) {
	db.Raw(sqlStr, paramArr...).Count(&count)
	return
}

func (r *ticketRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.Ticket{})
}

func (r *ticketRepository) Create(db *gorm.DB, t *models.Ticket) (err error) {
	err = db.Create(t).Error
	return
}

func (r *ticketRepository) Update(db *gorm.DB, t *models.Ticket) (err error) {
	err = db.Save(t).Error
	return
}

func (r *ticketRepository) Updates(db *gorm.DB, id int64, columns map[string]interface{}) (err error) {
	err = db.Model(&models.Ticket{}).Where("id = ?", id).Updates(columns).Error
	return
}

func (r *ticketRepository) UpdateColumn(db *gorm.DB, id int64, name string, value interface{}) (err error) {
	err = db.Model(&models.Ticket{}).Where("id = ?", id).UpdateColumn(name, value).Error
	return
}

func (r *ticketRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.Ticket{}, "id = ?", id)
}
