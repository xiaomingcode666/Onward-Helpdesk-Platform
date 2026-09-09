package builders

import (
	"testing"
	"time"

	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/services"
)

func TestBuildEnterpriseAIModelWorkspaceIncludesKeyProductBinding(t *testing.T) {
	aggregate := &services.EnterpriseAIModelWorkspaceAggregate{
		GeneratedAt: time.Now(),
		Keys: &providers.Sub2APIListKeysResponse{
			Data: providers.Sub2APIListKeysData{
				Items: []providers.Sub2APIKey{{
					ID:     33,
					UserID: 12,
					Key:    "sk-test-product-binding",
					Name:   "RHD-PRODUCT-1",
					Status: "active",
				}},
				Total:    1,
				Page:     1,
				PageSize: 20,
				Pages:    1,
			},
		},
		KeyProductBindings: map[int64]services.EnterpriseAIKeyProductBinding{
			33: {
				ProductID:   1,
				ProductName: "Production Smoke Product",
				ProductCode: "PROD-JOB-20260725203334",
			},
		},
	}

	result := BuildEnterpriseAIModelWorkspace(aggregate)
	if result == nil || len(result.Keys) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	key := result.Keys[0]
	if key.Name != "RHD-PRODUCT-1" {
		t.Fatalf("key name changed unexpectedly: %s", key.Name)
	}
	if key.ProductID != 1 || key.ProductName != "Production Smoke Product" || key.ProductCode != "PROD-JOB-20260725203334" {
		t.Fatalf("product binding not included: %+v", key)
	}
}
