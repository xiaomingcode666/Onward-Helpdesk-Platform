package migration

import (
	"testing"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestNotificationIdempotencyMigrationScopesKeysByTenant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.Notification{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Migrator().DropIndex(&models.Notification{}, "uk_notification_tenant_idempotency"); err != nil {
		t.Fatalf("drop current notification index: %v", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX uk_notification_idempotency_key ON t_notification (idempotency_key)").Error; err != nil {
		t.Fatalf("create legacy notification index: %v", err)
	}

	if err := ensureNotificationTenantIdempotencyIndex(db); err != nil {
		t.Fatalf("ensureNotificationTenantIdempotencyIndex() error = %v", err)
	}
	if db.Migrator().HasIndex(&models.Notification{}, "uk_notification_idempotency_key") {
		t.Fatal("legacy global notification idempotency index still exists")
	}
	if !db.Migrator().HasIndex(&models.Notification{}, "uk_notification_tenant_idempotency") {
		t.Fatal("tenant-scoped notification idempotency index was not created")
	}

	key := "ticket:42:assigned"
	for _, tenantID := range []int64{1, 2} {
		item := notificationIdempotencyFixture(tenantID, &key)
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("create notification for tenant %d: %v", tenantID, err)
		}
	}
	duplicate := notificationIdempotencyFixture(1, &key)
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("tenant-scoped idempotency index accepted a duplicate within one tenant")
	}
}

func notificationIdempotencyFixture(tenantID int64, key *string) models.Notification {
	return models.Notification{
		TenantID:        tenantID,
		IdempotencyKey:  key,
		RecipientUserID: tenantID * 100,
		Title:           "test notification",
	}
}
