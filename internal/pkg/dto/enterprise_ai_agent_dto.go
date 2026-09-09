package dto

type EnterpriseAIAgentReviewRequest struct {
	Comment string `json:"comment"`
}

type EnterpriseProductAgentProvisionFailureDTO struct {
	ProductID   int64  `json:"productId"`
	ProductName string `json:"productName"`
	Reason      string `json:"reason"`
}

type EnterpriseProductAgentProvisionResultDTO struct {
	Total    int                                         `json:"total"`
	Created  int                                         `json:"created"`
	Existing int                                         `json:"existing"`
	Failed   int                                         `json:"failed"`
	Failures []EnterpriseProductAgentProvisionFailureDTO `json:"failures"`
}
