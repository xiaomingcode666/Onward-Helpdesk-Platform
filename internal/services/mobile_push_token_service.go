package services

import (
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
)

var MobilePushTokenService = &mobilePushTokenService{}

type mobilePushTokenService struct{}

func (s *mobilePushTokenService) Register(tenantID, userID int64, req request.RegisterMobilePushTokenRequest) (*dto.MobilePushTokenDTO, error) {
	if tenantID <= 0 || userID <= 0 {
		return nil, errorsx.InvalidParam("tenant and user are required")
	}
	if member := repositories.EnterpriseIAMRepository.FindTenantMemberByUserID(sqls.DB(), tenantID, userID); member == nil || member.Status != enums.StatusOk {
		return nil, errorsx.Forbidden("mobile push registration is outside the tenant")
	}
	return s.register(tenantID, userID, req)
}

func (s *mobilePushTokenService) RegisterCustomer(tenantID, userID, customerUserID int64, req request.RegisterMobilePushTokenRequest) (*dto.MobilePushTokenDTO, error) {
	if tenantID <= 0 || userID <= 0 || customerUserID <= 0 {
		return nil, errorsx.InvalidParam("tenant, user, and customer are required")
	}
	customerUser := repositories.CustomerPortalRepository.FindCustomerUserByTenantAndUserID(sqls.DB(), tenantID, userID)
	if customerUser == nil || customerUser.ID != customerUserID || customerUser.Status != enums.StatusOk {
		return nil, errorsx.Forbidden("mobile push registration is outside the customer account")
	}
	return s.register(tenantID, userID, req)
}

func (s *mobilePushTokenService) register(tenantID, userID int64, req request.RegisterMobilePushTokenRequest) (*dto.MobilePushTokenDTO, error) {
	platform, err := normalizeMobilePushPlatform(req.Platform)
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(req.Token)
	if err := validateMobilePushToken(token); err != nil {
		return nil, err
	}
	deviceID := strings.TrimSpace(req.DeviceID)
	if len(deviceID) > 128 {
		return nil, errorsx.InvalidParam("deviceId must not exceed 128 characters")
	}
	appVersion := strings.TrimSpace(req.AppVersion)
	if len(appVersion) > 32 {
		return nil, errorsx.InvalidParam("appVersion must not exceed 32 characters")
	}
	ciphertext, err := secretstore.Encrypt(token)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	item := &models.MobilePushToken{
		TenantID:         tenantID,
		UserID:           userID,
		Platform:         platform,
		TokenFingerprint: secretstore.Fingerprint(token),
		TokenCiphertext:  ciphertext,
		DeviceID:         deviceID,
		AppVersion:       appVersion,
		LastSeenAt:       now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := repositories.MobilePushTokenRepository.Upsert(sqls.DB(), item); err != nil {
		return nil, err
	}
	return buildMobilePushTokenDTO(item), nil
}

func normalizeMobilePushPlatform(value string) (string, error) {
	platform := strings.ToLower(strings.TrimSpace(value))
	if platform != "android" && platform != "ios" {
		return "", errorsx.InvalidParam("platform must be android or ios")
	}
	return platform, nil
}

func validateMobilePushToken(value string) error {
	if len(strings.TrimSpace(value)) < 16 || len(value) > 4096 {
		return errorsx.InvalidParam("invalid mobile push token")
	}
	return nil
}

func (s *mobilePushTokenService) Revoke(tenantID, userID, id int64) error {
	if tenantID <= 0 || userID <= 0 || id <= 0 {
		return errorsx.InvalidParam("tenant, user, and token are required")
	}
	return repositories.MobilePushTokenRepository.Revoke(sqls.DB(), tenantID, userID, id, time.Now())
}

func buildMobilePushTokenDTO(item *models.MobilePushToken) *dto.MobilePushTokenDTO {
	if item == nil {
		return nil
	}
	return &dto.MobilePushTokenDTO{
		ID:         item.ID,
		Platform:   item.Platform,
		DeviceID:   item.DeviceID,
		AppVersion: item.AppVersion,
		LastSeenAt: item.LastSeenAt.Format(time.RFC3339),
	}
}
