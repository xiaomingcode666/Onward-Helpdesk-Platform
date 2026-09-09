package dashboard

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"

	"github.com/gin-gonic/gin"
)

func TestTicketCreateWithInitialAssigneeRequiresAssignPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name string
		body string
		call func(*gin.Context)
	}{
		{
			name: "plain create",
			body: `{"title":"工单","description":"问题","currentAssigneeId":101}`,
			call: TicketPostCreate,
		},
		{
			name: "conversation create",
			body: `{"conversationId":1,"title":"工单","description":"问题","currentAssigneeId":101}`,
			call: TicketPostCreate_from_conversation,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Request = httptest.NewRequest("POST", "/api/dashboard/ticket/create", bytes.NewBufferString(tc.body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			ctx.Set("authPrincipal", &dto.AuthPrincipal{
				Permissions: []string{constants.PermissionTicketCreate.Code},
			})

			tc.call(ctx)

			var envelope struct {
				Success   bool `json:"success"`
				ErrorCode int  `json:"errorCode"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("decode response: %v\nbody=%s", err, rec.Body.String())
			}
			if envelope.Success || envelope.ErrorCode != errorsx.CodeAuthForbidden {
				t.Fatalf("initial assignee create should be forbidden, got %+v body=%s", envelope, rec.Body.String())
			}
		})
	}
}
