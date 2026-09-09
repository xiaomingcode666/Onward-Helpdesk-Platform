package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestCustomerAccountDeletionCompletesPrivacyCleanup(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.CustomerUser{},
		&models.CustomerDeviceBinding{},
		&models.MobilePushToken{},
		&models.LoginSession{},
		&models.DSARRequest{},
		&models.DSARExecutionLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		sqls.SetDB(nil)
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("Account-delete-123!"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	now := time.Now()
	user := models.User{
		Username: "customer.delete.test", Nickname: "Delete Test", Password: string(passwordHash), Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	customerUser := models.CustomerUser{
		TenantID: 9, CustomerOrgID: 3, UserID: user.ID, DisplayName: "Delete Test",
		Email: "delete@example.com", Phone: "+1-555-0100", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&customerUser).Error; err != nil {
		t.Fatalf("create customer user: %v", err)
	}
	binding := models.CustomerDeviceBinding{
		TenantID: 9, CustomerOrgID: 3, CustomerUserID: customerUser.ID, DeviceID: 88, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&binding).Error; err != nil {
		t.Fatalf("create binding: %v", err)
	}
	push := models.MobilePushToken{
		TenantID: 9, UserID: user.ID, Platform: "ios", TokenFingerprint: strings.Repeat("a", 64),
		TokenCiphertext: "encrypted", LastSeenAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&push).Error; err != nil {
		t.Fatalf("create push token: %v", err)
	}
	session := models.LoginSession{
		UserID: user.ID, Token: "ak_customer_delete", DomainType: models.DomainTypeCustomer,
		TenantID: 9, SubjectType: models.SubjectTypeCustomerUser, SubjectID: customerUser.ID,
		ExpiredAt: now.Add(time.Hour), AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create login session: %v", err)
	}
	principal := &dto.AuthPrincipal{
		UserID: user.ID, TenantID: 9, DomainType: models.DomainTypeCustomer,
		SubjectType: models.SubjectTypeCustomerUser, SubjectID: customerUser.ID, CustomerUserID: customerUser.ID,
	}

	if _, err := CustomerAccountDeletionService.Delete(context.Background(), principal, request.DeleteCustomerPortalAccountRequest{
		CurrentPassword: "wrong-password", Confirmation: "DELETE",
	}); err == nil {
		t.Fatal("wrong password was accepted")
	}
	result, err := CustomerAccountDeletionService.Delete(context.Background(), principal, request.DeleteCustomerPortalAccountRequest{
		CurrentPassword: "Account-delete-123!", Confirmation: "DELETE",
	})
	if err != nil {
		t.Fatalf("delete account: %v", err)
	}
	if result.Status != "completed" || result.CompletedAt == nil || result.SubjectEmail != "" {
		t.Fatalf("unexpected deletion result: %+v", result)
	}

	if err := db.First(&customerUser, customerUser.ID).Error; err != nil {
		t.Fatalf("reload customer user: %v", err)
	}
	if customerUser.Status != enums.StatusDeleted || customerUser.DisplayName != "Deleted account" || customerUser.Email != "" || customerUser.Phone != "" {
		t.Fatalf("customer identity was not anonymized: %+v", customerUser)
	}
	if err := db.First(&binding, binding.ID).Error; err != nil || binding.Status != enums.StatusDeleted {
		t.Fatalf("device access was not revoked: %+v, %v", binding, err)
	}
	if err := db.First(&push, push.ID).Error; err != nil || push.RevokedAt == nil {
		t.Fatalf("push token was not revoked: %+v, %v", push, err)
	}
	if err := db.First(&session, session.ID).Error; err != nil || session.RevokedAt == nil {
		t.Fatalf("login session was not revoked: %+v, %v", session, err)
	}
	if err := db.First(&user, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if user.Status != enums.StatusDeleted || !strings.HasPrefix(user.Username, "deleted_") || user.Password != "" {
		t.Fatalf("account credentials were not removed: %+v", user)
	}
}
