package dto

type TicketKnowledgeCandidateDTO struct {
	ID                    int64    `json:"id"`
	TicketID              int64    `json:"ticket_id"`
	SourceType            string   `json:"source_type"`
	Title                 string   `json:"title"`
	Suggestion            string   `json:"suggestion"`
	RootCauseSummary      string   `json:"root_cause_summary"`
	SolutionSummary       string   `json:"solution_summary"`
	KnowledgeBaseID       int64    `json:"knowledge_base_id"`
	KnowledgeEntryID      int64    `json:"knowledge_entry_id"`
	QualityScore          int      `json:"quality_score"`
	ValueScore            int      `json:"value_score"`
	CandidateScore        int      `json:"candidate_score"`
	QualityFlags          []string `json:"quality_flags"`
	DeduplicationOverride bool     `json:"deduplication_override"`
	RecurrenceCount       int      `json:"recurrence_count"`
	AffectedDeviceCount   int      `json:"affected_device_count"`
	RequiresReassessment  bool     `json:"requires_reassessment"`
	ReviewEligible        bool     `json:"review_eligible"`
	Status                string   `json:"status"`
	ReviewStatus          string   `json:"review_status"`
	CreatedBy             string   `json:"created_by"`
	CreatedAt             string   `json:"created_at"`
	Created               bool     `json:"created,omitempty"`
}
