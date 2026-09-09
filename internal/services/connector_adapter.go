package services

import (
	"context"
	"time"

	"remotehelpdesk/internal/models"
)

// ConnectorMessage is the protocol-neutral envelope delivered by a background adapter.
type ConnectorMessage struct {
	TenantID    int64
	ConnectorID int64
	Topic       string
	MessageID   string
	Payload     []byte
	ReceivedAt  time.Time
}

// ConnectorIngestResult describes durable processing without leaking protocol clients.
type ConnectorIngestResult struct {
	InboxID           string
	TelemetryEventIDs []string
	AlarmID           string
	Duplicate         bool
}

// ConnectorMessageSink is the transaction boundary between a protocol adapter and domain persistence.
type ConnectorMessageSink interface {
	Ingest(ctx context.Context, message ConnectorMessage) (*ConnectorIngestResult, error)
}

// ConnectorCursorStore persists source offsets independently of handler requests.
type ConnectorCursorStore interface {
	Load(ctx context.Context, tenantID, connectorID int64, cursorName string) (*models.ConnectorSyncCursor, error)
	Save(ctx context.Context, cursor *models.ConnectorSyncCursor) error
}

// ConnectorLeaseStore prevents multiple workers from consuming the same connector partition.
type ConnectorLeaseStore interface {
	Acquire(ctx context.Context, tenantID, connectorID int64, workerKey, owner string, ttl time.Duration) (fenceToken int64, acquired bool, err error)
	Renew(ctx context.Context, tenantID, connectorID int64, workerKey, owner string, fenceToken int64, ttl time.Duration) (bool, error)
	Release(ctx context.Context, tenantID, connectorID int64, workerKey, owner string, fenceToken int64) error
}

// ConnectorAdapter owns protocol I/O. Run may block and must only be called by a supervised background worker.
type ConnectorAdapter interface {
	ConnectorType() string
	Validate(connector *models.AccessConnector) error
	Run(ctx context.Context, connector *models.AccessConnector, runtime ConnectorWorkerRuntime) error
}

// ConnectorWorkerRuntime contains only worker-scoped dependencies. HTTP handlers must not call adapter.Run.
type ConnectorWorkerRuntime struct {
	WorkerID string
	Sink     ConnectorMessageSink
	Cursors  ConnectorCursorStore
	Leases   ConnectorLeaseStore
}
