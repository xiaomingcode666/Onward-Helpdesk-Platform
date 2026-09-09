package api

import (
	"net/http"
	"net/url"
	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/services"
	"strings"

	"github.com/gin-gonic/gin"
)

func Login(ctx *gin.Context) {
	cfg := config.Current()
	req := request.LoginRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	ret, err := services.AuthService.Login(req, cfg.Auth, ctx.ClientIP(), ctx.GetHeader("User-Agent"))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, ret)
}

func CustomerRegistrationVerify(ctx *gin.Context) {
	req := request.CustomerRegistrationVerifyRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	registration, err := services.CustomerRegistrationService.Verify(req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildCustomerRegistrationContext(registration))
}

func CustomerRegistrationRegister(ctx *gin.Context) {
	req := request.CustomerRegistrationRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	login, err := services.CustomerRegistrationService.Register(req, config.Current().Auth, ctx.ClientIP(), ctx.GetHeader("User-Agent"))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, login)
}

func PortalInvitationVerify(ctx *gin.Context) {
	req := request.PortalInvitationVerifyRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	registration, err := services.CustomerRegistrationService.VerifyPortalInvitation(req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildCustomerRegistrationContext(registration))
}

func PortalInvitationRegister(ctx *gin.Context) {
	req := request.PortalInvitationRegisterRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	login, err := services.CustomerRegistrationService.RegisterPortalInvitation(req, config.Current().Auth, ctx.ClientIP(), ctx.GetHeader("User-Agent"))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, login)
}

func PortalInvitationBind(ctx *gin.Context) {
	req := request.PortalInvitationBindRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	login, err := services.CustomerRegistrationService.BindPortalInvitation(req, config.Current().Auth, ctx.ClientIP(), ctx.GetHeader("User-Agent"))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, login)
}

func AuthVerificationCodeSend(ctx *gin.Context) {
	req := request.AuthVerificationCodeRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.AuthVerificationService.Send(req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func EnterpriseRegistrationRegister(ctx *gin.Context) {
	req := request.EnterpriseRegistrationRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	login, err := services.PublicAuthService.RegisterEnterprise(req, config.Current().Auth, ctx.ClientIP(), ctx.GetHeader("User-Agent"))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, login)
}

func PasswordReset(ctx *gin.Context) {
	req := request.PasswordResetRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.PublicAuthService.ResetPassword(req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func PublicConfig(ctx *gin.Context) {
	cfg := config.Current()
	httpx.WriteJSON(ctx, &response.PublicConfigResponse{
		Language:      cfg.LanguageOrDefault(),
		WxWorkEnabled: cfg.WxWork.Enabled,
		OIDCEnabled:   cfg.OIDC.Enabled,
	})
}

func WxWorkLogin(ctx *gin.Context) {
	loginURL, err := services.WxWorkLoginService.BuildWxWorkLoginURL(ctx.Query("next"))
	if err != nil {
		ctx.Redirect(http.StatusFound, "/login?wxworkError="+url.QueryEscape(wxWorkErrorMessage(err.Error())))
		return
	}
	ctx.Redirect(http.StatusFound, loginURL)
}

func WxWorkQRLogin(ctx *gin.Context) {
	loginURL, err := services.WxWorkLoginService.BuildWxWorkQRCodeLoginURL(ctx.Query("next"))
	if err != nil {
		ctx.Redirect(http.StatusFound, "/login?wxworkError="+url.QueryEscape(wxWorkErrorMessage(err.Error())))
		return
	}
	ctx.Redirect(http.StatusFound, loginURL)
}

func WxWorkCallback(ctx *gin.Context) {
	cfg := config.Current()
	ticket, next, err := services.WxWorkLoginService.LoginByWxWork(
		ctx.Query("code"),
		ctx.Query("state"),
		cfg.Auth,
		ctx.ClientIP(),
		ctx.GetHeader("User-Agent"),
	)
	if err != nil {
		ctx.Redirect(http.StatusFound, "/login?wxworkError="+url.QueryEscape(wxWorkErrorMessage(err.Error())))
		return
	}
	ctx.Redirect(http.StatusFound, "/dashboard/login/wxwork/callback?ticket="+url.QueryEscape(ticket)+"&next="+url.QueryEscape(next))
}

func WxWorkExchange(ctx *gin.Context) {
	req := request.WxWorkExchangeRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	ret, err := services.WxWorkLoginService.ExchangeWxWorkLoginTicket(req.Ticket)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, ret)
}

func OIDCLogin(ctx *gin.Context) {
	loginURL, err := services.OIDCLoginService.BuildOIDCLoginURL(ctx.Query("next"))
	if err != nil {
		ctx.Redirect(http.StatusFound, "/dashboard/login?oidcError="+url.QueryEscape(loginErrorMessage(err.Error())))
		return
	}
	ctx.Redirect(http.StatusFound, loginURL)
}

func OIDCCallback(ctx *gin.Context) {
	cfg := config.Current()
	ticket, next, err := services.OIDCLoginService.LoginByOIDC(
		ctx.Request.Context(),
		ctx.Query("code"),
		ctx.Query("state"),
		cfg.Auth,
		ctx.ClientIP(),
		ctx.GetHeader("User-Agent"),
	)
	if err != nil {
		ctx.Redirect(http.StatusFound, "/dashboard/login?oidcError="+url.QueryEscape(loginErrorMessage(err.Error())))
		return
	}
	ctx.Redirect(http.StatusFound, "/dashboard/login/oidc/callback?ticket="+url.QueryEscape(ticket)+"&next="+url.QueryEscape(next))
}

func OIDCExchange(ctx *gin.Context) {
	req := request.OIDCExchangeRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	ret, err := services.OIDCLoginService.ExchangeOIDCLoginTicket(req.Ticket)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, ret)
}

func Logout(ctx *gin.Context) {
	if err := services.AuthService.Logout(ctx.GetHeader("Authorization")); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func Profile(ctx *gin.Context) {
	ret, err := services.AuthService.CurrentProfile(ctx)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, ret)
}

func UpdateProfile(ctx *gin.Context) {
	req := request.UpdateOwnProfileRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	principal, err := services.AuthService.Authenticate(ctx)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if _, err := services.UserService.UpdateOwnProfile(req, principal); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	ret, err := services.AuthService.CurrentProfile(ctx)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, ret)
}

func wxWorkErrorMessage(message string) string {
	return loginErrorMessage(message)
}

func loginErrorMessage(message string) string {
	if idx := strings.Index(message, ": "); idx >= 0 {
		message = message[idx+2:]
	}
	return message
}
