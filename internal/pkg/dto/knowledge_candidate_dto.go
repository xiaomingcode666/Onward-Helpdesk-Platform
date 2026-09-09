package dto

// 知识候选（§7.4）企业端 DTO 与请求。

// EnterpriseKnowledgeCandidateDTO 知识候选。
type EnterpriseKnowledgeCandidateDTO struct {
	ID                    int64                               `json:"id"`
	ProductID             int64                               `json:"product_id"`
	ProductModelID        int64                               `json:"product_model_id"`
	SourceType            string                              `json:"source_type"`
	SourceID              string                              `json:"source_id"`
	TicketID              int64                               `json:"ticket_id"`
	Title                 string                              `json:"title"`
	Suggestion            string                              `json:"suggestion"`
	RootCauseSummary      string                              `json:"root_cause_summary"`
	SolutionSummary       string                              `json:"solution_summary"`
	KnowledgeBaseID       int64                               `json:"knowledge_base_id"`
	KnowledgeEntryID      int64                               `json:"knowledge_entry_id"`
	QualityScore          int                                 `json:"quality_score"`
	ValueScore            int                                 `json:"value_score"`
	CandidateScore        int                                 `json:"candidate_score"`
	ScoreBreakdown        KnowledgeCandidateScoreBreakdownDTO `json:"score_breakdown"`
	QualityFlags          []string                            `json:"quality_flags"`
	ScoreVersion          string                              `json:"score_version"`
	ScoredAt              string                              `json:"scored_at"`
	SimilarityHash        string                              `json:"similarity_hash"`
	DuplicateGroupID      string                              `json:"duplicate_group_id"`
	MergedToCandidateID   int64                               `json:"merged_to_candidate_id"`
	DeduplicationOverride bool                                `json:"deduplication_override"`
	RecurrenceCount       int                                 `json:"recurrence_count"`
	AffectedDeviceCount   int                                 `json:"affected_device_count"`
	RequiresReassessment  bool                                `json:"requires_reassessment"`
	ReviewEligible        bool                                `json:"review_eligible"`
	ReviewStatus          string                              `json:"review_status"`
	ReviewRemark          string                              `json:"review_remark"`
	ReviewedAt            string                              `json:"reviewed_at"`
	CreatedAt             string                              `json:"created_at"`
}

type KnowledgeCandidateScoreBreakdownDTO struct {
	EvidenceCompleteness int `json:"evidence_completeness"`
	OutcomeConfidence    int `json:"outcome_confidence"`
	ContextCompleteness  int `json:"context_completeness"`
	ContentUsability     int `json:"content_usability"`
	RecurrenceValue      int `json:"recurrence_value"`
	ImpactValue          int `json:"impact_value"`
	SeverityValue        int `json:"severity_value"`
	KnowledgeGapValue    int `json:"knowledge_gap_value"`
}

// EnterpriseKnowledgeCandidateApproveRequest 批准候选请求。
// 未提供标题/内容时采用候选自带摘要；Publish 决定批准后保留草稿还是发布并进入索引。
type EnterpriseKnowledgeCandidateApproveRequest struct {
	Title             string `json:"title"`
	Content           string `json:"content"`
	Language          string `json:"language"`
	Category          string `json:"category"`
	KnowledgeBaseName string `json:"knowledge_base_name"`
	Visibility        string `json:"visibility"`
	Publish           bool   `json:"publish"`
}

// EnterpriseKnowledgeCandidateEnrichRequest 补充候选结构化内容并重新评分。
type EnterpriseKnowledgeCandidateEnrichRequest struct {
	Title            string `json:"title"`
	Suggestion       string `json:"suggestion"`
	RootCauseSummary string `json:"root_cause_summary"`
	SolutionSummary  string `json:"solution_summary"`
}

// EnterpriseKnowledgeCandidateRejectRequest 拒绝候选请求。
type EnterpriseKnowledgeCandidateRejectRequest struct {
	Remark string `json:"remark"`
}

// EnterpriseKnowledgeCandidateMergeRequest 合并候选到已有知识条目。
type EnterpriseKnowledgeCandidateMergeRequest struct {
	KnowledgeEntryID int64 `json:"knowledge_entry_id"`
}
