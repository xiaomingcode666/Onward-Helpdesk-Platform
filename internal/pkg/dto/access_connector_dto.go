package dto

type AccessConnectorListItemDTO struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	ConnectorType     string `json:"connector_type"`
	BaseURL           string `json:"base_url"`
	AuthType          string `json:"auth_type"`
	Active            bool   `json:"active"`
	HealthStatus      string `json:"health_status"`
	LastHealthCheckAt string `json:"last_health_check_at"`
	CreatedAt         string `json:"created_at"`
}
