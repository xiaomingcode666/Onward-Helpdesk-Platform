package builders

import (
	"testing"
	"time"

	"remotehelpdesk/internal/services"
)

func TestBuildEnterpriseAICapabilitiesIncludesDefaultCredentialState(t *testing.T) {
	result := BuildEnterpriseAICapabilities(&services.EnterpriseAICapabilityAggregate{
		GeneratedAt: time.Now(),
		DefaultCredential: &services.EnterpriseAIDefaultCredentialState{
			KeyName:         "tenant-default",
			KeyStatus:       "active",
			ProvisionStatus: "active",
			Ready:           true,
		},
	})
	if result == nil || result.DefaultCredential == nil {
		t.Fatalf("default credential state was not mapped: %+v", result)
	}
	if result.DefaultCredential.KeyName != "tenant-default" || !result.DefaultCredential.Ready {
		t.Fatalf("default credential state = %+v", result.DefaultCredential)
	}
}
