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

// LocaleHeader 由前端按当前界面语言显式设置。浏览器自动发送的 Accept-Language
// 反映操作系统语言而非用户在系统内的选择，因此不参与解析。
const LocaleHeader = "X-Locale"

func ResolveRequestLocale(req *http.Request) string {
	if req == nil {
		return DefaultLocale
	}
	return NormalizeLocale(req.Header.Get(LocaleHeader))
}

func Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		SetLocale(ctx, ResolveRequestLocale(ctx.Request))
		ctx.Next()
	}
}
