package dashboard

import (
	"net/http/httptest"
	"testing"

	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"

	"github.com/gin-gonic/gin"
)

func TestAgentDispatchCandidatePermissionAllowsAssignmentRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, permission := range []constants.Permission{
		constants.PermissionAgentView,
		constants.PermissionAgentTeamView,
		constants.PermissionTicketAssign,
		constants.PermissionTicketChangeStatus,
		constants.PermissionConversationAssign,
		constants.PermissionConversationTransfer,
	} {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Set("authPrincipal", &dto.AuthPrincipal{
			Permissions: []string{permission.Code},
		})
		if _, err := requireAgentDispatchCandidatePermission(ctx); err != nil {
			t.Fatalf("permission %s should load dispatch candidates: %v", permission.Code, err)
		}
	}
}

func TestAgentDispatchCandidatePermissionRejectsUnrelatedRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("authPrincipal", &dto.AuthPrincipal{
		Permissions: []string{constants.PermissionTicketView.Code},
	})
	if _, err := requireAgentDispatchCandidatePermission(ctx); err == nil {
		t.Fatal("ticket.view alone should not load dispatch candidates")
	}
}
