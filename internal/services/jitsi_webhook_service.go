package services

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

const (
	jitsiWebhookInboxLockTTL     = time.Minute
	jitsiWebhookInboxMaxAttempts = 12
	jitsiWebhookRetryBase        = 5 * time.Second
	jitsiWebhookRetryMax         = 5 * time.Minute
)

var JitsiWebhookService = newJitsiWebhookService()

var errJitsiWebhookEventCollision = errors.New("jitsi webhook event id was reused with a different payload")

type jitsiWebhookService struct{}

type JitsiWebhookProcessInput struct {
	EventID         string
	Action          string
	PayloadHash     string
	MeetingID       string
	RoomName        string
	ParticipantID   string
	ParticipantName string
	OccurredAt      time.Time
	ReceivedAt      time.Time
}

func newJitsiWebhookService() *jitsiWebhookService {
	return &jitsiWebhookService{}
}

// Process claims the provider event in a durable inbox before applying its
// business effect. A duplicate returns duplicate=true without replaying it.
func (s *jitsiWebhookService) Process(ctx context.Context, input JitsiWebhookProcessInput) (duplicate bool, err error) {
	input.EventID = strings.TrimSpace(input.EventID)
	input.Action = strings.TrimSpace(input.Action)
	input.PayloadHash = strings.TrimSpace(input.PayloadHash)
	input.MeetingID = strings.TrimSpace(input.MeetingID)
	input.RoomName = strings.TrimSpace(input.RoomName)
	input.ParticipantID = strings.TrimSpace(input.ParticipantID)
	input.ParticipantName = strings.TrimSpace(input.ParticipantName)
	if input.EventID == "" || input.Action == "" || input.PayloadHash == "" {
		return false, errors.New("jitsi webhook event id, action and payload hash are required")
	}
	if input.ReceivedAt.IsZero() {
		input.ReceivedAt = time.Now()
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = input.ReceivedAt
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return false, err
	}

	item, claimed, err := repositories.WebhookEventInboxRepository.Claim(sqls.DB(), &models.WebhookEventInbox{
		Provider:    "jitsi",
		EventID:     input.EventID,
		EventType:   input.Action,
		PayloadHash: input.PayloadHash,
		PayloadJSON: string(payload),
		OccurredAt:  &input.OccurredAt,
		ReceivedAt:  input.ReceivedAt,
	}, input.ReceivedAt, jitsiWebhookInboxLockTTL)
	if err != nil {
		return false, err
	}
	if item.PayloadHash != "" && item.PayloadHash != input.PayloadHash {
		return false, errJitsiWebhookEventCollision
	}
	if !claimed {
		return true, nil
	}

	processErr := s.apply(ctx, input)
	now := time.Now()
	if processErr != nil {
		_ = repositories.WebhookEventInboxRepository.MarkFailed(
			sqls.DB(), item.ID, processErr.Error(), now, now.Add(jitsiWebhookRetryDelay(item.AttemptCount)),
		)
		return false, processErr
	}
	if err := repositories.WebhookEventInboxRepository.MarkProcessed(sqls.DB(), item.ID, now); err != nil {
		return false, err
	}
	return false, nil
}

// RecoverPending replays failed or abandoned callbacks from their durable
// inbox payload. Business handlers are idempotent, so a crash after applying
// the effect but before MarkProcessed is also safe to recover.
func (s *jitsiWebhookService) RecoverPending(ctx context.Context, limit int) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	now := time.Now()
	items, err := repositories.WebhookEventInboxRepository.ClaimRetryable(
		sqls.DB(), now, jitsiWebhookInboxLockTTL, jitsiWebhookInboxMaxAttempts, limit,
	)
	if err != nil {
		return 0, err
	}
	processed := 0
	for i := range items {
		var input JitsiWebhookProcessInput
		if err := json.Unmarshal([]byte(items[i].PayloadJSON), &input); err != nil {
			retryAt := now.Add(jitsiWebhookRetryDelay(items[i].AttemptCount))
			_ = repositories.WebhookEventInboxRepository.MarkFailed(sqls.DB(), items[i].ID, "decode replay payload: "+err.Error(), now, retryAt)
			continue
		}
		input.EventID = items[i].EventID
		input.Action = items[i].EventType
		input.PayloadHash = items[i].PayloadHash
		input.ReceivedAt = items[i].ReceivedAt
		if items[i].OccurredAt != nil {
			input.OccurredAt = *items[i].OccurredAt
		}
		if err := s.apply(ctx, input); err != nil {
			retryAt := now.Add(jitsiWebhookRetryDelay(items[i].AttemptCount))
			_ = repositories.WebhookEventInboxRepository.MarkFailed(sqls.DB(), items[i].ID, err.Error(), now, retryAt)
			slog.Warn("replay jitsi webhook failed", "eventId", items[i].EventID, "attempt", items[i].AttemptCount, "error", err)
			continue
		}
		if err := repositories.WebhookEventInboxRepository.MarkProcessed(sqls.DB(), items[i].ID, now); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func jitsiWebhookRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := jitsiWebhookRetryBase
	for i := 1; i < attempt && delay < jitsiWebhookRetryMax; i++ {
		delay *= 2
		if delay >= jitsiWebhookRetryMax {
			return jitsiWebhookRetryMax
		}
	}
	return delay
}

func (s *jitsiWebhookService) apply(ctx context.Context, input JitsiWebhookProcessInput) error {
	switch input.Action {
	case "meeting.ended":
		return MeetingService.EndMeetingFromWebhookAt(ctx, input.MeetingID, input.RoomName, input.OccurredAt)
	case "participant.joined":
		return MeetingService.RecordParticipantJoinFromWebhook(ctx, input.MeetingID, input.RoomName, input.ParticipantID, input.ParticipantName, input.OccurredAt)
	case "participant.left":
		return MeetingService.RecordParticipantLeftFromWebhook(ctx, input.MeetingID, input.RoomName, input.ParticipantID, input.OccurredAt)
	default:
		return nil
	}
}
