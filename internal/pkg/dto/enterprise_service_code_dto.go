package dto

type EnterpriseServiceCodeListItemDTO struct {
	ID          int64   `json:"id"`
	ServiceCode string  `json:"service_code"`
	EntryURL    string  `json:"entry_url"`
	QRURL       string  `json:"qr_url"`
	QRImageURL  string  `json:"qr_image_url"`
	BatchNo     string  `json:"batch_no"`
	Mode        string  `json:"mode"`
	ProductName string  `json:"product_name"`
	BoundDevice *string `json:"bound_device"`
	Status      string  `json:"status"`
	ActivatedAt *string `json:"activated_at"`
	CreatedAt   string  `json:"created_at"`
}

type EnterpriseServiceCodeBatchListItemDTO struct {
	ID          int64  `json:"id"`
	BatchNo     string `json:"batch_no"`
	ProductName string `json:"product_name"`
	Mode        string `json:"mode"`
	Quantity    int64  `json:"quantity"`
	Generated   int64  `json:"generated"`
	Exported    int64  `json:"exported"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}
