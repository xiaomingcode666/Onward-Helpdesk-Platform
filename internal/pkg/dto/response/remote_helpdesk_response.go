package response

type CustomerServiceCodeResolveResponse struct {
	Valid        bool                          `json:"valid"`
	EntryState   string                        `json:"entryState"`
	Reason       string                        `json:"reason,omitempty"`
	ServiceCode  *ResolvedServiceCodeResponse  `json:"serviceCode,omitempty"`
	Tenant       *ResolvedTenantResponse       `json:"tenant,omitempty"`
	Product      *ResolvedProductResponse      `json:"product,omitempty"`
	ProductModel *ResolvedProductModelResponse `json:"productModel,omitempty"`
	Device       *ResolvedDeviceResponse       `json:"device,omitempty"`
}

type ResolvedServiceCodeResponse struct {
	ID          int64  `json:"id"`
	ServiceCode string `json:"serviceCode"`
	EntryURL    string `json:"entryUrl"`
	QRURL       string `json:"qrUrl"`
	QRImageURL  string `json:"qrImageUrl"`
	Mode        string `json:"mode"`
	Status      string `json:"status"`
}

type ResolvedTenantResponse struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	DefaultLocale string `json:"defaultLocale"`
	Timezone      string `json:"timezone"`
}

type ResolvedProductResponse struct {
	ID       int64  `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

type ResolvedProductModelResponse struct {
	ID        int64  `json:"id"`
	ModelCode string `json:"modelCode"`
	Name      string `json:"name"`
}

type ResolvedDeviceResponse struct {
	ID         int64  `json:"id"`
	DeviceNo   string `json:"deviceNo"`
	SerialNo   string `json:"serialNo"`
	RegionCode string `json:"regionCode"`
}

type ProductResponse struct {
	ID            int64  `json:"id"`
	TenantID      int64  `json:"tenantId"`
	ProductLineID int64  `json:"productLineId"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	Category      string `json:"category"`
	OwnerMemberID int64  `json:"ownerMemberId"`
	DefaultLocale string `json:"defaultLocale"`
	Status        int    `json:"status"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

type ProductModelResponse struct {
	ID              int64  `json:"id"`
	TenantID        int64  `json:"tenantId"`
	ProductID       int64  `json:"productId"`
	ModelCode       string `json:"modelCode"`
	Name            string `json:"name"`
	VersionPolicy   string `json:"versionPolicy"`
	RegionScopeJSON string `json:"regionScopeJson"`
	Status          int    `json:"status"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

type DeviceResponse struct {
	ID                  int64  `json:"id"`
	TenantID            int64  `json:"tenantId"`
	DeviceNo            string `json:"deviceNo"`
	ProductID           int64  `json:"productId"`
	ProductModelID      int64  `json:"productModelId"`
	SerialNo            string `json:"serialNo"`
	CustomerOrgID       int64  `json:"customerOrgId"`
	ExternalDeviceID    string `json:"externalDeviceId"`
	InstallLocationJSON string `json:"installLocationJson"`
	RegionCode          string `json:"regionCode"`
	Source              string `json:"source"`
	Status              int    `json:"status"`
	MetadataJSON        string `json:"metadataJson"`
	CreatedAt           string `json:"createdAt"`
	UpdatedAt           string `json:"updatedAt"`
}
