package services_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func TestTicketAutoCloseClosesEligibleTicketsAndSkipsSafetyCriticalModules(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "ticket-auto-close")
	tenant, product, productModel, device, serviceCode := createTicketAfterSalesFixture(t, "ticket-auto-close", enums.StatusOk)
	operator.TenantID = tenant.ID

	regularModule := &models.ProductModule{
		TenantID: tenant.ID, ProductID: product.ID, ModuleCode: "CTRL", Name: "控制模块", Status: enums.StatusOk,
	}
	if err := repositories.ProductModuleRepository.Create(sqls.DB(), regularModule); err != nil {
		t.Fatalf("create regular product module: %v", err)
	}
	safetyModule := &models.ProductModule{
		TenantID: tenant.ID, ProductID: product.ID, ModuleCode: "SAFE", Name: "安全模块", IsSafetyCritical: true, Status: enums.StatusOk,
	}
	if err := repositories.ProductModuleRepository.Create(sqls.DB(), safetyModule); err != nil {
		t.Fatalf("create safety product module: %v", err)
	}

	createResolvedTicket := func(title, faultCode string, moduleID int64) *models.Ticket {
		t.Helper()
		ticket, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
			Title: title, Description: title, TenantID: tenant.ID, ProductID: product.ID,
			ProductModelID: productModel.ID, ProductModuleID: moduleID, DeviceID: device.ID,
			ServiceCodeID: serviceCode.ID, FaultCode: faultCode,
		}, operator)
		if err != nil {
			t.Fatalf("CreateTicket(%s): %v", title, err)
		}
		if err := repositories.TicketRepository.Updates(sqls.DB(), ticket.ID, map[string]any{"status": enums.TicketStatusProcessing}); err != nil {
			t.Fatalf("set processing status: %v", err)
		}
		if _, err := services.TicketService.CreateRepairRecord(request.CreateTicketRepairRecordRequest{
			TicketID: ticket.ID, Conclusion: "设备恢复正常", RootCause: faultCode, RepairMethod: "更换故障模块",
		}, operator); err != nil {
			t.Fatalf("CreateRepairRecord(%s): %v", title, err)
		}
		resolvedAt := time.Now().AddDate(0, 0, -8)
		if err := repositories.TicketRepository.Updates(sqls.DB(), ticket.ID, map[string]any{
			"status": enums.TicketStatusResolved, "resolved_at": &resolvedAt,
		}); err != nil {
			t.Fatalf("set resolved status: %v", err)
		}
		return ticket
	}

	eligible := createResolvedTicket("普通模块故障", "CTRL-001", regularModule.ID)
	protected := createResolvedTicket("安全模块故障", "SAFE-001", safetyModule.ID)
	pendingMeeting := createResolvedTicket("视频协作待入会", "CTRL-VIDEO", regularModule.ID)
	notDue := createResolvedTicket("仍在客户确认期", "CTRL-NOT-DUE", regularModule.ID)
	notDueAt := time.Now().AddDate(0, 0, -6)
	if err := repositories.TicketRepository.Updates(sqls.DB(), notDue.ID, map[string]any{"resolved_at": &notDueAt}); err != nil {
		t.Fatalf("set not-due resolution time: %v", err)
	}
	unresolved := createResolvedTicket("尚未解决", "CTRL-OPEN", regularModule.ID)
	if err := repositories.TicketRepository.Updates(sqls.DB(), unresolved.ID, map[string]any{"status": enums.TicketStatusProcessing}); err != nil {
		t.Fatalf("set unresolved status: %v", err)
	}
	pendingConfirmation := createResolvedTicket("等待客户确认", "CTRL-CONFIRM", regularModule.ID)
	if err := repositories.TicketRepository.Updates(sqls.DB(), pendingConfirmation.ID, map[string]any{"status": enums.TicketStatusPendingCustomerConfirm}); err != nil {
		t.Fatalf("set pending-customer-confirm status: %v", err)
	}
	activeSupplier := createResolvedTicket("供应商仍在处理", "CTRL-SUPPLIER", regularModule.ID)
	now := time.Now()
	if err := sqls.DB().Create(&models.MeetingRoomJitsi{
		ID: "auto-close-waiting-meeting", TenantID: tenant.ID, TicketID: fmt.Sprint(pendingMeeting.ID),
		RoomName: "auto-close-waiting-room", Status: "waiting", CreatedBy: fmt.Sprint(operator.UserID),
		BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create pending meeting: %v", err)
	}
	if err := sqls.DB().Create(&models.TicketSupplierCollaboration{
		TenantID: tenant.ID, TicketID: activeSupplier.ID, ProductID: product.ID, ProductModuleID: regularModule.ID,
		PartnerCompanyID: 101, Status: "processing", InvitedAt: now, RecordStatus: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create active supplier collaboration: %v", err)
	}

	if closed := services.TicketAutoCloseService.CloseDueTickets(100); closed != 0 {
		t.Fatalf("disabled policy closed = %d, want 0", closed)
	}
	if err := repositories.TenantRepository.Updates(sqls.DB(), tenant.ID, map[string]any{
		"ticket_auto_close_enabled": true,
		"ticket_auto_close_days":    7,
	}); err != nil {
		t.Fatalf("enable auto close: %v", err)
	}

	if closed := services.TicketAutoCloseService.CloseDueTickets(100); closed != 2 {
		t.Fatalf("CloseDueTickets() closed = %d, want 2", closed)
	}
	if current := services.TicketService.Get(eligible.ID); current == nil || current.Status != enums.TicketStatusClosed {
		t.Fatalf("eligible ticket was not closed: %+v", current)
	}
	if candidate := repositories.KnowledgeCandidateRepository.FindOne(sqls.DB(), sqls.NewCnd().Eq("ticket_id", eligible.ID)); candidate == nil {
		t.Fatal("expected auto close to generate a pending knowledge candidate")
	}
	if current := services.TicketService.Get(protected.ID); current == nil || current.Status != enums.TicketStatusResolved {
		t.Fatalf("safety critical ticket should remain resolved: %+v", current)
	}
	if current := services.TicketService.Get(pendingMeeting.ID); current == nil || current.Status != enums.TicketStatusResolved {
		t.Fatalf("ticket with unfinished meeting should remain resolved: %+v", current)
	}
	if current := services.TicketService.Get(notDue.ID); current == nil || current.Status != enums.TicketStatusResolved {
		t.Fatalf("ticket inside confirmation window should remain resolved: %+v", current)
	}
	if current := services.TicketService.Get(unresolved.ID); current == nil || current.Status != enums.TicketStatusProcessing {
		t.Fatalf("unresolved ticket should remain processing: %+v", current)
	}
	if current := services.TicketService.Get(pendingConfirmation.ID); current == nil || current.Status != enums.TicketStatusClosed {
		t.Fatalf("due pending-confirmation ticket was not closed: %+v", current)
	}
	if current := services.TicketService.Get(activeSupplier.ID); current == nil || current.Status != enums.TicketStatusResolved {
		t.Fatalf("ticket with active supplier collaboration should remain resolved: %+v", current)
	}

	progress := make([]models.TicketProgress, 0)
	if err := sqls.DB().Where("ticket_id = ? AND event_type = ?", eligible.ID, enums.TicketProgressEventClosed).Order("id ASC").Find(&progress).Error; err != nil {
		t.Fatalf("find auto-close progress: %v", err)
	}
	if len(progress) != 1 || !strings.Contains(progress[0].Content, "客户确认期已结束，系统自动关闭") {
		t.Fatalf("auto-close audit progress = %+v", progress)
	}
	if err := services.TicketLifecycleService.Reopen(eligible.ID, "客户补充了新的故障信息", operator); err != nil {
		t.Fatalf("reopen auto-closed ticket: %v", err)
	}
	reopened := services.TicketService.Get(eligible.ID)
	if reopened == nil || reopened.Status != enums.TicketStatusReopened || reopened.ResolvedAt != nil || reopened.HandledAt != nil {
		t.Fatalf("auto-closed ticket did not preserve reopen workflow: %+v", reopened)
	}
	if count := repositories.TicketProgressRepository.Count(sqls.DB(), sqls.NewCnd().Eq("ticket_id", eligible.ID).Eq("event_type", enums.TicketProgressEventClosed)); count != 1 {
		t.Fatalf("closed audit history count = %d, want 1 after reopen", count)
	}
}
