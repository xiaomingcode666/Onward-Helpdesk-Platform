package migration

import (
	"strconv"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(74, "separate tenant-shared and product knowledge bases", func() error {
		return ensureKnowledgeBaseAccessScope(sqls.DB())
	})
}

func ensureKnowledgeBaseAccessScope(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.KnowledgeBase{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&models.KnowledgeBase{}, "AccessScope") {
		if err := db.Migrator().AddColumn(&models.KnowledgeBase{}, "AccessScope"); err != nil {
			return err
		}
	}
	if err := db.Model(&models.KnowledgeBase{}).
		Where("access_scope IS NULL OR access_scope = '' OR access_scope NOT IN ?", []string{
			string(enums.KnowledgeBaseAccessScopeTenant),
			string(enums.KnowledgeBaseAccessScopeProduct),
		}).
		Update("access_scope", string(enums.KnowledgeBaseAccessScopeTenant)).Error; err != nil {
		return err
	}

	productKnowledgeBaseIDs := make(map[int64]struct{})
	collectKnowledgeBaseIDs := func(model any, column string, where string, args ...any) error {
		if !db.Migrator().HasTable(model) {
			return nil
		}
		var ids []int64
		query := db.Model(model).Distinct(column).Where(column + " > 0")
		if strings.TrimSpace(where) != "" {
			query = query.Where(where, args...)
		}
		if err := query.Pluck(column, &ids).Error; err != nil {
			return err
		}
		for _, id := range ids {
			productKnowledgeBaseIDs[id] = struct{}{}
		}
		return nil
	}
	if err := collectKnowledgeBaseIDs(&models.ProductServiceProfile{}, "default_knowledge_base_id", "status <> ?", enums.StatusDeleted); err != nil {
		return err
	}
	if err := collectKnowledgeBaseIDs(&models.ProductKnowledgeBinding{}, "knowledge_base_id", "status <> ?", enums.StatusDeleted); err != nil {
		return err
	}
	if err := collectKnowledgeBaseIDs(&models.ProductKnowledgeLink{}, "knowledge_base_id", "status <> ?", enums.StatusDeleted); err != nil {
		return err
	}
	if db.Migrator().HasTable(&models.AIAgent{}) {
		var agents []models.AIAgent
		if err := db.Select("knowledge_ids").Where("status <> ?", enums.StatusDeleted).Find(&agents).Error; err != nil {
			return err
		}
		for _, agent := range agents {
			for _, value := range strings.Split(agent.KnowledgeIDs, ",") {
				id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
				if err == nil && id > 0 {
					productKnowledgeBaseIDs[id] = struct{}{}
				}
			}
		}
	}
	if len(productKnowledgeBaseIDs) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(productKnowledgeBaseIDs))
	for id := range productKnowledgeBaseIDs {
		ids = append(ids, id)
	}
	return db.Model(&models.KnowledgeBase{}).
		Where("id IN ?", ids).
		Update("access_scope", string(enums.KnowledgeBaseAccessScopeProduct)).Error
}
