package enums

type VectorDBType string

const (
	VectorDBTypeQdrant  VectorDBType = "qdrant"
	VectorDBTypeLanceDB VectorDBType = "lancedb"
)

var vectorDBTypeLabelMap = map[VectorDBType]string{
	VectorDBTypeQdrant:  "Qdrant",
	VectorDBTypeLanceDB: "LanceDB",
}

func GetVectorDBTypeLabel(dbType VectorDBType) string {
	return vectorDBTypeLabelMap[dbType]
}

type KnowledgeDocumentContentType string

const (
	KnowledgeDocumentContentTypeHTML     KnowledgeDocumentContentType = "html"
	KnowledgeDocumentContentTypeMarkdown KnowledgeDocumentContentType = "markdown"
)

type KnowledgeBaseType string

const (
	KnowledgeBaseTypeDocument KnowledgeBaseType = "document"
	KnowledgeBaseTypeFAQ      KnowledgeBaseType = "faq"
)

var knowledgeBaseTypeLabelMap = map[KnowledgeBaseType]string{
	KnowledgeBaseTypeDocument: "enum.KnowledgeBaseTypeDocument",
	KnowledgeBaseTypeFAQ:      "enum.KnowledgeBaseTypeFAQ",
}

func GetKnowledgeBaseTypeLabel(knowledgeType KnowledgeBaseType, locale string) string {
	return enumLabel(locale, knowledgeBaseTypeLabelMap[knowledgeType])
}

type KnowledgeBaseAccessScope string

const (
	KnowledgeBaseAccessScopeTenant  KnowledgeBaseAccessScope = "tenant"
	KnowledgeBaseAccessScopeProduct KnowledgeBaseAccessScope = "product"
)

var knowledgeBaseAccessScopeLabelMap = map[KnowledgeBaseAccessScope]string{
	KnowledgeBaseAccessScopeTenant:  "enum.KnowledgeBaseAccessScopeTenant",
	KnowledgeBaseAccessScopeProduct: "enum.KnowledgeBaseAccessScopeProduct",
}

func GetKnowledgeBaseAccessScopeLabel(scope KnowledgeBaseAccessScope, locale string) string {
	return enumLabel(locale, knowledgeBaseAccessScopeLabelMap[scope])
}

func IsValidKnowledgeBaseAccessScope(scope string) bool {
	switch KnowledgeBaseAccessScope(scope) {
	case KnowledgeBaseAccessScopeTenant, KnowledgeBaseAccessScopeProduct:
		return true
	default:
		return false
	}
}

var knowledgeDocumentContentTypeLabelMap = map[KnowledgeDocumentContentType]string{
	KnowledgeDocumentContentTypeHTML:     "HTML",
	KnowledgeDocumentContentTypeMarkdown: "Markdown",
}

func GetKnowledgeDocumentContentTypeLabel(contentType KnowledgeDocumentContentType) string {
	return knowledgeDocumentContentTypeLabelMap[contentType]
}

type KnowledgeDocumentIndexStatus string

const (
	KnowledgeDocumentIndexStatusPending KnowledgeDocumentIndexStatus = "pending"
	KnowledgeDocumentIndexStatusIndexed KnowledgeDocumentIndexStatus = "indexed"
	KnowledgeDocumentIndexStatusFailed  KnowledgeDocumentIndexStatus = "failed"
)

var KnowledgeDocumentIndexStatusValues = []KnowledgeDocumentIndexStatus{
	KnowledgeDocumentIndexStatusPending,
	KnowledgeDocumentIndexStatusIndexed,
	KnowledgeDocumentIndexStatusFailed,
}

var knowledgeDocumentIndexStatusLabelMap = map[KnowledgeDocumentIndexStatus]string{
	KnowledgeDocumentIndexStatusPending: "enum.KnowledgeDocumentIndexStatusPending",
	KnowledgeDocumentIndexStatusIndexed: "enum.KnowledgeDocumentIndexStatusIndexed",
	KnowledgeDocumentIndexStatusFailed:  "enum.KnowledgeDocumentIndexStatusFailed",
}

func GetKnowledgeDocumentIndexStatusLabel(status KnowledgeDocumentIndexStatus, locale string) string {
	return enumLabel(locale, knowledgeDocumentIndexStatusLabelMap[status])
}

func IsValidKnowledgeDocumentIndexStatus(status string) bool {
	for _, item := range KnowledgeDocumentIndexStatusValues {
		if string(item) == status {
			return true
		}
	}
	return false
}

type KnowledgeCandidateReviewStatus string

const (
	KnowledgeCandidateReviewStatusPending         KnowledgeCandidateReviewStatus = "pending"
	KnowledgeCandidateReviewStatusNeedsEnrichment KnowledgeCandidateReviewStatus = "needs_enrichment"
	KnowledgeCandidateReviewStatusLowQuality      KnowledgeCandidateReviewStatus = "low_quality"
	KnowledgeCandidateReviewStatusLowValue        KnowledgeCandidateReviewStatus = "low_value"
	KnowledgeCandidateReviewStatusDuplicate       KnowledgeCandidateReviewStatus = "duplicate"
	KnowledgeCandidateReviewStatusApproved        KnowledgeCandidateReviewStatus = "approved"
	KnowledgeCandidateReviewStatusRejected        KnowledgeCandidateReviewStatus = "rejected"
	KnowledgeCandidateReviewStatusMerged          KnowledgeCandidateReviewStatus = "merged"
)

const (
	KnowledgeCandidateScoreVersion        = "knowledge-candidate-v2"
	KnowledgeCandidateMinimumQualityScore = 75
	KnowledgeCandidateMinimumValueScore   = 40
	KnowledgeEntryMinimumQualityScore     = 70
	KnowledgeEntryMinimumValueScore       = 40
)

func IsKnowledgeCandidateReviewReady(status string, qualityScore, valueScore int, mergedToCandidateID int64) bool {
	return status == string(KnowledgeCandidateReviewStatusPending) &&
		qualityScore >= KnowledgeCandidateMinimumQualityScore &&
		valueScore >= KnowledgeCandidateMinimumValueScore &&
		mergedToCandidateID == 0
}

type KnowledgeChunkType string

const (
	KnowledgeChunkTypeText  KnowledgeChunkType = "text"
	KnowledgeChunkTypeFAQ   KnowledgeChunkType = "faq"
	KnowledgeChunkTypeTable KnowledgeChunkType = "table"
	KnowledgeChunkTypeCode  KnowledgeChunkType = "code"
)

var knowledgeChunkTypeLabelMap = map[KnowledgeChunkType]string{
	KnowledgeChunkTypeText:  "enum.KnowledgeChunkTypeText",
	KnowledgeChunkTypeFAQ:   "enum.KnowledgeChunkTypeFAQ",
	KnowledgeChunkTypeTable: "enum.KnowledgeChunkTypeTable",
	KnowledgeChunkTypeCode:  "enum.KnowledgeChunkTypeCode",
}

func GetKnowledgeChunkTypeLabel(chunkType KnowledgeChunkType, locale string) string {
	return enumLabel(locale, knowledgeChunkTypeLabelMap[chunkType])
}

type KnowledgeChunkProvider string

const (
	KnowledgeChunkProviderFixed      KnowledgeChunkProvider = "fixed"
	KnowledgeChunkProviderStructured KnowledgeChunkProvider = "structured"
	KnowledgeChunkProviderFAQ        KnowledgeChunkProvider = "faq"
	KnowledgeChunkProviderSemantic   KnowledgeChunkProvider = "semantic"
)

var knowledgeChunkProviderLabelMap = map[KnowledgeChunkProvider]string{
	KnowledgeChunkProviderFixed:      "enum.KnowledgeChunkProviderFixed",
	KnowledgeChunkProviderStructured: "enum.KnowledgeChunkProviderStructured",
	KnowledgeChunkProviderFAQ:        "enum.KnowledgeChunkProviderFAQ",
	KnowledgeChunkProviderSemantic:   "enum.KnowledgeChunkProviderSemantic",
}

func GetKnowledgeChunkProviderLabel(provider KnowledgeChunkProvider, locale string) string {
	return enumLabel(locale, knowledgeChunkProviderLabelMap[provider])
}

type KnowledgeRetrieveChannel string

const (
	KnowledgeRetrieveChannelIM          KnowledgeRetrieveChannel = "im"
	KnowledgeRetrieveChannelAgentAssist KnowledgeRetrieveChannel = "agent_assist"
	KnowledgeRetrieveChannelAPI         KnowledgeRetrieveChannel = "api"
	KnowledgeRetrieveChannelDebug       KnowledgeRetrieveChannel = "debug"
)

var knowledgeRetrieveChannelLabelMap = map[KnowledgeRetrieveChannel]string{
	KnowledgeRetrieveChannelIM:          "enum.KnowledgeRetrieveChannelIM",
	KnowledgeRetrieveChannelAgentAssist: "enum.KnowledgeRetrieveChannelAgentAssist",
	KnowledgeRetrieveChannelAPI:         "enum.KnowledgeRetrieveChannelAPI",
	KnowledgeRetrieveChannelDebug:       "enum.KnowledgeRetrieveChannelDebug",
}

func GetKnowledgeRetrieveChannelLabel(channel KnowledgeRetrieveChannel, locale string) string {
	return enumLabel(locale, knowledgeRetrieveChannelLabelMap[channel])
}

type KnowledgeRetrieveScene string

const (
	KnowledgeRetrieveSceneFirstResponse KnowledgeRetrieveScene = "first_response"
	KnowledgeRetrieveSceneAssist        KnowledgeRetrieveScene = "assist"
	KnowledgeRetrieveSceneRuntime       KnowledgeRetrieveScene = "runtime"
	KnowledgeRetrieveSceneDiagnosis     KnowledgeRetrieveScene = "diagnosis"
	KnowledgeRetrieveSceneQA            KnowledgeRetrieveScene = "qa"
)

var knowledgeRetrieveSceneLabelMap = map[KnowledgeRetrieveScene]string{
	KnowledgeRetrieveSceneFirstResponse: "enum.KnowledgeRetrieveSceneFirstResponse",
	KnowledgeRetrieveSceneAssist:        "enum.KnowledgeRetrieveSceneAssist",
	KnowledgeRetrieveSceneRuntime:       "enum.KnowledgeRetrieveSceneRuntime",
	KnowledgeRetrieveSceneDiagnosis:     "enum.KnowledgeRetrieveSceneDiagnosis",
	KnowledgeRetrieveSceneQA:            "enum.KnowledgeRetrieveSceneQA",
}

func GetKnowledgeRetrieveSceneLabel(scene KnowledgeRetrieveScene, locale string) string {
	return enumLabel(locale, knowledgeRetrieveSceneLabelMap[scene])
}

type KnowledgeAnswerStatus int

const (
	KnowledgeAnswerStatusNormal   KnowledgeAnswerStatus = 1
	KnowledgeAnswerStatusNoAnswer KnowledgeAnswerStatus = 2
	KnowledgeAnswerStatusFallback KnowledgeAnswerStatus = 3
	KnowledgeAnswerStatusBlocked  KnowledgeAnswerStatus = 4
)

var knowledgeAnswerStatusLabelMap = map[KnowledgeAnswerStatus]string{
	KnowledgeAnswerStatusNormal:   "enum.KnowledgeAnswerStatusNormal",
	KnowledgeAnswerStatusNoAnswer: "enum.KnowledgeAnswerStatusNoAnswer",
	KnowledgeAnswerStatusFallback: "enum.KnowledgeAnswerStatusFallback",
	KnowledgeAnswerStatusBlocked:  "enum.KnowledgeAnswerStatusBlocked",
}

func GetKnowledgeAnswerStatusLabel(status KnowledgeAnswerStatus, locale string) string {
	return enumLabel(locale, knowledgeAnswerStatusLabelMap[status])
}

type KnowledgeFeedbackType int

const (
	KnowledgeFeedbackTypeLike          KnowledgeFeedbackType = 1
	KnowledgeFeedbackTypeDislike       KnowledgeFeedbackType = 2
	KnowledgeFeedbackTypeNotHelpful    KnowledgeFeedbackType = 3
	KnowledgeFeedbackTypeWrongCitation KnowledgeFeedbackType = 4
	KnowledgeFeedbackTypeOther         KnowledgeFeedbackType = 5
)

var knowledgeFeedbackTypeLabelMap = map[KnowledgeFeedbackType]string{
	KnowledgeFeedbackTypeLike:          "enum.KnowledgeFeedbackTypeLike",
	KnowledgeFeedbackTypeDislike:       "enum.KnowledgeFeedbackTypeDislike",
	KnowledgeFeedbackTypeNotHelpful:    "enum.KnowledgeFeedbackTypeNotHelpful",
	KnowledgeFeedbackTypeWrongCitation: "enum.KnowledgeFeedbackTypeWrongCitation",
	KnowledgeFeedbackTypeOther:         "enum.KnowledgeFeedbackTypeOther",
}

func GetKnowledgeFeedbackTypeLabel(feedbackType KnowledgeFeedbackType, locale string) string {
	return enumLabel(locale, knowledgeFeedbackTypeLabelMap[feedbackType])
}

type KnowledgeAnswerMode int

const (
	KnowledgeAnswerModeStrict KnowledgeAnswerMode = 1
	KnowledgeAnswerModeAssist KnowledgeAnswerMode = 2
)

var knowledgeAnswerModeLabelMap = map[KnowledgeAnswerMode]string{
	KnowledgeAnswerModeStrict: "enum.KnowledgeAnswerModeStrict",
	KnowledgeAnswerModeAssist: "enum.KnowledgeAnswerModeAssist",
}

func GetKnowledgeAnswerModeLabel(mode KnowledgeAnswerMode, locale string) string {
	return enumLabel(locale, knowledgeAnswerModeLabelMap[mode])
}
