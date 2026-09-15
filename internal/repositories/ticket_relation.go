package repositories

import (
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"remotehelpdesk/internal/models"
)

// All graph writes and lifecycle transitions serialize within a tenant. Try-lock
// avoids deadlocks with existing ticket->user/IAM writers; the whole request retries.
func LockTicketGraphDB(db *gorm.DB, tenantID int64) error {
	if db.Dialector.Name() != "postgres" {
		return nil
	}
	var acquired bool
	if err := db.Raw("SELECT pg_try_advisory_xact_lock(hashtextextended(?, 0))", fmt.Sprintf("ticket-graph:%d", tenantID)).Scan(&acquired).Error; err != nil {
		return err
	}
	if !acquired {
		return fmt.Errorf("工单关系或状态正在更新，请刷新后重试")
	}
	return nil
}
func GuardTicketRelationsDB(db *gorm.DB, id int64, columns map[string]interface{}) error {
	_, c := columns["case_status"]
	_, s := columns["status"]
	if !db.Migrator().HasTable(&models.TicketRelation{}) {
		return nil
	}
	var ticket models.Ticket
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&ticket, id).Error; err != nil {
		return err
	}
	if ticket.MergedIntoID > 0 {
		return fmt.Errorf("该工单已合并，请在主工单继续处理")
	}
	if !c && !s {
		return nil
	}
	if err := LockTicketGraphDB(db, ticket.TenantID); err != nil {
		return err
	}
	state := fmt.Sprint(columns["case_status"])
	status := fmt.Sprint(columns["status"])
	if state != "closed" && state != "cancelled" && status != "closed" && status != "cancelled" && status != "done" {
		return nil
	}
	ids := db.Model(&models.TicketRelation{}).Select("target_id").Where("tenant_id = ? AND source_id = ? AND kind = 'parent' AND active_key IS NOT NULL", ticket.TenantID, id)
	var children []models.Ticket
	if err := db.Where("tenant_id = ? AND id IN (?)", ticket.TenantID, ids).Find(&children).Error; err != nil {
		return err
	}
	for _, child := range children {
		st := models.EffectiveTicketCaseStatus(child)
		if st != "closed" && st != "cancelled" {
			return fmt.Errorf("存在未结束的子工单，请先完成跟进或有理由地调整关联后再结束父工单")
		}
	}
	return nil
}
