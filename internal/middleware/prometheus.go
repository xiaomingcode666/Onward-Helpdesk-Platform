package middleware

import (
	"strconv"
	"time"

	"remotehelpdesk/internal/pkg/observability"

	"github.com/gin-gonic/gin"
)

func PrometheusMetricsMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if ctx.Request.URL.Path == "/metrics" || isWebSocketRequest(ctx) {
			ctx.Next()
			return
		}

		start := time.Now()
		observability.HTTPRequestsInFlight.Inc()
		defer observability.HTTPRequestsInFlight.Dec()

		ctx.Next()

		route := ctx.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := strconv.Itoa(ctx.Writer.Status())
		labels := []string{ctx.Request.Method, route, status}
		observability.HTTPRequestsTotal.WithLabelValues(labels...).Inc()
		observability.HTTPRequestDuration.WithLabelValues(labels...).Observe(time.Since(start).Seconds())
	}
}

func isWebSocketRequest(ctx *gin.Context) bool {
	return ctx.GetHeader("Upgrade") == "websocket"
}
