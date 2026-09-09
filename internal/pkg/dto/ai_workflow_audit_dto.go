package dto

import (
	"time"

	"remotehelpdesk/internal/pkg/enums"
)

const (
	AIWorkflowSkillStateSelected    = "selected"
	AIWorkflowSkillStateNotSelected = "not_selected"
	AIWorkflowSkillStateNotEnabled  = "not_enabled"
)

type AIWorkflowSkillCandidate struct {
	ID          int64
	Name        string
	Description string
}

type AIWorkflowSkillAudit struct {
	State                    string
	MiddlewareEnabled        bool
	CandidateSkills          []AIWorkflowSkillCandidate
	SelectedSkillID          int64
	SelectedSkillName        string
	SelectedSkillDescription string
	MatchReason              string
	RouteTrace               string
	ExposedToolCodes         []string
	InvokedToolCodes         []string
	SourceMessageID          int64
	CreatedAt                time.Time
}

type AIWorkflowHumanHandlingAudit struct {
	HandoffOccurred          bool
	HandoffAt                time.Time
	HandoffReason            string
	HandledByHuman           bool
	HandlerUserID            int64
	HandlerName              string
	FirstHumanReplyMessageID int64
	FirstHumanReplyAt        time.Time
	ConversationStatus       enums.IMConversationStatus
	ConversationStatusName   string
}
