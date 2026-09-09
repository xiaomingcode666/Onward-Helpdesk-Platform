package services

import (
	"testing"
	"time"

	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/repositories"
)

func TestBuildPlatformJitsiAggregateHeartbeatStatus(t *testing.T) {
	idle := buildPlatformJitsiAggregate(&repositories.PlatformJitsiSnapshot{})
	if idle.HeartbeatStatus != "idle" || idle.HeartbeatTimeoutSeconds != 180 {
		t.Fatalf("idle heartbeat aggregate = %#v", idle)
	}
	healthy := buildPlatformJitsiAggregate(&repositories.PlatformJitsiSnapshot{
		ActiveMeetings: 1, OnlineParticipants: 2,
	})
	if healthy.HeartbeatStatus != "healthy" {
		t.Fatalf("healthy heartbeat aggregate = %#v", healthy)
	}
	degraded := buildPlatformJitsiAggregate(&repositories.PlatformJitsiSnapshot{
		ActiveMeetings: 1, OnlineParticipants: 1, StaleParticipants: 1, StaleMeetings: 1,
	})
	if degraded.HeartbeatStatus != "degraded" {
		t.Fatalf("degraded heartbeat aggregate = %#v", degraded)
	}
}

func TestApplyPlatformJitsiServiceHealth(t *testing.T) {
	checkedAt := time.Now()
	aggregate := &PlatformJitsiAggregate{}
	applyPlatformJitsiServiceHealth(aggregate, providers.JitsiServiceHealth{
		Status: "healthy", ServiceURL: "https://meet.example.com",
		ProbeURL: "https://meet.example.com/config.js", HTTPStatus: 200,
		Latency: 42 * time.Millisecond, CheckedAt: checkedAt,
	})
	if aggregate.ServiceStatus != "healthy" || aggregate.ProbeHTTPStatus != 200 || aggregate.ProbeLatencyMs != 42 ||
		aggregate.ProbeCheckedAt == nil || !aggregate.ProbeCheckedAt.Equal(checkedAt) {
		t.Fatalf("Jitsi service health aggregate = %#v", aggregate)
	}
}
