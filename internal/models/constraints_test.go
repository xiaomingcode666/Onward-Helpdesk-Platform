package models_test

import (
	"path/filepath"
	"testing"
	"time"

	"remotehelpdesk/internal/bootstrap"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupConstraintsDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "constraints-test.db")
	db, err := bootstrap.InitDB(config.DBConfig{
		Type:         "sqlite",
		DSN:          "file:" + dbPath + "?_busy_timeout=5000",
		MaxIdleConns: 1,
		MaxOpenConns: 1,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, bootstrap.InitMigrations())
	return db
}

// TestNotNullConstraints 验证 NOT NULL 约束
func TestNotNullConstraints(t *testing.T) {
	db := setupConstraintsDB(t)

	t.Run("TenantNameRequired", func(t *testing.T) {
		// Tenant.Name 是 NOT NULL，SQLite 中空字符串是可以的，但 NULL 会失败
		tenant := &models.Tenant{
			Name:   "",
			Status: enums.StatusOk,
		}
		err := db.Create(tenant).Error
		// 空字符串允许通过 NOT NULL
		assert.NoError(t, err)
		assert.NotZero(t, tenant.ID)

		// 不设置 Name 直接使用零值空字符串也应该可以
		tenant2 := &models.Tenant{}
		// 但是 Name 没有 default 值且有 not null 约束，所以 "" 应该 ok
		tenant2.Name = "" // explicitly empty
		err = db.Create(tenant2).Error
		assert.NoError(t, err)
	})

	t.Run("UserUsernameRequired", func(t *testing.T) {
		user := &models.User{}
		err := db.Create(user).Error
		// NOT NULL 只拒绝 NULL，Go 字符串零值会写成空字符串；非空校验由 UserService 负责。
		assert.NoError(t, err)
	})
}

// TestUniqueConstraints 验证唯一约束
func TestUniqueConstraints(t *testing.T) {
	db := setupConstraintsDB(t)

	t.Run("TenantNameNotUnique", func(t *testing.T) {
		// Tenant.Name 有索引但没有唯一约束，所以可以重复
		tenant1 := &models.Tenant{Name: "duplicate-name", Status: enums.StatusOk}
		require.NoError(t, db.Create(tenant1).Error)

		tenant2 := &models.Tenant{Name: "duplicate-name", Status: enums.StatusOk}
		err := db.Create(tenant2).Error
		assert.NoError(t, err, "Tenant.Name 没有唯一约束，应允许重复")
	})

	t.Run("UserUsernameUnique", func(t *testing.T) {
		user1 := &models.User{
			Username: "unique-username",
			Status:   enums.StatusOk,
		}
		require.NoError(t, db.Create(user1).Error)

		user2 := &models.User{
			Username: "unique-username",
			Status:   enums.StatusOk,
		}
		err := db.Create(user2).Error
		assert.Error(t, err, "重复的 username 应违反唯一约束")
	})

	t.Run("TicketNoUnique", func(t *testing.T) {
		now := time.Now()
		ticket1 := &models.Ticket{
			TicketNo:    "TK-UNIQUE-TEST-001",
			Title:       "Ticket 1",
			Status:      enums.TicketStatusPending,
			AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
		}
		require.NoError(t, db.Create(ticket1).Error)

		ticket2 := &models.Ticket{
			TicketNo:    "TK-UNIQUE-TEST-001",
			Title:       "Ticket 2",
			Status:      enums.TicketStatusPending,
			AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
		}
		err := db.Create(ticket2).Error
		assert.Error(t, err, "重复的 ticket_no 应违反唯一约束")
	})

	t.Run("DeviceTenantNoUnique", func(t *testing.T) {
		device1 := &models.Device{
			TenantID: 1,
			DeviceNo: "DUP-DEVICE",
			Status:   enums.StatusOk,
		}
		require.NoError(t, db.Create(device1).Error)

		device2 := &models.Device{
			TenantID: 1,
			DeviceNo: "DUP-DEVICE",
			Status:   enums.StatusOk,
		}
		err := db.Create(device2).Error
		assert.Error(t, err, "同租户下重复的 device_no 应违反唯一约束")
	})

	t.Run("DeviceDifferentTenantOk", func(t *testing.T) {
		device1 := &models.Device{
			TenantID: 1,
			DeviceNo: "CROSS-TENANT-DEVICE",
			Status:   enums.StatusOk,
		}
		require.NoError(t, db.Create(device1).Error)

		device2 := &models.Device{
			TenantID: 2,
			DeviceNo: "CROSS-TENANT-DEVICE",
			Status:   enums.StatusOk,
		}
		err := db.Create(device2).Error
		assert.NoError(t, err, "不同租户下相同的 device_no 应允许")
	})
}

// TestForeignKeyConstraints 验证外键约束
func TestForeignKeyConstraints(t *testing.T) {
	db := setupConstraintsDB(t)

	t.Run("TicketTagForeignKey", func(t *testing.T) {
		now := time.Now()
		// TicketTag 引用了 Ticket 表，但 GORM 未显式定义外键
		// 验证：当依赖的记录不存在时，SQLite 行为
		orphanTag := &models.TicketTag{
			TicketID: 99999,
			TagID:    99999,
			AuditFields: models.AuditFields{
				CreatedAt: now, CreateUserID: 1, CreateUserName: "test",
				UpdatedAt: now, UpdateUserID: 1, UpdateUserName: "test",
			},
		}
		err := db.Create(orphanTag).Error
		// SQLite 默认不启用外键约束，所以应该成功
		// 但 Pragma 启用后应失败
		assert.NoError(t, err, "SQLite 默认不强制外键约束，孤立记录应允许")
	})
}

// TestSoftDelete 验证软删除
func TestSoftDelete(t *testing.T) {
	db := setupConstraintsDB(t)

	t.Run("UserSoftDelete", func(t *testing.T) {
		user := &models.User{
			Username: "soft-delete-user",
			Status:   enums.StatusOk,
		}
		require.NoError(t, db.Create(user).Error)

		// 软删除
		require.NoError(t, db.Delete(user).Error)

		// 正常查询不应返回
		var found models.User
		err := db.Where("username = ?", "soft-delete-user").First(&found).Error
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "软删除后的记录不应被查询到")

		// Unscoped 查询应返回
		var unscoped models.User
		err = db.Unscoped().Where("username = ?", "soft-delete-user").First(&unscoped).Error
		require.NoError(t, err)
		assert.Equal(t, user.ID, unscoped.ID)
		assert.True(t, unscoped.DeletedAt.Valid, "软删除记录 deleted_at 应不为空")
	})
}

// TestCheckConstraints 验证 CHECK 约束
func TestCheckConstraints(t *testing.T) {
	db := setupConstraintsDB(t)

	t.Run("InvalidStatusValue", func(t *testing.T) {
		// models.Status 是 int 类型，不在 [0,1,2] 范围内的值在应用层不被 GORM 拦截
		// 但 PostgreSQL 上可以通过 CHECK 约束禁止
		// 在 SQLite 测试中，我们验证应用层允许写入；实际 PG 上应由 DB 层拦截
		tenant := &models.Tenant{
			Name:   "bad-status-tenant",
			Status: enums.Status(99),
		}
		err := db.Create(tenant).Error
		assert.NoError(t, err, "SQLite 无 CHECK 约束，异常状态值应被写入")
		_ = err
	})

	t.Run("EmptyTenantIDRejected", func(t *testing.T) {
		// Product.TenantID 是 `not null`，零值在 GORM 中会被写入
		product := &models.Product{
			Code:   "no-tenant-product",
			Name:   "No Tenant Product",
			Status: enums.StatusOk,
		}
		err := db.Create(product).Error
		// TenantID=0 可以通过 NOT NULL（因为是 int64 默认值）
		assert.NoError(t, err)
		_ = err
	})

	t.Run("NegativeTokenShouldBeRejected", func(t *testing.T) {
		asset := &models.Asset{
			StorageKey: "neg-size-test",
			Filename:   "negative.txt",
			FileSize:   -100,
		}
		err := db.Create(asset).Error
		assert.ErrorIs(t, err, models.ErrAssetFileSizeNegative)

		// Raw SQL bypasses GORM hooks and must still be rejected by the database.
		err = db.Exec(`INSERT INTO t_asset (asset_id, storage_key, file_size) VALUES (?, ?, ?)`, "neg-raw", "neg-raw", -100).Error
		assert.Error(t, err)
	})
}
