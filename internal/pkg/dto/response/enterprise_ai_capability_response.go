package response

type EnterpriseAICapabilityResponse struct {
	Type      string   `json:"type"`
	Available bool     `json:"available"`
	ModelName string   `json:"modelName"`
	Models    []string `json:"models"`
	Dimension int      `json:"dimension"`
}

type EnterpriseAICapabilitiesResponse struct {
	GeneratedAt       string                                 `json:"generatedAt"`
	Capabilities      []EnterpriseAICapabilityResponse       `json:"capabilities"`
	DefaultCredential *EnterpriseAIDefaultCredentialResponse `json:"defaultCredential,omitempty"`
}

type EnterpriseAIDefaultCredentialResponse struct {
	KeyName         string `json:"keyName"`
	KeyStatus       string `json:"keyStatus"`
	ProvisionStatus string `json:"provisionStatus"`
	Ready           bool   `json:"ready"`
}
