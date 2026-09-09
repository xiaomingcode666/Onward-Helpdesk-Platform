package observability

import (
	"net/http"
	"time"

	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	HTTPRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of completed HTTP requests.",
	}, []string{"method", "route", "status"})
	HTTPRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30},
	}, []string{"method", "route", "status"})
	HTTPRequestsInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "Number of HTTP requests currently being served.",
	})
	DependencyUp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "application_dependency_up",
		Help: "Whether a required application dependency passed its latest readiness check.",
	}, []string{"dependency"})
	DependencyLatency = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "application_dependency_check_duration_seconds",
		Help: "Duration of the latest application dependency readiness check.",
	}, []string{"dependency"})
	RealtimeConnections = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "realtime_websocket_connections",
		Help: "Number of active WebSocket connections in this application instance.",
	}, []string{"role"})
)

func init() {
	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		HTTPRequestsInFlight,
		DependencyUp,
		DependencyLatency,
		RealtimeConnections,
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "ai_reply_jobs_failed_recent_total",
			Help: "Number of AI reply jobs that exhausted retries in the last hour.",
		}, func() float64 {
			return databaseCount(&models.AIReplyJob{}, "status = ? AND finished_at >= ?", models.AIReplyJobStatusFailed, time.Now().Add(-time.Hour))
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "ai_reply_jobs_pending_total",
			Help: "Number of AI reply jobs waiting to run or retry.",
		}, func() float64 {
			return databaseCount(&models.AIReplyJob{}, "status IN ?", []string{models.AIReplyJobStatusPending, models.AIReplyJobStatusWaitingRetry})
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "tickets_sla_overdue_total",
			Help: "Number of open tickets whose SLA due time has passed.",
		}, func() float64 {
			return databaseCount(&models.Ticket{}, "sla_due_at IS NOT NULL AND sla_due_at < ? AND status NOT IN ?", time.Now(), []string{"resolved", "closed", "cancelled"})
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "tickets_unassigned_open_total",
			Help: "Number of open tickets that remain in a team pool without an engineer.",
		}, func() float64 {
			return databaseCount(&models.Ticket{}, "current_assignee_id = 0 AND status NOT IN ?", []string{"resolved", "closed", "cancelled"})
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "db_connection_pool_active",
			Help: "Number of active database connections.",
		}, func() float64 { return databaseStat(func(stats dbStats) int { return stats.inUse }) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "db_connection_pool_idle",
			Help: "Number of idle database connections.",
		}, func() float64 { return databaseStat(func(stats dbStats) int { return stats.idle }) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "db_connection_pool_max",
			Help: "Maximum number of open database connections.",
		}, func() float64 { return databaseStat(func(stats dbStats) int { return stats.maxOpen }) }),
	)
	for _, dependency := range []string{"postgres", "redis", "qdrant", "storage", "outbox"} {
		DependencyUp.WithLabelValues(dependency).Set(0)
		DependencyLatency.WithLabelValues(dependency).Set(0)
	}
}

func databaseCount(model any, query string, args ...any) float64 {
	db := sqls.DB()
	if db == nil || !db.Migrator().HasTable(model) {
		return 0
	}
	var count int64
	if err := db.Model(model).Where(query, args...).Count(&count).Error; err != nil {
		return 0
	}
	return float64(count)
}

type dbStats struct {
	inUse   int
	idle    int
	maxOpen int
}

func databaseStat(selectValue func(dbStats) int) float64 {
	db := sqls.DB()
	if db == nil {
		return 0
	}
	sqlDB, err := db.DB()
	if err != nil {
		return 0
	}
	stats := sqlDB.Stats()
	return float64(selectValue(dbStats{inUse: stats.InUse, idle: stats.Idle, maxOpen: stats.MaxOpenConnections}))
}

func RecordDependencyCheck(name string, healthy bool, latency time.Duration) {
	value := float64(0)
	if healthy {
		value = 1
	}
	DependencyUp.WithLabelValues(name).Set(value)
	DependencyLatency.WithLabelValues(name).Set(latency.Seconds())
}

func Handler() http.Handler {
	return promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}
