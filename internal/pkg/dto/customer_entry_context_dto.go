package dto

type CustomerEntryContextSummaryDTO struct {
	ID         int64  `json:"id"`
	Code       string `json:"code,omitempty"`
	Name       string `json:"name,omitempty"`
	ModelCode  string `json:"modelCode,omitempty"`
	DeviceNo   string `json:"deviceNo,omitempty"`
	SerialNo   string `json:"serialNo,omitempty"`
	RegionCode string `json:"regionCode,omitempty"`
	Status     string `json:"status,omitempty"`
}

type CustomerEntryDeviceDTO struct {
	ID            int64  `json:"id"`
	DeviceNo      string `json:"deviceNo"`
	SerialNo      string `json:"serialNo"`
	ProductName   string `json:"productName"`
	ProductCode   string `json:"productCode"`
	ModelName     string `json:"modelName"`
	RegionCode    string `json:"regionCode"`
	Status        string `json:"status"`
	LastServiceAt string `json:"lastServiceAt"`
}

type CustomerEntryTicketDTO struct {
	ID             int64                           `json:"id"`
	ConversationID int64                           `json:"conversationId"`
	TicketNo       string                          `json:"ticketNo"`
	Title          string                          `json:"title"`
	Status         string                          `json:"status"`
	Priority       string                          `json:"priority"`
	CreatedAt      string                          `json:"createdAt"`
	DeviceNo       string                          `json:"deviceNo"`
	RepairSummary  string                          `json:"repairSummary"`
	Feedback       *CustomerEntryTicketFeedbackDTO `json:"feedback,omitempty"`
	CanConfirm     bool                            `json:"canConfirm"`
	CanReopen      bool                            `json:"canReopen"`
	CanRate        bool                            `json:"canRate"`
}

type CustomerEntryTicketFeedbackDTO struct {
	ID          int64    `json:"id"`
	Rating      int      `json:"rating"`
	Tags        []string `json:"tags"`
	Comment     string   `json:"comment"`
	SubmittedAt string   `json:"submittedAt"`
}

type CustomerEntryConversationDTO struct {
	ID        int64  `json:"id"`
	Summary   string `json:"summary"`
	StartedAt string `json:"startedAt"`
	EndedAt   string `json:"endedAt,omitempty"`
	AgentType string `json:"agentType"`
	DeviceNo  string `json:"deviceNo"`
}

type CustomerEntryRepairHistoryDTO struct {
	ID          int64  `json:"id"`
	TicketID    int64  `json:"ticketId"`
	TicketNo    string `json:"ticketNo"`
	DeviceNo    string `json:"deviceNo"`
	ServiceType string `json:"serviceType"`
	Summary     string `json:"summary"`
	RootCause   string `json:"rootCause"`
	Solution    string `json:"solution"`
	OccurredAt  string `json:"occurredAt"`
}

type CustomerEntryKnowledgeDTO struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	Language    string `json:"language"`
	Version     string `json:"version"`
	Content     string `json:"content,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	UpdatedAt   string `json:"updatedAt"`
}

type CustomerEntryManualFileDTO struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	Filename   string `json:"filename"`
	FileSize   int64  `json:"fileSize"`
	MimeType   string `json:"mimeType"`
	URL        string `json:"url"`
	UploadedAt string `json:"uploadedAt"`
}

type CustomerEntryMeetingDTO struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	ScheduledAt string `json:"scheduledAt"`
	StartedAt   string `json:"startedAt"`
	EndedAt     string `json:"endedAt"`
	Initiator   string `json:"initiator"`
	TicketID    int64  `json:"ticketId"`
}

type CustomerEntryContextDTO struct {
	EntrySessionID   int64                           `json:"entrySessionId"`
	ServiceCode      string                          `json:"serviceCode,omitempty"`
	Tenant           *CustomerEntryContextSummaryDTO `json:"tenant,omitempty"`
	Product          *CustomerEntryContextSummaryDTO `json:"product,omitempty"`
	Model            *CustomerEntryContextSummaryDTO `json:"model,omitempty"`
	Device           *CustomerEntryContextSummaryDTO `json:"device,omitempty"`
	Devices          []CustomerEntryDeviceDTO        `json:"devices"`
	Tickets          []CustomerEntryTicketDTO        `json:"tickets"`
	Conversations    []CustomerEntryConversationDTO  `json:"conversations"`
	RepairHistory    []CustomerEntryRepairHistoryDTO `json:"repairHistory"`
	KnowledgeEntries []CustomerEntryKnowledgeDTO     `json:"knowledgeEntries"`
	ManualFiles      []CustomerEntryManualFileDTO    `json:"manualFiles"`
	Meetings         []CustomerEntryMeetingDTO       `json:"meetings"`
}
