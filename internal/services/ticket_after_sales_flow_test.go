package services_test

import (
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func TestTicketAfterSalesTransitionAllowsDefinedFlowAndRejectsIllegalJumps(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "after-sales-flow")
	tenant, product, productModel, device, serviceCode := createTicketAfterSalesFixture(t, "after-sales-flow", enums.StatusOk)

	ticket, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:          "售后状态流转",
		Description:    "售后状态流转",
		TenantID:       tenant.ID,
		ProductID:      product.ID,
		ProductModelID: productModel.ID,
		DeviceID:       device.ID,
		ServiceCodeID:  serviceCode.ID,
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if err := repositories.TicketRepository.Updates(sqls.DB(), ticket.ID, map[string]any{
		"status": enums.TicketStatusPendingAcceptance,
	}); err != nil {
		t.Fatalf("seed pending_acceptance status: %v", err)
	}

	_, err = services.TicketService.Transition(request.TransitionTicketRequest{
		TicketID: ticket.ID,
		Status:   string(enums.TicketStatusProcessing),
		Remark:   "非法跨阶段跳转",
	}, operator)
	if err == nil {
		t.Fatalf("expected direct pending_acceptance -> processing transition to be rejected")
	}
	afterIllegalJump := services.TicketService.Get(ticket.ID)
	if afterIllegalJump.Status != enums.TicketStatusPendingAcceptance {
		t.Fatalf("expected status to remain pending_acceptance, got %s", afterIllegalJump.Status)
	}

	allowedFlow := []enums.TicketStatus{
		enums.TicketStatusAccepted,
		enums.TicketStatusPendingDispatch,
		enums.TicketStatusPendingAssigneeAccept,
		enums.TicketStatusProcessing,
		enums.TicketStatusVideoSupport,
		enums.TicketStatusResolved,
		enums.TicketStatusClosed,
	}
	for _, nextStatus := range allowedFlow {
		updated, err := services.TicketService.Transition(request.TransitionTicketRequest{
			TicketID: ticket.ID,
			Status:   string(nextStatus),
			Remark:   "按流程推进",
		}, operator)
		if err != nil {
			t.Fatalf("Transition() to %s error = %v", nextStatus, err)
		}
		if updated.Status != nextStatus {
			t.Fatalf("expected status %s, got %s", nextStatus, updated.Status)
		}
		if nextStatus == enums.TicketStatusResolved && updated.ResolvedAt == nil {
			t.Fatalf("expected resolved_at to be set when ticket is resolved")
		}
		if nextStatus == enums.TicketStatusClosed && updated.HandledAt == nil {
			t.Fatalf("expected handled_at to be set when ticket is closed")
		}
	}

	_, err = services.TicketService.Transition(request.TransitionTicketRequest{
		TicketID: ticket.ID,
		Status:   string(enums.TicketStatusProcessing),
		Remark:   "关闭后不允许回退",
	}, operator)
	if err == nil {
		t.Fatalf("expected closed -> processing transition to be rejected")
	}
}

func TestTicketRepairRecordCreateRequiresAfterSalesContextAndPersistsRepairFields(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "repair-record")
	withoutContext, err := services.TicketService.CreateTicket(createTestTicketRequest("repair-record-without-context"), operator)
	if err != nil {
		t.Fatalf("CreateTicket() without context error = %v", err)
	}

	_, err = services.TicketService.CreateRepairRecord(request.CreateTicketRepairRecordRequest{
		TicketID:     withoutContext.ID,
		Conclusion:   "已修复",
		RootCause:    "电源模块接触不良",
		RepairMethod: "重新固定电源模块",
	}, operator)
	if err == nil {
		t.Fatalf("expected repair record creation to require tenant/product/device context")
	}

	tenant, product, productModel, device, serviceCode := createTicketAfterSalesFixture(t, "repair-record", enums.StatusOk)
	ticket, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:          "维修记录",
		Description:    "维修记录",
		TenantID:       tenant.ID,
		ProductID:      product.ID,
		ProductModelID: productModel.ID,
		DeviceID:       device.ID,
		ServiceCodeID:  serviceCode.ID,
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() with context error = %v", err)
	}

	record, err := services.TicketService.CreateRepairRecord(request.CreateTicketRepairRecordRequest{
		TicketID:     ticket.ID,
		Conclusion:   "已恢复正常",
		RootCause:    "风扇模块堵塞导致过热保护",
		RepairMethod: "清洁风扇模块并复测温控",
	}, operator)
	if err != nil {
		t.Fatalf("CreateRepairRecord() error = %v", err)
	}
	if record.ID <= 0 {
		t.Fatalf("expected persisted repair record id")
	}
	assertRepairRecordSnapshot(t, record, ticket, tenant.ID, product.ID, device.ID)
	if record.Conclusion != "已恢复正常" || record.RootCause != "风扇模块堵塞导致过热保护" || record.RepairMethod != "清洁风扇模块并复测温控" {
		t.Fatalf("unexpected repair record fields: %+v", record)
	}
	if record.VisibleToCustomer {
		t.Fatalf("expected repair record to stay internal unless explicitly shared")
	}

	persisted := repositories.TicketRepairRecordRepository.Get(sqls.DB(), record.ID)
	if persisted == nil {
		t.Fatalf("expected repair record to be persisted")
	}
	assertRepairRecordSnapshot(t, persisted, ticket, tenant.ID, product.ID, device.ID)
	if persisted.Conclusion != record.Conclusion || persisted.RootCause != record.RootCause || persisted.RepairMethod != record.RepairMethod {
		t.Fatalf("unexpected persisted repair record: %+v", persisted)
	}

	detail, err := services.TicketService.GetDetail(ticket.ID)
	if err != nil {
		t.Fatalf("GetDetail() error = %v", err)
	}
	if len(detail.RepairRecords) != 1 || detail.RepairRecords[0].ID != record.ID {
		t.Fatalf("expected detail repair records to include created record, got %+v", detail.RepairRecords)
	}
}

func assertRepairRecordSnapshot(t *testing.T, record *models.TicketRepairRecord, ticket *models.Ticket, tenantID, productID, deviceID int64) {
	t.Helper()
	if record.TenantID != tenantID || record.ProductID != productID || record.DeviceID != deviceID {
		t.Fatalf("unexpected repair record context: %+v", record)
	}
	if record.TicketID != ticket.ID || record.ProductModelID != ticket.ProductModelID || record.ServiceCodeID != ticket.ServiceCodeID {
		t.Fatalf("unexpected repair record ticket snapshot: %+v ticket=%+v", record, ticket)
	}
}
