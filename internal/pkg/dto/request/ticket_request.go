package request

import (
	"remotehelpdesk/internal/pkg/dto"
	"time"
)

type CreateTicketRequest struct {
	dto.TicketIntakeInput
	IdempotencyKey              string     `json:"idempotencyKey"`
	Title                       string     `json:"title"`
	Description                 string     `json:"description"`
	PriorityCode                string     `json:"priorityCode"`
	Source                      string     `json:"source"`
	Channel                     string     `json:"channel"`
	CustomerID                  int64      `json:"customerId"`
	CustomerRegistrationGrantID int64      `json:"customerRegistrationGrantId"`
	ConversationID              int64      `json:"conversationId"`
	TagIDs                      []int64    `json:"tagIds"`
	CurrentTeamID               int64      `json:"currentTeamId"`
	CurrentAssigneeID           int64      `json:"currentAssigneeId"`
	TenantID                    int64      `json:"tenantId"`
	ProductID                   int64      `json:"productId"`
	ProductModelID              int64      `json:"productModelId"`
	ProductModuleID             int64      `json:"productModuleId"`
	DeviceID                    int64      `json:"deviceId"`
	ServiceCodeID               int64      `json:"serviceCodeId"`
	CustomerEntrySessionID      int64      `json:"customerEntrySessionId"`
	ServiceRegion               string     `json:"serviceRegion"`
	FaultCode                   string     `json:"faultCode"`
	SymptomSummary              string     `json:"symptomSummary"`
	DiagnosisSummary            string     `json:"diagnosisSummary"`
	SLADueAt                    *time.Time `json:"slaDueAt"`
	ResolvedAt                  *time.Time `json:"resolvedAt"`
}

type CreateTicketFromConversationRequest struct {
	IdempotencyKey         string     `json:"idempotencyKey"`
	ConversationID         int64      `json:"conversationId"`
	Title                  string     `json:"title"`
	Description            string     `json:"description"`
	PriorityCode           string     `json:"priorityCode"`
	TagIDs                 []int64    `json:"tagIds"`
	CurrentTeamID          int64      `json:"currentTeamId"`
	CurrentAssigneeID      int64      `json:"currentAssigneeId"`
	TenantID               int64      `json:"tenantId"`
	ProductID              int64      `json:"productId"`
	ProductModelID         int64      `json:"productModelId"`
	ProductModuleID        int64      `json:"productModuleId"`
	DeviceID               int64      `json:"deviceId"`
	ServiceCodeID          int64      `json:"serviceCodeId"`
	CustomerEntrySessionID int64      `json:"customerEntrySessionId"`
	ServiceRegion          string     `json:"serviceRegion"`
	FaultCode              string     `json:"faultCode"`
	SymptomSummary         string     `json:"symptomSummary"`
	DiagnosisSummary       string     `json:"diagnosisSummary"`
	SLADueAt               *time.Time `json:"slaDueAt"`
	ResolvedAt             *time.Time `json:"resolvedAt"`
}

type RepairHistoryFilter struct {
	DeviceID   int64
	ProductID  int64
	CustomerID int64
	Limit      int
}

type UpdateTicketRequest struct {
	TicketID               int64      `json:"ticketId"`
	Title                  string     `json:"title"`
	Description            string     `json:"description"`
	PriorityCode           string     `json:"priorityCode"`
	TagIDs                 []int64    `json:"tagIds"`
	CurrentAssigneeID      int64      `json:"currentAssigneeId"`
	TenantID               int64      `json:"tenantId"`
	ProductID              int64      `json:"productId"`
	ProductModelID         int64      `json:"productModelId"`
	ProductModuleID        int64      `json:"productModuleId"`
	DeviceID               int64      `json:"deviceId"`
	ServiceCodeID          int64      `json:"serviceCodeId"`
	CustomerEntrySessionID int64      `json:"customerEntrySessionId"`
	ServiceRegion          string     `json:"serviceRegion"`
	FaultCode              string     `json:"faultCode"`
	SymptomSummary         string     `json:"symptomSummary"`
	DiagnosisSummary       string     `json:"diagnosisSummary"`
	SLADueAt               *time.Time `json:"slaDueAt"`
	ResolvedAt             *time.Time `json:"resolvedAt"`
}

type LinkTicketCustomerRequest struct {
	TicketID   int64 `json:"ticketId"`
	CustomerID int64 `json:"customerId"`
}

type AssignTicketRequest struct {
	TicketID int64  `json:"ticketId"`
	ToUserID int64  `json:"toUserId"`
	Reason   string `json:"reason"`
}

type ChangeTicketStatusRequest struct {
	TicketID   int64  `json:"ticketId"`
	Status     string `json:"status"`
	Resolution string `json:"resolution"`
}

type CreateTicketProgressRequest struct {
	TicketID          int64  `json:"ticketId"`
	Content           string `json:"content"`
	VisibleToCustomer bool   `json:"visibleToCustomer"`
}

type SaveTicketViewRequest struct {
	ID      int64          `json:"id"`
	Name    string         `json:"name"`
	Filters map[string]any `json:"filters"`
}

type DeleteTicketViewRequest struct {
	ID int64 `json:"id"`
}

type AcceptTicketRequest struct {
	AssigneeID int64 `json:"assigneeId"`
}

// TransitionTicketRequest 售后状态机通用流转请求。
type TransitionTicketRequest struct {
	TicketID int64  `json:"ticketId"`
	Status   string `json:"status"`
	Remark   string `json:"remark"`
}

// CreateTicketRepairRecordRequest 创建工单维修记录请求。
type CreateTicketRepairRecordRequest struct {
	TicketID          int64               `json:"ticketId"`
	FaultCode         string              `json:"faultCode"`
	Conclusion        string              `json:"conclusion"`
	Solution          string              `json:"solution"`
	RootCause         string              `json:"rootCause"`
	RepairMethod      string              `json:"repairMethod"`
	ServiceMethod     string              `json:"serviceMethod"`
	TestResult        string              `json:"testResult"`
	WarrantyCovered   bool                `json:"warrantyCovered"`
	RemoteResolved    bool                `json:"remoteResolved"`
	VisibleToCustomer bool                `json:"visibleToCustomer"`
	Parts             []RepairPartRequest `json:"parts"`
	CostHours         float64             `json:"costHours"`
	MarkResolved      bool                `json:"markResolved"`
}

type RepairPartRequest struct {
	Name     string `json:"name"`
	Quantity int64  `json:"quantity"`
}

// SubmitTicketFeedbackRequest 客户提交工单评价请求。
type SubmitTicketFeedbackRequest struct {
	TicketID int64    `json:"ticketId"`
	Rating   int      `json:"rating"` // 1-5
	Tags     []string `json:"tags"`
	Comment  string   `json:"comment"`
}
