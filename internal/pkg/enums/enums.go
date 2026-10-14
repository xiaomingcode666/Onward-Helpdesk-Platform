package enums

type Status int

const (
	StatusOk       Status = 0
	StatusDisabled Status = 1
	StatusDeleted  Status = 2
)

var StatusValues = []Status{StatusOk, StatusDisabled, StatusDeleted}

var statusLabelMap = map[Status]string{
	StatusOk:       "enum.StatusOk",
	StatusDisabled: "enum.StatusDisabled",
	StatusDeleted:  "enum.StatusDeleted",
}

func GetStatusLabel(status Status, locale string) string {
	return enumLabel(locale, statusLabelMap[status])
}

func IsValidStatus(status int) bool {
	for _, s := range StatusValues {
		if int(s) == status {
			return true
		}
	}
	return false
}

type Gender int

const (
	GenderUnknown Gender = 0
	GenderMale    Gender = 1
	GenderFemale  Gender = 2
)

var GenderValues = []Gender{GenderUnknown, GenderMale, GenderFemale}

var genderLabelMap = map[Gender]string{
	GenderUnknown: "enum.GenderUnknown",
	GenderMale:    "enum.GenderMale",
	GenderFemale:  "enum.GenderFemale",
}

func GetGenderLabel(gender Gender, locale string) string {
	return enumLabel(locale, genderLabelMap[gender])
}

func IsValidGender(gender int) bool {
	for _, g := range GenderValues {
		if int(g) == gender {
			return true
		}
	}
	return false
}

type ThirdProvider string

const (
	ThirdProviderWxWork   ThirdProvider = "wxwork"
	ThirdProviderDingtalk ThirdProvider = "dingtalk"
	ThirdProviderOIDC     ThirdProvider = "oidc"
)

var ThirdProviderValues = []ThirdProvider{ThirdProviderWxWork, ThirdProviderDingtalk, ThirdProviderOIDC}

var thirdProviderLabelMap = map[ThirdProvider]string{
	ThirdProviderWxWork:   "enum.ThirdProviderWxWork",
	ThirdProviderDingtalk: "enum.ThirdProviderDingtalk",
	ThirdProviderOIDC:     "enum.ThirdProviderOIDC",
}

func GetThirdProviderLabel(provider ThirdProvider, locale string) string {
	return enumLabel(locale, thirdProviderLabelMap[provider])
}

func IsValidThirdProvider(provider string) bool {
	for _, p := range ThirdProviderValues {
		if string(p) == provider {
			return true
		}
	}
	return false
}
