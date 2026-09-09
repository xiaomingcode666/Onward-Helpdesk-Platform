package repositories

import (
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestFindVisibleConversationsUsesConfiguredTableNames(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:customer-portal-visible-conversations?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.Conversation{}, &models.ConversationParticipant{}); err != nil {
		t.Fatal(err)
	}
	conversations := []models.Conversation{
		{TenantID: 7, CustomerID: 11},
		{TenantID: 8, CustomerID: 11},
	}
	if err = db.Create(&conversations).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.ConversationParticipant{
		ConversationID: conversations[1].ID, ParticipantType: string(enums.IMParticipantTypeCustomer),
		ExternalParticipantID: "user:42", Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatal(err)
	}

	got := CustomerPortalRepository.FindVisibleConversations(db, []int64{7}, 11, []string{"user:42"})
	if len(got) != 2 {
		t.Fatalf("visible conversations = %d, want tenant and participant matches", len(got))
	}
}

func TestFindActiveBindingsIncludesEveryDeviceVisibleToCustomerOrganization(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:customer-portal-active-bindings?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.CustomerDeviceBinding{}); err != nil {
		t.Fatal(err)
	}
	bindings := []models.CustomerDeviceBinding{
		{TenantID: 2, CustomerOrgID: 5, CustomerUserID: 6, DeviceID: 101, Status: enums.StatusOk},
		{TenantID: 2, CustomerOrgID: 5, CustomerUserID: 7, DeviceID: 102, Status: enums.StatusOk},
		{TenantID: 2, CustomerOrgID: 5, CustomerUserID: 0, DeviceID: 103, Status: enums.StatusOk},
		{TenantID: 2, CustomerOrgID: 8, CustomerUserID: 9, DeviceID: 104, Status: enums.StatusOk},
	}
	if err = db.Create(&bindings).Error; err != nil {
		t.Fatal(err)
	}

	got := CustomerPortalRepository.FindActiveBindings(db, 6, 5)
	if len(got) != 2 {
		t.Fatalf("active bindings = %d, want own and organization-wide bindings", len(got))
	}
	visibleDevices := map[int64]bool{}
	for _, binding := range got {
		visibleDevices[binding.DeviceID] = true
		if visible := CustomerDeviceBindingRepository.FindVisibleForCustomer(db, binding.TenantID, binding.DeviceID, 6, 5); visible == nil {
			t.Fatalf("device %d is listed but rejected by conversation visibility lookup", binding.DeviceID)
		}
	}
	if !visibleDevices[101] || visibleDevices[102] || !visibleDevices[103] || visibleDevices[104] {
		t.Fatalf("unexpected visible devices: %#v", visibleDevices)
	}

	organizationOnly := CustomerPortalRepository.FindActiveBindings(db, 0, 5)
	if len(organizationOnly) != 3 {
		t.Fatalf("organization-only bindings = %#v, want every organization device", organizationOnly)
	}
	for _, binding := range organizationOnly {
		if visible := CustomerDeviceBindingRepository.FindVisibleForCustomer(db, binding.TenantID, binding.DeviceID, 0, 5); visible == nil {
			t.Fatalf("organization-visible device %d is rejected by conversation visibility lookup", binding.DeviceID)
		}
	}
}
