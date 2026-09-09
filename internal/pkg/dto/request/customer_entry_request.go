package request

type CreateCustomerEntrySessionRequest struct {
	ServiceCode    string `json:"serviceCode"`
	DeviceNo       string `json:"deviceNo"`
	SerialNo       string `json:"serialNo"`
	RegionCode     string `json:"regionCode"`
	VisitorID      string `json:"visitorId"`
	VisitorToken   string `json:"visitorToken"`
	Locale         string `json:"locale"`
	CustomerUserID int64  `json:"customerUserId"`
}

type ExchangeCustomerEntrySessionRequest struct {
	EntrySessionID int64  `json:"entrySessionId"`
	VisitorID      string `json:"visitorId"`
	VisitorToken   string `json:"visitorToken"`
}

type ConfirmCustomerPrivacyConsentRequest struct {
	EntrySessionID    int64  `json:"entrySessionId"`
	PolicyVersion     string `json:"policyVersion"`
	RequiredAccepted  bool   `json:"required"`
	AnalyticsAccepted bool   `json:"analytics"`
	MarketingAccepted bool   `json:"marketing"`
}

type ConfirmCustomerDeviceBindingRequest struct {
	EntrySessionID int64  `json:"entrySessionId"`
	VisitorID      string `json:"visitorId"`
	VisitorToken   string `json:"visitorToken"`
	CustomerUserID int64  `json:"customerUserId"`
	CustomerOrgID  int64  `json:"customerOrgId"`
	BindingRole    string `json:"bindingRole"`
}

type BindCustomerDeviceByServiceCodeRequest struct {
	ServiceCode string `json:"serviceCode" binding:"required"`
	DeviceNo    string `json:"deviceNo"`
	SerialNo    string `json:"serialNo"`
	RegionCode  string `json:"regionCode"`
}
