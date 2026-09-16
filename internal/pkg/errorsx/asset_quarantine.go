package errorsx

import "remotehelpdesk/internal/pkg/i18nx"

type AssetQuarantinedError struct {
	AssetID int64
	Reason  string
}

func (e *AssetQuarantinedError) Error() string { return e.Message(i18nx.DefaultLocale) }
func (e *AssetQuarantinedError) Message(locale string) string {
	return i18nx.Getf(locale, "error.upload.quarantined", e.AssetID, i18nx.Getf(locale, "error.upload.reason."+e.Reason))
}
