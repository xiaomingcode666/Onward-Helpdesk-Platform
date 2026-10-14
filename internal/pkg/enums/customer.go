package enums

const CustomerSourceTypeWebChat = ExternalSourceGuest

func GetCustomerSourceTypeLabel(sourceType ExternalSource, locale string) string {
	return GetExternalSourceLabel(ExternalSource(sourceType), locale)
}

// 联系方式类型
type ContactType string

const (
	ContactTypeMobile ContactType = "mobile"
	ContactTypeEmail  ContactType = "email"
	ContactTypeWeChat ContactType = "wechat"
	ContactTypeOther  ContactType = "other"
)

var contactTypeLabelMap = map[ContactType]string{
	ContactTypeMobile: "enum.ContactTypeMobile",
	ContactTypeEmail:  "enum.ContactTypeEmail",
	ContactTypeOther:  "enum.ContactTypeOther",
}

func GetContactTypeLabel(contactType ContactType, locale string) string {
	return enumLabel(locale, contactTypeLabelMap[contactType])
}

// IsValidContactType 判断字符串是否为合法联系方式类型。
func IsValidContactType(v string) bool {
	switch ContactType(v) {
	case ContactTypeMobile, ContactTypeEmail, ContactTypeWeChat, ContactTypeOther:
		return true
	default:
		return false
	}
}
