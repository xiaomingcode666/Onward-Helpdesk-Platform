package repositories

import (
	"remotehelpdesk/internal/models"

	"gorm.io/gorm"
)

var FaultTreeRepository = &faultTreeRepository{}

type faultTreeRepository struct{}

func (r *faultTreeRepository) ListByTenantProduct(db *gorm.DB, tenantID int64, productID string) ([]models.FaultTreeNode, error) {
	var rows []models.FaultTreeNode
	err := db.Where("tenant_id = ? AND product_id = ?", tenantID, productID).
		Order("order_index ASC, created_at ASC, id ASC").
		Find(&rows).Error
	return rows, err
}

func (r *faultTreeRepository) ListPublishedByProductType(db *gorm.DB, tenantID int64, productID, nodeType string) ([]models.FaultTreeNode, error) {
	var rows []models.FaultTreeNode
	query := db.Where("product_id = ? AND status = ? AND node_type = ?", productID, "published", nodeType)
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	err := query.
		Order("order_index ASC, id ASC").
		Find(&rows).Error
	return rows, err
}

func (r *faultTreeRepository) GetByTenant(db *gorm.DB, tenantID int64, id string) *models.FaultTreeNode {
	if tenantID <= 0 || id == "" {
		return nil
	}
	var row models.FaultTreeNode
	if err := db.First(&row, "tenant_id = ? AND id = ?", tenantID, id).Error; err != nil {
		return nil
	}
	return &row
}

func (r *faultTreeRepository) GetPublished(db *gorm.DB, id string) *models.FaultTreeNode {
	if id == "" {
		return nil
	}
	var row models.FaultTreeNode
	if err := db.First(&row, "id = ? AND status = ?", id, "published").Error; err != nil {
		return nil
	}
	return &row
}

func (r *faultTreeRepository) FindNextPublished(db *gorm.DB, current *models.FaultTreeNode) *models.FaultTreeNode {
	if current == nil {
		return nil
	}
	var child models.FaultTreeNode
	if err := db.Where("parent_id = ? AND status = ?", current.ID, "published").
		Order("order_index ASC, id ASC").First(&child).Error; err == nil {
		return &child
	}

	var sibling models.FaultTreeNode
	if err := db.Where("parent_id = ? AND status = ? AND order_index > ?", current.ParentID, "published", current.OrderIndex).
		Order("order_index ASC, id ASC").First(&sibling).Error; err != nil {
		return nil
	}
	return &sibling
}

func (r *faultTreeRepository) ListPublishedByIDs(db *gorm.DB, ids []string) ([]models.FaultTreeNode, error) {
	if len(ids) == 0 {
		return []models.FaultTreeNode{}, nil
	}
	var rows []models.FaultTreeNode
	err := db.Where("id IN ? AND status = ?", ids, "published").Find(&rows).Error
	return rows, err
}

func (r *faultTreeRepository) Create(db *gorm.DB, row *models.FaultTreeNode) error {
	return db.Create(row).Error
}

func (r *faultTreeRepository) UpdatesByTenant(db *gorm.DB, tenantID int64, id string, columns map[string]any) error {
	return db.Model(&models.FaultTreeNode{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(columns).Error
}
