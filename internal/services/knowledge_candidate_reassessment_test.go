package services

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestApprovedCandidateIsQuarantinedOnReopenAndCanBeRepublishedAfterNewVerification(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Ticket{}, &models.TicketRepairRecord{}, &models.TicketProgress{},
		&models.KnowledgeCandidate{}, &models.KnowledgeBase{}, &models.KnowledgeDocument{},
		&models.KnowledgeRevision{}, &models.ProductKnowledgeLink{}, &models.TicketQualityClue{},
		&models.FaultStatsEventInbox{}, &models.ProductFaultStatsDaily{},
		&models.DomainEvent{}, &models.OutboxRecord{}, &models.KnowledgeIndexGeneration{},
		&models.KnowledgeIndexSyncTask{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if raw, closeErr := db.DB(); closeErr == nil {
			_ = raw.Close()
		}
	})

	now := time.Now().UTC().Truncate(time.Second)
	operator := &dto.AuthPrincipal{TenantID: 41, UserID: 9, Username: "service-manager"}
	kb := &models.KnowledgeBase{
		TenantID: 41, Name: "Controller KB", KnowledgeType: string(enums.KnowledgeBaseTypeDocument), Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(kb).Error; err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}
	ticket := &models.Ticket{
		TenantID: 41, ProductID: 100, ProductModelID: 200, DeviceID: 300,
		TicketNo: "TK-REASSESS-1", Title: "控制器过热停机", FaultCode: "TEMP-001", PriorityCode: "p1",
		Status: enums.TicketStatusClosed, ResolvedAt: &now, HandledAt: &now,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
	}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	candidate := &models.KnowledgeCandidate{
		TenantID: 41, ProductID: 100, ProductModelID: 200, TicketID: ticket.ID,
		SourceType: "ticket_repair", SourceID: ticket.TicketNo, Title: "TEMP-001 控制器过热停机",
		Suggestion: "重新固定风扇接头后运行稳定", RootCauseSummary: "散热风扇接头松动导致控制器温度过高",
		SolutionSummary: "重新固定风扇接头并清理散热通道，连续复测三十分钟",
		KnowledgeBaseID: kb.ID, QualityScore: 95, ValueScore: 55, CandidateScore: 81,
		ScoreVersion: enums.KnowledgeCandidateScoreVersion, ReviewStatus: string(enums.KnowledgeCandidateReviewStatusApproved),
		ReviewedAt: &now, ReviewerID: operator.UserID, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(candidate).Error; err != nil {
		t.Fatalf("create approved candidate: %v", err)
	}
	document := &models.KnowledgeDocument{
		TenantID: 41, KnowledgeBaseID: kb.ID, Title: candidate.Title, Content: candidate.SolutionSummary,
		SourceType: "knowledge_candidate", SourceReferenceID: candidate.ID, ReviewStatus: "published",
		Status: enums.StatusOk, IndexStatus: enums.KnowledgeDocumentIndexStatusIndexed,
		ContentHash: knowledgeContentHash(candidate.SolutionSummary), AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(document).Error; err != nil {
		t.Fatalf("create published candidate document: %v", err)
	}
	candidate.KnowledgeEntryID = encodeEnterpriseKnowledgeID("document", document.ID)
	if err := db.Model(candidate).Update("knowledge_entry_id", candidate.KnowledgeEntryID).Error; err != nil {
		t.Fatalf("link candidate entry: %v", err)
	}
	link := &models.ProductKnowledgeLink{
		TenantID: 41, ProductID: 100, ProductModelID: 200, KnowledgeBaseID: kb.ID, KnowledgeEntryID: document.ID,
		LinkType: "document", PublishStatus: "published", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(link).Error; err != nil {
		t.Fatalf("create product knowledge link: %v", err)
	}

	if err := TicketLifecycleService.Reopen(ticket.ID, "设备再次出现过热停机", operator); err != nil {
		t.Fatalf("reopen ticket: %v", err)
	}
	quarantinedCandidate := repositories.KnowledgeCandidateRepository.Get(db, candidate.ID)
	quarantinedDocument := repositories.KnowledgeDocumentRepository.Get(db, document.ID)
	quarantinedLink := repositories.ProductKnowledgeLinkRepository.Get(db, link.ID)
	if quarantinedCandidate == nil || !quarantinedCandidate.RequiresReassessment || quarantinedCandidate.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusApproved) {
		t.Fatalf("reopened approved candidate = %+v, want reassessment", quarantinedCandidate)
	}
	if quarantinedDocument == nil || quarantinedDocument.ReviewStatus != "deprecated" || quarantinedDocument.Status != enums.StatusDisabled || quarantinedDocument.IndexStatus != enums.KnowledgeDocumentIndexStatusPending {
		t.Fatalf("reopened candidate document = %+v, want quarantined", quarantinedDocument)
	}
	if quarantinedLink == nil || quarantinedLink.PublishStatus != "deprecated" {
		t.Fatalf("reopened candidate link = %+v, want deprecated", quarantinedLink)
	}

	secondResolvedAt := now.Add(time.Hour)
	if err := db.Model(ticket).Updates(map[string]any{
		"status": enums.TicketStatusClosed, "resolved_at": secondResolvedAt, "handled_at": secondResolvedAt, "updated_at": secondResolvedAt,
	}).Error; err != nil {
		t.Fatalf("close second repair round: %v", err)
	}
	secondRepair := &models.TicketRepairRecord{
		TenantID: 41, TicketID: ticket.ID, ProductID: 100, ProductModelID: 200, DeviceID: 300,
		RootCause: "风扇供电端子压接不足导致间歇断路", Solution: "重新压接供电端子并更换松动插壳",
		Conclusion: "满载连续复测六十分钟温度稳定", TestResult: "passed",
		AuditFields: models.AuditFields{CreatedAt: secondResolvedAt, UpdatedAt: secondResolvedAt},
	}
	if err := db.Create(secondRepair).Error; err != nil {
		t.Fatalf("create second repair: %v", err)
	}
	closedTicket := repositories.TicketRepository.Get(db, ticket.ID)
	if closedTicket == nil {
		t.Fatal("closed ticket was not found")
	}
	if recovered := TicketLifecycleService.RecoverClosedTicketSideEffects(10); recovered != 1 {
		t.Fatalf("recover reassessment close side effects = %d, want 1", recovered)
	}
	rescored := repositories.KnowledgeCandidateRepository.Get(db, candidate.ID)
	if rescored == nil || rescored.RequiresReassessment || rescored.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusPending) || rescored.KnowledgeEntryID <= 0 {
		t.Fatalf("candidate after new verified close = %+v, want review-ready reassessment", rescored)
	}
	if _, err := KnowledgeCandidateReviewService.ApproveCandidate(41, candidate.ID, dto.EnterpriseKnowledgeCandidateApproveRequest{
		Title: "Unrelated note", Content: "This text does not describe the verified controller repair.", Publish: true,
	}, operator); err == nil {
		t.Fatal("candidate score was reused to approve unrelated replacement content")
	}
	if _, err := KnowledgeCandidateReviewService.ApproveCandidate(41, candidate.ID, dto.EnterpriseKnowledgeCandidateApproveRequest{Publish: true}, operator); err != nil {
		t.Fatalf("republish reassessed candidate: %v", err)
	}
	if _, err := KnowledgeCandidateReviewService.ApproveCandidate(41, candidate.ID, dto.EnterpriseKnowledgeCandidateApproveRequest{Publish: true}, operator); err != nil {
		t.Fatalf("idempotent candidate approval retry: %v", err)
	}
	republishedDocument := repositories.KnowledgeDocumentRepository.Get(db, document.ID)
	if republishedDocument == nil || republishedDocument.ReviewStatus != "published" || republishedDocument.Status != enums.StatusOk || republishedDocument.PublishedRevisionID <= 0 {
		t.Fatalf("republished candidate document = %+v", republishedDocument)
	}
	var documentCount, revisionCount, linkCount int64
	if err := db.Model(&models.KnowledgeDocument{}).Where("tenant_id = ? AND source_type = ? AND source_reference_id = ? AND status <> ?", 41, "knowledge_candidate", candidate.ID, enums.StatusDeleted).Count(&documentCount).Error; err != nil {
		t.Fatalf("count candidate documents: %v", err)
	}
	if err := db.Model(&models.KnowledgeRevision{}).Where("tenant_id = ? AND entry_type = ? AND entry_id = ?", 41, "document", document.ID).Count(&revisionCount).Error; err != nil {
		t.Fatalf("count candidate revisions: %v", err)
	}
	if err := db.Model(&models.ProductKnowledgeLink{}).Where("tenant_id = ? AND product_id = ? AND knowledge_entry_id = ? AND status <> ?", 41, 100, document.ID, enums.StatusDeleted).Count(&linkCount).Error; err != nil {
		t.Fatalf("count candidate links: %v", err)
	}
	if documentCount != 1 || revisionCount != 1 || linkCount != 1 {
		t.Fatalf("approval retry created duplicates: documents=%d revisions=%d links=%d", documentCount, revisionCount, linkCount)
	}
	if err := db.Model(&models.KnowledgeDocument{}).Where("id = ?", document.ID).Updates(map[string]any{
		"title": "Unrelated note", "content": "This text is unrelated to the verified repair evidence.",
		"review_status": "draft", "status": enums.StatusDisabled,
	}).Error; err != nil {
		t.Fatalf("tamper candidate document: %v", err)
	}
	if _, err := EnterpriseKnowledgeService.UpdateEntryStatus(41, encodeEnterpriseKnowledgeID("document", document.ID), "published", operator); err == nil {
		t.Fatal("candidate score was reused to publish content unrelated to its evidence")
	}
}

func TestNegativeFeedbackRescoresAndQuarantinesApprovedCandidate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Ticket{}, &models.TicketRepairRecord{}, &models.TicketFeedback{},
		&models.KnowledgeCandidate{}, &models.KnowledgeBase{}, &models.KnowledgeDocument{},
		&models.ProductKnowledgeLink{}, &models.DomainEvent{}, &models.OutboxRecord{},
		&models.KnowledgeIndexGeneration{}, &models.KnowledgeIndexSyncTask{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if raw, closeErr := db.DB(); closeErr == nil {
			_ = raw.Close()
		}
	})

	now := time.Now().Add(-time.Minute)
	operator := &dto.AuthPrincipal{TenantID: 52, Username: "customer", DomainType: models.DomainTypeCustomer, SubjectType: models.SubjectTypeTempVisitor}
	kb := &models.KnowledgeBase{TenantID: 52, Name: "Repair KB", KnowledgeType: string(enums.KnowledgeBaseTypeDocument), Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(kb).Error; err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}
	ticket := &models.Ticket{
		TenantID: 52, ProductID: 501, ProductModelID: 502, DeviceID: 503,
		TicketNo: "TK-NEGATIVE-1", Title: "控制器掉电", FaultCode: "PWR-12", PriorityCode: "p1",
		Status: enums.TicketStatusResolved, ResolvedAt: &now,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
	}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	repair := &models.TicketRepairRecord{
		TenantID: 52, TicketID: ticket.ID, ProductID: 501, ProductModelID: 502, DeviceID: 503,
		RootCause: "电源端子松动导致控制器间歇掉电", Solution: "重新压接电源端子并固定线束",
		Conclusion: "连续复测三十分钟未再掉电", TestResult: "passed",
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(repair).Error; err != nil {
		t.Fatalf("create repair: %v", err)
	}
	candidate := &models.KnowledgeCandidate{
		TenantID: 52, ProductID: 501, ProductModelID: 502, TicketID: ticket.ID,
		SourceType: "ticket_repair", SourceID: ticket.TicketNo, Title: "PWR-12 控制器掉电",
		Suggestion: repair.Conclusion, RootCauseSummary: repair.RootCause, SolutionSummary: repair.Solution,
		KnowledgeBaseID: kb.ID, QualityScore: 95, ValueScore: 45, CandidateScore: 78,
		ScoreVersion: enums.KnowledgeCandidateScoreVersion, ReviewStatus: string(enums.KnowledgeCandidateReviewStatusApproved),
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(candidate).Error; err != nil {
		t.Fatalf("create candidate: %v", err)
	}
	document := &models.KnowledgeDocument{
		TenantID: 52, KnowledgeBaseID: kb.ID, Title: candidate.Title, Content: candidate.SolutionSummary,
		SourceType: "knowledge_candidate", SourceReferenceID: candidate.ID, ReviewStatus: "published",
		Status: enums.StatusOk, IndexStatus: enums.KnowledgeDocumentIndexStatusIndexed,
		ContentHash: knowledgeContentHash(candidate.SolutionSummary), AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(document).Error; err != nil {
		t.Fatalf("create candidate document: %v", err)
	}
	candidate.KnowledgeEntryID = encodeEnterpriseKnowledgeID("document", document.ID)
	if err := db.Model(candidate).Update("knowledge_entry_id", candidate.KnowledgeEntryID).Error; err != nil {
		t.Fatalf("link candidate document: %v", err)
	}
	if err := db.Create(&models.ProductKnowledgeLink{
		TenantID: 52, ProductID: 501, ProductModelID: 502, KnowledgeBaseID: kb.ID, KnowledgeEntryID: document.ID,
		LinkType: "document", PublishStatus: "published", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create product link: %v", err)
	}

	if _, err := CustomerTicketActionService.SubmitFeedback(ticket, 0, request.SubmitTicketFeedbackRequest{
		TicketID: ticket.ID, Rating: 1, Comment: "处理后仍然掉电",
	}, operator); err != nil {
		t.Fatalf("submit negative feedback: %v", err)
	}
	rescored := repositories.KnowledgeCandidateRepository.Get(db, candidate.ID)
	quarantined := repositories.KnowledgeDocumentRepository.Get(db, document.ID)
	if rescored == nil || !rescored.RequiresReassessment || !strings.Contains(rescored.QualityFlagsJSON, "negative_customer_feedback") {
		t.Fatalf("negative feedback candidate = %+v, want reassessment flag", rescored)
	}
	if quarantined == nil || quarantined.ReviewStatus != "deprecated" || quarantined.Status != enums.StatusDisabled {
		t.Fatalf("negative feedback document = %+v, want quarantined", quarantined)
	}
	if _, err := KnowledgeCandidateReviewService.EnrichCandidate(52, candidate.ID, dto.EnterpriseKnowledgeCandidateEnrichRequest{
		Title: candidate.Title, Suggestion: candidate.Suggestion, RootCauseSummary: candidate.RootCauseSummary, SolutionSummary: candidate.SolutionSummary,
	}, &dto.AuthPrincipal{TenantID: 52, UserID: 7, Username: "reviewer"}); err == nil {
		t.Fatal("editing text bypassed the required new repair verification round")
	}
	if _, err := KnowledgeCandidateReviewService.ApproveCandidate(52, candidate.ID, dto.EnterpriseKnowledgeCandidateApproveRequest{Publish: true}, &dto.AuthPrincipal{
		TenantID: 52, UserID: 7, Username: "reviewer",
	}); err == nil {
		t.Fatal("reapproval bypassed the required new repair verification round")
	}
}
