package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/alicebob/miniredis/v2"
)

func TestRealtimeBrokerPublishesAcrossServiceInstances(t *testing.T) {
	redisServer := miniredis.RunT(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	redisConfig := config.RedisConfig{Addr: redisServer.Addr()}
	source := newWsService()
	target := newWsService()
	source.startBrokerWithConfig(ctx, redisConfig)
	target.startBrokerWithConfig(ctx, redisConfig)
	t.Cleanup(func() {
		_ = source.broker.client.Close()
		_ = target.broker.client.Close()
	})

	topic := source.conversationTopic(42)
	targetSession := &ClientSession{
		ID:     "target-session",
		Role:   realtimeRoleAdmin,
		Topics: make(map[string]struct{}),
		Send:   make(chan []byte, 2),
	}
	target.manager.Register(targetSession, []string{topic})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		counts, err := source.broker.client.PubSubNumSub(ctx, realtimeBrokerChannel).Result()
		if err == nil && counts[realtimeBrokerChannel] >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	event := source.newEvent(topic, RealtimeResyncRequiredEvent{
		Payload: RealtimeResyncRequiredPayload{Reason: enums.IMRealtimeResyncReasonManual},
	})
	source.PublishToTopic(topic, event)

	select {
	case payload := <-targetSession.Send:
		var received map[string]any
		if err := json.Unmarshal(payload, &received); err != nil {
			t.Fatalf("decode broker event: %v", err)
		}
		if received["eventId"] != event.EventID || received["type"] != event.Type {
			t.Fatalf("unexpected broker event: %s", payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for cross-instance realtime event")
	}
}

func TestRealtimeBrokerUnavailableKeepsLocalConversationEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	service := newWsService()
	service.startBrokerWithConfig(ctx, config.RedisConfig{Addr: "127.0.0.1:1"})
	t.Cleanup(func() {
		if service.broker.client != nil {
			_ = service.broker.client.Close()
		}
	})

	conversationID := int64(88)
	topic := service.conversationTopic(conversationID)
	observer := &ClientSession{
		ID: "local-observer", Role: realtimeRoleAdmin,
		Principal: &dto.AuthPrincipal{UserID: 99, TenantID: 1, DomainType: models.DomainTypeEnterprise},
		Topics:    make(map[string]struct{}), Send: make(chan []byte, 8),
	}
	actor := &ClientSession{
		ID: "local-actor", Role: realtimeRoleAdmin,
		Principal: &dto.AuthPrincipal{UserID: 25, TenantID: 1, DomainType: models.DomainTypeEnterprise, Nickname: "本机工程师"},
		Topics:    make(map[string]struct{}), Send: make(chan []byte, 8),
	}
	service.manager.Register(observer, []string{topic})
	service.manager.Register(actor, []string{topic})

	event := service.newEvent(topic, RealtimeResyncRequiredEvent{Payload: RealtimeResyncRequiredPayload{Reason: enums.IMRealtimeResyncReasonManual}})
	service.PublishToTopic(topic, event)
	select {
	case raw := <-observer.Send:
		var received struct {
			EventID string `json:"eventId"`
			Type    string `json:"type"`
		}
		if err := json.Unmarshal(raw, &received); err != nil || received.EventID != event.EventID {
			t.Fatalf("local event while Redis is unavailable = %+v err=%v", received, err)
		}
	case <-time.After(time.Second):
		t.Fatal("Redis failure blocked local realtime event delivery")
	}
	drainRealtimeEvents(observer.Send)
	service.publishSessionPresence(actor, conversationID, true)
	select {
	case raw := <-observer.Send:
		var received struct {
			Type string                         `json:"type"`
			Data RealtimePresenceChangedPayload `json:"data"`
		}
		if err := json.Unmarshal(raw, &received); err != nil || received.Type != enums.IMRealtimeEventPresenceChanged || !received.Data.Online {
			t.Fatalf("local presence while Redis is unavailable = %+v err=%v", received, err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Redis failure blocked local presence delivery")
	}
}

func TestRealtimeBrokerRecoversCrossInstanceDeliveryAfterRedisRestart(t *testing.T) {
	redisServer := miniredis.RunT(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	redisConfig := config.RedisConfig{Addr: redisServer.Addr()}
	source := newWsService()
	target := newWsService()
	source.startBrokerWithConfig(ctx, redisConfig)
	target.startBrokerWithConfig(ctx, redisConfig)
	t.Cleanup(func() {
		_ = source.broker.client.Close()
		_ = target.broker.client.Close()
	})
	waitRealtimeBrokerSubscribers(t, ctx, source, 2)

	topic := source.conversationTopic(99)
	localObserver := &ClientSession{ID: "source-local", Role: realtimeRoleAdmin, Topics: make(map[string]struct{}), Send: make(chan []byte, 8)}
	remoteObserver := &ClientSession{ID: "target-remote", Role: realtimeRoleAdmin, Topics: make(map[string]struct{}), Send: make(chan []byte, 8)}
	source.manager.Register(localObserver, []string{topic})
	target.manager.Register(remoteObserver, []string{topic})

	redisServer.Close()
	outageEvent := source.newEvent(topic, RealtimeResyncRequiredEvent{Payload: RealtimeResyncRequiredPayload{Reason: enums.IMRealtimeResyncReasonManual}})
	source.PublishToTopic(topic, outageEvent)
	select {
	case raw := <-localObserver.Send:
		var received struct {
			EventID string `json:"eventId"`
		}
		if err := json.Unmarshal(raw, &received); err != nil || received.EventID != outageEvent.EventID {
			t.Fatalf("local outage event = %+v err=%v", received, err)
		}
	case <-time.After(time.Second):
		t.Fatal("Redis restart interrupted local event delivery")
	}
	select {
	case raw := <-remoteObserver.Send:
		t.Fatalf("cross-instance outage event unexpectedly crossed unavailable Redis: %s", raw)
	case <-time.After(200 * time.Millisecond):
	}

	if err := redisServer.Restart(); err != nil {
		t.Fatalf("restart Redis: %v", err)
	}
	waitRealtimeBrokerSubscribers(t, ctx, source, 2)
	recovered := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && !recovered {
		recoveryEvent := source.newEvent(topic, RealtimeResyncRequiredEvent{Payload: RealtimeResyncRequiredPayload{Reason: enums.IMRealtimeResyncReasonManual}})
		source.PublishToTopic(topic, recoveryEvent)
		select {
		case raw := <-remoteObserver.Send:
			var received struct {
				EventID string `json:"eventId"`
			}
			if err := json.Unmarshal(raw, &received); err == nil && received.EventID == recoveryEvent.EventID {
				recovered = true
			}
		case <-time.After(100 * time.Millisecond):
		}
	}
	if !recovered {
		t.Fatal("cross-instance delivery did not recover after Redis restarted")
	}
}

func TestRealtimeBrokerKeepsActorOnlineUntilFinalCrossInstanceSessionCloses(t *testing.T) {
	redisServer := miniredis.RunT(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	redisConfig := config.RedisConfig{Addr: redisServer.Addr()}
	source := newWsService()
	target := newWsService()
	source.startBrokerWithConfig(ctx, redisConfig)
	target.startBrokerWithConfig(ctx, redisConfig)
	t.Cleanup(func() {
		_ = source.broker.client.Close()
		_ = target.broker.client.Close()
	})
	waitRealtimeBrokerSubscribers(t, ctx, source, 2)

	conversationID := int64(77)
	topic := source.conversationTopic(conversationID)
	newActorSession := func(id string) *ClientSession {
		return &ClientSession{
			ID:   id,
			Role: realtimeRoleAdmin,
			Principal: &dto.AuthPrincipal{
				UserID: 25, TenantID: 1, DomainType: models.DomainTypeEnterprise, Nickname: "同一工程师",
			},
			Topics: make(map[string]struct{}),
			Send:   make(chan []byte, 8),
		}
	}
	actorOnSource := newActorSession("actor-source")
	actorOnTarget := newActorSession("actor-target")
	source.manager.Register(actorOnSource, []string{topic})
	target.manager.Register(actorOnTarget, []string{topic})
	source.publishSessionPresence(actorOnSource, conversationID, true)
	target.publishSessionPresence(actorOnTarget, conversationID, true)

	observer := &ClientSession{
		ID: "observer", Role: realtimeRoleAdmin,
		Principal: &dto.AuthPrincipal{UserID: 99, TenantID: 1, DomainType: models.DomainTypeEnterprise},
		Topics:    make(map[string]struct{}), Send: make(chan []byte, 8),
	}
	source.manager.Register(observer, []string{topic})
	time.Sleep(100 * time.Millisecond)
	drainRealtimeEvents(observer.Send)

	source.manager.Unregister(actorOnSource)
	source.publishSessionPresence(actorOnSource, conversationID, false)
	select {
	case payload := <-observer.Send:
		t.Fatalf("closing one cross-instance session published a false offline event: %s", payload)
	case <-time.After(200 * time.Millisecond):
	}

	target.manager.Unregister(actorOnTarget)
	target.publishSessionPresence(actorOnTarget, conversationID, false)
	select {
	case payload := <-observer.Send:
		var event struct {
			Type string                         `json:"type"`
			Data RealtimePresenceChangedPayload `json:"data"`
		}
		if err := json.Unmarshal(payload, &event); err != nil {
			t.Fatalf("decode final cross-instance offline event: %v", err)
		}
		if event.Type != enums.IMRealtimeEventPresenceChanged || event.Data.ActorID != "user:25" || event.Data.Online {
			t.Fatalf("unexpected final cross-instance offline event: %+v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("closing the final cross-instance session did not publish offline presence")
	}
}

func waitRealtimeBrokerSubscribers(t *testing.T, ctx context.Context, service *wsService, want int64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		counts, err := service.broker.client.PubSubNumSub(ctx, realtimeBrokerChannel).Result()
		if err == nil && counts[realtimeBrokerChannel] >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("realtime broker subscribers did not reach %d", want)
}
