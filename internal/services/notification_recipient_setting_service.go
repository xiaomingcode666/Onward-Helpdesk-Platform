package services

import (
	"net/mail"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var NotificationRecipientSettingService = &notificationRecipientSettingService{}

type notificationRecipientSettingService struct{}

func (s *notificationRecipientSettingService) Get(tenantID, userID int64) (*dto.EnterpriseNotificationRecipientSettingDTO, error) {
	user, err := s.resolveUser(tenantID, userID)
	if err != nil {
		return nil, err
	}
	setting := repositories.NotificationRecipientSettingRepository.Get(sqls.DB(), tenantID, userID)
	return buildNotificationRecipientSettingDTO(user, setting), nil
}

func (s *notificationRecipientSettingService) Save(tenantID, userID int64, req request.UpdateNotificationRecipientSettingRequest) (*dto.EnterpriseNotificationRecipientSettingDTO, error) {
	user, err := s.resolveUser(tenantID, userID)
	if err != nil {
		return nil, err
	}
	override := strings.TrimSpace(req.Email)
	if req.UseProfileEmail {
		if req.EmailEnabled && notificationProfileEmail(user) == "" {
			return nil, errorsx.InvalidParam("profile email is required when email notifications are enabled")
		}
		override = ""
	} else if err := validateNotificationEmail(override); err != nil {
		return nil, err
	}
	now := time.Now()
	setting := &models.NotificationRecipientSetting{
		TenantID:      tenantID,
		UserID:        userID,
		EmailOverride: override,
		EmailEnabled:  req.EmailEnabled,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := repositories.NotificationRecipientSettingRepository.Upsert(sqls.DB(), setting); err != nil {
		return nil, err
	}
	return buildNotificationRecipientSettingDTO(user, setting), nil
}

// ResolveEmail returns the effective address used when a notification delivery
// is scheduled. A disabled email preference intentionally resolves to empty.
func (s *notificationRecipientSettingService) ResolveEmail(tenantID, userID int64) string {
	user, err := s.resolveUser(tenantID, userID)
	if err != nil {
		return ""
	}
	setting := repositories.NotificationRecipientSettingRepository.Get(sqls.DB(), tenantID, userID)
	if setting != nil {
		if !setting.EmailEnabled {
			return ""
		}
		if email := strings.TrimSpace(setting.EmailOverride); email != "" {
			return email
		}
	}
	return notificationProfileEmail(user)
}

func (s *notificationRecipientSettingService) resolveUser(tenantID, userID int64) (*models.User, error) {
	if tenantID <= 0 || userID <= 0 {
		return nil, errorsx.InvalidParam("tenant and user are required")
	}
	if repositories.EnterpriseIAMRepository.FindTenantMemberByUserID(sqls.DB(), tenantID, userID) == nil {
		return nil, errorsx.InvalidParam("enterprise member not found")
	}
	user := repositories.UserRepository.Get(sqls.DB(), userID)
	if user == nil {
		return nil, errorsx.InvalidParam("user not found")
	}
	return user, nil
}

func validateNotificationEmail(value string) error {
	if value == "" {
		return errorsx.InvalidParam("recipient email is required")
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || !strings.EqualFold(parsed.Address, value) {
		return errorsx.InvalidParam("invalid recipient email")
	}
	return nil
}

func notificationProfileEmail(user *models.User) string {
	if user == nil || user.Email == nil {
		return ""
	}
	return strings.TrimSpace(*user.Email)
}

func buildNotificationRecipientSettingDTO(user *models.User, setting *models.NotificationRecipientSetting) *dto.EnterpriseNotificationRecipientSettingDTO {
	profileEmail := notificationProfileEmail(user)
	override := ""
	emailEnabled := true
	updatedAt := ""
	if setting != nil {
		override = strings.TrimSpace(setting.EmailOverride)
		emailEnabled = setting.EmailEnabled
		updatedAt = setting.UpdatedAt.Format(time.RFC3339)
	}
	effective := profileEmail
	if override != "" {
		effective = override
	}
	return &dto.EnterpriseNotificationRecipientSettingDTO{
		ProfileEmail:    profileEmail,
		OverrideEmail:   override,
		EffectiveEmail:  effective,
		UseProfileEmail: override == "",
		EmailEnabled:    emailEnabled,
		ProfileSource:   "enterprise_people",
		UpdatedAt:       updatedAt,
	}
}
