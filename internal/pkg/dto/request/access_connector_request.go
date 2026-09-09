package request

type AccessConnectorCreateRequest struct {
	Name          string `json:"name" binding:"required"`
	ConnectorType string `json:"connectorType" binding:"required"`
	BaseURL       string `json:"baseUrl"`
	AuthType      string `json:"authType"`
	AuthConfig    string `json:"authConfig"`
	FieldMapping  string `json:"fieldMapping"`
	TemplateCode  string `json:"templateCode"`
}

type AccessConnectorUpdateRequest struct {
	ID            int64  `json:"id" binding:"required"`
	Name          string `json:"name" binding:"required"`
	ConnectorType string `json:"connectorType" binding:"required"`
	BaseURL       string `json:"baseUrl"`
	AuthType      string `json:"authType"`
	AuthConfig    string `json:"authConfig" binding:"required"`
	FieldMapping  string `json:"fieldMapping"`
	TemplateCode  string `json:"templateCode"`
}
