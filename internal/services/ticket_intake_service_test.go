package services_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"
)

func TestEvaluateTicketIntake(t *testing.T) {
	policy := dto.TicketIntakePolicy{Rules: []dto.TicketIntakeRule{
		{ProjectKey: "alpha", Channel: "phone", TicketType: "incident", RequiredFields: []string{"caller_phone", "device_id"}},
		{ProjectKey: "alpha", Channel: "phone", TicketType: "request", RequiredFields: []string{"caller_name"}},
		{ProjectKey: "alpha", Channel: "email", TicketType: "incident", RequiredFields: []string{"customer_id"}},
	}}
	for _, tc := range []struct {
		name   string
		ticket models.Ticket
		want   []string
	}{
		{"missing policy", models.Ticket{Channel: "phone"}, []string{"project_key", "ticket_type", "projectPolicy"}},
		{"missing context", models.Ticket{ProjectKey: "alpha", Channel: "phone", TicketType: "incident"}, []string{"caller_phone", "device_id"}},
		{"complete", models.Ticket{ProjectKey: "alpha", Channel: "phone", TicketType: "incident", CallerPhone: "123", DeviceID: 1}, []string{}},
		{"type changes requirements", models.Ticket{ProjectKey: "alpha", Channel: "phone", TicketType: "request", CallerName: "   "}, []string{"caller_name"}},
		{"channel changes requirements", models.Ticket{ProjectKey: "alpha", Channel: "email", TicketType: "incident"}, []string{"customer_id"}},
		{"unknown project", models.Ticket{ProjectKey: "beta", Channel: "phone", TicketType: "incident"}, []string{"projectPolicy"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := services.EvaluateTicketIntake(&tc.ticket, policy); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("missing = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidateTicketIntakePolicy(t *testing.T) {
	rule := dto.TicketIntakeRule{ProjectKey: "project", Channel: "phone", TicketType: "incident", RequiredFields: []string{"caller_name"}}
	if err := services.ValidateTicketIntakePolicy(dto.TicketIntakePolicy{Rules: []dto.TicketIntakeRule{rule}}); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []func(*dto.TicketIntakeRule){
		func(r *dto.TicketIntakeRule) { r.Channel = "public-phone" }, func(r *dto.TicketIntakeRule) { r.ProjectKey = "" },
		func(r *dto.TicketIntakeRule) { r.TicketType = " " }, func(r *dto.TicketIntakeRule) { r.RequiredFields = []string{"made_up"} },
		func(r *dto.TicketIntakeRule) { r.RequiredFields = []string{"caller_name", "caller_name"} },
	} {
		copy := rule
		mutation(&copy)
		if err := services.ValidateTicketIntakePolicy(dto.TicketIntakePolicy{Rules: []dto.TicketIntakeRule{copy}}); err == nil {
			t.Fatalf("accepted invalid rule %+v", copy)
		}
	}
	if err := services.ValidateTicketIntakePolicy(dto.TicketIntakePolicy{Rules: []dto.TicketIntakeRule{rule, rule}}); err == nil {
		t.Fatal("duplicate rule accepted")
	}
}

func TestManualPhoneIntake(t *testing.T) {
	t.Setenv("RHD_BOOTSTRAP_ADMIN_PASSWORD", "Intake-test-only-2026!")
	setupTicketTestDB(t)
	tenant, product, _, device, _ := createTicketAfterSalesFixture(t, "phone-intake", enums.StatusOk)
	operator := createTestOperator(t, "phone-agent")
	operator.TenantID = tenant.ID
	operator.Permissions = []string{constants.PermissionTicketUpdate.Code, constants.PermissionTicketCreate.Code, constants.PermissionTicketView.Code}
	policy := dto.TicketIntakePolicy{Rules: []dto.TicketIntakeRule{{ProjectKey: "project-a", Channel: "phone", TicketType: "incident", RequiredFields: []string{"caller_name", "caller_phone", "product_id", "device_id"}}}}
	if err := services.UpdateTicketIntakePolicy(tenant.ID, policy, operator); err != nil {
		t.Fatal(err)
	}
	received := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	base := request.CreateTicketRequest{Title: "Phone support", Description: "Customer reports a fault", Source: "manual", Channel: "phone", TicketIntakeInput: dto.TicketIntakeInput{ProjectKey: "project-a", TicketType: "incident", ReceivedAt: &received}}
	var recorded *models.Ticket
	t.Run("missing values persist without fabrication and can be completed", func(t *testing.T) {
		var err error
		recorded, err = services.TicketService.CreateTicket(base, operator)
		if err != nil {
			t.Fatal(err)
		}
		if recorded.ContextStatus != "context_incomplete" || recorded.CallerName != "" || recorded.CallerPhone != "" || recorded.CustomerID != 0 || !strings.HasPrefix(recorded.SourceRecordID, "MP-") {
			t.Fatalf("unexpected intake %+v", recorded)
		}
		if !recorded.ReceivedAt.Equal(received) {
			t.Fatal("received timestamp changed")
		}
		beforeID := recorded.SourceRecordID
		input := dto.CompleteTicketIntakeRequest{ProjectKey: "project-a", TicketType: "incident", CallerName: "Caller snapshot", CallerPhone: "+86 10000000000", ProductID: product.ID, DeviceID: device.ID}
		if err := services.CompleteTicketIntake(recorded.ID, input, operator); err != nil {
			t.Fatal(err)
		}
		recorded = services.TicketService.Get(recorded.ID)
		if recorded.ContextStatus != "complete" || recorded.SourceRecordID != beforeID || recorded.Channel != "phone" || !recorded.ReceivedAt.Equal(received) {
			t.Fatalf("completion altered provenance or failed: %+v", recorded)
		}
		input.CallerPhone = "different"
		if err := services.CompleteTicketIntake(recorded.ID, input, operator); err == nil {
			t.Fatal("caller snapshot overwrite accepted")
		}
		var progresses []models.TicketProgress
		if err := sqls.DB().Where("ticket_id = ?", recorded.ID).Order("id").Find(&progresses).Error; err != nil {
			t.Fatal(err)
		}
		if len(progresses) != 2 || !strings.Contains(progresses[0].MetadataJSON, beforeID) || !strings.Contains(progresses[1].MetadataJSON, "before") || progresses[1].AuthorID != operator.UserID {
			t.Fatalf("audit missing: %+v", progresses)
		}
	})
	if recorded == nil {
		t.Fatal("initial intake was not created")
	}
	t.Run("responses and source ID search", func(t *testing.T) {
		list, err := services.EnterpriseTicketService.List(tenant.ID, services.EnterpriseTicketQuery{Search: recorded.SourceRecordID})
		if err != nil || list == nil || list.Total != 1 {
			t.Fatalf("search failed: %+v, %v", list, err)
		}
		if list.Items[0].SourceRecordID != recorded.SourceRecordID || list.Items[0].ContextStatus != "complete" {
			t.Fatalf("missing intake list fields: %+v", list.Items[0])
		}
		body, err := json.Marshal(builders.BuildTicket(recorded))
		if err != nil || !strings.Contains(string(body), `"source_record_id":"`+recorded.SourceRecordID+`"`) {
			t.Fatalf("dashboard response: %s %v", body, err)
		}
		aggregate, err := services.EnterpriseTicketService.GetAggregate(tenant.ID, recorded.ID)
		if err != nil || aggregate.Ticket.CallerPhone != recorded.CallerPhone {
			t.Fatalf("aggregate lost snapshot: %v", err)
		}
	})
	t.Run("idempotency and source uniqueness", func(t *testing.T) {
		req := base
		req.SourceRecordID = "external-unique"
		req.IdempotencyKey = "phone-submit-retry"
		first, err := services.TicketService.CreateTicket(req, operator)
		if err != nil {
			t.Fatal(err)
		}
		retry, err := services.TicketService.CreateTicket(req, operator)
		if err != nil || retry.ID != first.ID {
			t.Fatalf("retry duplicated: %v", err)
		}
		req.IdempotencyKey = "different-attempt"
		if _, err := services.TicketService.CreateTicket(req, operator); err == nil {
			t.Fatal("duplicate source accepted")
		}
		clone := *first
		clone.ID = 0
		clone.TicketNo = "UNIQUE-RACE-PROBE"
		clone.IdempotencyKey = nil
		if err := sqls.DB().Create(&clone).Error; err == nil {
			t.Fatal("database unique constraint missing")
		}
		var count int64
		sqls.DB().Model(&models.DomainEvent{}).Where("event_type = ? AND aggregate_id = ?", "ticket.created", fmt.Sprint(first.ID)).Count(&count)
		if count != 1 {
			t.Fatalf("expected one durable event, got %d", count)
		}
	})
	t.Run("tenant isolation", func(t *testing.T) {
		other := &models.Tenant{Name: "other-intake-tenant", Status: enums.StatusOk}
		if err := sqls.DB().Create(other).Error; err != nil {
			t.Fatal(err)
		}
		otherOp := *operator
		otherOp.TenantID = other.ID
		req := base
		req.SourceRecordID = recorded.SourceRecordID
		req.TenantID = tenant.ID
		created, err := services.TicketService.CreateTicket(req, &otherOp)
		if err != nil {
			t.Fatal(err)
		}
		if created.TenantID != other.ID || !strings.Contains(created.MissingContextJSON, "projectPolicy") {
			t.Fatal("tenant was not derived from identity")
		}
		if err := services.CompleteTicketIntake(recorded.ID, dto.CompleteTicketIntakeRequest{}, &otherOp); err == nil {
			t.Fatal("cross tenant completion accepted")
		}
		if _, err := services.EnterpriseTicketService.GetAggregate(other.ID, recorded.ID); err == nil {
			t.Fatal("cross tenant detail exposed")
		}
		if err := services.UpdateTicketIntakePolicy(tenant.ID, policy, &otherOp); err == nil {
			t.Fatal("cross tenant policy edit accepted")
		}
		badRef := base
		badRef.ProductID = product.ID
		if _, err := services.TicketService.CreateTicket(badRef, &otherOp); err == nil {
			t.Fatal("foreign product accepted")
		}
	})
	t.Run("invalid requests rejected", func(t *testing.T) {
		future := time.Now().Add(time.Hour)
		for _, mutate := range []func(*request.CreateTicketRequest){
			func(r *request.CreateTicketRequest) { r.Channel = "unknown" }, func(r *request.CreateTicketRequest) { r.Source = "conversation" },
			func(r *request.CreateTicketRequest) { r.SourceRecordID = strings.Repeat("x", 161) }, func(r *request.CreateTicketRequest) { r.ReceivedAt = &future },
			func(r *request.CreateTicketRequest) { r.CallerPhone = "123\n456" },
		} {
			req := base
			mutate(&req)
			if _, err := services.TicketService.CreateTicket(req, operator); err == nil {
				t.Fatalf("invalid input accepted: %+v", req)
			}
		}
	})
	t.Run("legacy manual channels remain compatible", func(t *testing.T) {
		for _, channel := range []string{"", "enterprise", "dashboard", "web", "im", "widget"} {
			req := request.CreateTicketRequest{Title: "Legacy manual", Description: "Existing creation client", Source: "manual", Channel: channel}
			created, err := services.TicketService.CreateTicket(req, operator)
			if err != nil || created.SourceRecordID != "" || created.ContextStatus != "not_evaluated" {
				t.Fatalf("legacy channel %q changed: %v", channel, err)
			}
		}
	})
	t.Run("shared source preservation across canonical channels", func(t *testing.T) {
		for _, channel := range []string{"email", "monitoring_alert", "api", "webhook", "whatsapp", "chatbot_handoff"} {
			req := base
			req.Channel = channel
			req.SourceRecordID = "shared-external-ref"
			created, err := services.TicketService.CreateTicket(req, operator)
			if err != nil || created.Channel != channel || created.SourceRecordID != req.SourceRecordID {
				t.Fatalf("channel %s: %v", channel, err)
			}
		}
	})
	t.Run("policy change is applied on completion", func(t *testing.T) {
		identityPolicy := dto.TicketIntakePolicy{Rules: []dto.TicketIntakeRule{{ProjectKey: "identity", Channel: "phone", TicketType: "request", RequiredFields: []string{"customer_id"}}}}
		if err := services.UpdateTicketIntakePolicy(tenant.ID, identityPolicy, operator); err != nil {
			t.Fatal(err)
		}
		req := base
		req.ProjectKey = "identity"
		req.TicketType = "request"
		ticket, err := services.TicketService.CreateTicket(req, operator)
		if err != nil {
			t.Fatal(err)
		}
		customerID := createTestCustomer(t, "phone-confirmed-identity")
		completion := dto.CompleteTicketIntakeRequest{ProjectKey: "identity", TicketType: "request", CustomerID: 99999999}
		if err := services.CompleteTicketIntake(ticket.ID, completion, operator); err == nil {
			t.Fatal("unknown customer identity accepted")
		}
		completion.CustomerID = customerID
		if err := services.CompleteTicketIntake(ticket.ID, completion, operator); err != nil {
			t.Fatal(err)
		}
		if updated := services.TicketService.Get(ticket.ID); updated.CustomerID != customerID || updated.ContextStatus != "complete" || updated.CallerName != "" || updated.CallerPhone != "" {
			t.Fatalf("identity completion fabricated snapshot: %+v", updated)
		}
		if err := services.UpdateTicketIntakePolicy(tenant.ID, dto.TicketIntakePolicy{}, operator); err != nil {
			t.Fatal(err)
		}
		input := dto.CompleteTicketIntakeRequest{ProjectKey: recorded.ProjectKey, TicketType: recorded.TicketType, CallerName: recorded.CallerName, CallerPhone: recorded.CallerPhone}
		if err := services.CompleteTicketIntake(recorded.ID, input, operator); err != nil {
			t.Fatal(err)
		}
		if got := services.TicketService.Get(recorded.ID); got.ContextStatus != "context_incomplete" || !strings.Contains(got.MissingContextJSON, "projectPolicy") {
			t.Fatal("stale policy result")
		}
	})
	t.Run("create event failure rolls back ticket and progress", func(t *testing.T) {
		callback := "test:intake-outbox-failure"
		if err := sqls.DB().Callback().Create().Before("gorm:create").Register(callback, func(db *gorm.DB) {
			if db.Statement.Schema != nil && db.Statement.Schema.Name == "OutboxRecord" {
				db.AddError(errors.New("injected outbox failure"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		defer sqls.DB().Callback().Create().Remove(callback)
		req := base
		req.SourceRecordID = "rollback-source"
		if _, err := services.TicketService.CreateTicket(req, operator); err == nil {
			t.Fatal("expected outbox failure")
		}
		var count int64
		if err := sqls.DB().Model(&models.Ticket{}).Where("source_record_id = ?", req.SourceRecordID).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("ticket survived rollback: %d %v", count, err)
		}
	})
}
