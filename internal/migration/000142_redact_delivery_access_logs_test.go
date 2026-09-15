package migration

import (
	"fmt"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/logprivacy"
	"remotehelpdesk/internal/pkg/secretstore"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRedactDeliveryAccessLogsPreservesRetryAndMetadata(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqls.SetDB(db)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	require.NoError(t, db.AutoMigrate(&models.DeliveryLog{}, &models.AccessCallLog{}))
	require.NoError(t, db.Migrator().DropColumn(&models.DeliveryLog{}, "RecipientCiphertext"))
	// More than one batch, including other tenants and a non-email channel.
	for i := 1; i <= 205; i++ {
		require.NoError(t, db.Table("delivery_logs").Create(map[string]any{
			"id": i, "tenant_id": i%2 + 1, "notification_id": i, "channel": "email", "recipient_id": "customer@example.test",
			"status": "waiting_retry", "retry_count": 1, "error_msg": "secret in remote error", "created_at": time.Now(), "updated_at": time.Now(),
		}).Error)
		require.NoError(t, db.Create(&models.AccessCallLog{ID: fmt.Sprintf("call-%03d", i), TenantID: int64(i%2 + 1), ConnectorID: 10,
			RequestURL: "https://example.test/secret?password=hidden", RequestBody: "private request", ResponseBody: "private response",
			ResponseCode: 403, DurationMs: 27, TraceID: "test-trace", ErrorMessage: "private error", CreatedAt: time.Now()}).Error)
	}
	require.NoError(t, db.Table("delivery_logs").Create(map[string]any{"id": 206, "notification_id": 0, "channel": "email", "recipient_id": "sent@example.test", "status": "sent", "created_at": time.Now(), "updated_at": time.Now()}).Error)
	require.NoError(t, db.Table("delivery_logs").Create(map[string]any{"id": 207, "notification_id": 0, "channel": "push", "recipient_id": "123", "status": "pending", "created_at": time.Now(), "updated_at": time.Now()}).Error)
	require.NoError(t, redactDeliveryAccessLogs())
	var row models.DeliveryLog
	require.NoError(t, db.First(&row, 205).Error)
	require.Equal(t, logprivacy.Redacted, row.RecipientID)
	plain, err := secretstore.Decrypt(row.RecipientCiphertext)
	require.NoError(t, err)
	require.Equal(t, "customer@example.test", plain)
	require.Equal(t, "waiting_retry", row.Status)
	require.Equal(t, 1, row.RetryCount)
	ciphertext := row.RecipientCiphertext
	require.NoError(t, redactDeliveryAccessLogs())
	require.NoError(t, db.First(&row, 205).Error)
	require.Equal(t, ciphertext, row.RecipientCiphertext)
	var sent, push models.DeliveryLog
	require.NoError(t, db.First(&sent, 206).Error)
	require.Empty(t, sent.RecipientCiphertext)
	require.Equal(t, logprivacy.Redacted, sent.RecipientID)
	require.NoError(t, db.First(&push, 207).Error)
	require.Equal(t, "123", push.RecipientID)
	var logs []models.AccessCallLog
	require.NoError(t, db.Find(&logs).Error)
	require.Len(t, logs, 205)
	for _, log := range logs {
		require.Equal(t, logprivacy.Redacted, log.RequestURL)
		require.Equal(t, logprivacy.Redacted, log.RequestBody)
		require.Equal(t, logprivacy.Redacted, log.ResponseBody)
		require.Equal(t, logprivacy.Redacted, log.ErrorMessage)
		require.Equal(t, 403, log.ResponseCode)
		require.EqualValues(t, 27, log.DurationMs)
		require.Equal(t, "test-trace", log.TraceID)
	}
}
