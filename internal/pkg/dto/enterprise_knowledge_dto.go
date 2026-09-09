package dto

type EnterpriseKnowledgeEntryListItemDTO struct {
	ID              int64    `json:"id"`
	KnowledgeBaseID int64    `json:"knowledge_base_id"`
	Title           string   `json:"title"`
	Type            string   `json:"type"`
	Category        string   `json:"category"`
	Status          string   `json:"status"`
	Tags            []string `json:"tags"`
	Languages       []string `json:"languages"`
	ProductScope    string   `json:"product_scope"`
	ProductIDs      []int64  `json:"-"`
	RelevanceScore  int      `json:"relevance_score"`
	QualityScore    int      `json:"quality_score"`
	ValueScore      int      `json:"value_score"`
	EntryScore      int      `json:"entry_score"`
	ReviewEligible  bool     `json:"review_eligible"`
	QualityFlags    []string `json:"quality_flags"`
	HitRate         int      `json:"hit_rate"`
	AuthorName      string   `json:"author_name"`
	UpdatedAt       string   `json:"updated_at"`
}

type EnterpriseKnowledgeRelatedProductDTO struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

type EnterpriseKnowledgeEntryDTO struct {
	ID                int64                                  `json:"id"`
	KnowledgeBaseID   int64                                  `json:"knowledge_base_id"`
	Title             string                                 `json:"title"`
	Content           string                                 `json:"content"`
	Category          string                                 `json:"category"`
	Status            string                                 `json:"status"`
	Tags              []string                               `json:"tags"`
	RelatedProducts   []EnterpriseKnowledgeRelatedProductDTO `json:"related_products"`
	FaultCodes        []string                               `json:"fault_codes"`
	Languages         []string                               `json:"languages"`
	HitRate           int                                    `json:"hit_rate"`
	CompletenessScore int                                    `json:"completeness_score"`
	QualityScore      int                                    `json:"quality_score"`
	ValueScore        int                                    `json:"value_score"`
	EntryScore        int                                    `json:"entry_score"`
	ReviewEligible    bool                                   `json:"review_eligible"`
	QualityFlags      []string                               `json:"quality_flags"`
	AuthorName        string                                 `json:"author_name"`
	ReviewerName      string                                 `json:"reviewer_name,omitempty"`
	CreatedAt         string                                 `json:"created_at"`
	UpdatedAt         string                                 `json:"updated_at"`
	PublishedAt       string                                 `json:"published_at,omitempty"`
}

type EnterpriseKnowledgeMutationRequest struct {
	KnowledgeBaseID   int64    `json:"knowledge_base_id"`
	Title             string   `json:"title"`
	Content           string   `json:"content"`
	Category          string   `json:"category"`
	Type              string   `json:"type"`
	Visibility        string   `json:"visibility"`
	Tags              []string `json:"tags"`
	RelatedProductIDs []int64  `json:"related_product_ids"`
	FaultCodes        []string `json:"fault_codes"`
	Language          string   `json:"language"`
}

type EnterpriseKnowledgeStatsDTO struct {
	TotalEntries    int64 `json:"total_entries"`
	PublishedCount  int64 `json:"published_count"`
	ReviewCount     int64 `json:"review_count"`
	DraftCount      int64 `json:"draft_count"`
	DeprecatedCount int64 `json:"deprecated_count"`
	AvgHitRate      int64 `json:"avg_hit_rate"`
}

type EnterpriseKnowledgeVersionDTO struct {
	Version    int    `json:"version"`
	EntryID    int64  `json:"entry_id"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	Language   string `json:"language"`
	ChangeNote string `json:"change_note"`
	UpdatedBy  string `json:"updated_by"`
	UpdatedAt  string `json:"updated_at"`
}

type EnterpriseKnowledgeQualityDTO struct {
	CompletenessScore int `json:"completeness_score"`
	HitRate           int `json:"hit_rate"`
	PositiveFeedback  int `json:"positive_feedback"`
	NegativeFeedback  int `json:"negative_feedback"`
	TotalViews        int `json:"total_views"`
}

type EnterpriseKnowledgeTranslationDTO struct {
	Language  string `json:"language"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
}
