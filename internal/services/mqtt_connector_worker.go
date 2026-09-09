package services

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/secretstore"
	"remotehelpdesk/internal/pkg/utils"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	mqttWorkerKey              = "mqtt-subscription"
	mqttHealthCursorName       = "mqtt_worker_health"
	mqttSupervisorPollInterval = 5 * time.Second
	mqttLeaseTTL               = 30 * time.Second
	mqttLeaseRenewInterval     = 10 * time.Second
	mqttRetryBaseDelay         = 5 * time.Second
	mqttRetryMaxDelay          = time.Minute
	mqttRetryResetAfter        = 2 * time.Minute
	mqttDefaultQueueCapacity   = 256
	mqttDefaultKeepAlive       = 60 * time.Second
	mqttConnectTimeout         = 15 * time.Second
	mqttOperationTimeout       = 10 * time.Second
	mqttDisconnectQuiesceMs    = 500
)

var errMQTTConnectorLeaseHeld = errors.New("MQTT connector lease is held by another worker")

// StartMQTTConnectorWorkers starts the process-local MQTT supervisor and returns immediately.
func StartMQTTConnectorWorkers(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	db := sqls.DB()
	owner := mqttWorkerOwner()
	leases := NewDBConnectorLeaseStore(db)
	cursors := newDBConnectorCursorStore(db)
	health := newDBMQTTWorkerHealthStore(db, cursors)
	adapter := newMQTTConnectorAdapter(pahoMQTTClientFactory{}, health)
	runner := &leasedMQTTConnectorRunner{
		owner:         owner,
		workerKey:     mqttWorkerKey,
		leaseTTL:      mqttLeaseTTL,
		renewInterval: mqttLeaseRenewInterval,
		leases:        leases,
		health:        health,
		adapter:       adapter,
		runtime: ConnectorWorkerRuntime{
			WorkerID: owner,
			Sink:     MQTTIngestService,
			Cursors:  cursors,
			Leases:   leases,
		},
	}
	supervisor := newMQTTConnectorSupervisor(
		newDBMQTTConnectorSource(db),
		runner,
		mqttSupervisorPollInterval,
	)
	go supervisor.Run(ctx)
}

func mqttWorkerOwner() string {
	hostname, _ := os.Hostname()
	if strings.TrimSpace(hostname) == "" {
		hostname = "worker"
	}
	return fmt.Sprintf("%s-%d-%s", hostname, os.Getpid(), strings.ReplaceAll(utils.UUID(), "-", ""))
}

type mqttConnectorSource interface {
	ListActiveMQTTConnectors(ctx context.Context) ([]models.AccessConnector, error)
}

type dbMQTTConnectorSource struct {
	db *gorm.DB
}

func newDBMQTTConnectorSource(db *gorm.DB) *dbMQTTConnectorSource {
	return &dbMQTTConnectorSource{db: db}
}

func (s *dbMQTTConnectorSource) ListActiveMQTTConnectors(ctx context.Context) ([]models.AccessConnector, error) {
	var connectors []models.AccessConnector
	err := s.db.WithContext(ctx).
		Where("connector_type = ? AND status = ?", models.ConnectorTypeMQTT, "active").
		Order("tenant_id ASC, id ASC").
		Find(&connectors).Error
	if err != nil {
		return nil, fmt.Errorf("list active MQTT connectors: %w", err)
	}
	return connectors, nil
}

type mqttConnectorRunner interface {
	Run(ctx context.Context, connector *models.AccessConnector) error
}

type mqttConnectorKey struct {
	TenantID    int64
	ConnectorID int64
}

type supervisedMQTTWorker struct {
	fingerprint string
	startedAt   time.Time
	cancel      context.CancelFunc
	done        chan struct{}
	result      chan error
}

type mqttConnectorRetryState struct {
	fingerprint string
	failures    int
	retryAt     time.Time
}

type mqttConnectorSupervisor struct {
	source       mqttConnectorSource
	runner       mqttConnectorRunner
	pollInterval time.Duration
	retryBase    time.Duration
	retryMax     time.Duration
	retryReset   time.Duration
	now          func() time.Time
	workers      map[mqttConnectorKey]*supervisedMQTTWorker
	retries      map[mqttConnectorKey]mqttConnectorRetryState
}

func newMQTTConnectorSupervisor(source mqttConnectorSource, runner mqttConnectorRunner, pollInterval time.Duration) *mqttConnectorSupervisor {
	if pollInterval <= 0 {
		pollInterval = mqttSupervisorPollInterval
	}
	return &mqttConnectorSupervisor{
		source:       source,
		runner:       runner,
		pollInterval: pollInterval,
		retryBase:    mqttRetryBaseDelay,
		retryMax:     mqttRetryMaxDelay,
		retryReset:   mqttRetryResetAfter,
		now:          time.Now,
		workers:      make(map[mqttConnectorKey]*supervisedMQTTWorker),
		retries:      make(map[mqttConnectorKey]mqttConnectorRetryState),
	}
}

func (s *mqttConnectorSupervisor) Run(ctx context.Context) {
	slog.Info("MQTT connector supervisor started")
	defer slog.Info("MQTT connector supervisor stopped")
	if err := s.reconcile(ctx); err != nil {
		slog.Error("MQTT connector supervisor initial reconcile failed", "error", err)
	}
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.stopAll()
			return
		case <-ticker.C:
			if err := s.reconcile(ctx); err != nil {
				slog.Error("MQTT connector supervisor reconcile failed", "error", err)
			}
		}
	}
}

func (s *mqttConnectorSupervisor) reconcile(ctx context.Context) error {
	connectors, err := s.source.ListActiveMQTTConnectors(ctx)
	if err != nil {
		return err
	}
	desired := make(map[mqttConnectorKey]models.AccessConnector, len(connectors))
	for i := range connectors {
		connector := connectors[i]
		if connector.TenantID <= 0 || connector.ID <= 0 {
			continue
		}
		key := mqttConnectorKey{TenantID: connector.TenantID, ConnectorID: connector.ID}
		desired[key] = connector
	}

	for key, worker := range s.workers {
		connector, active := desired[key]
		select {
		case <-worker.done:
			delete(s.workers, key)
			runErr := <-worker.result
			if active && worker.fingerprint == mqttConnectorFingerprint(&connector) {
				s.recordFailure(key, worker, runErr)
			} else {
				delete(s.retries, key)
			}
			continue
		default:
		}
		if !active || worker.fingerprint != mqttConnectorFingerprint(&connector) {
			worker.cancel()
			<-worker.done
			<-worker.result
			delete(s.workers, key)
			delete(s.retries, key)
		}
	}
	for key, connector := range desired {
		if _, exists := s.workers[key]; exists {
			continue
		}
		fingerprint := mqttConnectorFingerprint(&connector)
		if retry, exists := s.retries[key]; exists {
			if retry.fingerprint != fingerprint {
				delete(s.retries, key)
			} else if s.now().UTC().Before(retry.retryAt) {
				continue
			}
		}
		s.start(ctx, key, connector)
	}
	return nil
}

func (s *mqttConnectorSupervisor) start(parent context.Context, key mqttConnectorKey, connector models.AccessConnector) {
	workerCtx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	result := make(chan error, 1)
	worker := &supervisedMQTTWorker{
		fingerprint: mqttConnectorFingerprint(&connector),
		startedAt:   s.now().UTC(),
		cancel:      cancel,
		done:        done,
		result:      result,
	}
	s.workers[key] = worker
	go func() {
		defer close(done)
		err := s.runner.Run(workerCtx, &connector)
		result <- err
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, errMQTTConnectorLeaseHeld) {
			slog.Error("MQTT connector worker stopped with error", "tenant_id", key.TenantID, "connector_id", key.ConnectorID, "error", sanitizeMQTTError(err))
		}
	}()
}

func (s *mqttConnectorSupervisor) recordFailure(key mqttConnectorKey, worker *supervisedMQTTWorker, runErr error) {
	previous := s.retries[key]
	failures := previous.failures + 1
	if s.retryReset > 0 && s.now().UTC().Sub(worker.startedAt) >= s.retryReset {
		failures = 1
	}
	delay := s.retryBase
	for attempt := 1; attempt < failures && delay < s.retryMax; attempt++ {
		delay *= 2
		if delay > s.retryMax {
			delay = s.retryMax
		}
	}
	if delay <= 0 {
		delay = s.pollInterval
	}
	s.retries[key] = mqttConnectorRetryState{
		fingerprint: worker.fingerprint,
		failures:    failures,
		retryAt:     s.now().UTC().Add(delay),
	}
	if runErr == nil {
		runErr = errors.New("MQTT connector worker exited unexpectedly")
	}
	slog.Warn("MQTT connector worker scheduled for retry",
		"tenant_id", key.TenantID,
		"connector_id", key.ConnectorID,
		"attempt", failures,
		"retry_after", delay,
		"error", sanitizeMQTTError(runErr),
	)
}

func (s *mqttConnectorSupervisor) stopAll() {
	for _, worker := range s.workers {
		worker.cancel()
	}
	for key, worker := range s.workers {
		<-worker.done
		<-worker.result
		delete(s.workers, key)
	}
}

func mqttConnectorFingerprint(connector *models.AccessConnector) string {
	if connector == nil {
		return ""
	}
	material := strings.Join([]string{
		strconv.FormatInt(connector.TenantID, 10),
		strconv.FormatInt(connector.ID, 10),
		connector.BaseURL,
		connector.AuthType,
		connector.AuthConfig,
		connector.FieldMapping,
		connector.TemplateCode,
		connector.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}, "\x00")
	sum := sha256.Sum256([]byte(material))
	return hex.EncodeToString(sum[:])
}

type leasedMQTTConnectorRunner struct {
	owner         string
	workerKey     string
	leaseTTL      time.Duration
	renewInterval time.Duration
	leases        ConnectorLeaseStore
	health        mqttWorkerHealthStore
	adapter       ConnectorAdapter
	runtime       ConnectorWorkerRuntime
}

func (r *leasedMQTTConnectorRunner) Run(ctx context.Context, connector *models.AccessConnector) error {
	if connector == nil {
		return fmt.Errorf("MQTT connector is required")
	}
	fence, acquired, err := r.leases.Acquire(ctx, connector.TenantID, connector.ID, r.workerKey, r.owner, r.leaseTTL)
	if err != nil {
		return fmt.Errorf("acquire MQTT connector lease: %w", err)
	}
	if !acquired {
		return errMQTTConnectorLeaseHeld
	}

	workerCtx, cancel := context.WithCancel(ctx)
	renewDone := make(chan error, 1)
	go r.renewLease(workerCtx, cancel, connector, fence, renewDone)

	adapterErr := r.adapter.Run(workerCtx, connector, r.runtime)
	cancel()
	renewErr := <-renewDone
	releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 3*time.Second)
	releaseErr := r.leases.Release(releaseCtx, connector.TenantID, connector.ID, r.workerKey, r.owner, fence)
	releaseCancel()
	if releaseErr != nil {
		slog.Warn("release MQTT connector lease failed", "tenant_id", connector.TenantID, "connector_id", connector.ID, "error", sanitizeMQTTError(releaseErr))
	}
	if renewErr != nil {
		return renewErr
	}
	return adapterErr
}

func (r *leasedMQTTConnectorRunner) renewLease(ctx context.Context, cancel context.CancelFunc, connector *models.AccessConnector, fence int64, done chan<- error) {
	ticker := time.NewTicker(r.renewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			done <- nil
			return
		case <-ticker.C:
			renewed, err := r.leases.Renew(ctx, connector.TenantID, connector.ID, r.workerKey, r.owner, fence, r.leaseTTL)
			if err == nil && renewed {
				continue
			}
			leaseErr := errors.New("MQTT connector lease renewal was rejected")
			if err != nil {
				leaseErr = fmt.Errorf("renew MQTT connector lease: %w", err)
			}
			healthCtx, healthCancel := context.WithTimeout(context.Background(), 3*time.Second)
			_ = r.health.Update(healthCtx, connector, "degraded", leaseErr, nil)
			healthCancel()
			cancel()
			done <- leaseErr
			return
		}
	}
}

// dbConnectorLeaseStore is the persistent owner/fence/expiry implementation used across app instances.
type dbConnectorLeaseStore struct {
	db  *gorm.DB
	now func() time.Time
}

func NewDBConnectorLeaseStore(db *gorm.DB) ConnectorLeaseStore {
	return &dbConnectorLeaseStore{db: db, now: time.Now}
}

func (s *dbConnectorLeaseStore) Acquire(ctx context.Context, tenantID, connectorID int64, workerKey, owner string, ttl time.Duration) (int64, bool, error) {
	if err := validateLeaseInput(tenantID, connectorID, workerKey, owner, ttl); err != nil {
		return 0, false, err
	}
	now := s.now().UTC()
	expiresAt := now.Add(ttl)
	query := s.db.WithContext(ctx).Model(&models.ConnectorWorkerLease{}).
		Where("tenant_id = ? AND connector_id = ? AND worker_key = ?", tenantID, connectorID, workerKey).
		Where("lease_owner = ? OR lease_expires_at IS NULL OR lease_expires_at <= ?", owner, now).
		Updates(map[string]any{
			"lease_owner":      owner,
			"fence_token":      gorm.Expr("fence_token + 1"),
			"lease_expires_at": &expiresAt,
			"heartbeat_at":     &now,
			"updated_at":       now,
		})
	if query.Error != nil {
		return 0, false, fmt.Errorf("update connector worker lease: %w", query.Error)
	}
	if query.RowsAffected == 0 {
		lease := &models.ConnectorWorkerLease{
			ID:             utils.UUID(),
			TenantID:       tenantID,
			ConnectorID:    connectorID,
			WorkerKey:      workerKey,
			LeaseOwner:     owner,
			FenceToken:     1,
			LeaseExpiresAt: &expiresAt,
			HeartbeatAt:    &now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		created := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(lease)
		if created.Error != nil {
			return 0, false, fmt.Errorf("create connector worker lease: %w", created.Error)
		}
		if created.RowsAffected == 1 {
			return lease.FenceToken, true, nil
		}
		return 0, false, nil
	}
	var lease models.ConnectorWorkerLease
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND connector_id = ? AND worker_key = ? AND lease_owner = ?", tenantID, connectorID, workerKey, owner).
		First(&lease).Error; err != nil {
		return 0, false, fmt.Errorf("load acquired connector worker lease: %w", err)
	}
	return lease.FenceToken, true, nil
}

func (s *dbConnectorLeaseStore) Renew(ctx context.Context, tenantID, connectorID int64, workerKey, owner string, fenceToken int64, ttl time.Duration) (bool, error) {
	if err := validateLeaseInput(tenantID, connectorID, workerKey, owner, ttl); err != nil {
		return false, err
	}
	if fenceToken <= 0 {
		return false, fmt.Errorf("fence token is required")
	}
	now := s.now().UTC()
	expiresAt := now.Add(ttl)
	result := s.db.WithContext(ctx).Model(&models.ConnectorWorkerLease{}).
		Where("tenant_id = ? AND connector_id = ? AND worker_key = ?", tenantID, connectorID, workerKey).
		Where("lease_owner = ? AND fence_token = ? AND lease_expires_at > ?", owner, fenceToken, now).
		Updates(map[string]any{
			"lease_expires_at": &expiresAt,
			"heartbeat_at":     &now,
			"updated_at":       now,
		})
	if result.Error != nil {
		return false, fmt.Errorf("renew connector worker lease: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

func (s *dbConnectorLeaseStore) Release(ctx context.Context, tenantID, connectorID int64, workerKey, owner string, fenceToken int64) error {
	if tenantID <= 0 || connectorID <= 0 || strings.TrimSpace(workerKey) == "" || strings.TrimSpace(owner) == "" || fenceToken <= 0 {
		return fmt.Errorf("tenant, connector, worker key, owner, and fence token are required")
	}
	now := s.now().UTC()
	result := s.db.WithContext(ctx).Model(&models.ConnectorWorkerLease{}).
		Where("tenant_id = ? AND connector_id = ? AND worker_key = ? AND lease_owner = ? AND fence_token = ?", tenantID, connectorID, workerKey, owner, fenceToken).
		Updates(map[string]any{
			"lease_owner":      "",
			"lease_expires_at": nil,
			"heartbeat_at":     &now,
			"updated_at":       now,
		})
	if result.Error != nil {
		return fmt.Errorf("release connector worker lease: %w", result.Error)
	}
	return nil
}

func validateLeaseInput(tenantID, connectorID int64, workerKey, owner string, ttl time.Duration) error {
	if tenantID <= 0 || connectorID <= 0 {
		return fmt.Errorf("tenant and connector are required")
	}
	if strings.TrimSpace(workerKey) == "" || strings.TrimSpace(owner) == "" {
		return fmt.Errorf("worker key and owner are required")
	}
	if ttl <= 0 {
		return fmt.Errorf("lease TTL must be positive")
	}
	return nil
}

type dbConnectorCursorStore struct {
	db  *gorm.DB
	now func() time.Time
}

func newDBConnectorCursorStore(db *gorm.DB) *dbConnectorCursorStore {
	return &dbConnectorCursorStore{db: db, now: time.Now}
}

func (s *dbConnectorCursorStore) Load(ctx context.Context, tenantID, connectorID int64, cursorName string) (*models.ConnectorSyncCursor, error) {
	var cursor models.ConnectorSyncCursor
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND connector_id = ? AND cursor_name = ?", tenantID, connectorID, cursorName).
		First(&cursor).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load connector cursor: %w", err)
	}
	return &cursor, nil
}

func (s *dbConnectorCursorStore) Save(ctx context.Context, cursor *models.ConnectorSyncCursor) error {
	if cursor == nil || cursor.TenantID <= 0 || cursor.ConnectorID <= 0 || strings.TrimSpace(cursor.CursorName) == "" {
		return fmt.Errorf("valid connector cursor is required")
	}
	now := s.now().UTC()
	if cursor.ID == "" {
		cursor.ID = utils.UUID()
	}
	if cursor.CreatedAt.IsZero() {
		cursor.CreatedAt = now
	}
	cursor.UpdatedAt = now
	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "connector_id"}, {Name: "cursor_name"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"cursor_value", "last_event_at", "last_success_at", "last_error", "updated_at",
		}),
	}).Create(cursor).Error
	if err != nil {
		return fmt.Errorf("save connector cursor: %w", err)
	}
	return nil
}

type mqttWorkerHealthStore interface {
	Update(ctx context.Context, connector *models.AccessConnector, status string, cause error, lastMessageAt *time.Time) error
	DeadLetter(ctx context.Context, connector *models.AccessConnector, message queuedMQTTMessage, stage string, cause error) error
}

type dbMQTTWorkerHealthStore struct {
	db      *gorm.DB
	cursors ConnectorCursorStore
	now     func() time.Time
}

func newDBMQTTWorkerHealthStore(db *gorm.DB, cursors ConnectorCursorStore) *dbMQTTWorkerHealthStore {
	return &dbMQTTWorkerHealthStore{db: db, cursors: cursors, now: time.Now}
}

func (s *dbMQTTWorkerHealthStore) Update(ctx context.Context, connector *models.AccessConnector, status string, cause error, lastMessageAt *time.Time) error {
	if connector == nil || connector.TenantID <= 0 || connector.ID <= 0 {
		return fmt.Errorf("MQTT connector is required")
	}
	now := s.now().UTC()
	updates := map[string]any{
		"health_status":  status,
		"last_tested_at": &now,
		"updated_at":     now,
	}
	if lastMessageAt != nil {
		messageAt := lastMessageAt.UTC()
		updates["last_message_at"] = &messageAt
	}
	result := s.db.WithContext(ctx).Model(&models.AccessConnector{}).
		Where("id = ? AND tenant_id = ? AND connector_type = ?", connector.ID, connector.TenantID, models.ConnectorTypeMQTT).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update MQTT connector health: %w", result.Error)
	}
	errorSummary := ""
	if cause != nil {
		errorSummary = sanitizeMQTTError(cause)
	}
	cursor := &models.ConnectorSyncCursor{
		TenantID:      connector.TenantID,
		ConnectorID:   connector.ID,
		CursorName:    mqttHealthCursorName,
		CursorValue:   status,
		LastError:     errorSummary,
		LastEventAt:   lastMessageAt,
		LastSuccessAt: nil,
	}
	if status == "healthy" {
		cursor.LastSuccessAt = &now
	}
	if err := s.cursors.Save(ctx, cursor); err != nil {
		return fmt.Errorf("save MQTT connector health summary: %w", err)
	}
	return nil
}

func (s *dbMQTTWorkerHealthStore) DeadLetter(ctx context.Context, connector *models.AccessConnector, message queuedMQTTMessage, stage string, cause error) error {
	if connector == nil {
		return fmt.Errorf("MQTT connector is required")
	}
	now := s.now().UTC()
	messageID := mqttMessageID(message.payload, message.messageIDField)
	payload := string(message.payload)
	if len(payload) > mqttPayloadLimit {
		payload = payload[:mqttPayloadLimit]
	}
	item := &models.ConnectorDeadLetter{
		ID:            utils.UUID(),
		TenantID:      connector.TenantID,
		ConnectorID:   connector.ID,
		Topic:         message.topic,
		MessageID:     messageID,
		PayloadHash:   hashConnectorPayload(message.payload),
		PayloadJSON:   payload,
		FailureStage:  truncateConnectorText(stage, 64),
		FailureReason: truncateConnectorText(sanitizeMQTTError(cause), mqttErrorLimit),
		Status:        models.ConnectorDeadLetterStatusPending,
		FirstFailedAt: now,
		LastFailedAt:  now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(item).Error
	if err != nil {
		return fmt.Errorf("create MQTT worker dead letter: %w", err)
	}
	return s.Update(ctx, connector, "degraded", cause, &message.receivedAt)
}

type mqttClientFactory interface {
	NewClient(options *mqtt.ClientOptions) mqtt.Client
}

type pahoMQTTClientFactory struct{}

func (pahoMQTTClientFactory) NewClient(options *mqtt.ClientOptions) mqtt.Client {
	return mqtt.NewClient(options)
}

type mqttConnectorAdapter struct {
	clients mqttClientFactory
	health  mqttWorkerHealthStore
	now     func() time.Time
}

func newMQTTConnectorAdapter(clients mqttClientFactory, health mqttWorkerHealthStore) *mqttConnectorAdapter {
	return &mqttConnectorAdapter{clients: clients, health: health, now: time.Now}
}

func (a *mqttConnectorAdapter) ConnectorType() string {
	return models.ConnectorTypeMQTT
}

func (a *mqttConnectorAdapter) Validate(connector *models.AccessConnector) error {
	_, _, err := parseMQTTWorkerConfiguration(connector)
	return err
}

func (a *mqttConnectorAdapter) Run(ctx context.Context, connector *models.AccessConnector, runtime ConnectorWorkerRuntime) error {
	if runtime.Sink == nil {
		return fmt.Errorf("MQTT message sink is required")
	}
	settings, credential, err := parseMQTTWorkerConfiguration(connector)
	if err != nil {
		_ = a.updateHealth(connector, "unhealthy", err, nil)
		return err
	}

	queue := make(chan queuedMQTTMessage, settings.QueueCapacity)
	fatal := make(chan error, 1)
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	options := mqtt.NewClientOptions().
		AddBroker(settings.BrokerURL).
		SetClientID(credential.ClientID).
		SetUsername(credential.Username).
		SetPassword(credential.Password).
		SetCleanSession(settings.CleanSession).
		SetResumeSubs(settings.ResumeSubscriptions).
		SetKeepAlive(settings.KeepAlive).
		SetPingTimeout(min(settings.KeepAlive/2, 15*time.Second)).
		SetConnectTimeout(mqttConnectTimeout).
		SetWriteTimeout(mqttOperationTimeout).
		SetAutoReconnect(true).
		SetConnectRetry(false).
		SetConnectRetryInterval(time.Second).
		SetMaxReconnectInterval(30 * time.Second).
		SetOrderMatters(false).
		SetAutoAckDisabled(true).
		SetStore(mqtt.NewMemoryStore())
	if settings.TLSConfig != nil {
		options.SetTLSConfig(settings.TLSConfig)
	}

	var client mqtt.Client
	messageHandler := func(_ mqtt.Client, message mqtt.Message) {
		item := queuedMQTTMessage{
			topic:          message.Topic(),
			payload:        append([]byte(nil), message.Payload()...),
			receivedAt:     a.now().UTC(),
			messageIDField: settings.MessageIDField,
			ack:            message.Ack,
			ackOnce:        &sync.Once{},
		}
		select {
		case queue <- item:
		default:
			go a.deadLetterBackpressure(connector, item)
		}
	}
	options.SetOnConnectHandler(func(connected mqtt.Client) {
		go func() {
			token := connected.SubscribeMultiple(settings.Topics, messageHandler)
			if !token.WaitTimeout(mqttOperationTimeout) {
				a.reportFatal(connector, fatal, fmt.Errorf("MQTT subscription timed out"))
				return
			}
			if err := token.Error(); err != nil {
				a.reportFatal(connector, fatal, fmt.Errorf("MQTT subscription failed: %w", err))
				return
			}
			_ = a.updateHealth(connector, "healthy", nil, nil)
		}()
	})
	options.SetConnectionLostHandler(func(_ mqtt.Client, cause error) {
		_ = a.updateHealth(connector, "degraded", fmt.Errorf("MQTT connection lost: %w", cause), nil)
	})
	options.SetReconnectingHandler(func(_ mqtt.Client, _ *mqtt.ClientOptions) {
		_ = a.updateHealth(connector, "degraded", errors.New("MQTT reconnecting"), nil)
	})

	client = a.clients.NewClient(options)
	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		a.consume(workerCtx, connector, runtime.Sink, queue)
	}()

	connectToken := client.Connect()
	if !connectToken.WaitTimeout(mqttConnectTimeout + time.Second) {
		err := errors.New("MQTT connection timed out")
		_ = a.updateHealth(connector, "unhealthy", err, nil)
		cancel()
		<-consumerDone
		return err
	}
	if err := connectToken.Error(); err != nil {
		wrapped := fmt.Errorf("MQTT connection failed: %w", err)
		_ = a.updateHealth(connector, "unhealthy", wrapped, nil)
		cancel()
		<-consumerDone
		return wrapped
	}

	var runErr error
	select {
	case <-ctx.Done():
		runErr = ctx.Err()
	case runErr = <-fatal:
	}
	cancel()
	client.Disconnect(mqttDisconnectQuiesceMs)
	<-consumerDone
	return runErr
}

func (a *mqttConnectorAdapter) consume(ctx context.Context, connector *models.AccessConnector, sink ConnectorMessageSink, queue <-chan queuedMQTTMessage) {
	for {
		select {
		case <-ctx.Done():
			return
		case item := <-queue:
			message := ConnectorMessage{
				TenantID:    connector.TenantID,
				ConnectorID: connector.ID,
				Topic:       item.topic,
				MessageID:   mqttMessageID(item.payload, item.messageIDField),
				Payload:     item.payload,
				ReceivedAt:  item.receivedAt,
			}
			result, err := sink.Ingest(ctx, message)
			if err == nil || result != nil {
				item.ackMessage()
			}
			if err != nil {
				_ = a.updateHealth(connector, "degraded", fmt.Errorf("MQTT ingestion failed: %w", err), &item.receivedAt)
			}
		}
	}
}

func (a *mqttConnectorAdapter) deadLetterBackpressure(connector *models.AccessConnector, item queuedMQTTMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), mqttOperationTimeout)
	defer cancel()
	cause := errors.New("MQTT worker queue capacity exhausted")
	if err := a.health.DeadLetter(ctx, connector, item, "worker_backpressure", cause); err != nil {
		slog.Error("record MQTT backpressure dead letter failed", "tenant_id", connector.TenantID, "connector_id", connector.ID, "error", sanitizeMQTTError(err))
		return
	}
	item.ackMessage()
}

func (a *mqttConnectorAdapter) reportFatal(connector *models.AccessConnector, fatal chan<- error, cause error) {
	_ = a.updateHealth(connector, "unhealthy", cause, nil)
	select {
	case fatal <- cause:
	default:
	}
}

func (a *mqttConnectorAdapter) updateHealth(connector *models.AccessConnector, status string, cause error, lastMessageAt *time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return a.health.Update(ctx, connector, status, cause, lastMessageAt)
}

type queuedMQTTMessage struct {
	topic          string
	payload        []byte
	receivedAt     time.Time
	messageIDField string
	ack            func()
	ackOnce        *sync.Once
}

func (m *queuedMQTTMessage) ackMessage() {
	if m.ack == nil {
		return
	}
	if m.ackOnce == nil {
		m.ackOnce = &sync.Once{}
	}
	m.ackOnce.Do(m.ack)
}

type mqttWorkerSettings struct {
	BrokerURL           string
	Topics              map[string]byte
	KeepAlive           time.Duration
	CleanSession        bool
	ResumeSubscriptions bool
	QueueCapacity       int
	MessageIDField      string
	TLSConfig           *tls.Config
}

type mqttWorkerCredential struct {
	Username string `json:"username"`
	Password string `json:"password"`
	ClientID string `json:"clientId"`
}

type mqttRuntimeConfig struct {
	QoS                 *int            `json:"qos"`
	Topics              json.RawMessage `json:"topics"`
	Topic               string          `json:"topic"`
	KeepAliveSeconds    *int            `json:"keepAliveSeconds"`
	CleanSession        *bool           `json:"cleanSession"`
	ResumeSubscriptions *bool           `json:"resumeSubscriptions"`
	QueueCapacity       *int            `json:"queueCapacity"`
	MessageIDField      string          `json:"messageIdField"`
	Mappings            []mqttMapping   `json:"mappings"`
	FieldMappings       []mqttMapping   `json:"fieldMappings"`
}

type mqttMapping struct {
	SourceExpression string `json:"sourceExpression"`
	TargetObject     string `json:"targetObject"`
	TargetField      string `json:"targetField"`
	External         string `json:"external"`
	Standard         string `json:"standard"`
}

func parseMQTTWorkerConfiguration(connector *models.AccessConnector) (*mqttWorkerSettings, *mqttWorkerCredential, error) {
	if connector == nil || connector.TenantID <= 0 || connector.ID <= 0 {
		return nil, nil, fmt.Errorf("valid MQTT connector is required")
	}
	if connector.ConnectorType != models.ConnectorTypeMQTT {
		return nil, nil, fmt.Errorf("connector %d is not MQTT", connector.ID)
	}
	parsed, err := validateMQTTBrokerURL(connector.BaseURL)
	if err != nil {
		return nil, nil, err
	}
	brokerURL := normalizedMQTTBrokerURL(parsed)

	runtime := mqttRuntimeConfig{}
	if template := GetConnectorTemplateByCode(connector.TemplateCode); template != nil && template.ConnectorType == models.ConnectorTypeMQTT {
		if err := overlayMQTTRuntimeConfig(&runtime, template.TemplateConfig); err != nil {
			return nil, nil, fmt.Errorf("invalid MQTT template configuration")
		}
	}
	if err := overlayMQTTRuntimeConfig(&runtime, connector.FieldMapping); err != nil {
		return nil, nil, fmt.Errorf("invalid MQTT field mapping configuration")
	}
	qos := 1
	if runtime.QoS != nil {
		qos = *runtime.QoS
	}
	if qos < 0 || qos > 2 {
		return nil, nil, fmt.Errorf("MQTT QoS must be 0, 1, or 2")
	}
	topics, err := parseMQTTTopics(runtime.Topics, runtime.Topic, byte(qos))
	if err != nil {
		return nil, nil, err
	}

	keepAliveSeconds := int(mqttDefaultKeepAlive / time.Second)
	if runtime.KeepAliveSeconds != nil {
		keepAliveSeconds = *runtime.KeepAliveSeconds
	}
	if keepAliveSeconds < 5 || keepAliveSeconds > 3600 {
		return nil, nil, fmt.Errorf("MQTT keepalive must be between 5 and 3600 seconds")
	}
	cleanSession := false
	if runtime.CleanSession != nil {
		cleanSession = *runtime.CleanSession
	}
	resumeSubscriptions := !cleanSession
	if runtime.ResumeSubscriptions != nil {
		resumeSubscriptions = *runtime.ResumeSubscriptions
	}
	if cleanSession {
		resumeSubscriptions = false
	}
	queueCapacity := mqttDefaultQueueCapacity
	if runtime.QueueCapacity != nil {
		queueCapacity = *runtime.QueueCapacity
	}
	if queueCapacity < 1 || queueCapacity > 65536 {
		return nil, nil, fmt.Errorf("MQTT queue capacity must be between 1 and 65536")
	}
	messageIDField := strings.TrimSpace(runtime.MessageIDField)
	if messageIDField == "" {
		messageIDField = findMQTTMessageIDField(append(runtime.Mappings, runtime.FieldMappings...))
	}
	if messageIDField == "" {
		messageIDField = "message_id"
	}

	credential, err := parseMQTTWorkerCredential(connector.AuthConfig)
	if err != nil {
		return nil, nil, err
	}
	if credential.ClientID == "" {
		credential.ClientID = stableMQTTClientID(connector.TenantID, connector.ID)
	}
	if len(credential.ClientID) > 128 || strings.ContainsRune(credential.ClientID, '\x00') {
		return nil, nil, fmt.Errorf("MQTT client ID is invalid")
	}

	settings := &mqttWorkerSettings{
		BrokerURL:           brokerURL,
		Topics:              topics,
		KeepAlive:           time.Duration(keepAliveSeconds) * time.Second,
		CleanSession:        cleanSession,
		ResumeSubscriptions: resumeSubscriptions,
		QueueCapacity:       queueCapacity,
		MessageIDField:      messageIDField,
	}
	if parsed.Scheme == "mqtts" || parsed.Scheme == "wss" {
		settings.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: parsed.Hostname(),
		}
	}
	return settings, credential, nil
}

func overlayMQTTRuntimeConfig(target *mqttRuntimeConfig, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var overlay mqttRuntimeConfig
	if err := json.Unmarshal([]byte(raw), &overlay); err != nil {
		return err
	}
	if overlay.QoS != nil {
		target.QoS = overlay.QoS
	}
	if len(overlay.Topics) > 0 && string(overlay.Topics) != "null" {
		target.Topics = append(json.RawMessage(nil), overlay.Topics...)
	}
	if strings.TrimSpace(overlay.Topic) != "" {
		target.Topic = overlay.Topic
	}
	if overlay.KeepAliveSeconds != nil {
		target.KeepAliveSeconds = overlay.KeepAliveSeconds
	}
	if overlay.CleanSession != nil {
		target.CleanSession = overlay.CleanSession
	}
	if overlay.ResumeSubscriptions != nil {
		target.ResumeSubscriptions = overlay.ResumeSubscriptions
	}
	if overlay.QueueCapacity != nil {
		target.QueueCapacity = overlay.QueueCapacity
	}
	if strings.TrimSpace(overlay.MessageIDField) != "" {
		target.MessageIDField = overlay.MessageIDField
	}
	if len(overlay.Mappings) > 0 {
		target.Mappings = overlay.Mappings
	}
	if len(overlay.FieldMappings) > 0 {
		target.FieldMappings = overlay.FieldMappings
	}
	return nil
}

func parseMQTTTopics(raw json.RawMessage, single string, defaultQoS byte) (map[string]byte, error) {
	topics := make(map[string]byte)
	if topic := strings.TrimSpace(single); topic != "" {
		topics[topic] = defaultQoS
	}
	if len(raw) > 0 && string(raw) != "null" {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("invalid MQTT topics configuration")
		}
		if err := collectMQTTTopics(topics, value, defaultQoS); err != nil {
			return nil, err
		}
	}
	if len(topics) == 0 {
		return nil, fmt.Errorf("at least one MQTT topic is required")
	}
	return topics, nil
}

func collectMQTTTopics(topics map[string]byte, value any, defaultQoS byte) error {
	switch typed := value.(type) {
	case string:
		return addMQTTTopic(topics, typed, defaultQoS)
	case []any:
		for _, item := range typed {
			if err := collectMQTTTopics(topics, item, defaultQoS); err != nil {
				return err
			}
		}
	case map[string]any:
		if filter, ok := mqttTopicFilter(typed); ok {
			qos, err := mqttTopicQoS(typed, defaultQoS)
			if err != nil {
				return err
			}
			return addMQTTTopic(topics, filter, qos)
		}
		for _, item := range typed {
			if err := collectMQTTTopics(topics, item, defaultQoS); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("invalid MQTT topic entry")
	}
	return nil
}

func mqttTopicFilter(value map[string]any) (string, bool) {
	for _, key := range []string{"filter", "topic", "name"} {
		if candidate, ok := value[key].(string); ok && strings.TrimSpace(candidate) != "" {
			return candidate, true
		}
	}
	return "", false
}

func mqttTopicQoS(value map[string]any, fallback byte) (byte, error) {
	raw, ok := value["qos"]
	if !ok {
		return fallback, nil
	}
	number, ok := raw.(float64)
	if !ok || number < 0 || number > 2 || number != float64(int(number)) {
		return 0, fmt.Errorf("MQTT topic QoS must be 0, 1, or 2")
	}
	return byte(number), nil
}

func addMQTTTopic(topics map[string]byte, topic string, qos byte) error {
	topic = strings.TrimSpace(topic)
	if topic == "" || len(topic) > 512 || strings.ContainsRune(topic, '\x00') {
		return fmt.Errorf("MQTT topic must be non-empty and at most 512 characters")
	}
	if qos > 2 {
		return fmt.Errorf("MQTT QoS must be 0, 1, or 2")
	}
	topics[topic] = qos
	return nil
}

func parseMQTTWorkerCredential(encrypted string) (*mqttWorkerCredential, error) {
	plain, err := secretstore.Decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt MQTT connector credential")
	}
	credential := &mqttWorkerCredential{}
	if strings.TrimSpace(plain) == "" {
		return credential, nil
	}
	if err := json.Unmarshal([]byte(plain), credential); err != nil {
		return nil, fmt.Errorf("invalid MQTT connector credential JSON")
	}
	credential.Username = strings.TrimSpace(credential.Username)
	credential.ClientID = strings.TrimSpace(credential.ClientID)
	if len(credential.Username) > 1024 || len(credential.Password) > 4096 {
		return nil, fmt.Errorf("MQTT connector credential is too long")
	}
	return credential, nil
}

func normalizedMQTTBrokerURL(parsed *url.URL) string {
	copyURL := *parsed
	if copyURL.Port() == "" {
		port := "1883"
		switch copyURL.Scheme {
		case "mqtts":
			port = "8883"
		case "ws":
			port = "80"
		case "wss":
			port = "443"
		}
		copyURL.Host = net.JoinHostPort(copyURL.Hostname(), port)
	}
	return copyURL.String()
}

func stableMQTTClientID(tenantID, connectorID int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%d", tenantID, connectorID)))
	return "rhd-" + hex.EncodeToString(sum[:9])
}

func findMQTTMessageIDField(mappings []mqttMapping) string {
	for _, mapping := range mappings {
		target := strings.TrimSpace(mapping.Standard)
		if target == "" {
			target = strings.Trim(strings.TrimSpace(mapping.TargetObject)+"."+strings.TrimSpace(mapping.TargetField), ".")
		}
		if !strings.EqualFold(target, "connector.messageId") {
			continue
		}
		source := strings.TrimSpace(mapping.External)
		if source == "" {
			source = strings.TrimSpace(mapping.SourceExpression)
		}
		return strings.TrimPrefix(source, "$.")
	}
	return ""
}

func mqttMessageID(payload []byte, configuredField string) string {
	fields := []string{strings.TrimSpace(configuredField), "message_id", "messageId", "id"}
	var decoded any
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err == nil {
		for _, field := range fields {
			if field == "" {
				continue
			}
			if value, ok := mqttPayloadValue(decoded, strings.TrimPrefix(field, "$.")); ok {
				messageID := strings.TrimSpace(fmt.Sprint(value))
				if messageID != "" && messageID != "<nil>" {
					if len(messageID) <= 256 {
						return messageID
					}
					return hashConnectorPayload([]byte(messageID))
				}
			}
		}
	}
	return hashConnectorPayload(payload)
}

func mqttPayloadValue(root any, path string) (any, bool) {
	current := root
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func sanitizeMQTTError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if r < 0x20 {
			return -1
		}
		return r
	}, err.Error())
	for {
		scheme := strings.Index(message, "://")
		if scheme < 0 {
			break
		}
		at := strings.Index(message[scheme+3:], "@")
		if at < 0 {
			break
		}
		at += scheme + 3
		start := strings.LastIndexAny(message[:scheme], " \"'(") + 1
		message = message[:start] + "[redacted]@" + message[at+1:]
	}
	return truncateConnectorText(strings.TrimSpace(message), 1024)
}
