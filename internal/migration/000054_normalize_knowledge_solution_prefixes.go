package migration

import (
	"strings"
	"time"

	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(54, "normalize duplicated solution prefixes in knowledge content", func() error {
		return normalizeKnowledgeSolutionPrefixes(sqls.DB())
	})
}

func normalizeKnowledgeSolutionPrefixes(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	now := time.Now()
	if db.Migrator().HasTable(&models.KnowledgeDocument{}) {
		var documents []models.KnowledgeDocument
		if err := db.Where("content LIKE ?", "%处理方案：处理方案：%").Find(&documents).Error; err != nil {
			return err
		}
		for i := range documents {
			next := normalizeDuplicatedSolutionPrefix(documents[i].Content)
			if next == documents[i].Content {
				continue
			}
			if err := db.Model(&models.KnowledgeDocument{}).Where("id = ?", documents[i].ID).Updates(map[string]any{
				"content":          next,
				"content_hash":     migrationContentHash("", next),
				"updated_at":       now,
				"update_user_name": "migration-54",
			}).Error; err != nil {
				return err
			}
		}
	}
	if db.Migrator().HasTable(&models.KnowledgeChunk{}) {
		var chunks []models.KnowledgeChunk
		if err := db.Where("content LIKE ?", "%处理方案：处理方案：%").Find(&chunks).Error; err != nil {
			return err
		}
		for i := range chunks {
			next := normalizeDuplicatedSolutionPrefix(chunks[i].Content)
			if next == chunks[i].Content {
				continue
			}
			if err := db.Model(&models.KnowledgeChunk{}).Where("id = ?", chunks[i].ID).Updates(map[string]any{
				"content":      next,
				"content_hash": migrationContentHash("", next),
				"updated_at":   now,
			}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func normalizeDuplicatedSolutionPrefix(content string) string {
	return strings.ReplaceAll(content, "处理方案：处理方案：", "处理方案：")
}
