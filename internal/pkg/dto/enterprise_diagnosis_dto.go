package dto

import "time"

type EnterpriseFaultTreeNodeCreateRequest struct {
	ProductID         int64  `json:"product_id" binding:"required"`
	ParentID          string `json:"parent_id"`
	Title             string `json:"title" binding:"required"`
	Description       string `json:"description"`
	NodeType          string `json:"node_type" binding:"required"`
	FaultPattern      string `json:"fault_pattern"`
	TriggerConditions string `json:"trigger_conditions"`
	IsComposite       bool   `json:"is_composite"`
	RiskLevel         string `json:"risk_level"`
	OrderIndex        int    `json:"order_index"`
}

type EnterpriseFaultTreeNodeUpdateRequest struct {
	ParentID          *string `json:"parent_id"`
	Title             *string `json:"title"`
	Description       *string `json:"description"`
	NodeType          *string `json:"node_type"`
	FaultPattern      *string `json:"fault_pattern"`
	TriggerConditions *string `json:"trigger_conditions"`
	IsComposite       *bool   `json:"is_composite"`
	RiskLevel         *string `json:"risk_level"`
	OrderIndex        *int    `json:"order_index"`
	Status            *string `json:"status"`
}

type EnterpriseFaultTreeNodeDTO struct {
	ID                string    `json:"id"`
	TenantID          int64     `json:"tenant_id"`
	ProductID         int64     `json:"product_id"`
	ParentID          string    `json:"parent_id"`
	Title             string    `json:"title"`
	Description       string    `json:"description"`
	NodeType          string    `json:"node_type"`
	FaultPattern      string    `json:"fault_pattern"`
	TriggerConditions string    `json:"trigger_conditions"`
	IsComposite       bool      `json:"is_composite"`
	RiskLevel         string    `json:"risk_level"`
	OrderIndex        int       `json:"order_index"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
