package events

import "time"

// TicketReopenedEvent 工单重新打开事件（故障统计补偿源，§6.1）。
type TicketReopenedEvent struct {
	EventID    string
	TicketID   int64
	TenantID   int64
	OperatorID int64
	Reason     string
	OccurredAt time.Time
}

// CustomerRatingCreatedEvent 客户评价创建事件（低分计入故障统计，§6.1）。
type CustomerRatingCreatedEvent struct {
	EventID    string
	FeedbackID int64
	TenantID   int64
	TicketID   int64
	Score      int
	OperatorID int64
}

// 故障统计领域事件类型（§6.1）。
const (
	FaultStatsEventTicketClosed        = "ticket.closed"
	FaultStatsEventTicketReopened      = "ticket.reopened"
	FaultStatsEventRepairHistoryCreate = "repair_history.created"
	FaultStatsEventFaultConfirmed      = "diagnosis.fault_confirmed"
	FaultStatsEventCustomerRating      = "customer_rating.created"
	FaultStatsEventVideoMeeting        = "video_meeting.completed"
)

// ProductFaultStatsEvent 归一化故障统计事件。
// 业务事务完成后由发布方转换为该事件；Projector 以 EventID 做消费幂等，
// Delta 为 +1（累计）或 -1（补偿）。
type ProductFaultStatsEvent struct {
	EventID        string
	EventType      string
	TenantID       int64
	ProductID      int64
	ProductModelID int64
	DeviceID       int64
	TicketID       int64
	FaultCode      string
	FaultPart      string
	ModuleID       int64
	OccurredAt     time.Time
	OperatorID     int64
	Delta          int
}
