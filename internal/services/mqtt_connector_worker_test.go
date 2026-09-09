package services

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/secretstore"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupMQTTWorkerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+sanitizeMQTTTestName(t.Name())+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open SQLite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.AccessConnector{},
		&models.ConnectorWorkerLease{},
		&models.ConnectorSyncCursor{},
		&models.ConnectorDeadLetter{},
	); err != nil {
		t.Fatalf("migrate MQTT worker tables: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func sanitizeMQTTTestName(value string) string {
	result := make([]rune, 0, len(value))
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' {
			result = append(result, char)
		} else {
			result = append(result, '_')
		}
	}
	return string(result)
}

func TestDBConnectorLeaseStoreUsesOwnerFenceAndExpiry(t *testing.T) {
	db := setupMQTTWorkerTestDB(t)
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	store := &dbConnectorLeaseStore{db: db, now: func() time.Time { return now }}

	fenceA, acquired, err := store.Acquire(context.Background(), 1, 9, mqttWorkerKey, "owner-a", 30*time.Second)
	if err != nil || !acquired || fenceA != 1 {
		t.Fatalf("first acquire = fence %d, acquired %v, err %v", fenceA, acquired, err)
	}
	if _, acquired, err := store.Acquire(context.Background(), 1, 9, mqttWorkerKey, "owner-b", 30*time.Second); err != nil || acquired {
		t.Fatalf("competing acquire = acquired %v, err %v", acquired, err)
	}
	if renewed, err := store.Renew(context.Background(), 1, 9, mqttWorkerKey, "owner-a", fenceA+1, 30*time.Second); err != nil || renewed {
		t.Fatalf("stale fence renewal = renewed %v, err %v", renewed, err)
	}
	if renewed, err := store.Renew(context.Background(), 1, 9, mqttWorkerKey, "owner-a", fenceA, 30*time.Second); err != nil || !renewed {
		t.Fatalf("valid renewal = renewed %v, err %v", renewed, err)
	}

	now = now.Add(31 * time.Second)
	fenceB, acquired, err := store.Acquire(context.Background(), 1, 9, mqttWorkerKey, "owner-b", 30*time.Second)
	if err != nil || !acquired || fenceB <= fenceA {
		t.Fatalf("expired takeover = fence %d, acquired %v, err %v", fenceB, acquired, err)
	}
	if err := store.Release(context.Background(), 1, 9, mqttWorkerKey, "owner-a", fenceA); err != nil {
		t.Fatalf("stale release returned error: %v", err)
	}
	if renewed, err := store.Renew(context.Background(), 1, 9, mqttWorkerKey, "owner-b", fenceB, 30*time.Second); err != nil || !renewed {
		t.Fatalf("stale release affected new owner: renewed %v, err %v", renewed, err)
	}

	otherTenantFence, acquired, err := store.Acquire(context.Background(), 2, 9, mqttWorkerKey, "owner-c", 30*time.Second)
	if err != nil || !acquired || otherTenantFence != 1 {
		t.Fatalf("cross-tenant lease was not isolated: fence %d, acquired %v, err %v", otherTenantFence, acquired, err)
	}
}

func TestLeasedMQTTConnectorRunnerCancelsAdapterWhenRenewalIsLost(t *testing.T) {
	leases := &fakeConnectorLeaseStore{renewed: false}
	adapter := &cancelAwareMQTTAdapter{stopped: make(chan struct{})}
	health := newFakeMQTTHealthStore()
	runner := &leasedMQTTConnectorRunner{
		owner:         "worker-a",
		workerKey:     mqttWorkerKey,
		leaseTTL:      30 * time.Millisecond,
		renewInterval: 5 * time.Millisecond,
		leases:        leases,
		health:        health,
		adapter:       adapter,
		runtime:       ConnectorWorkerRuntime{Sink: &recordingMQTTSink{}},
	}
	err := runner.Run(context.Background(), mqttWorkerConnector(1, 8, "mqtt://broker.example.com:1883", 8))
	if err == nil || !containsText(err.Error(), "renewal was rejected") {
		t.Fatalf("runner error = %v", err)
	}
	select {
	case <-adapter.stopped:
	case <-time.After(time.Second):
		t.Fatal("adapter was not cancelled after lease renewal failure")
	}
	if leases.releaseCount.Load() != 1 {
		t.Fatalf("release count = %d", leases.releaseCount.Load())
	}
	if !health.hasStatus("degraded") {
		t.Fatal("lease loss did not degrade connector health")
	}
}

func TestMQTTConnectorSupervisorStartsStopsAndIsolatesTenants(t *testing.T) {
	source := &mutableMQTTConnectorSource{}
	source.set([]models.AccessConnector{
		*mqttWorkerConnector(1, 7, "mqtt://tenant-1.example.com:1883", 8),
		*mqttWorkerConnector(2, 7, "mqtt://tenant-2.example.com:1883", 8),
	})
	runner := &recordingMQTTConnectorRunner{
		started: make(chan mqttConnectorKey, 8),
		stopped: make(chan mqttConnectorKey, 8),
	}
	supervisor := newMQTTConnectorSupervisor(source, runner, 5*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		supervisor.Run(ctx)
		close(done)
	}()

	started := collectMQTTKeys(t, runner.started, 2)
	if _, ok := started[mqttConnectorKey{TenantID: 1, ConnectorID: 7}]; !ok {
		t.Fatal("tenant 1 worker was not started")
	}
	if _, ok := started[mqttConnectorKey{TenantID: 2, ConnectorID: 7}]; !ok {
		t.Fatal("tenant 2 worker with the same connector id was not started")
	}

	source.set([]models.AccessConnector{*mqttWorkerConnector(1, 7, "mqtt://tenant-1.example.com:1883", 8)})
	select {
	case key := <-runner.stopped:
		if key.TenantID != 2 || key.ConnectorID != 7 {
			t.Fatalf("unexpected worker stopped: %+v", key)
		}
	case <-time.After(time.Second):
		t.Fatal("inactive tenant worker was not stopped")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("supervisor did not stop")
	}
}

func TestMQTTConnectorSupervisorRetriesInitialFailureWithBackoff(t *testing.T) {
	connector := mqttWorkerConnector(6, 61, "mqtt://broker.example.com:1883", 8)
	source := &mutableMQTTConnectorSource{}
	source.set([]models.AccessConnector{*connector})
	runner := &failOnceMQTTConnectorRunner{started: make(chan int, 4)}
	supervisor := newMQTTConnectorSupervisor(source, runner, 2*time.Millisecond)
	supervisor.retryBase = 20 * time.Millisecond
	supervisor.retryMax = 20 * time.Millisecond
	supervisor.retryReset = time.Second
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		supervisor.Run(ctx)
		close(done)
	}()

	select {
	case attempt := <-runner.started:
		if attempt != 1 {
			t.Fatalf("first attempt = %d", attempt)
		}
	case <-time.After(time.Second):
		t.Fatal("initial worker was not started")
	}
	select {
	case attempt := <-runner.started:
		t.Fatalf("worker retried without backoff, attempt %d", attempt)
	case <-time.After(10 * time.Millisecond):
	}
	select {
	case attempt := <-runner.started:
		if attempt != 2 {
			t.Fatalf("retry attempt = %d", attempt)
		}
	case <-time.After(time.Second):
		t.Fatal("failed initial worker was not restarted")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("retrying supervisor did not stop")
	}
}

func TestMQTTConnectorAdapterUsesPahoResubscribesAndForwardsMessages(t *testing.T) {
	credential, err := secretstore.Encrypt(`{"username":"device-user","password":"device-secret","clientId":"tenant-42-client"}`)
	if err != nil {
		t.Fatalf("encrypt credential: %v", err)
	}
	connector := mqttWorkerConnector(42, 19, "mqtts://broker.example.com", 8)
	connector.AuthConfig = credential
	connector.FieldMapping = `{
		"qos": 2,
		"topics": [
			{"filter":"devices/+/telemetry","qos":1},
			{"filter":"devices/+/events","qos":2}
		],
		"keepAliveSeconds": 30,
		"cleanSession": false,
		"resumeSubscriptions": true,
		"messageIdField": "event.id"
	}`
	health := newFakeMQTTHealthStore()
	factory := &fakeMQTTClientFactory{}
	adapter := newMQTTConnectorAdapter(factory, health)
	sink := &recordingMQTTSink{messages: make(chan ConnectorMessage, 8)}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- adapter.Run(ctx, connector, ConnectorWorkerRuntime{Sink: sink})
	}()
	client := factory.waitClient(t)
	client.waitSubscriptions(t, 1)

	options := factory.optionsSnapshot()
	if options.Username != "device-user" || options.Password != "device-secret" || options.ClientID != "tenant-42-client" {
		t.Fatal("decrypted MQTT credentials were not applied to Paho options")
	}
	if len(options.Servers) != 1 || options.Servers[0].String() != "mqtts://broker.example.com:8883" {
		t.Fatalf("broker URL = %v", options.Servers)
	}
	if options.TLSConfig == nil || options.TLSConfig.InsecureSkipVerify || options.TLSConfig.ServerName != "broker.example.com" {
		t.Fatalf("strict TLS options were not applied: %#v", options.TLSConfig)
	}
	if options.CleanSession || !options.ResumeSubs || !options.AutoReconnect || !options.AutoAckDisabled {
		t.Fatalf("session/reconnect options are incorrect: clean=%v resume=%v reconnect=%v manualAck=%v", options.CleanSession, options.ResumeSubs, options.AutoReconnect, options.AutoAckDisabled)
	}
	if got := client.topicQoS("devices/+/telemetry"); got != 1 {
		t.Fatalf("telemetry QoS = %d", got)
	}
	if got := client.topicQoS("devices/+/events"); got != 2 {
		t.Fatalf("events QoS = %d", got)
	}

	message := newFakeMQTTMessage("devices/SERIAL-42/events", []byte(`{"event":{"id":"event-7742"},"device_id":"SERIAL-42"}`))
	client.emit(message)
	select {
	case forwarded := <-sink.messages:
		if forwarded.TenantID != 42 || forwarded.ConnectorID != 19 || forwarded.MessageID != "event-7742" {
			t.Fatalf("forwarded message = %+v", forwarded)
		}
	case <-time.After(time.Second):
		t.Fatal("MQTT message was not forwarded")
	}
	eventuallyMQTT(t, time.Second, func() bool { return message.ackCount.Load() == 1 }, "forwarded message was not acknowledged")

	options.OnConnect(client)
	client.waitSubscriptions(t, 2)

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("adapter stop error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("adapter did not disconnect after cancellation")
	}
	if client.disconnectCount.Load() != 1 {
		t.Fatalf("disconnect count = %d", client.disconnectCount.Load())
	}
}

func TestMQTTConnectorAdapterBackpressureCreatesDeadLetterWithoutBlockingCallback(t *testing.T) {
	connector := mqttWorkerConnector(3, 31, "ws://broker.example.com/mqtt", 1)
	health := newFakeMQTTHealthStore()
	factory := &fakeMQTTClientFactory{}
	adapter := newMQTTConnectorAdapter(factory, health)
	block := make(chan struct{})
	sink := &recordingMQTTSink{
		messages: make(chan ConnectorMessage, 8),
		block:    block,
		started:  make(chan struct{}, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- adapter.Run(ctx, connector, ConnectorWorkerRuntime{Sink: sink})
	}()
	client := factory.waitClient(t)
	client.waitSubscriptions(t, 1)

	first := newFakeMQTTMessage("devices/1/telemetry", []byte(`{"message_id":"m-1"}`))
	second := newFakeMQTTMessage("devices/1/telemetry", []byte(`{"message_id":"m-2"}`))
	overflow := newFakeMQTTMessage("devices/1/telemetry", []byte(`{"message_id":"m-overflow"}`))
	client.emit(first)
	select {
	case <-sink.started:
	case <-time.After(time.Second):
		t.Fatal("consumer did not begin processing first message")
	}
	client.emit(second)
	startedAt := time.Now()
	client.emit(overflow)
	if elapsed := time.Since(startedAt); elapsed > 50*time.Millisecond {
		t.Fatalf("Paho callback blocked for %s", elapsed)
	}
	select {
	case deadLetter := <-health.deadLetters:
		if deadLetter.tenantID != 3 || deadLetter.connectorID != 31 || deadLetter.messageID != "m-overflow" || deadLetter.stage != "worker_backpressure" {
			t.Fatalf("backpressure dead letter = %+v", deadLetter)
		}
	case <-time.After(time.Second):
		t.Fatal("queue overflow was not dead-lettered")
	}
	eventuallyMQTT(t, time.Second, func() bool { return overflow.ackCount.Load() == 1 }, "dead-lettered message was not acknowledged")
	if !health.hasStatus("degraded") {
		t.Fatal("queue overflow did not degrade connector health")
	}

	close(block)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("adapter did not stop after backpressure test")
	}
}

func TestDBMQTTWorkerHealthPersistsRedactedErrorAndDeadLetter(t *testing.T) {
	db := setupMQTTWorkerTestDB(t)
	connector := mqttWorkerConnector(5, 0, "mqtt://broker.example.com:1883", 4)
	connector.ID = 0
	if err := db.Create(connector).Error; err != nil {
		t.Fatalf("create connector: %v", err)
	}
	cursors := newDBConnectorCursorStore(db)
	health := newDBMQTTWorkerHealthStore(db, cursors)
	receivedAt := time.Date(2026, 8, 10, 13, 0, 0, 0, time.UTC)
	item := queuedMQTTMessage{
		topic:          "devices/5/events",
		payload:        []byte(`{"message_id":"health-1"}`),
		receivedAt:     receivedAt,
		messageIDField: "message_id",
	}
	if err := health.DeadLetter(context.Background(), connector, item, "worker_backpressure", errors.New("connect mqtt://user:password@broker failed")); err != nil {
		t.Fatalf("record dead letter: %v", err)
	}
	var persisted models.AccessConnector
	if err := db.First(&persisted, connector.ID).Error; err != nil {
		t.Fatalf("load connector: %v", err)
	}
	if persisted.HealthStatus != "degraded" || persisted.LastTestedAt == nil || persisted.LastMessageAt == nil || !persisted.LastMessageAt.Equal(receivedAt) {
		t.Fatalf("persisted connector health = %+v", persisted)
	}
	cursor, err := cursors.Load(context.Background(), connector.TenantID, connector.ID, mqttHealthCursorName)
	if err != nil || cursor == nil {
		t.Fatalf("load health cursor = %+v, %v", cursor, err)
	}
	if containsText(cursor.LastError, "password") || !containsText(cursor.LastError, "[redacted]") {
		t.Fatalf("credential leaked in health summary: %q", cursor.LastError)
	}
	var deadLetter models.ConnectorDeadLetter
	if err := db.First(&deadLetter).Error; err != nil {
		t.Fatalf("load dead letter: %v", err)
	}
	if deadLetter.TenantID != 5 || deadLetter.MessageID != "health-1" || deadLetter.FailureStage != "worker_backpressure" {
		t.Fatalf("dead letter = %+v", deadLetter)
	}
}

func TestMQTTMessageIDUsesConfiguredFieldThenStablePayloadHash(t *testing.T) {
	payload := []byte(`{"meta":{"eventId":7742},"value":10}`)
	if got := mqttMessageID(payload, "meta.eventId"); got != "7742" {
		t.Fatalf("configured message id = %q", got)
	}
	withoutID := []byte(`{"value":10}`)
	first := mqttMessageID(withoutID, "meta.eventId")
	second := mqttMessageID(append([]byte(nil), withoutID...), "meta.eventId")
	if first == "" || first != second || first != hashConnectorPayload(withoutID) {
		t.Fatalf("fallback ids = %q and %q", first, second)
	}
}

func mqttWorkerConnector(tenantID, connectorID int64, broker string, queueCapacity int) *models.AccessConnector {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	return &models.AccessConnector{
		ID:            connectorID,
		TenantID:      tenantID,
		Name:          fmt.Sprintf("tenant-%d-mqtt", tenantID),
		ConnectorType: models.ConnectorTypeMQTT,
		BaseURL:       broker,
		AuthType:      "basic",
		FieldMapping: fmt.Sprintf(`{
			"qos":1,
			"topics":{"telemetry":"devices/+/telemetry"},
			"queueCapacity":%d,
			"keepAliveSeconds":30,
			"cleanSession":false
		}`, queueCapacity),
		Status:       "active",
		HealthStatus: "unknown",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func eventuallyMQTT(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal(message)
}

func containsText(value, part string) bool {
	for index := 0; index+len(part) <= len(value); index++ {
		if value[index:index+len(part)] == part {
			return true
		}
	}
	return false
}

type fakeConnectorLeaseStore struct {
	renewed      bool
	releaseCount atomic.Int32
}

func (s *fakeConnectorLeaseStore) Acquire(context.Context, int64, int64, string, string, time.Duration) (int64, bool, error) {
	return 7, true, nil
}

func (s *fakeConnectorLeaseStore) Renew(context.Context, int64, int64, string, string, int64, time.Duration) (bool, error) {
	return s.renewed, nil
}

func (s *fakeConnectorLeaseStore) Release(context.Context, int64, int64, string, string, int64) error {
	s.releaseCount.Add(1)
	return nil
}

type cancelAwareMQTTAdapter struct {
	stopped chan struct{}
}

func (a *cancelAwareMQTTAdapter) ConnectorType() string                  { return models.ConnectorTypeMQTT }
func (a *cancelAwareMQTTAdapter) Validate(*models.AccessConnector) error { return nil }
func (a *cancelAwareMQTTAdapter) Run(ctx context.Context, _ *models.AccessConnector, _ ConnectorWorkerRuntime) error {
	<-ctx.Done()
	close(a.stopped)
	return ctx.Err()
}

type mutableMQTTConnectorSource struct {
	mu         sync.RWMutex
	connectors []models.AccessConnector
}

func (s *mutableMQTTConnectorSource) set(connectors []models.AccessConnector) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connectors = append([]models.AccessConnector(nil), connectors...)
}

func (s *mutableMQTTConnectorSource) ListActiveMQTTConnectors(context.Context) ([]models.AccessConnector, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]models.AccessConnector(nil), s.connectors...), nil
}

type recordingMQTTConnectorRunner struct {
	started chan mqttConnectorKey
	stopped chan mqttConnectorKey
}

type failOnceMQTTConnectorRunner struct {
	attempt atomic.Int32
	started chan int
}

func (r *failOnceMQTTConnectorRunner) Run(ctx context.Context, _ *models.AccessConnector) error {
	attempt := int(r.attempt.Add(1))
	r.started <- attempt
	if attempt == 1 {
		return errors.New("initial broker connection failed")
	}
	<-ctx.Done()
	return ctx.Err()
}

func (r *recordingMQTTConnectorRunner) Run(ctx context.Context, connector *models.AccessConnector) error {
	key := mqttConnectorKey{TenantID: connector.TenantID, ConnectorID: connector.ID}
	r.started <- key
	<-ctx.Done()
	r.stopped <- key
	return ctx.Err()
}

func collectMQTTKeys(t *testing.T, source <-chan mqttConnectorKey, count int) map[mqttConnectorKey]struct{} {
	t.Helper()
	result := make(map[mqttConnectorKey]struct{}, count)
	deadline := time.After(time.Second)
	for len(result) < count {
		select {
		case key := <-source:
			result[key] = struct{}{}
		case <-deadline:
			t.Fatalf("received %d of %d worker starts", len(result), count)
		}
	}
	return result
}

type mqttHealthUpdate struct {
	status string
	cause  string
}

type mqttDeadLetterRecord struct {
	tenantID    int64
	connectorID int64
	messageID   string
	stage       string
}

type fakeMQTTHealthStore struct {
	mu          sync.Mutex
	updates     []mqttHealthUpdate
	deadLetters chan mqttDeadLetterRecord
}

func newFakeMQTTHealthStore() *fakeMQTTHealthStore {
	return &fakeMQTTHealthStore{deadLetters: make(chan mqttDeadLetterRecord, 16)}
}

func (s *fakeMQTTHealthStore) Update(_ context.Context, _ *models.AccessConnector, status string, cause error, _ *time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	update := mqttHealthUpdate{status: status}
	if cause != nil {
		update.cause = cause.Error()
	}
	s.updates = append(s.updates, update)
	return nil
}

func (s *fakeMQTTHealthStore) DeadLetter(ctx context.Context, connector *models.AccessConnector, message queuedMQTTMessage, stage string, cause error) error {
	record := mqttDeadLetterRecord{
		tenantID:    connector.TenantID,
		connectorID: connector.ID,
		messageID:   mqttMessageID(message.payload, message.messageIDField),
		stage:       stage,
	}
	select {
	case s.deadLetters <- record:
	case <-ctx.Done():
		return ctx.Err()
	}
	return s.Update(ctx, connector, "degraded", cause, &message.receivedAt)
}

func (s *fakeMQTTHealthStore) hasStatus(status string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, update := range s.updates {
		if update.status == status {
			return true
		}
	}
	return false
}

type recordingMQTTSink struct {
	messages chan ConnectorMessage
	block    <-chan struct{}
	started  chan struct{}
}

func (s *recordingMQTTSink) Ingest(ctx context.Context, message ConnectorMessage) (*ConnectorIngestResult, error) {
	if s.messages != nil {
		s.messages <- message
	}
	if s.started != nil {
		select {
		case s.started <- struct{}{}:
		default:
		}
	}
	if s.block != nil {
		select {
		case <-s.block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return &ConnectorIngestResult{InboxID: "inbox-test"}, nil
}

type fakeMQTTClientFactory struct {
	mu      sync.Mutex
	options *mqtt.ClientOptions
	client  *fakeMQTTClient
	ready   chan struct{}
}

func (f *fakeMQTTClientFactory) NewClient(options *mqtt.ClientOptions) mqtt.Client {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ready == nil {
		f.ready = make(chan struct{})
	}
	f.options = options
	f.client = &fakeMQTTClient{options: options, connected: true, subscribed: make(chan struct{}, 16)}
	close(f.ready)
	return f.client
}

func (f *fakeMQTTClientFactory) waitClient(t *testing.T) *fakeMQTTClient {
	t.Helper()
	for {
		f.mu.Lock()
		if f.client != nil {
			client := f.client
			f.mu.Unlock()
			return client
		}
		f.mu.Unlock()
		select {
		case <-time.After(time.Millisecond):
		case <-time.After(time.Second):
			t.Fatal("Paho client was not created")
		}
	}
}

func (f *fakeMQTTClientFactory) optionsSnapshot() mqtt.ClientOptions {
	f.mu.Lock()
	defer f.mu.Unlock()
	return *f.options
}

type fakeMQTTClient struct {
	mu              sync.Mutex
	options         *mqtt.ClientOptions
	connected       bool
	topics          map[string]byte
	handler         mqtt.MessageHandler
	subscribeCount  int
	subscribed      chan struct{}
	disconnectCount atomic.Int32
}

func (c *fakeMQTTClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

func (c *fakeMQTTClient) IsConnectionOpen() bool { return c.IsConnected() }

func (c *fakeMQTTClient) Connect() mqtt.Token {
	c.mu.Lock()
	c.connected = true
	onConnect := c.options.OnConnect
	c.mu.Unlock()
	if onConnect != nil {
		onConnect(c)
	}
	return newFakeMQTTToken(nil)
}

func (c *fakeMQTTClient) Disconnect(uint) {
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()
	c.disconnectCount.Add(1)
}

func (c *fakeMQTTClient) Publish(string, byte, bool, interface{}) mqtt.Token {
	return newFakeMQTTToken(nil)
}

func (c *fakeMQTTClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
	return c.SubscribeMultiple(map[string]byte{topic: qos}, callback)
}

func (c *fakeMQTTClient) SubscribeMultiple(topics map[string]byte, callback mqtt.MessageHandler) mqtt.Token {
	c.mu.Lock()
	c.topics = make(map[string]byte, len(topics))
	for topic, qos := range topics {
		c.topics[topic] = qos
	}
	c.handler = callback
	c.subscribeCount++
	c.mu.Unlock()
	c.subscribed <- struct{}{}
	return newFakeMQTTToken(nil)
}

func (c *fakeMQTTClient) Unsubscribe(...string) mqtt.Token        { return newFakeMQTTToken(nil) }
func (c *fakeMQTTClient) AddRoute(string, mqtt.MessageHandler)    {}
func (c *fakeMQTTClient) OptionsReader() mqtt.ClientOptionsReader { return mqtt.ClientOptionsReader{} }

func (c *fakeMQTTClient) waitSubscriptions(t *testing.T, count int) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		c.mu.Lock()
		current := c.subscribeCount
		c.mu.Unlock()
		if current >= count {
			return
		}
		select {
		case <-c.subscribed:
		case <-deadline:
			t.Fatalf("subscriptions = %d, want at least %d", current, count)
		}
	}
}

func (c *fakeMQTTClient) topicQoS(topic string) byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.topics[topic]
}

func (c *fakeMQTTClient) emit(message mqtt.Message) {
	c.mu.Lock()
	handler := c.handler
	c.mu.Unlock()
	if handler == nil {
		panic("MQTT test emitted before subscription")
	}
	handler(c, message)
}

type fakeMQTTToken struct {
	done chan struct{}
	err  error
}

func newFakeMQTTToken(err error) *fakeMQTTToken {
	done := make(chan struct{})
	close(done)
	return &fakeMQTTToken{done: done, err: err}
}

func (t *fakeMQTTToken) Wait() bool {
	<-t.done
	return true
}

func (t *fakeMQTTToken) WaitTimeout(timeout time.Duration) bool {
	select {
	case <-t.done:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (t *fakeMQTTToken) Done() <-chan struct{} { return t.done }
func (t *fakeMQTTToken) Error() error          { return t.err }

type fakeMQTTMessage struct {
	topic    string
	payload  []byte
	ackCount atomic.Int32
}

func newFakeMQTTMessage(topic string, payload []byte) *fakeMQTTMessage {
	return &fakeMQTTMessage{topic: topic, payload: payload}
}

func (m *fakeMQTTMessage) Duplicate() bool   { return false }
func (m *fakeMQTTMessage) Qos() byte         { return 1 }
func (m *fakeMQTTMessage) Retained() bool    { return false }
func (m *fakeMQTTMessage) Topic() string     { return m.topic }
func (m *fakeMQTTMessage) MessageID() uint16 { return 1 }
func (m *fakeMQTTMessage) Payload() []byte   { return m.payload }
func (m *fakeMQTTMessage) Ack()              { m.ackCount.Add(1) }
