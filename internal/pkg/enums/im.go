package enums

type IMConversationStatus int

const (
	IMConversationStatusAIServing IMConversationStatus = 1
	IMConversationStatusPending   IMConversationStatus = 2
	IMConversationStatusActive    IMConversationStatus = 3
	IMConversationStatusClosed    IMConversationStatus = 4
)

var imConversationStatusLabelMap = map[IMConversationStatus]string{
	IMConversationStatusAIServing: "enum.IMConversationStatusAIServing",
	IMConversationStatusPending:   "enum.IMConversationStatusPending",
	IMConversationStatusActive:    "enum.IMConversationStatusActive",
	IMConversationStatusClosed:    "enum.IMConversationStatusClosed",
}

var IMConversationStatusValues = []IMConversationStatus{
	IMConversationStatusAIServing,
	IMConversationStatusPending,
	IMConversationStatusActive,
	IMConversationStatusClosed,
}

func GetIMConversationStatusLabel(status IMConversationStatus, locale string) string {
	return enumLabel(locale, imConversationStatusLabelMap[status])
}

type IMConversationServiceMode int

const (
	IMConversationServiceModeAIOnly    IMConversationServiceMode = 1
	IMConversationServiceModeHumanOnly IMConversationServiceMode = 2
	IMConversationServiceModeAIFirst   IMConversationServiceMode = 3
)

var IMConversationServiceModeValues = []IMConversationServiceMode{
	IMConversationServiceModeAIOnly,
	IMConversationServiceModeHumanOnly,
	IMConversationServiceModeAIFirst,
}

var imConversationServiceModeLabelMap = map[IMConversationServiceMode]string{
	IMConversationServiceModeAIOnly:    "enum.IMConversationServiceModeAIOnly",
	IMConversationServiceModeHumanOnly: "enum.IMConversationServiceModeHumanOnly",
	IMConversationServiceModeAIFirst:   "enum.IMConversationServiceModeAIFirst",
}

func GetIMConversationServiceModeLabel(mode IMConversationServiceMode, locale string) string {
	return enumLabel(locale, imConversationServiceModeLabelMap[mode])
}

type IMSenderType string

const (
	IMSenderTypeAgent    IMSenderType = "agent"    // 客服
	IMSenderTypeCustomer IMSenderType = "customer" // 客户
	IMSenderTypeAI       IMSenderType = "ai"       // AI
	IMSenderTypePartner  IMSenderType = "partner"  // 供应商
	IMSenderTypeSystem   IMSenderType = "system"   // 系统
)

var imSenderTypeLabelMap = map[IMSenderType]string{
	IMSenderTypeAgent:    "enum.IMSenderTypeAgent",
	IMSenderTypeCustomer: "enum.IMSenderTypeCustomer",
	IMSenderTypeAI:       "enum.IMSenderTypeAI",
	IMSenderTypePartner:  "enum.IMSenderTypePartner",
	IMSenderTypeSystem:   "enum.IMSenderTypeSystem",
}

func GetIMSenderTypeLabel(senderType IMSenderType, locale string) string {
	return enumLabel(locale, imSenderTypeLabelMap[senderType])
}

type IMParticipantType string

const (
	IMParticipantTypeCustomer IMParticipantType = "customer"
	IMParticipantTypeAgent    IMParticipantType = "agent"
	IMParticipantTypeAI       IMParticipantType = "ai"
	IMParticipantTypePartner  IMParticipantType = "partner"
	IMParticipantTypeSystem   IMParticipantType = "system"
)

var imParticipantTypeLabelMap = map[IMParticipantType]string{
	IMParticipantTypeCustomer: "enum.IMParticipantTypeCustomer",
	IMParticipantTypeAgent:    "enum.IMParticipantTypeAgent",
	IMParticipantTypeAI:       "enum.IMParticipantTypeAI",
	IMParticipantTypePartner:  "enum.IMParticipantTypePartner",
	IMParticipantTypeSystem:   "enum.IMParticipantTypeSystem",
}

func GetIMParticipantTypeLabel(participantType IMParticipantType, locale string) string {
	return enumLabel(locale, imParticipantTypeLabelMap[participantType])
}

type IMAssignmentType string

const (
	IMAssignmentTypeAssign   IMAssignmentType = "assign"
	IMAssignmentTypeTransfer IMAssignmentType = "transfer"
)

var imAssignmentTypeLabelMap = map[IMAssignmentType]string{
	IMAssignmentTypeAssign:   "enum.IMAssignmentTypeAssign",
	IMAssignmentTypeTransfer: "enum.IMAssignmentTypeTransfer",
}

func GetIMAssignmentTypeLabel(assignType IMAssignmentType, locale string) string {
	return enumLabel(locale, imAssignmentTypeLabelMap[assignType])
}

type IMAssignmentStatus int

const (
	IMAssignmentStatusActive   IMAssignmentStatus = 0
	IMAssignmentStatusInactive IMAssignmentStatus = 1
)

var imAssignmentStatusLabelMap = map[IMAssignmentStatus]string{
	IMAssignmentStatusActive:   "enum.IMAssignmentStatusActive",
	IMAssignmentStatusInactive: "enum.IMAssignmentStatusInactive",
}

func GetIMAssignmentStatusLabel(status IMAssignmentStatus, locale string) string {
	return enumLabel(locale, imAssignmentStatusLabelMap[status])
}

type IMMessageType string

const (
	IMMessageTypeText       IMMessageType = "text"
	IMMessageTypeImage      IMMessageType = "image"
	IMMessageTypeAudio      IMMessageType = "audio"
	IMMessageTypeAttachment IMMessageType = "attachment"
	IMMessageTypeHTML       IMMessageType = "html"
)

var imMessageTypeLabelMap = map[IMMessageType]string{
	IMMessageTypeText:       "enum.IMMessageTypeText",
	IMMessageTypeImage:      "enum.IMMessageTypeImage",
	IMMessageTypeAudio:      "enum.IMMessageTypeAudio",
	IMMessageTypeAttachment: "enum.IMMessageTypeAttachment",
	IMMessageTypeHTML:       "enum.IMMessageTypeHTML",
}

func GetIMMessageTypeLabel(messageType IMMessageType, locale string) string {
	return enumLabel(locale, imMessageTypeLabelMap[messageType])
}

type IMMessageStatus int

const (
	IMMessageStatusSending   IMMessageStatus = 1
	IMMessageStatusSent      IMMessageStatus = 2
	IMMessageStatusDelivered IMMessageStatus = 3
	IMMessageStatusRead      IMMessageStatus = 4
	IMMessageStatusFailed    IMMessageStatus = 5
	IMMessageStatusRecalled  IMMessageStatus = 6
)

var imMessageStatusLabelMap = map[IMMessageStatus]string{
	IMMessageStatusSending:   "enum.IMMessageStatusSending",
	IMMessageStatusSent:      "enum.IMMessageStatusSent",
	IMMessageStatusDelivered: "enum.IMMessageStatusDelivered",
	IMMessageStatusRead:      "enum.IMMessageStatusRead",
	IMMessageStatusFailed:    "enum.IMMessageStatusFailed",
	IMMessageStatusRecalled:  "enum.IMMessageStatusRecalled",
}

func GetIMMessageStatusLabel(status IMMessageStatus, locale string) string {
	return enumLabel(locale, imMessageStatusLabelMap[status])
}

type IMEventType string

const (
	IMEventTypeCreate        IMEventType = "create"
	IMEventTypeAssign        IMEventType = "assign"
	IMEventTypeTransfer      IMEventType = "transfer"
	IMEventTypeClose         IMEventType = "close"
	IMEventTypeMessageSend   IMEventType = "message_send"
	IMEventTypeMessageRecall IMEventType = "message_recall"
)

var imEventTypeLabelMap = map[IMEventType]string{
	IMEventTypeCreate:        "enum.IMEventTypeCreate",
	IMEventTypeAssign:        "enum.IMEventTypeAssign",
	IMEventTypeTransfer:      "enum.IMEventTypeTransfer",
	IMEventTypeClose:         "enum.IMEventTypeClose",
	IMEventTypeMessageSend:   "enum.IMEventTypeMessageSend",
	IMEventTypeMessageRecall: "enum.IMEventTypeMessageRecall",
}

func GetIMEventTypeLabel(eventType IMEventType, locale string) string {
	return enumLabel(locale, imEventTypeLabelMap[eventType])
}

type AIAgentHandoffMode int

const (
	AIAgentHandoffModeWaitPool        AIAgentHandoffMode = 1
	AIAgentHandoffModeDefaultTeamPool AIAgentHandoffMode = 2
	AIAgentHandoffModeAIHoldAndNotify AIAgentHandoffMode = 3
)

var AIAgentHandoffModeValues = []AIAgentHandoffMode{
	AIAgentHandoffModeWaitPool,
	AIAgentHandoffModeDefaultTeamPool,
	AIAgentHandoffModeAIHoldAndNotify,
}

var aiAgentHandoffModeLabelMap = map[AIAgentHandoffMode]string{
	AIAgentHandoffModeWaitPool:        "enum.AIAgentHandoffModeWaitPool",
	AIAgentHandoffModeDefaultTeamPool: "enum.AIAgentHandoffModeDefaultTeamPool",
	AIAgentHandoffModeAIHoldAndNotify: "enum.AIAgentHandoffModeAIHoldAndNotify",
}

func GetAIAgentHandoffModeLabel(mode AIAgentHandoffMode, locale string) string {
	return enumLabel(locale, aiAgentHandoffModeLabelMap[mode])
}

type AIAgentFallbackMode int

const (
	AIAgentFallbackModeNoAnswer     AIAgentFallbackMode = 1
	AIAgentFallbackModeSuggestRetry AIAgentFallbackMode = 2
)

var AIAgentFallbackModeValues = []AIAgentFallbackMode{
	AIAgentFallbackModeNoAnswer,
	AIAgentFallbackModeSuggestRetry,
}

var aiAgentFallbackModeLabelMap = map[AIAgentFallbackMode]string{
	AIAgentFallbackModeNoAnswer:     "enum.AIAgentFallbackModeNoAnswer",
	AIAgentFallbackModeSuggestRetry: "enum.AIAgentFallbackModeSuggestRetry",
}

func GetAIAgentFallbackModeLabel(mode AIAgentFallbackMode, locale string) string {
	return enumLabel(locale, aiAgentFallbackModeLabelMap[mode])
}

const (
	IMRealtimeEventConnected               = "connected"
	IMRealtimeEventPong                    = "pong"
	IMRealtimeEventSubscribed              = "subscribed"
	IMRealtimeEventUnsubscribed            = "unsubscribed"
	IMRealtimeEventResyncRequired          = "resyncRequired"
	IMRealtimeEventMessageCreated          = "message.created"
	IMRealtimeEventMessageRecalled         = "message.recalled"
	IMRealtimeEventConversationCreated     = "conversation.created"
	IMRealtimeEventConversationUpdated     = "conversation.updated"
	IMRealtimeEventConversationAssigned    = "conversation.assigned"
	IMRealtimeEventConversationTransferred = "conversation.transferred"
	IMRealtimeEventConversationClosed      = "conversation.closed"
	IMRealtimeEventConversationRead        = "conversation.read"
	IMRealtimeEventPresenceSnapshot        = "presence.snapshot"
	IMRealtimeEventPresenceChanged         = "presence.changed"
	IMRealtimeEventTypingChanged           = "typing.changed"
	IMRealtimeEventNotificationCreated     = "notification.created"
	IMRealtimeEventCustomerSessionRefresh  = "customer_session.refresh"
)

const (
	IMRealtimeClientTypePing        = "ping"
	IMRealtimeClientTypeSubscribe   = "subscribe"
	IMRealtimeClientTypeUnsubscribe = "unsubscribe"
	IMRealtimeClientTypeAck         = "ack"
	IMRealtimeClientTypeTyping      = "typing"
)

const (
	IMRealtimeResyncReasonInvalidPayload         = "invalid_payload"
	IMRealtimeResyncReasonUnsupportedMessageType = "unsupported_message_type"
	IMRealtimeResyncReasonManual                 = "manual"
	IMRealtimeResyncReasonNotificationUpdated    = "notification_updated"
)
