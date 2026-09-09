package enterprise

import (
	"strings"
	"time"

	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

// ReminderPoll returns lightweight, user-scoped reminders for the enterprise shell.
// GET /api/enterprise/v1/reminders/poll
func ReminderPoll(ctx *gin.Context) {
	tenantID, _, principal, ok := resolveWorkbenchIdentity(ctx)
	if !ok {
		return
	}
	result, err := services.EnterpriseReminderService.Poll(
		ctx.Request.Context(),
		tenantID,
		principal,
		parseReminderSince(ctx.Query("since")),
		time.Now(),
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func parseReminderSince(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed
	}
	return time.Time{}
}
