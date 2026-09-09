package request

type CreateServiceCodeBatchRequest struct {
	TenantID        int64  `json:"tenantId"`
	BatchNo         string `json:"batchNo"`
	Mode            string `json:"mode"`
	ProductID       int64  `json:"productId"`
	ProductModelID  int64  `json:"productModelId"`
	Quantity        int64  `json:"quantity"`
	LabelTemplateID int64  `json:"labelTemplateId"`
}

type UpdateServiceCodeBatchStatusRequest struct {
	ID     int64 `json:"id"`
	Status int   `json:"status"`
}

type DeleteServiceCodeBatchRequest struct {
	ID int64 `json:"id"`
}

type GenerateServiceCodesRequest struct {
	BatchID int64 `json:"batchId"`
	Count   int64 `json:"count"`
}

type RevokeServiceCodeRequest struct {
	ID int64 `json:"id"`
}
