package services

import (
	"time"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
)

type TicketMergeContent struct {
	TicketID    int64                       `json:"ticket_id"`
	TicketNo    string                      `json:"ticket_no"`
	Title       string                      `json:"title"`
	Description string                      `json:"description"`
	Author      string                      `json:"author"`
	CreatedAt   time.Time                   `json:"created_at"`
	Timeline    []dto.TicketTimelineItemDTO `json:"timeline"`
	Assets      []dto.AssetRefDTO           `json:"assets"`
}

type TicketMergeView struct {
	CanMerge bool                 `json:"can_merge"`
	Into     *TicketRelationView  `json:"into,omitempty"`
	Sources  []TicketMergeContent `json:"sources"`
}

func readTicketMergeDB(db *gorm.DB, t *models.Ticket, op *dto.AuthPrincipal) (TicketMergeView, error) {
	v := TicketMergeView{CanMerge: false, Sources: []TicketMergeContent{}}
	if t.MergedIntoID > 0 {
		var main models.Ticket
		if err := db.Where("tenant_id = ? AND id = ?", t.TenantID, t.MergedIntoID).First(&main).Error; err != nil {
			return v, err
		}
		if ticketGovernanceAccess(&main, op) == nil {
			v.Into = &TicketRelationView{OtherID: main.ID, TicketNo: main.TicketNo, Title: main.Title, Status: models.EffectiveTicketCaseStatus(main), Priority: ticketEffectivePriority(main)}
		}
	}
	var sources []models.Ticket
	if err := db.Where("tenant_id = ? AND merged_into_id = ?", t.TenantID, t.ID).Order("merged_at ASC, id ASC").Find(&sources).Error; err != nil {
		return v, err
	}
	if len(sources) == 0 {
		return v, nil
	}
	v.CanMerge = false
	seen := map[int64]bool{}
	for _, a := range EnterpriseTicketService.buildAssets(t) {
		seen[a.ID] = true
	}
	for _, source := range sources {
		if ticketGovernanceAccess(&source, op) != nil {
			continue
		}
		content := TicketMergeContent{TicketID: source.ID, TicketNo: source.TicketNo, Title: source.Title, Description: source.Description, Author: source.CreateUserName, CreatedAt: source.CreatedAt, Timeline: EnterpriseTicketService.buildTimeline(&source), Assets: []dto.AssetRefDTO{}}
		for _, asset := range EnterpriseTicketService.buildAssets(&source) {
			if !seen[asset.ID] {
				content.Assets = append(content.Assets, asset)
				seen[asset.ID] = true
			}
		}
		v.Sources = append(v.Sources, content)
	}
	return v, nil
}

// ResolveWorkflowTicket resolves durable effect receipts without re-executing a
// create node. Never follow a reference across tenants or customers.
func ResolveWorkflowTicket(tenantID, ticketID int64) (*models.Ticket, error) {
	if tenantID <= 0 || ticketID <= 0 {
		return nil, errorsx.InvalidParam("invalid workflow ticket scope")
	}
	db := sqls.DB()
	var ticket models.Ticket
	if err := db.Where("tenant_id = ? AND id = ?", tenantID, ticketID).First(&ticket).Error; err != nil {
		return nil, err
	}
	if ticket.MergedIntoID > 0 {
		var main models.Ticket
		if err := db.Where("tenant_id = ? AND customer_id = ? AND id = ? AND merged_into_id = 0", tenantID, ticket.CustomerID, ticket.MergedIntoID).First(&main).Error; err != nil {
			return nil, err
		}
		return &main, nil
	}
	return &ticket, nil
}
