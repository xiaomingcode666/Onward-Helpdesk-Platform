package enums

type AssetStatus int

const (
	AssetStatusPending AssetStatus = 1
	AssetStatusSuccess AssetStatus = 2
	AssetStatusFailed  AssetStatus = 3
	AssetStatusDeleted AssetStatus = 4
)

var assetStatusLabelMap = map[AssetStatus]string{
	AssetStatusPending: "enum.AssetStatusPending",
	AssetStatusSuccess: "enum.AssetStatusSuccess",
	AssetStatusFailed:  "enum.AssetStatusFailed",
	AssetStatusDeleted: "enum.AssetStatusDeleted",
}

func GetAssetStatusLabel(status AssetStatus, locale string) string {
	return enumLabel(locale, assetStatusLabelMap[status])
}

type AssetProvider string

const (
	AssetProviderLocal AssetProvider = "local"
	AssetProviderOSS   AssetProvider = "oss"
	AssetProviderMinIO AssetProvider = "minio"
)

var assetProviderLabelMap = map[AssetProvider]string{
	AssetProviderLocal: "enum.AssetProviderLocal",
	AssetProviderOSS:   "enum.AssetProviderOSS",
	AssetProviderMinIO: "enum.AssetProviderMinIO",
}

func GetAssetProviderLabel(provider AssetProvider, locale string) string {
	return enumLabel(locale, assetProviderLabelMap[provider])
}
