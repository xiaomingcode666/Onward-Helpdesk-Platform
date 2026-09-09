package i18nx

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	LocaleZhCN = "zh-CN"
	LocaleEnUS = "en-US"
	LocaleEsES = "es-ES"
)

var supportedLocales = map[string]string{
	"zh":      LocaleZhCN,
	"zh-cn":   LocaleZhCN,
	"zh_cn":   LocaleZhCN,
	"zh-hans": LocaleZhCN,
	"en":      LocaleEnUS,
	"en-us":   LocaleEnUS,
	"en_us":   LocaleEnUS,
	"es":      LocaleEsES,
	"es-es":   LocaleEsES,
	"es_es":   LocaleEsES,
	"es-mx":   LocaleEsES,
	"es_mx":   LocaleEsES,
}

var DefaultLocale = LocaleZhCN

func SetDefaultLocale(locale string) {
	DefaultLocale = NormalizeLocale(locale)
}

func NormalizeLocale(value string) string {
	key := strings.ToLower(strings.TrimSpace(value))
	if key == "" {
		return DefaultLocale
	}
	if locale, ok := supportedLocales[key]; ok {
		return locale
	}
	return DefaultLocale
}

func ResolveRequestLocale(_ *http.Request) string {
	return DefaultLocale
}

func Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		SetLocale(ctx, ResolveRequestLocale(ctx.Request))
		ctx.Next()
	}
}
