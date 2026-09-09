package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"

	"gorm.io/gorm"
)

var KnowledgeCandidateScoringService = newKnowledgeCandidateScoringService()

type knowledgeCandidateScoringService struct{}

type knowledgeCandidateAssessment struct {
	QualityScore   int
	ValueScore     int
	CandidateScore int
	Breakdown      dto.KnowledgeCandidateScoreBreakdownDTO
	Flags          []string
	ReviewStatus   string
	ReviewRemark   string
}

func newKnowledgeCandidateScoringService() *knowledgeCandidateScoringService {
	return &knowledgeCandidateScoringService{}
}

// PrepareCandidateDB scores a candidate and checks whether an equivalent primary
// candidate already exists. It must run before inserting or refreshing a candidate.
func (s *knowledgeCandidateScoringService) PrepareCandidateDB(db *gorm.DB, candidate *models.KnowledgeCandidate, ticket *models.Ticket, repairs []models.TicketRepairRecord) error {
	if db == nil || candidate == nil {
		return nil
	}
	if ticket == nil && candidate.TicketID > 0 {
		ticket = loadCandidateTicket(db, candidate.TenantID, candidate.TicketID)
	}
	candidate.SimilarityHash = s.resolveKnowledgeCandidateSimilarityHash(db, candidate, ticket)
	candidate.MergedToCandidateID = 0
	if candidate.SimilarityHash != "" {
		candidate.DuplicateGroupID = candidate.SimilarityHash[:24]
	}
	group, err := s.loadGroupCandidates(db, candidate)
	if err != nil {
		return err
	}
	recurrenceCount, affectedDeviceCount, err := loadKnowledgeCandidateGroupStats(db, group, ticket)
	if err != nil {
		return err
	}
	candidate.RecurrenceCount = recurrenceCount
	candidate.AffectedDeviceCount = affectedDeviceCount
	assessment := s.assessCandidate(db, candidate, ticket, repairs, recurrenceCount, affectedDeviceCount)
	s.applyAssessment(candidate, assessment, false)

	if primary := selectKnowledgeCandidatePrimary(group); primary != nil {
		candidate.MergedToCandidateID = primary.ID
		candidate.ReviewStatus = string(enums.KnowledgeCandidateReviewStatusDuplicate)
		candidate.ReviewRemark = fmt.Sprintf("与候选 #%d 内容重复，已归入同一故障组", primary.ID)
		candidate.ReviewedAt = nil
		candidate.ReviewerID = 0
	}
	return nil
}

func (s *knowledgeCandidateScoringService) resolveKnowledgeCandidateSimilarityHash(db *gorm.DB, candidate *models.KnowledgeCandidate, ticket *models.Ticket) string {
	baseHash := buildKnowledgeCandidateSimilarityHash(candidate, ticket)
	if db == nil || candidate == nil || baseHash == "" || candidate.DeduplicationOverride {
		return baseHash
	}
	query := db.Where("tenant_id = ? AND product_id = ? AND product_model_id = ? AND status <> ? AND review_status <> ? AND similarity_hash <> ''",
		candidate.TenantID, candidate.ProductID, candidate.ProductModelID, enums.StatusDeleted, enums.KnowledgeCandidateReviewStatusRejected)
	if candidate.ID > 0 {
		query = query.Where("id <> ?", candidate.ID)
	}
	lastID := int64(0)
	for {
		var candidates []models.KnowledgeCandidate
		page := query
		if lastID > 0 {
			page = page.Where("id < ?", lastID)
		}
		if err := page.Order("id DESC").Limit(500).Find(&candidates).Error; err != nil {
			return baseHash
		}
		tickets := loadCandidateTicketPage(db, candidate.TenantID, candidates)
		for i := range candidates {
			otherTicket := tickets[candidates[i].TicketID]
			if knowledgeCandidatesEquivalent(candidate, ticket, &candidates[i], otherTicket) {
				return candidates[i].SimilarityHash
			}
		}
		if len(candidates) < 500 {
			break
		}
		lastID = candidates[len(candidates)-1].ID
	}
	return baseHash
}

func loadCandidateTicketPage(db *gorm.DB, tenantID int64, candidates []models.KnowledgeCandidate) map[int64]*models.Ticket {
	result := make(map[int64]*models.Ticket)
	if db == nil || len(candidates) == 0 || !db.Migrator().HasTable(&models.Ticket{}) {
		return result
	}
	ticketIDs := make([]int64, 0, len(candidates))
	seen := make(map[int64]struct{}, len(candidates))
	for i := range candidates {
		if candidates[i].TicketID <= 0 {
			continue
		}
		if _, ok := seen[candidates[i].TicketID]; ok {
			continue
		}
		seen[candidates[i].TicketID] = struct{}{}
		ticketIDs = append(ticketIDs, candidates[i].TicketID)
	}
	if len(ticketIDs) == 0 {
		return result
	}
	var tickets []models.Ticket
	query := db.Where("id IN ?", ticketIDs)
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if err := query.Find(&tickets).Error; err != nil {
		return result
	}
	for i := range tickets {
		result[tickets[i].ID] = &tickets[i]
	}
	return result
}

func knowledgeCandidatesEquivalent(left *models.KnowledgeCandidate, leftTicket *models.Ticket, right *models.KnowledgeCandidate, rightTicket *models.Ticket) bool {
	if left == nil || right == nil {
		return false
	}
	leftFault := ""
	rightFault := ""
	if leftTicket != nil {
		leftFault = normalizeKnowledgeCandidateIdentity(leftTicket.FaultCode)
	}
	if rightTicket != nil {
		rightFault = normalizeKnowledgeCandidateIdentity(rightTicket.FaultCode)
	}
	if leftFault != "" && rightFault != "" && leftFault != rightFault {
		return false
	}
	if meaningfulKnowledgeText(left.RootCauseSummary) && meaningfulKnowledgeText(right.RootCauseSummary) {
		return knowledgeCandidateTextSimilarity(left.RootCauseSummary, right.RootCauseSummary) >= 0.78
	}
	return knowledgeCandidateTextSimilarity(left.Title+" "+left.SolutionSummary, right.Title+" "+right.SolutionSummary) >= 0.85
}

// RefreshDuplicateGroupDB recomputes recurrence and impact after a candidate was inserted.
func (s *knowledgeCandidateScoringService) RefreshDuplicateGroupDB(db *gorm.DB, similarityHash string) error {
	if db == nil || strings.TrimSpace(similarityHash) == "" {
		return nil
	}
	var group []models.KnowledgeCandidate
	if err := db.Where("similarity_hash = ? AND status <> ? AND review_status <> ?", similarityHash, enums.StatusDeleted, enums.KnowledgeCandidateReviewStatusRejected).
		Order("id ASC").Find(&group).Error; err != nil {
		return err
	}
	if len(group) == 0 {
		return nil
	}
	primary := selectKnowledgeCandidatePrimary(group)
	if primary == nil {
		return nil
	}
	recurrenceCount, affectedDeviceCount, err := loadKnowledgeCandidateGroupStats(db, group, nil)
	if err != nil {
		return err
	}
	for i := range group {
		candidate := &group[i]
		candidate.RecurrenceCount = recurrenceCount
		candidate.AffectedDeviceCount = affectedDeviceCount
		ticket := loadCandidateTicket(db, candidate.TenantID, candidate.TicketID)
		repairs := loadCandidateRepairs(db, candidate.TicketID)
		assessment := s.assessCandidate(db, candidate, ticket, repairs, recurrenceCount, affectedDeviceCount)
		preserveStatus := isTerminalKnowledgeCandidateStatus(candidate.ReviewStatus)
		s.applyAssessment(candidate, assessment, preserveStatus)
		if candidate.ID == primary.ID {
			candidate.MergedToCandidateID = 0
			if candidate.ReviewStatus == string(enums.KnowledgeCandidateReviewStatusDuplicate) {
				candidate.ReviewStatus = assessment.ReviewStatus
				candidate.ReviewRemark = assessment.ReviewRemark
			}
		} else if !preserveStatus {
			candidate.MergedToCandidateID = primary.ID
			candidate.ReviewStatus = string(enums.KnowledgeCandidateReviewStatusDuplicate)
			candidate.ReviewRemark = fmt.Sprintf("与候选 #%d 内容重复，已归入同一故障组", primary.ID)
		}
		if err := db.Model(&models.KnowledgeCandidate{}).Where("id = ?", candidate.ID).
			Updates(knowledgeCandidateScoreUpdates(candidate)).Error; err != nil {
			return err
		}
	}
	return nil
}

// RescoreCandidateDB repairs legacy or edited candidates before review.
func (s *knowledgeCandidateScoringService) RescoreCandidateDB(db *gorm.DB, candidate *models.KnowledgeCandidate) error {
	if db == nil || candidate == nil {
		return nil
	}
	previousSimilarityHash := candidate.SimilarityHash
	ticket := loadCandidateTicket(db, candidate.TenantID, candidate.TicketID)
	repairs := loadCandidateRepairs(db, candidate.TicketID)
	preservedStatus := preserveKnowledgeCandidateReviewState(candidate)
	if err := s.PrepareCandidateDB(db, candidate, ticket, repairs); err != nil {
		return err
	}
	preservedStatus.restore(candidate)
	candidate.RequiresReassessment = candidateKnowledgeNeedsReassessment(candidate)
	if candidate.RequiresReassessment {
		candidate.ReviewRemark = "客户反馈或验证结果发生变化，已暂停知识条目并要求重新评估"
	}
	if err := db.Model(&models.KnowledgeCandidate{}).Where("id = ?", candidate.ID).
		Updates(knowledgeCandidateScoreUpdates(candidate)).Error; err != nil {
		return err
	}
	if err := s.RefreshDuplicateGroupDB(db, candidate.SimilarityHash); err != nil {
		return err
	}
	if previousSimilarityHash != "" && previousSimilarityHash != candidate.SimilarityHash {
		return s.RefreshDuplicateGroupDB(db, previousSimilarityHash)
	}
	return nil
}

// RescoreStaleCandidatesDB updates scoring-version drift before queue filters
// are applied, so candidates cannot remain hidden under an obsolete status.
func (s *knowledgeCandidateScoringService) RescoreStaleCandidatesDB(db *gorm.DB, tenantID, productID int64) error {
	if db == nil || tenantID <= 0 || !db.Migrator().HasTable(&models.KnowledgeCandidate{}) {
		return nil
	}
	query := db.Where("tenant_id = ? AND status <> ? AND score_version <> ?", tenantID, enums.StatusDeleted, enums.KnowledgeCandidateScoreVersion)
	if productID > 0 {
		query = query.Where("product_id = ?", productID)
	}
	var candidates []models.KnowledgeCandidate
	if err := query.Order("id ASC").Find(&candidates).Error; err != nil {
		return err
	}
	for i := range candidates {
		if err := s.RescoreCandidateDB(db, &candidates[i]); err != nil {
			return err
		}
	}
	return nil
}

// RescoreTicketCandidatesDB refreshes all active candidates affected by ticket
// evidence changes such as customer feedback or repair verification updates.
func (s *knowledgeCandidateScoringService) RescoreTicketCandidatesDB(db *gorm.DB, tenantID, ticketID int64) error {
	if db == nil || ticketID <= 0 {
		return nil
	}
	var candidates []models.KnowledgeCandidate
	if err := db.Where("tenant_id = ? AND ticket_id = ? AND status <> ?", tenantID, ticketID, enums.StatusDeleted).
		Order("id ASC").Find(&candidates).Error; err != nil {
		return err
	}
	for i := range candidates {
		if err := s.RescoreCandidateDB(db, &candidates[i]); err != nil {
			return err
		}
	}
	return nil
}

// BackfillDB scores active legacy candidates during migration.
func (s *knowledgeCandidateScoringService) BackfillDB(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.KnowledgeCandidate{}) {
		return nil
	}
	var candidates []models.KnowledgeCandidate
	if err := db.Where("status <> ?", enums.StatusDeleted).Order("id ASC").Find(&candidates).Error; err != nil {
		return err
	}
	groups := make(map[string]struct{})
	for i := range candidates {
		candidate := &candidates[i]
		ticket := loadCandidateTicket(db, candidate.TenantID, candidate.TicketID)
		repairs := loadCandidateRepairs(db, candidate.TicketID)
		preservedStatus := preserveKnowledgeCandidateReviewState(candidate)
		if err := s.PrepareCandidateDB(db, candidate, ticket, repairs); err != nil {
			return err
		}
		preservedStatus.restore(candidate)
		candidate.RequiresReassessment = candidateKnowledgeNeedsReassessment(candidate)
		if candidate.RequiresReassessment {
			candidate.ReviewRemark = "客户反馈或验证结果发生变化，已暂停知识条目并要求重新评估"
		}
		if err := db.Model(&models.KnowledgeCandidate{}).Where("id = ?", candidate.ID).
			Updates(knowledgeCandidateScoreUpdates(candidate)).Error; err != nil {
			return err
		}
		if candidate.SimilarityHash != "" {
			groups[candidate.SimilarityHash] = struct{}{}
		}
	}
	for similarityHash := range groups {
		if err := s.RefreshDuplicateGroupDB(db, similarityHash); err != nil {
			return err
		}
	}
	return nil
}

func (s *knowledgeCandidateScoringService) assessCandidate(db *gorm.DB, candidate *models.KnowledgeCandidate, ticket *models.Ticket, repairs []models.TicketRepairRecord, recurrenceCount, affectedDeviceCount int) knowledgeCandidateAssessment {
	result := knowledgeCandidateAssessment{}
	flags := make([]string, 0, 6)
	addFlag := func(flag string) {
		for _, current := range flags {
			if current == flag {
				return
			}
		}
		flags = append(flags, flag)
	}

	if meaningfulKnowledgeText(candidate.RootCauseSummary) {
		result.Breakdown.EvidenceCompleteness += scoreByRuneLength(candidate.RootCauseSummary, 4, 10, 20)
	} else {
		addFlag("missing_root_cause")
	}
	if meaningfulKnowledgeText(candidate.SolutionSummary) {
		result.Breakdown.EvidenceCompleteness += scoreByRuneLength(candidate.SolutionSummary, 8, 10, 20)
	} else {
		addFlag("missing_solution")
	}
	if meaningfulKnowledgeText(candidate.Suggestion) {
		result.Breakdown.EvidenceCompleteness += scoreByRuneLength(candidate.Suggestion, 8, 5, 10)
	} else {
		addFlag("missing_resolution_summary")
	}

	verified := latestVerifiedKnowledgeRepair(repairs)
	if verified == nil {
		addFlag("missing_verification")
	} else {
		switch strings.ToLower(strings.TrimSpace(verified.TestResult)) {
		case "passed":
			result.Breakdown.OutcomeConfidence += 20
		case "partial":
			result.Breakdown.OutcomeConfidence += 5
			addFlag("partial_verification")
		case "failed":
			result.Breakdown.OutcomeConfidence -= 25
			addFlag("verification_failed")
		default:
			addFlag("missing_verification")
		}
	}
	feedback := loadLatestCandidateFeedback(db, candidate.TenantID, candidate.TicketID)
	if feedback != nil && ticket != nil && ticket.ResolvedAt != nil && feedback.SubmittedAt.Before(*ticket.ResolvedAt) {
		feedback = nil
	}
	if feedback != nil {
		switch {
		case feedback.Rating >= 4:
			result.Breakdown.OutcomeConfidence += 10
		case feedback.Rating == 3:
			result.Breakdown.OutcomeConfidence += 3
		case feedback.Rating > 0:
			result.Breakdown.OutcomeConfidence -= 10
			addFlag("negative_customer_feedback")
		}
	}
	if ticket != nil {
		status := enums.NormalizeTicketStatus(string(ticket.Status))
		if status == enums.TicketStatusClosed || status == enums.TicketStatusDone {
			result.Breakdown.OutcomeConfidence += 5
		}
	}

	if candidate.ProductID > 0 {
		result.Breakdown.ContextCompleteness += 5
	} else {
		addFlag("missing_product_context")
	}
	if candidate.ProductModelID > 0 {
		result.Breakdown.ContextCompleteness += 3
	}
	if ticket != nil && ticket.DeviceID > 0 {
		result.Breakdown.ContextCompleteness += 3
	}
	if ticket != nil && strings.TrimSpace(ticket.FaultCode) != "" {
		result.Breakdown.ContextCompleteness += 4
	} else {
		addFlag("missing_fault_code")
	}

	if meaningfulKnowledgeText(candidate.Title) {
		result.Breakdown.ContentUsability = scoreByRuneLength(candidate.Title, 6, 2, 5)
	} else {
		addFlag("generic_title")
	}

	result.QualityScore = clampScore(
		result.Breakdown.EvidenceCompleteness +
			result.Breakdown.OutcomeConfidence +
			result.Breakdown.ContextCompleteness +
			result.Breakdown.ContentUsability,
	)

	result.Breakdown.RecurrenceValue = minKnowledgeCandidateInt(maxKnowledgeCandidateInt(recurrenceCount-1, 0)*15, 30)
	if affectedDeviceCount > 0 {
		result.Breakdown.ImpactValue = 5 + minKnowledgeCandidateInt(maxKnowledgeCandidateInt(affectedDeviceCount-1, 0)*5, 10)
	}
	result.Breakdown.KnowledgeGapValue = knowledgeCandidateGapValue(db, candidate, ticket)
	result.Breakdown.SeverityValue = knowledgeCandidateSeverityValue(ticket)
	if feedback != nil {
		if feedback.Rating > 0 && feedback.Rating <= 2 {
			result.Breakdown.ImpactValue += 10
		} else if feedback.Rating == 3 {
			result.Breakdown.ImpactValue += 5
		}
	}
	result.ValueScore = clampScore(
		result.Breakdown.RecurrenceValue +
			result.Breakdown.ImpactValue +
			result.Breakdown.SeverityValue +
			result.Breakdown.KnowledgeGapValue,
	)
	result.CandidateScore = (result.QualityScore*65 + result.ValueScore*35 + 50) / 100
	result.Flags = flags
	result.ReviewStatus, result.ReviewRemark = classifyKnowledgeCandidate(result)
	return result
}

func (s *knowledgeCandidateScoringService) applyAssessment(candidate *models.KnowledgeCandidate, assessment knowledgeCandidateAssessment, preserveStatus bool) {
	now := time.Now()
	breakdownJSON, _ := json.Marshal(assessment.Breakdown)
	flagsJSON, _ := json.Marshal(assessment.Flags)
	candidate.QualityScore = assessment.QualityScore
	candidate.ValueScore = assessment.ValueScore
	candidate.CandidateScore = assessment.CandidateScore
	candidate.ScoreBreakdownJSON = string(breakdownJSON)
	candidate.QualityFlagsJSON = string(flagsJSON)
	candidate.ScoreVersion = enums.KnowledgeCandidateScoreVersion
	candidate.ScoredAt = &now
	if candidate.RecurrenceCount <= 0 {
		candidate.RecurrenceCount = 1
	}
	if !preserveStatus {
		candidate.ReviewStatus = assessment.ReviewStatus
		candidate.ReviewRemark = assessment.ReviewRemark
		candidate.ReviewedAt = nil
		candidate.ReviewerID = 0
	}
	candidate.RecurrenceCount = maxKnowledgeCandidateInt(candidate.RecurrenceCount, 1)
}

func (s *knowledgeCandidateScoringService) loadGroupCandidates(db *gorm.DB, candidate *models.KnowledgeCandidate) ([]models.KnowledgeCandidate, error) {
	if candidate.SimilarityHash == "" {
		return nil, nil
	}
	query := db.Where("tenant_id = ? AND product_id = ? AND product_model_id = ? AND similarity_hash = ? AND status <> ? AND review_status <> ?",
		candidate.TenantID, candidate.ProductID, candidate.ProductModelID, candidate.SimilarityHash, enums.StatusDeleted, enums.KnowledgeCandidateReviewStatusRejected)
	if candidate.ID > 0 {
		query = query.Where("id <> ?", candidate.ID)
	}
	var group []models.KnowledgeCandidate
	if err := query.Order("id ASC").Find(&group).Error; err != nil {
		return nil, err
	}
	return group, nil
}

func selectKnowledgeCandidatePrimary(group []models.KnowledgeCandidate) *models.KnowledgeCandidate {
	eligible := make([]models.KnowledgeCandidate, 0, len(group))
	for i := range group {
		if !group[i].RequiresReassessment {
			eligible = append(eligible, group[i])
		}
	}
	if len(eligible) == 0 {
		return nil
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		leftRank := knowledgeCandidatePrimaryRank(eligible[i])
		rightRank := knowledgeCandidatePrimaryRank(eligible[j])
		if leftRank != rightRank {
			return leftRank > rightRank
		}
		if eligible[i].CandidateScore != eligible[j].CandidateScore {
			return eligible[i].CandidateScore > eligible[j].CandidateScore
		}
		return eligible[i].ID < eligible[j].ID
	})
	return &eligible[0]
}

func knowledgeCandidatePrimaryRank(candidate models.KnowledgeCandidate) int {
	switch candidate.ReviewStatus {
	case string(enums.KnowledgeCandidateReviewStatusApproved), string(enums.KnowledgeCandidateReviewStatusMerged):
		return 4
	case string(enums.KnowledgeCandidateReviewStatusPending):
		if candidate.MergedToCandidateID == 0 {
			return 3
		}
		return 1
	default:
		if candidate.MergedToCandidateID == 0 {
			return 2
		}
		return 1
	}
}

func loadKnowledgeCandidateGroupStats(db *gorm.DB, group []models.KnowledgeCandidate, ticket *models.Ticket) (int, int, error) {
	ticketIDs := make([]int64, 0, len(group)+1)
	seenTickets := make(map[int64]struct{}, len(group)+1)
	for i := range group {
		if group[i].TicketID > 0 {
			seenTickets[group[i].TicketID] = struct{}{}
		}
	}
	if ticket != nil && ticket.ID > 0 {
		seenTickets[ticket.ID] = struct{}{}
	}
	for ticketID := range seenTickets {
		ticketIDs = append(ticketIDs, ticketID)
	}
	deviceIDs := make(map[int64]struct{})
	if len(ticketIDs) > 0 && db.Migrator().HasTable(&models.Ticket{}) {
		var tickets []models.Ticket
		if err := db.Select("id", "device_id").Where("id IN ?", ticketIDs).Find(&tickets).Error; err != nil {
			return 0, 0, err
		}
		for i := range tickets {
			if tickets[i].DeviceID > 0 {
				deviceIDs[tickets[i].DeviceID] = struct{}{}
			}
		}
	}
	recurrenceCount := len(seenTickets)
	if recurrenceCount == 0 {
		recurrenceCount = 1
	}
	return recurrenceCount, len(deviceIDs), nil
}

func classifyKnowledgeCandidate(result knowledgeCandidateAssessment) (string, string) {
	if containsString(result.Flags, "verification_failed") || result.QualityScore < 60 {
		return string(enums.KnowledgeCandidateReviewStatusLowQuality), fmt.Sprintf("质量分 %d，未达到最低入池标准 60", result.QualityScore)
	}
	if result.QualityScore < enums.KnowledgeCandidateMinimumQualityScore ||
		containsString(result.Flags, "missing_root_cause") ||
		containsString(result.Flags, "missing_solution") ||
		containsString(result.Flags, "missing_verification") ||
		containsString(result.Flags, "partial_verification") ||
		containsString(result.Flags, "negative_customer_feedback") {
		return string(enums.KnowledgeCandidateReviewStatusNeedsEnrichment), "待补充：" + strings.Join(knowledgeCandidateFlagLabels(result.Flags), "、")
	}
	if result.ValueScore < enums.KnowledgeCandidateMinimumValueScore {
		return string(enums.KnowledgeCandidateReviewStatusLowValue), fmt.Sprintf("价值分 %d，未达到审核入池标准 %d", result.ValueScore, enums.KnowledgeCandidateMinimumValueScore)
	}
	return string(enums.KnowledgeCandidateReviewStatusPending), ""
}

func knowledgeCandidateGapValue(db *gorm.DB, candidate *models.KnowledgeCandidate, ticket *models.Ticket) int {
	if db == nil || candidate == nil {
		return 0
	}
	knowledgeBaseID := candidate.KnowledgeBaseID
	if knowledgeBaseID <= 0 && candidate.ProductID > 0 && db.Migrator().HasTable(&models.ProductServiceProfile{}) {
		var profile models.ProductServiceProfile
		if result := db.Where("tenant_id = ? AND product_id = ? AND status <> ?", candidate.TenantID, candidate.ProductID, enums.StatusDeleted).First(&profile); result.Error == nil {
			knowledgeBaseID = profile.DefaultKnowledgeBaseID
		}
	}
	faultCode := ""
	if ticket != nil {
		faultCode = normalizeKnowledgeCandidateIdentity(ticket.FaultCode)
	}
	candidateText := strings.TrimSpace(candidate.Title + " " + candidate.RootCauseSummary + " " + candidate.SolutionSummary)
	hasEquivalent := false
	if db.Migrator().HasTable(&models.KnowledgeDocument{}) {
		lastID := int64(0)
		for !hasEquivalent {
			var documents []models.KnowledgeDocument
			query := db.Where("tenant_id = ? AND status = ? AND review_status = ? AND published_revision_id > 0", candidate.TenantID, enums.StatusOk, "published")
			if candidate.ID > 0 {
				query = query.Where("NOT (source_type IN ? AND source_reference_id = ?)", []string{"knowledge_candidate", "knowledge_entry"}, candidate.ID)
			}
			if knowledgeBaseID > 0 {
				query = query.Where("knowledge_base_id = ?", knowledgeBaseID)
			}
			if lastID > 0 {
				query = query.Where("id < ?", lastID)
			}
			if err := query.Order("id DESC").Limit(500).Find(&documents).Error; err != nil {
				return 0
			}
			for i := range documents {
				if knowledgeCandidateEntryEquivalent(faultCode, candidateText, documents[i].Title+" "+documents[i].Content, documents[i].FaultCodesJSON) {
					hasEquivalent = true
					break
				}
			}
			if len(documents) < 500 {
				break
			}
			lastID = documents[len(documents)-1].ID
		}
	}
	if !hasEquivalent && db.Migrator().HasTable(&models.KnowledgeFAQ{}) {
		lastID := int64(0)
		for !hasEquivalent {
			var faqs []models.KnowledgeFAQ
			query := db.Where("tenant_id = ? AND status = ? AND review_status = ? AND published_revision_id > 0", candidate.TenantID, enums.StatusOk, "published")
			if knowledgeBaseID > 0 {
				query = query.Where("knowledge_base_id = ?", knowledgeBaseID)
			}
			if lastID > 0 {
				query = query.Where("id < ?", lastID)
			}
			if err := query.Order("id DESC").Limit(500).Find(&faqs).Error; err != nil {
				return 0
			}
			for i := range faqs {
				if knowledgeCandidateEntryEquivalent(faultCode, candidateText, faqs[i].Question+" "+faqs[i].Answer, faqs[i].FaultCodesJSON) {
					hasEquivalent = true
					break
				}
			}
			if len(faqs) < 500 {
				break
			}
			lastID = faqs[len(faqs)-1].ID
		}
	}
	if hasEquivalent {
		return 0
	}
	return 25
}

func knowledgeCandidateEntryEquivalent(faultCode, candidateText, entryText, faultCodesJSON string) bool {
	similarity := knowledgeCandidateTextSimilarity(candidateText, entryText)
	if faultCode != "" && knowledgeFaultCodeJSONContains(faultCodesJSON, faultCode) {
		return similarity >= 0.35
	}
	return similarity >= 0.72
}

func knowledgeFaultCodeJSONContains(faultCodesJSON, target string) bool {
	target = normalizeKnowledgeCandidateIdentity(target)
	if target == "" {
		return false
	}
	for _, faultCode := range knowledgeStringSlice(faultCodesJSON) {
		if normalizeKnowledgeCandidateIdentity(faultCode) == target {
			return true
		}
	}
	return false
}

func knowledgeCandidateTextSimilarity(left, right string) float64 {
	leftBigrams := knowledgeCandidateBigrams(normalizeKnowledgeCandidateIdentity(left))
	rightBigrams := knowledgeCandidateBigrams(normalizeKnowledgeCandidateIdentity(right))
	if len(leftBigrams) == 0 || len(rightBigrams) == 0 {
		return 0
	}
	intersection := 0
	for value := range leftBigrams {
		if _, ok := rightBigrams[value]; ok {
			intersection++
		}
	}
	return float64(2*intersection) / float64(len(leftBigrams)+len(rightBigrams))
}

func knowledgeCandidateBigrams(value string) map[string]struct{} {
	runes := []rune(value)
	result := make(map[string]struct{})
	if len(runes) == 1 {
		result[string(runes)] = struct{}{}
		return result
	}
	for i := 0; i+1 < len(runes); i++ {
		result[string(runes[i:i+2])] = struct{}{}
	}
	return result
}

func knowledgeCandidateFlagLabels(flags []string) []string {
	labels := make([]string, 0, len(flags))
	for _, flag := range flags {
		label := map[string]string{
			"missing_root_cause":         "缺少明确根因",
			"missing_solution":           "缺少可执行方案",
			"missing_resolution_summary": "缺少处理结论",
			"missing_verification":       "缺少验证结果",
			"partial_verification":       "仅部分验证通过",
			"verification_failed":        "验证失败",
			"negative_customer_feedback": "客户反馈未解决",
			"missing_product_context":    "缺少产品上下文",
			"missing_fault_code":         "缺少故障码",
			"generic_title":              "标题过于笼统",
		}[flag]
		if label != "" {
			labels = append(labels, label)
		}
	}
	if len(labels) == 0 {
		labels = append(labels, "内容证据不足")
	}
	return labels
}

func knowledgeCandidateSeverityValue(ticket *models.Ticket) int {
	if ticket == nil {
		return 10
	}
	switch strings.ToLower(strings.TrimSpace(ticket.PriorityCode)) {
	case "p0":
		return 20
	case "p1":
		return 15
	case "p3":
		return 5
	case "p4":
		return 2
	default:
		return 10
	}
}

func buildKnowledgeCandidateSimilarityHash(candidate *models.KnowledgeCandidate, ticket *models.Ticket) string {
	if candidate == nil {
		return ""
	}
	faultCode := ""
	if ticket != nil {
		faultCode = ticket.FaultCode
	}
	identity := normalizeKnowledgeCandidateIdentity(faultCode) + "|" + normalizeKnowledgeCandidateIdentity(candidate.RootCauseSummary)
	if strings.Trim(identity, "|") == "" {
		identity = normalizeKnowledgeCandidateIdentity(candidate.Title) + "|" + normalizeKnowledgeCandidateIdentity(candidate.SolutionSummary)
	}
	if strings.Trim(identity, "|") == "" {
		return ""
	}
	payload := fmt.Sprintf("%d|%d|%d|%s", candidate.TenantID, candidate.ProductID, candidate.ProductModelID, identity)
	if candidate.DeduplicationOverride {
		payload += fmt.Sprintf("|distinct|%d", candidate.ID)
	}
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func normalizeKnowledgeCandidateIdentity(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, current := range value {
		if unicode.IsLetter(current) || unicode.IsNumber(current) {
			builder.WriteRune(current)
		}
	}
	return builder.String()
}

func meaningfulKnowledgeText(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "", "-", "无", "未知", "待补充", "n/a", "na", "维修知识候选":
		return false
	default:
		return true
	}
}

func scoreByRuneLength(value string, completeLength, partialScore, completeScore int) int {
	if utf8.RuneCountInString(strings.TrimSpace(value)) >= completeLength {
		return completeScore
	}
	return partialScore
}

func loadCandidateTicket(db *gorm.DB, tenantID, ticketID int64) *models.Ticket {
	if db == nil || ticketID <= 0 || !db.Migrator().HasTable(&models.Ticket{}) {
		return nil
	}
	var ticket models.Ticket
	query := db.Where("id = ?", ticketID)
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if result := query.First(&ticket); result.Error != nil {
		return nil
	}
	return &ticket
}

func loadCandidateRepairs(db *gorm.DB, ticketID int64) []models.TicketRepairRecord {
	if db == nil || ticketID <= 0 || !db.Migrator().HasTable(&models.TicketRepairRecord{}) {
		return nil
	}
	var repairs []models.TicketRepairRecord
	_ = db.Where("ticket_id = ?", ticketID).Order("id ASC").Find(&repairs).Error
	return repairs
}

func loadLatestCandidateFeedback(db *gorm.DB, tenantID, ticketID int64) *models.TicketFeedback {
	if db == nil || ticketID <= 0 || !db.Migrator().HasTable(&models.TicketFeedback{}) {
		return nil
	}
	var feedback models.TicketFeedback
	query := db.Where("ticket_id = ?", ticketID)
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	result := query.Order("submitted_at DESC, id DESC").Limit(1).Find(&feedback)
	if result.Error != nil || result.RowsAffected == 0 {
		return nil
	}
	return &feedback
}

func knowledgeCandidateScoreUpdates(candidate *models.KnowledgeCandidate) map[string]any {
	return map[string]any{
		"quality_score":          candidate.QualityScore,
		"value_score":            candidate.ValueScore,
		"candidate_score":        candidate.CandidateScore,
		"score_breakdown_json":   candidate.ScoreBreakdownJSON,
		"quality_flags_json":     candidate.QualityFlagsJSON,
		"score_version":          candidate.ScoreVersion,
		"scored_at":              candidate.ScoredAt,
		"similarity_hash":        candidate.SimilarityHash,
		"duplicate_group_id":     candidate.DuplicateGroupID,
		"merged_to_candidate_id": candidate.MergedToCandidateID,
		"deduplication_override": candidate.DeduplicationOverride,
		"recurrence_count":       candidate.RecurrenceCount,
		"affected_device_count":  candidate.AffectedDeviceCount,
		"requires_reassessment":  candidate.RequiresReassessment,
		"review_status":          candidate.ReviewStatus,
		"review_remark":          candidate.ReviewRemark,
		"reviewed_at":            candidate.ReviewedAt,
		"reviewer_id":            candidate.ReviewerID,
		"updated_at":             time.Now(),
	}
}

func candidateKnowledgeNeedsReassessment(candidate *models.KnowledgeCandidate) bool {
	if candidate == nil || candidate.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusApproved) {
		return false
	}
	if candidate.QualityScore < enums.KnowledgeCandidateMinimumQualityScore || candidate.ValueScore < enums.KnowledgeCandidateMinimumValueScore {
		return true
	}
	var flags []string
	_ = json.Unmarshal([]byte(candidate.QualityFlagsJSON), &flags)
	return containsString(flags, "negative_customer_feedback") ||
		containsString(flags, "verification_failed") ||
		containsString(flags, "missing_verification") ||
		containsString(flags, "partial_verification")
}

func isTerminalKnowledgeCandidateStatus(status string) bool {
	switch status {
	case string(enums.KnowledgeCandidateReviewStatusApproved),
		string(enums.KnowledgeCandidateReviewStatusRejected),
		string(enums.KnowledgeCandidateReviewStatusMerged):
		return true
	default:
		return false
	}
}

type knowledgeCandidateReviewState struct {
	preserve            bool
	reviewStatus        string
	reviewRemark        string
	reviewedAt          *time.Time
	reviewerID          int64
	mergedToCandidateID int64
}

func preserveKnowledgeCandidateReviewState(candidate *models.KnowledgeCandidate) knowledgeCandidateReviewState {
	if candidate == nil || !isTerminalKnowledgeCandidateStatus(candidate.ReviewStatus) {
		return knowledgeCandidateReviewState{}
	}
	return knowledgeCandidateReviewState{
		preserve:            true,
		reviewStatus:        candidate.ReviewStatus,
		reviewRemark:        candidate.ReviewRemark,
		reviewedAt:          candidate.ReviewedAt,
		reviewerID:          candidate.ReviewerID,
		mergedToCandidateID: candidate.MergedToCandidateID,
	}
}

func (state knowledgeCandidateReviewState) restore(candidate *models.KnowledgeCandidate) {
	if !state.preserve || candidate == nil {
		return
	}
	candidate.ReviewStatus = state.reviewStatus
	candidate.ReviewRemark = state.reviewRemark
	candidate.ReviewedAt = state.reviewedAt
	candidate.ReviewerID = state.reviewerID
	candidate.MergedToCandidateID = state.mergedToCandidateID
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func clampScore(value int) int {
	return minKnowledgeCandidateInt(maxKnowledgeCandidateInt(value, 0), 100)
}

func minKnowledgeCandidateInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxKnowledgeCandidateInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
