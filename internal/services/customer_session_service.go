package services

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/openidentity"
	"remotehelpdesk/internal/repositories"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/mlogclub/simple/sqls"
)

const (
	customerSessionTokenType = "customer_session"
	customerSessionHeader    = "X-Customer-Session-Token"
	customerSessionExpHeader = "X-Customer-Session-Expires-At"
)

var CustomerSessionService = newCustomerSessionService()

func newCustomerSessionService() *customerSessionService {
	return &customerSessionService{}
}

type customerSessionService struct {
}

type customerSessionClaims struct {
	TokenType      string `json:"typ"`
	ChannelID      int64  `json:"channelId,omitempty"`
	ChannelCode    string `json:"channelCode,omitempty"`
	EntrySessionID int64  `json:"entrySessionId,omitempty"`
	TenantID       int64  `json:"tenantId,omitempty"`
	ProductID      int64  `json:"productId,omitempty"`
	CustomerID     int64  `json:"customerId"`
	CustomerName   string `json:"customerName"`
	IdentityKey    string `json:"identityKey"`
	jwt.RegisteredClaims
}

type CustomerSessionVerifyResult struct {
	ExternalUser   *openidentity.ExternalUser
	ChannelID      int64
	EntrySessionID int64
	TenantID       int64
	ProductID      int64
	Token          string
	ExpiresAt      time.Time
	Refreshed      bool
}

func (s *customerSessionService) ExchangeEntrySession(req request.ExchangeCustomerEntrySessionRequest) (*response.CustomerSessionExchangeResponse, error) {
	return nil, errorsx.Unauthorized("guest customer sessions are no longer supported; sign in with a customer account")
}

func (s *customerSessionService) Exchange(channel *models.Channel, externalUser openidentity.ExternalUser) (*response.CustomerSessionExchangeResponse, error) {
	if channel == nil || channel.Status != enums.StatusOk {
		return nil, errorsx.InvalidParamI18n("error.e0209")
	}
	var customerID int64
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		id, err := CustomerService.EnsureExternalCustomer(ctx, externalUser)
		if err != nil {
			return err
		}
		customerID = id
		return nil
	}); err != nil {
		return nil, err
	}
	customer := CustomerService.Get(customerID)
	if customer == nil || customer.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParamI18n("error.e0155")
	}
	token, expiresAt, err := s.Sign(channel, customer, externalUser)
	if err != nil {
		return nil, err
	}
	return &response.CustomerSessionExchangeResponse{
		CustomerSessionToken: token,
		ExpiresAt:            expiresAt.Format(time.DateTime),
		IdentityKey:          s.identityKey(externalUser),
		Customer: response.CustomerSessionCustomerResponse{
			ID:   customer.ID,
			Name: strings.TrimSpace(customer.Name),
		},
	}, nil
}

func (s *customerSessionService) Sign(channel *models.Channel, customer *models.Customer, externalUser openidentity.ExternalUser) (string, time.Time, error) {
	cfg := config.Current().CustomerSession
	secret := strings.TrimSpace(cfg.Secret)
	if secret == "" {
		return "", time.Time{}, errorsx.BusinessErrorI18n(1, "error.customerSession.secretMissing")
	}
	if channel == nil || customer == nil {
		return "", time.Time{}, errorsx.InvalidParamI18n("error.e0158")
	}
	now := time.Now()
	expiresAt := now.Add(time.Duration(cfg.TTL()) * time.Minute)
	claims := customerSessionClaims{
		TokenType:    customerSessionTokenType,
		ChannelID:    channel.ID,
		ChannelCode:  strings.TrimSpace(channel.ChannelID),
		CustomerID:   customer.ID,
		CustomerName: strings.TrimSpace(customer.Name),
		IdentityKey:  s.identityKey(externalUser),
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

func (s *customerSessionService) signEntrySession(session *models.CustomerEntrySession, customer *models.Customer, externalUser openidentity.ExternalUser) (string, time.Time, error) {
	cfg := config.Current().CustomerSession
	secret := strings.TrimSpace(cfg.Secret)
	if secret == "" {
		return "", time.Time{}, errorsx.BusinessErrorI18n(1, "error.customerSession.secretMissing")
	}
	if session == nil || customer == nil || session.TenantID <= 0 {
		return "", time.Time{}, errorsx.InvalidParam("customer entry session is invalid")
	}
	now := time.Now()
	expiresAt := now.Add(time.Duration(cfg.TTL()) * time.Minute)
	if session.ExpiresAt != nil && session.ExpiresAt.Before(expiresAt) {
		expiresAt = *session.ExpiresAt
	}
	claims := customerSessionClaims{
		TokenType:      customerSessionTokenType,
		EntrySessionID: session.ID,
		TenantID:       session.TenantID,
		ProductID:      customerEntrySessionProductID(session),
		CustomerID:     customer.ID,
		CustomerName:   strings.TrimSpace(customer.Name),
		IdentityKey:    s.identityKey(externalUser),
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

func (s *customerSessionService) VerifyRequest(ctx *gin.Context, channel *models.Channel) (*CustomerSessionVerifyResult, error) {
	token := s.getCustomerSessionToken(ctx)
	if token == "" {
		return nil, errorsx.UnauthorizedI18n("error.e0157")
	}
	claims, err := s.verifyToken(token)
	if err != nil {
		return nil, err
	}
	customer := CustomerService.Get(claims.CustomerID)
	if customer == nil || customer.Status == enums.StatusDeleted {
		return nil, errorsx.UnauthorizedI18n("error.e0161")
	}
	external, err := s.externalUserFromClaims(claims, customer)
	if err != nil {
		return nil, err
	}
	result := &CustomerSessionVerifyResult{
		ExternalUser:   external,
		ChannelID:      claims.ChannelID,
		EntrySessionID: claims.EntrySessionID,
		TenantID:       claims.TenantID,
		ProductID:      claims.ProductID,
		Token:          token,
		ExpiresAt:      claims.ExpiresAt.Time,
	}
	var refresh func() (string, time.Time, error)
	if claims.EntrySessionID > 0 {
		session := repositories.CustomerEntrySessionRepository.FindActive(sqls.DB(), claims.EntrySessionID, time.Now())
		if session == nil || session.TenantID != claims.TenantID || customerEntrySessionProductID(session) != claims.ProductID || entrySessionExternalID(session.ID, session.VisitorID) != external.ExternalID {
			return nil, errorsx.UnauthorizedI18n("error.e0161")
		}
		if err := CustomerEntryService.RequirePrivacyConsent(session); err != nil {
			return nil, err
		}
		refresh = func() (string, time.Time, error) {
			return s.signEntrySession(session, customer, *external)
		}
	} else {
		if channel == nil || channel.Status != enums.StatusOk {
			return nil, errorsx.InvalidParamI18n("error.e0209")
		}
		if claims.ChannelID != channel.ID || strings.TrimSpace(claims.ChannelCode) != strings.TrimSpace(channel.ChannelID) {
			return nil, errorsx.UnauthorizedI18n("error.e0161")
		}
		refresh = func() (string, time.Time, error) {
			return s.Sign(channel, customer, *external)
		}
	}
	if s.shouldRefresh(claims.ExpiresAt.Time) {
		newToken, expiresAt, err := refresh()
		if err != nil {
			return nil, err
		}
		result.Token = newToken
		result.ExpiresAt = expiresAt
		result.Refreshed = true
	}
	return result, nil
}

func entrySessionExternalID(entrySessionID int64, visitorID string) string {
	return "entry_" + strconv.FormatInt(entrySessionID, 10) + "_" + hashCustomerEntrySecret(visitorID)
}

func (s *customerSessionService) SetRefreshHeaders(ctx *gin.Context, result *CustomerSessionVerifyResult) {
	if ctx == nil || result == nil || !result.Refreshed {
		return
	}
	ctx.Header(customerSessionHeader, result.Token)
	ctx.Header(customerSessionExpHeader, result.ExpiresAt.Format(time.DateTime))
}

func (s *customerSessionService) verifyToken(rawToken string) (*customerSessionClaims, error) {
	cfg := config.Current().CustomerSession
	secret := strings.TrimSpace(cfg.Secret)
	if secret == "" {
		return nil, errorsx.BusinessErrorI18n(1, "error.customerSession.secretMissing")
	}
	claims := &customerSessionClaims{}
	token, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unsupported signing method")
		}
		return []byte(secret), nil
	}, jwt.WithExpirationRequired(), jwt.WithValidMethods([]string{
		jwt.SigningMethodHS256.Alg(),
		jwt.SigningMethodHS384.Alg(),
		jwt.SigningMethodHS512.Alg(),
	}))
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errorsx.UnauthorizedI18n("error.e0160")
		}
		return nil, errorsx.UnauthorizedI18n("error.e0161")
	}
	if token == nil || !token.Valid || claims.TokenType != customerSessionTokenType || claims.ExpiresAt == nil {
		return nil, errorsx.UnauthorizedI18n("error.e0161")
	}
	channelScopeValid := claims.ChannelID > 0 && strings.TrimSpace(claims.ChannelCode) != ""
	entryScopeValid := claims.EntrySessionID > 0 && claims.TenantID > 0 && claims.ProductID > 0
	if (!channelScopeValid && !entryScopeValid) || claims.CustomerID <= 0 || strings.TrimSpace(claims.IdentityKey) == "" {
		return nil, errorsx.UnauthorizedI18n("error.e0161")
	}
	return claims, nil
}

func customerEntrySessionProductID(session *models.CustomerEntrySession) int64 {
	if session == nil {
		return 0
	}
	if session.ProductID > 0 {
		return session.ProductID
	}
	var entryContext struct {
		ProductID int64 `json:"productId"`
	}
	if json.Unmarshal([]byte(session.EntryContextJSON), &entryContext) == nil {
		return entryContext.ProductID
	}
	return 0
}

func (s *customerSessionService) externalUserFromClaims(claims *customerSessionClaims, customer *models.Customer) (*openidentity.ExternalUser, error) {
	identityKey := strings.TrimSpace(claims.IdentityKey)
	parts := strings.SplitN(identityKey, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return nil, errorsx.UnauthorizedI18n("error.e0161")
	}
	var source enums.ExternalSource
	switch parts[0] {
	case "user":
		source = enums.ExternalSourceUser
	default:
		return nil, errorsx.UnauthorizedI18n("error.e0161")
	}
	identity := repositories.CustomerIdentityRepository.GetBy(sqls.DB(), source, parts[1])
	if identity == nil || identity.CustomerID != claims.CustomerID {
		return nil, errorsx.UnauthorizedI18n("error.e0161")
	}
	name := strings.TrimSpace(claims.CustomerName)
	if customer != nil && strings.TrimSpace(customer.Name) != "" {
		name = strings.TrimSpace(customer.Name)
	}
	return &openidentity.ExternalUser{
		ExternalSource: source,
		ExternalID:     parts[1],
		ExternalName:   name,
	}, nil
}

func (s *customerSessionService) shouldRefresh(expiresAt time.Time) bool {
	threshold := config.Current().CustomerSession.RefreshThreshold()
	return time.Until(expiresAt) <= time.Duration(threshold)*time.Minute
}

func (s *customerSessionService) identityKey(externalUser openidentity.ExternalUser) string {
	switch externalUser.ExternalSource {
	case enums.ExternalSourceUser:
		return "user:" + strings.TrimSpace(externalUser.ExternalID)
	default:
		return "guest:" + strings.TrimSpace(externalUser.ExternalID)
	}
}

func (s *customerSessionService) getCustomerSessionToken(ctx *gin.Context) string {
	auth := strings.TrimSpace(ctx.GetHeader("Authorization"))
	if len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
		if token := strings.TrimSpace(auth[7:]); token != "" {
			return token
		}
	}
	if token := webSocketProtocolCredential(ctx, webSocketCustomerSessionProtocolPrefix); token != "" {
		return token
	}
	token, _ := params.Get(ctx, "customerSessionToken")
	return strings.TrimSpace(token)
}
