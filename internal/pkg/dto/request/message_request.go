package request

import "remotehelpdesk/internal/pkg/enums"

type MessageListRequest struct {
	ConversationID int64  `json:"conversationId"`
	SenderType     string `json:"senderType"`
	MessageType    string `json:"messageType"`
}

type SendConversationMessageRequest struct {
	ConversationID int64               `json:"conversationId"`
	MessageType    enums.IMMessageType `json:"messageType"`
	Content        string              `json:"content"`
	Payload        string              `json:"payload"`
	ClientMsgID    string              `json:"clientMsgId"`
}

type RecallConversationMessageRequest struct {
	MessageID int64 `json:"messageId"`
}

type ForwardConversationImageRequest struct {
	SourceMessageID      int64  `json:"sourceMessageId"`
	TargetConversationID int64  `json:"targetConversationId"`
	ClientMsgID          string `json:"clientMsgId"`
}
