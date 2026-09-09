package repositories

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"time"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var AgentTeamScheduleRepository = newAgentTeamScheduleRepository()

func newAgentTeamScheduleRepository() *agentTeamScheduleRepository {
	return &agentTeamScheduleRepository{}
}

type agentTeamScheduleRepository struct {
}

func (r *agentTeamScheduleRepository) Get(db *gorm.DB, id int64) *models.AgentTeamSchedule {
	ret := &models.AgentTeamSchedule{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *agentTeamScheduleRepository) Take(db *gorm.DB, where ...interface{}) *models.AgentTeamSchedule {
	ret := &models.AgentTeamSchedule{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *agentTeamScheduleRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.AgentTeamSchedule) {
	cnd.Find(db, &list)
	return
}

func (r *agentTeamScheduleRepository) FindByTimeRange(db *gorm.DB, startAt, endAt time.Time, teamID, tenantID int64) (list []models.AgentTeamSchedule) {
	query := db.Model(&models.AgentTeamSchedule{}).
		Where("start_at < ? AND end_at > ?", endAt, startAt)
	if teamID > 0 {
		query = query.Where("team_id = ?", teamID)
	}
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	query.Order("team_id ASC").Order("start_at ASC").Order("id ASC").Find(&list)
	return
}

func (r *agentTeamScheduleRepository) FindOverlappingByTeamIDsAndTimeRange(db *gorm.DB, teamIDs []int64, startAt, endAt time.Time) (list []models.AgentTeamSchedule) {
	if len(teamIDs) == 0 {
		return
	}
	db.Model(&models.AgentTeamSchedule{}).
		Where("team_id IN ? AND status = ? AND start_at < ? AND end_at > ?", teamIDs, enums.StatusOk, endAt, startAt).
		Order("team_id ASC").
		Order("start_at ASC").
		Order("id ASC").
		Find(&list)
	return
}

func (r *agentTeamScheduleRepository) FindWeeklyByTeamAndWeekday(db *gorm.DB, tenantID, teamID int64, weekday int) (list []models.AgentTeamSchedule) {
	query := db.Model(&models.AgentTeamSchedule{}).
		Where("team_id = ? AND repeat_type = ? AND weekday = ? AND status = ?", teamID, "weekly", weekday, enums.StatusOk)
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	query.Order("start_minute ASC").Order("user_id ASC").Order("id ASC").Find(&list)
	return
}

func (r *agentTeamScheduleRepository) FindVersion(db *gorm.DB, tenantID, teamID int64, publishStatus string, version int) (list []models.AgentTeamSchedule) {
	query := db.Model(&models.AgentTeamSchedule{}).
		Where("tenant_id = ? AND team_id = ? AND publish_status = ? AND status = ?", tenantID, teamID, publishStatus, enums.StatusOk)
	if version > 0 {
		query = query.Where("version = ?", version)
	}
	query.Order("weekday ASC").Order("start_minute ASC").Order("id ASC").Find(&list)
	return
}

func (r *agentTeamScheduleRepository) FindWeeklyForUser(db *gorm.DB, tenantID, userID int64) (list []models.AgentTeamSchedule) {
	if db == nil || tenantID <= 0 || userID <= 0 {
		return
	}
	db.Model(&models.AgentTeamSchedule{}).
		Where("tenant_id = ? AND user_id = ? AND repeat_type = ? AND status = ?", tenantID, userID, "weekly", enums.StatusOk).
		Where("publish_status IN ?", []string{"draft", "published"}).
		Order("weekday ASC").Order("start_minute ASC").Order("id ASC").Find(&list)
	return
}

func (r *agentTeamScheduleRepository) FindActiveHolidaysByTeamIDs(db *gorm.DB, tenantID int64, teamIDs []int64) ([]models.AgentTeamHoliday, error) {
	items := make([]models.AgentTeamHoliday, 0)
	if db == nil || tenantID <= 0 || len(teamIDs) == 0 || !db.Migrator().HasTable(&models.AgentTeamHoliday{}) {
		return items, nil
	}
	err := db.Model(&models.AgentTeamHoliday{}).
		Where("tenant_id = ? AND team_id IN ? AND status = ?", tenantID, uniqueRepositoryInt64s(teamIDs), enums.StatusOk).
		Order("team_id ASC").Order("holiday_date ASC").Order("id ASC").
		Find(&items).Error
	return items, err
}

func (r *agentTeamScheduleRepository) DeleteDraft(db *gorm.DB, tenantID, teamID int64) error {
	return db.Where("tenant_id = ? AND team_id = ? AND publish_status = ?", tenantID, teamID, "draft").
		Delete(&models.AgentTeamSchedule{}).Error
}

func (r *agentTeamScheduleRepository) UpdatePublishStatus(db *gorm.DB, tenantID, teamID int64, fromStatus, toStatus string, version int, updates map[string]any) error {
	query := db.Model(&models.AgentTeamSchedule{}).
		Where("tenant_id = ? AND team_id = ? AND publish_status = ?", tenantID, teamID, fromStatus)
	if version > 0 {
		query = query.Where("version = ?", version)
	}
	columns := make(map[string]any, len(updates)+1)
	for key, value := range updates {
		columns[key] = value
	}
	columns["publish_status"] = toStatus
	return query.Updates(columns).Error
}

func (r *agentTeamScheduleRepository) FindLatestArchivedVersion(db *gorm.DB, tenantID, teamID int64, beforeVersion int) int {
	var row struct{ Version int }
	query := db.Model(&models.AgentTeamSchedule{}).
		Select("version").
		Where("tenant_id = ? AND team_id = ? AND publish_status = ?", tenantID, teamID, "archived")
	if beforeVersion > 0 {
		query = query.Where("version < ?", beforeVersion)
	}
	query.Order("version DESC").Limit(1).Scan(&row)
	return row.Version
}

func (r *agentTeamScheduleRepository) CreateBatch(db *gorm.DB, list []models.AgentTeamSchedule) error {
	if len(list) == 0 {
		return nil
	}
	return db.Create(&list).Error
}

func (r *agentTeamScheduleRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.AgentTeamSchedule {
	ret := &models.AgentTeamSchedule{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *agentTeamScheduleRepository) FindPageByParams(db *gorm.DB, params *params.QueryParams) (list []models.AgentTeamSchedule, paging *sqls.Paging) {
	return r.FindPageByCnd(db, &params.Cnd)
}

func (r *agentTeamScheduleRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.AgentTeamSchedule, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.AgentTeamSchedule{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *agentTeamScheduleRepository) FindBySql(db *gorm.DB, sqlStr string, paramArr ...interface{}) (list []models.AgentTeamSchedule) {
	db.Raw(sqlStr, paramArr...).Scan(&list)
	return
}

func (r *agentTeamScheduleRepository) CountBySql(db *gorm.DB, sqlStr string, paramArr ...interface{}) (count int64) {
	db.Raw(sqlStr, paramArr...).Count(&count)
	return
}

func (r *agentTeamScheduleRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.AgentTeamSchedule{})
}

func (r *agentTeamScheduleRepository) Create(db *gorm.DB, t *models.AgentTeamSchedule) (err error) {
	err = db.Create(t).Error
	return
}

func (r *agentTeamScheduleRepository) Update(db *gorm.DB, t *models.AgentTeamSchedule) (err error) {
	err = db.Save(t).Error
	return
}

func (r *agentTeamScheduleRepository) Updates(db *gorm.DB, id int64, columns map[string]interface{}) (err error) {
	err = db.Model(&models.AgentTeamSchedule{}).Where("id = ?", id).Updates(columns).Error
	return
}

func (r *agentTeamScheduleRepository) UpdateColumn(db *gorm.DB, id int64, name string, value interface{}) (err error) {
	err = db.Model(&models.AgentTeamSchedule{}).Where("id = ?", id).UpdateColumn(name, value).Error
	return
}

func (r *agentTeamScheduleRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.AgentTeamSchedule{}, "id = ?", id)
}
