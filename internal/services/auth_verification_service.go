package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/mail"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const authVerificationCodeTTL = 10 * time.Minute

var AuthVerificationService = &authVerificationService{}

type authVerificationService struct{}

func (s *authVerificationService) Send(req request.AuthVerificationCodeRequest) error {
	purpose, err := normalizeAuthVerificationPurpose(req.Purpose)
	if err != nil {
		return err
	}
	email, err := normalizeAuthVerificationEmail(req.Email)
	if err != nil {
		return err
	}
	if !smtpRuntimeConfigured(config.CurrentOrDefault().Email) {
		return errorsx.InvalidParam("平台 SMTP 未配置，请先在平台侧配置 SMTP Server")
	}
	code, err := generateNumericVerificationCode(6)
	if err != nil {
		return err
	}
	now := time.Now()
	item := &models.AuthVerificationCode{
		Purpose:     purpose,
		Email:       email,
		CodeHash:    hashAuthVerificationCode(purpose, email, code),
		ExpiresAt:   now.Add(authVerificationCodeTTL),
		Status:      models.AuthVerificationStatusPending,
		AuditFields: utils.BuildAuditFields(nil),
	}
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := ctx.Tx.Model(&models.AuthVerificationCode{}).
			Where("purpose = ? AND email = ? AND status = ?", purpose, email, models.AuthVerificationStatusPending).
			Updates(map[string]any{"status": models.AuthVerificationStatusExpired, "updated_at": now}).Error; err != nil {
			return err
		}
		return ctx.Tx.Create(item).Error
	}); err != nil {
		return err
	}
	return EmailNotificationService.SendEmail(0, email, "", "notification_generic", map[string]interface{}{
		"Title":     "RemoteHelpDesk 验证码",
		"Content":   fmt.Sprintf("您的验证码是：%s\n验证码 10 分钟内有效，请勿转发给他人。", code),
		"ActionURL": "",
	})
}

func (s *authVerificationService) Verify(purpose, email, code string, consume bool) error {
	return s.VerifyDB(sqls.DB(), purpose, email, code, consume)
}

func (s *authVerificationService) VerifyDB(db *gorm.DB, purpose, email, code string, consume bool) error {
	purpose, err := normalizeAuthVerificationPurpose(purpose)
	if err != nil {
		return err
	}
	email, err = normalizeAuthVerificationEmail(email)
	if err != nil {
		return err
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return errorsx.InvalidParam("verification code is required")
	}
	item := &models.AuthVerificationCode{}
	query := db.Where("purpose = ? AND email = ? AND code_hash = ? AND status = ?",
		purpose, email, hashAuthVerificationCode(purpose, email, code), models.AuthVerificationStatusPending)
	if consume {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.Order("id DESC").First(item).Error; err != nil {
		return errorsx.InvalidParam("verification code is invalid")
	}
	now := time.Now()
	if !item.ExpiresAt.After(now) {
		_ = db.Model(&models.AuthVerificationCode{}).Where("id = ?", item.ID).Updates(map[string]any{"status": models.AuthVerificationStatusExpired, "updated_at": now}).Error
		return errorsx.InvalidParam("verification code has expired")
	}
	if !consume {
		return nil
	}
	return db.Model(&models.AuthVerificationCode{}).Where("id = ?", item.ID).Updates(map[string]any{
		"status":      models.AuthVerificationStatusConsumed,
		"consumed_at": &now,
		"updated_at":  now,
	}).Error
}

func normalizeAuthVerificationPurpose(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case models.AuthVerificationPurposeEnterpriseRegister:
		return models.AuthVerificationPurposeEnterpriseRegister, nil
	case models.AuthVerificationPurposeCustomerRegister:
		return models.AuthVerificationPurposeCustomerRegister, nil
	case models.AuthVerificationPurposePasswordReset:
		return models.AuthVerificationPurposePasswordReset, nil
	default:
		return "", errorsx.InvalidParam("unsupported verification purpose")
	}
}

func normalizeAuthVerificationEmail(value string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(value))
	parsed, err := mail.ParseAddress(email)
	if err != nil || !strings.EqualFold(parsed.Address, email) {
		return "", errorsx.InvalidParam("a valid email address is required")
	}
	return email, nil
}

func generateNumericVerificationCode(length int) (string, error) {
	if length <= 0 {
		length = 6
	}
	var builder strings.Builder
	for builder.Len() < length {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		builder.WriteByte(byte('0' + n.Int64()))
	}
	return builder.String(), nil
}

func hashAuthVerificationCode(purpose, email, code string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(purpose) + "|" + strings.ToLower(strings.TrimSpace(email)) + "|" + strings.TrimSpace(code)))
	return hex.EncodeToString(sum[:])
}

func findUserByEmail(email string) *models.User {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil
	}
	return repositories.UserRepository.GetByEmail(sqls.DB(), email)
}
