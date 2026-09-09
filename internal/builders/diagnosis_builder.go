package builders

import (
	"strconv"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
)

func BuildEnterpriseFaultTreeNode(row *models.FaultTreeNode) dto.EnterpriseFaultTreeNodeDTO {
	productID, _ := strconv.ParseInt(row.ProductID, 10, 64)
	return dto.EnterpriseFaultTreeNodeDTO{
		ID:                row.ID,
		TenantID:          row.TenantID,
		ProductID:         productID,
		ParentID:          row.ParentID,
		Title:             row.Title,
		Description:       row.Description,
		NodeType:          row.NodeType,
		FaultPattern:      row.FaultPattern,
		TriggerConditions: row.TriggerConditions,
		IsComposite:       row.IsComposite,
		RiskLevel:         row.RiskLevel,
		OrderIndex:        row.OrderIndex,
		Status:            row.Status,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}

func BuildEnterpriseFaultTreeNodes(rows []models.FaultTreeNode) []dto.EnterpriseFaultTreeNodeDTO {
	result := make([]dto.EnterpriseFaultTreeNodeDTO, 0, len(rows))
	for i := range rows {
		result = append(result, BuildEnterpriseFaultTreeNode(&rows[i]))
	}
	return result
}
