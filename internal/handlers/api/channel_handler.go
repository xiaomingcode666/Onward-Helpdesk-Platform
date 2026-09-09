package api

import (
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func ChannelAnyConfig(ctx *gin.Context) {
	channel := services.ChannelService.GetEnabledChannel(ctx)
	if channel == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0211"))
		return
	}
	cfg, err := resolveWidgetConfig(channel.ChannelType, channel.ConfigJSON)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	ret := response.WidgetConfigResponse{
		ChannelID:   channel.ChannelID,
		ChannelType: channel.ChannelType,
		Title:       cfg.Title,
		Subtitle:    cfg.Subtitle,
		ThemeColor:  cfg.ThemeColor,
		Position:    cfg.Position,
		Width:       cfg.Width,
	}
	httpx.WriteJSON(ctx, ret)
}

type webLikeWidgetConfig struct {
	Title      string
	Subtitle   string
	ThemeColor string
	Position   string
	Width      string
}

func resolveWidgetConfig(channelType, rawConfig string) (*webLikeWidgetConfig, error) {
	switch channelType {
	case enums.ChannelTypeWeb:
		cfg, err := services.ChannelService.ParseWebChannelConfig(rawConfig)
		if err != nil {
			return nil, err
		}
		return &webLikeWidgetConfig{
			Title:      cfg.Title,
			Subtitle:   cfg.Subtitle,
			ThemeColor: cfg.ThemeColor,
			Position:   cfg.Position,
			Width:      cfg.Width,
		}, nil
	case enums.ChannelTypeWechatMP:
		cfg, err := services.ChannelService.ParseWechatMPChannelConfig(rawConfig)
		if err != nil {
			return nil, err
		}
		return &webLikeWidgetConfig{
			Title:      cfg.Title,
			Subtitle:   cfg.Subtitle,
			ThemeColor: cfg.ThemeColor,
			Position:   "right",
			Width:      "100%",
		}, nil
	default:
		return nil, errorsx.InvalidParamI18n("error.e0313")
	}
}
