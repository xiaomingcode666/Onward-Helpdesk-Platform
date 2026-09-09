package models

import (
	"time"
)

// AccessConnector 外部数据连接器定义。
type AccessConnector struct {
	ID            int64      `gorm:"primaryKey;autoIncrement"`
	TenantID      int64      `gorm:"type:bigint;not null;index"`
	Name          string     `gorm:"type:varchar(128);not null;default:'';index"`
	ConnectorType string     `gorm:"type:varchar(32);not null;default:'';index"` // openapi / webhook / database / mysql / postgres / api / elasticsearch / mqtt
	BaseURL       string     `gorm:"type:varchar(512);not null;default:''"`
	AuthType      string     `gorm:"type:varchar(32);not null;default:''"` // api_key / basic / bearer / mtls
	AuthConfig    string     `gorm:"type:text"`                            // AES-GCM encrypted credential payload
	FieldMapping  string     `gorm:"type:text"`                            // JSON field mapping rules
	TemplateCode  string     `gorm:"type:varchar(64);not null;default:''"`
	Status        string     `gorm:"type:varchar(20);not null;default:'inactive';index"` // active / inactive / testing
	HealthStatus  string     `gorm:"type:varchar(20);not null;default:'unknown'"`        // healthy / degraded / unhealthy / unknown
	LastTestedAt  *time.Time `gorm:"type:timestamp"`
	LastMessageAt *time.Time `gorm:"type:timestamp;index"` // streaming connector transport activity
	CreatedAt     time.Time  `gorm:"type:timestamp;not null;index"`
	UpdatedAt     time.Time  `gorm:"type:timestamp;not null;index"`
}

// TableName 设置 AccessConnector 表名
func (AccessConnector) TableName() string {
	return "access_connectors"
}

// AccessQueryLog 受控查询调用日志。
type AccessQueryLog struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	ConnectorID  int64     `gorm:"type:bigint;not null;index"`
	TenantID     int64     `gorm:"type:bigint;not null;index"`
	Query        string    `gorm:"type:text;not null"`
	ResultSize   int       `gorm:"type:int;not null;default:0"`
	ResultSample string    `gorm:"type:text"` // 结果样例（脱敏后）
	DurationMs   int64     `gorm:"type:bigint;not null;default:0"`
	Status       string    `gorm:"type:varchar(20);not null;default:'success';index"` // success / failure / blocked
	ErrorMsg     string    `gorm:"type:text"`
	RequestID    string    `gorm:"type:varchar(128);not null;default:'';index"`
	ActorID      string    `gorm:"type:varchar(64);not null;default:'';index"`
	ActorType    string    `gorm:"type:varchar(32);not null;default:''"`
	CreatedAt    time.Time `gorm:"type:timestamp;not null;index"`
}

// TableName 设置 AccessQueryLog 表名
func (AccessQueryLog) TableName() string {
	return "access_query_logs"
}

// 连接器类型常量
const (
	ConnectorTypeMySQL         = "mysql"
	ConnectorTypePostgres      = "postgres"
	ConnectorTypeAPI           = "api"
	ConnectorTypeElasticsearch = "elasticsearch"
	ConnectorTypeFeishu        = "feishu"
	ConnectorTypeMQTT          = "mqtt"
)

// AccessCallLog 连接器调用日志，记录每次外部系统调用的请求和响应摘要。
type AccessCallLog struct {
	ID            string    `gorm:"primaryKey;type:varchar(36)"`
	TenantID      int64     `gorm:"type:bigint;not null;index"`
	ConnectorID   int64     `gorm:"type:bigint;not null;index"`
	RequestURL    string    `gorm:"type:varchar(2048);not null;default:''"`
	RequestMethod string    `gorm:"type:varchar(16);not null;default:''"`
	RequestBody   string    `gorm:"type:text"`
	ResponseCode  int       `gorm:"type:int;not null;default:0"`
	ResponseBody  string    `gorm:"type:text"`
	DurationMs    int64     `gorm:"type:bigint;not null;default:0"`
	ErrorMessage  string    `gorm:"type:text"`
	TraceID       string    `gorm:"type:varchar(128);not null;default:'';index"`
	CreatedAt     time.Time `gorm:"type:timestamp;not null;index"`
}

// TableName 设置 AccessCallLog 表名
func (AccessCallLog) TableName() string {
	return "access_call_logs"
}

// ConnectorSyncCursor stores an adapter's durable source checkpoint.
type ConnectorSyncCursor struct {
	ID            string     `gorm:"primaryKey;type:varchar(36)"`
	TenantID      int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_connector_sync_cursor"`
	ConnectorID   int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_connector_sync_cursor"`
	CursorName    string     `gorm:"type:varchar(128);not null;uniqueIndex:uk_connector_sync_cursor"`
	CursorValue   string     `gorm:"type:text;not null;default:''"`
	LastEventAt   *time.Time `gorm:"type:timestamp;index"`
	LastSuccessAt *time.Time `gorm:"type:timestamp"`
	LastError     string     `gorm:"type:text"`
	CreatedAt     time.Time  `gorm:"type:timestamp;not null;index"`
	UpdatedAt     time.Time  `gorm:"type:timestamp;not null;index"`
}

// ConnectorWorkerLease coordinates one long-running adapter worker per key.
type ConnectorWorkerLease struct {
	ID             string     `gorm:"primaryKey;type:varchar(36)"`
	TenantID       int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_connector_worker_lease"`
	ConnectorID    int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_connector_worker_lease"`
	WorkerKey      string     `gorm:"type:varchar(128);not null;uniqueIndex:uk_connector_worker_lease"`
	LeaseOwner     string     `gorm:"type:varchar(128);not null;default:'';index"`
	FenceToken     int64      `gorm:"type:bigint;not null;default:0"`
	LeaseExpiresAt *time.Time `gorm:"type:timestamp;index"`
	HeartbeatAt    *time.Time `gorm:"type:timestamp"`
	CreatedAt      time.Time  `gorm:"type:timestamp;not null;index"`
	UpdatedAt      time.Time  `gorm:"type:timestamp;not null;index"`
}

// ConnectorEventInbox is the idempotency boundary for inbound connector events.
type ConnectorEventInbox struct {
	ID           string     `gorm:"primaryKey;type:varchar(36)"`
	TenantID     int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_connector_event_inbox"`
	ConnectorID  int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_connector_event_inbox"`
	Topic        string     `gorm:"type:varchar(512);not null;default:'';index;uniqueIndex:uk_connector_event_inbox"`
	MessageID    string     `gorm:"type:varchar(256);not null;default:'';index;uniqueIndex:uk_connector_event_inbox"`
	PayloadHash  string     `gorm:"type:varchar(64);not null;default:''"`
	PayloadJSON  string     `gorm:"type:text;not null;default:''"`
	Status       string     `gorm:"type:varchar(24);not null;default:'received';index"`
	AttemptCount int        `gorm:"type:int;not null;default:1"`
	LastError    string     `gorm:"type:text"`
	ReceivedAt   time.Time  `gorm:"type:timestamp;not null;index"`
	ProcessedAt  *time.Time `gorm:"type:timestamp;index"`
	CreatedAt    time.Time  `gorm:"type:timestamp;not null;index"`
	UpdatedAt    time.Time  `gorm:"type:timestamp;not null;index"`
}

// DeviceTelemetryEvent is the append-only normalized metric history.
type DeviceTelemetryEvent struct {
	ID             string    `gorm:"primaryKey;type:varchar(36)"`
	TenantID       int64     `gorm:"type:bigint;not null;index"`
	ConnectorID    int64     `gorm:"type:bigint;not null;index"`
	InboxID        string    `gorm:"type:varchar(36);not null;index;uniqueIndex:uk_device_telemetry_event_metric"`
	Topic          string    `gorm:"type:varchar(512);not null;default:'';index"`
	MessageID      string    `gorm:"type:varchar(256);not null;default:'';index"`
	DeviceSerial   string    `gorm:"type:varchar(100);not null;default:'';index"`
	DeviceID       int64     `gorm:"type:bigint;not null;default:0;index"`
	ProductID      int64     `gorm:"type:bigint;not null;default:0;index"`
	ProductModelID int64     `gorm:"type:bigint;not null;default:0;index"`
	MetricKey      string    `gorm:"type:varchar(128);not null;default:'';index;uniqueIndex:uk_device_telemetry_event_metric"`
	MetricValue    float64   `gorm:"type:double precision;not null;default:0"`
	MetricUnit     string    `gorm:"type:varchar(32);not null;default:''"`
	FaultCode      string    `gorm:"type:varchar(128);not null;default:'';index"`
	RecordedAt     time.Time `gorm:"type:timestamp;not null;index"`
	RawPayloadHash string    `gorm:"type:varchar(64);not null;default:''"`
	CreatedAt      time.Time `gorm:"type:timestamp;not null;index"`
}

// DeviceTelemetrySnapshot stores the newest value for each connector/device/metric.
type DeviceTelemetrySnapshot struct {
	ID              string    `gorm:"primaryKey;type:varchar(36)"`
	TenantID        int64     `gorm:"type:bigint;not null;index;uniqueIndex:uk_device_telemetry_snapshot"`
	ConnectorID     int64     `gorm:"type:bigint;not null;index;uniqueIndex:uk_device_telemetry_snapshot"`
	DeviceSerial    string    `gorm:"type:varchar(100);not null;default:'';index;uniqueIndex:uk_device_telemetry_snapshot"`
	DeviceID        int64     `gorm:"type:bigint;not null;default:0;index"`
	ProductID       int64     `gorm:"type:bigint;not null;default:0;index"`
	ProductModelID  int64     `gorm:"type:bigint;not null;default:0;index"`
	MetricKey       string    `gorm:"type:varchar(128);not null;default:'';index;uniqueIndex:uk_device_telemetry_snapshot"`
	MetricValue     float64   `gorm:"type:double precision;not null;default:0"`
	MetricUnit      string    `gorm:"type:varchar(32);not null;default:''"`
	FaultCode       string    `gorm:"type:varchar(128);not null;default:'';index"`
	SourceEventID   string    `gorm:"type:varchar(36);not null;default:'';index"`
	SourceMessageID string    `gorm:"type:varchar(256);not null;default:''"`
	RecordedAt      time.Time `gorm:"type:timestamp;not null;index"`
	CreatedAt       time.Time `gorm:"type:timestamp;not null;index"`
	UpdatedAt       time.Time `gorm:"type:timestamp;not null;index"`
}

// DeviceAlarmEvent records a normalized device fault for downstream alerting.
type DeviceAlarmEvent struct {
	ID             string     `gorm:"primaryKey;type:varchar(36)"`
	TenantID       int64      `gorm:"type:bigint;not null;index"`
	ConnectorID    int64      `gorm:"type:bigint;not null;index"`
	InboxID        string     `gorm:"type:varchar(36);not null;index;uniqueIndex:uk_device_alarm_event"`
	DeviceSerial   string     `gorm:"type:varchar(100);not null;default:'';index"`
	DeviceID       int64      `gorm:"type:bigint;not null;default:0;index"`
	ProductID      int64      `gorm:"type:bigint;not null;default:0;index"`
	ProductModelID int64      `gorm:"type:bigint;not null;default:0;index"`
	FaultCode      string     `gorm:"type:varchar(128);not null;default:'';index;uniqueIndex:uk_device_alarm_event"`
	Severity       string     `gorm:"type:varchar(24);not null;default:'warning';index"`
	Message        string     `gorm:"type:text"`
	Status         string     `gorm:"type:varchar(24);not null;default:'open';index"`
	OccurredAt     time.Time  `gorm:"type:timestamp;not null;index"`
	ResolvedAt     *time.Time `gorm:"type:timestamp;index"`
	CreatedAt      time.Time  `gorm:"type:timestamp;not null;index"`
	UpdatedAt      time.Time  `gorm:"type:timestamp;not null;index"`
}

// ConnectorDeadLetter preserves rejected messages for audited replay.
type ConnectorDeadLetter struct {
	ID            string     `gorm:"primaryKey;type:varchar(36)"`
	TenantID      int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_connector_dead_letter"`
	ConnectorID   int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_connector_dead_letter"`
	InboxID       string     `gorm:"type:varchar(36);not null;default:'';index"`
	Topic         string     `gorm:"type:varchar(512);not null;default:'';index;uniqueIndex:uk_connector_dead_letter"`
	MessageID     string     `gorm:"type:varchar(256);not null;default:'';index;uniqueIndex:uk_connector_dead_letter"`
	PayloadHash   string     `gorm:"type:varchar(64);not null;default:'';uniqueIndex:uk_connector_dead_letter"`
	PayloadJSON   string     `gorm:"type:text;not null;default:''"`
	FailureStage  string     `gorm:"type:varchar(64);not null;default:'';index"`
	FailureReason string     `gorm:"type:text;not null;default:''"`
	Status        string     `gorm:"type:varchar(24);not null;default:'pending';index"`
	RetryCount    int        `gorm:"type:int;not null;default:0"`
	NextRetryAt   *time.Time `gorm:"type:timestamp;index"`
	FirstFailedAt time.Time  `gorm:"type:timestamp;not null;index"`
	LastFailedAt  time.Time  `gorm:"type:timestamp;not null;index"`
	CreatedAt     time.Time  `gorm:"type:timestamp;not null;index"`
	UpdatedAt     time.Time  `gorm:"type:timestamp;not null;index"`
}

const (
	ConnectorInboxStatusReceived     = "received"
	ConnectorInboxStatusProcessed    = "processed"
	ConnectorInboxStatusDeadLettered = "dead_lettered"
	ConnectorDeadLetterStatusPending = "pending"
	DeviceAlarmStatusOpen            = "open"
)

// 查询状态常量
const (
	QueryStatusSuccess = "success"
	QueryStatusFailure = "failure"
	QueryStatusBlocked = "blocked"
)
