package dto

// MobilePushTokenDTO intentionally omits the provider token and its
// fingerprint. Those values are operational secrets, not client state.
type MobilePushTokenDTO struct {
	ID         int64  `json:"id"`
	Platform   string `json:"platform"`
	DeviceID   string `json:"deviceId,omitempty"`
	AppVersion string `json:"appVersion,omitempty"`
	LastSeenAt string `json:"lastSeenAt"`
}
