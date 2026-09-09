package repositories

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var KnowledgeDocumentRepository = newKnowledgeDocumentRepository()

func newKnowledgeDocumentRepository() *knowledgeDocumentRepository {
	return &knowledgeDocumentRepository{}
}

type knowledgeDocumentRepository struct {
}

func (r *knowledgeDocumentRepository) Get(db *gorm.DB, id int64) *models.KnowledgeDocument {
	ret := &models.KnowledgeDocument{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *knowledgeDocumentRepository) Take(db *gorm.DB, where ...interface{}) *models.KnowledgeDocument {
	ret := &models.KnowledgeDocument{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *knowledgeDocumentRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.KnowledgeDocument) {
	cnd.Find(db, &list)
	return
}

func (r *knowledgeDocumentRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.KnowledgeDocument {
	ret := &models.KnowledgeDocument{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *knowledgeDocumentRepository) FindPageByParams(db *gorm.DB, params *params.QueryParams) (list []models.KnowledgeDocument, paging *sqls.Paging) {
	return r.FindPageByCnd(db, &params.Cnd)
}

func (r *knowledgeDocumentRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.KnowledgeDocument, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.KnowledgeDocument{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *knowledgeDocumentRepository) FindPageListByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.KnowledgeDocument, paging *sqls.Paging) {
	cnd.Find(db.Omit("content"), &list)
	count := cnd.Count(db, &models.KnowledgeDocument{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *knowledgeDocumentRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.KnowledgeDocument{})
}

func (r *knowledgeDocumentRepository) CountActiveByDirectoryID(db *gorm.DB, directoryID int64) int64 {
	var count int64
	db.Model(&models.KnowledgeDocument{}).
		Where("directory_id = ? AND status <> ?", directoryID, enums.StatusDeleted).
		Count(&count)
	return count
}

func (r *knowledgeDocumentRepository) Create(db *gorm.DB, t *models.KnowledgeDocument) (err error) {
	err = db.Create(t).Error
	return
}

func (r *knowledgeDocumentRepository) Update(db *gorm.DB, t *models.KnowledgeDocument) (err error) {
	err = db.Save(t).Error
	return
}

func (r *knowledgeDocumentRepository) Updates(db *gorm.DB, id int64, columns map[string]interface{}) (err error) {
	err = db.Model(&models.KnowledgeDocument{}).Where("id = ?", id).Updates(columns).Error
	return
}

func (r *knowledgeDocumentRepository) UpdateColumn(db *gorm.DB, id int64, name string, value interface{}) (err error) {
	err = db.Model(&models.KnowledgeDocument{}).Where("id = ?", id).UpdateColumn(name, value).Error
	return
}

func (r *knowledgeDocumentRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.KnowledgeDocument{}, "id = ?", id)
}

func (r *knowledgeDocumentRepository) DeleteByKnowledgeBaseID(db *gorm.DB, knowledgeBaseID int64) error {
	return db.Delete(&models.KnowledgeDocument{}, "knowledge_base_id = ?", knowledgeBaseID).Error
}

func (r *knowledgeDocumentRepository) FindByIDs(db *gorm.DB, ids []int64) (list []models.KnowledgeDocument) {
	if len(ids) == 0 {
		return nil
	}
	db.Where("id IN ?", ids).Find(&list)
	return
}

func (r *knowledgeDocumentRepository) CountByKnowledgeBaseID(db *gorm.DB, knowledgeBaseID int64) int64 {
	var count int64
	db.Model(&models.KnowledgeDocument{}).Where("knowledge_base_id = ? AND status <> ?", knowledgeBaseID, enums.StatusDeleted).Count(&count)
	return count
}

func (r *knowledgeDocumentRepository) CountActiveByTenantID(db *gorm.DB, tenantID int64) int64 {
	var count int64
	db.Model(&models.KnowledgeDocument{}).
		Where("tenant_id = ? AND status <> ?", tenantID, enums.StatusDeleted).
		Count(&count)
	return count
}

func (r *knowledgeDocumentRepository) CountActiveByTenantUserID(db *gorm.DB, tenantID, userID int64) int64 {
	var count int64
	db.Model(&models.KnowledgeDocument{}).
		Where("tenant_id = ? AND create_user_id = ? AND status <> ?", tenantID, userID, enums.StatusDeleted).
		Count(&count)
	return count
}

func (r *knowledgeDocumentRepository) CountActiveByTenantProductID(db *gorm.DB, tenantID, productID int64) int64 {
	if db == nil || tenantID <= 0 || productID <= 0 {
		return 0
	}
	documentTable := db.NamingStrategy.TableName("KnowledgeDocument")
	linkTable := db.NamingStrategy.TableName("ProductKnowledgeLink")
	var count int64
	db.Table(documentTable+" AS knowledge_documents").
		Joins("JOIN "+linkTable+" AS product_knowledge_links ON product_knowledge_links.tenant_id = knowledge_documents.tenant_id AND product_knowledge_links.knowledge_base_id = knowledge_documents.knowledge_base_id AND product_knowledge_links.knowledge_entry_id = knowledge_documents.id").
		Where("knowledge_documents.tenant_id = ? AND knowledge_documents.status <> ?", tenantID, enums.StatusDeleted).
		Where("product_knowledge_links.product_id = ? AND product_knowledge_links.status <> ? AND product_knowledge_links.link_type <> ?", productID, enums.StatusDeleted, "faq").
		Distinct("knowledge_documents.id").
		Count(&count)
	return count
}

func (r *knowledgeDocumentRepository) ActiveUsageByTenantID(db *gorm.DB, tenantID int64) (count int64, totalFileSize int64, err error) {
	if db == nil || tenantID <= 0 {
		return 0, 0, nil
	}
	documentTable := db.NamingStrategy.TableName("KnowledgeDocument")
	assetTable := db.NamingStrategy.TableName("Asset")
	var row struct {
		DocumentCount int64
		TotalFileSize int64
	}
	err = db.Table(documentTable+" AS knowledge_documents").
		Select("COUNT(*) AS document_count, COALESCE(SUM(CASE WHEN knowledge_documents.source_asset_id > 0 THEN assets.file_size ELSE LENGTH(knowledge_documents.content) END), 0) AS total_file_size").
		Joins("LEFT JOIN "+assetTable+" AS assets ON assets.id = knowledge_documents.source_asset_id").
		Where("knowledge_documents.tenant_id = ? AND knowledge_documents.status <> ?", tenantID, enums.StatusDeleted).
		Scan(&row).Error
	return row.DocumentCount, row.TotalFileSize, err
}

func (r *knowledgeDocumentRepository) ActiveUsageByTenantUserID(db *gorm.DB, tenantID, userID int64) (count int64, totalFileSize int64, err error) {
	if db == nil || tenantID <= 0 || userID <= 0 {
		return 0, 0, nil
	}
	documentTable := db.NamingStrategy.TableName("KnowledgeDocument")
	assetTable := db.NamingStrategy.TableName("Asset")
	var row struct {
		DocumentCount int64
		TotalFileSize int64
	}
	err = db.Table(documentTable+" AS knowledge_documents").
		Select("COUNT(*) AS document_count, COALESCE(SUM(CASE WHEN knowledge_documents.source_asset_id > 0 THEN assets.file_size ELSE LENGTH(knowledge_documents.content) END), 0) AS total_file_size").
		Joins("LEFT JOIN "+assetTable+" AS assets ON assets.id = knowledge_documents.source_asset_id").
		Where("knowledge_documents.tenant_id = ? AND knowledge_documents.create_user_id = ? AND knowledge_documents.status <> ?", tenantID, userID, enums.StatusDeleted).
		Scan(&row).Error
	return row.DocumentCount, row.TotalFileSize, err
}
