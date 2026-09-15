package services

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// This opt-in regression needs an explicitly supplied disposable PostgreSQL
// database. It creates only its own random schema and never reads app config.
func setupCaseOwnerPostgresTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("HELPDESK_CASE_OWNER_TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("set HELPDESK_CASE_OWNER_TEST_POSTGRES_DSN to an isolated test database for PostgreSQL concurrency coverage")
	}
	connectionConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid PostgreSQL test connection configuration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	admin := stdlib.OpenDB(*connectionConfig)
	t.Cleanup(func() { _ = admin.Close() })
	schemaName := "case_owner_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schemaName); err != nil {
		t.Fatalf("create isolated test schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := admin.ExecContext(cleanupCtx, "DROP SCHEMA "+schemaName+" CASCADE"); err != nil {
			t.Errorf("remove isolated test schema: %v", err)
		}
	})
	connectionConfig.RuntimeParams["search_path"] = schemaName
	connectionConfig.RuntimeParams["statement_timeout"] = "10000"
	connection := stdlib.OpenDB(*connectionConfig)
	t.Cleanup(func() { _ = connection.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: connection}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open isolated PostgreSQL schema: %v", err)
	}
	db = db.WithContext(ctx)
	if err := db.AutoMigrate(&models.User{}, &models.Tenant{}, &models.TenantMember{}, &models.AuthRole{}, &models.AuthRoleBinding{},
		&models.AuthRolePermission{}, &models.AuthSubjectPermissionOverride{}, &models.Ticket{}, &models.EngineerProfile{},
		&models.AgentProfile{}, &models.AgentTeam{}, &models.Department{}, &models.AuthAuditLog{}, &models.LoginSession{}); err != nil {
		t.Fatal(err)
	}
	previousDB := sqls.DB()
	sqls.SetDB(db)
	t.Cleanup(func() { sqls.SetDB(previousDB) })
	if err := db.Create(&models.Tenant{ID: 91, Name: "Owner concurrency fixture"}).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCaseOwnerPostgresRoleEditSerializesWithNewOwnerBinding(t *testing.T) {
	db := setupCaseOwnerPostgresTestDB(t)
	owner, member, _ := seedCaseOwnerMember(t, db, 91, "current-service", constants.PermissionTicketUpdate.Code)
	_, _, nextRole := seedCaseOwnerMember(t, db, 91, "next-service", constants.PermissionTicketChangeStatus.Code)
	seedOwnedTicket(t, db, owner.ID)
	op := caseOwnerPrincipal(owner)

	policyEntered := make(chan struct{})
	releasePolicy := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releasePolicy) }) }
	defer release()
	policyDone := make(chan error, 1)
	go func() {
		policyDone <- db.Transaction(func(tx *gorm.DB) error {
			return withCaseOwnerRoleGuardDB(tx, &nextRole, func() error {
				// The guard has already enumerated the members of this role.
				close(policyEntered)
				<-releasePolicy
				return tx.Model(&models.AuthRolePermission{}).Where("role_id = ?", nextRole.ID).
					Update("permission_code", constants.PermissionTicketView.Code).Error
			})
		})
	}()
	select {
	case <-policyEntered:
	case err := <-policyDone:
		t.Fatalf("policy edit ended before race setup: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("policy edit did not reach race setup")
	}
	memberDone := make(chan error, 1)
	go func() {
		_, err := EnterpriseIAMService.UpdateMember(91, member.ID, request.EnterpriseMemberUpdateRequest{RoleCodes: []string{nextRole.Code}}, op)
		memberDone <- err
	}()
	var memberErr error
	select {
	case memberErr = <-memberDone:
		if memberErr == nil || !strings.Contains(memberErr.Error(), "稍后重试") {
			t.Errorf("in-flight IAM change must reject a concurrent binding without mutation: %v", memberErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("concurrent binding waited on IAM instead of returning a retry error")
	}
	release()
	if err := <-policyDone; err != nil {
		t.Errorf("unowned role should be editable before rebinding: %v", err)
	}
	_, memberErr = EnterpriseIAMService.UpdateMember(91, member.ID, request.EnterpriseMemberUpdateRequest{RoleCodes: []string{nextRole.Code}}, op)
	if memberErr == nil || !strings.Contains(memberErr.Error(), "交接") {
		t.Errorf("rebinding must validate the committed read-only policy and require handover: %v", memberErr)
	}
	if err := ValidateCaseOwnerDB(db, 91, owner.ID); err != nil {
		t.Fatalf("concurrent IAM changes orphaned the active case owner: %v", err)
	}
}

func TestCaseOwnerPostgresIAMContentionCannotDeadlockHeldUser(t *testing.T) {
	db := setupCaseOwnerPostgresTestDB(t)
	owner, _, role := seedCaseOwnerMember(t, db, 91, "owner", constants.PermissionTicketUpdate.Code)
	provisioning := db.Begin()
	if provisioning.Error != nil {
		t.Fatal(provisioning.Error)
	}
	defer provisioning.Rollback()
	var locked models.User
	if err := provisioning.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, owner.ID).Error; err != nil {
		t.Fatal(err)
	}
	policyHasLock := make(chan struct{})
	policyDone := make(chan error, 1)
	go func() {
		policyDone <- db.Transaction(func(tx *gorm.DB) error {
			if err := lockCaseOwnerIAMChangesDB(tx); err != nil {
				return err
			}
			close(policyHasLock)
			return withCaseOwnerRoleGuardDB(tx, &role, func() error { return nil })
		})
	}()
	select {
	case <-policyHasLock:
	case err := <-policyDone:
		t.Fatalf("role edit ended before acquiring IAM lock: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("role edit did not acquire IAM lock")
	}
	// The provisioner already owns User, while role editing owns IAM and needs
	// User. Refresh must fail immediately, allowing its outer transaction to
	// roll back instead of waiting for IAM and completing the deadlock cycle.
	refreshDone := make(chan error, 1)
	go func() { refreshDone <- EnsureTenantDefaultIAMRolesDB(provisioning, 91, nil) }()
	select {
	case err := <-refreshDone:
		if err == nil || !strings.Contains(err.Error(), "稍后重试") {
			t.Errorf("nested provisioning refresh must return a retry error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("provisioning refresh waited while holding User and introduced a lock cycle")
	}
	if err := provisioning.Rollback().Error; err != nil {
		t.Fatal(err)
	}
	if err := <-policyDone; err != nil {
		t.Fatalf("role edit should complete once provisioning rolls back: %v", err)
	}
}
