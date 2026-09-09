package builders

import (
	"remotehelpdesk/internal/models"
	"testing"
)

func TestIntegrationBuildersMaskSecrets(t *testing.T) {
	config := BuildTenantIntegrationConfig(&models.TenantIntegrationConfig{AppSecretFingerprint: "1234567890abcdef"})
	if config.MaskedAppSecret != "1234****cdef" {
		t.Fatalf("masked app secret = %q", config.MaskedAppSecret)
	}
	credential := BuildProductAIUsageCredential(&models.ProductAIUsageCredential{APIKeyFingerprint: "abcdef1234567890"})
	if credential.MaskedAPIKey != "abcd****7890" {
		t.Fatalf("masked api key = %q", credential.MaskedAPIKey)
	}
}
