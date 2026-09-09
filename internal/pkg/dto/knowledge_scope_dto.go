package dto

type KnowledgeScopeContext struct {
	TenantID       int64  `json:"tenantId"`
	ProductID      int64  `json:"productId"`
	ProductModelID int64  `json:"productModelId"`
	DeviceID       int64  `json:"deviceId"`
	ServiceCodeID  int64  `json:"serviceCodeId"`
	CustomerID     int64  `json:"customerId"`
	Locale         string `json:"locale"`
	RegionCode     string `json:"regionCode"`
	Audience       string `json:"audience"`
}

type KnowledgeScopeTraceItem struct {
	Step    string         `json:"step"`
	Reason  string         `json:"reason"`
	Applied bool           `json:"applied"`
	Meta    map[string]any `json:"meta,omitempty"`
}

type ResolvedKnowledgeScope struct {
	Context                      KnowledgeScopeContext     `json:"context"`
	Languages                    []string                  `json:"languages,omitempty"`
	KnowledgeBaseIDs             []int64                   `json:"knowledgeBaseIds"`
	ProductKnowledgeBaseIDs      []int64                   `json:"productKnowledgeBaseIds,omitempty"`
	TenantSharedKnowledgeBaseIDs []int64                   `json:"tenantSharedKnowledgeBaseIds,omitempty"`
	EntryKeys                    []string                  `json:"entryKeys"`
	RevisionIDs                  []int64                   `json:"revisionIds"`
	ResolutionTrace              []KnowledgeScopeTraceItem `json:"resolutionTrace"`
}

type KnowledgeCitation struct {
	DocumentID    int64   `json:"documentId"`
	DocumentTitle string  `json:"documentTitle"`
	FaqID         int64   `json:"faqId"`
	FaqQuestion   string  `json:"faqQuestion"`
	ChunkNo       int     `json:"chunkNo"`
	Title         string  `json:"title"`
	SectionPath   string  `json:"sectionPath"`
	Snippet       string  `json:"snippet"`
	Score         float64 `json:"score"`
}
