package enums

// ExternalSource 外部身份来源。
//
// 与 ExternalID 组合即可唯一标识某渠道下的访客身份。
type ExternalSource string

const (
	ExternalSourceGuest    ExternalSource = "guest"     // 访客
	ExternalSourceWxWorkKF ExternalSource = "wxwork_kf" // 企业微信客服
	ExternalSourceWhatsApp ExternalSource = "whatsapp"  // WhatsApp Cloud API（含本地模拟入口）
	ExternalSourceUser     ExternalSource = "user"      // 用户信息
)

var externalSourceLabelMap = map[ExternalSource]string{
	ExternalSourceGuest:    "enum.ExternalSourceGuest",
	ExternalSourceWxWorkKF: "enum.ExternalSourceWxWorkKF",
	ExternalSourceWhatsApp: "enum.ExternalSourceWhatsApp",
	ExternalSourceUser:     "enum.ExternalSourceUser",
}

func GetExternalSourceLabel(v ExternalSource, locale string) string {
	if s, ok := externalSourceLabelMap[v]; ok {
		return enumLabel(locale, s)
	}
	return string(v)
}
