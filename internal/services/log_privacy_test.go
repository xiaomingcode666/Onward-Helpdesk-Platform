package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/logprivacy"
	"remotehelpdesk/internal/pkg/secretstore"

	"github.com/stretchr/testify/require"
)

type privacyLogBuffer struct {
	mu sync.Mutex
	bytes.Buffer
}

func (buffer *privacyLogBuffer) Write(p []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.Buffer.Write(p)
}
func (buffer *privacyLogBuffer) text() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.Buffer.String()
}

func TestEmailRuntimeLogsNeverContainRecipientOrSMTPReply(t *testing.T) {
	setupNotificationDeliveryTestDB(t)
	previousConfig := config.CurrentOrDefault()
	previousLogger := slog.Default()
	t.Cleanup(func() { config.SetCurrent(&previousConfig); slog.SetDefault(previousLogger) })
	var output privacyLogBuffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	for _, reject := range []bool{false, true} {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		done := make(chan struct{})
		go func() {
			defer close(done)
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
			reader := bufio.NewReader(conn)
			_, _ = fmt.Fprint(conn, "220 localhost fixture\r\n")
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				switch {
				case strings.HasPrefix(line, "RCPT") && reject:
					_, _ = fmt.Fprint(conn, "550 customer@example.test password=secret-value\r\n")
					return
				case strings.HasPrefix(line, "DATA"):
					_, _ = fmt.Fprint(conn, "354 data\r\n")
					for {
						line, err = reader.ReadString('\n')
						if err != nil {
							return
						}
						if line == ".\r\n" {
							break
						}
					}
					_, _ = fmt.Fprint(conn, "250 received\r\n")
				case strings.HasPrefix(line, "QUIT"):
					_, _ = fmt.Fprint(conn, "221 bye\r\n")
					return
				default:
					_, _ = fmt.Fprint(conn, "250 localhost\r\n")
				}
			}
		}()
		cfg := previousConfig
		cfg.Email = config.EmailConfig{SMTPHost: "127.0.0.1", SMTPPort: listener.Addr().(*net.TCPAddr).Port, FromAddress: "sender@example.test"}
		config.SetCurrent(&cfg)
		err = EmailNotificationService.SendEmail(0, "customer@example.test", "", "notification_generic", map[string]interface{}{"Title": "Synthetic test", "Content": "private-body"})
		_ = listener.Close()
		<-done
		if reject {
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret-value")
			require.NotContains(t, err.Error(), "customer@example.test")
		} else {
			require.NoError(t, err)
		}
	}
	logs := output.text()
	require.Contains(t, logs, "email sent successfully")
	require.Contains(t, logs, "send email failed")
	require.Contains(t, logs, "SMTP 550")
	for _, secret := range []string{"customer@example.test", "secret-value", "private-body"} {
		require.NotContains(t, logs, secret)
	}
}

type privacyRoundTripper func(*http.Request) (*http.Response, error)

func (f privacyRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestConnectorLogsOmitPayloadsWithoutChangingTransport(t *testing.T) {
	db := setupNotificationDeliveryTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.AccessConnector{}, &models.AccessCallLog{}))
	connector := &models.AccessConnector{TenantID: 1, Name: "Synthetic connector", BaseURL: "https://example.test", Status: "active", AuthType: "none"}
	require.NoError(t, db.Create(connector).Error)
	requestBody := map[string]any{"name": "张三", "nested": []any{map[string]any{"email": "customer@example.test", "password": "secret-value"}}}
	responseBody := `{"contact":"13812345678","token":"private-response"}`
	service := &accessConnectorService{httpClient: &http.Client{Transport: privacyRoundTripper(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		expected, _ := json.Marshal(requestBody)
		require.JSONEq(t, string(expected), string(body))
		require.Equal(t, "/customer/customer@example.test", req.URL.Path)
		require.Equal(t, "secret-query", req.URL.Query().Get("access_token"))
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(responseBody))}, nil
	})}}
	result, err := service.CallConnector(context.Background(), 1, connector.ID, ConnectorRequest{Method: "POST", Path: "/customer/customer@example.test", Body: requestBody, QueryParams: map[string]string{"access_token": "secret-query"}})
	require.NoError(t, err)
	require.Equal(t, responseBody, string(result.Body))
	var stored models.AccessCallLog
	require.NoError(t, db.First(&stored).Error)
	require.Equal(t, logprivacy.Redacted, stored.RequestURL)
	require.Equal(t, logprivacy.Redacted, stored.RequestBody)
	require.Equal(t, logprivacy.Redacted, stored.ResponseBody)
	require.Equal(t, 200, stored.ResponseCode)
	require.NotEmpty(t, stored.TraceID)
	require.Empty(t, stored.ErrorMessage)
	service.httpClient.Transport = privacyRoundTripper(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("password=secret-value customer@example.test")
	})
	_, err = service.CallConnector(context.Background(), 1, connector.ID, ConnectorRequest{Path: "/private"})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "secret-value")
	require.NotContains(t, err.Error(), "customer@example.test")
	var failed models.AccessCallLog
	require.NoError(t, db.Where("response_code = 0").First(&failed).Error)
	require.Equal(t, "connector transport failed (details redacted)", failed.ErrorMessage)
	// Legacy log reads are redacted as well; tenant filtering remains effective.
	require.NoError(t, db.Create(&models.AccessCallLog{ID: "legacy-private", TenantID: 1, ConnectorID: connector.ID, RequestURL: "private-url", RequestBody: "private-body", ResponseBody: "private-response", ErrorMessage: "private-error", CreatedAt: time.Now()}).Error)
	logs, err := service.GetCallLogs(context.Background(), 1, connector.ID, 20)
	require.NoError(t, err)
	for _, entry := range logs {
		raw, _ := json.Marshal(entry)
		require.NotContains(t, string(raw), "private-")
		require.NotContains(t, string(raw), "secret-value")
	}
	other, err := service.GetCallLogs(context.Background(), 2, connector.ID, 20)
	require.NoError(t, err)
	require.Empty(t, other)
}

type privacyEmailTransport struct {
	recipients []string
	fail       bool
}

func (transport *privacyEmailTransport) SendNotificationEmail(_ int64, to, _, _ string, _ map[string]interface{}) error {
	transport.recipients = append(transport.recipients, to)
	if transport.fail {
		return errors.New("customer@example.test password=secret-value")
	}
	return nil
}

func TestEmailLogPrivacyPreservesDestinationAcrossRetries(t *testing.T) {
	db := setupNotificationDeliveryTestDB(t)
	email := "customer@example.test"
	seedNotificationDeliveryUser(t, db, 1, 101, &email)
	item := createEmailNotificationForDeliveryTest(t, 1, 101, "privacy-test")
	transport := &privacyEmailTransport{fail: true}
	service := newNotificationDeliveryService(transport)
	clock := time.Now()
	service.now = func() time.Time { return clock }
	require.NoError(t, service.Schedule(item))
	var row models.DeliveryLog
	require.NoError(t, db.First(&row).Error)
	require.Equal(t, logprivacy.Redacted, row.RecipientID)
	require.True(t, strings.HasPrefix(row.RecipientCiphertext, "enc:v1:"))
	plain, err := secretstore.Decrypt(row.RecipientCiphertext)
	require.NoError(t, err)
	require.Equal(t, email, plain)
	exported, err := json.Marshal(row)
	require.NoError(t, err)
	require.NotContains(t, string(exported), email)
	require.NotContains(t, string(exported), row.RecipientCiphertext)
	require.Equal(t, 1, service.ProcessDue(context.Background(), 20))
	require.NoError(t, db.First(&row, row.ID).Error)
	require.Equal(t, notificationDeliveryStatusWaitingRetry, row.Status)
	require.NotContains(t, row.ErrorMsg, "secret-value")
	require.NotContains(t, row.ErrorMsg, email)
	require.NotEmpty(t, row.RecipientCiphertext)
	// A changed contact must not silently redirect an already queued message.
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", 101).Update("email", "changed@example.test").Error)
	clock = *row.NextAttemptAt
	transport.fail = false
	require.Equal(t, 1, service.ProcessDue(context.Background(), 20))
	require.Equal(t, []string{email, email}, transport.recipients)
	var finished models.DeliveryLog
	require.NoError(t, db.First(&finished, row.ID).Error)
	require.Equal(t, notificationDeliveryStatusSent, finished.Status)
	require.Empty(t, finished.RecipientCiphertext)
	EmailNotificationService.logDelivery(1, email, "mail_test", errors.New("password=secret-value "+email))
	var direct models.DeliveryLog
	require.NoError(t, db.Where("notification_id = 0").First(&direct).Error)
	require.Equal(t, logprivacy.Redacted, direct.RecipientID)
	require.NotContains(t, direct.ErrorMsg, email)
	require.Empty(t, direct.RecipientCiphertext)
}

func TestEmailLogPrivacyRejectsCorruptCiphertextAndClearsTerminalDestination(t *testing.T) {
	db := setupNotificationDeliveryTestDB(t)
	email := "customer@example.test"
	seedNotificationDeliveryUser(t, db, 1, 101, &email)
	item := createEmailNotificationForDeliveryTest(t, 1, 101, "corrupt-recipient-test")
	transport := &privacyEmailTransport{}
	service := newNotificationDeliveryService(transport)
	require.NoError(t, service.Schedule(item))
	var row models.DeliveryLog
	require.NoError(t, db.First(&row).Error)
	require.NoError(t, db.Model(&row).Updates(map[string]any{"recipient_ciphertext": "corrupt@example.test", "max_retries": 0}).Error)
	require.Equal(t, 1, service.ProcessDue(context.Background(), 20))
	var failed models.DeliveryLog
	require.NoError(t, db.First(&failed, row.ID).Error)
	require.Equal(t, notificationDeliveryStatusFailed, failed.Status)
	require.Empty(t, failed.RecipientCiphertext)
	require.Empty(t, transport.recipients)
	require.NotContains(t, failed.ErrorMsg, "corrupt@example.test")
}
