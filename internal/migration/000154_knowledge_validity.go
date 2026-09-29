package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(154, "add knowledge validity metadata", func() error {
		return sqls.DB().AutoMigrate(&models.KnowledgeDocument{}, &models.KnowledgeFAQ{})
	})
}
