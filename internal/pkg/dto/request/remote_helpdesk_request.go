package request

type CreateProductRequest struct {
	TenantID      int64 `json:"tenantId"`
	ProductLineID int64 `json:"productLineId"`
	// ProductLine 产品线名称。ProductLineID 为空时，Service 在当前租户内按名称解析，
	// 不存在则创建对应产品线主数据，避免产品线信息被静默丢弃。
	ProductLine   string `json:"productLine"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Category      string `json:"category"`
	OwnerMemberID int64  `json:"ownerMemberId"`
	DefaultLocale string `json:"defaultLocale"`
}

type UpdateProductRequest struct {
	ID int64 `json:"id"`
	CreateProductRequest
	// StatusText 可选状态文本（"active" | "inactive"），非空时同步更新产品状态。
	StatusText string `json:"statusText"`
	// DescriptionProvided 为 PATCH 语义标记：true 时 description 按请求值写入（允许置空），
	// false 时保持原值，避免未携带描述字段的调用方清空已有描述。
	DescriptionProvided bool `json:"-"`
}

type DeleteProductRequest struct {
	ID int64 `json:"id"`
}

type UpdateProductStatusRequest struct {
	ID     int64 `json:"id"`
	Status int   `json:"status"`
}

type CreateProductModelRequest struct {
	TenantID        int64  `json:"tenantId"`
	ProductID       int64  `json:"productId"`
	ModelCode       string `json:"modelCode"`
	Name            string `json:"name"`
	VersionPolicy   string `json:"versionPolicy"`
	RegionScopeJSON string `json:"regionScopeJson"`
}

type UpdateProductModelRequest struct {
	ID int64 `json:"id"`
	CreateProductModelRequest
}

type DeleteProductModelRequest struct {
	ID int64 `json:"id"`
}

type UpdateProductModelStatusRequest struct {
	ID     int64 `json:"id"`
	Status int   `json:"status"`
}

type CreateDeviceRequest struct {
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
	MetadataJSON        string `json:"metadataJson"`
}

type UpdateDeviceRequest struct {
	ID int64 `json:"id"`
	CreateDeviceRequest
}

type DeleteDeviceRequest struct {
	ID int64 `json:"id"`
}

type UpdateDeviceStatusRequest struct {
	ID     int64 `json:"id"`
	Status int   `json:"status"`
}

type TransferDeviceOwnershipRequest struct {
	DeviceID               int64  `json:"deviceId"`
	TargetCustomerOrgID    int64  `json:"targetCustomerOrgId"`
	TargetCustomerUserID   int64  `json:"targetCustomerUserId"`
	TransferReason         string `json:"transferReason"`
	PreserveServiceHistory bool   `json:"preserveServiceHistory"`
	NotifyCurrentOwner     bool   `json:"notifyCurrentOwner"`
	NotifyNewOwner         bool   `json:"notifyNewOwner"`
}

type BatchTransferDeviceRequest struct {
	TenantID               int64   `json:"tenantId"`
	DeviceIDs              []int64 `json:"deviceIds"`
	TargetCustomerOrgID    int64   `json:"targetCustomerOrgId"`
	TargetCustomerUserID   int64   `json:"targetCustomerUserId"`
	TransferReason         string  `json:"transferReason"`
	PreserveServiceHistory bool    `json:"preserveServiceHistory"`
	NotifyCurrentOwner     bool    `json:"notifyCurrentOwner"`
	NotifyNewOwner         bool    `json:"notifyNewOwner"`
}

type BatchUpdateDeviceWarrantyRequest struct {
	TenantID       int64   `json:"tenantId"`
	DeviceIDs      []int64 `json:"deviceIds"`
	WarrantyEndAt  string  `json:"warrantyEndAt"`
	WarrantyPolicy string  `json:"warrantyPolicy"`
}
