package request

type EnterpriseAIModelWorkspaceQueryRequest struct {
	StartDate   string
	EndDate     string
	Granularity string
	Timezone    string
	Page        int
	PageSize    int
	SortBy      string
	SortOrder   string
}

type EnterpriseAIKeyQuotaUpdateRequest struct {
	Quota *float64 `json:"quota" binding:"required"`
}

type EnterpriseAIModelDefaultLLMUpdateRequest struct {
	ModelName string `json:"modelName" binding:"required"`
}
