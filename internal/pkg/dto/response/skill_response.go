package response

import "time"

type SkillDefinitionResponse struct {
	ID             int64     `json:"id"`
	TenantID       int64     `json:"tenantId"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Instruction    string    `json:"instruction"`
	Examples       []string  `json:"examples"`
	ToolWhitelist  []string  `json:"toolWhitelist"`
	Status         int       `json:"status"`
	StatusName     string    `json:"statusName"`
	Remark         string    `json:"remark"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	CreateUserName string    `json:"createUserName"`
	UpdateUserName string    `json:"updateUserName"`
}

type SkillDebugRunResponse struct {
	SkillDefinitionID int64    `json:"skillDefinitionId"`
	SkillName         string   `json:"skillName"`
	ReplyText         string   `json:"replyText"`
	PlanReason        string   `json:"planReason"`
	SkillRouteTrace   string   `json:"skillRouteTrace"`
	ToolWhitelist     []string `json:"toolWhitelist"`
	ExposedToolCodes  []string `json:"exposedToolCodes"`
	InvokedToolCodes  []string `json:"invokedToolCodes"`
	ToolSearchTrace   string   `json:"toolSearchTrace"`
	GraphToolTrace    string   `json:"graphToolTrace"`
	GraphToolCode     string   `json:"graphToolCode"`
	InterruptType     string   `json:"interruptType"`
	CheckPointID      string   `json:"checkPointId"`
	Interrupted       bool     `json:"interrupted"`
	TraceData         string   `json:"traceData"`
	ErrorMessage      string   `json:"errorMessage"`
	ConversationID    int64    `json:"conversationId"`
	AIAgentID         int64    `json:"aiAgentId"`
}
