package request

type CreateTicketKnowledgeCandidateRequest struct {
	Title            string `json:"title"`
	Suggestion       string `json:"suggestion"`
	RootCauseSummary string `json:"root_cause_summary"`
	SolutionSummary  string `json:"solution_summary"`
	KnowledgeBaseID  int64  `json:"knowledge_base_id"`
}
