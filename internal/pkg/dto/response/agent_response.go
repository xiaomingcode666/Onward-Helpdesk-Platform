package response

import "remotehelpdesk/internal/pkg/enums"

type AgentProfileResponse struct {
	ID                    int64               `json:"id"`
	TenantID              int64               `json:"tenantId"`
	UserID                int64               `json:"userId"`
	TeamID                int64               `json:"teamId"`
	TeamIDs               []int64             `json:"teamIds,omitempty"`
	TeamName              string              `json:"teamName,omitempty"`
	Username              string              `json:"username,omitempty"`
	Nickname              string              `json:"nickname,omitempty"`
	AgentCode             string              `json:"agentCode"`
	DisplayName           string              `json:"displayName"`
	Avatar                string              `json:"avatar"`
	ServiceStatus         enums.ServiceStatus `json:"serviceStatus"`
	MaxConcurrentCount    int                 `json:"maxConcurrentCount"`
	PriorityLevel         int                 `json:"priorityLevel"`
	AutoAssignEnabled     bool                `json:"autoAssignEnabled"`
	TeamDispatchEnabled   bool                `json:"teamDispatchEnabled"`
	DispatchWeight        int                 `json:"dispatchWeight,omitempty"`
	ReceiveOfflineMessage bool                `json:"receiveOfflineMessage"`
	LastOnlineAt          string              `json:"lastOnlineAt,omitempty"`
	LastStatusAt          string              `json:"lastStatusAt,omitempty"`
	Remark                string              `json:"remark"`
}

type AgentDispatchCandidateResponse struct {
	AgentProfileResponse
	AssignmentMode          string  `json:"assignmentMode"`
	TeamEligible            bool    `json:"teamEligible"`
	WorkStatusConfirmed     bool    `json:"workStatusConfirmed"`
	Reachable               bool    `json:"reachable"`
	CapacityAvailable       bool    `json:"capacityAvailable"`
	ActiveConversationCount int     `json:"activeConversationCount"`
	OpenTicketCount         int     `json:"openTicketCount"`
	Workload                int     `json:"workload"`
	LoadRate                float64 `json:"loadRate"`
	CapacityRemaining       int     `json:"capacityRemaining"`
}

type AgentTeamResponse struct {
	ID               int64        `json:"id"`
	TenantID         int64        `json:"tenantId"`
	ParentID         int64        `json:"parentId"`
	ProductID        int64        `json:"productId"`
	ProductName      string       `json:"productName,omitempty"`
	DepartmentID     int64        `json:"departmentId"`
	TeamType         string       `json:"teamType"`
	SystemManaged    bool         `json:"systemManaged"`
	Name             string       `json:"name"`
	LeaderUserID     int64        `json:"leaderUserId"`
	LeaderUsername   string       `json:"leaderUsername,omitempty"`
	LeaderNickname   string       `json:"leaderNickname,omitempty"`
	AssignmentMode   string       `json:"assignmentMode"`
	ScheduleEnforced bool         `json:"scheduleEnforced"`
	ScheduleVersion  int          `json:"scheduleVersion"`
	Status           enums.Status `json:"status"`
	Description      string       `json:"description"`
	Remark           string       `json:"remark"`
}

type AgentTeamMemberResponse struct {
	ID              int64        `json:"id"`
	TenantID        int64        `json:"tenantId"`
	TeamID          int64        `json:"teamId"`
	UserID          int64        `json:"userId"`
	MemberID        int64        `json:"memberId"`
	DispatchEnabled bool         `json:"dispatchEnabled"`
	DispatchWeight  int          `json:"dispatchWeight"`
	Status          enums.Status `json:"status"`
}

type AgentTeamMemberRemovalResponse struct {
	TeamID                        int64 `json:"teamId"`
	UserID                        int64 `json:"userId"`
	PendingTicketsRecovered       int   `json:"pendingTicketsRecovered"`
	PendingConversationsRecovered int   `json:"pendingConversationsRecovered"`
	AlreadyRemoved                bool  `json:"alreadyRemoved"`
}

type AgentTeamScheduleResponse struct {
	ID             int64  `json:"id"`
	TenantID       int64  `json:"tenantId"`
	TeamID         int64  `json:"teamId"`
	TeamName       string `json:"teamName,omitempty"`
	UserID         int64  `json:"userId"`
	UserName       string `json:"userName,omitempty"`
	UserAvatar     string `json:"userAvatar,omitempty"`
	RepeatType     string `json:"repeatType"`
	DayType        string `json:"dayType"`
	Weekday        int    `json:"weekday"`
	StartMinute    int    `json:"startMinute"`
	EndMinute      int    `json:"endMinute"`
	StartTime      string `json:"startTime,omitempty"`
	EndTime        string `json:"endTime,omitempty"`
	Timezone       string `json:"timezone"`
	EffectiveFrom  string `json:"effectiveFrom,omitempty"`
	EffectiveUntil string `json:"effectiveUntil,omitempty"`
	PublishStatus  string `json:"publishStatus"`
	Version        int    `json:"version"`
	StartAt        string `json:"startAt"`
	EndAt          string `json:"endAt"`
	Remark         string `json:"remark"`
}

type AgentTeamScheduleTemplateResponse struct {
	TenantID  int64  `json:"tenantId"`
	Workdays  []int  `json:"workdays"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Timezone  string `json:"timezone"`
}

type EngineerWorkScheduleResponse struct {
	IsEngineer     bool                              `json:"isEngineer"`
	Teams          []AgentTeamResponse               `json:"teams"`
	Schedules      []AgentTeamScheduleResponse       `json:"schedules"`
	DraftSchedules []AgentTeamScheduleResponse       `json:"draftSchedules"`
	BaseSchedule   AgentTeamScheduleTemplateResponse `json:"baseSchedule"`
	Timezone       string                            `json:"timezone"`
	Exceptions     []AgentScheduleExceptionResponse  `json:"exceptions"`
}

type AgentScheduleExceptionResponse struct {
	ID               int64  `json:"id"`
	TenantID         int64  `json:"tenantId"`
	UserID           int64  `json:"userId"`
	UserName         string `json:"userName,omitempty"`
	UserAvatar       string `json:"userAvatar,omitempty"`
	RequestKey       string `json:"requestKey,omitempty"`
	ExceptionType    string `json:"exceptionType"`
	StartAt          string `json:"startAt"`
	EndAt            string `json:"endAt"`
	ApprovalStatus   string `json:"approvalStatus"`
	Reason           string `json:"reason,omitempty"`
	ReviewNote       string `json:"reviewNote,omitempty"`
	RequestedAt      string `json:"requestedAt"`
	ReviewedAt       string `json:"reviewedAt,omitempty"`
	ReviewerUserID   int64  `json:"reviewerUserId,omitempty"`
	ReviewerUserName string `json:"reviewerUserName,omitempty"`
}

type AgentTeamMemberAvailabilityResponse struct {
	UserID              int64                           `json:"userId"`
	MemberID            int64                           `json:"memberId"`
	ProfileID           int64                           `json:"profileId"`
	Username            string                          `json:"username,omitempty"`
	Nickname            string                          `json:"nickname,omitempty"`
	DisplayName         string                          `json:"displayName"`
	Avatar              string                          `json:"avatar,omitempty"`
	AgentCode           string                          `json:"agentCode,omitempty"`
	DispatchEnabled     bool                            `json:"dispatchEnabled"`
	DispatchWeight      int                             `json:"dispatchWeight"`
	AutoAssignEnabled   bool                            `json:"autoAssignEnabled"`
	ServiceStatus       enums.ServiceStatus             `json:"serviceStatus"`
	MaxConcurrentCount  int                             `json:"maxConcurrentCount"`
	WorkStatus          string                          `json:"workStatus"`
	WorkStatusNote      string                          `json:"workStatusNote,omitempty"`
	WorkStatusConfirmed bool                            `json:"workStatusConfirmed"`
	AvailableNow        bool                            `json:"availableNow"`
	UnavailableReason   string                          `json:"unavailableReason,omitempty"`
	Workdays            []int                           `json:"workdays"`
	StartTime           string                          `json:"startTime"`
	EndTime             string                          `json:"endTime"`
	Timezone            string                          `json:"timezone"`
	ActiveLeave         *AgentScheduleExceptionResponse `json:"activeLeave,omitempty"`
	PendingLeave        *AgentScheduleExceptionResponse `json:"pendingLeave,omitempty"`
}

type AgentTeamScheduleBatchPreviewResponse struct {
	Total    int                                 `json:"total"`
	Conflict bool                                `json:"conflict"`
	Items    []AgentTeamScheduleBatchPreviewItem `json:"items"`
}

type AgentTeamScheduleBatchPreviewItem struct {
	TeamID         int64  `json:"teamId"`
	TeamName       string `json:"teamName"`
	Date           string `json:"date"`
	Weekday        int    `json:"weekday"`
	StartAt        string `json:"startAt"`
	EndAt          string `json:"endAt"`
	Remark         string `json:"remark"`
	Conflict       bool   `json:"conflict"`
	ConflictReason string `json:"conflictReason"`
}

type AgentTeamScheduleBatchGenerateResponse struct {
	Created int `json:"created"`
}

type AgentTeamScheduleDraftResponse struct {
	TeamID  int64 `json:"teamId"`
	Version int   `json:"version"`
	Count   int   `json:"count"`
}

type AgentTeamSchedulePublishResponse struct {
	TeamID          int64 `json:"teamId"`
	Version         int   `json:"version"`
	Published       int   `json:"published"`
	MissingWeekdays []int `json:"missingWeekdays"`
}
