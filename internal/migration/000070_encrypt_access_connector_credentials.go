package migration

import (
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/secretstore"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(70, "encrypt access connector credentials", func() error {
		var connectors []models.AccessConnector
		if err := sqls.DB().Where("auth_config <> ''").Find(&connectors).Error; err != nil {
			return err
		}
		for _, connector := range connectors {
			if strings.HasPrefix(strings.TrimSpace(connector.AuthConfig), "enc:v1:") {
				continue
			}
			encrypted, err := secretstore.Encrypt(connector.AuthConfig)
			if err != nil {
				return err
			}
			if err := sqls.DB().Model(&models.AccessConnector{}).
				Where("tenant_id = ? AND id = ?", connector.TenantID, connector.ID).
				Update("auth_config", encrypted).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
