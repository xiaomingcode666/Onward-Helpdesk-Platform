package services

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/secretstore"
	"remotehelpdesk/internal/pkg/utils"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const (
	ChannelSecretMetaAccessToken    = "meta_access_token"
	ChannelSecretMetaAppSecret      = "meta_app_secret"
	ChannelSecretWebhookVerifyToken = "webhook_verify_token"

	channelSecretMaxLength = 4096
)

// whatsAppSecretKeys maps interface keys to stored names in a stable order.
var whatsAppSecretKeys = []struct{ Key, Name string }{
	{"metaAccessToken", ChannelSecretMetaAccessToken},
	{"metaAppSecret", ChannelSecretMetaAppSecret},
	{"webhookVerifyToken", ChannelSecretWebhookVerifyToken},
}

type ChannelSecretChanges struct {
	Set   map[string]string
	Clear map[string]bool
}

func (c ChannelSecretChanges) Empty() bool { return len(c.Set) == 0 && len(c.Clear) == 0 }

var ChannelSecretService = &channelSecretService{}

type channelSecretService struct{}

func NormalizeWhatsAppSecretsInput(in *request.WhatsAppSecretsInput) (ChannelSecretChanges, error) {
	changes := ChannelSecretChanges{Set: map[string]string{}, Clear: map[string]bool{}}
	if in == nil {
		return changes, nil
	}
	nameByKey := make(map[string]string, len(whatsAppSecretKeys))
	for _, item := range whatsAppSecretKeys {
		nameByKey[item.Key] = item.Name
	}
	for _, key := range in.Clear {
		name, ok := nameByKey[strings.TrimSpace(key)]
		if !ok {
			return changes, errorsx.InvalidParam("unsupported WhatsApp secret: " + key)
		}
		changes.Clear[name] = true
	}
	values := map[string]string{
		"metaAccessToken":    in.MetaAccessToken,
		"metaAppSecret":      in.MetaAppSecret,
		"webhookVerifyToken": in.WebhookVerifyToken,
	}
	for _, item := range whatsAppSecretKeys {
		value := strings.TrimSpace(values[item.Key])
		if value == "" {
			continue
		}
		if changes.Clear[item.Name] {
			return changes, errorsx.InvalidParam("WhatsApp secret cannot be set and cleared together: " + item.Key)
		}
		if len(value) > channelSecretMaxLength || strings.IndexFunc(value, unicode.IsSpace) >= 0 || strings.HasPrefix(strings.ToLower(value), "secret://") {
			return changes, errorsx.InvalidParam("invalid WhatsApp secret value: " + item.Key)
		}
		changes.Set[item.Name] = value
	}
	return changes, nil
}

// find tolerates a missing table so callers on databases that have not run
// migration 161 fall back to secret file references.
func (s *channelSecretService) find(db *gorm.DB, channelID int64) []models.ChannelSecret {
	var list []models.ChannelSecret
	if channelID <= 0 {
		return list
	}
	_ = db.Where("channel_id = ?", channelID).Find(&list).Error
	return list
}

func (s *channelSecretService) ConfiguredNames(channelID int64) map[string]bool {
	ret := map[string]bool{}
	for _, item := range s.find(sqls.DB(), channelID) {
		ret[item.Name] = true
	}
	return ret
}

func (s *channelSecretService) Statuses(channelID int64) map[string]response.ChannelSecretStatusResponse {
	stored := map[string]models.ChannelSecret{}
	for _, item := range s.find(sqls.DB(), channelID) {
		stored[item.Name] = item
	}
	ret := make(map[string]response.ChannelSecretStatusResponse, len(whatsAppSecretKeys))
	for _, key := range whatsAppSecretKeys {
		item, ok := stored[key.Name]
		if !ok {
			ret[key.Key] = response.ChannelSecretStatusResponse{}
			continue
		}
		ret[key.Key] = response.ChannelSecretStatusResponse{
			Configured:        true,
			FingerprintSuffix: fingerprintSuffix(item.Fingerprint),
			UpdatedAt:         item.UpdatedAt.Format(time.DateTime),
		}
	}
	return ret
}

func fingerprintSuffix(fingerprint string) string {
	if len(fingerprint) <= 4 {
		return fingerprint
	}
	return fingerprint[len(fingerprint)-4:]
}

// Available reports whether a secret can be resolved, without decrypting it.
func (s *channelSecretService) Available(channel *models.Channel, name, fallbackRef string) bool {
	if channel == nil {
		return false
	}
	if strings.TrimSpace(fallbackRef) != "" {
		return true
	}
	return s.ConfiguredNames(channel.ID)[name]
}

// Resolve prefers the dashboard-entered secret, then the deployment secret
// file reference. Decryption failures fail closed and never log the value.
func (s *channelSecretService) Resolve(channel *models.Channel, name, fallbackRef string) (string, error) {
	if channel == nil {
		return "", errors.New("WhatsApp channel is required")
	}
	var item models.ChannelSecret
	if err := sqls.DB().Where("channel_id = ? AND name = ?", channel.ID, name).Take(&item).Error; err == nil {
		plain, decryptErr := secretstore.Decrypt(item.Ciphertext)
		plain = strings.TrimSpace(plain)
		if decryptErr != nil || plain == "" {
			slog.Warn("channel secret decrypt failed", "channelId", channel.ID, "name", name)
			return "", errors.New("channel secret cannot be decrypted")
		}
		return plain, nil
	}
	if strings.TrimSpace(fallbackRef) == "" {
		return "", errors.New("channel secret is not configured")
	}
	return resolveWhatsAppChannelSecret(channel, fallbackRef)
}

func (s *channelSecretService) ApplyTx(txCtx *sqls.TxContext, channel *models.Channel, changes ChannelSecretChanges, operator *dto.AuthPrincipal) error {
	if changes.Empty() {
		return nil
	}
	if txCtx == nil || txCtx.Tx == nil || channel == nil || channel.ID <= 0 || operator == nil {
		return errors.New("channel secret transaction requires channel and operator")
	}
	tx := txCtx.Tx
	tenantID := int64(0)
	if agent := AIAgentService.Get(channel.AIAgentID); agent != nil {
		tenantID = agent.TenantID
	}
	for _, key := range whatsAppSecretKeys {
		name := key.Name
		if changes.Clear[name] {
			result := tx.Where("channel_id = ? AND name = ?", channel.ID, name).Delete(&models.ChannelSecret{})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				if err := s.recordAuditTx(txCtx, channel.ID, tenantID, operator, name, "cleared", ""); err != nil {
					return err
				}
			}
			continue
		}
		value, ok := changes.Set[name]
		if !ok {
			continue
		}
		ciphertext, err := secretstore.Encrypt(value)
		if err != nil || ciphertext == "" {
			return errorsx.InvalidParam("WhatsApp secret could not be saved")
		}
		fingerprint := secretstore.Fingerprint(value)
		var existing models.ChannelSecret
		err = tx.Where("channel_id = ? AND name = ?", channel.ID, name).Take(&existing).Error
		switch {
		case err == nil:
			if err := tx.Model(&models.ChannelSecret{}).Where("id = ?", existing.ID).Updates(map[string]any{
				"ciphertext":       ciphertext,
				"fingerprint":      fingerprint,
				"update_user_id":   operator.UserID,
				"update_user_name": operator.Username,
				"updated_at":       time.Now(),
			}).Error; err != nil {
				return err
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			item := &models.ChannelSecret{ChannelID: channel.ID, Name: name, Ciphertext: ciphertext, Fingerprint: fingerprint, AuditFields: utils.BuildAuditFields(operator)}
			if err := tx.Create(item).Error; err != nil {
				return err
			}
		default:
			return err
		}
		if err := s.recordAuditTx(txCtx, channel.ID, tenantID, operator, name, "set", fingerprintSuffix(fingerprint)); err != nil {
			return err
		}
	}
	return nil
}

func (s *channelSecretService) recordAuditTx(txCtx *sqls.TxContext, channelID, tenantID int64, operator *dto.AuthPrincipal, name, action, suffix string) error {
	if !txCtx.Tx.Migrator().HasTable(&models.AuditLog{}) {
		return nil
	}
	return AuditService.RecordAuditTx(txCtx, RecordAuditInput{
		TenantID:     tenantID,
		ActorID:      strconv.FormatInt(operator.UserID, 10),
		ActorType:    "user",
		Domain:       "settings",
		ResourceType: "channel_secret",
		ResourceID:   strconv.FormatInt(channelID, 10),
		Action:       action,
		AfterState:   map[string]any{"name": name, "fingerprintSuffix": suffix},
		RiskLevel:    models.RiskLevelHigh,
	})
}

func (s *channelSecretService) DeleteAllTx(tx *gorm.DB, channelID int64) error {
	if tx == nil || channelID <= 0 || !tx.Migrator().HasTable(&models.ChannelSecret{}) {
		return nil
	}
	return tx.Where("channel_id = ?", channelID).Delete(&models.ChannelSecret{}).Error
}
