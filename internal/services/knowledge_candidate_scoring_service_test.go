package services

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestKnowledgeCandidateScoringGatesLowQualityAndGroupsDuplicates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Tenant{}, &models.Ticket{}, &models.TicketRepairRecord{},
		&models.TicketFeedback{}, &models.TicketProgress{}, &models.KnowledgeCandidate{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	now := time.Now()
	tenant := &models.Tenant{Name: "candidate-score-tenant", Status: enums.StatusOk}
	if err := db.Create(tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	operator := &dto.AuthPrincipal{TenantID: tenant.ID, UserID: 99, Username: "reviewer"}

	createHighCandidate := func(ticketNo string, deviceID int64) *models.KnowledgeCandidate {
		t.Helper()
		ticket := &models.Ticket{
			TenantID: tenant.ID, ProductID: 100, ProductModelID: 200, DeviceID: deviceID,
			TicketNo: ticketNo, Title: "控制器过热停机", FaultCode: "TEMP-001",
			PriorityCode: "p2", Status: enums.TicketStatusClosed,
			AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
		}
		if err := db.Create(ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		repair := &models.TicketRepairRecord{
			TenantID: tenant.ID, TicketID: ticket.ID, ProductID: 100, ProductModelID: 200, DeviceID: deviceID,
			RootCause:  "散热风扇接头松动导致控制器温度过高",
			Solution:   "重新固定风扇接头并清理散热通道后连续运行三十分钟",
			Conclusion: "复测期间温度稳定且未再次停机", TestResult: "passed",
			AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
		}
		if err := db.Create(repair).Error; err != nil {
			t.Fatalf("create repair: %v", err)
		}
		candidate := &models.KnowledgeCandidate{
			TenantID: tenant.ID, ProductID: 100, ProductModelID: 200, TicketID: ticket.ID,
			SourceType: "ticket_repair", SourceID: ticketNo, Title: "TEMP-001 控制器过热停机",
			Suggestion:       "复测期间温度稳定且未再次停机",
			RootCauseSummary: repair.RootCause, SolutionSummary: repair.Solution,
			Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
		}
		if err := KnowledgeCandidateScoringService.PrepareCandidateDB(db, candidate, ticket, []models.TicketRepairRecord{*repair}); err != nil {
			t.Fatalf("prepare candidate: %v", err)
		}
		if err := db.Create(candidate).Error; err != nil {
			t.Fatalf("create candidate: %v", err)
		}
		if err := KnowledgeCandidateScoringService.RefreshDuplicateGroupDB(db, candidate.SimilarityHash); err != nil {
			t.Fatalf("refresh duplicate group: %v", err)
		}
		return candidate
	}

	first := createHighCandidate("TK-SCORE-1", 301)
	second := createHighCandidate("TK-SCORE-2", 302)
	if err := db.First(first, first.ID).Error; err != nil {
		t.Fatalf("reload primary candidate: %v", err)
	}
	if err := db.First(second, second.ID).Error; err != nil {
		t.Fatalf("reload duplicate candidate: %v", err)
	}
	if first.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusPending) || !enums.IsKnowledgeCandidateReviewReady(first.ReviewStatus, first.QualityScore, first.ValueScore, first.MergedToCandidateID) {
		t.Fatalf("primary candidate should be review ready: %+v", first)
	}
	if second.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusDuplicate) || second.MergedToCandidateID != first.ID {
		t.Fatalf("second candidate should be grouped as duplicate: %+v", second)
	}
	if first.RecurrenceCount != 2 || first.AffectedDeviceCount != 2 {
		t.Fatalf("primary group statistics = recurrence %d devices %d, want 2/2", first.RecurrenceCount, first.AffectedDeviceCount)
	}
	if _, err := KnowledgeCandidateReviewService.RejectCandidate(tenant.ID, first.ID, "内容不适合作为主条目", operator); err != nil {
		t.Fatalf("reject duplicate-group primary: %v", err)
	}
	var rejectionProgress []models.TicketProgress
	if err := db.Where("tenant_id = ? AND ticket_id = ? AND event_type = ?", tenant.ID, first.TicketID, enums.TicketProgressEventProgress).
		Order("id ASC").Find(&rejectionProgress).Error; err != nil {
		t.Fatalf("query rejection progress: %v", err)
	}
	if len(rejectionProgress) != 1 || !strings.Contains(rejectionProgress[0].Content, "知识候选审核已驳回") || rejectionProgress[0].VisibleToCustomer {
		t.Fatalf("unexpected rejection progress: %+v", rejectionProgress)
	}
	if _, err := KnowledgeCandidateReviewService.RejectCandidate(tenant.ID, first.ID, "重复驳回不应重复写进展", operator); err != nil {
		t.Fatalf("repeat reject duplicate-group primary: %v", err)
	}
	if err := db.Where("tenant_id = ? AND ticket_id = ? AND event_type = ?", tenant.ID, first.TicketID, enums.TicketProgressEventProgress).
		Find(&rejectionProgress).Error; err != nil {
		t.Fatalf("query repeated rejection progress: %v", err)
	}
	if len(rejectionProgress) != 1 {
		t.Fatalf("repeated rejection created duplicate progress: %+v", rejectionProgress)
	}
	if err := db.First(second, second.ID).Error; err != nil {
		t.Fatalf("reload promoted duplicate: %v", err)
	}
	if second.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusPending) || second.MergedToCandidateID != 0 {
		t.Fatalf("next valid duplicate should be promoted after primary rejection: %+v", second)
	}
	third := createHighCandidate("TK-SCORE-3", 304)
	if err := db.First(third, third.ID).Error; err != nil {
		t.Fatalf("reload third duplicate: %v", err)
	}
	if third.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusDuplicate) {
		t.Fatalf("third candidate should initially be grouped as duplicate: %+v", third)
	}
	detached, err := KnowledgeCandidateReviewService.DetachDuplicateCandidate(tenant.ID, third.ID, operator)
	if err != nil {
		t.Fatalf("detach false-positive duplicate: %v", err)
	}
	if !detached.DeduplicationOverride || detached.MergedToCandidateID != 0 || detached.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusPending) {
		t.Fatalf("detached duplicate should become an independent review candidate: %+v", detached)
	}

	lowTicket := &models.Ticket{
		TenantID: tenant.ID, ProductID: 100, ProductModelID: 200, DeviceID: 303,
		TicketNo: "TK-SCORE-LOW", Title: "咨询", FaultCode: "TEMP-LOW",
		PriorityCode: "p4", Status: enums.TicketStatusClosed,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(lowTicket).Error; err != nil {
		t.Fatalf("create low-value ticket: %v", err)
	}
	if err := db.Create(&models.TicketRepairRecord{
		TenantID: tenant.ID, TicketID: lowTicket.ID, ProductID: 100, ProductModelID: 200, DeviceID: 303,
		RootCause: "温度探头固定位置偏移", Solution: "重新固定探头并执行温度标定",
		Conclusion: "连续复测温度恢复稳定", TestResult: "passed",
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create low candidate source repair: %v", err)
	}
	low := &models.KnowledgeCandidate{
		TenantID: tenant.ID, ProductID: 100, ProductModelID: 200, TicketID: lowTicket.ID,
		SourceType: "ticket_repair", SourceID: lowTicket.TicketNo, Title: "咨询", Suggestion: "已回复",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := KnowledgeCandidateScoringService.PrepareCandidateDB(db, low, lowTicket, nil); err != nil {
		t.Fatalf("prepare low-quality candidate: %v", err)
	}
	if err := db.Create(low).Error; err != nil {
		t.Fatalf("create low-quality candidate: %v", err)
	}
	if low.ReviewStatus == string(enums.KnowledgeCandidateReviewStatusPending) {
		t.Fatalf("low-quality candidate entered review queue: %+v", low)
	}
	if !strings.Contains(low.QualityFlagsJSON, "missing_verification") {
		t.Fatalf("candidate without supplied verification should be blocked: %+v", low)
	}
	low.ReviewStatus = string(enums.KnowledgeCandidateReviewStatusPending)
	if err := db.Model(&models.KnowledgeCandidate{}).Where("id = ?", low.ID).Update("review_status", low.ReviewStatus).Error; err != nil {
		t.Fatalf("tamper low candidate status: %v", err)
	}
	if _, err := KnowledgeCandidateReviewService.ApproveCandidate(
		tenant.ID,
		low.ID,
		dto.EnterpriseKnowledgeCandidateApproveRequest{Publish: true},
		operator,
	); err == nil {
		t.Fatal("server-side gate allowed a low-quality candidate to be approved")
	}
	enriched, err := KnowledgeCandidateReviewService.EnrichCandidate(
		tenant.ID,
		low.ID,
		dto.EnterpriseKnowledgeCandidateEnrichRequest{
			Title:            "TEMP-LOW 温度探头偏移校准",
			Suggestion:       "连续复测三十分钟后温度保持稳定",
			RootCauseSummary: "温度探头固定位置偏移导致采样值异常",
			SolutionSummary:  "重新固定温度探头，执行温度标定并连续复测三十分钟",
		},
		operator,
	)
	if err != nil {
		t.Fatalf("enrich low-quality candidate: %v", err)
	}
	if enriched.ReviewEligible || enriched.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusLowValue) {
		t.Fatalf("one-off low-impact candidate should remain outside the review queue after content enrichment: %+v", enriched)
	}
	if err := db.Model(&models.Ticket{}).Where("id = ?", lowTicket.ID).Update("priority_code", "p1").Error; err != nil {
		t.Fatalf("raise ticket impact: %v", err)
	}
	reloaded := repositories.KnowledgeCandidateRepository.Get(db, low.ID)
	if err := KnowledgeCandidateScoringService.RescoreCandidateDB(db, reloaded); err != nil {
		t.Fatalf("rescore high-impact candidate: %v", err)
	}
	reloaded = repositories.KnowledgeCandidateRepository.Get(db, low.ID)
	if reloaded.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusPending) ||
		!enums.IsKnowledgeCandidateReviewReady(reloaded.ReviewStatus, reloaded.QualityScore, reloaded.ValueScore, reloaded.MergedToCandidateID) {
		t.Fatalf("verified high-impact candidate should enter the review queue: %+v", reloaded)
	}
}

func TestKnowledgeCandidateCompatibilityListReturnsAllPages(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Tenant{}, &models.KnowledgeCandidate{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	tenant := &models.Tenant{Name: "candidate-page-tenant", Status: enums.StatusOk}
	if err := db.Create(tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	now := time.Now()
	rows := make([]models.KnowledgeCandidate, 0, 205)
	for i := 0; i < 205; i++ {
		rows = append(rows, models.KnowledgeCandidate{
			TenantID: tenant.ID, ProductID: 999, Title: "candidate", QualityScore: 90, ValueScore: 60,
			CandidateScore: 80, ScoreVersion: enums.KnowledgeCandidateScoreVersion,
			ReviewStatus: string(enums.KnowledgeCandidateReviewStatusPending), Status: enums.StatusOk,
			AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
		})
	}
	if err := db.CreateInBatches(&rows, 50).Error; err != nil {
		t.Fatalf("create candidates: %v", err)
	}
	stale := &models.KnowledgeCandidate{
		TenantID: tenant.ID, ProductID: 999, Title: "x",
		QualityScore: 99, ValueScore: 99, CandidateScore: 99, ScoreVersion: "legacy-score-v0",
		ReviewStatus: string(enums.KnowledgeCandidateReviewStatusPending), Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(stale).Error; err != nil {
		t.Fatalf("create stale candidate: %v", err)
	}
	items, err := KnowledgeCandidateReviewService.ListCandidates(tenant.ID, 999, string(enums.KnowledgeCandidateReviewStatusPending))
	if err != nil {
		t.Fatalf("list candidates: %v", err)
	}
	if len(items) != 205 {
		t.Fatalf("compatibility list returned %d candidates, want 205", len(items))
	}
	lowQuality, err := KnowledgeCandidateReviewService.ListCandidates(tenant.ID, 999, string(enums.KnowledgeCandidateReviewStatusLowQuality))
	if err != nil {
		t.Fatalf("list rescored candidates: %v", err)
	}
	if len(lowQuality) != 1 || lowQuality[0].ID != stale.ID || lowQuality[0].ScoreVersion != enums.KnowledgeCandidateScoreVersion {
		t.Fatalf("stale candidate was not moved into its current scoring queue: %+v", lowQuality)
	}
}

func TestKnowledgeCandidateMergeRequiresEquivalentPublishedEntry(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Tenant{}, &models.Product{}, &models.ProductModel{}, &models.Ticket{},
		&models.KnowledgeBase{}, &models.KnowledgeCandidate{}, &models.KnowledgeDocument{},
		&models.KnowledgeFAQ{}, &models.KnowledgeRevision{}, &models.ProductKnowledgeLink{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	now := time.Now()
	tenant := &models.Tenant{ID: 7, Name: "merge-tenant", Status: enums.StatusOk}
	product := &models.Product{ID: 10, TenantID: tenant.ID, Code: "P-MERGE", Name: "Merge Product", Status: enums.StatusOk}
	productModel := &models.ProductModel{ID: 20, TenantID: tenant.ID, ProductID: product.ID, ModelCode: "M-MERGE", Name: "Merge Model", Status: enums.StatusOk}
	documentKB := &models.KnowledgeBase{ID: 30, TenantID: tenant.ID, Name: "Document KB", KnowledgeType: "document", Status: enums.StatusOk}
	faqKB := &models.KnowledgeBase{ID: 31, TenantID: tenant.ID, Name: "FAQ KB", KnowledgeType: "faq", Status: enums.StatusOk}
	for _, item := range []any{tenant, product, productModel, documentKB, faqKB} {
		if err := db.Create(item).Error; err != nil {
			t.Fatalf("create merge fixture %T: %v", item, err)
		}
	}
	ticket := &models.Ticket{
		TenantID: tenant.ID, ProductID: product.ID, ProductModelID: productModel.ID, TicketNo: "TK-MERGE-1", FaultCode: "TEMP-001",
		Status: enums.TicketStatusClosed, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	candidate := &models.KnowledgeCandidate{
		TenantID: tenant.ID, ProductID: product.ID, ProductModelID: productModel.ID, TicketID: ticket.ID,
		Title: "TEMP-001 控制器过热停机", Suggestion: "复测温度稳定",
		RootCauseSummary: "散热风扇接头松动导致控制器过热", SolutionSummary: "固定风扇接头并清理散热通道后复测",
		QualityScore: 95, ValueScore: 60, CandidateScore: 83, ScoreVersion: enums.KnowledgeCandidateScoreVersion,
		ReviewStatus: string(enums.KnowledgeCandidateReviewStatusPending), Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(candidate).Error; err != nil {
		t.Fatalf("create candidate: %v", err)
	}
	unrelated := &models.KnowledgeDocument{
		TenantID: tenant.ID, KnowledgeBaseID: documentKB.ID, Title: "网络连接失败", Content: "检查网线与交换机配置并重新连接。",
		ReviewStatus: "published", Status: enums.StatusOk, FaultCodesJSON: `["NET-009"]`,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(unrelated).Error; err != nil {
		t.Fatalf("create unrelated entry: %v", err)
	}
	unrelatedRevision := &models.KnowledgeRevision{TenantID: tenant.ID, KnowledgeBaseID: documentKB.ID, EntryType: "document", EntryID: unrelated.ID, VersionNo: 1, Title: unrelated.Title, Content: unrelated.Content, ReviewStatus: "published", AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(unrelatedRevision).Error; err != nil {
		t.Fatalf("create unrelated revision: %v", err)
	}
	if err := db.Model(unrelated).Update("published_revision_id", unrelatedRevision.ID).Error; err != nil {
		t.Fatalf("attach unrelated revision: %v", err)
	}
	operator := &dto.AuthPrincipal{TenantID: tenant.ID, UserID: 9, Username: "reviewer"}
	if _, err := KnowledgeCandidateReviewService.MergeCandidate(tenant.ID, candidate.ID, encodeEnterpriseKnowledgeID("document", unrelated.ID), operator); err == nil {
		t.Fatal("candidate was merged into an unrelated published entry")
	}
	matching := &models.KnowledgeDocument{
		TenantID: tenant.ID, KnowledgeBaseID: documentKB.ID, Title: "TEMP-001 控制器过热停机", Content: "散热风扇接头松动时，固定接头并清理散热通道，随后复测温度。",
		ReviewStatus: "published", Status: enums.StatusOk, PublishedRevisionID: 999999, IndexStatus: enums.KnowledgeDocumentIndexStatusIndexed, FaultCodesJSON: `["TEMP-001"]`,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(matching).Error; err != nil {
		t.Fatalf("create matching entry: %v", err)
	}
	if _, err := KnowledgeCandidateReviewService.MergeCandidate(tenant.ID, candidate.ID, encodeEnterpriseKnowledgeID("document", matching.ID), operator); err == nil {
		t.Fatal("candidate was merged through a nonexistent published revision")
	}
	matchingRevision := &models.KnowledgeRevision{TenantID: tenant.ID, KnowledgeBaseID: documentKB.ID, EntryType: "document", EntryID: matching.ID, VersionNo: 1, Title: matching.Title, Content: matching.Content, ReviewStatus: "published", AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(matchingRevision).Error; err != nil {
		t.Fatalf("create matching revision: %v", err)
	}
	if err := db.Model(matching).Update("published_revision_id", matchingRevision.ID).Error; err != nil {
		t.Fatalf("attach matching revision: %v", err)
	}
	draftLink := &models.ProductKnowledgeLink{TenantID: tenant.ID, ProductID: product.ID, ProductModelID: productModel.ID, KnowledgeBaseID: documentKB.ID, KnowledgeEntryID: matching.ID, LinkType: "document", PublishStatus: "draft", Status: enums.StatusOk}
	if err := db.Create(draftLink).Error; err != nil {
		t.Fatalf("create draft product link: %v", err)
	}
	merged, err := KnowledgeCandidateReviewService.MergeCandidate(tenant.ID, candidate.ID, encodeEnterpriseKnowledgeID("document", matching.ID), operator)
	if err != nil {
		t.Fatalf("merge equivalent entry: %v", err)
	}
	if merged.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusMerged) || merged.KnowledgeEntryID != encodeEnterpriseKnowledgeID("document", matching.ID) {
		t.Fatalf("merged candidate = %+v", merged)
	}
	var documentLink models.ProductKnowledgeLink
	if err := db.Where("knowledge_base_id = ? AND knowledge_entry_id = ?", matching.KnowledgeBaseID, matching.ID).First(&documentLink).Error; err != nil {
		t.Fatalf("load merged document link: %v", err)
	}
	if documentLink.LinkType != "document" {
		t.Fatalf("merged document link type = %q", documentLink.LinkType)
	}
	if documentLink.ID != draftLink.ID || documentLink.PublishStatus != "published" {
		t.Fatalf("merged document link = %+v, want promoted draft link", documentLink)
	}
	if err := db.First(matching, matching.ID).Error; err != nil {
		t.Fatalf("reload matching entry: %v", err)
	}
	if matching.IndexStatus != enums.KnowledgeDocumentIndexStatusPending {
		t.Fatalf("merged document index status = %q, want pending", matching.IndexStatus)
	}

	faqCandidate := *candidate
	faqCandidate.ID = 0
	faqCandidate.ReviewStatus = string(enums.KnowledgeCandidateReviewStatusPending)
	faqCandidate.KnowledgeEntryID = 0
	if err := db.Create(&faqCandidate).Error; err != nil {
		t.Fatalf("create faq candidate: %v", err)
	}
	matchingFAQ := &models.KnowledgeFAQ{
		TenantID: tenant.ID, KnowledgeBaseID: faqKB.ID, Question: "TEMP-001 控制器过热如何处理？", Answer: "固定散热风扇接头并清理通道，随后复测温度。",
		ReviewStatus: "published", Status: enums.StatusOk, IndexStatus: enums.KnowledgeDocumentIndexStatusIndexed, FaultCodesJSON: `["TEMP-001"]`,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(matchingFAQ).Error; err != nil {
		t.Fatalf("create matching faq: %v", err)
	}
	faqRevision := &models.KnowledgeRevision{TenantID: tenant.ID, KnowledgeBaseID: faqKB.ID, EntryType: "faq", EntryID: matchingFAQ.ID, VersionNo: 1, Title: matchingFAQ.Question, Content: matchingFAQ.Answer, ReviewStatus: "published", AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(faqRevision).Error; err != nil {
		t.Fatalf("create faq revision: %v", err)
	}
	if err := db.Model(matchingFAQ).Update("published_revision_id", faqRevision.ID).Error; err != nil {
		t.Fatalf("attach faq revision: %v", err)
	}
	if _, err := KnowledgeCandidateReviewService.MergeCandidate(tenant.ID, faqCandidate.ID, encodeEnterpriseKnowledgeID("faq", matchingFAQ.ID), operator); err != nil {
		t.Fatalf("merge equivalent faq: %v", err)
	}
	var faqLink models.ProductKnowledgeLink
	if err := db.Where("knowledge_base_id = ? AND knowledge_entry_id = ?", matchingFAQ.KnowledgeBaseID, matchingFAQ.ID).First(&faqLink).Error; err != nil {
		t.Fatalf("load merged faq link: %v", err)
	}
	if faqLink.LinkType != "faq" {
		t.Fatalf("merged faq link type = %q", faqLink.LinkType)
	}
}

func TestKnowledgeCandidateApproximateDuplicateScanBeyondFirstPage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.KnowledgeCandidate{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	now := time.Now()
	primary := &models.KnowledgeCandidate{
		TenantID: 7, ProductID: 10, ProductModelID: 20,
		Title: "控制器过热停机", RootCauseSummary: "散热风扇接头松动导致控制器持续过热停机", SolutionSummary: "固定接头并清理散热通道后复测",
		ReviewStatus: string(enums.KnowledgeCandidateReviewStatusPending), Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	primary.SimilarityHash = buildKnowledgeCandidateSimilarityHash(primary, nil)
	primary.DuplicateGroupID = primary.SimilarityHash[:24]
	if err := db.Create(primary).Error; err != nil {
		t.Fatalf("create primary: %v", err)
	}
	noise := make([]models.KnowledgeCandidate, 0, 501)
	for i := 0; i < 501; i++ {
		item := models.KnowledgeCandidate{
			TenantID: 7, ProductID: 10, ProductModelID: 20,
			Title: fmt.Sprintf("网络故障 %d", i), RootCauseSummary: fmt.Sprintf("交换机端口配置异常编号%d", i), SolutionSummary: "重新配置网络端口并验证",
			ReviewStatus: string(enums.KnowledgeCandidateReviewStatusPending), Status: enums.StatusOk,
			AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
		}
		item.SimilarityHash = buildKnowledgeCandidateSimilarityHash(&item, nil)
		item.DuplicateGroupID = item.SimilarityHash[:24]
		noise = append(noise, item)
	}
	if err := db.CreateInBatches(&noise, 100).Error; err != nil {
		t.Fatalf("create noise candidates: %v", err)
	}
	candidate := &models.KnowledgeCandidate{
		TenantID: 7, ProductID: 10, ProductModelID: 20,
		Title: "控制器过热停机", RootCauseSummary: "散热风扇接头松动导致控制器过热停机", SolutionSummary: "固定接头并清理散热通道后复测",
		Status: enums.StatusOk,
	}
	if err := KnowledgeCandidateScoringService.PrepareCandidateDB(db, candidate, nil, nil); err != nil {
		t.Fatalf("prepare duplicate candidate: %v", err)
	}
	if candidate.SimilarityHash != primary.SimilarityHash || candidate.MergedToCandidateID != primary.ID || candidate.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusDuplicate) {
		t.Fatalf("historical duplicate was missed: %+v", candidate)
	}
}
