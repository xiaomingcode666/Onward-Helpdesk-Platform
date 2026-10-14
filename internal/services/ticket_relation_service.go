package services

import (
	"fmt"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"time"
)

type TicketRelationView struct {
	models.TicketRelation
	OtherID     int64  `json:"other_id"`
	TicketNo    string `json:"ticket_no"`
	Title       string `json:"title"`
	CaseType    string `json:"case_type"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	OwnerID     int64  `json:"owner_id"`
	SLABreached bool   `json:"sla_breached"`
	CanRemove   bool   `json:"can_remove"`
}

func readTicketRelationsDB(db *gorm.DB, t *models.Ticket, op *dto.AuthPrincipal) ([]TicketRelationView, error) {
	result := []TicketRelationView{}
	var rows []models.TicketRelation
	if err := db.Where("tenant_id = ? AND (source_id = ? OR target_id = ?) AND active_key IS NOT NULL", t.TenantID, t.ID, t.ID).Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		id := row.TargetID
		if id == t.ID {
			id = row.SourceID
		}
		var other models.Ticket
		if err := db.Where("id = ? AND tenant_id = ?", id, t.TenantID).First(&other).Error; err != nil {
			return nil, err
		}
		if ticketGovernanceAccess(&other, op) != nil {
			continue
		}
		result = append(result, TicketRelationView{TicketRelation: row, OtherID: other.ID, TicketNo: other.TicketNo, Title: other.Title, CaseType: other.CaseType, Status: models.EffectiveTicketCaseStatus(other), Priority: ticketEffectivePriority(other), OwnerID: other.CaseOwnerID, SLABreached: other.SLADueAt != nil && other.SLADueAt.Before(time.Now()) && !ticketResolutionSLAStopped(other), CanRemove: row.Kind != "merged" && t.MergedIntoID == 0 && other.MergedIntoID == 0 && canManageTicketGovernance(t, op) && canManageTicketGovernance(&other, op)})
	}
	return result, nil
}
func mutateTicketRelationDB(db *gorm.DB, t *models.Ticket, cmd TicketGovernanceCommand, op *dto.AuthPrincipal) error {
	source, target, kind := t.ID, cmd.TargetID, cmd.RelationKind
	var row models.TicketRelation
	if cmd.Action == "duplicate_child" {
		source, target, kind = cmd.TargetID, t.ID, "parent"
	}
	if cmd.Action == "unlink" {
		if err := db.Where("id = ? AND tenant_id = ? AND (source_id = ? OR target_id = ?) AND active_key IS NOT NULL", cmd.RelationID, t.TenantID, t.ID, t.ID).First(&row).Error; err != nil {
			return err
		}
		source, target, kind = row.SourceID, row.TargetID, row.Kind
	} else {
		if kind == "child_of" {
			source, target, kind = target, source, "parent"
		}
		if kind != "parent" || source <= 0 || target <= 0 || source == target {
			return errorsx.InvalidParamI18n("error.z0163")
		}
	}
	otherID := target
	if otherID == t.ID {
		otherID = source
	}
	var other models.Ticket
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", otherID, t.TenantID).First(&other).Error; err != nil {
		return errorsx.InvalidParamI18n("error.z0164")
	}
	if !canManageTicketGovernance(&other, op) {
		return errorsx.ForbiddenI18n("error.z0165")
	}
	if kind == "merged" || t.MergedIntoID > 0 || other.MergedIntoID > 0 {
		return errorsx.InvalidParamI18n("error.z0166")
	}
	if cmd.Action == "duplicate_child" {
		if cmd.TargetRevision == nil {
			return errorsx.InvalidParamI18n("error.z0167")
		}
		if other.GovernanceRevision != *cmd.TargetRevision {
			return ErrTicketCaseConflict
		}
		if t.Status == "draft" || other.Status == "draft" || !ticketCreatedBefore(other, *t) {
			return errorsx.InvalidParamI18n("error.z0168")
		}
	}
	now := time.Now()
	if cmd.Action == "link" || cmd.Action == "duplicate_child" {
		if kind == "parent" && ticketCaseClosed(other) {
			return errorsx.InvalidParamI18n("error.z0169")
		}
		var graph []models.TicketRelation
		if err := db.Where("tenant_id = ? AND kind = ? AND active_key IS NOT NULL", t.TenantID, kind).Find(&graph).Error; err != nil {
			return err
		}
		for _, edge := range graph {
			if cmd.Action == "duplicate_child" && (edge.TargetID == source || edge.SourceID == target) {
				return errorsx.InvalidParamI18n("error.z0170")
			}

			if edge.SourceID == source && edge.TargetID == target {
				return errorsx.InvalidParamI18n("error.z0171")
			}
			if kind == "parent" && edge.TargetID == target {
				return errorsx.InvalidParamI18n("error.z0172")
			}
			if kind == "duplicate" && (edge.SourceID == source || edge.SourceID == target) {
				return errorsx.InvalidParamI18n("error.z0173")
			}
		}
		if kind != "related" && ticketGraphReaches(graph, target, source) {
			return errorsx.InvalidParamI18n("error.z0174")
		}
		key := fmt.Sprintf("%s:%d:%d", kind, source, target)
		row = models.TicketRelation{TenantID: t.TenantID, SourceID: source, TargetID: target, Kind: kind, ActiveKey: &key, Reason: cmd.Reason, CreatedBy: op.UserID, CreatedAt: now}
		if kind == "parent" {
			child := target
			row.ChildKey = &child
		}
		if err := db.Create(&row).Error; err != nil {
			return err
		}
	} else {
		if err := db.Model(&row).Updates(map[string]any{"active_key": nil, "child_key": nil, "removed_at": now, "removed_by": op.UserID, "removal_reason": cmd.Reason}).Error; err != nil {
			return err
		}
	}
	for _, item := range []*models.Ticket{t, &other} {
		item.GovernanceRevision++
		if err := db.Model(&models.Ticket{}).Where("id = ? AND tenant_id = ?", item.ID, item.TenantID).Updates(map[string]any{"governance_revision": item.GovernanceRevision, "updated_at": now}).Error; err != nil {
			return err
		}
		related := t.ID
		if item.ID == t.ID {
			related = other.ID
		}
		if err := recordGovernanceDB(db, item, op, cmd.Action, cmd.Reason, map[string]any{"relation_id": row.ID, "kind": kind, "related_ticket_id": related, "source_id": source, "target_id": target, "sla_effect": "unchanged", "clock_reset": false, "sla_snapshot": ticketRelationSLASnapshot(*item, now)}); err != nil {
			return err
		}
	}
	return nil
}
func ticketGraphReaches(rows []models.TicketRelation, from, to int64) bool {
	queue := []int64{from}
	seen := map[int64]bool{}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		if node == to {
			return true
		}
		if seen[node] {
			continue
		}
		seen[node] = true
		for _, row := range rows {
			if row.SourceID == node {
				queue = append(queue, row.TargetID)
			}
		}
	}
	return false
}

// Lookup returns only tickets the actor may manage, even when searching by an exact number.
func SearchTicketRelationCandidates(ticketID int64, query string, op *dto.AuthPrincipal) ([]TicketRelationView, error) {
	if op == nil {
		return nil, errorsx.ForbiddenI18n("error.z0135")
	}
	db := sqls.DB()
	var current models.Ticket
	if err := db.Where("id = ? AND tenant_id = ?", ticketID, op.EffectiveTenantID()).First(&current).Error; err != nil {
		return nil, err
	}
	if !canManageTicketGovernance(&current, op) {
		return nil, errorsx.ForbiddenI18n("error.z0136")
	}
	var rows []models.Ticket
	q := db.Where("tenant_id = ? AND id <> ?", current.TenantID, current.ID)
	if query != "" {
		q = q.Where("ticket_no LIKE ? OR title LIKE ?", "%"+query+"%", "%"+query+"%")
	}
	if err := q.Order("id DESC").Limit(100).Find(&rows).Error; err != nil {
		return nil, err
	}
	result := []TicketRelationView{}
	for _, t := range rows {
		if canManageTicketGovernance(&t, op) {
			result = append(result, TicketRelationView{OtherID: t.ID, TicketNo: t.TicketNo, Title: t.Title, CaseType: t.CaseType, Status: models.EffectiveTicketCaseStatus(t), Priority: ticketEffectivePriority(t), OwnerID: t.CaseOwnerID})
		}
	}
	return result, nil
}
