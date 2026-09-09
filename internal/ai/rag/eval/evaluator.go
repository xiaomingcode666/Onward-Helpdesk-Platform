package eval

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/ai/rag"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/services"
)

type RetrieveFunc func(ctx context.Context, req services.KnowledgeRetrievalRequest) (*services.KnowledgeRetrievalResult, error)

type Evaluator struct {
	Retrieve       RetrieveFunc
	GenerateAnswer GenerateAnswerFunc
	Now            func() time.Time
}

func NewEvaluator() *Evaluator {
	return &Evaluator{
		Retrieve:       services.KnowledgeRetrievalService.Retrieve,
		GenerateAnswer: DefaultGenerateAnswer,
		Now:            time.Now,
	}
}

func (e *Evaluator) Evaluate(ctx context.Context, suite *Suite, refs *Refs, opts RunOptions) (*Report, error) {
	if suite == nil {
		return nil, fmt.Errorf("suite is required")
	}
	if refs == nil {
		return nil, fmt.Errorf("refs is required")
	}
	if err := suite.Validate(); err != nil {
		return nil, err
	}
	if e == nil || e.Retrieve == nil {
		return nil, fmt.Errorf("retrieve function is required")
	}
	if e.Now == nil {
		e.Now = time.Now
	}

	cases := suite.NormalizedCases()
	report := &Report{
		SuiteName:    suite.Name,
		SuiteVersion: suite.Version,
		GeneratedAt:  e.Now(),
		Options:      normalizedRunOptions(opts),
		Cases:        make([]CaseReport, 0, len(cases)),
	}

	for _, item := range cases {
		caseReport := CaseReport{
			ID:         item.ID,
			Name:       item.Name,
			Enabled:    item.Enabled == nil || *item.Enabled,
			Query:      item.Input.Query,
			Tags:       append([]string(nil), item.Tags...),
			IntentTags: append([]string(nil), item.IntentTags...),
			FaultCodes: append([]string(nil), item.FaultCodes...),
			Scope: CaseScopeReport{
				TenantRef:         item.Scope.TenantRef,
				ProductRef:        item.Scope.ProductRef,
				ProductModelRef:   item.Scope.ProductModelRef,
				KnowledgeBaseRefs: append([]string(nil), item.Scope.KnowledgeBaseRefs...),
				Locale:            item.Scope.Locale,
				RegionCode:        item.Scope.RegionCode,
				Audience:          item.Scope.Audience,
			},
		}
		report.Summary.TotalCases++
		if !caseReport.Enabled {
			report.Cases = append(report.Cases, caseReport)
			continue
		}
		report.Summary.EnabledCases++

		resolved, err := refs.ResolveCase(item)
		if err != nil {
			caseReport.Error = err.Error()
			caseReport.Failures = append(caseReport.Failures, "case refs resolution failed")
			report.Cases = append(report.Cases, finalizeCaseReport(caseReport))
			continue
		}
		caseReport.Scope.TenantID = resolved.TenantID
		caseReport.Scope.ProductID = resolved.ProductID
		caseReport.Scope.ProductModelID = resolved.ProductModelID
		caseReport.Scope.KnowledgeBaseIDs = append([]int64(nil), resolved.KnowledgeBaseIDs...)

		runCtx := ctx
		cancel := func() {}
		if report.Options.CaseTimeout > 0 {
			runCtx, cancel = context.WithTimeout(ctx, report.Options.CaseTimeout)
		}
		runCtx = ai.WithCapabilityScope(runCtx, ai.CapabilityScope{
			TenantID:        resolved.TenantID,
			ProductID:       resolved.ProductID,
			KnowledgeBaseID: firstResolvedKnowledgeBaseID(resolved.KnowledgeBaseIDs),
		})
		retrievalStartedAt := time.Now()
		retrieval, retrieveErr := e.Retrieve(runCtx, services.KnowledgeRetrievalRequest{
			Scope: dto.KnowledgeScopeContext{
				TenantID:       resolved.TenantID,
				ProductID:      resolved.ProductID,
				ProductModelID: resolved.ProductModelID,
				Locale:         item.Scope.Locale,
				RegionCode:     item.Scope.RegionCode,
				Audience:       item.Scope.Audience,
			},
			AllowedKnowledgeBaseIDs: append([]int64(nil), resolved.KnowledgeBaseIDs...),
			Query:                   item.Input.Query,
			TopK:                    report.Options.TopK,
			ScoreThreshold:          report.Options.ScoreThreshold,
			RerankTopN:              report.Options.RerankTopN,
			ContextMaxTokens:        report.Options.ContextMaxTokens,
			Channel:                 report.Options.Channel,
			Scene:                   report.Options.Scene,
			WriteRetrieveLog:        false,
		})
		caseReport.Retrieval.LatencyMs = time.Since(retrievalStartedAt).Milliseconds()
		if retrieveErr != nil {
			cancel()
			caseReport.Error = retrieveErr.Error()
			caseReport.Failures = append(caseReport.Failures, "retrieval execution failed")
			report.Cases = append(report.Cases, finalizeCaseReport(caseReport))
			continue
		}

		populateRetrievalReport(&caseReport, item, resolved, retrieval)
		evaluateRetrievalAssertions(&caseReport, item)

		if report.Options.GenerateAnswer {
			report.Summary.AnswerEvaluated = true
			answerReport := evaluateAnswer(runCtx, e.GenerateAnswer, item, retrieval, caseReport.Retrieval.CitationEntryKeys, caseReport.Retrieval.HitEntryKeys)
			caseReport.Answer = answerReport
			evaluateAnswerAssertions(&caseReport, item)
		} else if hasAnswerAssertions(item) {
			caseReport.SkippedAssertions = append(caseReport.SkippedAssertions, "answer assertions skipped (run with -with-answer to evaluate)")
		}
		cancel()

		report.Cases = append(report.Cases, finalizeCaseReport(caseReport))
	}

	buildReportSummary(report)
	return report, nil
}

func firstResolvedKnowledgeBaseID(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	return values[0]
}

func normalizedRunOptions(opts RunOptions) RunOptions {
	if opts.TopK <= 0 {
		opts.TopK = 10
	}
	if opts.TopK < 10 {
		opts.TopK = 10
	}
	if opts.ScoreThreshold <= 0 {
		opts.ScoreThreshold = 0.3
	}
	if strings.TrimSpace(opts.Channel) == "" {
		opts.Channel = "rag_eval"
	}
	if strings.TrimSpace(opts.Scene) == "" {
		opts.Scene = "rag_eval"
	}
	return opts
}

func populateRetrievalReport(caseReport *CaseReport, item Case, resolved *ResolvedCase, retrieval *services.KnowledgeRetrievalResult) {
	if caseReport == nil {
		return
	}
	caseReport.Retrieval.ExpectedEntryKeys = append([]string(nil), resolved.ExpectedEntryKeys...)
	caseReport.Retrieval.ForbiddenEntryKeys = append([]string(nil), resolved.ForbiddenEntryKeys...)
	caseReport.Retrieval.ExpectedHitCount = len(resolved.ExpectedEntryKeys)
	caseReport.Retrieval.MinExpectedHitsAt5 = item.RetrievalExpectation.MinExpectedHitsAt5
	caseReport.Retrieval.MinExpectedHitsAt10 = item.RetrievalExpectation.MinExpectedHitsAt10
	caseReport.Retrieval.CitationRequired = boolValue(item.AnswerExpectation.CitationRequired)
	caseReport.Retrieval.MinCitationCount = item.AnswerExpectation.MinCitationCount

	if retrieval == nil {
		return
	}
	caseReport.Retrieval.NoAnswerReason = retrieval.NoAnswerReason
	caseReport.Retrieval.RequestedMode = retrieval.Strategy.RequestedMode
	caseReport.Retrieval.AppliedMode = retrieval.Strategy.AppliedMode
	caseReport.Retrieval.TopScore = retrieval.TopScore
	caseReport.Retrieval.TriggeredByExactTerms = append([]string(nil), retrieval.Strategy.ExactMatchTerms...)
	caseReport.Retrieval.HitEntryKeys = orderedHitEntryKeys(retrieval.RerankedHits)
	caseReport.Retrieval.ContextEntryKeys = orderedHitEntryKeys(retrieval.ContextHits)
	caseReport.Retrieval.CitationEntryKeys = orderedCitationEntryKeys(retrieval.Citations)
	caseReport.Retrieval.ExpectedHitsAt5 = countHitsWithin(caseReport.Retrieval.HitEntryKeys, resolved.ExpectedEntryKeys, 5)
	caseReport.Retrieval.ExpectedHitsAt10 = countHitsWithin(caseReport.Retrieval.HitEntryKeys, resolved.ExpectedEntryKeys, 10)
	caseReport.Retrieval.RecallAt5 = ratio(caseReport.Retrieval.ExpectedHitsAt5, len(resolved.ExpectedEntryKeys))
	caseReport.Retrieval.RecallAt10 = ratio(caseReport.Retrieval.ExpectedHitsAt10, len(resolved.ExpectedEntryKeys))
	caseReport.Retrieval.MRR = reciprocalRank(caseReport.Retrieval.HitEntryKeys, resolved.ExpectedEntryKeys)
	caseReport.Retrieval.ForbiddenHitCount = countIntersect(caseReport.Retrieval.HitEntryKeys, resolved.ForbiddenEntryKeys)
	caseReport.Retrieval.ActualCitationCount = len(caseReport.Retrieval.CitationEntryKeys)
	caseReport.Retrieval.CitationSatisfied = evaluateCitationSatisfied(item, resolved, caseReport.Retrieval.CitationEntryKeys)
	caseReport.Retrieval.MatchedExpectedKeys, caseReport.Retrieval.MissingExpectedKeys = splitExpectedMatches(caseReport.Retrieval.HitEntryKeys, resolved.ExpectedEntryKeys)
}

func evaluateRetrievalAssertions(caseReport *CaseReport, item Case) {
	if caseReport == nil {
		return
	}
	if caseReport.Retrieval.ExpectedHitsAt5 < item.RetrievalExpectation.MinExpectedHitsAt5 {
		caseReport.Failures = append(caseReport.Failures, fmt.Sprintf("expected at least %d hit(s) within top5, got %d", item.RetrievalExpectation.MinExpectedHitsAt5, caseReport.Retrieval.ExpectedHitsAt5))
	}
	if caseReport.Retrieval.ExpectedHitsAt10 < item.RetrievalExpectation.MinExpectedHitsAt10 {
		caseReport.Failures = append(caseReport.Failures, fmt.Sprintf("expected at least %d hit(s) within top10, got %d", item.RetrievalExpectation.MinExpectedHitsAt10, caseReport.Retrieval.ExpectedHitsAt10))
	}
	if caseReport.Retrieval.ForbiddenHitCount > 0 {
		caseReport.Failures = append(caseReport.Failures, fmt.Sprintf("forbidden hit count must be 0, got %d", caseReport.Retrieval.ForbiddenHitCount))
	}
	if caseReport.Retrieval.CitationRequired && !caseReport.Retrieval.CitationSatisfied {
		caseReport.Failures = append(caseReport.Failures, "citation requirement not satisfied")
	}
}

func evaluateAnswer(runCtx context.Context, fn GenerateAnswerFunc, item Case, retrieval *services.KnowledgeRetrievalResult, citationEntryKeys []string, hitEntryKeys []string) *CaseAnswerReport {
	report := &CaseAnswerReport{
		ExpectedShouldAnswer:   item.AnswerExpectation.ShouldAnswer,
		ExpectHandoff:          item.AnswerExpectation.ExpectHandoff,
		MustContain:            append([]string(nil), item.AnswerExpectation.MustContain...),
		MustNotContain:         append([]string(nil), item.AnswerExpectation.MustNotContain...),
		AllowedNoAnswerReasons: append([]string(nil), item.AnswerExpectation.AllowedNoAnswerReasons...),
	}
	if fn == nil {
		return report
	}
	answer, err := fn(runCtx, item, retrieval)
	if err != nil {
		report.Content = err.Error()
		return report
	}
	if answer == nil {
		return report
	}
	report.Generated = answer.Generated
	report.LatencyMs = answer.LatencyMs
	report.ModelName = answer.ModelName
	report.PromptTokens = answer.PromptTokens
	report.CompletionTokens = answer.CompletionTokens
	report.Content = strings.TrimSpace(answer.Content)
	report.ActualShouldAnswer = strings.TrimSpace(report.Content) != ""
	report.ActualHandoff = detectHandoff(report.Content) || detectHandoffFromEntries(citationEntryKeys, hitEntryKeys)
	return report
}

func evaluateAnswerAssertions(caseReport *CaseReport, item Case) {
	if caseReport == nil || caseReport.Answer == nil {
		return
	}
	contentLower := strings.ToLower(strings.TrimSpace(caseReport.Answer.Content))
	if item.AnswerExpectation.ShouldAnswer && !caseReport.Answer.ActualShouldAnswer {
		caseReport.Failures = append(caseReport.Failures, "answer expected but no answer content was generated")
	}
	if !item.AnswerExpectation.ShouldAnswer && !item.AnswerExpectation.ExpectHandoff && caseReport.Answer.ActualShouldAnswer && !detectNoAnswer(caseReport.Answer.Content) {
		caseReport.Failures = append(caseReport.Failures, "no-answer case generated a substantive answer")
	}
	if item.AnswerExpectation.ExpectHandoff && !caseReport.Answer.ActualHandoff {
		caseReport.Failures = append(caseReport.Failures, "handoff expected but answer did not clearly escalate")
	}
	for _, token := range item.AnswerExpectation.MustContain {
		if !strings.Contains(contentLower, strings.ToLower(strings.TrimSpace(token))) {
			caseReport.Failures = append(caseReport.Failures, fmt.Sprintf("answer missing required token: %s", token))
		}
	}
	for _, token := range item.AnswerExpectation.MustNotContain {
		if strings.Contains(contentLower, strings.ToLower(strings.TrimSpace(token))) {
			caseReport.Failures = append(caseReport.Failures, fmt.Sprintf("answer contains forbidden token: %s", token))
		}
	}
	if !item.AnswerExpectation.ShouldAnswer && !item.AnswerExpectation.ExpectHandoff && len(item.AnswerExpectation.AllowedNoAnswerReasons) > 0 {
		if !containsString(item.AnswerExpectation.AllowedNoAnswerReasons, caseReport.Retrieval.NoAnswerReason) && !detectNoAnswer(caseReport.Answer.Content) {
			caseReport.Failures = append(caseReport.Failures, "actual no-answer signal does not match allowed reasons")
		}
	}
}

func finalizeCaseReport(caseReport CaseReport) CaseReport {
	caseReport.Passed = caseReport.Error == "" && len(caseReport.Failures) == 0
	return caseReport
}

func buildReportSummary(report *Report) {
	if report == nil {
		return
	}
	latencies := make([]int64, 0, len(report.Cases))
	var recall5Total float64
	var recall10Total float64
	var mrrTotal float64
	var retrievalCases int
	var citationCases int
	var citationSatisfied int
	var noAnswerCases int
	var noAnswerTP int
	var noAnswerPred int
	var handoffCases int
	var handoffSatisfied int

	for _, item := range report.Cases {
		if !item.Enabled {
			continue
		}
		if item.Passed {
			report.Summary.PassedCases++
		} else {
			report.Summary.FailedCases++
		}
		if item.Retrieval.LatencyMs > 0 {
			latencies = append(latencies, item.Retrieval.LatencyMs)
		}
		if item.Retrieval.ExpectedHitCount > 0 {
			retrievalCases++
			recall5Total += item.Retrieval.RecallAt5
			recall10Total += item.Retrieval.RecallAt10
			mrrTotal += item.Retrieval.MRR
		}
		report.Summary.ForbiddenHitCount += item.Retrieval.ForbiddenHitCount

		if item.Retrieval.CitationRequired {
			citationCases++
			if item.Retrieval.CitationSatisfied {
				citationSatisfied++
			}
		}

		if item.Answer == nil {
			report.Summary.AnswerAssertionsSkipped += len(item.SkippedAssertions)
			continue
		}
		report.Summary.AnswerCases++

		if !item.Answer.ExpectedShouldAnswer && !item.Answer.ExpectHandoff {
			noAnswerCases++
			if !item.Answer.ActualShouldAnswer || detectNoAnswer(item.Answer.Content) {
				noAnswerTP++
			}
		}
		if !item.Answer.ActualShouldAnswer || detectNoAnswer(item.Answer.Content) {
			noAnswerPred++
		}
		if item.Answer.ExpectHandoff {
			handoffCases++
			if item.Answer.ActualHandoff {
				handoffSatisfied++
			}
		}
	}

	report.Summary.RetrievalCases = retrievalCases
	report.Summary.CitationCases = citationCases
	report.Summary.CitationSatisfiedCases = citationSatisfied
	report.Summary.NoAnswerCases = noAnswerCases
	report.Summary.NoAnswerTruePositive = noAnswerTP
	report.Summary.NoAnswerPredicted = noAnswerPred
	report.Summary.HandoffCases = handoffCases
	report.Summary.HandoffSatisfiedCases = handoffSatisfied

	if retrievalCases > 0 {
		report.Summary.RecallAt5 = recall5Total / float64(retrievalCases)
		report.Summary.RecallAt10 = recall10Total / float64(retrievalCases)
		report.Summary.MRR = mrrTotal / float64(retrievalCases)
	}
	if citationCases > 0 {
		report.Summary.CitationCoverage = float64(citationSatisfied) / float64(citationCases)
	}
	if report.Summary.AnswerEvaluated && noAnswerPred > 0 {
		report.Summary.NoAnswerPrecision = float64(noAnswerTP) / float64(noAnswerPred)
	}
	if report.Summary.AnswerEvaluated && noAnswerCases > 0 {
		report.Summary.NoAnswerRecall = float64(noAnswerTP) / float64(noAnswerCases)
	}
	if report.Summary.AnswerEvaluated && handoffCases > 0 {
		report.Summary.HandoffAccuracy = float64(handoffSatisfied) / float64(handoffCases)
	}
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		report.Summary.P50LatencyMs = percentileLatency(latencies, 0.50)
		report.Summary.P95LatencyMs = percentileLatency(latencies, 0.95)
	}
}

func orderedHitEntryKeys(items []rag.RetrieveResult) []string {
	ret := make([]string, 0, len(items))
	seen := make(map[string]struct{})
	for _, item := range items {
		key := entryKeyForHit(item.DocumentID, item.FaqID, item.ChunkID)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		ret = append(ret, key)
	}
	return ret
}

func orderedCitationEntryKeys(items []dto.KnowledgeCitation) []string {
	ret := make([]string, 0, len(items))
	seen := make(map[string]struct{})
	for _, item := range items {
		key := entryKeyForHit(item.DocumentID, item.FaqID, 0)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		ret = append(ret, key)
	}
	return ret
}

func entryKeyForHit(documentID int64, faqID int64, chunkID int64) string {
	switch {
	case documentID > 0:
		return fmt.Sprintf("document:%d", documentID)
	case faqID > 0:
		return fmt.Sprintf("faq:%d", faqID)
	case chunkID > 0:
		return fmt.Sprintf("chunk:%d", chunkID)
	default:
		return ""
	}
}

func countHitsWithin(actual []string, expected []string, limit int) int {
	if limit <= 0 || len(actual) == 0 || len(expected) == 0 {
		return 0
	}
	if len(actual) > limit {
		actual = actual[:limit]
	}
	return countIntersect(actual, expected)
}

func countIntersect(actual []string, expected []string) int {
	if len(actual) == 0 || len(expected) == 0 {
		return 0
	}
	expectedSet := make(map[string]struct{}, len(expected))
	for _, item := range expected {
		expectedSet[item] = struct{}{}
	}
	count := 0
	for _, item := range actual {
		if _, ok := expectedSet[item]; ok {
			count++
		}
	}
	return count
}

func reciprocalRank(actual []string, expected []string) float64 {
	if len(actual) == 0 || len(expected) == 0 {
		return 0
	}
	expectedSet := make(map[string]struct{}, len(expected))
	for _, item := range expected {
		expectedSet[item] = struct{}{}
	}
	for idx, item := range actual {
		if _, ok := expectedSet[item]; ok {
			return 1.0 / float64(idx+1)
		}
	}
	return 0
}

func ratio(numerator int, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func splitExpectedMatches(actual []string, expected []string) ([]string, []string) {
	if len(expected) == 0 {
		return nil, nil
	}
	actualSet := make(map[string]struct{}, len(actual))
	for _, item := range actual {
		actualSet[item] = struct{}{}
	}
	matched := make([]string, 0)
	missing := make([]string, 0)
	for _, item := range expected {
		if _, ok := actualSet[item]; ok {
			matched = append(matched, item)
		} else {
			missing = append(missing, item)
		}
	}
	return matched, missing
}

func evaluateCitationSatisfied(item Case, resolved *ResolvedCase, actualCitationKeys []string) bool {
	if !boolValue(item.AnswerExpectation.CitationRequired) {
		return true
	}
	if item.AnswerExpectation.MinCitationCount > 0 && len(actualCitationKeys) < item.AnswerExpectation.MinCitationCount {
		return false
	}
	required := resolved.MustCiteEntryKeys
	if len(required) == 0 {
		required = resolved.ExpectedEntryKeys
	}
	if len(required) == 0 {
		return len(actualCitationKeys) > 0
	}
	return countIntersect(actualCitationKeys, required) > 0
}

func percentileLatency(values []int64, percentile float64) int64 {
	if len(values) == 0 {
		return 0
	}
	if percentile <= 0 {
		return values[0]
	}
	if percentile >= 1 {
		return values[len(values)-1]
	}
	index := int(math.Ceil(percentile*float64(len(values)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

func hasAnswerAssertions(item Case) bool {
	if item.AnswerExpectation.ExpectHandoff {
		return true
	}
	if !item.AnswerExpectation.ShouldAnswer {
		return true
	}
	if len(item.AnswerExpectation.MustContain) > 0 || len(item.AnswerExpectation.MustNotContain) > 0 {
		return true
	}
	return false
}

func detectNoAnswer(content string) bool {
	text := strings.ToLower(strings.TrimSpace(content))
	if text == "" {
		return true
	}
	markers := []string{
		"insufficient",
		"not enough information",
		"cannot determine",
		"can't determine",
		"unable to determine",
		"not available in the provided knowledge",
		"unknown from the provided context",
		"没有足够信息",
		"无法判断",
		"无法确认",
	}
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func detectHandoff(content string) bool {
	text := strings.ToLower(strings.TrimSpace(content))
	if text == "" {
		return false
	}
	markers := []string{
		"contact support",
		"contact our support team",
		"contact a technician",
		"reach out to support",
		"escalate",
		"human support",
		"service team",
		"人工",
		"客服",
		"工程师",
		"联系支持",
	}
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func detectHandoffFromEntries(citationEntryKeys []string, hitEntryKeys []string) bool {
	targets := []string{"faq.contact_support"}
	_ = targets
	for _, item := range append(append([]string(nil), citationEntryKeys...), hitEntryKeys...) {
		if strings.Contains(item, "contact_support") {
			return true
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, item := range values {
		if strings.EqualFold(strings.TrimSpace(item), target) {
			return true
		}
	}
	return false
}
