package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/pkg/secretstore"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

type sub2APIProvisionRecoveryProvider struct {
	providers.Sub2APIProvider
	remoteUsers []providers.Sub2APIAdminUser
	loginCalls  int
	listCalls   int
	createCalls int
}

type sub2APIDefaultKeyProvider struct {
	providers.Sub2APIProvider
	keys        []providers.Sub2APIKey
	listCalls   int
	createCalls int
	updateCalls int
	updatedID   int64
}

func (p *sub2APIDefaultKeyProvider) ListKeys(_ context.Context, _ string, query providers.Sub2APIListKeysQuery) (*providers.Sub2APIListKeysResponse, error) {
	p.listCalls++
	return &providers.Sub2APIListKeysResponse{Data: providers.Sub2APIListKeysData{
		Items: p.keys, Total: int64(len(p.keys)), Page: query.Page, PageSize: query.PageSize, Pages: 1,
	}}, nil
}

func (p *sub2APIDefaultKeyProvider) CreateKey(_ context.Context, _ string, req providers.Sub2APICreateKeyRequest) (*providers.Sub2APICreateKeyResponse, error) {
	p.createCalls++
	item := providers.Sub2APIKey{
		ID: 99, Key: "sk-created-default", Name: req.Name, GroupID: req.GroupID, Quota: req.Quota, Status: "active",
	}
	p.keys = append(p.keys, item)
	return &providers.Sub2APICreateKeyResponse{Data: item}, nil
}

func (p *sub2APIDefaultKeyProvider) UpdateKey(_ context.Context, _ string, keyID int64, req providers.Sub2APIUpdateKeyRequest) (*providers.Sub2APIUpdateKeyResponse, error) {
	p.updateCalls++
	p.updatedID = keyID
	for i := range p.keys {
		if p.keys[i].ID != keyID {
			continue
		}
		p.keys[i].Name = req.Name
		p.keys[i].GroupID = req.GroupID
		if req.Quota != nil {
			p.keys[i].Quota = *req.Quota
		}
		p.keys[i].Status = req.Status
		return &providers.Sub2APIUpdateKeyResponse{Data: p.keys[i]}, nil
	}
	return nil, &providers.Sub2APIClientError{StatusCode: 404, Message: "key not found"}
}

func (p *sub2APIProvisionRecoveryProvider) Login(_ context.Context, _ providers.Sub2APILoginRequest) (*providers.Sub2APILoginResponse, error) {
	p.loginCalls++
	if p.loginCalls == 1 {
		return nil, &providers.Sub2APIClientError{
			StatusCode: 401,
			Code:       "401",
			Message:    "invalid email or password",
			Reason:     "INVALID_CREDENTIALS",
		}
	}
	return &providers.Sub2APILoginResponse{Data: providers.Sub2APILoginData{
		AccessToken: "access-token",
		ExpiresIn:   3600,
		User: providers.Sub2APIUser{
			ID:     88,
			Email:  "t-008008@remotedesk.com",
			Status: "active",
		},
	}}, nil
}

func (p *sub2APIProvisionRecoveryProvider) AdminListUsers(_ context.Context, _ string, _ providers.Sub2APIAdminListUsersQuery) (*providers.Sub2APIAdminListUsersResponse, error) {
	p.listCalls++
	return &providers.Sub2APIAdminListUsersResponse{Data: providers.Sub2APIAdminListUsersData{
		Items:    p.remoteUsers,
		Total:    int64(len(p.remoteUsers)),
		Page:     1,
		PageSize: 100,
		Pages:    1,
	}}, nil
}

func (p *sub2APIProvisionRecoveryProvider) AdminCreateUser(_ context.Context, _ string, _ providers.Sub2APIAdminCreateUserRequest) (*providers.Sub2APIAdminCreateUserResponse, error) {
	p.createCalls++
	return &providers.Sub2APIAdminCreateUserResponse{Data: providers.Sub2APIAdminUser{
		ID:     88,
		Email:  "t-008008@remotedesk.com",
		Status: "active",
	}}, nil
}

func TestEnsureSub2APIUserRecreatesMissingUpstreamUserForStaleMapping(t *testing.T) {
	account, operator := setupSub2APIProvisionRecoveryTest(t)
	provider := &sub2APIProvisionRecoveryProvider{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resolution, err := (&platformAIModelService{}).ensureSub2APIUser(ctx, provider, "admin-key", account, &models.Tenant{Name: "Recovery Tenant"}, operator)
	if err != nil {
		t.Fatalf("ensureSub2APIUser() error = %v", err)
	}
	if !resolution.Created || resolution.UserID != 88 {
		t.Fatalf("resolution = %+v", resolution)
	}
	if provider.listCalls != 1 || provider.createCalls != 1 || provider.loginCalls != 2 {
		t.Fatalf("calls: list=%d create=%d login=%d", provider.listCalls, provider.createCalls, provider.loginCalls)
	}
}

func TestEnsureSub2APIUserDoesNotTakeOverExistingRemoteAccount(t *testing.T) {
	account, operator := setupSub2APIProvisionRecoveryTest(t)
	provider := &sub2APIProvisionRecoveryProvider{remoteUsers: []providers.Sub2APIAdminUser{{
		ID:    88,
		Email: account.LoginEmail,
	}}}

	resolution, err := (&platformAIModelService{}).ensureSub2APIUser(context.Background(), provider, "admin-key", account, &models.Tenant{Name: "Recovery Tenant"}, operator)
	if err == nil || resolution != nil {
		t.Fatalf("resolution = %+v, error = %v", resolution, err)
	}
	if !strings.Contains(err.Error(), "登录凭据与远端不一致") {
		t.Fatalf("error = %v", err)
	}
	if provider.createCalls != 0 || provider.listCalls != 1 || provider.loginCalls != 1 {
		t.Fatalf("calls: list=%d create=%d login=%d", provider.listCalls, provider.createCalls, provider.loginCalls)
	}
}

func TestClearSub2APIDefaultKeyRemovesStaleRemoteMapping(t *testing.T) {
	account := &models.Sub2APITenantAccount{
		DefaultKeyID:          "stale-key",
		DefaultKeyName:        "Stale",
		DefaultKeyGroupID:     9,
		DefaultKeyQuota:       10,
		DefaultKeyQuotaUsed:   2,
		DefaultKeyCiphertext:  "ciphertext",
		DefaultKeyFingerprint: "fingerprint",
		DefaultKeyStatus:      "active",
		QuotaSnapshotJSON:     `{"stale":true}`,
	}
	clearSub2APIDefaultKey(account)
	if account.DefaultKeyID != "" || account.DefaultKeyCiphertext != "" || account.DefaultKeyStatus != "unknown" {
		t.Fatalf("default key was not cleared: %+v", account)
	}
	if account.DefaultKeyGroupID != platformSub2APIDefaultKeyGroupID || account.QuotaSnapshotJSON != "{}" {
		t.Fatalf("default key defaults were not restored: %+v", account)
	}
}

func TestEnsureRemoteTenantDefaultKeyUpdatesExistingDefaultInsteadOfCreatingDuplicate(t *testing.T) {
	provider := &sub2APIDefaultKeyProvider{keys: []providers.Sub2APIKey{
		{ID: 57, Key: "sk-product", Name: "RHD-PRODUCT-13", GroupID: 2, Quota: 2.11, Status: "active"},
		{ID: 56, Key: "sk-default-56", Name: "SIT Tenant默认key", GroupID: 2, Quota: 15, Status: "active"},
		{ID: 55, Key: "sk-default-55", Name: "SIT Tenant默认key", GroupID: 2, Quota: 15, Status: "active"},
	}}
	account := &models.Sub2APITenantAccount{
		TenantID: 11, AccountName: "SIT Tenant", DefaultKeyID: "57", DefaultKeyName: "RHD-PRODUCT-13", DefaultKeyCiphertext: "product-ciphertext",
	}

	key, err := (&platformAIModelService{}).ensureRemoteTenantDefaultKey(
		context.Background(), provider, "token", account, "SIT Tenant默认key", 16, nil, "",
	)
	if err != nil {
		t.Fatalf("ensureRemoteTenantDefaultKey() error = %v", err)
	}
	if key == nil || key.ID != 56 || key.Quota != 16 {
		t.Fatalf("updated key = %+v, want existing default key 56", key)
	}
	if provider.createCalls != 0 || provider.updateCalls != 1 || provider.updatedID != 56 {
		t.Fatalf("calls: create=%d update=%d updatedID=%d", provider.createCalls, provider.updateCalls, provider.updatedID)
	}

	key, err = (&platformAIModelService{}).ensureRemoteTenantDefaultKey(
		context.Background(), provider, "token", account, "SIT Tenant默认key", 16, nil, "",
	)
	if err != nil {
		t.Fatalf("second ensureRemoteTenantDefaultKey() error = %v", err)
	}
	if key == nil || key.ID != 56 || provider.createCalls != 0 || provider.updateCalls != 1 {
		t.Fatalf("second call was not idempotent: key=%+v create=%d update=%d", key, provider.createCalls, provider.updateCalls)
	}
}

func TestEnsureRemoteTenantDefaultKeyCreatesOnlyWhenNamedDefaultIsMissing(t *testing.T) {
	provider := &sub2APIDefaultKeyProvider{keys: []providers.Sub2APIKey{{
		ID: 57, Key: "sk-product", Name: "RHD-PRODUCT-13", GroupID: 2, Quota: 2.11, Status: "active",
	}}}
	account := &models.Sub2APITenantAccount{TenantID: 11, AccountName: "SIT Tenant", DefaultKeyID: "57", DefaultKeyName: "RHD-PRODUCT-13"}

	key, err := (&platformAIModelService{}).ensureRemoteTenantDefaultKey(
		context.Background(), provider, "token", account, "SIT Tenant默认key", 15, nil, "",
	)
	if err != nil {
		t.Fatalf("ensureRemoteTenantDefaultKey() error = %v", err)
	}
	if key == nil || key.ID != 99 || provider.createCalls != 1 || provider.updateCalls != 0 {
		t.Fatalf("created key=%+v create=%d update=%d", key, provider.createCalls, provider.updateCalls)
	}
}

func TestDefaultKeyValidationRejectsProductKeyMapping(t *testing.T) {
	account := &models.Sub2APITenantAccount{
		AccountName: "SIT Tenant", ProvisionStatus: "active", DefaultKeyID: "57", DefaultKeyName: "RHD-PRODUCT-13",
		DefaultKeyGroupID: 2, DefaultKeyCiphertext: "ciphertext", DefaultKeyStatus: "active", LoginEmail: "tenant@example.com", LoginPasswordCiphertext: "password",
	}
	if !sub2APIAccountNeedsAttention(account) {
		t.Fatal("product key mapping must require platform repair")
	}
	keys := []providers.Sub2APIKey{
		{ID: 57, Name: "RHD-PRODUCT-13"},
		{ID: 56, Name: "SIT Tenant默认key"},
	}
	if got := findAccountDefaultSub2APIKey(account, keys); got != nil {
		t.Fatalf("enterprise sync selected product key as tenant default: %+v", got)
	}
	account.DefaultKeyID = "56"
	account.DefaultKeyName = "SIT Tenant默认key"
	if got := findAccountDefaultSub2APIKey(account, keys); got == nil || got.ID != 56 {
		t.Fatalf("enterprise sync did not select the valid tenant default key: %+v", got)
	}
}

func TestResolveTenantDefaultKeySecretUsesRemoteSecretWhenLocalMappingIsWrong(t *testing.T) {
	account := &models.Sub2APITenantAccount{DefaultKeyID: "57", DefaultKeyName: "RHD-PRODUCT-13", DefaultKeyCiphertext: "product-ciphertext"}
	key := &providers.Sub2APIKey{ID: 56, Name: "SIT Tenant默认key", Key: "sk-default-56"}
	ciphertext, fingerprint, err := resolveTenantDefaultKeySecret(account, key, "SIT Tenant默认key")
	if err != nil {
		t.Fatalf("resolveTenantDefaultKeySecret() error = %v", err)
	}
	if ciphertext == "" || ciphertext == account.DefaultKeyCiphertext || fingerprint != secretstore.Fingerprint(key.Key) {
		t.Fatalf("resolved secret ciphertext=%q fingerprint=%q", ciphertext, fingerprint)
	}
}

func TestRemoteTenantDefaultKeyExpiryComparisonUsesSeconds(t *testing.T) {
	expiresAt := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	formatted := expiresAt.Format(time.RFC3339)
	key := &providers.Sub2APIKey{ID: 56, Name: "SIT Tenant默认key", GroupID: 2, Quota: 15, Status: "active", ExpiresAt: &formatted}
	if !remoteTenantDefaultKeyMatches(key, key.Name, key.Quota, &expiresAt) {
		t.Fatal("equivalent default key expiry should be reusable")
	}
}

func TestSub2APIProvisionClaimRejectsConcurrentUpdateAndRecoversStaleClaim(t *testing.T) {
	account, _ := setupSub2APIProvisionRecoveryTest(t)
	now := time.Now()
	claimed, err := repositories.PlatformIAMRepository.ClaimSub2APIAccountProvision(sqls.DB(), account.ID, now.Add(-platformSub2APIProvisionClaimTTL), map[string]any{
		"provision_status": "provisioning_user",
		"updated_at":       now,
	})
	if err != nil || !claimed {
		t.Fatalf("first claim = %v, error = %v", claimed, err)
	}
	claimed, err = repositories.PlatformIAMRepository.ClaimSub2APIAccountProvision(sqls.DB(), account.ID, now.Add(-platformSub2APIProvisionClaimTTL), map[string]any{
		"provision_status": "provisioning_user",
		"updated_at":       now,
	})
	if err != nil || claimed {
		t.Fatalf("concurrent claim = %v, error = %v", claimed, err)
	}
	if err := sqls.DB().Model(&models.Sub2APITenantAccount{}).Where("id = ?", account.ID).Update("updated_at", now.Add(-2*platformSub2APIProvisionClaimTTL)).Error; err != nil {
		t.Fatalf("age provision claim: %v", err)
	}
	claimed, err = repositories.PlatformIAMRepository.ClaimSub2APIAccountProvision(sqls.DB(), account.ID, now.Add(-platformSub2APIProvisionClaimTTL), map[string]any{
		"provision_status": "provisioning_user",
		"updated_at":       now,
	})
	if err != nil || !claimed {
		t.Fatalf("stale claim recovery = %v, error = %v", claimed, err)
	}
}

func setupSub2APIProvisionRecoveryTest(t *testing.T) (*models.Sub2APITenantAccount, *dto.AuthPrincipal) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&models.Sub2APITenantAccount{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	ciphertext, err := secretstore.Encrypt(platformSub2APIAccountPassword)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	account := &models.Sub2APITenantAccount{
		TenantID:                 8008,
		Sub2APIAccountID:         "stale-remote-id",
		AccountName:              "Recovery Tenant",
		LoginEmail:               "t-008008@remotedesk.com",
		LoginPasswordCiphertext:  ciphertext,
		LoginPasswordFingerprint: secretstore.Fingerprint(platformSub2APIAccountPassword),
		ProvisionStatus:          "failed",
		DefaultKeyID:             "stale-key",
		DefaultKeyCiphertext:     "stale-ciphertext",
		DefaultKeyStatus:         "active",
	}
	if err := db.Create(account).Error; err != nil {
		t.Fatalf("create account error = %v", err)
	}
	return account, &dto.AuthPrincipal{UserID: 1, Username: "platform-admin"}
}
