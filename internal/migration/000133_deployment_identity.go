package migration

import (
	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/pkg/deploymentidentity"
)

func init() {
	register(133, "add immutable deployment instance binding", func() error {
		return sqls.DB().AutoMigrate(&deploymentidentity.Record{})
	})
}
