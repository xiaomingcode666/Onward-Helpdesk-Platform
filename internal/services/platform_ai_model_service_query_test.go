package services

import (
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/providers"
)

func TestNormalizePlatformSub2APIUsersQuery(t *testing.T) {
	query := normalizePlatformSub2APIUsersQuery(request.PlatformSub2APIUsersQueryRequest{
		Page:     -1,
		PageSize: 500,
	})
	if query.Page != 1 {
		t.Fatalf("page = %d", query.Page)
	}
	if query.PageSize != 100 {
		t.Fatalf("pageSize = %d", query.PageSize)
	}
	if query.SortBy != "created_at" || query.SortOrder != "desc" {
		t.Fatalf("sort = %s %s", query.SortBy, query.SortOrder)
	}
}

func TestFilterPlatformSub2APIUsersUsesTenantBindings(t *testing.T) {
	allowed := platformSub2APIUserIDs([]models.Sub2APITenantAccount{
		{Sub2APIAccountID: "12", Status: enums.StatusOk},
		{Sub2APIAccountID: "13", Status: enums.StatusOk},
		{Sub2APIAccountID: "14", Status: enums.StatusDeleted},
		{Sub2APIAccountID: "invalid", Status: enums.StatusOk},
	})
	got := filterPlatformSub2APIUsers([]providers.Sub2APIAdminUser{
		{ID: 15, Email: "external@example.com"},
		{ID: 13, Email: "tenant-b@example.com"},
		{ID: 12, Email: "tenant-a@example.com"},
		{ID: 14, Email: "deleted@example.com"},
	}, allowed)
	if len(got) != 2 || got[0].ID != 13 || got[1].ID != 12 {
		t.Fatalf("filtered users = %+v, want bound users 13 and 12", got)
	}
}

func TestPaginatePlatformSub2APIUsers(t *testing.T) {
	users := []providers.Sub2APIAdminUser{{ID: 1}, {ID: 2}, {ID: 3}}
	got, total, pages := paginatePlatformSub2APIUsers(users, 2, 2)
	if len(got) != 1 || got[0].ID != 3 || total != 3 || pages != 2 {
		t.Fatalf("page = %+v, total = %d, pages = %d", got, total, pages)
	}
}

func TestSub2APIAccountNeedsAttentionIncludesLoginCredentials(t *testing.T) {
	valid := models.Sub2APITenantAccount{
		AccountName:             "Tenant",
		ProvisionStatus:         "active",
		DefaultKeyID:            "101",
		DefaultKeyName:          "Tenant默认key",
		DefaultKeyGroupID:       platformSub2APIDefaultKeyGroupID,
		DefaultKeyCiphertext:    "encrypted-key",
		DefaultKeyStatus:        "active",
		LoginEmail:              "tenant@example.com",
		LoginPasswordCiphertext: "encrypted-password",
	}
	for _, testCase := range []struct {
		name    string
		account *models.Sub2APITenantAccount
		want    bool
	}{
		{name: "missing account", want: true},
		{name: "valid account", account: &valid, want: false},
		{name: "missing email", account: func() *models.Sub2APITenantAccount { item := valid; item.LoginEmail = ""; return &item }(), want: true},
		{name: "missing password", account: func() *models.Sub2APITenantAccount { item := valid; item.LoginPasswordCiphertext = ""; return &item }(), want: true},
		{name: "missing default key", account: func() *models.Sub2APITenantAccount { item := valid; item.DefaultKeyCiphertext = ""; return &item }(), want: true},
		{name: "failed provisioning", account: func() *models.Sub2APITenantAccount { item := valid; item.ProvisionStatus = "failed"; return &item }(), want: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := sub2APIAccountNeedsAttention(testCase.account); got != testCase.want {
				t.Fatalf("needs attention = %t, want %t", got, testCase.want)
			}
		})
	}
}
