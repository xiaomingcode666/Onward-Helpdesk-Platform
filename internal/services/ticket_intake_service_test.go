package services_test

import (
	"testing"
	"time"

	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"
)

// 受理规则已移除；这里只验证保留下来的来源登记：来源去重、来电资料快照和接入时间。
func TestTicketSourceRecordProvenanceAndDedup(t *testing.T) {
	t.Setenv("RHD_BOOTSTRAP_ADMIN_PASSWORD", "Intake-test-only-2026!")
	setupTicketTestDB(t)
	tenant, _, _, _, _ := createTicketAfterSalesFixture(t, "source-record", enums.StatusOk)
	operator := createTestOperator(t, "phone-agent")
	operator.TenantID = tenant.ID
	operator.Permissions = []string{constants.PermissionTicketUpdate.Code, constants.PermissionTicketCreate.Code, constants.PermissionTicketView.Code}

	received := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	external := request.CreateTicketRequest{
		Title: "邮件报修", Description: "客户来信反映故障", Source: string(enums.TicketSourceEmail), Channel: "email",
		TicketIntakeInput: dto.TicketIntakeInput{
			SourceRecordID: "mail-1", ProjectKey: "project-a", TicketType: "incident",
			CallerName: "来电人", CallerPhone: "13800000000", ReceivedAt: &received,
		},
	}
	first, err := services.TicketService.CreateTicket(external, operator)
	if err != nil {
		t.Fatal(err)
	}
	if first.SourceRecordID != "mail-1" || first.ProjectKey != "project-a" || first.TicketType != "incident" {
		t.Fatalf("provenance not recorded: %+v", first)
	}
	if first.CallerName != "来电人" || first.CallerPhone != "13800000000" || first.ReceivedAt == nil || !first.ReceivedAt.Equal(received) {
		t.Fatalf("caller snapshot not recorded: %+v", first)
	}
	if first.SourceRecordKey == nil {
		t.Fatal("source record key missing")
	}

	// 同一条来源记录换个请求编号也不能重复建单。
	duplicate := external
	duplicate.IdempotencyKey = "another-attempt"
	if _, err := services.TicketService.CreateTicket(duplicate, operator); err == nil {
		t.Fatal("duplicate source record accepted")
	}

	// 页面建单不允许夹带来电资料。
	manual := request.CreateTicketRequest{
		Title: "页面报修", Description: "客户来电报修", Source: "manual", Channel: "enterprise",
		TicketIntakeInput: dto.TicketIntakeInput{ProjectKey: "project-a", TicketType: "incident", CallerPhone: "13800000000"},
	}
	if _, err := services.TicketService.CreateTicket(manual, operator); err == nil {
		t.Fatal("phone snapshot accepted on page intake")
	}

	// 接入时间不能是未来。
	future := time.Now().Add(48 * time.Hour)
	invalid := external
	invalid.IdempotencyKey = ""
	invalid.TicketIntakeInput.SourceRecordID = "mail-2"
	invalid.TicketIntakeInput.ReceivedAt = &future
	if _, err := services.TicketService.CreateTicket(invalid, operator); err == nil {
		t.Fatal("future received_at accepted")
	}

	view := services.BuildTicketIntakeDTO(first)
	if view.SourceRecordID != "mail-1" || view.CallerName != "来电人" || view.ProjectKey != "project-a" {
		t.Fatalf("unexpected intake view: %+v", view)
	}
}
