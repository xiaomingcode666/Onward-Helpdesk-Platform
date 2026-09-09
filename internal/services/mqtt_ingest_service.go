package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/pkg/utils"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	mqttPayloadLimit            = 1 << 20
	mqttErrorLimit              = 4096
	mqttUnknownDeviceDeadLetter = "dead_letter"
	mqttUnknownDeviceAccept     = "accept_unbound"
)

var MQTTIngestService = NewMQTTMessageIngestService(nil)

type mqttMessageIngestService struct {
	db  *gorm.DB
	now func() time.Time
}

func NewMQTTMessageIngestService(db *gorm.DB) *mqttMessageIngestService {
	return &mqttMessageIngestService{db: db, now: time.Now}
}

func (s *mqttMessageIngestService) database() *gorm.DB {
	if s.db != nil {
		return s.db
	}
	return sqls.DB()
}

// Ingest validates and durably applies one message delivered by an MQTT worker.
func (s *mqttMessageIngestService) Ingest(ctx context.Context, message ConnectorMessage) (*ConnectorIngestResult, error) {
	if message.TenantID <= 0 || message.ConnectorID <= 0 {
		return nil, fmt.Errorf("tenant and connector are required")
	}
	message.Topic = strings.TrimSpace(message.Topic)
	message.MessageID = strings.TrimSpace(message.MessageID)
	if message.Topic == "" || len(message.Topic) > 512 {
		return nil, fmt.Errorf("MQTT topic is required and must not exceed 512 characters")
	}
	if message.MessageID == "" || len(message.MessageID) > 256 {
		return nil, fmt.Errorf("MQTT message ID is required and must not exceed 256 characters")
	}
	if message.ReceivedAt.IsZero() {
		message.ReceivedAt = s.now().UTC()
	} else {
		message.ReceivedAt = message.ReceivedAt.UTC()
	}

	payloadHash := hashConnectorPayload(message.Payload)
	storedPayload := string(message.Payload)
	if len(storedPayload) > mqttPayloadLimit {
		storedPayload = storedPayload[:mqttPayloadLimit]
	}
	result := &ConnectorIngestResult{}
	var processErr error

	err := s.database().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		connector, err := loadMQTTConnectorForIngest(tx, message.TenantID, message.ConnectorID)
		if err != nil {
			return err
		}

		now := s.now().UTC()
		inbox := &models.ConnectorEventInbox{
			ID:           utils.UUID(),
			TenantID:     message.TenantID,
			ConnectorID:  message.ConnectorID,
			Topic:        message.Topic,
			MessageID:    message.MessageID,
			PayloadHash:  payloadHash,
			PayloadJSON:  storedPayload,
			Status:       models.ConnectorInboxStatusReceived,
			AttemptCount: 1,
			ReceivedAt:   message.ReceivedAt,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(inbox)
		if created.Error != nil {
			return fmt.Errorf("create connector event inbox: %w", created.Error)
		}
		if created.RowsAffected == 0 {
			var existing models.ConnectorEventInbox
			if err := tx.Where(
				"tenant_id = ? AND connector_id = ? AND topic = ? AND message_id = ?",
				message.TenantID, message.ConnectorID, message.Topic, message.MessageID,
			).First(&existing).Error; err != nil {
				return fmt.Errorf("load connector event inbox: %w", err)
			}
			result.InboxID = existing.ID
			if err := touchMQTTConnectorActivity(tx, connector, message.ReceivedAt); err != nil {
				return err
			}
			if existing.PayloadHash == payloadHash {
				result.Duplicate = true
				if existing.Status == models.ConnectorInboxStatusDeadLettered {
					processErr = fmt.Errorf("MQTT message was previously dead-lettered: %s", existing.LastError)
				}
				return nil
			}

			processErr = fmt.Errorf("MQTT message ID was reused with a different payload")
			return createConnectorDeadLetter(tx, message, existing.ID, payloadHash, storedPayload, "idempotency", processErr, now)
		}
		result.InboxID = inbox.ID

		if err := touchMQTTConnectorActivity(tx, connector, message.ReceivedAt); err != nil {
			return err
		}

		normalized, stage, err := normalizeMQTTMessage(message.Payload, connector.FieldMapping, message.ReceivedAt)
		if err != nil {
			processErr = err
			if err := markConnectorInboxDeadLettered(tx, inbox.ID, err, now); err != nil {
				return err
			}
			return createConnectorDeadLetter(tx, message, inbox.ID, payloadHash, storedPayload, stage, err, now)
		}
		device, err := resolveMQTTDevice(tx, message.TenantID, normalized.DeviceSerial, normalized.UnknownDevicePolicy)
		if err != nil {
			processErr = err
			if err := markConnectorInboxDeadLettered(tx, inbox.ID, err, now); err != nil {
				return err
			}
			return createConnectorDeadLetter(tx, message, inbox.ID, payloadHash, storedPayload, "device_resolution", err, now)
		}
		var deviceID, productID, productModelID int64
		if device != nil {
			deviceID = device.ID
			productID = device.ProductID
			productModelID = device.ProductModelID
			if strings.TrimSpace(device.SerialNo) != "" {
				normalized.DeviceSerial = strings.TrimSpace(device.SerialNo)
			} else {
				normalized.DeviceSerial = strings.TrimSpace(device.DeviceNo)
			}
		}

		for _, metric := range normalized.Metrics {
			event := &models.DeviceTelemetryEvent{
				ID:             utils.UUID(),
				TenantID:       message.TenantID,
				ConnectorID:    message.ConnectorID,
				InboxID:        inbox.ID,
				Topic:          message.Topic,
				MessageID:      message.MessageID,
				DeviceSerial:   normalized.DeviceSerial,
				DeviceID:       deviceID,
				ProductID:      productID,
				ProductModelID: productModelID,
				MetricKey:      metric.Key,
				MetricValue:    metric.Value,
				MetricUnit:     metric.Unit,
				FaultCode:      normalized.FaultCode,
				RecordedAt:     normalized.RecordedAt,
				RawPayloadHash: payloadHash,
				CreatedAt:      now,
			}
			createdEvent := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(event)
			if createdEvent.Error != nil {
				return fmt.Errorf("create telemetry event: %w", createdEvent.Error)
			}
			if createdEvent.RowsAffected > 0 {
				result.TelemetryEventIDs = append(result.TelemetryEventIDs, event.ID)
			}
			if err := upsertTelemetrySnapshot(tx, event, now); err != nil {
				return err
			}
		}

		if normalized.FaultCode != "" {
			alarm := &models.DeviceAlarmEvent{
				ID:             utils.UUID(),
				TenantID:       message.TenantID,
				ConnectorID:    message.ConnectorID,
				InboxID:        inbox.ID,
				DeviceSerial:   normalized.DeviceSerial,
				DeviceID:       deviceID,
				ProductID:      productID,
				ProductModelID: productModelID,
				FaultCode:      normalized.FaultCode,
				Severity:       normalized.Severity,
				Message:        normalized.AlarmMessage,
				Status:         models.DeviceAlarmStatusOpen,
				OccurredAt:     normalized.RecordedAt,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			createdAlarm := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(alarm)
			if createdAlarm.Error != nil {
				return fmt.Errorf("create device alarm: %w", createdAlarm.Error)
			}
			if createdAlarm.RowsAffected > 0 {
				result.AlarmID = alarm.ID
				eventID := "tenant:" + strconv.FormatInt(message.TenantID, 10) + ":device_alarm.raised:" + alarm.ID
				if _, err := eventbus.EnqueueTx(tx, eventbus.DurableEvent{
					TenantID:       message.TenantID,
					IdempotencyKey: eventID,
					EventType:      events.EventDeviceAlarmRaised,
					Payload: events.DeviceAlarmRaisedEvent{
						EventID: eventID, AlarmID: alarm.ID, TenantID: alarm.TenantID, ConnectorID: alarm.ConnectorID,
						DeviceID: alarm.DeviceID, ProductID: alarm.ProductID, ProductModelID: alarm.ProductModelID,
						DeviceSerial: alarm.DeviceSerial, FaultCode: alarm.FaultCode, Severity: alarm.Severity,
						Message: alarm.Message, OccurredAt: alarm.OccurredAt,
					},
					Source:      "mqtt_ingest_service",
					AggregateID: alarm.ID,
					ActorType:   "connector",
					CreatedAt:   now,
				}); err != nil {
					return fmt.Errorf("enqueue device alarm event: %w", err)
				}
			}
		}

		processedAt := now
		if err := tx.Model(&models.ConnectorEventInbox{}).Where("id = ?", inbox.ID).Updates(map[string]any{
			"status":       models.ConnectorInboxStatusProcessed,
			"last_error":   "",
			"processed_at": &processedAt,
			"updated_at":   now,
		}).Error; err != nil {
			return fmt.Errorf("mark connector event processed: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if processErr != nil {
		return result, processErr
	}
	if result.AlarmID != "" {
		eventbus.WakeDefaultOutboxPublisher()
	}
	return result, nil
}

func resolveMQTTDevice(tx *gorm.DB, tenantID int64, serialOrNo, unknownDevicePolicy string) (*models.Device, error) {
	var devices []models.Device
	if err := tx.Where("tenant_id = ? AND (serial_no = ? OR device_no = ?)", tenantID, serialOrNo, serialOrNo).
		Order("id ASC").Limit(2).Find(&devices).Error; err != nil {
		return nil, fmt.Errorf("resolve MQTT device: %w", err)
	}
	if len(devices) == 0 {
		if unknownDevicePolicy == mqttUnknownDeviceAccept {
			return nil, nil
		}
		return nil, fmt.Errorf("no device matched tenant %d and serial/no %q", tenantID, serialOrNo)
	}
	if len(devices) > 1 {
		return nil, fmt.Errorf("multiple devices matched tenant %d and serial/no %q", tenantID, serialOrNo)
	}
	return &devices[0], nil
}

func loadMQTTConnectorForIngest(tx *gorm.DB, tenantID, connectorID int64) (*models.AccessConnector, error) {
	var connector models.AccessConnector
	if err := tx.Where("id = ? AND tenant_id = ?", connectorID, tenantID).First(&connector).Error; err != nil {
		return nil, fmt.Errorf("MQTT connector not found: %d", connectorID)
	}
	if connector.ConnectorType != models.ConnectorTypeMQTT {
		return nil, fmt.Errorf("connector %d is not an MQTT connector", connectorID)
	}
	if connector.Status != "active" && connector.Status != "testing" {
		return nil, fmt.Errorf("MQTT connector is not active, current status: %s", connector.Status)
	}
	if _, err := validateMQTTBrokerURL(connector.BaseURL); err != nil {
		return nil, err
	}
	return &connector, nil
}

func touchMQTTConnectorActivity(tx *gorm.DB, connector *models.AccessConnector, receivedAt time.Time) error {
	updates := map[string]any{
		"last_message_at": receivedAt,
		"health_status":   "healthy",
		"updated_at":      receivedAt,
	}
	result := tx.Model(&models.AccessConnector{}).
		Where("id = ? AND tenant_id = ? AND (last_message_at IS NULL OR last_message_at <= ?)", connector.ID, connector.TenantID, receivedAt).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update MQTT connector activity: %w", result.Error)
	}
	return nil
}

func markConnectorInboxDeadLettered(tx *gorm.DB, inboxID string, cause error, now time.Time) error {
	if err := tx.Model(&models.ConnectorEventInbox{}).Where("id = ?", inboxID).Updates(map[string]any{
		"status":     models.ConnectorInboxStatusDeadLettered,
		"last_error": truncateConnectorText(cause.Error(), mqttErrorLimit),
		"updated_at": now,
	}).Error; err != nil {
		return fmt.Errorf("mark connector event dead-lettered: %w", err)
	}
	return nil
}

func createConnectorDeadLetter(
	tx *gorm.DB,
	message ConnectorMessage,
	inboxID, payloadHash, storedPayload, stage string,
	cause error,
	now time.Time,
) error {
	item := &models.ConnectorDeadLetter{
		ID:            utils.UUID(),
		TenantID:      message.TenantID,
		ConnectorID:   message.ConnectorID,
		InboxID:       inboxID,
		Topic:         message.Topic,
		MessageID:     message.MessageID,
		PayloadHash:   payloadHash,
		PayloadJSON:   storedPayload,
		FailureStage:  stage,
		FailureReason: truncateConnectorText(cause.Error(), mqttErrorLimit),
		Status:        models.ConnectorDeadLetterStatusPending,
		FirstFailedAt: now,
		LastFailedAt:  now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(item).Error; err != nil {
		return fmt.Errorf("create connector dead letter: %w", err)
	}
	return nil
}

func upsertTelemetrySnapshot(tx *gorm.DB, event *models.DeviceTelemetryEvent, now time.Time) error {
	snapshot := &models.DeviceTelemetrySnapshot{
		ID:              utils.UUID(),
		TenantID:        event.TenantID,
		ConnectorID:     event.ConnectorID,
		DeviceSerial:    event.DeviceSerial,
		DeviceID:        event.DeviceID,
		ProductID:       event.ProductID,
		ProductModelID:  event.ProductModelID,
		MetricKey:       event.MetricKey,
		MetricValue:     event.MetricValue,
		MetricUnit:      event.MetricUnit,
		FaultCode:       event.FaultCode,
		SourceEventID:   event.ID,
		SourceMessageID: event.MessageID,
		RecordedAt:      event.RecordedAt,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(snapshot)
	if created.Error != nil {
		return fmt.Errorf("create telemetry snapshot: %w", created.Error)
	}
	if created.RowsAffected > 0 {
		return nil
	}
	updated := tx.Model(&models.DeviceTelemetrySnapshot{}).
		Where("tenant_id = ? AND connector_id = ? AND device_serial = ? AND metric_key = ? AND recorded_at <= ?",
			event.TenantID, event.ConnectorID, event.DeviceSerial, event.MetricKey, event.RecordedAt).
		Updates(map[string]any{
			"metric_value":      event.MetricValue,
			"metric_unit":       event.MetricUnit,
			"fault_code":        event.FaultCode,
			"device_id":         event.DeviceID,
			"product_id":        event.ProductID,
			"product_model_id":  event.ProductModelID,
			"source_event_id":   event.ID,
			"source_message_id": event.MessageID,
			"recorded_at":       event.RecordedAt,
			"updated_at":        now,
		})
	if updated.Error != nil {
		return fmt.Errorf("update telemetry snapshot: %w", updated.Error)
	}
	return nil
}

type mqttFieldMapping struct {
	SourceField      string `json:"sourceField"`
	SourceExpression string `json:"sourceExpression"`
	TargetField      string `json:"targetField"`
	TargetObject     string `json:"targetObject"`
	TransformType    string `json:"transformType"`
	Status           string `json:"status"`
	External         string `json:"external"`
	Standard         string `json:"standard"`
}

type mqttExplicitMapping struct {
	UnknownDevicePolicy string             `json:"unknownDevicePolicy"`
	Mappings            []mqttFieldMapping `json:"mappings"`
	DeviceSerial        string             `json:"deviceSerial"`
	MetricName          string             `json:"metricName"`
	MetricValue         string             `json:"metricValue"`
	MetricUnit          string             `json:"metricUnit"`
	FaultCode           string             `json:"faultCode"`
	Severity            string             `json:"severity"`
	AlarmMessage        string             `json:"alarmMessage"`
	RecordedAt          string             `json:"recordedAt"`
	Metrics             map[string]string  `json:"metrics"`
}

type mqttMappingAssignment struct {
	Source string
	Target string
}

type normalizedMQTTMessage struct {
	DeviceSerial        string
	Metrics             []normalizedMQTTMetric
	FaultCode           string
	Severity            string
	AlarmMessage        string
	RecordedAt          time.Time
	UnknownDevicePolicy string
}

type normalizedMQTTMetric struct {
	Key   string
	Value float64
	Unit  string
}

func normalizeMQTTMessage(payload []byte, rawMapping string, receivedAt time.Time) (*normalizedMQTTMessage, string, error) {
	if len(payload) == 0 {
		return nil, "schema", fmt.Errorf("MQTT payload must not be empty")
	}
	if len(payload) > mqttPayloadLimit {
		return nil, "schema", fmt.Errorf("MQTT payload exceeds 1 MiB")
	}

	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var document map[string]any
	if err := decoder.Decode(&document); err != nil {
		return nil, "schema", fmt.Errorf("MQTT payload must be a JSON object: %w", err)
	}
	if document == nil {
		return nil, "schema", fmt.Errorf("MQTT payload must be a JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, "schema", fmt.Errorf("MQTT payload must contain one JSON object")
	}

	assignments, unknownDevicePolicy, err := parseMQTTFieldMapping(rawMapping)
	if err != nil {
		return nil, "mapping", err
	}
	normalized, err := applyMQTTFieldMapping(document, assignments, receivedAt)
	if err != nil {
		return nil, "schema", err
	}
	normalized.UnknownDevicePolicy = unknownDevicePolicy
	return normalized, "", nil
}

func parseMQTTFieldMapping(raw string) ([]mqttMappingAssignment, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" || raw == "{}" {
		return nil, mqttUnknownDeviceDeadLetter, nil
	}
	if strings.HasPrefix(raw, "[") {
		var mappings []mqttFieldMapping
		if err := json.Unmarshal([]byte(raw), &mappings); err != nil {
			return nil, "", fmt.Errorf("invalid MQTT field mapping: %w", err)
		}
		assignments, err := parseMQTTMappingRows(mappings)
		return assignments, mqttUnknownDeviceDeadLetter, err
	}

	var mapping mqttExplicitMapping
	if err := json.Unmarshal([]byte(raw), &mapping); err != nil {
		return nil, "", fmt.Errorf("invalid MQTT field mapping: %w", err)
	}
	policy := strings.ToLower(strings.TrimSpace(mapping.UnknownDevicePolicy))
	if policy == "" {
		policy = mqttUnknownDeviceDeadLetter
	}
	if policy != mqttUnknownDeviceDeadLetter && policy != mqttUnknownDeviceAccept {
		return nil, "", fmt.Errorf("unknownDevicePolicy must be dead_letter or accept_unbound")
	}
	if len(mapping.Mappings) > 0 {
		assignments, err := parseMQTTMappingRows(mapping.Mappings)
		return assignments, policy, err
	}
	assignments := []mqttMappingAssignment{
		{Source: mapping.DeviceSerial, Target: "device.serialNo"},
		{Source: mapping.MetricName, Target: "deviceTelemetry.metricName"},
		{Source: mapping.MetricValue, Target: "deviceTelemetry.metricValue"},
		{Source: mapping.MetricUnit, Target: "deviceTelemetry.unit"},
		{Source: mapping.FaultCode, Target: "faultCode.code"},
		{Source: mapping.Severity, Target: "deviceEvent.severity"},
		{Source: mapping.AlarmMessage, Target: "deviceEvent.message"},
		{Source: mapping.RecordedAt, Target: "deviceTelemetry.recordedAt"},
	}
	for metric, source := range mapping.Metrics {
		assignments = append(assignments, mqttMappingAssignment{Source: source, Target: "deviceTelemetry.metrics." + metric})
	}
	filtered := assignments[:0]
	for _, assignment := range assignments {
		if strings.TrimSpace(assignment.Source) != "" {
			filtered = append(filtered, assignment)
		}
	}
	if len(filtered) == 0 {
		return nil, "", fmt.Errorf("MQTT field mapping does not define any supported fields")
	}
	return filtered, policy, nil
}

func parseMQTTMappingRows(mappings []mqttFieldMapping) ([]mqttMappingAssignment, error) {
	assignments := make([]mqttMappingAssignment, 0, len(mappings))
	for _, mapping := range mappings {
		if strings.EqualFold(mapping.Status, "inactive") {
			continue
		}
		transform := strings.ToLower(strings.TrimSpace(mapping.TransformType))
		if transform != "" && transform != "direct" && transform != "jsonpath" {
			return nil, fmt.Errorf("unsupported MQTT mapping transform %q", mapping.TransformType)
		}
		source := strings.TrimSpace(mapping.SourceExpression)
		if source == "" {
			source = strings.TrimSpace(mapping.SourceField)
		}
		if source == "" {
			source = strings.TrimSpace(mapping.External)
		}
		target := strings.Trim(strings.TrimSpace(mapping.TargetObject)+"."+strings.TrimSpace(mapping.TargetField), ".")
		if strings.TrimSpace(mapping.Standard) != "" {
			target = strings.TrimSpace(mapping.Standard)
		}
		if source == "" || target == "" {
			return nil, fmt.Errorf("MQTT field mapping requires source and target")
		}
		assignments = append(assignments, mqttMappingAssignment{Source: source, Target: target})
	}
	return assignments, nil
}

func applyMQTTFieldMapping(document map[string]any, assignments []mqttMappingAssignment, receivedAt time.Time) (*normalizedMQTTMessage, error) {
	if len(assignments) == 0 {
		assignments = defaultMQTTFieldMappings(document)
	}

	normalized := &normalizedMQTTMessage{RecordedAt: receivedAt, Severity: "warning"}
	metricValues := map[string]any{}
	metricUnits := map[string]string{}
	var genericMetricName string
	var genericMetricValue any
	var genericMetricUnit string
	var recordedAtValue any

	for _, assignment := range assignments {
		value, ok := lookupMQTTValue(document, assignment.Source)
		if !ok || value == nil {
			continue
		}
		target := strings.ToLower(strings.TrimSpace(assignment.Target))
		switch {
		case target == "device.serialno" || target == "device.serial" || target == "device.externalid":
			normalized.DeviceSerial = connectorStringValue(value)
		case target == "devicetelemetry.metric" || target == "devicetelemetry.metricname" || target == "devicetelemetry.name":
			genericMetricName = connectorStringValue(value)
		case target == "devicetelemetry.value" || target == "devicetelemetry.metricvalue":
			genericMetricValue = value
		case target == "devicetelemetry.unit" || target == "devicetelemetry.metricunit":
			genericMetricUnit = connectorStringValue(value)
		case strings.HasPrefix(target, "devicetelemetry.metrics."):
			key := strings.TrimSpace(assignment.Target[len("deviceTelemetry.metrics."):])
			if key != "" {
				metricValues[key] = value
			}
		case strings.HasPrefix(target, "devicetelemetry.units."):
			key := strings.TrimSpace(assignment.Target[len("deviceTelemetry.units."):])
			if key != "" {
				metricUnits[key] = connectorStringValue(value)
			}
		case target == "faultcode.code" || target == "faultcode.faultcode" || target == "deviceevent.faultcode":
			normalized.FaultCode = connectorStringValue(value)
		case target == "deviceevent.severity" || target == "faultcode.severity":
			normalized.Severity = strings.ToLower(connectorStringValue(value))
		case target == "deviceevent.message" || target == "deviceevent.description" || target == "faultcode.description":
			normalized.AlarmMessage = connectorStringValue(value)
		case target == "devicetelemetry.recordedat" || target == "devicetelemetry.timestamp" || target == "deviceevent.occurredat":
			recordedAtValue = value
		}
	}

	if strings.TrimSpace(genericMetricName) != "" {
		metricValues[strings.TrimSpace(genericMetricName)] = genericMetricValue
		metricUnits[strings.TrimSpace(genericMetricName)] = genericMetricUnit
	}
	if recordedAtValue != nil {
		parsed, err := parseMQTTRecordedAt(recordedAtValue)
		if err != nil {
			return nil, err
		}
		normalized.RecordedAt = parsed
	}

	normalized.DeviceSerial = strings.TrimSpace(normalized.DeviceSerial)
	if normalized.DeviceSerial == "" {
		return nil, fmt.Errorf("mapped device serial is required")
	}
	if len(normalized.DeviceSerial) > 100 {
		return nil, fmt.Errorf("mapped device serial exceeds 100 characters")
	}
	normalized.FaultCode = strings.TrimSpace(normalized.FaultCode)
	if len(normalized.FaultCode) > 128 {
		return nil, fmt.Errorf("mapped fault code exceeds 128 characters")
	}
	if normalized.FaultCode != "" {
		switch normalized.Severity {
		case "info", "warning", "critical":
		case "":
			normalized.Severity = "warning"
		default:
			return nil, fmt.Errorf("mapped alarm severity must be info, warning, or critical")
		}
		if strings.TrimSpace(normalized.AlarmMessage) == "" {
			normalized.AlarmMessage = "device reported fault " + normalized.FaultCode
		}
	}
	if len(normalized.AlarmMessage) > 2000 {
		return nil, fmt.Errorf("mapped alarm message exceeds 2000 characters")
	}

	keys := make([]string, 0, len(metricValues))
	for key := range metricValues {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, rawKey := range keys {
		key := strings.TrimSpace(rawKey)
		if key == "" || len(key) > 128 {
			return nil, fmt.Errorf("mapped metric key is invalid")
		}
		value, err := connectorNumericValue(metricValues[rawKey])
		if err != nil {
			return nil, fmt.Errorf("mapped metric %q must be numeric", key)
		}
		unit := strings.TrimSpace(metricUnits[rawKey])
		if unit == "" {
			unit = strings.TrimSpace(genericMetricUnit)
		}
		if len(unit) > 32 {
			return nil, fmt.Errorf("mapped metric unit exceeds 32 characters")
		}
		normalized.Metrics = append(normalized.Metrics, normalizedMQTTMetric{Key: key, Value: value, Unit: unit})
	}
	if len(normalized.Metrics) == 0 && normalized.FaultCode == "" {
		return nil, fmt.Errorf("MQTT payload must map at least one metric or fault code")
	}
	return normalized, nil
}

func defaultMQTTFieldMappings(document map[string]any) []mqttMappingAssignment {
	assignments := []mqttMappingAssignment{
		{Source: firstExistingMQTTPath(document, "deviceSerial", "device_serial", "serialNo", "device_id"), Target: "device.serialNo"},
		{Source: firstExistingMQTTPath(document, "metric", "metricName", "metric_name"), Target: "deviceTelemetry.metricName"},
		{Source: firstExistingMQTTPath(document, "value", "metricValue", "metric_value"), Target: "deviceTelemetry.metricValue"},
		{Source: firstExistingMQTTPath(document, "unit", "metricUnit", "metric_unit"), Target: "deviceTelemetry.unit"},
		{Source: firstExistingMQTTPath(document, "faultCode", "fault_code"), Target: "faultCode.code"},
		{Source: firstExistingMQTTPath(document, "severity"), Target: "deviceEvent.severity"},
		{Source: firstExistingMQTTPath(document, "message", "alarmMessage", "alarm_message"), Target: "deviceEvent.message"},
		{Source: firstExistingMQTTPath(document, "recordedAt", "recorded_at", "timestamp"), Target: "deviceTelemetry.recordedAt"},
	}
	if rawMetrics, ok := document["metrics"].(map[string]any); ok {
		keys := make([]string, 0, len(rawMetrics))
		for key := range rawMetrics {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			assignments = append(assignments, mqttMappingAssignment{Source: "$.metrics." + key, Target: "deviceTelemetry.metrics." + key})
		}
	}
	return assignments
}

func firstExistingMQTTPath(document map[string]any, candidates ...string) string {
	for _, candidate := range candidates {
		if _, ok := document[candidate]; ok {
			return "$." + candidate
		}
	}
	return ""
}

func lookupMQTTValue(document map[string]any, path string) (any, bool) {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "$")
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return nil, false
	}
	var current any = document
	for _, segment := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func connectorStringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return ""
	}
}

func connectorNumericValue(value any) (float64, error) {
	switch typed := value.(type) {
	case json.Number:
		return typed.Float64()
	case float64:
		return typed, nil
	case float32:
		return float64(typed), nil
	case int:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	default:
		return 0, fmt.Errorf("not numeric")
	}
}

func parseMQTTRecordedAt(value any) (time.Time, error) {
	switch typed := value.(type) {
	case string:
		parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(typed))
		if err != nil {
			return time.Time{}, fmt.Errorf("mapped recorded time must use RFC3339")
		}
		return parsed.UTC(), nil
	case json.Number:
		unixValue, err := typed.Int64()
		if err != nil {
			return time.Time{}, fmt.Errorf("mapped recorded time must be an integer Unix timestamp")
		}
		if unixValue > 1_000_000_000_000 {
			return time.UnixMilli(unixValue).UTC(), nil
		}
		return time.Unix(unixValue, 0).UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("mapped recorded time must use RFC3339 or a Unix timestamp")
	}
}

func hashConnectorPayload(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func truncateConnectorText(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}
