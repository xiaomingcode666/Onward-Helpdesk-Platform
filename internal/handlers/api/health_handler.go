package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"remotehelpdesk/internal/ai/rag/vectordb"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/cache"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/observability"
	"remotehelpdesk/internal/services/storage"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

type healthResponse struct {
	Status    string               `json:"status"`
	Timestamp string               `json:"timestamp,omitempty"`
	Checks    map[string]checkInfo `json:"checks,omitempty"`
}

type checkInfo struct {
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

// GET /api/health — 基础健康检查（返回 200 OK）
func Health(ctx *gin.Context) {
	httpx.WriteJSON(ctx, &healthResponse{Status: "healthy"})
}

// GET /api/health/ready — 就绪检查（检查 DB、Redis、Qdrant 连接）
func Ready(ctx *gin.Context) {
	checks := make(map[string]checkInfo)

	checks["postgres"] = checkPostgres()
	checks["redis"] = checkRedis()
	checks["qdrant"] = checkQdrant()
	checks["storage"] = checkStorage()
	checks["outbox"] = checkOutbox()
	for name, check := range checks {
		observability.RecordDependencyCheck(name, check.Status == "ok", time.Duration(check.LatencyMs)*time.Millisecond)
	}

	writeReadinessResponse(ctx, checks)
}

func checkStorage() checkInfo {
	start := time.Now()
	provider, err := storage.GetDefault()
	if err != nil {
		return checkInfo{Status: "error", Detail: err.Error()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := provider.HealthCheck(ctx); err != nil {
		return checkInfo{Status: "error", LatencyMs: time.Since(start).Milliseconds(), Detail: err.Error()}
	}
	return checkInfo{Status: "ok", LatencyMs: time.Since(start).Milliseconds()}
}

func checkOutbox() checkInfo {
	return checkOutboxDB(sqls.DB(), time.Now())
}

func checkOutboxDB(db *gorm.DB, now time.Time) checkInfo {
	start := time.Now()
	if db == nil {
		return checkInfo{Status: "error", Detail: "database not initialized"}
	}
	if !db.Migrator().HasTable(&models.OutboxRecord{}) {
		return checkInfo{Status: "error", Detail: "outbox table is missing"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	db = db.WithContext(ctx)

	var deadCount int64
	if err := db.Model(&models.OutboxRecord{}).
		Where("status = ? OR (status = ? AND retry_count >= max_retries)", models.OutboxStatusDead, models.OutboxStatusFailed).
		Count(&deadCount).Error; err != nil {
		return checkInfo{Status: "error", Detail: err.Error()}
	}
	if deadCount > 0 {
		return checkInfo{Status: "error", LatencyMs: time.Since(start).Milliseconds(), Detail: fmt.Sprintf("dead_letter_count=%d", deadCount)}
	}

	staleBefore := now.Add(-5 * time.Minute)
	var staleCount int64
	if err := db.Model(&models.OutboxRecord{}).
		Where("created_at <= ? AND (((status IN ?) AND retry_count < max_retries AND (next_retry_at IS NULL OR next_retry_at <= ?)) OR (status = ? AND (locked_until IS NULL OR locked_until <= ?)))",
			staleBefore,
			[]string{models.OutboxStatusPending, models.OutboxStatusFailed}, now,
			models.OutboxStatusPublishing, now).
		Count(&staleCount).Error; err != nil {
		return checkInfo{Status: "error", Detail: err.Error()}
	}
	if staleCount > 0 {
		return checkInfo{Status: "error", LatencyMs: time.Since(start).Milliseconds(), Detail: fmt.Sprintf("stale_delivery_count=%d", staleCount)}
	}
	return checkInfo{Status: "ok", LatencyMs: time.Since(start).Milliseconds()}
}

func writeReadinessResponse(ctx *gin.Context, checks map[string]checkInfo) {
	overallStatus := "healthy"
	statusCode := http.StatusOK
	for _, c := range checks {
		if c.Status != "ok" {
			overallStatus = "unhealthy"
			statusCode = http.StatusServiceUnavailable
			break
		}
	}

	httpx.WriteHttpStatusJSON(ctx, statusCode, &healthResponse{
		Status:    overallStatus,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
	})
}

// GET /api/health/live — 存活检查
func Live(ctx *gin.Context) {
	httpx.WriteJSON(ctx, &healthResponse{
		Status:    "alive",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// checkPostgres 检查 PostgreSQL 数据库连接
func checkPostgres() checkInfo {
	start := time.Now()

	db := sqls.DB()
	if db == nil {
		return checkInfo{
			Status: "error",
			Detail: "database not initialized",
		}
	}

	sqlDB, err := db.DB()
	if err != nil {
		return checkInfo{
			Status: "error",
			Detail: err.Error(),
		}
	}

	if err := sqlDB.Ping(); err != nil {
		return checkInfo{
			Status: "error",
			Detail: err.Error(),
		}
	}

	return checkInfo{
		Status:    "ok",
		LatencyMs: time.Since(start).Milliseconds(),
	}
}

// checkRedis checks the configured Redis instance with authentication.
func checkRedis() checkInfo {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := cache.Ping(ctx); err != nil {
		return checkInfo{
			Status: "error",
			Detail: err.Error(),
		}
	}

	return checkInfo{
		Status:    "ok",
		LatencyMs: time.Since(start).Milliseconds(),
	}
}

// checkQdrant 检查 Qdrant 向量数据库连接
func checkQdrant() checkInfo {
	start := time.Now()

	provider := vectordb.GetProvider()
	if provider == nil {
		return checkInfo{
			Status: "error",
			Detail: "vector database provider not initialized",
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := provider.ListCollections(ctx); err != nil {
		return checkInfo{
			Status: "error",
			Detail: err.Error(),
		}
	}

	return checkInfo{
		Status:    "ok",
		LatencyMs: time.Since(start).Milliseconds(),
	}
}
