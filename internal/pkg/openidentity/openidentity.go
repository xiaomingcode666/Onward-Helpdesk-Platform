package openidentity

import (
	"errors"
	"strings"

	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/mlogclub/simple/common/strs"
)

// ExternalUser 外部客户身份（IM 客户），与站内 AuthPrincipal 区分。
type ExternalUser struct {
	ExternalSource enums.ExternalSource `json:"externalSource"`
	ExternalID     string               `json:"externalId"`
	ExternalName   string               `json:"externalName"`
}

type UserTokenClaims struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
	jwt.RegisteredClaims
}

func GetExternalUser(ctx *gin.Context, secret string) (*ExternalUser, error) {
	claims, err := verifyUserToken(getUserToken(ctx), secret)
	if err != nil {
		return nil, err
	}
	return &ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     claims.UserID,
		ExternalName:   claims.Name,
	}, nil
}

func verifyUserToken(userToken, secret string) (*UserTokenClaims, error) {
	if strs.IsBlank(userToken) {
		return nil, errorsx.UnauthorizedI18n("error.e0263")
	}
	if strs.IsBlank(secret) {
		return nil, errorsx.UnauthorizedI18n("error.e0266")
	}

	claims := &UserTokenClaims{}
	token, err := jwt.ParseWithClaims(userToken, claims, func(token *jwt.Token) (any, error) {
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
			return nil, errorsx.UnauthorizedI18n("error.e0264")
		}
		return nil, errorsx.UnauthorizedI18n("error.e0265")
	}
	if token == nil || !token.Valid {
		return nil, errorsx.UnauthorizedI18n("error.e0265")
	}

	if strs.IsBlank(claims.UserID) {
		return nil, errorsx.UnauthorizedI18n("error.e0262")
	}
	if strs.IsBlank(claims.Name) {
		return nil, errorsx.UnauthorizedI18n("error.e0261")
	}
	if claims.ExpiresAt == nil {
		return nil, errorsx.UnauthorizedI18n("error.e0264")
	}

	return claims, nil
}

func getUserToken(ctx *gin.Context) string {
	auth := strings.TrimSpace(ctx.GetHeader("Authorization"))
	if len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
		if token := strings.TrimSpace(auth[7:]); token != "" {
			return token
		}
	}
	userToken, _ := params.Get(ctx, "userToken")
	return strings.TrimSpace(userToken)
}
