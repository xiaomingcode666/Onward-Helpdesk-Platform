package dto

type EnterpriseSearchResultDTO struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	URL         string            `json:"url"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type EnterpriseSearchResultsDTO struct {
	Items          []EnterpriseSearchResultDTO `json:"items"`
	Total          int                         `json:"total"`
	Query          string                      `json:"query"`
	Scopes         []string                    `json:"scopes"`
	Suggestions    []string                    `json:"suggestions"`
	RecentSearches []string                    `json:"recentSearches"`
}
