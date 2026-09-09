package dto

type EnterpriseDeviceCreateRequest struct {
	ServiceCode         string `json:"service_code"`
	ProductID           int64  `json:"product_id"`
	ModelID             int64  `json:"model_id"`
	RegionCode          string `json:"region_code"`
	InstallDate         string `json:"install_date"`
	WarrantyEnd         string `json:"warranty_end"`
	Description         string `json:"description"`
	ExternalDeviceID    string `json:"external_device_id"`
	InstallLocationJSON string `json:"install_location_json"`
	MetadataJSON        string `json:"metadata_json"`
	Source              string `json:"source"`
}

type EnterpriseDeviceUpdateRequest struct {
	ProductID           int64   `json:"product_id"`
	ModelID             int64   `json:"model_id"`
	RegionCode          string  `json:"region_code"`
	InstallDate         *string `json:"install_date"`
	WarrantyEnd         *string `json:"warranty_end"`
	Description         string  `json:"description"`
	ExternalDeviceID    string  `json:"external_device_id"`
	InstallLocationJSON string  `json:"install_location_json"`
	MetadataJSON        string  `json:"metadata_json"`
	Source              string  `json:"source"`
}

type EnterpriseDeviceBatchCreateRequest struct {
	Devices []EnterpriseDeviceCreateRequest `json:"devices"`
}

type EnterpriseDeviceBatchImportError struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
}

type EnterpriseDeviceBatchImportResult struct {
	Total     int                                `json:"total"`
	Succeeded int                                `json:"succeeded"`
	Failed    int                                `json:"failed"`
	Errors    []EnterpriseDeviceBatchImportError `json:"errors,omitempty"`
	Created   []EnterpriseDeviceListItemDTO      `json:"created,omitempty"`
}

type EnterpriseDeviceListItemDTO struct {
	ID                 int64  `json:"id"`
	DeviceNo           string `json:"device_no"`
	SerialNo           string `json:"serial_no"`
	ProductID          int64  `json:"product_id"`
	ProductName        string `json:"product_name"`
	ProductCode        string `json:"product_code"`
	ModelID            int64  `json:"model_id"`
	ModelName          string `json:"model_name"`
	CustomerOrgID      int64  `json:"customer_org_id"`
	CustomerUserID     int64  `json:"customer_user_id"`
	CustomerName       string `json:"customer_name"`
	CustomerOrg        string `json:"customer_org"`
	RegionCode         string `json:"region_code"`
	InstallDate        string `json:"install_date"`
	Status             string `json:"status"`
	DeviceStatus       string `json:"device_status"`
	WarrantyEnd        string `json:"warranty_end"`
	WarrantyStatus     string `json:"warranty_status"`
	ServiceCode        string `json:"service_code"`
	EntryURL           string `json:"entry_url"`
	QRURL              string `json:"qr_url"`
	QRImageURL         string `json:"qr_image_url"`
	LastServiceAt      string `json:"last_service_at"`
	OpenTicketCount    int64  `json:"open_ticket_count"`
	TotalTicketCount   int64  `json:"total_ticket_count"`
	MeetingCount       int64  `json:"meeting_count"`
	ActiveMeetingCount int64  `json:"active_meeting_count"`
	BindingCount       int64  `json:"binding_count"`
	UpdatedAt          string `json:"updated_at"`
}

type EnterpriseDeviceDetailDTO struct {
	EnterpriseDeviceListItemDTO
	InstallDate      string                         `json:"install_date"`
	InstallLocation  string                         `json:"install_location"`
	Source           string                         `json:"source"`
	StatusReason     string                         `json:"status_reason"`
	Description      string                         `json:"description"`
	SoftwareVersions []EnterpriseSoftwareVersionDTO `json:"software_versions"`
	RepairHistory    []EnterpriseRepairHistoryDTO   `json:"repair_history"`
	RecentTickets    []EnterpriseTicketListItemDTO  `json:"recent_tickets"`
	CustomerBindings []EnterpriseCustomerBindingDTO `json:"customer_bindings"`
	CustomerVisible  bool                           `json:"customer_visible"`
	CanCreateTicket  bool                           `json:"can_create_ticket"`
	CanStartMeeting  bool                           `json:"can_start_meeting"`
}

type EnterpriseSoftwareVersionDTO struct {
	ID            int64  `json:"id"`
	Component     string `json:"component"`
	ComponentType string `json:"component_type"`
	Version       string `json:"version"`
	InstalledAt   string `json:"installed_at"`
	Source        string `json:"source"`
}

type EnterpriseRepairHistoryDTO struct {
	ID                int64  `json:"id"`
	TicketID          int64  `json:"ticket_id"`
	TicketNo          string `json:"ticket_no"`
	FaultType         string `json:"fault_type"`
	Resolution        string `json:"resolution"`
	RepairMethod      string `json:"repair_method"`
	TechnicianName    string `json:"technician_name"`
	CompletedAt       string `json:"completed_at"`
	VisibleToCustomer bool   `json:"visible_to_customer"`
}

type EnterpriseCustomerBindingDTO struct {
	ID             int64  `json:"id"`
	CustomerOrgID  int64  `json:"customer_org_id"`
	CustomerUserID int64  `json:"customer_user_id"`
	CustomerName   string `json:"customer_name"`
	BindingRole    string `json:"binding_role"`
	Source         string `json:"source"`
	Status         string `json:"status"`
	ConfirmedAt    string `json:"confirmed_at"`
}
