package services

import (
	"fmt"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/secretstore"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var MailSettingService = newMailSettingService()

func newMailSettingService() *mailSettingService {
	return &mailSettingService{}
}

type mailSettingService struct{}

// defaultMailSetting 未配置时返回的原型默认值。
func defaultMailSetting() *models.TenantMailSetting {
	return &models.TenantMailSetting{
		SMTPPort:    465,
		RetryPolicy: "retry_3_10m",
		UseTLS:      true,
		Status:      int(enums.StatusDisabled),
	}
}

func (s *mailSettingService) Get(tenantID int64) (*dto.EnterpriseNotificationMailSettingDTO, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	r, _, err := projectRuntimeDB(sqls.DB(), tenantID, 0)
	if err != nil {
		return nil, err
	}
	if r != nil {
		m := r.Mail
		result := &dto.EnterpriseNotificationMailSettingDTO{SMTPHost: m.Host, SMTPPort: m.Port, Username: m.Username, FromAddress: m.FromAddress, FromName: m.FromName, UseTLS: m.UseTLS, Connected: m.Enabled, HasPassword: m.PasswordRef != "", ReplyTo: m.ReplyTo, RetryPolicy: m.RetryPolicy, ManagedByProject: true, IMAPUseTLS: true, IMAPPort: 993}
		if i := m.IMAP; i != nil {
			result.IMAPHost = i.Host
			result.IMAPPort = i.Port
			result.IMAPUsername = i.Username
			result.IMAPEnabled = i.Enabled
			result.HasIMAPPassword = i.PasswordRef != ""
		}
		return result, nil
	}
	setting := repositories.TenantMailSettingRepository.GetByTenantID(sqls.DB(), tenantID)
	if setting == nil {
		setting = defaultMailSetting()
	}
	return buildMailSettingDTO(setting), nil
}

func (s *mailSettingService) Save(tenantID int64, req request.UpdateMailSettingRequest) (*dto.EnterpriseNotificationMailSettingDTO, error) {
	if err := requireLegacyProjectSettingsDB(sqls.DB(), tenantID); err != nil {
		return nil, err
	}
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	if strings.TrimSpace(req.FromAddress) == "" {
		return nil, errorsx.InvalidParam("from address is required")
	}
	if strings.TrimSpace(req.SMTPHost) == "" {
		return nil, errorsx.InvalidParam("smtp host is required")
	}
	if req.SMTPPort <= 0 || req.SMTPPort > 65535 {
		return nil, errorsx.InvalidParam("invalid smtp port")
	}
	retryPolicy := strings.TrimSpace(req.RetryPolicy)
	switch retryPolicy {
	case "retry_3_10m", "retry_1_5m", "no_retry":
	case "":
		retryPolicy = "retry_3_10m"
	default:
		return nil, errorsx.InvalidParam("invalid retry policy")
	}

	now := time.Now()
	req.IMAPHost = strings.TrimSpace(req.IMAPHost)
	req.IMAPUsername = strings.TrimSpace(req.IMAPUsername)
	if req.IMAPPort == 0 {
		req.IMAPPort = 993
	}
	if req.IMAPEnabled {
		if req.IMAPHost == "" || strings.ContainsAny(req.IMAPHost, "/\r\n ") || req.IMAPPort < 1 || req.IMAPPort > 65535 || !req.IMAPUseTLS {
			return nil, errorsx.InvalidParam("收件需填写有效 IMAP 地址、端口并使用 TLS")
		}
		if _, err := emailAddress(req.IMAPUsername); err != nil {
			return nil, errorsx.InvalidParam("IMAP 账号必须是邮箱地址")
		}
	}
	existing := repositories.TenantMailSettingRepository.GetByTenantID(sqls.DB(), tenantID)
	password := strings.TrimSpace(req.Password)
	if password != "" {
		var err error
		password, err = secretstore.Encrypt(password)
		if err != nil {
			return nil, err
		}
	} else if existing != nil {
		password = existing.Password // 留空沿用已保存的密码
		if password != "" && !strings.HasPrefix(password, "enc:v1:") {
			var err error
			password, err = secretstore.Encrypt(password)
			if err != nil {
				return nil, err
			}
		}
	}
	imapPassword := strings.TrimSpace(req.IMAPPassword)
	if imapPassword != "" {
		var err error
		imapPassword, err = secretstore.Encrypt(imapPassword)
		if err != nil {
			return nil, err
		}
	} else if existing != nil {
		imapPassword = existing.IMAPPassword
	}
	if req.IMAPEnabled && imapPassword == "" {
		return nil, errorsx.InvalidParam("请填写 IMAP 授权码")
	}
	item := &models.TenantMailSetting{
		TenantID:    tenantID,
		FromAddress: strings.TrimSpace(req.FromAddress),
		FromName:    strings.TrimSpace(req.FromName),
		SMTPHost:    strings.TrimSpace(req.SMTPHost),
		SMTPPort:    req.SMTPPort,
		Username:    strings.TrimSpace(req.Username),
		Password:    password,
		ReplyTo:     strings.TrimSpace(req.ReplyTo),
		RetryPolicy: retryPolicy,
		UseTLS:      req.UseTLS,
		IMAPHost:    req.IMAPHost, IMAPPort: req.IMAPPort, IMAPUsername: req.IMAPUsername,
		IMAPPassword: imapPassword, IMAPUseTLS: req.IMAPUseTLS, IMAPEnabled: req.IMAPEnabled,
		Status:    int(enums.StatusOk),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := legacyProjectSettingsWrite(tenantID, func(db *gorm.DB) error { return repositories.TenantMailSettingRepository.Upsert(db, item) }); err != nil {
		return nil, err
	}
	return buildMailSettingDTO(item), nil
}

// SendTest 用当前租户邮箱配置发送一封测试邮件。
func (s *mailSettingService) SendTest(tenantID int64, to string) error {
	if tenantID <= 0 {
		return errorsx.InvalidParam("tenant is required")
	}
	to = strings.TrimSpace(to)
	if to == "" {
		return errorsx.InvalidParam("recipient is required")
	}
	setting, err := s.Get(tenantID)
	if err != nil {
		return err
	}
	fromAddress := setting.FromAddress
	if fromAddress == "" {
		fromAddress = to
	}
	return EmailNotificationService.SendEmail(tenantID, to, "", "mail_test", map[string]interface{}{
		"FromAddress": fromAddress,
		"SMTPServer":  fmt.Sprintf("%s:%d", setting.SMTPHost, setting.SMTPPort),
		"SentAt":      time.Now().Format("2006-01-02 15:04:05"),
	})
}

func buildMailSettingDTO(setting *models.TenantMailSetting) *dto.EnterpriseNotificationMailSettingDTO {
	return &dto.EnterpriseNotificationMailSettingDTO{
		FromAddress: setting.FromAddress,
		FromName:    setting.FromName,
		SMTPHost:    setting.SMTPHost,
		SMTPPort:    setting.SMTPPort,
		Username:    setting.Username,
		ReplyTo:     setting.ReplyTo,
		RetryPolicy: setting.RetryPolicy,
		UseTLS:      setting.UseTLS,
		IMAPHost:    setting.IMAPHost, IMAPPort: setting.IMAPPort, IMAPUsername: setting.IMAPUsername,
		HasIMAPPassword: setting.IMAPPassword != "", IMAPUseTLS: setting.IMAPUseTLS, IMAPEnabled: setting.IMAPEnabled,
		Connected:   setting.Status == int(enums.StatusOk) && strings.TrimSpace(setting.SMTPHost) != "" && strings.TrimSpace(setting.FromAddress) != "",
		HasPassword: setting.Password != "",
	}
}
