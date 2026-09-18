package dto

type EnterpriseKnowledgeAccessGrantDTO struct {
	ID          int64  `json:"id"`
	SubjectType string `json:"subject_type"`
	SubjectID   int64  `json:"subject_id"`
	SubjectName string `json:"subject_name"`
	AccessLevel string `json:"access_level"`
	Note        string `json:"note"`
}

type EnterpriseKnowledgeAccessGrantRequest struct {
	SubjectType string `json:"subject_type"`
	SubjectID   int64  `json:"subject_id"`
	AccessLevel string `json:"access_level"`
	Note        string `json:"note"`
}

type EnterpriseKnowledgeAccessGrantsReplaceRequest struct {
	Grants []EnterpriseKnowledgeAccessGrantRequest `json:"grants"`
}
