package repositories

import (
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var MessageRepository = newMessageRepository()

func newMessageRepository() *messageRepository {
	return &messageRepository{}
}

type messageRepository struct {
}

func (r *messageRepository) Get(db *gorm.DB, id int64) *models.Message {
	ret := &models.Message{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *messageRepository) Take(db *gorm.DB, where ...interface{}) *models.Message {
	ret := &models.Message{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *messageRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.Message) {
	cnd.Find(db, &list)
	return
}

func (r *messageRepository) FindLastUnrecalledByConversationID(db *gorm.DB, conversationID int64) *models.Message {
	ret := &models.Message{}
	if err := db.
		Where("conversation_id = ? AND recalled_at IS NULL AND send_status <> ?", conversationID, 6).
		Order("id DESC").
		Limit(1).
		Take(ret).Error; err != nil {
		return nil
	}
	return ret
}

func (r *messageRepository) FindLastValidByConversationIDAndSenderType(db *gorm.DB, conversationID int64, senderType enums.IMSenderType) *models.Message {
	ret := &models.Message{}
	if err := db.
		Where("conversation_id = ? AND sender_type = ? AND recalled_at IS NULL AND send_status NOT IN ?", conversationID, senderType, []enums.IMMessageStatus{
			enums.IMMessageStatusFailed,
			enums.IMMessageStatusRecalled,
		}).
		Order("id DESC").
		Limit(1).
		Take(ret).Error; err != nil {
		return nil
	}
	return ret
}

func (r *messageRepository) FindFirstValidByConversationIDAndSenderTypeBetweenMessageIDs(db *gorm.DB, conversationID int64, senderType enums.IMSenderType, afterMessageID, beforeMessageID int64) *models.Message {
	ret := &models.Message{}
	query := db.
		Where("conversation_id = ? AND sender_type = ? AND id > ? AND recalled_at IS NULL", conversationID, senderType, afterMessageID).
		Where("send_status NOT IN ?", []enums.IMMessageStatus{enums.IMMessageStatusFailed, enums.IMMessageStatusRecalled})
	if beforeMessageID > afterMessageID {
		query = query.Where("id < ?", beforeMessageID)
	}
	if err := query.
		Order("id ASC").
		Limit(1).
		Take(ret).Error; err != nil {
		return nil
	}
	return ret
}

func (r *messageRepository) FindSupplierConversationMessages(db *gorm.DB, conversationID int64, from time.Time, until *time.Time) (list []models.Message) {
	query := db.
		Where("conversation_id = ? AND created_at >= ?", conversationID, from).
		Where("sender_type IN ?", []enums.IMSenderType{enums.IMSenderTypeCustomer, enums.IMSenderTypeAgent, enums.IMSenderTypeAI, enums.IMSenderTypePartner, enums.IMSenderTypeSystem}).
		Where("send_status NOT IN ? AND recalled_at IS NULL", []enums.IMMessageStatus{enums.IMMessageStatusFailed, enums.IMMessageStatusRecalled})
	if until != nil {
		query = query.Where("created_at <= ?", *until)
	}
	query.Order("id ASC").Find(&list)
	return
}

func (r *messageRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.Message {
	ret := &models.Message{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *messageRepository) FindPageByParams(db *gorm.DB, params *params.QueryParams) (list []models.Message, paging *sqls.Paging) {
	return r.FindPageByCnd(db, &params.Cnd)
}

func (r *messageRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.Message, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.Message{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *messageRepository) FindBySql(db *gorm.DB, sqlStr string, paramArr ...interface{}) (list []models.Message) {
	db.Raw(sqlStr, paramArr...).Scan(&list)
	return
}

// FindAssetReferenceCandidates narrows legacy messages that may reference an
// asset. Callers must parse the returned payload/content before authorizing it;
// LIKE is intentionally not an authorization boundary.
func (r *messageRepository) FindAssetReferenceCandidates(db *gorm.DB, assetID, storageKey string) []models.Message {
	assetID = strings.TrimSpace(assetID)
	storageKey = strings.TrimSpace(storageKey)
	if db == nil || assetID == "" && storageKey == "" {
		return nil
	}
	messages := make([]models.Message, 0)
	query := db.Model(&models.Message{}).
		Select("id", "conversation_id", "message_type", "content", "payload")
	if assetID != "" && storageKey != "" {
		assetPattern := "%" + assetID + "%"
		storagePattern := "%" + storageKey + "%"
		query = query.Where("payload LIKE ? OR content LIKE ? OR payload LIKE ? OR content LIKE ?", assetPattern, assetPattern, storagePattern, storagePattern)
	} else {
		pattern := "%" + assetID + storageKey + "%"
		query = query.Where("payload LIKE ? OR content LIKE ?", pattern, pattern)
	}
	query.Find(&messages)
	return messages
}

func (r *messageRepository) CountBySql(db *gorm.DB, sqlStr string, paramArr ...interface{}) (count int64) {
	db.Raw(sqlStr, paramArr...).Count(&count)
	return
}

func (r *messageRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.Message{})
}

func (r *messageRepository) Create(db *gorm.DB, t *models.Message) (err error) {
	err = db.Create(t).Error
	return
}

func (r *messageRepository) CreateIfClientMsgIDAbsent(db *gorm.DB, t *models.Message) (bool, error) {
	result := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "conversation_id"},
			{Name: "client_msg_id"},
		},
		DoNothing: true,
	}).Create(t)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *messageRepository) Update(db *gorm.DB, t *models.Message) (err error) {
	err = db.Save(t).Error
	return
}

func (r *messageRepository) Updates(db *gorm.DB, id int64, columns map[string]interface{}) (err error) {
	err = db.Model(&models.Message{}).Where("id = ?", id).Updates(columns).Error
	return
}

func (r *messageRepository) UpdateColumn(db *gorm.DB, id int64, name string, value interface{}) (err error) {
	err = db.Model(&models.Message{}).Where("id = ?", id).UpdateColumn(name, value).Error
	return
}

func (r *messageRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.Message{}, "id = ?", id)
}

// GetByClientMsgID 根据 conversationID 和 clientMsgID 获取消息
func (r *messageRepository) GetByClientMsgID(db *gorm.DB, conversationID int64, clientMsgID string) *models.Message {
	return r.FindOne(db, sqls.NewCnd().Where("conversation_id = ? AND client_msg_id = ?", conversationID, clientMsgID))
}
