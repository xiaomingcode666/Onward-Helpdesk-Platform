package response

type CustomerEntrySummaryResponse struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	ModelCode string `json:"modelCode,omitempty"`
}

type CustomerEntryChatResponse struct {
	EntrySessionID int64  `json:"entrySessionId"`
	VisitorID      string `json:"visitorId"`
	VisitorToken   string `json:"visitorToken"`
	Locale         string `json:"locale"`
}

type CustomerEntrySessionResponse struct {
	EntrySessionID int64                          `json:"entrySessionId"`
	Tenant         *CustomerEntrySummaryResponse  `json:"tenant,omitempty"`
	Product        *CustomerEntrySummaryResponse  `json:"product,omitempty"`
	Model          *CustomerEntrySummaryResponse  `json:"model,omitempty"`
	Device         *CustomerEntrySummaryResponse  `json:"device,omitempty"`
	ServiceCode    string                         `json:"serviceCode,omitempty"`
	ExpiresAt      string                         `json:"expiresAt"`
	Chat           CustomerEntryChatResponse      `json:"chat"`
	PrivacyConsent CustomerPrivacyConsentResponse `json:"privacyConsent"`
	Session        map[string]any                 `json:"session"`
}

type CustomerPrivacyConsentResponse struct {
	Required      bool   `json:"required"`
	Accepted      bool   `json:"accepted"`
	Analytics     bool   `json:"analytics"`
	Marketing     bool   `json:"marketing"`
	PolicyVersion string `json:"policyVersion"`
	ConsentedAt   string `json:"consentedAt,omitempty"`
}

type CustomerDeviceBindingResponse struct {
	ID             int64  `json:"id"`
	EntrySessionID int64  `json:"entrySessionId"`
	TenantID       int64  `json:"tenantId"`
	DeviceID       int64  `json:"deviceId"`
	CustomerUserID int64  `json:"customerUserId"`
	CustomerOrgID  int64  `json:"customerOrgId"`
	BindingRole    string `json:"bindingRole"`
	Status         int    `json:"status"`
}

type CustomerPortalDeviceBindingResponse struct {
	DeviceID    int64  `json:"deviceId"`
	DeviceNo    string `json:"deviceNo,omitempty"`
	ProductName string `json:"productName,omitempty"`
	BindingRole string `json:"bindingRole,omitempty"`
	Status      int    `json:"status"`
}
