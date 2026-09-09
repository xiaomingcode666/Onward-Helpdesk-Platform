package services

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestProductOnlyTicketRepairFeedbackConfirmAndReopen(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Tenant{}, &models.User{}, &models.Ticket{}, &models.TicketRepairRecord{}, &models.TicketProgress{},
		&models.TicketFeedback{}, &models.DeviceServiceRecord{}, &models.KnowledgeCandidate{},
		&models.TicketQualityClue{}, &models.ProductFaultStatsDaily{}, &models.FaultStatsEventInbox{},
		&models.Channel{},
		&models.MeetingRoomJitsi{}, &models.MeetingParticipant{},
		&models.TicketSupplierCollaboration{}, &models.TicketSupplierCollaborationParticipant{}, &models.PartnerAuthorizationScope{},
		&models.Conversation{}, &models.ConversationAssignment{}, &models.ConversationParticipant{},
		&models.ConversationReadState{}, &models.ConversationEventLog{}, &models.Message{},
		&models.AgentTeamScheduleTemplate{}, &models.AgentWorkStatus{},
		&models.AuditLog{}, &models.DomainEvent{}, &models.OutboxRecord{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		eventbus.WaitAsync[events.KnowledgeCandidateCreatedEvent]()
		eventbus.WaitAsync[events.TicketClosedEvent]()
		eventbus.WaitAsync[events.TicketReopenedEvent]()
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	now := time.Now()
	if err := db.Create(&models.Tenant{ID: 1, Name: "ticket action tenant", Status: enums.StatusOk, AIEnabled: boolRef(true)}).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	if err := db.Create(&models.User{ID: 101, Username: "engineer", Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create engineer: %v", err)
	}
	if err := db.Create(&models.AgentTeamScheduleTemplate{
		TenantID: 1, Workdays: "[" + strconv.Itoa(scheduleWeekday(now)) + "]",
		StartMinute: max(0, scheduleMinute(now)-1), EndMinute: min(24*60, scheduleMinute(now)+30),
		Timezone: EngineerScheduleTimezone, Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create enterprise work-time template: %v", err)
	}
	if err := db.Create(&models.AgentWorkStatus{
		TenantID: 1, UserID: 101, Status: AgentWorkStatusAvailable, ConfirmedAt: now, StatusChangedAt: now,
	}).Error; err != nil {
		t.Fatalf("create engineer work status: %v", err)
	}
	conversation := &models.Conversation{
		TenantID: 1, ProductID: 11, Status: enums.IMConversationStatusActive,
		LastMessageAt: now, LastActiveAt: now,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	ticket := &models.Ticket{
		TenantID: 1, ProductID: 11, DeviceID: 0, TicketNo: "PRODUCT-ONLY-1", Title: "产品配置咨询",
		ConversationID: conversation.ID, Status: enums.TicketStatusAccepted,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	engineer := &dto.AuthPrincipal{TenantID: 1, UserID: 101, Username: "engineer"}
	if _, err := TicketService.CreateRepairRecord(request.CreateTicketRepairRecordRequest{
		TicketID: ticket.ID, FaultCode: "configuration", Conclusion: "复测仍失败",
		Solution: "继续排查", TestResult: "failed", VisibleToCustomer: true, MarkResolved: true,
	}, engineer); err == nil {
		t.Fatal("failed verification must not move a ticket to customer confirmation")
	}
	var rejectedRepairCount int64
	if err := db.Model(&models.TicketRepairRecord{}).Where("ticket_id = ?", ticket.ID).Count(&rejectedRepairCount).Error; err != nil || rejectedRepairCount != 0 {
		t.Fatalf("rejected repair must not persist: count=%d err=%v", rejectedRepairCount, err)
	}
	repairRequest := request.CreateTicketRepairRecordRequest{
		TicketID: ticket.ID, FaultCode: "configuration", Conclusion: "参数已修正并验证通过",
		Solution: "恢复推荐参数并重新启动", RootCause: "参数配置不正确", ServiceMethod: "remote",
		TestResult: "passed", RemoteResolved: true, VisibleToCustomer: true, MarkResolved: true,
	}
	repair, err := TicketService.CreateRepairRecord(repairRequest, engineer)
	if err != nil {
		t.Fatalf("create product-only repair: %v", err)
	}
	retriedRepair, err := TicketService.CreateRepairRecord(repairRequest, engineer)
	if err != nil || retriedRepair == nil || retriedRepair.ID != repair.ID {
		t.Fatalf("identical repair conclusion retry must be idempotent: repair=%+v err=%v", retriedRepair, err)
	}
	if _, err := TicketService.CreateRepairRecord(request.CreateTicketRepairRecordRequest{
		TicketID: ticket.ID, FaultCode: "configuration", Conclusion: "重复请求却改写了结论",
		Solution: "另一套处理方法", RootCause: "不同根因", ServiceMethod: "remote",
		TestResult: "passed", RemoteResolved: true, VisibleToCustomer: true, MarkResolved: true,
	}, engineer); err == nil {
		t.Fatal("a different repair conclusion must not overwrite an already submitted resolution")
	}
	var repairCount int64
	if err := db.Model(&models.TicketRepairRecord{}).Where("ticket_id = ?", ticket.ID).Count(&repairCount).Error; err != nil || repairCount != 1 {
		t.Fatalf("repair record count=%d err=%v, want one immutable conclusion", repairCount, err)
	}
	var repairEventCount int64
	if err := db.Model(&models.Message{}).
		Where("conversation_id = ? AND sender_type = ? AND payload LIKE ?", conversation.ID, enums.IMSenderTypeSystem, "%ticket_resolved%").
		Count(&repairEventCount).Error; err != nil || repairEventCount != 1 {
		t.Fatalf("repair conclusion conversation event count=%d err=%v, want one", repairEventCount, err)
	}
	if repair.DeviceID != 0 {
		t.Fatalf("product-only repair device = %d, want 0", repair.DeviceID)
	}
	resolved := repositories.TicketRepository.Get(db, ticket.ID)
	if resolved == nil || resolved.Status != enums.TicketStatusResolved || resolved.ResolvedAt == nil || resolved.FaultCode != "configuration" {
		t.Fatalf("ticket was not moved to customer confirmation: %+v", resolved)
	}
	candidateAfterRepair := repositories.KnowledgeCandidateRepository.FindOne(db, sqls.NewCnd().Eq("ticket_id", ticket.ID).Eq("source_type", "ticket_repair"))
	if candidateAfterRepair == nil || candidateAfterRepair.Title != "configuration 参数配置不正确" {
		t.Fatalf("repair conclusion did not create an immediate knowledge candidate: %+v", candidateAfterRepair)
	}
	if !strings.Contains(candidateAfterRepair.SolutionSummary, "维修结论：参数已修正并验证通过") {
		t.Fatalf("repair conclusion must preserve repair details in knowledge summary: %+v", candidateAfterRepair)
	}
	var deviceHistoryCount int64
	if err := db.Model(&models.DeviceServiceRecord{}).Where("ticket_id = ?", ticket.ID).Count(&deviceHistoryCount).Error; err != nil {
		t.Fatalf("count device history: %v", err)
	}
	if deviceHistoryCount != 0 {
		t.Fatalf("product-only repair created %d device histories", deviceHistoryCount)
	}

	customer := &dto.AuthPrincipal{TenantID: 1, Username: "customer", DomainType: models.DomainTypeCustomer, SubjectType: models.SubjectTypeTempVisitor}
	feedback, err := CustomerTicketActionService.SubmitFeedback(resolved, 0, request.SubmitTicketFeedbackRequest{
		TicketID: ticket.ID, Rating: 5, Tags: []string{"响应及时"}, Comment: "问题已解决",
	}, customer)
	if err != nil || feedback == nil || feedback.Rating != 5 {
		t.Fatalf("submit feedback: feedback=%+v err=%v", feedback, err)
	}
	if duplicate, duplicateErr := CustomerTicketActionService.SubmitFeedback(resolved, 0, request.SubmitTicketFeedbackRequest{
		TicketID: ticket.ID, Rating: 5, Tags: []string{"响应及时"}, Comment: "问题已解决",
	}, customer); duplicateErr != nil || duplicate == nil || duplicate.ID != feedback.ID {
		t.Fatalf("identical feedback retry must be idempotent: feedback=%+v err=%v", duplicate, duplicateErr)
	}
	var feedbackAuditCount int64
	if err := db.Model(&models.AuditLog{}).
		Where("tenant_id = ? AND resource_id = ? AND action = ?", ticket.TenantID, strconv.FormatInt(ticket.ID, 10), "ticket.feedback_submitted").
		Count(&feedbackAuditCount).Error; err != nil || feedbackAuditCount != 1 {
		t.Fatalf("feedback audit count=%d err=%v, want one", feedbackAuditCount, err)
	}
	var feedbackMessageCount int64
	if err := db.Model(&models.Message{}).
		Where("conversation_id = ? AND sender_type = ? AND payload LIKE ?", conversation.ID, enums.IMSenderTypeSystem, "%ticket_feedback_submitted%").
		Count(&feedbackMessageCount).Error; err != nil || feedbackMessageCount != 1 {
		t.Fatalf("feedback conversation event count=%d err=%v", feedbackMessageCount, err)
	}
	candidateAfterFeedback := repositories.KnowledgeCandidateRepository.Get(db, candidateAfterRepair.ID)
	if candidateAfterFeedback == nil || candidateAfterFeedback.ID != candidateAfterRepair.ID {
		t.Fatalf("feedback must rescore the immediate repair candidate instead of losing it: %+v", candidateAfterFeedback)
	}
	if _, err := CustomerTicketActionService.SubmitFeedback(resolved, 0, request.SubmitTicketFeedbackRequest{
		TicketID: ticket.ID, Rating: 2, Comment: "重复改写本轮评价",
	}, customer); err == nil {
		t.Fatal("feedback from the same resolution must not be overwritten")
	}
	meetingStartedAt := time.Now().Add(-2 * time.Minute)
	meeting := &models.MeetingRoomJitsi{
		ID:        "meeting-close-with-ticket",
		TenantID:  ticket.TenantID,
		TicketID:  strconv.FormatInt(ticket.ID, 10),
		RoomName:  "ticket-close-room",
		Status:    "active",
		StartedAt: &meetingStartedAt,
		BaseModel: models.BaseModel{CreatedAt: meetingStartedAt, UpdatedAt: meetingStartedAt},
	}
	if err := db.Create(meeting).Error; err != nil {
		t.Fatalf("create active meeting: %v", err)
	}
	if err := CustomerTicketActionService.ConfirmResolved(resolved, customer); err != nil {
		t.Fatalf("confirm resolved: %v", err)
	}
	drainTicketLifecycleOutbox(t)
	closed := repositories.TicketRepository.Get(db, ticket.ID)
	if closed == nil || closed.Status != enums.TicketStatusClosed || closed.HandledAt == nil {
		t.Fatalf("ticket was not closed after customer confirmation: %+v", closed)
	}
	var endedMeeting models.MeetingRoomJitsi
	if err := db.First(&endedMeeting, "id = ?", meeting.ID).Error; err != nil || endedMeeting.Status != "ended" || endedMeeting.EndedAt == nil {
		t.Fatalf("ticket close did not end meeting: meeting=%+v err=%v", endedMeeting, err)
	}
	var meetingTimelineCount, meetingEventCount int64
	if err := db.Model(&models.TicketProgress{}).
		Where("ticket_id = ? AND event_type = ?", ticket.ID, enums.TicketProgressEventMeetingEnded).
		Count(&meetingTimelineCount).Error; err != nil {
		t.Fatalf("count meeting timeline: %v", err)
	}
	if err := db.Model(&models.DomainEvent{}).
		Where("event_type = ? AND aggregate_id = ?", events.EventMeetingEnded, meeting.ID).
		Count(&meetingEventCount).Error; err != nil {
		t.Fatalf("count meeting ended events: %v", err)
	}
	if meetingTimelineCount != 1 || meetingEventCount != 1 {
		t.Fatalf("meeting close side effects timeline=%d event=%d, want 1/1", meetingTimelineCount, meetingEventCount)
	}
	if err := CustomerTicketActionService.ConfirmResolved(closed, customer); err != nil {
		t.Fatalf("duplicate customer confirmation must be idempotent: %v", err)
	}
	var customerConfirmAuditCount int64
	if err := db.Model(&models.AuditLog{}).
		Where("tenant_id = ? AND resource_id = ? AND action = ?", ticket.TenantID, strconv.FormatInt(ticket.ID, 10), "ticket.customer_confirmed").
		Count(&customerConfirmAuditCount).Error; err != nil || customerConfirmAuditCount != 1 {
		t.Fatalf("customer confirmation audit count=%d err=%v, want one", customerConfirmAuditCount, err)
	}
	var closeProgressCount int64
	if err := db.Model(&models.TicketProgress{}).
		Where("ticket_id = ? AND event_type = ?", ticket.ID, enums.TicketProgressEventClosed).
		Count(&closeProgressCount).Error; err != nil || closeProgressCount != 1 {
		t.Fatalf("close progress count=%d err=%v, want one", closeProgressCount, err)
	}
	candidate := repositories.KnowledgeCandidateRepository.FindOne(db, sqls.NewCnd().Eq("ticket_id", ticket.ID).Eq("source_type", "ticket_repair"))
	if candidate == nil || candidate.ID != candidateAfterRepair.ID || candidate.Title != "configuration 参数配置不正确" || candidate.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusLowValue) {
		t.Fatalf("ticket close did not create a correctly gated knowledge candidate: %+v", candidate)
	}
	if !strings.Contains(candidate.SolutionSummary, "维修结论：参数已修正并验证通过") {
		t.Fatalf("ticket close must preserve repair conclusion in knowledge summary: %+v", candidate)
	}
	manualResult, err := TicketKnowledgeCandidateService.Create(ticket.TenantID, ticket.ID, request.CreateTicketKnowledgeCandidateRequest{}, engineer)
	if err != nil || manualResult == nil || manualResult.Created || manualResult.Candidate == nil || manualResult.Candidate.ID != candidate.ID {
		t.Fatalf("manual generation must reuse the automatic repair candidate: result=%+v err=%v", manualResult, err)
	}
	var activeCandidateCount int64
	if err := db.Model(&models.KnowledgeCandidate{}).
		Where("tenant_id = ? AND ticket_id = ? AND status <> ?", ticket.TenantID, ticket.ID, enums.StatusDeleted).
		Count(&activeCandidateCount).Error; err != nil || activeCandidateCount != 1 {
		t.Fatalf("ticket has %d active knowledge candidates, want 1: %v", activeCandidateCount, err)
	}
	if err := CustomerTicketActionService.Reopen(closed, "  ", customer); err == nil {
		t.Fatal("reopening without a recurrence description must be rejected")
	}
	stillClosed := repositories.TicketRepository.Get(db, ticket.ID)
	if stillClosed == nil || stillClosed.Status != enums.TicketStatusClosed {
		t.Fatalf("rejected reopen changed ticket state: %+v", stillClosed)
	}
	if err := CustomerTicketActionService.Reopen(closed, "复测后问题再次出现", customer); err != nil {
		t.Fatalf("reopen ticket: %v", err)
	}
	drainTicketLifecycleOutbox(t)
	reopened := repositories.TicketRepository.Get(db, ticket.ID)
	if reopened == nil || reopened.Status != enums.TicketStatusReopened || reopened.ResolvedAt != nil || reopened.HandledAt != nil {
		t.Fatalf("ticket was not reopened: %+v", reopened)
	}
	if err := CustomerTicketActionService.Reopen(reopened, "复测后问题再次出现", customer); err != nil {
		t.Fatalf("duplicate reopen retry must be idempotent: %v", err)
	}
	var reopenProgressCount int64
	if err := db.Model(&models.TicketProgress{}).
		Where("ticket_id = ? AND event_type = ?", ticket.ID, enums.TicketProgressEventReopened).
		Count(&reopenProgressCount).Error; err != nil || reopenProgressCount != 1 {
		t.Fatalf("reopen progress count=%d err=%v, want one", reopenProgressCount, err)
	}
	reopenMessage := repositories.MessageRepository.FindLastValidByConversationIDAndSenderType(db, conversation.ID, enums.IMSenderTypeCustomer)
	if reopenMessage == nil || !strings.Contains(reopenMessage.Content, "复测后问题再次出现") || !strings.Contains(reopenMessage.Payload, "ticket_reopen") {
		t.Fatalf("reopen reason was not synchronized to the customer conversation: %+v", reopenMessage)
	}
	reopenedConversation := repositories.ConversationRepository.Get(db, conversation.ID)
	if reopenedConversation == nil || reopenedConversation.Status != enums.IMConversationStatusPending || reopenedConversation.LastMessageID != reopenMessage.ID || reopenedConversation.AgentUnreadCount != 1 {
		t.Fatalf("conversation was not queued with the reopen message: %+v", reopenedConversation)
	}
	invalidatedCandidate := repositories.KnowledgeCandidateRepository.Get(db, candidate.ID)
	if invalidatedCandidate == nil || invalidatedCandidate.ReviewStatus != "rejected" {
		t.Fatalf("reopening must invalidate the pending knowledge candidate: %+v", invalidatedCandidate)
	}
	staleRepairs := repositories.TicketRepairRepository.FindByTicketID(db, ticket.ID)
	TicketLifecycleService.generateCloseSideEffects(closed, staleRepairs, "过期关闭任务", customer)
	staleCandidate := repositories.KnowledgeCandidateRepository.Get(db, candidate.ID)
	if staleCandidate == nil || staleCandidate.ReviewStatus != "rejected" {
		t.Fatalf("a delayed close side effect must not revive knowledge after reopen: %+v", staleCandidate)
	}
	if _, err := KnowledgeCandidateReviewService.ApproveCandidate(
		ticket.TenantID,
		candidate.ID,
		dto.EnterpriseKnowledgeCandidateApproveRequest{Publish: true},
		engineer,
	); err == nil {
		t.Fatal("an invalidated knowledge candidate must not be reviewable")
	}
	if err := TicketLifecycleService.Accept(ticket.ID, engineer.UserID, engineer); err != nil {
		t.Fatalf("accept reopened ticket: %v", err)
	}
	if _, err := TicketService.CreateRepairRecord(request.CreateTicketRepairRecordRequest{
		TicketID: ticket.ID, FaultCode: "configuration", Conclusion: "二次修复并验证通过",
		Solution: "更新配置并完成二次复测", RootCause: "配置再次漂移", ServiceMethod: "remote",
		TestResult: "passed", RemoteResolved: true, VisibleToCustomer: true, MarkResolved: true,
	}, engineer); err != nil {
		t.Fatalf("create second repair after reopen: %v", err)
	}
	resolvedAgain := repositories.TicketRepository.Get(db, ticket.ID)
	canConfirm, _, canRate := customerTicketActionFlags(resolvedAgain, feedback)
	if !canConfirm || !canRate {
		t.Fatalf("a new resolution must open a new confirmation and rating window: confirm=%v rate=%v", canConfirm, canRate)
	}
	secondFeedbackRequest := request.SubmitTicketFeedbackRequest{
		TicketID: ticket.ID, Rating: 4, Comment: "二次处理后已解决",
	}
	secondFeedback, err := CustomerTicketActionService.SubmitFeedback(resolvedAgain, 0, secondFeedbackRequest, customer)
	if err != nil {
		t.Fatalf("submit feedback for second resolution: %v", err)
	}
	if secondFeedback == nil || secondFeedback.ID == feedback.ID {
		t.Fatalf("second resolution feedback must be a new immutable record: first=%+v second=%+v", feedback, secondFeedback)
	}
	secondFeedbackRetry, err := CustomerTicketActionService.SubmitFeedback(resolvedAgain, 0, secondFeedbackRequest, customer)
	if err != nil || secondFeedbackRetry == nil || secondFeedbackRetry.ID != secondFeedback.ID {
		t.Fatalf("identical second resolution feedback retry must be idempotent: feedback=%+v err=%v", secondFeedbackRetry, err)
	}
	var feedbackHistory []models.TicketFeedback
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id ASC").Find(&feedbackHistory).Error; err != nil {
		t.Fatalf("list feedback history: %v", err)
	}
	if len(feedbackHistory) != 2 || feedbackHistory[0].Rating != 5 || feedbackHistory[0].Comment != "问题已解决" || feedbackHistory[1].Rating != 4 {
		t.Fatalf("feedback history was not preserved across resolutions: %+v", feedbackHistory)
	}
	if err := CustomerTicketActionService.ConfirmResolved(resolvedAgain, customer); err != nil {
		t.Fatalf("confirm second resolution: %v", err)
	}
	drainTicketLifecycleOutbox(t)
	refreshedCandidate := repositories.KnowledgeCandidateRepository.Get(db, candidate.ID)
	if refreshedCandidate == nil ||
		refreshedCandidate.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusLowValue) ||
		refreshedCandidate.Title != "configuration 配置再次漂移" ||
		refreshedCandidate.RootCauseSummary != "配置再次漂移" ||
		!strings.Contains(refreshedCandidate.SolutionSummary, "二次复测") ||
		!strings.Contains(refreshedCandidate.SolutionSummary, "维修结论：二次修复并验证通过") ||
		strings.Contains(refreshedCandidate.SolutionSummary, "恢复推荐参数") ||
		strings.Contains(refreshedCandidate.SolutionSummary, "参数已修正并验证通过") {
		t.Fatalf("second close must refresh the knowledge candidate from the latest repair round: %+v", refreshedCandidate)
	}
	var faultEvents []models.FaultStatsEventInbox
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id ASC").Find(&faultEvents).Error; err != nil {
		t.Fatalf("list ticket lifecycle fault events: %v", err)
	}
	closedEventIDs := make(map[string]bool)
	reopenedEventIDs := make(map[string]bool)
	for i := range faultEvents {
		switch faultEvents[i].EventType {
		case events.FaultStatsEventTicketClosed:
			closedEventIDs[faultEvents[i].EventID] = true
		case events.FaultStatsEventTicketReopened:
			reopenedEventIDs[faultEvents[i].EventID] = true
		}
	}
	if len(closedEventIDs) != 2 || len(reopenedEventIDs) != 1 {
		t.Fatalf("lifecycle event identities closed=%v reopened=%v, want two close rounds and one reopen", closedEventIDs, reopenedEventIDs)
	}
	var projectedTicketCount int64
	if err := db.Model(&models.ProductFaultStatsDaily{}).
		Where("tenant_id = ? AND product_id = ?", ticket.TenantID, ticket.ProductID).
		Select("COALESCE(SUM(ticket_count), 0)").Scan(&projectedTicketCount).Error; err != nil || projectedTicketCount != 1 {
		t.Fatalf("projected ticket count=%d err=%v, want one currently closed ticket", projectedTicketCount, err)
	}
}

func TestTicketFeedbackPublishesServiceEventForClosedConversation(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Ticket{}, &models.TicketFeedback{}, &models.Conversation{}, &models.ConversationReadState{},
		&models.ConversationEventLog{}, &models.Message{}, &models.DomainEvent{}, &models.OutboxRecord{},
		&models.KnowledgeCandidate{}, &models.KnowledgeDocument{}, &models.KnowledgeFAQ{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	now := time.Now()
	conversation := &models.Conversation{
		TenantID: 1, ProductID: 11, Status: enums.IMConversationStatusClosed, ClosedAt: &now,
		LastMessageAt: now, LastActiveAt: now,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatalf("create closed conversation: %v", err)
	}
	ticket := &models.Ticket{
		TenantID: 1, ProductID: 11, TicketNo: "CLOSED-FEEDBACK-1", Title: "已关闭会话评价",
		ConversationID: conversation.ID, Status: enums.TicketStatusClosed, ResolvedAt: &now,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	customer := &dto.AuthPrincipal{TenantID: 1, Username: "customer", DomainType: models.DomainTypeCustomer, SubjectType: models.SubjectTypeCustomerUser}
	feedback, err := CustomerTicketActionService.SubmitFeedback(ticket, 0, request.SubmitTicketFeedbackRequest{
		TicketID: ticket.ID, Rating: 5, Comment: "关闭后补充评价",
	}, customer)
	if err != nil || feedback == nil {
		t.Fatalf("submit closed conversation feedback: feedback=%+v err=%v", feedback, err)
	}
	var message models.Message
	if err := db.Where("conversation_id = ? AND sender_type = ? AND payload LIKE ?", conversation.ID, enums.IMSenderTypeSystem, "%ticket_feedback_submitted%").
		First(&message).Error; err != nil {
		t.Fatalf("feedback service event missing for closed conversation: %v", err)
	}
	if !strings.Contains(message.Content, "客户已提交服务评价") || !strings.Contains(message.Payload, "关闭后补充评价") {
		t.Fatalf("feedback service event content is incomplete: %+v", message)
	}
	currentConversation := repositories.ConversationRepository.Get(db, conversation.ID)
	if currentConversation == nil || currentConversation.Status != enums.IMConversationStatusClosed {
		t.Fatalf("feedback service event must not reopen conversation: %+v", currentConversation)
	}
}

func TestRecoverClosedTicketSideEffectsAfterRestartIsIdempotent(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Ticket{}, &models.TicketRepairRecord{}, &models.KnowledgeCandidate{}, &models.TicketQualityClue{},
		&models.FaultStatsEventInbox{}, &models.ProductFaultStatsDaily{},
		&models.DomainEvent{}, &models.OutboxRecord{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		eventbus.WaitAsync[events.KnowledgeCandidateCreatedEvent]()
		eventbus.WaitAsync[events.TicketClosedEvent]()
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	now := time.Now().UTC().Truncate(time.Second)
	ticket := models.Ticket{
		TenantID: 1, ProductID: 21, TicketNo: "RECOVER-CLOSE-1", Title: "关闭后恢复知识沉淀",
		Status: enums.TicketStatusClosed, FaultCode: "PWR-RECOVERY", ResolvedAt: &now, HandledAt: &now,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now, UpdateUserID: 77, UpdateUserName: "recovery-engineer"},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create closed ticket: %v", err)
	}
	for index, conclusion := range []string{"首次处理后复发", "二次修复并验证通过"} {
		repair := models.TicketRepairRecord{
			TenantID: 1, TicketID: ticket.ID, ProductID: ticket.ProductID, Conclusion: conclusion,
			RootCause: "接线端子松动", Solution: "重新压接并复测", TestResult: "passed",
			AuditFields: models.AuditFields{CreatedAt: now.Add(time.Duration(index) * time.Minute), UpdatedAt: now},
		}
		if err := db.Create(&repair).Error; err != nil {
			t.Fatalf("create repair %d: %v", index, err)
		}
	}

	if recovered := TicketLifecycleService.RecoverClosedTicketSideEffects(10); recovered != 1 {
		t.Fatalf("recovered count=%d, want 1", recovered)
	}
	drainTicketLifecycleOutbox(t)
	assertRecoveredTicketSideEffectCounts(t, db, ticket.ID, 1, 1)
	if recovered := TicketLifecycleService.RecoverClosedTicketSideEffects(10); recovered != 0 {
		t.Fatalf("second recovery count=%d, want 0", recovered)
	}
	assertRecoveredTicketSideEffectCounts(t, db, ticket.ID, 1, 1)

	if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, events.FaultStatsEventTicketClosed).
		Delete(&models.FaultStatsEventInbox{}).Error; err != nil {
		t.Fatalf("remove projected close event: %v", err)
	}
	if recovered := TicketLifecycleService.RecoverClosedTicketSideEffects(10); recovered != 1 {
		t.Fatalf("recovery of interrupted close event count=%d, want 1", recovered)
	}
	drainTicketLifecycleOutbox(t)
	assertRecoveredTicketSideEffectCounts(t, db, ticket.ID, 1, 1)
	var closeEventCount int64
	if err := db.Model(&models.FaultStatsEventInbox{}).
		Where("ticket_id = ? AND event_type = ?", ticket.ID, events.FaultStatsEventTicketClosed).
		Count(&closeEventCount).Error; err != nil || closeEventCount != 1 {
		t.Fatalf("recovered close event count=%d err=%v, want 1", closeEventCount, err)
	}

	if err := db.Model(&models.KnowledgeCandidate{}).Where("ticket_id = ?", ticket.ID).
		Update("review_status", "rejected").Error; err != nil {
		t.Fatalf("invalidate candidate: %v", err)
	}
	if recovered := TicketLifecycleService.RecoverClosedTicketSideEffects(10); recovered != 1 {
		t.Fatalf("recovery of interrupted re-close count=%d, want 1", recovered)
	}
	drainTicketLifecycleOutbox(t)
	assertRecoveredTicketSideEffectCounts(t, db, ticket.ID, 1, 1)
	adminRejectedAt := now.Add(time.Minute)
	if err := db.Model(&models.KnowledgeCandidate{}).Where("ticket_id = ?", ticket.ID).Updates(map[string]any{
		"review_status": "rejected", "reviewed_at": adminRejectedAt, "review_remark": "管理员确认无需沉淀",
	}).Error; err != nil {
		t.Fatalf("admin reject candidate: %v", err)
	}
	if recovered := TicketLifecycleService.RecoverClosedTicketSideEffects(10); recovered != 0 {
		t.Fatalf("admin-rejected candidate was revived by recovery: recovered=%d", recovered)
	}
	var rejected models.KnowledgeCandidate
	if err := db.Where("ticket_id = ?", ticket.ID).First(&rejected).Error; err != nil || rejected.ReviewStatus != "rejected" || rejected.ReviewRemark != "管理员确认无需沉淀" {
		t.Fatalf("admin rejection was not preserved: candidate=%+v err=%v", rejected, err)
	}
}

func drainTicketLifecycleOutbox(t *testing.T) {
	t.Helper()
	eventbus.RegisterDurable[events.KnowledgeCandidateCreatedEvent](events.EventKnowledgeCandidateCreated)
	eventbus.RegisterDurable[events.TicketClosedEvent](events.EventTicketClosed)
	eventbus.RegisterDurable[events.TicketReopenedEvent](events.FaultStatsEventTicketReopened)
	publisher := eventbus.NewOutboxPublisher(eventbus.Get[any]())
	publisher.ProcessPending(context.Background())
}

func assertRecoveredTicketSideEffectCounts(t *testing.T, db *gorm.DB, ticketID, candidateWant, clueWant int64) {
	t.Helper()
	var candidateCount int64
	if err := db.Model(&models.KnowledgeCandidate{}).
		Where("ticket_id = ? AND status <> ?", ticketID, enums.StatusDeleted).
		Count(&candidateCount).Error; err != nil || candidateCount != candidateWant {
		t.Fatalf("candidate count=%d err=%v, want %d", candidateCount, err, candidateWant)
	}
	var clueCount int64
	if err := db.Model(&models.TicketQualityClue{}).Where("ticket_id = ?", ticketID).
		Count(&clueCount).Error; err != nil || clueCount != clueWant {
		t.Fatalf("quality clue count=%d err=%v, want %d", clueCount, err, clueWant)
	}
}
