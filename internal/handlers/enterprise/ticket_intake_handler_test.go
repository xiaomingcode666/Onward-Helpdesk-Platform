package enterprise

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"testing"
)

func TestPhoneIntakeEnterpriseContract(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	if err := db.AutoMigrate(&models.TicketNoSequence{}, &models.TicketContextSnapshot{}, &models.TicketTag{}, &models.DomainEvent{}, &models.OutboxRecord{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Tenant{ID: 8001, Name: "Phone intake", Status: enums.StatusOk}).Error; err != nil {
		t.Fatal(err)
	}
	createCtx, createRec := newEnterpriseContractJSONContext(http.MethodPost, "/api/enterprise/v1/tickets", 8001,
		`{"title":"Phone request","description":"Customer needs assistance","source":"manual","channel":"phone","project_key":"p","ticket_type":"incident","source_record_id":"phone-contract-001","caller_name":"Caller","idempotency_key":"phone-contract-submit"}`)
	TicketCreate(createCtx)
	var created dto.EnterpriseTicketListItemDTO
	decodeEnterpriseData(t, createRec, &created)
	t.Logf("Synthetic manual phone creation response: %s", createRec.Body.String())
	if created.Channel != "phone" || created.SourceRecordID != "phone-contract-001" || created.CallerName != "Caller" || created.ContextStatus != "context_incomplete" {
		t.Fatalf("unexpected response %+v", created)
	}
	t.Run("policy configuration and completion", func(t *testing.T) {
		ctx, rec := newEnterpriseContractJSONContext(http.MethodPut, "/api/enterprise/v1/ticket-settings/intake", 8001, `{"rules":[{"project_key":"p","channel":"phone","ticket_type":"incident","required_fields":["caller_phone"]}]}`)
		TicketIntakePolicyUpdate(ctx)
		var policy dto.TicketIntakePolicy
		decodeEnterpriseData(t, rec, &policy)
		path := fmt.Sprintf("/api/enterprise/v1/tickets/%d/intake", created.ID)
		ctx, rec = newEnterpriseContractJSONContext(http.MethodPatch, path, 8001, `{"project_key":"p","ticket_type":"incident","caller_name":"Caller","caller_phone":"+1 0000000000"}`, gin.Param{Key: "id", Value: fmt.Sprint(created.ID)})
		TicketIntakeComplete(ctx)
		var aggregate dto.TicketAggregateDTO
		decodeEnterpriseData(t, rec, &aggregate)
		t.Logf("Synthetic phone context completion response: %s", rec.Body.String())
		if aggregate.Ticket.ContextStatus != "complete" || aggregate.Ticket.SourceRecordID != created.SourceRecordID {
			t.Fatalf("bad completion %+v", aggregate.Ticket)
		}
	})
	t.Run("immutable and cross tenant fields rejected", func(t *testing.T) {
		for _, tc := range []struct {
			tenant int64
			body   string
		}{
			{8001, `{"source_record_id":"overwrite"}`}, {8001, `{"channel":"email"}`}, {8002, `{"caller_phone":"another"}`},
		} {
			ctx, rec := newEnterpriseContractJSONContext(http.MethodPatch, "/tickets/id/intake", tc.tenant, tc.body, gin.Param{Key: "id", Value: fmt.Sprint(created.ID)})
			TicketIntakeComplete(ctx)
			if decodeEnterpriseEnvelope(t, rec).Success {
				t.Fatalf("invalid mutation accepted %s", rec.Body.String())
			}
		}
	})
	t.Run("permissions required", func(t *testing.T) {
		for _, handler := range []gin.HandlerFunc{TicketIntakePolicyGet, TicketIntakePolicyUpdate, TicketIntakeComplete} {
			ctx, rec := newEnterpriseContractJSONContext(http.MethodPost, "/intake", 8001, `{}`)
			ctx.MustGet("authPrincipal").(*dto.AuthPrincipal).Permissions = nil
			handler(ctx)
			if decodeEnterpriseEnvelope(t, rec).Success {
				t.Fatal("permissionless access accepted")
			}
		}
	})
	t.Run("phone cannot silently enter conversation path", func(t *testing.T) {
		ctx, rec := newEnterpriseContractJSONContext(http.MethodPost, "/tickets", 8001, `{"title":"x","description":"x","channel":"phone","conversation_id":123}`)
		TicketCreate(ctx)
		if decodeEnterpriseEnvelope(t, rec).Success {
			t.Fatal("phone conversation bypass accepted")
		}
	})
}

func TestPhoneIntakeRequiresAuthentication(t *testing.T) {
	for _, handler := range []gin.HandlerFunc{TicketCreate, TicketIntakePolicyGet, TicketIntakePolicyUpdate, TicketIntakeComplete} {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/intake", nil)
		handler(ctx)
		if decodeEnterpriseEnvelope(t, rec).Success {
			t.Fatal("unauthenticated intake request accepted")
		}
	}
}
