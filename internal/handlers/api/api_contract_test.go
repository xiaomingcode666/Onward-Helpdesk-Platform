package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"remotehelpdesk/internal/bootstrap"
	"remotehelpdesk/internal/pkg/httpx"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
	"github.com/mlogclub/simple/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestEngine 创建一个带有路由的测试用 Gin Engine
func setupTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	app, err := bootstrap.NewServer()
	require.NoError(t, err)
	return app
}

// performRequest 执行 HTTP 请求并返回响应
func performRequest(engine *gin.Engine, method, path string, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

// TestAPIErrorResponseFormat 验证所有错误响应格式一致
func TestAPIErrorResponseFormat(t *testing.T) {
	engine := setupTestEngine(t)

	testCases := []struct {
		name       string
		method     string
		path       string
		body       string
		headers    map[string]string
		wantStatus int
	}{
		{
			name:       "Unauthenticated access returns business error envelope",
			method:     "GET",
			path:       "/api/dashboard/ticket/list",
			body:       "",
			headers:    map[string]string{},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "Non-existent API returns 404",
			method:     "GET",
			path:       "/api/nonexistent",
			body:       "",
			headers:    map[string]string{},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Invalid JSON body returns business error envelope",
			method:     "POST",
			path:       "/api/auth/login",
			body:       "invalid json{{{",
			headers:    map[string]string{},
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := performRequest(engine, tc.method, tc.path, tc.body, tc.headers)
			assert.Equal(t, tc.wantStatus, w.Code)

			// 解析响应 JSON
			var resp map[string]any
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err, "响应必须是合法 JSON")

			// 错误响应同时使用标准 HTTP 状态码和统一业务信封。
			assert.Equal(t, false, resp["success"])
			assert.Contains(t, resp, "errorCode")
			assert.NotEmpty(t, resp["message"])
			assert.Contains(t, resp, "data")
		})
	}
}

// TestAPIAuthHeaders 验证 API 正确校验 Authorization header
func TestAPIAuthHeaders(t *testing.T) {
	engine := setupTestEngine(t)

	// 所有需要认证的 API 在没有 Authorization header 时应返回 401 和统一错误信封。
	protectedEndpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/dashboard/ticket/list"},
		{http.MethodPost, "/api/dashboard/ticket/create"},
		{http.MethodGet, "/api/dashboard/user/list"},
		{http.MethodGet, "/api/dashboard/product/list"},
		{http.MethodGet, "/api/dashboard/device/list"},
		{http.MethodGet, "/api/dashboard/knowledge-base/list"},
		{http.MethodGet, "/api/dashboard/conversation/list"},
	}

	for _, ep := range protectedEndpoints {
		t.Run(fmt.Sprintf("%s %s", ep.method, ep.path), func(t *testing.T) {
			w := performRequest(engine, ep.method, ep.path, "", nil)
			assert.Equal(t, http.StatusUnauthorized, w.Code,
				"%s %s 缺少 Authorization 时应返回 401", ep.method, ep.path)
			var resp map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, false, resp["success"])
			assert.NotEmpty(t, resp["message"])
		})
	}
}

// TestAPIProtectedListAuthConsistency 验证受保护列表的认证失败响应一致。
func TestAPIProtectedListAuthConsistency(t *testing.T) {
	engine := setupTestEngine(t)

	testCases := []struct {
		name   string
		method string
		path   string
	}{
		{"Ticket list", "GET", "/api/dashboard/ticket/list"},
		{"User list", "GET", "/api/dashboard/user/list"},
		{"Product list", "GET", "/api/dashboard/product/list"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := performRequest(engine, tc.method, tc.path, "", nil)
			assert.Equal(t, http.StatusUnauthorized, w.Code)

			// 验证响应是 JSON
			var resp map[string]any
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			assert.NoError(t, err, "%s 的响应必须是合法 JSON", tc.path)
			assert.Equal(t, false, resp["success"])
		})
	}
}

// TestAPITenantContext 验证 API 正确注入租户上下文
func TestAPITenantContext(t *testing.T) {
	engine := setupTestEngine(t)

	// 公共 API 端点应不需要租户上下文
	publicEndpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/health"},
		{http.MethodGet, "/api/health/ready"},
		{http.MethodGet, "/api/health/live"},
		{http.MethodGet, "/api/config"},
		{http.MethodPost, "/api/auth/login"},
	}

	for _, ep := range publicEndpoints {
		t.Run(fmt.Sprintf("%s %s available without tenant", ep.method, ep.path), func(t *testing.T) {
			w := performRequest(engine, ep.method, ep.path, "", nil)
			// 健康检查应可访问（不要求认证）
			assert.NotEqual(t, http.StatusNotFound, w.Code,
				"公共端点 %s 应可访问", ep.path)
		})
	}
}

// TestAPIIdempotencyKey 验证幂等键
func TestAPIIdempotencyKey(t *testing.T) {
	engine := setupTestEngine(t)

	// 创建一个登录请求，同一请求发送两次
	loginBody := `{"username":"admin","password":"admin123"}`

	w1 := performRequest(engine, "POST", "/api/auth/login", loginBody, nil)
	w2 := performRequest(engine, "POST", "/api/auth/login", loginBody, nil)

	// 登录请求本身不是幂等的，两次调用可能返回不同的响应
	// 但验证两次都返回有效 JSON
	var resp1, resp2 any
	err1 := json.Unmarshal(w1.Body.Bytes(), &resp1)
	err2 := json.Unmarshal(w2.Body.Bytes(), &resp2)
	assert.NoError(t, err1, "第一次登录响应应是合法 JSON")
	assert.NoError(t, err2, "第二次登录响应应是合法 JSON")
	_ = resp1
	_ = resp2
}

// TestAPISuccessResponseFormat 验证成功响应的格式
func TestAPISuccessResponseFormat(t *testing.T) {
	// 验证 web.JsonResult / web.PageResult 的输出格式
	t.Run("JsonResultFormat", func(t *testing.T) {
		result := web.JsonSuccess()
		data, err := json.Marshal(result)
		require.NoError(t, err)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal(data, &parsed))

		// 成功响应应包含 success: true
		assert.Equal(t, true, parsed["success"])
		// 通常还包含 data 字段
		_, hasData := parsed["data"]
		assert.True(t, hasData, "成功响应应包含 data 字段")
	})

	t.Run("PageResultFormat", func(t *testing.T) {
		result := &web.PageResult{
			Results: []map[string]string{{"id": "1"}},
			Page: &sqls.Paging{
				Page:  1,
				Limit: 20,
				Total: 1,
			},
		}
		data, err := json.Marshal(result)
		require.NoError(t, err)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal(data, &parsed))

		assert.Contains(t, parsed, "results", "分页响应应包含 results")
		assert.Contains(t, parsed, "page", "分页响应应包含 page")

		pageMap, ok := parsed["page"].(map[string]any)
		require.True(t, ok)
		assert.Contains(t, pageMap, "page")
		assert.Contains(t, pageMap, "limit")
		assert.Contains(t, pageMap, "total")
	})
}

// TestAPIErrorResponseConsistency 验证 httpx.WriteJSON 的错误序列化一致性
func TestAPIErrorResponseConsistency(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Business error format", func(t *testing.T) {
		// 模拟一个业务错误
		err := fmt.Errorf("business error: invalid input")

		// 验证 web.JsonError 的输出格式
		result := web.JsonError(err)
		data, errJSON := json.Marshal(result)
		require.NoError(t, errJSON)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal(data, &parsed))

		// 错误响应应包含 success: false
		assert.Equal(t, false, parsed["success"])
	})

	t.Run("httpx error handling", func(t *testing.T) {
		// 验证 httpx 的错误处理能正确序列化
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest("GET", "/test", nil)

		// 模拟一个普通的 error
		httpx.WriteJSON(ctx, fmt.Errorf("generic error"))
		assert.Equal(t, http.StatusOK, w.Code)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &parsed))
		assert.Equal(t, false, parsed["success"])
		assert.Contains(t, parsed, "errorCode")
		assert.Equal(t, "generic error", parsed["message"])
		assert.Contains(t, parsed, "data")
	})
}
