package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"remotehelpdesk/internal/pkg/cache"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"

	"github.com/go-redis/redis/v8"
	"github.com/mlogclub/simple/web"
)

const (
	NotificationJobCreate               = "notification.create"
	DefaultNotificationDeadLetterStream = "remotehelpdesk:notifications:dead:v1"
)

type NotificationQueueHandler func(ctx context.Context, payload json.RawMessage) error

type NotificationQueueJob struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Payload    json.RawMessage `json:"payload"`
	EnqueuedAt time.Time       `json:"enqueued_at"`
}

type notificationJobContextKey struct{}

type notificationQueueService struct {
	client           *redis.Client
	stream           string
	group            string
	consumer         string
	deadLetterStream string
	maxStreamLength  int64
	maxDeliveries    int64
	batchSize        int64
	blockTimeout     time.Duration
	reclaimIdle      time.Duration
	reclaimInterval  time.Duration

	handlerMu sync.RWMutex
	handlers  map[string]NotificationQueueHandler
	startMu   sync.Mutex
	started   bool
	workerWG  sync.WaitGroup
}

var NotificationQueueService = newNotificationQueueService(nil, config.RedisConfig{})

func newNotificationQueueService(client *redis.Client, redisConfig config.RedisConfig) *notificationQueueService {
	hostname, _ := os.Hostname()
	if strings.TrimSpace(hostname) == "" {
		hostname = "worker"
	}
	service := &notificationQueueService{
		client:           client,
		stream:           redisConfig.NotificationStreamOrDefault(),
		group:            redisConfig.NotificationConsumerGroupOrDefault(),
		consumer:         fmt.Sprintf("%s-%d-%s", hostname, os.Getpid(), strings.ReplaceAll(utils.UUID(), "-", "")),
		deadLetterStream: DefaultNotificationDeadLetterStream,
		maxStreamLength:  10000,
		maxDeliveries:    3,
		batchSize:        20,
		blockTimeout:     5 * time.Second,
		reclaimIdle:      30 * time.Second,
		reclaimInterval:  10 * time.Second,
		handlers:         make(map[string]NotificationQueueHandler),
	}
	service.Register(NotificationJobCreate, func(ctx context.Context, payload json.RawMessage) error {
		var req request.CreateNotificationRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			return fmt.Errorf("decode notification create request: %w", err)
		}
		if req.IdempotencyKey == "" {
			req.IdempotencyKey = NotificationJobIDFromContext(ctx)
		}
		item, err := NotificationService.CreateAndPush(req)
		if err != nil {
			if isNonRetryableNotificationCreateError(err) {
				slog.Info("notification queue create job skipped", "jobId", NotificationJobIDFromContext(ctx), "error", err)
				return nil
			}
			return err
		}
		return NotificationDeliveryService.Schedule(item)
	})
	return service
}

func isNonRetryableNotificationCreateError(err error) bool {
	if err == nil {
		return false
	}
	var codeErr *web.CodeError
	if !errors.As(err, &codeErr) {
		return false
	}
	return codeErr.Code == errorsx.CodeAuthForbidden &&
		strings.Contains(strings.ToLower(codeErr.Message), "notification recipient is outside the tenant or inactive")
}

func NewNotificationQueueService(client *redis.Client, redisConfig config.RedisConfig) *notificationQueueService {
	return newNotificationQueueService(client, redisConfig)
}

func (s *notificationQueueService) Register(jobType string, handler NotificationQueueHandler) {
	jobType = strings.TrimSpace(jobType)
	if jobType == "" || handler == nil {
		panic("notification queue: job type and handler are required")
	}
	s.handlerMu.Lock()
	defer s.handlerMu.Unlock()
	if _, exists := s.handlers[jobType]; exists {
		panic("notification queue: duplicate handler for " + jobType)
	}
	s.handlers[jobType] = handler
}

func (s *notificationQueueService) Enqueue(ctx context.Context, jobType string, payload any) (string, error) {
	s.configure()
	jobType = strings.TrimSpace(jobType)
	if jobType == "" {
		return "", errors.New("notification queue: job type is required")
	}
	if !s.hasHandler(jobType) {
		return "", fmt.Errorf("notification queue: no handler registered for %s", jobType)
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("notification queue: marshal payload: %w", err)
	}
	job := NotificationQueueJob{
		ID:         utils.UUID(),
		Type:       jobType,
		Payload:    payloadJSON,
		EnqueuedAt: time.Now().UTC(),
	}
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return "", fmt.Errorf("notification queue: marshal job: %w", err)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_, err = s.client.XAdd(ctx, &redis.XAddArgs{
		Stream: s.stream,
		MaxLen: s.maxStreamLength,
		Approx: true,
		Values: map[string]any{"job": string(jobJSON)},
	}).Result()
	if err != nil {
		return "", fmt.Errorf("notification queue: enqueue: %w", err)
	}
	return job.ID, nil
}

func (s *notificationQueueService) EnqueueCreate(ctx context.Context, req request.CreateNotificationRequest) (string, error) {
	return s.Enqueue(ctx, NotificationJobCreate, req)
}

func (s *notificationQueueService) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	s.configure()
	if err := s.ensureConsumerGroup(ctx); err != nil {
		return err
	}
	s.startMu.Lock()
	defer s.startMu.Unlock()
	if s.started {
		return nil
	}
	s.started = true
	s.workerWG.Add(1)
	go func() {
		defer s.workerWG.Done()
		s.consume(ctx)
	}()
	slog.Info("notification queue consumer started", "stream", s.stream, "group", s.group, "consumer", s.consumer)
	return nil
}

func (s *notificationQueueService) Wait() {
	s.workerWG.Wait()
}

func (s *notificationQueueService) configure() {
	s.startMu.Lock()
	defer s.startMu.Unlock()
	if s.client != nil {
		return
	}
	redisConfig := config.CurrentOrDefault().Redis
	s.client = cache.Client()
	s.stream = redisConfig.NotificationStreamOrDefault()
	s.group = redisConfig.NotificationConsumerGroupOrDefault()
}

func (s *notificationQueueService) ensureConsumerGroup(ctx context.Context) error {
	err := s.client.XGroupCreateMkStream(ctx, s.stream, s.group, "0").Err()
	if err == nil || strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return fmt.Errorf("notification queue: create consumer group: %w", err)
}

func (s *notificationQueueService) consume(ctx context.Context) {
	reclaimTicker := time.NewTicker(s.reclaimInterval)
	defer reclaimTicker.Stop()
	if err := s.reclaimPending(ctx); err != nil && ctx.Err() == nil {
		slog.Warn("notification queue initial reclaim failed", "error", err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-reclaimTicker.C:
			if err := s.reclaimPending(ctx); err != nil && ctx.Err() == nil {
				slog.Warn("notification queue reclaim failed", "error", err)
			}
		default:
		}

		streams, err := s.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    s.group,
			Consumer: s.consumer,
			Streams:  []string{s.stream, ">"},
			Count:    s.batchSize,
			Block:    s.blockTimeout,
		}).Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("notification queue read failed", "error", err)
			if !waitNotificationQueue(ctx, time.Second) {
				return
			}
			continue
		}
		for _, stream := range streams {
			for _, message := range stream.Messages {
				s.processMessage(ctx, message)
			}
		}
	}
}

func (s *notificationQueueService) reclaimPending(ctx context.Context) error {
	for {
		pending, err := s.client.XPendingExt(ctx, &redis.XPendingExtArgs{
			Stream: s.stream,
			Group:  s.group,
			Idle:   s.reclaimIdle,
			Start:  "-",
			End:    "+",
			Count:  s.batchSize,
		}).Result()
		if err == redis.Nil {
			return nil
		}
		if err != nil {
			return err
		}
		if len(pending) == 0 {
			return nil
		}
		messageIDs := make([]string, 0, len(pending))
		for _, item := range pending {
			messageIDs = append(messageIDs, item.ID)
		}
		messages, err := s.client.XClaim(ctx, &redis.XClaimArgs{
			Stream:   s.stream,
			Group:    s.group,
			Consumer: s.consumer,
			MinIdle:  s.reclaimIdle,
			Messages: messageIDs,
		}).Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			return err
		}
		for _, message := range messages {
			s.processMessage(ctx, message)
		}
		if int64(len(pending)) < s.batchSize {
			return nil
		}
	}
}

func (s *notificationQueueService) processMessage(ctx context.Context, message redis.XMessage) {
	job, raw, err := decodeNotificationQueueJob(message)
	if err != nil {
		s.deadLetterAndAck(ctx, message.ID, raw, err)
		return
	}
	handler := s.handler(job.Type)
	if handler == nil {
		s.deadLetterAndAck(ctx, message.ID, raw, fmt.Errorf("no handler registered for %s", job.Type))
		return
	}
	jobCtx := context.WithValue(ctx, notificationJobContextKey{}, job.ID)
	if err := callNotificationQueueHandler(jobCtx, handler, job.Payload); err != nil {
		deliveries, countErr := s.deliveryCount(ctx, message.ID)
		if countErr != nil {
			slog.Error("notification queue delivery count failed", "messageId", message.ID, "jobType", job.Type, "error", countErr)
			return
		}
		if deliveries >= s.maxDeliveries {
			s.deadLetterAndAck(ctx, message.ID, raw, err)
			return
		}
		slog.Warn("notification queue job failed and remains pending", "messageId", message.ID, "jobId", job.ID, "jobType", job.Type, "delivery", deliveries, "error", err)
		return
	}
	if err := s.client.XAck(ctx, s.stream, s.group, message.ID).Err(); err != nil {
		slog.Error("notification queue ack failed", "messageId", message.ID, "jobId", job.ID, "jobType", job.Type, "error", err)
	}
}

func (s *notificationQueueService) deliveryCount(ctx context.Context, messageID string) (int64, error) {
	items, err := s.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: s.stream,
		Group:  s.group,
		Start:  messageID,
		End:    messageID,
		Count:  1,
	}).Result()
	if err != nil {
		return 0, err
	}
	if len(items) == 0 || items[0].ID != messageID {
		return 1, nil
	}
	return items[0].RetryCount, nil
}

func (s *notificationQueueService) deadLetterAndAck(ctx context.Context, messageID, raw string, cause error) {
	_, err := s.client.XAdd(ctx, &redis.XAddArgs{
		Stream: s.deadLetterStream,
		MaxLen: s.maxStreamLength,
		Approx: true,
		Values: map[string]any{
			"source_stream":     s.stream,
			"source_message_id": messageID,
			"job":               raw,
			"error":             cause.Error(),
			"failed_at":         time.Now().UTC().Format(time.RFC3339Nano),
		},
	}).Result()
	if err != nil {
		slog.Error("notification queue dead letter write failed", "messageId", messageID, "cause", cause, "error", err)
		return
	}
	if err := s.client.XAck(ctx, s.stream, s.group, messageID).Err(); err != nil {
		slog.Error("notification queue dead letter ack failed", "messageId", messageID, "error", err)
		return
	}
	slog.Error("notification queue job moved to dead letter", "messageId", messageID, "error", cause)
}

func (s *notificationQueueService) hasHandler(jobType string) bool {
	return s.handler(jobType) != nil
}

func (s *notificationQueueService) handler(jobType string) NotificationQueueHandler {
	s.handlerMu.RLock()
	defer s.handlerMu.RUnlock()
	return s.handlers[jobType]
}

func decodeNotificationQueueJob(message redis.XMessage) (NotificationQueueJob, string, error) {
	value, ok := message.Values["job"]
	if !ok {
		return NotificationQueueJob{}, "", errors.New("notification queue: message has no job field")
	}
	var raw string
	switch typed := value.(type) {
	case string:
		raw = typed
	case []byte:
		raw = string(typed)
	default:
		raw = fmt.Sprint(typed)
	}
	var job NotificationQueueJob
	if err := json.Unmarshal([]byte(raw), &job); err != nil {
		return NotificationQueueJob{}, raw, fmt.Errorf("notification queue: decode job: %w", err)
	}
	if strings.TrimSpace(job.ID) == "" || strings.TrimSpace(job.Type) == "" {
		return NotificationQueueJob{}, raw, errors.New("notification queue: job id and type are required")
	}
	return job, raw, nil
}

func callNotificationQueueHandler(ctx context.Context, handler NotificationQueueHandler, payload json.RawMessage) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("notification queue handler panic: %v\n%s", recovered, debug.Stack())
		}
	}()
	return handler(ctx, payload)
}

func NotificationJobIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(notificationJobContextKey{}).(string)
	return strings.TrimSpace(value)
}

func waitNotificationQueue(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
