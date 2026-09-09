package request

import "remotehelpdesk/internal/pkg/enums"

type CreateAgentProfileRequest struct {
	UserID                int64               `json:"userId"`
	TeamID                int64               `json:"teamId"`
	AgentCode             string              `json:"agentCode"`
	DisplayName           string              `json:"displayName"`
	Avatar                string              `json:"avatar"`
	ServiceStatus         enums.ServiceStatus `json:"serviceStatus"`
	MaxConcurrentCount    int                 `json:"maxConcurrentCount"`
	PriorityLevel         int                 `json:"priorityLevel"`
	AutoAssignEnabled     bool                `json:"autoAssignEnabled"`
	ReceiveOfflineMessage bool                `json:"receiveOfflineMessage"`
	Remark                string              `json:"remark"`
}

type UpdateAgentProfileRequest struct {
	ID int64 `json:"id"`
	CreateAgentProfileRequest
}

type DeleteAgentProfileRequest struct {
	ID int64 `json:"id"`
}

type CreateAgentTeamRequest struct {
	Name           string `json:"name"`
	LeaderUserID   int64  `json:"leaderUserId"`
	AssignmentMode string `json:"assignmentMode"`
	Status         int    `json:"status"`
	Description    string `json:"description"`
	Remark         string `json:"remark"`
}

type UpdateAgentTeamRequest struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	LeaderUserID   int64  `json:"leaderUserId"`
	AssignmentMode string `json:"assignmentMode"`
	Status         int    `json:"status"`
	Description    string `json:"description"`
	Remark         string `json:"remark"`
}

type DeleteAgentTeamRequest struct {
	ID int64 `json:"id"`
}

type UpsertAgentTeamMemberRequest struct {
	TeamID          int64 `json:"teamId"`
	UserID          int64 `json:"userId"`
	DispatchEnabled bool  `json:"dispatchEnabled"`
	DispatchWeight  int   `json:"dispatchWeight"`
}

type DeleteAgentTeamMemberRequest struct {
	TeamID int64 `json:"teamId"`
	UserID int64 `json:"userId"`
}

type CreateAgentTeamScheduleRequest struct {
	TeamID         int64  `json:"teamId"`
	UserID         int64  `json:"userId"`
	RepeatType     string `json:"repeatType"`
	DayType        string `json:"dayType"`
	Weekday        int    `json:"weekday"`
	StartTime      string `json:"startTime"`
	EndTime        string `json:"endTime"`
	StartAt        string `json:"startAt"`
	EndAt          string `json:"endAt"`
	Timezone       string `json:"timezone"`
	EffectiveFrom  string `json:"effectiveFrom"`
	EffectiveUntil string `json:"effectiveUntil"`
	Remark         string `json:"remark"`
}

type UpdateAgentTeamScheduleTemplateRequest struct {
	Workdays  []int  `json:"workdays"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Timezone  string `json:"timezone"`
}

type CreateAgentScheduleLeaveRequest struct {
	RequestKey string `json:"requestKey"`
	StartAt    string `json:"startAt"`
	EndAt      string `json:"endAt"`
	Reason     string `json:"reason"`
}

type CancelAgentScheduleLeaveRequest struct {
	ID int64 `json:"id"`
}

type ReviewAgentScheduleLeaveRequest struct {
	ID         int64  `json:"id"`
	Decision   string `json:"decision"`
	ReviewNote string `json:"reviewNote"`
}

type UpdateAgentTeamScheduleRequest struct {
	ID int64 `json:"id"`
	CreateAgentTeamScheduleRequest
}

type DeleteAgentTeamScheduleRequest struct {
	ID int64 `json:"id"`
}

type PrepareAgentTeamScheduleDraftRequest struct {
	TeamID int64 `json:"teamId"`
}

type PublishAgentTeamScheduleRequest struct {
	TeamID           int64 `json:"teamId"`
	AllowCoverageGap bool  `json:"allowCoverageGap"`
}

type RollbackAgentTeamScheduleRequest struct {
	TeamID int64 `json:"teamId"`
}

type DisableAgentTeamScheduleRequest struct {
	TeamID int64 `json:"teamId"`
}

type UpdateAgentWorkStatusRequest struct {
	Status      string `json:"status"`
	Note        string `json:"note"`
	AvailableAt string `json:"availableAt"`
}

type AgentTeamScheduleCalendarRequest struct {
	TenantID int64  `json:"-"`
	StartAt  string `json:"startAt"`
	EndAt    string `json:"endAt"`
	TeamID   int64  `json:"teamId"`
}

type AgentTeamScheduleBatchRequest struct {
	TenantID  int64   `json:"-"`
	TeamIDs   []int64 `json:"teamIds"`
	StartDate string  `json:"startDate"`
	EndDate   string  `json:"endDate"`
	Weekdays  []int   `json:"weekdays"`
	StartTime string  `json:"startTime"`
	EndTime   string  `json:"endTime"`
	Remark    string  `json:"remark"`
	Locale    string  `json:"-"`
}
