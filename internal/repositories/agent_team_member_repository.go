package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var AgentTeamMemberRepository = newAgentTeamMemberRepository()

func newAgentTeamMemberRepository() *agentTeamMemberRepository {
	return &agentTeamMemberRepository{}
}

type agentTeamMemberRepository struct{}

func (r *agentTeamMemberRepository) Get(db *gorm.DB, id int64) *models.AgentTeamMember {
	ret := &models.AgentTeamMember{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *agentTeamMemberRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.AgentTeamMember) {
	cnd.Find(db, &list)
	return
}

func (r *agentTeamMemberRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.AgentTeamMember {
	ret := &models.AgentTeamMember{}
	if err := cnd.FindOne(db, ret); err != nil {
		return nil
	}
	return ret
}

func (r *agentTeamMemberRepository) Create(db *gorm.DB, item *models.AgentTeamMember) error {
	dispatchEnabled := item.DispatchEnabled
	if err := db.Select("*").Create(item).Error; err != nil {
		return err
	}
	if err := db.Model(&models.AgentTeamMember{}).Where("id = ?", item.ID).UpdateColumn("dispatch_enabled", dispatchEnabled).Error; err != nil {
		return err
	}
	item.DispatchEnabled = dispatchEnabled
	return nil
}

func (r *agentTeamMemberRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.AgentTeamMember{}).Where("id = ?", id).Updates(columns).Error
}
