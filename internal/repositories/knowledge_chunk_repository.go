package repositories

import (
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var KnowledgeChunkRepository = newKnowledgeChunkRepository()

func newKnowledgeChunkRepository() *knowledgeChunkRepository {
	return &knowledgeChunkRepository{}
}

type knowledgeChunkRepository struct {
}

type KnowledgeChunkLexicalQuery struct {
	TenantID          int64
	KnowledgeBaseIDs  []int64
	DocumentIDs       []int64
	FAQIDs            []int64
	RevisionIDs       []int64
	Languages         []string
	ReviewStatus      string
	IndexGenerationID int64
	Terms             []string
	Limit             int
}

func (r *knowledgeChunkRepository) Get(db *gorm.DB, id int64) *models.KnowledgeChunk {
	ret := &models.KnowledgeChunk{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *knowledgeChunkRepository) Take(db *gorm.DB, where ...interface{}) *models.KnowledgeChunk {
	ret := &models.KnowledgeChunk{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *knowledgeChunkRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.KnowledgeChunk) {
	cnd.Find(db, &list)
	return
}

func (r *knowledgeChunkRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.KnowledgeChunk {
	ret := &models.KnowledgeChunk{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *knowledgeChunkRepository) FindPageByParams(db *gorm.DB, params *params.QueryParams) (list []models.KnowledgeChunk, paging *sqls.Paging) {
	return r.FindPageByCnd(db, &params.Cnd)
}

func (r *knowledgeChunkRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.KnowledgeChunk, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.KnowledgeChunk{})

	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *knowledgeChunkRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.KnowledgeChunk{})
}

func (r *knowledgeChunkRepository) Create(db *gorm.DB, t *models.KnowledgeChunk) (err error) {
	err = db.Create(t).Error
	return
}

func (r *knowledgeChunkRepository) BatchCreate(db *gorm.DB, list []models.KnowledgeChunk) (err error) {
	err = db.Create(&list).Error
	return
}

func (r *knowledgeChunkRepository) ReplaceByDocumentID(db *gorm.DB, documentID int64, list []models.KnowledgeChunk) error {
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := ctx.Tx.Where("document_id = ?", documentID).Delete(&models.KnowledgeChunk{}).Error; err != nil {
			return err
		}
		if len(list) == 0 {
			return nil
		}
		return ctx.Tx.Create(&list).Error
	})
}

func (r *knowledgeChunkRepository) ReplaceByDocumentIDAndGeneration(db *gorm.DB, documentID, generationID int64, list []models.KnowledgeChunk) error {
	return db.Transaction(func(tx *gorm.DB) error {
		query := tx.Where("document_id = ?", documentID)
		if generationID > 0 {
			query = query.Where("index_generation_id = ?", generationID)
		}
		if err := query.Delete(&models.KnowledgeChunk{}).Error; err != nil {
			return err
		}
		if len(list) == 0 {
			return nil
		}
		return tx.Create(&list).Error
	})
}

func (r *knowledgeChunkRepository) ReplaceByFaqID(db *gorm.DB, faqID int64, item *models.KnowledgeChunk) error {
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := ctx.Tx.Where("faq_id = ?", faqID).Delete(&models.KnowledgeChunk{}).Error; err != nil {
			return err
		}
		if item == nil {
			return nil
		}
		return ctx.Tx.Create(item).Error
	})
}

func (r *knowledgeChunkRepository) ReplaceByFaqIDAndGeneration(db *gorm.DB, faqID, generationID int64, item *models.KnowledgeChunk) error {
	return db.Transaction(func(tx *gorm.DB) error {
		query := tx.Where("faq_id = ?", faqID)
		if generationID > 0 {
			query = query.Where("index_generation_id = ?", generationID)
		}
		if err := query.Delete(&models.KnowledgeChunk{}).Error; err != nil {
			return err
		}
		if item == nil {
			return nil
		}
		return tx.Create(item).Error
	})
}

func (r *knowledgeChunkRepository) Update(db *gorm.DB, t *models.KnowledgeChunk) (err error) {
	err = db.Save(t).Error
	return
}

func (r *knowledgeChunkRepository) Updates(db *gorm.DB, id int64, columns map[string]interface{}) (err error) {
	err = db.Model(&models.KnowledgeChunk{}).Where("id = ?", id).Updates(columns).Error
	return
}

func (r *knowledgeChunkRepository) UpdateColumn(db *gorm.DB, id int64, name string, value interface{}) (err error) {
	err = db.Model(&models.KnowledgeChunk{}).Where("id = ?", id).UpdateColumn(name, value).Error
	return
}

func (r *knowledgeChunkRepository) Delete(db *gorm.DB, id int64) {
	db.Delete(&models.KnowledgeChunk{}, "id = ?", id)
}

func (r *knowledgeChunkRepository) DeleteByDocumentID(db *gorm.DB, documentID int64) error {
	return db.Delete(&models.KnowledgeChunk{}, "document_id = ?", documentID).Error
}

func (r *knowledgeChunkRepository) DeleteByFaqID(db *gorm.DB, faqID int64) error {
	return db.Delete(&models.KnowledgeChunk{}, "faq_id = ?", faqID).Error
}

func (r *knowledgeChunkRepository) DeleteByKnowledgeBaseID(db *gorm.DB, knowledgeBaseID int64) error {
	return db.Delete(&models.KnowledgeChunk{}, "knowledge_base_id = ?", knowledgeBaseID).Error
}

func (r *knowledgeChunkRepository) FindByDocumentID(db *gorm.DB, documentID int64) (list []models.KnowledgeChunk) {
	db.Where("document_id = ?", documentID).Order("chunk_no asc").Find(&list)
	return
}

func (r *knowledgeChunkRepository) FindByDocumentIDAndGeneration(db *gorm.DB, documentID, generationID int64) (list []models.KnowledgeChunk) {
	query := db.Where("document_id = ?", documentID)
	if generationID > 0 {
		query = query.Where("index_generation_id = ?", generationID)
	}
	query.Order("chunk_no asc").Find(&list)
	return
}

func (r *knowledgeChunkRepository) FindByFaqID(db *gorm.DB, faqID int64) (list []models.KnowledgeChunk) {
	db.Where("faq_id = ?", faqID).Order("chunk_no asc").Find(&list)
	return
}

func (r *knowledgeChunkRepository) FindByFaqIDAndGeneration(db *gorm.DB, faqID, generationID int64) (list []models.KnowledgeChunk) {
	query := db.Where("faq_id = ?", faqID)
	if generationID > 0 {
		query = query.Where("index_generation_id = ?", generationID)
	}
	query.Order("chunk_no asc").Find(&list)
	return
}

func (r *knowledgeChunkRepository) FindByKnowledgeBaseID(db *gorm.DB, knowledgeBaseID int64) (list []models.KnowledgeChunk) {
	db.Where("knowledge_base_id = ?", knowledgeBaseID).Order("id asc").Find(&list)
	return
}

func (r *knowledgeChunkRepository) FindByDocumentRevisionGeneration(db *gorm.DB, documentID, revisionID, generationID int64) (list []models.KnowledgeChunk) {
	query := db.Where("document_id = ?", documentID)
	if revisionID > 0 {
		query = query.Where("revision_id = ?", revisionID)
	}
	if generationID > 0 {
		query = query.Where("index_generation_id = ?", generationID)
	}
	query.Order("chunk_no asc").Find(&list)
	return
}

func (r *knowledgeChunkRepository) FindByFAQRevisionGeneration(db *gorm.DB, faqID, revisionID, generationID int64) (list []models.KnowledgeChunk) {
	query := db.Where("faq_id = ?", faqID)
	if revisionID > 0 {
		query = query.Where("revision_id = ?", revisionID)
	}
	if generationID > 0 {
		query = query.Where("index_generation_id = ?", generationID)
	}
	query.Order("chunk_no asc").Find(&list)
	return
}

func (r *knowledgeChunkRepository) FindByVectorIDs(db *gorm.DB, vectorIDs []string) (list []models.KnowledgeChunk) {
	if len(vectorIDs) == 0 {
		return nil
	}
	db.Where("vector_id IN ?", vectorIDs).Find(&list)
	return
}

func (r *knowledgeChunkRepository) FindLexicalCandidates(db *gorm.DB, req KnowledgeChunkLexicalQuery) (list []models.KnowledgeChunk) {
	if req.TenantID <= 0 || len(req.Terms) == 0 {
		return nil
	}
	if req.Limit <= 0 {
		req.Limit = 50
	}
	query := db.Model(&models.KnowledgeChunk{}).
		Where("tenant_id = ?", req.TenantID).
		Where("status = ?", enums.StatusOk)
	if len(req.KnowledgeBaseIDs) > 0 {
		query = query.Where("knowledge_base_id IN ?", req.KnowledgeBaseIDs)
	}
	if len(req.RevisionIDs) > 0 {
		query = query.Where("revision_id IN ?", req.RevisionIDs)
	}
	if len(req.Languages) > 0 {
		query = query.Where("language IN ?", req.Languages)
	}
	if strings.TrimSpace(req.ReviewStatus) != "" {
		query = query.Where("review_status = ?", strings.TrimSpace(req.ReviewStatus))
	}
	if req.IndexGenerationID > 0 {
		query = query.Where("index_generation_id = ?", req.IndexGenerationID)
	}
	switch {
	case len(req.DocumentIDs) > 0 && len(req.FAQIDs) > 0:
		query = query.Where(db.Where("document_id IN ?", req.DocumentIDs).Or("faq_id IN ?", req.FAQIDs))
	case len(req.DocumentIDs) > 0:
		query = query.Where("document_id IN ?", req.DocumentIDs)
	case len(req.FAQIDs) > 0:
		query = query.Where("faq_id IN ?", req.FAQIDs)
	}

	termClauses := make([]string, 0, len(req.Terms))
	termArgs := make([]any, 0, len(req.Terms)*3)
	for _, term := range req.Terms {
		normalized := strings.ToLower(strings.TrimSpace(term))
		if normalized == "" {
			continue
		}
		pattern := "%" + normalized + "%"
		termClauses = append(termClauses, "(LOWER(title) LIKE ? OR LOWER(content) LIKE ? OR LOWER(section_path) LIKE ?)")
		termArgs = append(termArgs, pattern, pattern, pattern)
	}
	if len(termClauses) == 0 {
		return nil
	}

	query.Where(strings.Join(termClauses, " OR "), termArgs...).
		Order("updated_at DESC, id ASC").
		Limit(req.Limit).
		Find(&list)
	return
}

func (r *knowledgeChunkRepository) CountByDocumentID(db *gorm.DB, documentID int64) int64 {
	var count int64
	db.Model(&models.KnowledgeChunk{}).Where("document_id = ?", documentID).Count(&count)
	return count
}

func (r *knowledgeChunkRepository) CountByFaqID(db *gorm.DB, faqID int64) int64 {
	var count int64
	db.Model(&models.KnowledgeChunk{}).Where("faq_id = ?", faqID).Count(&count)
	return count
}

func (r *knowledgeChunkRepository) CountByKnowledgeBaseID(db *gorm.DB, knowledgeBaseID int64) int64 {
	var count int64
	db.Model(&models.KnowledgeChunk{}).Where("knowledge_base_id = ?", knowledgeBaseID).Count(&count)
	return count
}
