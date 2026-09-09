package builders

import (
	"encoding/json"
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
)

func TestBuildCustomerEntrySessionKeepsProductNameAndCodeDistinct(t *testing.T) {
	session := &models.CustomerEntrySession{
		ID:             9,
		TenantID:       3,
		ProductID:      5,
		ProductModelID: 7,
		DeviceID:       11,
		ServiceCodeID:  13,
	}
	result := BuildCustomerEntrySession(
		session,
		&models.Tenant{ID: 3, Name: "测试企业"},
		&models.Product{ID: 5, Code: "PROD-5", Name: "测试产品"},
		nil,
		nil,
		nil,
		"SERVICE-13",
		"visitor-token",
	)
	if result == nil || result.Product == nil {
		t.Fatal("expected product summary")
	}
	if result.Product.Code != "PROD-5" || result.Product.Name != "测试产品" {
		t.Fatalf("unexpected product summary: %+v", result.Product)
	}
	if result.Session["tenantId"] != int64(3) || result.Session["productId"] != int64(5) || result.Session["serviceCodeId"] != int64(13) {
		t.Fatalf("entry session scope missing: %+v", result.Session)
	}
}

func TestBuildCustomerPortalDeviceBindingOmitsInternalBindingIDs(t *testing.T) {
	result := BuildCustomerPortalDeviceBinding(
		&models.CustomerDeviceBinding{
			ID:             91,
			TenantID:       3,
			DeviceID:       11,
			CustomerUserID: 21,
			CustomerOrgID:  31,
			BindingRole:    "owner",
			Status:         enums.StatusOk,
		},
		&models.Device{ID: 11, DeviceNo: "DEV-11"},
		&models.Product{ID: 5, Name: "Hydraulic Pump"},
	)
	if result == nil || result.DeviceID != 11 || result.DeviceNo != "DEV-11" || result.ProductName != "Hydraulic Pump" {
		t.Fatalf("unexpected portal binding response: %+v", result)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal portal binding response: %v", err)
	}
	for _, forbidden := range []string{`"id":`, "entrySessionId", "tenantId", "customerUserId", "customerOrgId"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("portal binding response exposed %s: %s", forbidden, raw)
		}
	}
}
