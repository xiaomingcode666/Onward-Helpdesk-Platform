package services_test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"remotehelpdesk/internal/bootstrap"
	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func TestTicketLightweightStatuses(t *testing.T) {
	if !enums.IsValidTicketStatus(string(enums.TicketStatusPending)) {
		t.Fatalf("pending should be valid")
	}
	if !enums.IsValidTicketStatus(string(enums.TicketStatusInProgress)) {
		t.Fatalf("in_progress should be valid")
	}
	if !enums.IsValidTicketStatus(string(enums.TicketStatusDone)) {
		t.Fatalf("done should be valid")
	}
	for _, status := range []string{"new", "open", "pending_customer", "pending_internal", "cancelled"} {
		if enums.IsValidTicketStatus(status) {
			t.Fatalf("legacy status %s should be invalid", status)
		}
	}
}

func TestCanCreateTicketWithAssigneeRequiresAssignPermission(t *testing.T) {
	createOnly := &dto.AuthPrincipal{Permissions: []string{constants.PermissionTicketCreate.Code}}
	if !services.CanCreateTicketWithAssignee(createOnly, 0) {
		t.Fatal("creating an unassigned ticket should not require ticket.assign")
	}
	if services.CanCreateTicketWithAssignee(createOnly, 101) {
		t.Fatal("creating a ticket with an initial assignee should require ticket.assign")
	}
	withAssign := &dto.AuthPrincipal{Permissions: []string{constants.PermissionTicketCreate.Code, constants.PermissionTicketAssign.Code}}
	if !services.CanCreateTicketWithAssignee(withAssign, 101) {
		t.Fatal("ticket.assign should allow initial assignee selection during ticket creation")
	}
}

func TestTicketServiceCreateTicketWithDeviceAfterSalesContext(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "after-sales-create")
	tenant, product, productModel, device, serviceCode := createTicketAfterSalesFixture(t, "after-sales-create", enums.StatusOk)

	created, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:          "设备无法启动",
		Description:    "客户反馈设备无法启动",
		TenantID:       tenant.ID,
		ProductID:      product.ID,
		ProductModelID: productModel.ID,
		DeviceID:       device.ID,
		ServiceCodeID:  serviceCode.ID,
		ServiceRegion:  "CN",
		FaultCode:      "BOOT-001",
		SymptomSummary: "开机后无响应",
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if created.TenantID != tenant.ID || created.ProductID != product.ID || created.DeviceID != device.ID || created.ServiceCodeID != serviceCode.ID {
		t.Fatalf("unexpected after-sales context: %+v", created)
	}
}

func TestTicketServiceCreateTicketAcceptsBoundGeneralServiceCode(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "after-sales-bound-code")
	tenant, product, productModel, device, serviceCode := createTicketAfterSalesFixture(t, "after-sales-bound-code", enums.StatusOk)
	if err := sqls.DB().Model(serviceCode).Update("status", enums.ServiceCodeStatusBound).Error; err != nil {
		t.Fatalf("bind service code: %v", err)
	}

	created, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:          "已绑定设备请求人工",
		Description:    "正式客户通过已绑定设备转人工",
		TenantID:       tenant.ID,
		ProductID:      product.ID,
		ProductModelID: productModel.ID,
		DeviceID:       device.ID,
		ServiceCodeID:  serviceCode.ID,
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if created.ServiceCodeID != serviceCode.ID || created.DeviceID != device.ID {
		t.Fatalf("unexpected bound service-code context: %+v", created)
	}
}

func TestTicketServiceUpdateTicketPreservesAfterSalesContextWhenOmitted(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "after-sales-preserve")
	tenant, product, productModel, device, serviceCode := createTicketAfterSalesFixture(t, "after-sales-preserve", enums.StatusOk)

	created, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:          "设备无法启动",
		Description:    "客户反馈设备无法启动",
		TenantID:       tenant.ID,
		ProductID:      product.ID,
		ProductModelID: productModel.ID,
		DeviceID:       device.ID,
		ServiceCodeID:  serviceCode.ID,
		ServiceRegion:  "CN",
		FaultCode:      "BOOT-001",
		SymptomSummary: "开机后无响应",
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	if err := services.TicketService.UpdateTicket(request.UpdateTicketRequest{
		TicketID:    created.ID,
		Title:       "设备无法启动 - 已补充",
		Description: "客户补充现场照片",
	}, operator); err != nil {
		t.Fatalf("UpdateTicket() error = %v", err)
	}

	updated := services.TicketService.Get(created.ID)
	if updated.TenantID != tenant.ID || updated.ProductID != product.ID || updated.ProductModelID != productModel.ID || updated.DeviceID != device.ID || updated.ServiceCodeID != serviceCode.ID {
		t.Fatalf("after-sales context was not preserved: %+v", updated)
	}
	if updated.ServiceRegion != "CN" || updated.FaultCode != "BOOT-001" || updated.SymptomSummary != "开机后无响应" {
		t.Fatalf("after-sales text fields were not preserved: %+v", updated)
	}
}

func TestTicketServiceUpdateDoesNotBypassAssignmentWorkflow(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "update-assignee-operator")
	assigneeID := createTestUser(t, "update-assignee-current")
	otherAssigneeID := createTestUser(t, "update-assignee-other")

	created, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:             "需要保持负责人",
		Description:       "普通编辑不应清空或改派负责人",
		CurrentAssigneeID: assigneeID,
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if err := services.TicketService.UpdateTicket(request.UpdateTicketRequest{
		TicketID:    created.ID,
		Title:       "只更新标题",
		Description: "仍然不传负责人",
	}, operator); err != nil {
		t.Fatalf("UpdateTicket() content-only error = %v", err)
	}
	updated := services.TicketService.Get(created.ID)
	if updated.CurrentAssigneeID != assigneeID {
		t.Fatalf("content update changed assignee: got %d want %d", updated.CurrentAssigneeID, assigneeID)
	}

	err = services.TicketService.UpdateTicket(request.UpdateTicketRequest{
		TicketID:          created.ID,
		Title:             "尝试绕过派单",
		Description:       "不同负责人必须走 ticket.assign",
		CurrentAssigneeID: otherAssigneeID,
	}, operator)
	if err == nil {
		t.Fatal("UpdateTicket() should reject assignee changes")
	}
	unchanged := services.TicketService.Get(created.ID)
	if unchanged.CurrentAssigneeID != assigneeID {
		t.Fatalf("rejected update mutated assignee: got %d want %d", unchanged.CurrentAssigneeID, assigneeID)
	}
}

func TestTicketServiceRepairConclusionPreservesReportedFaultCode(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "repair-fault-code")
	tenant, product, productModel, device, serviceCode := createTicketAfterSalesFixture(t, "repair-fault-code", enums.StatusOk)
	ensureTestProductRepairEngineer(t, tenant.ID, product.ID, operator.UserID)

	ticket, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title: "PWR-001 保护停机", Description: "设备运行十分钟后保护停机", TenantID: tenant.ID,
		ProductID: product.ID, ProductModelID: productModel.ID, DeviceID: device.ID,
		ServiceCodeID: serviceCode.ID, FaultCode: "PWR-001", CurrentAssigneeID: operator.UserID,
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if err := services.TicketLifecycleService.Accept(ticket.ID, operator.UserID, operator); err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	if _, err := services.TicketService.CreateRepairRecord(request.CreateTicketRepairRecordRequest{
		TicketID: ticket.ID, FaultCode: "保护输入端子接触不良", RootCause: "端子氧化松动",
		Solution: "更换端子并复测", TestResult: "passed", MarkResolved: true,
		VisibleToCustomer: true,
	}, operator); err != nil {
		t.Fatalf("CreateRepairRecord() error = %v", err)
	}

	updated := services.TicketService.Get(ticket.ID)
	if updated == nil || updated.FaultCode != "PWR-001" {
		t.Fatalf("repair conclusion must not overwrite the reported fault code: %+v", updated)
	}
}

func TestTicketServiceRepairConclusionCorrectsInferredFaultCode(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "repair-correct-fault-code")
	tenant, product, productModel, device, serviceCode := createTicketAfterSalesFixture(t, "repair-correct-fault-code", enums.StatusOk)
	ensureTestProductRepairEngineer(t, tenant.ID, product.ID, operator.UserID)

	ticket, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title: "红灯常亮", Description: "设备复位后红灯常亮", TenantID: tenant.ID,
		ProductID: product.ID, ProductModelID: productModel.ID, DeviceID: device.ID,
		ServiceCodeID: serviceCode.ID, FaultCode: "PROD-E2E-20260728-1346", CurrentAssigneeID: operator.UserID,
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if err := services.TicketLifecycleService.Accept(ticket.ID, operator.UserID, operator); err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	if _, err := services.TicketService.CreateRepairRecord(request.CreateTicketRepairRecordRequest{
		TicketID: ticket.ID, FaultCode: "RHD-FLOW-ALPHA-7742", RootCause: "保护端子接触不良",
		Solution: "重新压接并复测", TestResult: "passed", MarkResolved: true,
		VisibleToCustomer: true,
	}, operator); err != nil {
		t.Fatalf("CreateRepairRecord() error = %v", err)
	}

	updated := services.TicketService.Get(ticket.ID)
	if updated == nil || updated.FaultCode != "RHD-FLOW-ALPHA-7742" {
		t.Fatalf("confirmed repair fault code must replace inferred value: %+v", updated)
	}
}

func TestTicketServiceResolvedRepairRequiresRootCauseAndSolution(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "repair-quality-gate")
	tenant, product, productModel, device, serviceCode := createTicketAfterSalesFixture(t, "repair-quality-gate", enums.StatusOk)
	ensureTestProductRepairEngineer(t, tenant.ID, product.ID, operator.UserID)

	ticket, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title: "PWR-001 维修质量门槛", Description: "设备断电后仍报错", TenantID: tenant.ID,
		ProductID: product.ID, ProductModelID: productModel.ID, DeviceID: device.ID,
		ServiceCodeID: serviceCode.ID, FaultCode: "PWR-001", CurrentAssigneeID: operator.UserID,
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if err := services.TicketLifecycleService.Accept(ticket.ID, operator.UserID, operator); err != nil {
		t.Fatalf("Accept() error = %v", err)
	}

	_, err = services.TicketService.CreateRepairRecord(request.CreateTicketRepairRecordRequest{
		TicketID: ticket.ID, Solution: "重新压接端子", TestResult: "passed", MarkResolved: true,
	}, operator)
	if err == nil || !strings.Contains(err.Error(), "root cause") {
		t.Fatalf("expected missing root cause to be rejected, got %v", err)
	}
	_, err = services.TicketService.CreateRepairRecord(request.CreateTicketRepairRecordRequest{
		TicketID: ticket.ID, RootCause: "保险座接触不良", TestResult: "passed", MarkResolved: true,
	}, operator)
	if err == nil || !strings.Contains(err.Error(), "solution") {
		t.Fatalf("expected missing solution to be rejected, got %v", err)
	}
	if records := repositories.TicketRepairRecordRepository.Find(sqls.DB(), sqls.NewCnd().Eq("ticket_id", ticket.ID)); len(records) != 0 {
		t.Fatalf("invalid repair conclusions must not persist records: %+v", records)
	}
}

func TestTicketServiceRepairConclusionCompletesSupplierSupportTicket(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "repair-supplier-support")
	tenant, product, productModel, device, serviceCode := createTicketAfterSalesFixture(t, "repair-supplier-support", enums.StatusOk)
	ensureTestProductRepairEngineer(t, tenant.ID, product.ID, operator.UserID)

	ticket, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title: "供应商协同后完成维修", Description: "压力控制模块供应商已确认处理方案", TenantID: tenant.ID,
		ProductID: product.ID, ProductModelID: productModel.ID, DeviceID: device.ID,
		ServiceCodeID: serviceCode.ID, FaultCode: "RHD-FLOW-ALPHA-7742", CurrentAssigneeID: operator.UserID,
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	now := time.Now()
	if err := repositories.TicketRepository.Updates(sqls.DB(), ticket.ID, map[string]any{
		"status":      enums.TicketStatusSupplierSupport,
		"accepted_at": &now,
	}); err != nil {
		t.Fatalf("seed supplier_support ticket: %v", err)
	}

	record, err := services.TicketService.CreateRepairRecord(request.CreateTicketRepairRecordRequest{
		TicketID: ticket.ID, FaultCode: "RHD-FLOW-ALPHA-7742",
		Conclusion: "供应商确认端子重新压接后恢复", RootCause: "压力控制模块保护输入端子接触不良",
		Solution: "更换同规格端子并按标准扭矩重新压接", TestResult: "passed",
		RemoteResolved: true, VisibleToCustomer: true, MarkResolved: true,
	}, operator)
	if err != nil {
		t.Fatalf("CreateRepairRecord() from supplier_support error = %v", err)
	}
	if record.ID <= 0 || record.TicketID != ticket.ID || !record.VisibleToCustomer {
		t.Fatalf("unexpected repair record: %+v", record)
	}
	updated := services.TicketService.Get(ticket.ID)
	if updated == nil || updated.Status != enums.TicketStatusResolved || updated.ResolvedAt == nil {
		t.Fatalf("supplier_support repair should resolve ticket for customer confirmation: %+v", updated)
	}
	candidate := repositories.KnowledgeCandidateRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenant.ID).
		Eq("ticket_id", ticket.ID).
		Eq("source_type", "ticket_repair").
		NotEq("status", enums.StatusDeleted))
	if candidate == nil || candidate.ProductID != product.ID || candidate.ProductModelID != productModel.ID {
		t.Fatalf("expected repair knowledge candidate with product context, got %+v", candidate)
	}
}

func TestTicketServiceRepairConclusionClosesActiveSupplierCollaboration(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "repair-active-supplier")
	tenant, product, productModel, device, serviceCode := createTicketAfterSalesFixture(t, "repair-active-supplier", enums.StatusOk)
	ensureTestProductRepairEngineer(t, tenant.ID, product.ID, operator.UserID)

	ticket, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title: "供应商协同未结束", Description: "供应商还在确认故障部件", TenantID: tenant.ID,
		ProductID: product.ID, ProductModelID: productModel.ID, DeviceID: device.ID,
		ServiceCodeID: serviceCode.ID, FaultCode: "SUP-001", CurrentAssigneeID: operator.UserID,
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	now := time.Now()
	if err := repositories.TicketRepository.Updates(sqls.DB(), ticket.ID, map[string]any{
		"status":      enums.TicketStatusSupplierSupport,
		"accepted_at": &now,
	}); err != nil {
		t.Fatalf("seed supplier_support ticket: %v", err)
	}
	collaboration := &models.TicketSupplierCollaboration{
		TenantID: tenant.ID, TicketID: ticket.ID, ProductID: product.ID, ProductModuleID: 1,
		PartnerCompanyID: 1, Status: services.SupplierCollaborationProcessing, InvitedAt: now,
		RecordStatus: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := sqls.DB().Create(collaboration).Error; err != nil {
		t.Fatalf("create active supplier collaboration: %v", err)
	}

	record, err := services.TicketService.CreateRepairRecord(request.CreateTicketRepairRecordRequest{
		TicketID: ticket.ID, Conclusion: "先记录现场现象，等待供应商结论",
		RootCause: "供应商尚未确认根因", Solution: "等待供应商协同结论", TestResult: "failed",
		VisibleToCustomer: true,
	}, operator)
	if err != nil {
		t.Fatalf("CreateRepairRecord() should still allow non-final notes: %v", err)
	}
	if record.ID <= 0 || services.TicketService.Get(ticket.ID).Status != enums.TicketStatusSupplierSupport {
		t.Fatalf("non-final repair note changed ticket unexpectedly: record=%+v ticket=%+v", record, services.TicketService.Get(ticket.ID))
	}

	finalRecord, err := services.TicketService.CreateRepairRecord(request.CreateTicketRepairRecordRequest{
		TicketID: ticket.ID, FaultCode: "SUP-001", Conclusion: "供应商确认老化模块更换后恢复",
		RootCause: "烟雾模块老化影响检测质量", Solution: "采购并更换同规格烟雾模块",
		TestResult: "passed", RemoteResolved: true, VisibleToCustomer: true, MarkResolved: true,
	}, operator)
	if err != nil {
		t.Fatalf("CreateRepairRecord() should close active supplier collaboration with final conclusion: %v", err)
	}
	if finalRecord.ID <= 0 || services.TicketService.Get(ticket.ID).Status != enums.TicketStatusResolved {
		t.Fatalf("final repair conclusion should resolve ticket: record=%+v ticket=%+v", finalRecord, services.TicketService.Get(ticket.ID))
	}
	var updatedCollaboration models.TicketSupplierCollaboration
	if err := sqls.DB().First(&updatedCollaboration, collaboration.ID).Error; err != nil {
		t.Fatalf("reload supplier collaboration: %v", err)
	}
	if updatedCollaboration.Status != services.SupplierCollaborationResolved || updatedCollaboration.ResolvedAt == nil {
		t.Fatalf("final repair conclusion should resolve supplier collaboration: %+v", updatedCollaboration)
	}
	if !strings.Contains(updatedCollaboration.Resolution, "供应商协作随维修结论关闭") {
		t.Fatalf("unexpected supplier collaboration resolution: %q", updatedCollaboration.Resolution)
	}
}

func TestTicketServiceRepairConclusionRequiresVideoMeetingEnded(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "repair-open-meeting")
	tenant, product, productModel, device, serviceCode := createTicketAfterSalesFixture(t, "repair-open-meeting", enums.StatusOk)
	ensureTestProductRepairEngineer(t, tenant.ID, product.ID, operator.UserID)

	ticket, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title: "视频协作未结束", Description: "客户和供应商仍在会议准备中", TenantID: tenant.ID,
		ProductID: product.ID, ProductModelID: productModel.ID, DeviceID: device.ID,
		ServiceCodeID: serviceCode.ID, FaultCode: "VID-001", CurrentAssigneeID: operator.UserID,
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	now := time.Now()
	if err := repositories.TicketRepository.Updates(sqls.DB(), ticket.ID, map[string]any{
		"status":      enums.TicketStatusVideoSupport,
		"accepted_at": &now,
	}); err != nil {
		t.Fatalf("seed video_support ticket: %v", err)
	}
	meeting := &models.MeetingRoomJitsi{
		ID: "repair-open-meeting", TenantID: tenant.ID, TicketID: fmt.Sprint(ticket.ID),
		RoomName: "repair-open-meeting-room", Status: "waiting", CreatedBy: fmt.Sprint(operator.UserID),
		BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
	if err := sqls.DB().Create(meeting).Error; err != nil {
		t.Fatalf("create open meeting: %v", err)
	}

	_, err = services.TicketService.CreateRepairRecord(request.CreateTicketRepairRecordRequest{
		TicketID: ticket.ID, FaultCode: "VID-001", RootCause: "远程协助仍在进行",
		Solution: "等待会议结束后提交最终结论", TestResult: "passed", MarkResolved: true,
	}, operator)
	if err == nil || !strings.Contains(err.Error(), "video meeting") {
		t.Fatalf("expected unfinished video meeting to block completion, got %v", err)
	}

	endedAt := now.Add(10 * time.Minute)
	if err := sqls.DB().Model(&models.MeetingRoomJitsi{}).Where("id = ?", meeting.ID).Updates(map[string]any{
		"status": "ended", "ended_at": &endedAt, "updated_at": endedAt,
	}).Error; err != nil {
		t.Fatalf("end meeting: %v", err)
	}
	if _, err := services.TicketService.CreateRepairRecord(request.CreateTicketRepairRecordRequest{
		TicketID: ticket.ID, FaultCode: "VID-001", Conclusion: "视频确认复位后设备恢复",
		RootCause: "参数漂移导致保护触发", Solution: "远程校准参数并复测通过",
		TestResult: "passed", RemoteResolved: true, VisibleToCustomer: true, MarkResolved: true,
	}, operator); err != nil {
		t.Fatalf("CreateRepairRecord() after meeting ended error = %v", err)
	}
}

func TestTicketServiceCreateTicketRejectsMismatchedTenantContext(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "after-sales-mismatch")
	tenant, _, productModel, device, serviceCode := createTicketAfterSalesFixture(t, "after-sales-mismatch", enums.StatusOk)
	otherTenant := &models.Tenant{Name: "other tenant", Status: enums.StatusOk}
	if err := sqls.DB().Create(otherTenant).Error; err != nil {
		t.Fatalf("create other tenant: %v", err)
	}
	_, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:          "租户不一致",
		Description:    "租户不一致",
		TenantID:       otherTenant.ID,
		ProductModelID: productModel.ID,
		DeviceID:       device.ID,
		ServiceCodeID:  serviceCode.ID,
	}, operator)
	if err == nil {
		t.Fatalf("expected mismatched tenant to be rejected, tenant=%d fixtureTenant=%d", otherTenant.ID, tenant.ID)
	}
}

func TestTicketServiceRepairHistoryFiltersByDeviceID(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "repair-history")
	tenant, product, productModel, device, serviceCode := createTicketAfterSalesFixture(t, "repair-history", enums.StatusOk)
	otherDevice := &models.Device{TenantID: tenant.ID, ProductID: product.ID, ProductModelID: productModel.ID, DeviceNo: "OTHER-DEVICE", Status: enums.StatusOk}
	if err := repositories.DeviceRepository.Create(sqls.DB(), otherDevice); err != nil {
		t.Fatalf("create other device: %v", err)
	}
	otherServiceCode := &models.ServiceCode{TenantID: tenant.ID, ServiceCode: "repair-history-other-code", DeviceID: otherDevice.ID, ProductID: product.ID, ProductModelID: productModel.ID, Status: enums.ServiceCodeStatusActive}
	if err := repositories.ServiceCodeRepository.Create(sqls.DB(), otherServiceCode); err != nil {
		t.Fatalf("create other service code: %v", err)
	}
	for _, deviceID := range []int64{device.ID, otherDevice.ID} {
		codeID := serviceCode.ID
		if deviceID == otherDevice.ID {
			codeID = otherServiceCode.ID
		}
		_, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
			Title: "维修记录", Description: "维修记录", TenantID: tenant.ID, ProductID: product.ID,
			ProductModelID: productModel.ID, DeviceID: deviceID, ServiceCodeID: codeID,
		}, operator)
		if err != nil {
			t.Fatalf("CreateTicket() error = %v", err)
		}
	}
	list, paging := services.TicketService.FindRepairHistoryPage(request.RepairHistoryFilter{DeviceID: device.ID, Limit: 20})
	if len(list) != 1 || list[0].DeviceID != device.ID || paging.Total != 1 {
		t.Fatalf("unexpected repair history: list=%+v paging=%+v", list, paging)
	}
}

func createTicketAfterSalesFixture(t *testing.T, prefix string, status enums.Status) (*models.Tenant, *models.Product, *models.ProductModel, *models.Device, *models.ServiceCode) {
	t.Helper()
	tenant := &models.Tenant{Name: prefix + " tenant", Status: status}
	if err := sqls.DB().Create(tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	product := &models.Product{TenantID: tenant.ID, Code: prefix + "-product", Name: prefix + " product", Status: status}
	if err := repositories.ProductRepository.Create(sqls.DB(), product); err != nil {
		t.Fatalf("create product: %v", err)
	}
	productModel := &models.ProductModel{TenantID: tenant.ID, ProductID: product.ID, ModelCode: prefix + "-model", Name: prefix + " model", Status: status}
	if err := repositories.ProductModelRepository.Create(sqls.DB(), productModel); err != nil {
		t.Fatalf("create model: %v", err)
	}
	device := &models.Device{TenantID: tenant.ID, ProductID: product.ID, ProductModelID: productModel.ID, DeviceNo: prefix + "-device", Status: status}
	if err := repositories.DeviceRepository.Create(sqls.DB(), device); err != nil {
		t.Fatalf("create device: %v", err)
	}
	serviceCode := &models.ServiceCode{TenantID: tenant.ID, ServiceCode: prefix + "-code", DeviceID: device.ID, ProductID: product.ID, ProductModelID: productModel.ID, Status: enums.ServiceCodeStatusActive}
	if err := repositories.ServiceCodeRepository.Create(sqls.DB(), serviceCode); err != nil {
		t.Fatalf("create service code: %v", err)
	}
	return tenant, product, productModel, device, serviceCode
}

func ensureTestProductRepairEngineer(t *testing.T, tenantID, productID, userID int64) int64 {
	t.Helper()
	if tenantID <= 0 || productID <= 0 || userID <= 0 {
		t.Fatalf("invalid product repair engineer fixture: tenant=%d product=%d user=%d", tenantID, productID, userID)
	}
	now := time.Now()
	team := services.ProductSupportOrganizationService.FindProductRepairTeam(sqls.DB(), tenantID, productID)
	if team == nil {
		team = &models.AgentTeam{
			TenantID:    tenantID,
			ProductID:   productID,
			TeamType:    services.AgentTeamTypeProductRepair,
			Name:        fmt.Sprintf("product-%d-repair", productID),
			Status:      enums.StatusOk,
			AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
		}
		if err := repositories.AgentTeamRepository.Create(sqls.DB(), team); err != nil {
			t.Fatalf("create product repair team: %v", err)
		}
	}
	operator := &dto.AuthPrincipal{UserID: userID, Username: "test-engineer", TenantID: tenantID}
	if _, err := services.AgentTeamMemberService.EnsureMemberDB(sqls.DB(), tenantID, team.ID, userID, 0, 1, true, operator); err != nil {
		t.Fatalf("ensure product repair member: %v", err)
	}
	status := &models.AgentWorkStatus{
		TenantID:        tenantID,
		UserID:          userID,
		Status:          services.AgentWorkStatusAvailable,
		ConfirmedAt:     now,
		StatusChangedAt: now,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := repositories.AgentWorkStatusRepository.Save(sqls.DB(), status); err != nil {
		t.Fatalf("ensure available work status: %v", err)
	}
	ensureTestEnterpriseWorkTime(t, tenantID, now)
	return team.ID
}

func ensureTestEnterpriseWorkTime(t *testing.T, tenantID int64, at time.Time) {
	t.Helper()
	local := at.In(time.FixedZone("CST", 8*60*60))
	weekday := int(local.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	minute := local.Hour()*60 + local.Minute()
	startMinute := minute - 1
	if startMinute < 0 {
		startMinute = 0
	}
	endMinute := minute + 30
	if endMinute > 24*60 {
		endMinute = 24 * 60
	}
	if endMinute <= startMinute {
		startMinute = 0
		endMinute = 24 * 60
	}
	if err := repositories.AgentTeamScheduleTemplateRepository.Upsert(sqls.DB(), &models.AgentTeamScheduleTemplate{
		TenantID:    tenantID,
		Workdays:    "[" + strconv.Itoa(weekday) + "]",
		StartMinute: startMinute,
		EndMinute:   endMinute,
		Timezone:    services.EngineerScheduleTimezone,
		Status:      enums.StatusOk,
	}); err != nil {
		t.Fatalf("ensure enterprise work-time template: %v", err)
	}
}

func TestTicketProgressModelExists(t *testing.T) {
	item := models.TicketProgress{
		TicketID: 12,
		Content:  "已电话联系客户确认问题仍存在",
		AuthorID: 7,
	}
	if item.TicketID != 12 || item.AuthorID != 7 || item.Content == "" {
		t.Fatalf("unexpected progress model: %+v", item)
	}
}

func TestTicketServiceCreateTicketSetsPendingStatusAndTicketNo(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "creator")
	customerID := createTestCustomer(t, "create-customer")
	tagID := createTestTag(t, "create-tag")

	created, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:             "create ticket",
		Description:       "create ticket description",
		CustomerID:        customerID,
		TagIDs:            []int64{tagID},
		CurrentAssigneeID: operator.UserID,
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if created.TicketNo == "" || !strings.HasPrefix(created.TicketNo, "TK") {
		t.Fatalf("expected generated ticket number, got %q", created.TicketNo)
	}
	if created.Status != enums.TicketStatusPending {
		t.Fatalf("expected pending status, got %s", created.Status)
	}
	if created.Source != enums.TicketSourceManual {
		t.Fatalf("expected manual source, got %s", created.Source)
	}

	progresses := services.TicketProgressService.Find(sqls.NewCnd().Eq("ticket_id", created.ID))
	if len(progresses) != 1 {
		t.Fatalf("expected initial progress, got %d", len(progresses))
	}
	if progresses[0].Content != "Created ticket" || progresses[0].AuthorID != operator.UserID {
		t.Fatalf("unexpected initial progress: %+v", progresses[0])
	}

	tags := services.TicketService.GetTags(created.ID)
	if len(tags) != 1 || tags[0].ID != tagID {
		t.Fatalf("expected ticket tag %d, got %+v", tagID, tags)
	}
}

func TestTicketServiceCreateTicketEnqueuesDurableTicketCreatedEvent(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "event-creator")

	created, err := services.TicketService.CreateTicket(createTestTicketRequest("event-ticket"), operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	var domainEvent models.DomainEvent
	if err := sqls.DB().Where("event_type = ? AND aggregate_id = ?", events.EventTicketCreated, fmt.Sprintf("%d", created.ID)).First(&domainEvent).Error; err != nil {
		t.Fatalf("load durable ticket created event: %v", err)
	}
	var payload events.TicketCreatedEvent
	if err := json.Unmarshal([]byte(domainEvent.Payload), &payload); err != nil {
		t.Fatalf("decode durable event: %v", err)
	}
	if payload.TicketID != created.ID || payload.OperatorID != operator.UserID {
		t.Fatalf("unexpected durable event payload: %+v", payload)
	}
	var outbox models.OutboxRecord
	if err := sqls.DB().Where("event_id = ?", domainEvent.ID).First(&outbox).Error; err != nil {
		t.Fatalf("load durable outbox record: %v", err)
	}
	if outbox.Status != models.OutboxStatusPending {
		t.Fatalf("outbox status = %q, want pending", outbox.Status)
	}
}

func TestTicketServiceLinkCustomerUpdatesTicketCustomerID(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "link-ticket-customer")
	customerID := createTestCustomer(t, "link-ticket-customer")
	ticket, err := services.TicketService.CreateTicket(createTestTicketRequest("link-ticket-customer"), operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if ticket.CustomerID != 0 {
		t.Fatalf("expected ticket without customer, got %d", ticket.CustomerID)
	}

	if err := services.TicketService.LinkCustomer(ticket.ID, customerID, operator); err != nil {
		t.Fatalf("LinkCustomer() error = %v", err)
	}

	updated := services.TicketService.Get(ticket.ID)
	if updated == nil {
		t.Fatalf("expected ticket")
	}
	if updated.CustomerID != customerID {
		t.Fatalf("expected customer id %d, got %d", customerID, updated.CustomerID)
	}
}

func TestTicketServiceLinkCustomerRejectsMissingCustomer(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "link-ticket-missing-customer")
	ticket, err := services.TicketService.CreateTicket(createTestTicketRequest("link-ticket-missing-customer"), operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	if err := services.TicketService.LinkCustomer(ticket.ID, 999999, operator); err == nil {
		t.Fatalf("expected LinkCustomer() to reject missing customer")
	}
}

func TestTicketServiceChangeStatusRejectsDirectResolved(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "status-operator")
	ticket, err := services.TicketService.CreateTicket(createTestTicketRequest("status-ticket"), operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	for _, status := range []enums.TicketStatus{enums.TicketStatusAccepted, enums.TicketStatusInProgress} {
		if err := services.TicketService.ChangeStatus(request.ChangeTicketStatusRequest{
			TicketID: ticket.ID,
			Status:   string(status),
		}, operator); err != nil {
			t.Fatalf("ChangeStatus() %s error = %v", status, err)
		}
	}
	inProgress := services.TicketService.Get(ticket.ID)
	if inProgress == nil {
		t.Fatalf("expected ticket to exist")
	}
	if inProgress.Status != enums.TicketStatusInProgress {
		t.Fatalf("expected in_progress status, got %s", inProgress.Status)
	}
	if inProgress.HandledAt != nil {
		t.Fatalf("expected handled_at to remain nil before resolved")
	}

	if err = services.TicketService.ChangeStatus(request.ChangeTicketStatusRequest{
		TicketID: ticket.ID,
		Status:   string(enums.TicketStatusResolved),
	}, operator); err != nil {
		if !strings.Contains(err.Error(), "repair record") {
			t.Fatalf("unexpected direct resolved error: %v", err)
		}
	} else {
		t.Fatalf("direct ChangeStatus resolved should be rejected")
	}
	unchanged := services.TicketService.Get(ticket.ID)
	if unchanged == nil || unchanged.Status != enums.TicketStatusInProgress || unchanged.HandledAt != nil || unchanged.ResolvedAt != nil {
		t.Fatalf("direct resolved changed ticket unexpectedly: %+v", unchanged)
	}
}

func TestTicketServiceAddProgressStoresContentAndAuthor(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "progress-operator")
	ticket, err := services.TicketService.CreateTicket(createTestTicketRequest("progress-ticket"), operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	progress, err := services.TicketService.AddProgress(request.CreateTicketProgressRequest{
		TicketID: ticket.ID,
		Content:  "客户已确认问题复现路径",
	}, operator)
	if err != nil {
		t.Fatalf("AddProgress() error = %v", err)
	}
	if progress.ID <= 0 {
		t.Fatalf("expected progress id")
	}
	if progress.Content != "客户已确认问题复现路径" || progress.AuthorID != operator.UserID {
		t.Fatalf("unexpected progress: %+v", progress)
	}
}

func TestTicketServiceCreateTicketPreservesRichDescription(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "rich-description-operator")

	created, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:       "rich description ticket",
		Description: "<p>客户反馈<strong>无法登录</strong></p><ul><li>验证码错误</li></ul>",
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}
	if created.Description != "<p>客户反馈<strong>无法登录</strong></p><ul><li>验证码错误</li></ul>" {
		t.Fatalf("expected rich description to be preserved, got %q", created.Description)
	}
}

func TestTicketServiceAddProgressPreservesRichContent(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "rich-progress-operator")
	ticket, err := services.TicketService.CreateTicket(createTestTicketRequest("rich-progress-ticket"), operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	progress, err := services.TicketService.AddProgress(request.CreateTicketProgressRequest{
		TicketID: ticket.ID,
		Content:  "<p>已回访客户，结论：<strong>继续观察</strong></p>",
	}, operator)
	if err != nil {
		t.Fatalf("AddProgress() error = %v", err)
	}
	if progress.Content != "<p>已回访客户，结论：<strong>继续观察</strong></p>" {
		t.Fatalf("expected rich progress content to be preserved, got %q", progress.Content)
	}
}

func TestTicketServiceAssignTicketRequiresTargetUser(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "assign-operator")
	ticket, err := services.TicketService.CreateTicket(createTestTicketRequest("assign-ticket"), operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	err = services.TicketService.AssignTicket(request.AssignTicketRequest{
		TicketID: ticket.ID,
		ToUserID: 0,
		Reason:   "invalid assignment",
	}, operator)
	if err == nil {
		t.Fatalf("expected AssignTicket() to reject empty target user")
	}
}

func TestTicketServiceAssignTicketRejectsDisabledUser(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "assign-disabled-operator")
	disabledUserID := createTestUserWithStatus(t, "assign-disabled-user", enums.StatusDisabled)
	ticket, err := services.TicketService.CreateTicket(createTestTicketRequest("assign-disabled-ticket"), operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	err = services.TicketService.AssignTicket(request.AssignTicketRequest{
		TicketID: ticket.ID,
		ToUserID: disabledUserID,
		Reason:   "disabled assignment",
	}, operator)
	if err == nil {
		t.Fatalf("expected AssignTicket() to reject disabled target user")
	}
}

func TestTicketServiceAssignTicketCreatesProgressEntry(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "assign-progress-operator")
	operator.Roles = []string{services.EnterpriseRoleServiceManager}
	firstAssignee := createTestOperator(t, "assign-progress-first")
	nextAssignee := createTestOperator(t, "assign-progress-next")
	ticket, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:             "assign progress ticket",
		Description:       "assign progress description",
		CurrentAssigneeID: firstAssignee.UserID,
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	if err := services.TicketService.AssignTicket(request.AssignTicketRequest{
		TicketID: ticket.ID,
		ToUserID: nextAssignee.UserID,
		Reason:   "需要二线继续跟进",
	}, operator); err != nil {
		t.Fatalf("AssignTicket() error = %v", err)
	}

	progresses := services.TicketProgressService.Find(sqls.NewCnd().Eq("ticket_id", ticket.ID).Asc("id"))
	if len(progresses) != 2 {
		t.Fatalf("expected create progress and assignment progress, got %d: %+v", len(progresses), progresses)
	}
	assignmentProgress := progresses[1]
	if assignmentProgress.AuthorID != operator.UserID {
		t.Fatalf("expected assignment progress author %d, got %d", operator.UserID, assignmentProgress.AuthorID)
	}
	if !strings.Contains(assignmentProgress.Content, "分配工单") {
		t.Fatalf("expected assignment progress content to mention assignment, got %q", assignmentProgress.Content)
	}
	if !strings.Contains(assignmentProgress.Content, firstAssignee.Username) || !strings.Contains(assignmentProgress.Content, nextAssignee.Username) {
		t.Fatalf("expected assignment progress to include assignee names, got %q", assignmentProgress.Content)
	}
	if !strings.Contains(assignmentProgress.Content, "需要二线继续跟进") {
		t.Fatalf("expected assignment reason in progress content, got %q", assignmentProgress.Content)
	}
}

func TestTicketServiceCreateTicketRejectsMismatchedCustomerConversation(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "mismatch-operator")
	customerID := createTestCustomer(t, "mismatch-customer")
	otherCustomerID := createTestCustomer(t, "mismatch-other-customer")
	conversationID := createTestConversation(t, otherCustomerID, "mismatch-conversation")

	_, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:          "mismatch ticket",
		Description:    "mismatch ticket description",
		CustomerID:     customerID,
		ConversationID: conversationID,
	}, operator)
	if err == nil {
		t.Fatalf("expected CreateTicket() to reject mismatched customer and conversation")
	}
}

func TestTicketServiceSummaryCountsStaleTickets(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "summary-operator")
	mine, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:             "mine stale ticket",
		Description:       "mine stale description",
		CurrentAssigneeID: operator.UserID,
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() mine error = %v", err)
	}
	if _, err := services.TicketService.CreateTicket(createTestTicketRequest("unassigned ticket"), operator); err != nil {
		t.Fatalf("CreateTicket() unassigned error = %v", err)
	}
	staleUpdatedAt := time.Now().Add(-36 * time.Hour)
	if err := repositories.TicketRepository.Updates(sqls.DB(), mine.ID, map[string]any{
		"updated_at": staleUpdatedAt,
	}); err != nil {
		t.Fatalf("update stale ticket error = %v", err)
	}

	summary := services.TicketService.GetSummary(operator, 24)
	if summary.All != 2 {
		t.Fatalf("expected all count 2, got %d", summary.All)
	}
	if summary.Pending != 2 {
		t.Fatalf("expected pending count 2, got %d", summary.Pending)
	}
	if summary.Mine != 1 {
		t.Fatalf("expected mine count 1, got %d", summary.Mine)
	}
	if summary.Unassigned != 1 {
		t.Fatalf("expected unassigned count 1, got %d", summary.Unassigned)
	}
	if summary.Stale != 1 {
		t.Fatalf("expected stale count 1, got %d", summary.Stale)
	}

	summary48 := services.TicketService.GetSummary(operator, 48)
	if summary48.Stale != 0 {
		t.Fatalf("expected stale count 0 for 48 hour threshold, got %d", summary48.Stale)
	}
	summaryInvalid := services.TicketService.GetSummary(operator, 1<<30)
	if summaryInvalid.Stale != 1 {
		t.Fatalf("expected invalid stale threshold to use 24 hours, got %d", summaryInvalid.Stale)
	}
}

func TestTicketServiceFindPageAggregateFiltersStaleTickets(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "stale-list-operator")
	staleOpen, err := services.TicketService.CreateTicket(createTestTicketRequest("stale open ticket"), operator)
	if err != nil {
		t.Fatalf("CreateTicket() stale open error = %v", err)
	}
	staleDone, err := services.TicketService.CreateTicket(createTestTicketRequest("stale done ticket"), operator)
	if err != nil {
		t.Fatalf("CreateTicket() stale done error = %v", err)
	}
	freshOpen, err := services.TicketService.CreateTicket(createTestTicketRequest("fresh open ticket"), operator)
	if err != nil {
		t.Fatalf("CreateTicket() fresh open error = %v", err)
	}

	// 关闭工单必须走 TicketLifecycleService.Close；此测试只关心 stale 过滤，直接经仓储置为 closed
	if err := repositories.TicketRepository.Updates(sqls.DB(), staleDone.ID, map[string]any{
		"status": enums.TicketStatusClosed,
	}); err != nil {
		t.Fatalf("set staleDone closed error = %v", err)
	}
	staleUpdatedAt := time.Now().Add(-48 * time.Hour)
	for _, ticketID := range []int64{staleOpen.ID, staleDone.ID} {
		if err := repositories.TicketRepository.Updates(sqls.DB(), ticketID, map[string]any{
			"updated_at": staleUpdatedAt,
		}); err != nil {
			t.Fatalf("update stale ticket %d error = %v", ticketID, err)
		}
	}

	aggregate, err := services.TicketService.FindPageAggregateByCnd(
		services.TicketService.ApplyStaleFilter(sqls.NewCnd(), 24).Page(1, 10),
		operator.UserID,
	)
	if err != nil {
		t.Fatalf("FindPageAggregateByCnd() error = %v", err)
	}
	if len(aggregate.List) != 1 {
		t.Fatalf("expected 1 stale non-done ticket, got %d: %+v", len(aggregate.List), aggregate.List)
	}
	if aggregate.List[0].ID != staleOpen.ID {
		t.Fatalf("expected stale open ticket %d, got %d", staleOpen.ID, aggregate.List[0].ID)
	}
	if aggregate.List[0].ID == freshOpen.ID || aggregate.List[0].ID == staleDone.ID {
		t.Fatalf("stale list included fresh or done ticket: %+v", aggregate.List[0])
	}
}

func TestTicketServiceFindPageAggregateEnrichesLookups(t *testing.T) {
	setupTicketTestDB(t)
	operator := createTestOperator(t, "aggregate-operator")
	assignee := createTestOperator(t, "aggregate-assignee")
	customerID := createTestCustomer(t, "aggregate-customer")
	tagID := createTestTag(t, "aggregate-tag")

	ticket, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:             "aggregate ticket",
		Description:       "aggregate description",
		CustomerID:        customerID,
		TagIDs:            []int64{tagID},
		CurrentAssigneeID: assignee.UserID,
	}, operator)
	if err != nil {
		t.Fatalf("CreateTicket() error = %v", err)
	}

	aggregate, err := services.TicketService.FindPageAggregateByCnd(sqls.NewCnd().Eq("id", ticket.ID).Page(1, 10), operator.UserID)
	if err != nil {
		t.Fatalf("FindPageAggregateByCnd() error = %v", err)
	}
	if len(aggregate.List) != 1 {
		t.Fatalf("expected 1 ticket, got %d", len(aggregate.List))
	}
	if len(aggregate.TagsByTicketID[ticket.ID]) != 1 || aggregate.TagsByTicketID[ticket.ID][0].ID != tagID {
		t.Fatalf("expected tag lookup to be populated")
	}
	if aggregate.Customers[customerID] == nil {
		t.Fatalf("expected customer lookup to be populated")
	}
	if aggregate.Users[assignee.UserID] == nil {
		t.Fatalf("expected assignee lookup to be populated")
	}
}

func TestTicketServiceTicketNoNextConcurrent(t *testing.T) {
	setupTicketTestDBWithMaxOpenConns(t, 8)

	const count = 50
	results := make(chan string, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup

	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticketNo, err := services.TicketNoSequenceService.Next(time.Now())
			if err != nil {
				errs <- err
			}
			results <- ticketNo
		}()
	}

	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("TicketNoService.Next() concurrent error = %v", err)
		}
	}

	seen := make(map[string]struct{}, count)
	for ticketNo := range results {
		if _, ok := seen[ticketNo]; ok {
			t.Fatalf("duplicate ticket number generated: %s", ticketNo)
		}
		seen[ticketNo] = struct{}{}
	}
	if len(seen) != count {
		t.Fatalf("expected %d unique ticket numbers, got %d", count, len(seen))
	}
}

func TestTicketServiceCreateTicketConcurrentAllocatesUniqueTicketNos(t *testing.T) {
	setupTicketTestDBWithMaxOpenConns(t, 8)
	operator := createTestOperator(t, "concurrent-create-operator")

	const count = 50
	results := make(chan string, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			ticket, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
				Title:       fmt.Sprintf("concurrent ticket %d", index),
				Description: fmt.Sprintf("concurrent ticket %d description", index),
			}, operator)
			if err != nil {
				errs <- err
				return
			}
			results <- ticket.TicketNo
		}(i)
	}

	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("CreateTicket() concurrent error = %v", err)
		}
	}

	seen := make(map[string]struct{}, count)
	for ticketNo := range results {
		if ticketNo == "" {
			t.Fatalf("expected non-empty ticket number")
		}
		if _, ok := seen[ticketNo]; ok {
			t.Fatalf("duplicate ticket number generated: %s", ticketNo)
		}
		seen[ticketNo] = struct{}{}
	}
	if len(seen) != count {
		t.Fatalf("expected %d unique ticket numbers, got %d", count, len(seen))
	}
}

func TestTicketServiceIdempotencyKeyIsScopedByTenant(t *testing.T) {
	setupTicketTestDB(t)
	firstTenant := models.Tenant{Name: "idempotency tenant one", Status: enums.StatusOk}
	secondTenant := models.Tenant{Name: "idempotency tenant two", Status: enums.StatusOk}
	if err := sqls.DB().Create(&firstTenant).Error; err != nil {
		t.Fatalf("create first tenant: %v", err)
	}
	if err := sqls.DB().Create(&secondTenant).Error; err != nil {
		t.Fatalf("create second tenant: %v", err)
	}
	firstOperator := createTestOperator(t, "tenant-idempotency-first")
	firstOperator.TenantID = firstTenant.ID
	secondOperator := createTestOperator(t, "tenant-idempotency-second")
	secondOperator.TenantID = secondTenant.ID
	const key = "external-crm:create:42"

	first, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		IdempotencyKey: key,
		Title:          "first tenant ticket",
		Description:    "first tenant ticket description",
	}, firstOperator)
	if err != nil {
		t.Fatalf("create first tenant ticket: %v", err)
	}
	firstRetry, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		IdempotencyKey: key,
		Title:          "retry must return original",
		Description:    "retry must return original description",
	}, firstOperator)
	if err != nil || firstRetry == nil || firstRetry.ID != first.ID {
		t.Fatalf("same-tenant retry must be idempotent: ticket=%+v err=%v", firstRetry, err)
	}

	second, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		IdempotencyKey: key,
		Title:          "second tenant ticket",
		Description:    "second tenant ticket description",
	}, secondOperator)
	if err != nil {
		t.Fatalf("same idempotency key in another tenant must succeed: %v", err)
	}
	if second == nil || second.ID == first.ID || second.TenantID != secondOperator.TenantID {
		t.Fatalf("cross-tenant idempotency keys collided: first=%+v second=%+v", first, second)
	}
	if count := services.TicketService.Count(sqls.NewCnd().Eq("idempotency_key", key)); count != 2 {
		t.Fatalf("idempotency key should exist once per tenant, got %d tickets", count)
	}
}

func setupTicketTestDB(t *testing.T) {
	setupTicketTestDBWithMaxOpenConns(t, 0)
}

func setupTicketTestDBWithMaxOpenConns(t *testing.T, maxOpenConns int) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "ticket-test.db")
	db, err := bootstrap.InitDB(config.DBConfig{
		Type:         "sqlite",
		DSN:          "file:" + dbPath + "?_busy_timeout=5000",
		MaxIdleConns: 1,
		MaxOpenConns: maxOpenConns,
	})
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	t.Cleanup(func() {
		waitTicketIntegrationEvents()
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := bootstrap.InitMigrations(); err != nil {
		t.Fatalf("InitMigrations() error = %v", err)
	}
}

func createTestTicketRequest(title string) request.CreateTicketRequest {
	return request.CreateTicketRequest{
		Title:       title,
		Description: title + " description",
	}
}

func createTestOperator(t *testing.T, prefix string) *dto.AuthPrincipal {
	t.Helper()
	userID := createTestUser(t, prefix)
	return &dto.AuthPrincipal{UserID: userID, Username: prefix}
}

func createTestUser(t *testing.T, prefix string) int64 {
	return createTestUserWithStatus(t, prefix, enums.StatusOk)
}

func createTestUserWithStatus(t *testing.T, prefix string, status enums.Status) int64 {
	t.Helper()
	now := time.Now()
	username := fmt.Sprintf("%s_%d", prefix, now.UnixNano())
	user := &models.User{
		Username: username,
		Nickname: prefix,
		Status:   status,
		AuditFields: models.AuditFields{
			CreatedAt:      now,
			CreateUserID:   1,
			CreateUserName: "admin",
			UpdatedAt:      now,
			UpdateUserID:   1,
			UpdateUserName: "admin",
		},
	}
	if err := repositories.UserRepository.Create(sqls.DB(), user); err != nil {
		t.Fatalf("create user error = %v", err)
	}
	return user.ID
}

func createTestConversation(t *testing.T, customerID int64, prefix string) int64 {
	t.Helper()

	now := time.Now()
	item := &models.Conversation{
		CustomerID:    customerID,
		CustomerName:  prefix,
		Status:        enums.IMConversationStatusActive,
		ServiceMode:   enums.IMConversationServiceModeAIOnly,
		LastMessageAt: now,
		LastActiveAt:  now,
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := repositories.ConversationRepository.Create(sqls.DB(), item); err != nil {
		t.Fatalf("create conversation error = %v", err)
	}
	return item.ID
}

func createTestCustomer(t *testing.T, prefix string) int64 {
	t.Helper()

	now := time.Now()
	item := &models.Customer{
		Name:   fmt.Sprintf("%s-%d", prefix, now.UnixNano()),
		Status: enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt:      now,
			CreateUserID:   1,
			CreateUserName: "admin",
			UpdatedAt:      now,
			UpdateUserID:   1,
			UpdateUserName: "admin",
		},
	}
	if err := repositories.CustomerRepository.Create(sqls.DB(), item); err != nil {
		t.Fatalf("create customer error = %v", err)
	}
	return item.ID
}

func createTestTag(t *testing.T, prefix string) int64 {
	t.Helper()

	now := time.Now()
	item := &models.Tag{
		Name:   fmt.Sprintf("%s-%d", prefix, now.UnixNano()),
		Status: enums.StatusOk,
		SortNo: 1,
		AuditFields: models.AuditFields{
			CreatedAt:      now,
			CreateUserID:   1,
			CreateUserName: "admin",
			UpdatedAt:      now,
			UpdateUserID:   1,
			UpdateUserName: "admin",
		},
	}
	if err := repositories.TagRepository.Create(sqls.DB(), item); err != nil {
		t.Fatalf("create tag error = %v", err)
	}
	return item.ID
}
