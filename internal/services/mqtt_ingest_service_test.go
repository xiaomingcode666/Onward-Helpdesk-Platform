package services

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupMQTTIngestTestDB(t *testing.T) (*gorm.DB, *mqttMessageIngestService) {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.AccessConnector{},
		&models.Device{},
		&models.ConnectorSyncCursor{},
		&models.ConnectorWorkerLease{},
		&models.ConnectorEventInbox{},
		&models.DeviceTelemetryEvent{},
		&models.DeviceTelemetrySnapshot{},
		&models.DeviceAlarmEvent{},
		&models.ConnectorDeadLetter{},
		&models.DomainEvent{},
		&models.OutboxRecord{},
	); err != nil {
		t.Fatalf("migrate MQTT ingest models: %v", err)
	}
	if !db.Migrator().HasTable("t_connector_event_inbox") || !db.Migrator().HasTable("t_device_telemetry_event") {
		t.Fatal("new connector models did not follow the t_ + singular naming strategy")
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db, NewMQTTMessageIngestService(db)
}

func createMQTTConnector(t *testing.T, db *gorm.DB, tenantID int64, fieldMapping string) *models.AccessConnector {
	t.Helper()
	now := time.Now().UTC()
	connector := &models.AccessConnector{
		TenantID:      tenantID,
		Name:          fmt.Sprintf("tenant-%d-mqtt", tenantID),
		ConnectorType: models.ConnectorTypeMQTT,
		BaseURL:       "mqtts://broker.example.com:8883",
		FieldMapping:  fieldMapping,
		Status:        "active",
		HealthStatus:  "unknown",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.Create(connector).Error; err != nil {
		t.Fatalf("create MQTT connector: %v", err)
	}
	return connector
}

func createMQTTDevice(t *testing.T, db *gorm.DB, tenantID, productID, modelID int64, deviceNo, serialNo string) *models.Device {
	t.Helper()
	now := time.Now().UTC()
	device := &models.Device{
		TenantID:       tenantID,
		DeviceNo:       deviceNo,
		SerialNo:       serialNo,
		ProductID:      productID,
		ProductModelID: modelID,
		AuditFields: models.AuditFields{
			CreatedAt: now, UpdatedAt: now, CreateUserName: "system", UpdateUserName: "system",
		},
	}
	if err := db.Create(device).Error; err != nil {
		t.Fatalf("create MQTT device fixture: %v", err)
	}
	return device
}

func mqttFixtureMapping(policy string) string {
	return fmt.Sprintf(`{
		"unknownDevicePolicy": %q,
		"mappings": [
			{"sourceExpression":"$.device_id","targetObject":"device","targetField":"serialNo","transformType":"jsonpath","status":"active"},
			{"sourceExpression":"$.temperature","targetObject":"deviceTelemetry","targetField":"metrics.temperature","transformType":"jsonpath","status":"active"},
			{"sourceExpression":"$.pressure","targetObject":"deviceTelemetry","targetField":"metrics.pressure","transformType":"jsonpath","status":"active"},
			{"sourceExpression":"$.unit","targetObject":"deviceTelemetry","targetField":"unit","transformType":"jsonpath","status":"active"},
			{"sourceExpression":"$.fault_code","targetObject":"faultCode","targetField":"code","transformType":"jsonpath","status":"active"},
			{"sourceExpression":"$.severity","targetObject":"deviceEvent","targetField":"severity","transformType":"jsonpath","status":"active"},
			{"sourceExpression":"$.recorded_at","targetObject":"deviceTelemetry","targetField":"recordedAt","transformType":"jsonpath","status":"active"}
		]
	}`, policy)
}

func TestMQTTIngestPersistsIdempotentProductScopedTelemetryAlarmAndSnapshot(t *testing.T) {
	db, service := setupMQTTIngestTestDB(t)
	connector := createMQTTConnector(t, db, 1, mqttFixtureMapping(mqttUnknownDeviceDeadLetter))
	device := createMQTTDevice(t, db, 1, 101, 1001, "DEV-T1", "SERIAL-SHARED")
	_ = createMQTTDevice(t, db, 2, 202, 2002, "DEV-T2", "SERIAL-SHARED")

	receivedAt := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	payload := []byte(`{
		"device_id":"SERIAL-SHARED",
		"temperature":42.5,
		"pressure":1.8,
		"unit":"C",
		"fault_code":"RHD-FLOW-ALPHA-7742",
		"severity":"critical",
		"recorded_at":"2026-08-10T11:59:30Z"
	}`)
	message := ConnectorMessage{
		TenantID: 1, ConnectorID: connector.ID, Topic: "devices/SERIAL-SHARED/telemetry",
		MessageID: "message-001", Payload: payload, ReceivedAt: receivedAt,
	}
	result, err := service.Ingest(context.Background(), message)
	if err != nil {
		t.Fatalf("ingest MQTT message: %v", err)
	}
	if result.Duplicate || len(result.TelemetryEventIDs) != 2 || result.AlarmID == "" {
		t.Fatalf("unexpected ingest result: %+v", result)
	}

	var telemetryEvents []models.DeviceTelemetryEvent
	if err := db.Order("metric_key ASC").Find(&telemetryEvents).Error; err != nil {
		t.Fatalf("load telemetry events: %v", err)
	}
	if len(telemetryEvents) != 2 {
		t.Fatalf("telemetry event count = %d", len(telemetryEvents))
	}
	for _, event := range telemetryEvents {
		if event.TenantID != 1 || event.DeviceID != device.ID || event.ProductID != 101 || event.ProductModelID != 1001 {
			t.Fatalf("telemetry event escaped tenant/product device scope: %+v", event)
		}
	}

	var snapshots []models.DeviceTelemetrySnapshot
	if err := db.Order("metric_key ASC").Find(&snapshots).Error; err != nil {
		t.Fatalf("load telemetry snapshots: %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("telemetry snapshot count = %d", len(snapshots))
	}
	for _, snapshot := range snapshots {
		if snapshot.DeviceID != device.ID || snapshot.ProductID != 101 || snapshot.ProductModelID != 1001 {
			t.Fatalf("snapshot missing product device binding: %+v", snapshot)
		}
	}

	var alarm models.DeviceAlarmEvent
	if err := db.First(&alarm).Error; err != nil {
		t.Fatalf("load alarm: %v", err)
	}
	if alarm.DeviceID != device.ID || alarm.ProductID != 101 || alarm.ProductModelID != 1001 || alarm.Severity != "critical" {
		t.Fatalf("alarm missing product device binding: %+v", alarm)
	}
	var domainEvent models.DomainEvent
	if err := db.Where("event_type = ?", events.EventDeviceAlarmRaised).First(&domainEvent).Error; err != nil {
		t.Fatalf("load durable alarm event: %v", err)
	}
	var outbox models.OutboxRecord
	if err := db.Where("event_id = ?", domainEvent.ID).First(&outbox).Error; err != nil {
		t.Fatalf("load alarm outbox record: %v", err)
	}

	var persistedConnector models.AccessConnector
	if err := db.First(&persistedConnector, connector.ID).Error; err != nil {
		t.Fatalf("reload connector: %v", err)
	}
	if persistedConnector.LastMessageAt == nil || !persistedConnector.LastMessageAt.Equal(receivedAt) || persistedConnector.HealthStatus != "healthy" {
		t.Fatalf("connector activity not updated: %+v", persistedConnector)
	}

	duplicate, err := service.Ingest(context.Background(), message)
	if err != nil {
		t.Fatalf("ingest duplicate MQTT message: %v", err)
	}
	if !duplicate.Duplicate {
		t.Fatalf("duplicate result = %+v", duplicate)
	}
	var eventCount int64
	if err := db.Model(&models.DeviceTelemetryEvent{}).Count(&eventCount).Error; err != nil || eventCount != 2 {
		t.Fatalf("event count after duplicate = %d, err=%v", eventCount, err)
	}
	var domainEventCount int64
	if err := db.Model(&models.DomainEvent{}).Where("event_type = ?", events.EventDeviceAlarmRaised).Count(&domainEventCount).Error; err != nil || domainEventCount != 1 {
		t.Fatalf("alarm domain event count after duplicate = %d, err=%v", domainEventCount, err)
	}

	conflict := message
	conflict.Payload = []byte(`{"device_id":"SERIAL-SHARED","temperature":99}`)
	if _, err := service.Ingest(context.Background(), conflict); err == nil || !strings.Contains(err.Error(), "reused with a different payload") {
		t.Fatalf("idempotency conflict error = %v", err)
	}
	var deadLetter models.ConnectorDeadLetter
	if err := db.Where("failure_stage = ?", "idempotency").First(&deadLetter).Error; err != nil {
		t.Fatalf("load idempotency dead letter: %v", err)
	}
}

func TestMQTTIngestDoesNotLetOutOfOrderEventRegressSnapshot(t *testing.T) {
	db, service := setupMQTTIngestTestDB(t)
	connector := createMQTTConnector(t, db, 1, `{
		"unknownDevicePolicy":"dead_letter",
		"deviceSerial":"$.device_id",
		"metricName":"$.metric",
		"metricValue":"$.value",
		"recordedAt":"$.recorded_at"
	}`)
	device := createMQTTDevice(t, db, 1, 101, 1001, "DEV-ORDER", "SERIAL-ORDER")

	newer := ConnectorMessage{TenantID: 1, ConnectorID: connector.ID, Topic: "devices/order/telemetry", MessageID: "newer", Payload: []byte(
		`{"device_id":"SERIAL-ORDER","metric":"temperature","value":50,"recorded_at":"2026-08-10T12:00:00Z"}`)}
	if _, err := service.Ingest(context.Background(), newer); err != nil {
		t.Fatalf("ingest newer event: %v", err)
	}
	older := ConnectorMessage{TenantID: 1, ConnectorID: connector.ID, Topic: "devices/order/telemetry", MessageID: "older", Payload: []byte(
		`{"device_id":"SERIAL-ORDER","metric":"temperature","value":20,"recorded_at":"2026-08-10T11:00:00Z"}`)}
	if _, err := service.Ingest(context.Background(), older); err != nil {
		t.Fatalf("ingest older event: %v", err)
	}

	var snapshot models.DeviceTelemetrySnapshot
	if err := db.Where("device_id = ? AND metric_key = ?", device.ID, "temperature").First(&snapshot).Error; err != nil {
		t.Fatalf("load latest snapshot: %v", err)
	}
	if snapshot.MetricValue != 50 || !snapshot.RecordedAt.Equal(time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("out-of-order event regressed snapshot: %+v", snapshot)
	}
}

func TestMQTTIngestUnknownDevicePolicyAndTenantIsolation(t *testing.T) {
	db, service := setupMQTTIngestTestDB(t)
	deviceTenant1 := createMQTTDevice(t, db, 1, 101, 1001, "DEV-1", "SERIAL-SHARED")
	deviceTenant2 := createMQTTDevice(t, db, 2, 202, 2002, "DEV-2", "SERIAL-SHARED")
	connectorTenant1 := createMQTTConnector(t, db, 1, mqttFixtureMapping(mqttUnknownDeviceDeadLetter))
	connectorTenant2 := createMQTTConnector(t, db, 2, mqttFixtureMapping(mqttUnknownDeviceDeadLetter))

	payload := []byte(`{"device_id":"SERIAL-SHARED","temperature":30}`)
	for _, item := range []struct {
		tenantID    int64
		connectorID int64
		messageID   string
		wantDevice  *models.Device
	}{
		{tenantID: 1, connectorID: connectorTenant1.ID, messageID: "tenant-1", wantDevice: deviceTenant1},
		{tenantID: 2, connectorID: connectorTenant2.ID, messageID: "tenant-2", wantDevice: deviceTenant2},
	} {
		result, err := service.Ingest(context.Background(), ConnectorMessage{
			TenantID: item.tenantID, ConnectorID: item.connectorID, Topic: "devices/shared/telemetry",
			MessageID: item.messageID, Payload: payload,
		})
		if err != nil {
			t.Fatalf("ingest tenant %d message: %v", item.tenantID, err)
		}
		var event models.DeviceTelemetryEvent
		if err := db.First(&event, "id = ?", result.TelemetryEventIDs[0]).Error; err != nil {
			t.Fatalf("load tenant %d event: %v", item.tenantID, err)
		}
		if event.TenantID != item.tenantID || event.DeviceID != item.wantDevice.ID || event.ProductID != item.wantDevice.ProductID {
			t.Fatalf("tenant %d event bound to wrong device: %+v", item.tenantID, event)
		}
	}

	unknown := ConnectorMessage{
		TenantID: 1, ConnectorID: connectorTenant1.ID, Topic: "devices/unknown/telemetry",
		MessageID: "unknown-default", Payload: []byte(`{"device_id":"DOES-NOT-EXIST","temperature":30}`),
	}
	if _, err := service.Ingest(context.Background(), unknown); err == nil || !strings.Contains(err.Error(), "no device matched") {
		t.Fatalf("unknown device error = %v", err)
	}
	var unknownInbox models.ConnectorEventInbox
	if err := db.Where("message_id = ?", unknown.MessageID).First(&unknownInbox).Error; err != nil {
		t.Fatalf("load unknown-device inbox: %v", err)
	}
	if unknownInbox.Status != models.ConnectorInboxStatusDeadLettered {
		t.Fatalf("unknown-device inbox status = %q", unknownInbox.Status)
	}
	var unknownDLQ models.ConnectorDeadLetter
	if err := db.Where("message_id = ? AND failure_stage = ?", unknown.MessageID, "device_resolution").First(&unknownDLQ).Error; err != nil {
		t.Fatalf("load unknown-device dead letter: %v", err)
	}

	acceptConnector := createMQTTConnector(t, db, 1, `{
		"unknownDevicePolicy":"accept_unbound",
		"deviceSerial":"$.device_id",
		"metrics":{"temperature":"$.temperature"}
	}`)
	accepted, err := service.Ingest(context.Background(), ConnectorMessage{
		TenantID: 1, ConnectorID: acceptConnector.ID, Topic: "devices/unbound/telemetry",
		MessageID: "unknown-accepted", Payload: []byte(`{"device_id":"UNBOUND","temperature":31}`),
	})
	if err != nil {
		t.Fatalf("ingest configured unbound device: %v", err)
	}
	var unboundEvent models.DeviceTelemetryEvent
	if err := db.First(&unboundEvent, "id = ?", accepted.TelemetryEventIDs[0]).Error; err != nil {
		t.Fatalf("load unbound event: %v", err)
	}
	if unboundEvent.DeviceID != 0 || unboundEvent.ProductID != 0 || unboundEvent.ProductModelID != 0 {
		t.Fatalf("unbound event was incorrectly bound: %+v", unboundEvent)
	}
}

func TestMQTTIngestMalformedPayloadGoesToDeadLetter(t *testing.T) {
	db, service := setupMQTTIngestTestDB(t)
	connector := createMQTTConnector(t, db, 1, mqttFixtureMapping(mqttUnknownDeviceDeadLetter))

	message := ConnectorMessage{
		TenantID: 1, ConnectorID: connector.ID, Topic: "devices/bad/telemetry",
		MessageID: "bad-json", Payload: []byte(`{"device_id":`),
	}
	result, err := service.Ingest(context.Background(), message)
	if err == nil || !strings.Contains(err.Error(), "JSON object") {
		t.Fatalf("malformed payload error = %v", err)
	}
	if result == nil || result.InboxID == "" {
		t.Fatalf("malformed payload did not preserve inbox evidence: %+v", result)
	}

	var inbox models.ConnectorEventInbox
	if err := db.First(&inbox, "id = ?", result.InboxID).Error; err != nil {
		t.Fatalf("load malformed inbox: %v", err)
	}
	if inbox.Status != models.ConnectorInboxStatusDeadLettered {
		t.Fatalf("malformed inbox status = %q", inbox.Status)
	}
	var deadLetter models.ConnectorDeadLetter
	if err := db.Where("message_id = ? AND failure_stage = ?", message.MessageID, "schema").First(&deadLetter).Error; err != nil {
		t.Fatalf("load malformed payload dead letter: %v", err)
	}
	var telemetryCount int64
	if err := db.Model(&models.DeviceTelemetryEvent{}).Count(&telemetryCount).Error; err != nil || telemetryCount != 0 {
		t.Fatalf("malformed payload wrote telemetry: count=%d err=%v", telemetryCount, err)
	}
}
