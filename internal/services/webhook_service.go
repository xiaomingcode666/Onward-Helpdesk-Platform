package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"

	"github.com/mlogclub/simple/sqls"
)

var WebhookService = newWebhookService()

func newWebhookService() *webhookService {
	return &webhookService{}
}

type webhookService struct{}

// WebhookEndpoint 导出类型（映射到模型）
type WebhookEndpoint = models.WebhookEndpoint

// WebhookEvent Webhook 事件
type WebhookEvent struct {
	EventType string      `json:"event_type"`
	TenantID  string      `json:"tenant_id"`
	Payload   interface{} `json:"payload"`
	Timestamp int64       `json:"timestamp"`
}

// maxRetries 最大重试次数
const webhookMaxRetries = 5

// retryDelays 指数退避延迟（秒）
var webhookRetryDelays = []time.Duration{
	1 * time.Second,
	2 * time.Second,
	4 * time.Second,
	8 * time.Second,
	16 * time.Second,
}

// RegisterWebhook 注册 Webhook 端点
func (s *webhookService) RegisterWebhook(ctx interface{}, tenantID, name, url, eventsJSON, secret string) (*WebhookEndpoint, error) {
	if url == "" || eventsJSON == "" {
		return nil, errorsx.InvalidParamI18n("error.e0017")
	}

	// 解析 tenantID 为 int64
	tenantIDInt, err := strconv.ParseInt(tenantID, 10, 64)
	if err != nil || tenantIDInt <= 0 {
		return nil, errorsx.InvalidParam("invalid tenant id")
	}

	endpoint := &models.WebhookEndpoint{
		TenantID: tenantIDInt,
		Name:     name,
		URL:      url,
		Secret:   secret,
		Events:   eventsJSON,
		Active:   true,
		AuditFields: models.AuditFields{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	if err := sqls.DB().Create(endpoint).Error; err != nil {
		return nil, fmt.Errorf("create webhook endpoint error: %w", err)
	}

	return endpoint, nil
}

func (s *webhookService) ListWebhooks(tenantID string) ([]WebhookEndpoint, error) {
	tenantIDInt, err := strconv.ParseInt(tenantID, 10, 64)
	if err != nil || tenantIDInt <= 0 {
		return nil, errorsx.InvalidParam("invalid tenant id")
	}
	items := make([]WebhookEndpoint, 0)
	err = sqls.DB().Where("tenant_id = ? AND status <> ?", tenantIDInt, enums.StatusDeleted).Order("id DESC").Find(&items).Error
	return items, err
}

func (s *webhookService) SetWebhookActive(tenantID, webhookID string, active bool) error {
	tenantIDInt, err := strconv.ParseInt(tenantID, 10, 64)
	if err != nil || tenantIDInt <= 0 {
		return errorsx.InvalidParam("invalid tenant id")
	}
	webhookIDInt, err := strconv.ParseInt(webhookID, 10, 64)
	if err != nil || webhookIDInt <= 0 {
		return errorsx.InvalidParam("invalid webhook id")
	}
	result := sqls.DB().Model(&models.WebhookEndpoint{}).
		Where("id = ? AND tenant_id = ? AND status <> ?", webhookIDInt, tenantIDInt, enums.StatusDeleted).
		Update("active", active)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errorsx.InvalidParam("webhook endpoint not found")
	}
	return nil
}

// DeliverWebhook 投递 Webhook 事件
func (s *webhookService) DeliverWebhook(ctx interface{}, webhookID string, event WebhookEvent) error {
	// 查询端点配置
	var endpoint models.WebhookEndpoint
	if err := sqls.DB().First(&endpoint, webhookID).Error; err != nil {
		return fmt.Errorf("webhook endpoint not found: %w", err)
	}

	if !endpoint.Active {
		return nil // 端点已停用，跳过
	}

	// 序列化事件
	event.Timestamp = time.Now().UnixMilli()
	payloadBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event error: %w", err)
	}

	// 生成 HMAC-SHA256 签名
	signature := s.generateSignature(payloadBytes, endpoint.Secret)

	// 执行投递（含重试）
	return s.deliverWithRetry(ctx, &endpoint, payloadBytes, signature, event.EventType)
}

// deliverWithRetry 带重试的投递
func (s *webhookService) deliverWithRetry(ctx interface{}, endpoint *models.WebhookEndpoint, payload []byte, signature, eventType string) error {
	var lastErr error

	for attempt := 0; attempt <= webhookMaxRetries; attempt++ {
		startTime := time.Now()
		statusCode, respBody, err := s.doHTTPPost(endpoint.URL, payload, signature)
		durationMs := time.Since(startTime).Milliseconds()

		if err == nil && statusCode >= 200 && statusCode < 300 {
			// 成功
			s.logDelivery(endpoint.ID, eventType, endpoint.URL, string(payload), respBody, statusCode, durationMs, "success", attempt)
			return nil
		}

		// 失败
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		} else {
			errMsg = fmt.Sprintf("HTTP %d", statusCode)
		}
		lastErr = fmt.Errorf("webhook delivery failed (attempt %d/%d): %s", attempt+1, webhookMaxRetries+1, errMsg)

		// 记录失败日志（仅最后一次或特定错误时记录）
		if attempt == 0 || attempt >= webhookMaxRetries {
			s.logDelivery(endpoint.ID, eventType, endpoint.URL, string(payload), respBody, statusCode, durationMs, "failed", attempt)
		}

		// 最后一次尝试，不再重试
		if attempt >= webhookMaxRetries {
			break
		}

		// 指数退避等待
		delay := webhookRetryDelays[attempt]
		slog.Warn("webhook delivery failed, retrying",
			"url", endpoint.URL,
			"attempt", attempt+1,
			"maxRetries", webhookMaxRetries,
			"delay", delay,
			"error", errMsg,
		)
		time.Sleep(delay)
	}

	return lastErr
}

// doHTTPPost 执行 HTTP POST 请求
func (s *webhookService) doHTTPPost(url string, payload []byte, signature string) (int, string, error) {
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return 0, "", fmt.Errorf("create request error: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", time.Now().Unix()))
	req.Header.Set("User-Agent", "RemoteHelpDesk-Webhook/1.0")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("http request error: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody), nil
}

// generateSignature 生成 HMAC-SHA256 签名
func (s *webhookService) generateSignature(payload []byte, secret string) string {
	if secret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// logDelivery 记录投递日志
func (s *webhookService) logDelivery(webhookID int64, eventType, requestURL, requestBody, responseBody string, statusCode int, durationMs int64, status string, retryCount int) {
	log := &models.WebhookDeliveryLog{
		WebhookID:    webhookID,
		EventType:    eventType,
		RequestURL:   requestURL,
		RequestBody:  requestBody,
		ResponseBody: responseBody,
		StatusCode:   statusCode,
		DurationMs:   durationMs,
		Status:       status,
		RetryCount:   retryCount,
		CreatedAt:    time.Now(),
	}
	if err := sqls.DB().Create(log).Error; err != nil {
		slog.Error("failed to save webhook delivery log", "error", err)
	}
}
