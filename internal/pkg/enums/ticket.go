package enums

type TicketStatus string

type TicketProgressEventType string

const (
	TicketProgressEventCreated         TicketProgressEventType = "ticket_created"
	TicketProgressEventAccepted        TicketProgressEventType = "ticket_accepted"
	TicketProgressEventAssigned        TicketProgressEventType = "ticket_assigned"
	TicketProgressEventProcessing      TicketProgressEventType = "ticket_processing"
	TicketProgressEventEscalated       TicketProgressEventType = "ticket_escalated"
	TicketProgressEventRepairCompleted TicketProgressEventType = "repair_completed"
	TicketProgressEventClosed          TicketProgressEventType = "ticket_closed"
	TicketProgressEventReopened        TicketProgressEventType = "ticket_reopened"
	TicketProgressEventMeetingEnded    TicketProgressEventType = "meeting_ended"
	TicketProgressEventProgress        TicketProgressEventType = "progress"
)

const (
	TicketStatusPending         TicketStatus = "pending"
	TicketStatusAccepted        TicketStatus = "accepted"
	TicketStatusAssigned        TicketStatus = "assigned"
	TicketStatusInProgress      TicketStatus = "in_progress"
	TicketStatusEscalated       TicketStatus = "escalated"
	TicketStatusWaitingCustomer TicketStatus = "waiting_customer"
	TicketStatusResolved        TicketStatus = "resolved"
	TicketStatusClosed          TicketStatus = "closed"
	TicketStatusReopened        TicketStatus = "reopened"
	// TicketStatusDone 保留兼容旧版前端使用的 done 状态值
	TicketStatusDone TicketStatus = "done"
)

var TicketStatusValues = []TicketStatus{
	TicketStatusPending,
	TicketStatusAccepted,
	TicketStatusAssigned,
	TicketStatusInProgress,
	TicketStatusEscalated,
	TicketStatusWaitingCustomer,
	TicketStatusResolved,
	TicketStatusClosed,
	TicketStatusReopened,
	TicketStatusDone,
}

var ticketStatusLabelMap = map[TicketStatus]string{
	TicketStatusPending:         "待处理",
	TicketStatusAccepted:        "已受理",
	TicketStatusAssigned:        "已分配",
	TicketStatusInProgress:      "处理中",
	TicketStatusEscalated:       "已升级",
	TicketStatusWaitingCustomer: "等待客户",
	TicketStatusResolved:        "已解决",
	TicketStatusClosed:          "已关闭",
	TicketStatusReopened:        "已重新打开",
	TicketStatusDone:            "已完成",
}

func GetTicketStatusLabel(status TicketStatus) string {
	return ticketStatusLabelMap[status]
}

func IsValidTicketStatus(status string) bool {
	for _, item := range TicketStatusValues {
		if string(item) == status {
			return true
		}
	}
	return false
}

// TicketStatusTransitionMap 定义合法的工单状态流转
var TicketStatusTransitionMap = map[TicketStatus][]TicketStatus{
	TicketStatusPending:         {TicketStatusAccepted, TicketStatusAssigned, TicketStatusClosed, TicketStatusDone},
	TicketStatusAccepted:        {TicketStatusAssigned, TicketStatusInProgress, TicketStatusEscalated, TicketStatusClosed, TicketStatusDone},
	TicketStatusAssigned:        {TicketStatusAccepted, TicketStatusInProgress, TicketStatusEscalated, TicketStatusClosed, TicketStatusDone},
	TicketStatusInProgress:      {TicketStatusAssigned, TicketStatusResolved, TicketStatusEscalated, TicketStatusWaitingCustomer, TicketStatusClosed, TicketStatusDone},
	TicketStatusEscalated:       {TicketStatusInProgress, TicketStatusClosed, TicketStatusDone},
	TicketStatusWaitingCustomer: {TicketStatusInProgress, TicketStatusResolved, TicketStatusClosed, TicketStatusEscalated, TicketStatusDone},
	TicketStatusResolved:        {TicketStatusClosed, TicketStatusReopened, TicketStatusDone},
	TicketStatusClosed:          {TicketStatusReopened},
	TicketStatusReopened:        {TicketStatusAccepted, TicketStatusAssigned, TicketStatusInProgress, TicketStatusClosed, TicketStatusDone},
	TicketStatusDone:            {TicketStatusReopened},
}

// IsValidTicketStatusTransition 校验工单状态流转是否合法
func IsValidTicketStatusTransition(from, to string) bool {
	fromStatus := TicketStatus(from)
	toStatus := TicketStatus(to)
	allowed, ok := TicketStatusTransitionMap[fromStatus]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == toStatus {
			return true
		}
	}
	return false
}

// 售后语义状态（P0）。accepted / closed / reopened 复用上方已有的同值常量。
const (
	TicketStatusDraft                  TicketStatus = "draft"
	TicketStatusPendingAcceptance      TicketStatus = "pending_acceptance"
	TicketStatusPendingDispatch        TicketStatus = "pending_dispatch"
	TicketStatusPendingAssigneeAccept  TicketStatus = "pending_assignee_accept"
	TicketStatusProcessing             TicketStatus = "processing"
	TicketStatusVideoSupport           TicketStatus = "video_support"
	TicketStatusSupplierSupport        TicketStatus = "supplier_support"
	TicketStatusPendingCustomerConfirm TicketStatus = "pending_customer_confirm"
	TicketStatusQualityReview          TicketStatus = "quality_review"
	TicketStatusCancelled              TicketStatus = "cancelled"
)

// AfterSalesTicketStatusValues 售后状态机的全部合法状态。
var AfterSalesTicketStatusValues = []TicketStatus{
	TicketStatusDraft,
	TicketStatusPendingAcceptance,
	TicketStatusAccepted,
	TicketStatusPendingDispatch,
	TicketStatusPendingAssigneeAccept,
	TicketStatusProcessing,
	TicketStatusVideoSupport,
	TicketStatusSupplierSupport,
	TicketStatusResolved,
	TicketStatusPendingCustomerConfirm,
	TicketStatusClosed,
	TicketStatusQualityReview,
	TicketStatusReopened,
	TicketStatusCancelled,
}

// legacyToAfterSalesStatusMap 将旧状态兼容映射到售后状态。
// 注意：resolved 是售后态的正式一员（维修完成待确认），不在此映射中，规范化时保持原值。
var legacyToAfterSalesStatusMap = map[TicketStatus]TicketStatus{
	TicketStatusPending:         TicketStatusPendingAcceptance,
	TicketStatusAssigned:        TicketStatusPendingAssigneeAccept,
	TicketStatusInProgress:      TicketStatusProcessing,
	TicketStatusDone:            TicketStatusClosed,
	TicketStatusEscalated:       TicketStatusProcessing,
	TicketStatusWaitingCustomer: TicketStatusPendingCustomerConfirm,
}

// NormalizeTicketStatus 把任意工单状态规范化为售后状态：
// 旧状态映射到新状态；已是售后状态的保持不变；未知状态原样返回。
func NormalizeTicketStatus(status string) TicketStatus {
	s := TicketStatus(status)
	if mapped, ok := legacyToAfterSalesStatusMap[s]; ok {
		return mapped
	}
	return s
}

// afterSalesTransitionMap 定义售后状态机的合法流转（设计 §6.2）。
var afterSalesTransitionMap = map[TicketStatus][]TicketStatus{
	TicketStatusDraft:                  {TicketStatusPendingAcceptance, TicketStatusCancelled},
	TicketStatusPendingAcceptance:      {TicketStatusAccepted, TicketStatusCancelled},
	TicketStatusAccepted:               {TicketStatusPendingDispatch, TicketStatusCancelled},
	TicketStatusPendingDispatch:        {TicketStatusPendingAssigneeAccept, TicketStatusCancelled},
	TicketStatusPendingAssigneeAccept:  {TicketStatusProcessing, TicketStatusPendingDispatch, TicketStatusCancelled},
	TicketStatusProcessing:             {TicketStatusVideoSupport, TicketStatusSupplierSupport, TicketStatusResolved, TicketStatusPendingCustomerConfirm, TicketStatusPendingDispatch, TicketStatusQualityReview, TicketStatusCancelled},
	TicketStatusVideoSupport:           {TicketStatusProcessing, TicketStatusSupplierSupport, TicketStatusResolved, TicketStatusPendingCustomerConfirm, TicketStatusCancelled},
	TicketStatusSupplierSupport:        {TicketStatusProcessing, TicketStatusVideoSupport, TicketStatusResolved, TicketStatusPendingCustomerConfirm, TicketStatusCancelled},
	TicketStatusResolved:               {TicketStatusClosed, TicketStatusPendingCustomerConfirm, TicketStatusReopened},
	TicketStatusPendingCustomerConfirm: {TicketStatusClosed, TicketStatusReopened, TicketStatusProcessing},
	TicketStatusClosed:                 {TicketStatusQualityReview, TicketStatusReopened},
	TicketStatusQualityReview:          {TicketStatusClosed, TicketStatusProcessing},
	TicketStatusReopened:               {TicketStatusPendingDispatch, TicketStatusProcessing, TicketStatusCancelled},
	TicketStatusCancelled:              {},
}

// AllowedTransitions 返回某个售后状态下可流转到的目标状态列表。
func AllowedTransitions(status string) []TicketStatus {
	return afterSalesTransitionMap[NormalizeTicketStatus(status)]
}

// IsValidAfterSalesStatus 判断是否为合法的售后状态。
func IsValidAfterSalesStatus(status string) bool {
	s := TicketStatus(status)
	for _, item := range AfterSalesTicketStatusValues {
		if item == s {
			return true
		}
	}
	return false
}

// IsValidAfterSalesTransition 校验售后状态流转是否合法（入参先做规范化）。
func IsValidAfterSalesTransition(from, to string) bool {
	fromStatus := NormalizeTicketStatus(from)
	toStatus := NormalizeTicketStatus(to)
	for _, s := range afterSalesTransitionMap[fromStatus] {
		if s == toStatus {
			return true
		}
	}
	return false
}

type TicketSource string

const (
	TicketSourceManual       TicketSource = "manual"
	TicketSourceConversation TicketSource = "conversation"
)

var TicketSourceValues = []TicketSource{
	TicketSourceManual,
	TicketSourceConversation,
}

func IsValidTicketSource(source string) bool {
	for _, item := range TicketSourceValues {
		if string(item) == source {
			return true
		}
	}
	return false
}
