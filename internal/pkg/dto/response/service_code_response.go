package response

type ServiceCodeBatchResponse struct {
	ID              int64  `json:"id"`
	TenantID        int64  `json:"tenantId"`
	BatchNo         string `json:"batchNo"`
	Mode            string `json:"mode"`
	ProductID       int64  `json:"productId"`
	ProductModelID  int64  `json:"productModelId"`
	Quantity        int64  `json:"quantity"`
	LabelTemplateID int64  `json:"labelTemplateId"`
	Status          int    `json:"status"`
	GeneratedCount  int64  `json:"generatedCount"`
	ExportedCount   int64  `json:"exportedCount"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

type ServiceCodeResponse struct {
	ID             int64  `json:"id"`
	TenantID       int64  `json:"tenantId"`
	BatchID        int64  `json:"batchId"`
	ServiceCode    string `json:"serviceCode"`
	EntryURL       string `json:"entryUrl"`
	QRURL          string `json:"qrUrl"`
	QRImageURL     string `json:"qrImageUrl"`
	Mode           string `json:"mode"`
	DeviceID       int64  `json:"deviceId"`
	ProductID      int64  `json:"productId"`
	ProductModelID int64  `json:"productModelId"`
	Status         string `json:"status"`
	ActivatedAt    string `json:"activatedAt"`
	RevokedAt      string `json:"revokedAt"`
	MetadataJSON   string `json:"metadataJson"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}
