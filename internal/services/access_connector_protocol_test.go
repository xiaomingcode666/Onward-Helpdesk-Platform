package services

import (
	"errors"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
)

func TestValidateMQTTBrokerURL(t *testing.T) {
	valid := []string{
		"mqtt://broker.example.com:1883",
		"mqtts://broker.example.com:8883",
		"ws://broker.example.com/mqtt",
		"wss://broker.example.com/mqtt?tenant=1",
	}
	for _, raw := range valid {
		t.Run(raw, func(t *testing.T) {
			parsed, err := validateMQTTBrokerURL(raw)
			if err != nil {
				t.Fatalf("validate MQTT URL: %v", err)
			}
			if parsed.Hostname() != "broker.example.com" {
				t.Fatalf("hostname = %q", parsed.Hostname())
			}
		})
	}

	invalid := []struct {
		name    string
		raw     string
		message string
	}{
		{name: "credentials", raw: "mqtts://user:secret@broker.example.com:8883", message: "must not contain credentials"},
		{name: "missing host", raw: "mqtts:///telemetry", message: "must include a host"},
		{name: "invalid scheme", raw: "ftp://broker.example.com", message: "must use mqtt, mqtts, ws, or wss"},
	}
	for _, item := range invalid {
		t.Run(item.name, func(t *testing.T) {
			_, err := validateMQTTBrokerURL(item.raw)
			if err == nil || !strings.Contains(err.Error(), item.message) {
				t.Fatalf("error = %v, want %q", err, item.message)
			}
		})
	}
}

func TestConnectorEndpointValidationKeepsHTTPAndMQTTSeparate(t *testing.T) {
	if _, err := validateConnectorBaseURL("mqtts://broker.example.com:8883"); err == nil {
		t.Fatal("HTTP validator accepted an MQTT endpoint")
	}
	if _, err := validateConnectorBaseURL("http://127.0.0.1:8080"); err == nil {
		t.Fatal("HTTP validator no longer blocks loopback")
	}
	if _, err := validateConnectorBaseURL("https://api.example.com"); err != nil {
		t.Fatalf("HTTP validator rejected public HTTPS endpoint: %v", err)
	}

	connector := &models.AccessConnector{
		TenantID:      1,
		Name:          "factory telemetry",
		ConnectorType: models.ConnectorTypeMQTT,
		BaseURL:       "mqtts://broker.example.com:8883",
		AuthType:      "none",
		AuthConfig:    `{"clientId":"factory-telemetry"}`,
		FieldMapping:  `{"topics":["devices/+/telemetry"],"qos":1}`,
	}
	if err := validateConnectorConfiguration(connector); err != nil {
		t.Fatalf("validate MQTT connector: %v", err)
	}
	if _, err := AccessConnectorService.buildTestRequest(connector); !errors.Is(err, ErrConnectorWorkerRequired) {
		t.Fatalf("build test request error = %v", err)
	}
}

func TestValidateMQTTConnectorConfigurationFailsBeforeWorkerStartup(t *testing.T) {
	base := models.AccessConnector{
		TenantID:      1,
		Name:          "factory telemetry",
		ConnectorType: models.ConnectorTypeMQTT,
		BaseURL:       "mqtts://broker.example.com:8883",
		AuthType:      "none",
		AuthConfig:    `{"clientId":"factory-telemetry"}`,
		FieldMapping:  `{"topics":["devices/+/telemetry"],"qos":1}`,
	}

	tests := []struct {
		name    string
		mutate  func(*models.AccessConnector)
		message string
	}{
		{name: "missing topics", mutate: func(value *models.AccessConnector) { value.FieldMapping = `{}` }, message: "at least one MQTT topic is required"},
		{name: "invalid qos", mutate: func(value *models.AccessConnector) { value.FieldMapping = `{"topics":["devices/#"],"qos":3}` }, message: "MQTT QoS"},
		{name: "unsupported auth", mutate: func(value *models.AccessConnector) { value.AuthType = "bearer" }, message: "authentication must be none or basic"},
		{name: "basic without username", mutate: func(value *models.AccessConnector) { value.AuthType = "basic" }, message: "requires a username"},
	}
	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			connector := base
			item.mutate(&connector)
			if err := validateConnectorConfiguration(&connector); err == nil || !strings.Contains(err.Error(), item.message) {
				t.Fatalf("error = %v, want %q", err, item.message)
			}
		})
	}
}

func TestMQTTConnectorHealthUsesLastMessageActivity(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	if got := mqttConnectorHealthStatus(nil, now); got != "unknown" {
		t.Fatalf("nil activity status = %q", got)
	}
	recent := now.Add(-2 * time.Minute)
	if got := mqttConnectorHealthStatus(&recent, now); got != "healthy" {
		t.Fatalf("recent activity status = %q", got)
	}
	stale := now.Add(-10 * time.Minute)
	if got := mqttConnectorHealthStatus(&stale, now); got != "degraded" {
		t.Fatalf("stale activity status = %q", got)
	}
	expired := now.Add(-20 * time.Minute)
	if got := mqttConnectorHealthStatus(&expired, now); got != "unhealthy" {
		t.Fatalf("expired activity status = %q", got)
	}
}
