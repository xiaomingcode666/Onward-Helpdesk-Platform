package eval

import "time"

type RunOptions struct {
	TopK             int           `json:"topK"`
	ScoreThreshold   float64       `json:"scoreThreshold"`
	RerankTopN       int           `json:"rerankTopN"`
	ContextMaxTokens int           `json:"contextMaxTokens"`
	Channel          string        `json:"channel"`
	Scene            string        `json:"scene"`
	GenerateAnswer   bool          `json:"generateAnswer"`
	CaseTimeout      time.Duration `json:"caseTimeout,omitempty"`
}

type Report struct {
	SuiteName    string        `json:"suiteName"`
	SuiteVersion int           `json:"suiteVersion"`
	GeneratedAt  time.Time     `json:"generatedAt"`
	Options      RunOptions    `json:"options"`
	Summary      ReportSummary `json:"summary"`
	Cases        []CaseReport  `json:"cases"`
}

type ReportSummary struct {
	TotalCases              int     `json:"totalCases"`
	EnabledCases            int     `json:"enabledCases"`
	PassedCases             int     `json:"passedCases"`
	FailedCases             int     `json:"failedCases"`
	RetrievalCases          int     `json:"retrievalCases"`
	RecallAt5               float64 `json:"recallAt5,omitempty"`
	RecallAt10              float64 `json:"recallAt10,omitempty"`
	MRR                     float64 `json:"mrr,omitempty"`
	ForbiddenHitCount       int     `json:"forbiddenHitCount"`
	CitationCases           int     `json:"citationCases"`
	CitationSatisfiedCases  int     `json:"citationSatisfiedCases"`
	CitationCoverage        float64 `json:"citationCoverage,omitempty"`
	AnswerEvaluated         bool    `json:"answerEvaluated"`
	AnswerCases             int     `json:"answerCases"`
	NoAnswerCases           int     `json:"noAnswerCases"`
	NoAnswerTruePositive    int     `json:"noAnswerTruePositive"`
	NoAnswerPredicted       int     `json:"noAnswerPredicted"`
	NoAnswerPrecision       float64 `json:"noAnswerPrecision,omitempty"`
	NoAnswerRecall          float64 `json:"noAnswerRecall,omitempty"`
	HandoffCases            int     `json:"handoffCases"`
	HandoffSatisfiedCases   int     `json:"handoffSatisfiedCases"`
	HandoffAccuracy         float64 `json:"handoffAccuracy,omitempty"`
	AnswerAssertionsSkipped int     `json:"answerAssertionsSkipped"`
	P50LatencyMs            int64   `json:"p50LatencyMs,omitempty"`
	P95LatencyMs            int64   `json:"p95LatencyMs,omitempty"`
}

type CaseReport struct {
	ID                string              `json:"id"`
	Name              string              `json:"name"`
	Enabled           bool                `json:"enabled"`
	Passed            bool                `json:"passed"`
	Error             string              `json:"error,omitempty"`
	Failures          []string            `json:"failures,omitempty"`
	SkippedAssertions []string            `json:"skippedAssertions,omitempty"`
	Query             string              `json:"query"`
	Tags              []string            `json:"tags,omitempty"`
	IntentTags        []string            `json:"intentTags,omitempty"`
	FaultCodes        []string            `json:"faultCodes,omitempty"`
	Scope             CaseScopeReport     `json:"scope"`
	Retrieval         CaseRetrievalReport `json:"retrieval"`
	Answer            *CaseAnswerReport   `json:"answer,omitempty"`
}

type CaseScopeReport struct {
	TenantRef         string   `json:"tenantRef,omitempty"`
	ProductRef        string   `json:"productRef,omitempty"`
	ProductModelRef   string   `json:"productModelRef,omitempty"`
	KnowledgeBaseRefs []string `json:"knowledgeBaseRefs,omitempty"`
	Locale            string   `json:"locale,omitempty"`
	RegionCode        string   `json:"regionCode,omitempty"`
	Audience          string   `json:"audience,omitempty"`
	TenantID          int64    `json:"tenantId,omitempty"`
	ProductID         int64    `json:"productId,omitempty"`
	ProductModelID    int64    `json:"productModelId,omitempty"`
	KnowledgeBaseIDs  []int64  `json:"knowledgeBaseIds,omitempty"`
}

type CaseRetrievalReport struct {
	LatencyMs             int64    `json:"latencyMs"`
	NoAnswerReason        string   `json:"noAnswerReason,omitempty"`
	RequestedMode         string   `json:"requestedMode,omitempty"`
	AppliedMode           string   `json:"appliedMode,omitempty"`
	TopScore              float64  `json:"topScore,omitempty"`
	HitEntryKeys          []string `json:"hitEntryKeys,omitempty"`
	ContextEntryKeys      []string `json:"contextEntryKeys,omitempty"`
	CitationEntryKeys     []string `json:"citationEntryKeys,omitempty"`
	ExpectedEntryKeys     []string `json:"expectedEntryKeys,omitempty"`
	ForbiddenEntryKeys    []string `json:"forbiddenEntryKeys,omitempty"`
	ExpectedHitCount      int      `json:"expectedHitCount,omitempty"`
	ExpectedHitsAt5       int      `json:"expectedHitsAt5,omitempty"`
	ExpectedHitsAt10      int      `json:"expectedHitsAt10,omitempty"`
	MinExpectedHitsAt5    int      `json:"minExpectedHitsAt5,omitempty"`
	MinExpectedHitsAt10   int      `json:"minExpectedHitsAt10,omitempty"`
	RecallAt5             float64  `json:"recallAt5,omitempty"`
	RecallAt10            float64  `json:"recallAt10,omitempty"`
	MRR                   float64  `json:"mrr,omitempty"`
	ForbiddenHitCount     int      `json:"forbiddenHitCount"`
	CitationRequired      bool     `json:"citationRequired,omitempty"`
	CitationSatisfied     bool     `json:"citationSatisfied,omitempty"`
	MinCitationCount      int      `json:"minCitationCount,omitempty"`
	ActualCitationCount   int      `json:"actualCitationCount,omitempty"`
	MatchedExpectedKeys   []string `json:"matchedExpectedKeys,omitempty"`
	MissingExpectedKeys   []string `json:"missingExpectedKeys,omitempty"`
	TriggeredByExactTerms []string `json:"triggeredByExactTerms,omitempty"`
}

type CaseAnswerReport struct {
	Generated              bool     `json:"generated"`
	LatencyMs              int64    `json:"latencyMs,omitempty"`
	ModelName              string   `json:"modelName,omitempty"`
	PromptTokens           int      `json:"promptTokens,omitempty"`
	CompletionTokens       int      `json:"completionTokens,omitempty"`
	Content                string   `json:"content,omitempty"`
	ExpectedShouldAnswer   bool     `json:"expectedShouldAnswer"`
	ActualShouldAnswer     bool     `json:"actualShouldAnswer"`
	ExpectHandoff          bool     `json:"expectHandoff,omitempty"`
	ActualHandoff          bool     `json:"actualHandoff,omitempty"`
	MustContain            []string `json:"mustContain,omitempty"`
	MustNotContain         []string `json:"mustNotContain,omitempty"`
	AllowedNoAnswerReasons []string `json:"allowedNoAnswerReasons,omitempty"`
}
