package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/secretstore"
	"remotehelpdesk/internal/pkg/utils"

	"github.com/mlogclub/simple/sqls"
)

var AccessConnectorService = newAccessConnectorService()

func newAccessConnectorService() *accessConnectorService {
	return &accessConnectorService{}
}

type accessConnectorService struct{}

const connectorResponseLimit = 4 << 20

var ErrConnectorWorkerRequired = errors.New("streaming connector must be executed by a background connector worker")

type feishuAppCredential struct {
	AppID     string `json:"appId"`
	AppSecret string `json:"appSecret"`
}

type feishuTokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
}

// ConnectorRequest 调用外部系统的请求参数
type ConnectorRequest struct {
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	QueryParams map[string]string `json:"queryParams,omitempty"`
	Body        interface{}       `json:"body,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// ConnectorResponse 外部系统响应
type ConnectorResponse struct {
	StatusCode int               `json:"statusCode"`
	Body       []byte            `json:"body"`
	Headers    map[string]string `json:"headers,omitempty"`
	DurationMs int64             `json:"durationMs"`
}

// ConnectorHealth 连接器健康状态
type ConnectorHealth struct {
	ConnectorID   int64      `json:"connectorId"`
	Status        string     `json:"status"` // healthy / degraded / unhealthy / inactive / testing / unknown
	LastChecked   time.Time  `json:"lastChecked"`
	LastMessageAt *time.Time `json:"lastMessageAt,omitempty"`
	LatencyMs     int64      `json:"latencyMs"`
	ErrorMessage  string     `json:"errorMessage,omitempty"`
}

// CreateConnector 创建连接器
func (s *accessConnectorService) CreateConnector(ctx context.Context, connector *models.AccessConnector) error {
	if connector == nil || connector.TenantID <= 0 {
		return fmt.Errorf("tenant is required")
	}
	if err := validateConnectorConfiguration(connector); err != nil {
		return err
	}
	encryptedAuth, err := secretstore.Encrypt(connector.AuthConfig)
	if err != nil {
		return fmt.Errorf("encrypt connector credential: %w", err)
	}
	connector.AuthConfig = encryptedAuth
	if _, err := validateConnectorEndpoint(connector); err != nil {
		return err
	}
	now := time.Now()
	connector.CreatedAt = now
	connector.UpdatedAt = now
	if connector.Status == "" {
		connector.Status = "inactive"
	}
	if connector.HealthStatus == "" {
		connector.HealthStatus = "unknown"
	}

	if err := sqls.DB().WithContext(ctx).Create(connector).Error; err != nil {
		slog.Error("create connector failed", "error", err)
		return fmt.Errorf("failed to create connector: %w", err)
	}
	slog.Info("connector created", "id", connector.ID, "name", connector.Name, "tenantID", connector.TenantID)
	return nil
}

// UpdateConnector replaces a tenant connector's configuration and resets its health state.
func (s *accessConnectorService) UpdateConnector(ctx context.Context, connector *models.AccessConnector) error {
	if connector == nil || connector.ID <= 0 || connector.TenantID <= 0 {
		return fmt.Errorf("connector and tenant are required")
	}
	if _, err := s.getConnectorByID(connector.TenantID, connector.ID); err != nil {
		return err
	}
	if err := validateConnectorConfiguration(connector); err != nil {
		return err
	}
	encryptedAuth, err := secretstore.Encrypt(connector.AuthConfig)
	if err != nil {
		return fmt.Errorf("encrypt connector credential: %w", err)
	}
	connector.AuthConfig = encryptedAuth
	connector.Status = "inactive"
	connector.HealthStatus = "unknown"
	connector.UpdatedAt = time.Now()
	updates := map[string]any{
		"name": connector.Name, "connector_type": connector.ConnectorType, "base_url": connector.BaseURL,
		"auth_type": connector.AuthType, "auth_config": connector.AuthConfig, "field_mapping": connector.FieldMapping,
		"template_code": connector.TemplateCode, "status": connector.Status, "health_status": connector.HealthStatus,
		"last_tested_at": nil, "last_message_at": nil, "updated_at": connector.UpdatedAt,
	}
	if err := sqls.DB().WithContext(ctx).Model(&models.AccessConnector{}).
		Where("id = ? AND tenant_id = ?", connector.ID, connector.TenantID).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update connector: %w", err)
	}
	return nil
}

// TestConnector 测试连接器连通性
func (s *accessConnectorService) TestConnector(ctx context.Context, tenantID, connectorID int64) error {
	connector, err := s.getConnectorByID(tenantID, connectorID)
	if err != nil {
		return err
	}
	if connector.ConnectorType == models.ConnectorTypeMQTT {
		if _, err := validateMQTTBrokerURL(connector.BaseURL); err != nil {
			return err
		}
		return ErrConnectorWorkerRequired
	}

	startTime := time.Now()

	testBody, err := s.buildTestRequest(connector)
	if err != nil {
		return s.recordTestFailure(connector, startTime, err.Error())
	}

	req, err := http.NewRequestWithContext(ctx, testBody.Method, testBody.URL, bytes.NewReader(testBody.Data))
	if err != nil {
		return s.recordTestFailure(connector, startTime, err.Error())
	}
	for k, v := range testBody.Headers {
		req.Header.Set(k, v)
	}

	client := safeConnectorHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return s.recordTestFailure(connector, startTime, fmt.Sprintf("connection failed: %v", err))
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, connectorResponseLimit+1))
	if len(respBody) > connectorResponseLimit {
		return s.recordTestFailure(connector, startTime, "connector response exceeds 4 MiB")
	}
	durationMs := time.Since(startTime).Milliseconds()

	if err := validateConnectorTestResponse(connector, resp.StatusCode, respBody); err == nil {
		now := time.Now()
		sqls.DB().Model(&models.AccessConnector{}).Where("id = ?", connectorID).
			Updates(map[string]any{
				"status": "active", "health_status": "healthy", "last_tested_at": &now, "updated_at": now,
			})
		slog.Info("connector test passed", "id", connectorID, "durationMs", durationMs)
		return nil
	} else {
		return s.recordTestFailure(connector, startTime, err.Error())
	}
}

type testRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Data    []byte
}

// buildTestRequest 根据连接器类型构建测试请求
func (s *accessConnectorService) buildTestRequest(connector *models.AccessConnector) (*testRequest, error) {
	if connector.ConnectorType == models.ConnectorTypeMQTT {
		return nil, ErrConnectorWorkerRequired
	}
	baseURL := strings.TrimRight(connector.BaseURL, "/")
	if _, err := validateConnectorBaseURL(baseURL); err != nil {
		return nil, err
	}
	headers, err := s.buildAuthHeaders(connector)
	if err != nil {
		return nil, err
	}

	switch connector.ConnectorType {
	case models.ConnectorTypeFeishu:
		credential, err := parseFeishuCredential(connector.AuthConfig)
		if err != nil {
			return nil, err
		}
		payload, err := json.Marshal(map[string]string{"app_id": credential.AppID, "app_secret": credential.AppSecret})
		if err != nil {
			return nil, fmt.Errorf("encode feishu credential request: %w", err)
		}
		return &testRequest{
			Method: "POST", URL: baseURL + "/auth/v3/tenant_access_token/internal",
			Headers: map[string]string{"Content-Type": "application/json; charset=utf-8"}, Data: payload,
		}, nil
	case "openapi":
		headers["Content-Type"] = "application/json"
		return &testRequest{
			Method:  "GET",
			URL:     baseURL + "/health",
			Headers: headers,
		}, nil
	case "webhook":
		payload, _ := json.Marshal(map[string]string{"test": "ping"})
		headers["Content-Type"] = "application/json"
		return &testRequest{
			Method:  "POST",
			URL:     baseURL + "/webhook-test",
			Headers: headers,
			Data:    payload,
		}, nil
	case "database", "mysql", "postgres":
		return &testRequest{
			Method:  "GET",
			URL:     baseURL,
			Headers: headers,
		}, nil
	default:
		return &testRequest{
			Method:  "GET",
			URL:     baseURL,
			Headers: headers,
		}, nil
	}
}

// buildAuthHeaders 根据认证类型构建请求头
func (s *accessConnectorService) buildAuthHeaders(connector *models.AccessConnector) (map[string]string, error) {
	headers := make(map[string]string)
	authConfig, err := secretstore.Decrypt(connector.AuthConfig)
	if err != nil {
		return nil, fmt.Errorf("decrypt connector credential: %w", err)
	}
	switch connector.AuthType {
	case "api_key":
		headers["X-API-Key"] = authConfig
	case "basic":
		headers["Authorization"] = "Basic " + authConfig
	case "bearer":
		headers["Authorization"] = "Bearer " + authConfig
	case "mtls":
		headers["X-Auth-Type"] = "mtls"
	}
	return headers, nil
}

func (s *accessConnectorService) recordTestFailure(connector *models.AccessConnector, startTime time.Time, errMsg string) error {
	durationMs := time.Since(startTime).Milliseconds()
	now := time.Now()
	sqls.DB().Model(&models.AccessConnector{}).Where("id = ? AND tenant_id = ?", connector.ID, connector.TenantID).
		Updates(map[string]any{"health_status": "unhealthy", "last_tested_at": &now, "updated_at": now})
	slog.Warn("connector test failed", "id", connector.ID, "error", errMsg, "durationMs", durationMs)
	return fmt.Errorf("connector test failed: %s", errMsg)
}

func validateConnectorConfiguration(connector *models.AccessConnector) error {
	if strings.TrimSpace(connector.Name) == "" {
		return fmt.Errorf("connector name is required")
	}
	parsed, err := validateConnectorEndpoint(connector)
	if err != nil {
		return err
	}
	if connector.ConnectorType == models.ConnectorTypeMQTT {
		switch connector.AuthType {
		case "", "none", "basic":
		default:
			return fmt.Errorf("MQTT connector authentication must be none or basic")
		}
		validationConnector := *connector
		if validationConnector.ID <= 0 {
			validationConnector.ID = 1
		}
		_, credential, err := parseMQTTWorkerConfiguration(&validationConnector)
		if err != nil {
			return err
		}
		if connector.AuthType == "basic" && strings.TrimSpace(credential.Username) == "" {
			return fmt.Errorf("MQTT basic authentication requires a username")
		}
		return nil
	}
	if connector.ConnectorType != models.ConnectorTypeFeishu {
		return nil
	}
	if connector.AuthType != "app_credentials" {
		return fmt.Errorf("feishu connector requires app credentials")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "open.feishu.cn" && host != "open.larksuite.com" {
		return fmt.Errorf("feishu connector must use an official API endpoint")
	}
	if strings.TrimRight(parsed.Path, "/") != "/open-apis" {
		return fmt.Errorf("invalid feishu API base path")
	}
	_, err = parseFeishuCredential(connector.AuthConfig)
	return err
}

func parseFeishuCredential(value string) (*feishuAppCredential, error) {
	plain, err := secretstore.Decrypt(value)
	if err != nil {
		return nil, fmt.Errorf("decrypt feishu credential: %w", err)
	}
	credential := &feishuAppCredential{}
	if err := json.Unmarshal([]byte(plain), credential); err != nil {
		return nil, fmt.Errorf("invalid feishu app credential")
	}
	credential.AppID = strings.TrimSpace(credential.AppID)
	credential.AppSecret = strings.TrimSpace(credential.AppSecret)
	if credential.AppID == "" || credential.AppSecret == "" {
		return nil, fmt.Errorf("feishu app id and app secret are required")
	}
	return credential, nil
}

func validateConnectorTestResponse(connector *models.AccessConnector, statusCode int, body []byte) error {
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected status: %d", statusCode)
	}
	if connector.ConnectorType != models.ConnectorTypeFeishu {
		return nil
	}
	result := &feishuTokenResponse{}
	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("invalid feishu authentication response")
	}
	if result.Code != 0 {
		message := strings.TrimSpace(result.Msg)
		if message == "" {
			message = "credential verification failed"
		}
		return fmt.Errorf("feishu authentication failed (%d): %s", result.Code, message)
	}
	if strings.TrimSpace(result.TenantAccessToken) == "" {
		return fmt.Errorf("feishu authentication response did not include a tenant access token")
	}
	return nil
}

// CallConnector 调用外部系统连接器
func (s *accessConnectorService) CallConnector(ctx context.Context, tenantID, connectorID int64, request ConnectorRequest) (*ConnectorResponse, error) {
	connector, err := s.getConnectorByID(tenantID, connectorID)
	if err != nil {
		return nil, err
	}

	if connector.Status != "active" && connector.Status != "testing" {
		return nil, fmt.Errorf("connector is not active, current status: %s", connector.Status)
	}
	if connector.ConnectorType == models.ConnectorTypeMQTT {
		return nil, ErrConnectorWorkerRequired
	}

	startTime := time.Now()
	traceID := utils.UUID()

	baseURL, err := validateConnectorBaseURL(connector.BaseURL)
	if err != nil {
		return nil, err
	}
	reqPath := strings.TrimSpace(request.Path)
	if strings.Contains(reqPath, "://") || strings.HasPrefix(reqPath, "//") {
		return nil, fmt.Errorf("connector path must be relative")
	}
	baseURL.Path = strings.TrimRight(baseURL.Path, "/") + "/" + strings.TrimLeft(reqPath, "/")

	if len(request.QueryParams) > 0 {
		params := url.Values{}
		for k, v := range request.QueryParams {
			params.Set(k, v)
		}
		baseURL.RawQuery = params.Encode()
	}
	reqURL := baseURL.String()

	var reqBody []byte
	if request.Body != nil {
		reqBody, err = json.Marshal(request.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	method := strings.ToUpper(request.Method)
	if method == "" {
		method = "GET"
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, reqURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	authHeaders, err := s.buildAuthHeaders(connector)
	if err != nil {
		return nil, err
	}
	for k, v := range authHeaders {
		httpReq.Header.Set(k, v)
	}
	for k, v := range request.Headers {
		if isReservedConnectorHeader(k) {
			return nil, fmt.Errorf("connector header %q cannot be overridden", k)
		}
		httpReq.Header.Set(k, v)
	}
	httpReq.Header.Set("X-Trace-ID", traceID)
	if reqBody != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	client := safeConnectorHTTPClient(30 * time.Second)
	resp, err := client.Do(httpReq)
	durationMs := time.Since(startTime).Milliseconds()

	callLog := &models.AccessCallLog{
		ID:            utils.UUID(),
		TenantID:      tenantID,
		ConnectorID:   connectorID,
		RequestURL:    reqURL,
		RequestMethod: method,
		RequestBody:   utils.SanitizeForLog(string(reqBody), 4096),
		DurationMs:    durationMs,
		TraceID:       traceID,
		CreatedAt:     time.Now(),
	}

	if err != nil {
		callLog.ErrorMessage = err.Error()
		callLog.ResponseCode = 0
		sqls.DB().Create(callLog)
		return nil, fmt.Errorf("connector call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, connectorResponseLimit+1))
	if readErr != nil {
		callLog.ErrorMessage = readErr.Error()
		sqls.DB().Create(callLog)
		return nil, fmt.Errorf("read connector response: %w", readErr)
	}
	if len(respBody) > connectorResponseLimit {
		callLog.ErrorMessage = "connector response exceeds 4 MiB"
		sqls.DB().Create(callLog)
		return nil, fmt.Errorf("connector response exceeds 4 MiB")
	}
	callLog.ResponseCode = resp.StatusCode
	callLog.ResponseBody = utils.SanitizeForLog(string(respBody), 4096)

	if resp.StatusCode >= 400 {
		callLog.ErrorMessage = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	sqls.DB().Create(callLog)

	respHeaders := make(map[string]string)
	for k, vals := range resp.Header {
		if len(vals) > 0 {
			respHeaders[k] = vals[0]
		}
	}

	return &ConnectorResponse{
		StatusCode: resp.StatusCode,
		Body:       respBody,
		Headers:    respHeaders,
		DurationMs: durationMs,
	}, nil
}

// ListConnectors 获取连接器列表
func (s *accessConnectorService) ListConnectors(ctx context.Context, tenantID int64, connectorType string) ([]*models.AccessConnector, error) {
	query := sqls.DB().WithContext(ctx).Where("tenant_id = ?", tenantID)
	if connectorType != "" {
		query = query.Where("connector_type = ?", connectorType)
	}

	var connectors []*models.AccessConnector
	if err := query.Order("created_at DESC").Find(&connectors).Error; err != nil {
		return nil, fmt.Errorf("failed to list connectors: %w", err)
	}
	for _, connector := range connectors {
		connector.AuthConfig = ""
	}
	return connectors, nil
}

// GetConnectorStatus 获取连接器健康状态
func (s *accessConnectorService) GetConnectorStatus(ctx context.Context, tenantID, connectorID int64) (*ConnectorHealth, error) {
	connector, err := s.getConnectorByID(tenantID, connectorID)
	if err != nil {
		return nil, err
	}

	return &ConnectorHealth{
		ConnectorID:   connector.ID,
		Status:        connector.HealthStatus,
		LastChecked:   time.Now(),
		LastMessageAt: connector.LastMessageAt,
		LatencyMs:     0,
	}, nil
}

// GetCallLogs 获取连接器调用日志
func (s *accessConnectorService) GetCallLogs(ctx context.Context, tenantID, connectorID int64, limit int) ([]*models.AccessCallLog, error) {
	if limit <= 0 {
		limit = 20
	}

	var logs []*models.AccessCallLog
	if err := sqls.DB().WithContext(ctx).
		Where("tenant_id = ? AND connector_id = ?", tenantID, connectorID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to get call logs: %w", err)
	}
	return logs, nil
}

// ConnectorHealthCheckJob 定时扫描所有 active 连接器
func (s *accessConnectorService) ConnectorHealthCheckJob() {
	slog.Info("starting connector health check job")

	var connectors []*models.AccessConnector
	if err := sqls.DB().Where("status = ?", "active").Find(&connectors).Error; err != nil {
		slog.Error("health check: failed to query active connectors", "error", err)
		return
	}

	for _, connector := range connectors {
		s.checkSingleConnectorHealth(connector)
	}

	slog.Info("connector health check job completed", "checkedCount", len(connectors))
}

func (s *accessConnectorService) checkSingleConnectorHealth(connector *models.AccessConnector) {
	if connector.ConnectorType == models.ConnectorTypeMQTT {
		s.checkMQTTConnectorHealth(connector, time.Now())
		return
	}
	startTime := time.Now()

	testBody, err := s.buildTestRequest(connector)
	if err != nil {
		slog.Warn("health check: build test request failed", "id", connector.ID, "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, testBody.Method, testBody.URL, bytes.NewReader(testBody.Data))
	if err != nil {
		slog.Warn("health check: create request failed", "id", connector.ID, "error", err)
		return
	}
	for k, v := range testBody.Headers {
		req.Header.Set(k, v)
	}

	resp, err := safeConnectorHTTPClient(10 * time.Second).Do(req)
	durationMs := time.Since(startTime).Milliseconds()

	if err != nil {
		slog.Warn("health check: request failed", "id", connector.ID, "error", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		if durationMs > 5000 {
			slog.Warn("health check: degraded", "id", connector.ID, "latencyMs", durationMs)
		} else {
			slog.Info("health check: healthy", "id", connector.ID, "latencyMs", durationMs)
		}
	} else {
		slog.Warn("health check: unhealthy", "id", connector.ID, "statusCode", resp.StatusCode)
	}
}

func (s *accessConnectorService) checkMQTTConnectorHealth(connector *models.AccessConnector, now time.Time) {
	status := mqttConnectorHealthStatus(connector.LastMessageAt, now)
	if status == connector.HealthStatus {
		return
	}
	if err := sqls.DB().Model(&models.AccessConnector{}).
		Where("id = ? AND tenant_id = ?", connector.ID, connector.TenantID).
		Updates(map[string]any{"health_status": status, "updated_at": now}).Error; err != nil {
		slog.Warn("health check: update mqtt health failed", "id", connector.ID, "error", err)
		return
	}
	slog.Info("health check: mqtt activity evaluated", "id", connector.ID, "status", status, "lastMessageAt", connector.LastMessageAt)
}

func mqttConnectorHealthStatus(lastMessageAt *time.Time, now time.Time) string {
	if lastMessageAt == nil || lastMessageAt.IsZero() {
		return "unknown"
	}
	age := now.Sub(*lastMessageAt)
	if age <= 5*time.Minute {
		return "healthy"
	}
	if age <= 15*time.Minute {
		return "degraded"
	}
	return "unhealthy"
}

// getConnectorByID 根据 tenantID 和 connectorID 获取连接器
func (s *accessConnectorService) getConnectorByID(tenantID, connectorID int64) (*models.AccessConnector, error) {
	var connector models.AccessConnector
	if err := sqls.DB().Where("id = ? AND tenant_id = ?", connectorID, tenantID).First(&connector).Error; err != nil {
		return nil, fmt.Errorf("connector not found: %d", connectorID)
	}
	return &connector, nil
}

func validateConnectorBaseURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil || parsed.Hostname() == "" {
		return nil, fmt.Errorf("invalid connector base URL")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, fmt.Errorf("connector base URL must use http or https")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("connector base URL must not contain credentials")
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return nil, fmt.Errorf("connector base URL points to a blocked host")
	}
	if ip := net.ParseIP(host); ip != nil && isBlockedConnectorIP(ip) {
		return nil, fmt.Errorf("connector base URL points to a blocked network")
	}
	parsed.Fragment = ""
	return parsed, nil
}

func validateConnectorEndpoint(connector *models.AccessConnector) (*url.URL, error) {
	if connector == nil {
		return nil, fmt.Errorf("connector is required")
	}
	if connector.ConnectorType == models.ConnectorTypeMQTT {
		return validateMQTTBrokerURL(connector.BaseURL)
	}
	return validateConnectorBaseURL(connector.BaseURL)
}

func validateMQTTBrokerURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil {
		return nil, fmt.Errorf("invalid MQTT broker URL")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("MQTT broker URL must not contain credentials")
	}
	if parsed.Hostname() == "" {
		return nil, fmt.Errorf("MQTT broker URL must include a host")
	}
	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "mqtt", "mqtts", "ws", "wss":
		parsed.Scheme = scheme
	default:
		return nil, fmt.Errorf("MQTT broker URL must use mqtt, mqtts, ws, or wss")
	}
	parsed.Fragment = ""
	return parsed, nil
}

func safeConnectorHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: min(timeout, 10*time.Second), KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, fmt.Errorf("invalid connector address: %w", err)
			}
			addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("resolve connector host: %w", err)
			}
			if len(addresses) == 0 {
				return nil, fmt.Errorf("connector host has no address")
			}
			for _, address := range addresses {
				if isBlockedConnectorIP(address.IP) {
					return nil, fmt.Errorf("connector host resolves to a blocked network")
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
		},
		ForceAttemptHTTP2: true,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many connector redirects")
			}
			_, err := validateConnectorBaseURL(req.URL.String())
			return err
		},
	}
}

func isBlockedConnectorIP(ip net.IP) bool {
	return ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast() || ip.IsPrivate()
}

func isReservedConnectorHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "authorization", "proxy-authorization", "x-api-key", "host", "x-trace-id", "content-length":
		return true
	default:
		return false
	}
}

// ── 连接器模板 ──────────────────────────────────────────────
// ConnectorTemplate / GetConnectorTemplates / GetConnectorTemplateByCode
// 定义在 connector_templates.go，此处不重复。

// ── 安全查询 ────────────────────────────────────────────────

// ExecuteQuery 执行受控查询（带安全校验和脱敏）
func (s *accessConnectorService) ExecuteQuery(ctx context.Context, tenantID, connectorID int64, query string) (*ConnectorResponse, error) {
	connector, err := s.getConnectorByID(tenantID, connectorID)
	if err != nil {
		return nil, err
	}

	_ = connector // reserved for future field-level masking

	upperQuery := strings.ToUpper(query)
	dangerousOps := []string{"DROP ", "ALTER ", "TRUNCATE ", "CREATE ", "INSERT ", "UPDATE ", "DELETE ", "GRANT ", "REVOKE "}
	for _, op := range dangerousOps {
		if strings.Contains(upperQuery, op) {
			slog.Warn("blocked dangerous query", "connectorID", connectorID, "op", strings.TrimSpace(op))
			return nil, fmt.Errorf("query blocked: %s operations are not allowed through access connector", strings.TrimSpace(op))
		}
	}

	req := ConnectorRequest{
		Method: "POST",
		Path:   "/query",
		Body: map[string]string{
			"query": query,
		},
	}

	go func() {
		now := time.Now()
		logEntry := &models.AccessQueryLog{
			ConnectorID: connectorID,
			TenantID:    tenantID,
			Query:       query,
			Status:      models.QueryStatusSuccess,
			RequestID:   utils.UUID(),
			ActorID:     strconv.FormatInt(tenantID, 10),
			ActorType:   "system",
			CreatedAt:   now,
		}
		sqls.DB().Create(logEntry)
	}()

	return s.CallConnector(ctx, tenantID, connectorID, req)
}
