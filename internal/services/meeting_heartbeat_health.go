package services

import (
	"time"

	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/repositories"
)

const meetingParticipantStaleAfter = 3 * time.Minute

type PlatformJitsiAggregate struct {
	ActiveMeetings          int64
	Meetings24h             int64
	ParticipantMinutes24h   int64
	OnlineParticipants      int64
	StaleParticipants       int64
	StaleMeetings           int64
	HeartbeatStatus         string
	HeartbeatTimeoutSeconds int64
	LastMeetingAt           *time.Time
	LastHeartbeatAt         *time.Time
	ServiceStatus           string
	ServiceURL              string
	ProbeURL                string
	ProbeHTTPStatus         int
	ProbeLatencyMs          int64
	ProbeCheckedAt          *time.Time
	ProbeError              string
}

func buildPlatformJitsiAggregate(snapshot *repositories.PlatformJitsiSnapshot) *PlatformJitsiAggregate {
	if snapshot == nil {
		return nil
	}
	status := "idle"
	if snapshot.ActiveMeetings > 0 {
		status = "healthy"
		if snapshot.OnlineParticipants == 0 || snapshot.StaleParticipants > 0 || snapshot.StaleMeetings > 0 {
			status = "degraded"
		}
	}
	return &PlatformJitsiAggregate{
		ActiveMeetings: snapshot.ActiveMeetings, Meetings24h: snapshot.Meetings24h,
		ParticipantMinutes24h: snapshot.ParticipantMinutes24h,
		OnlineParticipants:    snapshot.OnlineParticipants, StaleParticipants: snapshot.StaleParticipants,
		StaleMeetings: snapshot.StaleMeetings, HeartbeatStatus: status,
		HeartbeatTimeoutSeconds: int64(meetingParticipantStaleAfter / time.Second),
		LastMeetingAt:           snapshot.LastMeetingAt, LastHeartbeatAt: snapshot.LastHeartbeatAt,
	}
}

func applyPlatformJitsiServiceHealth(aggregate *PlatformJitsiAggregate, health providers.JitsiServiceHealth) {
	if aggregate == nil {
		return
	}
	checkedAt := health.CheckedAt
	aggregate.ServiceStatus = health.Status
	aggregate.ServiceURL = health.ServiceURL
	aggregate.ProbeURL = health.ProbeURL
	aggregate.ProbeHTTPStatus = health.HTTPStatus
	aggregate.ProbeLatencyMs = health.Latency.Milliseconds()
	aggregate.ProbeCheckedAt = &checkedAt
	aggregate.ProbeError = health.Error
}
