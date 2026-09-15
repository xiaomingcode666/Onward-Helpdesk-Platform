package deploymentidentity

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInstanceEnvironmentNames(t *testing.T) {
	for _, tc := range []struct {
		instance, project, environment string
		valid                          bool
	}{
		{"onward-shared-integration", "daypop-shared", "integration", true},
		{"onward-acme-staging", "acme", "staging", true},
		{"onward-acme-production", "acme", "production", true},
		{"onward-acme-integration", "acme", "integration", false},
		{"onward-daypop-shared-production", "daypop-shared", "production", false},
		{"onward-shared-staging", "shared", "staging", false},
		{"onward-acme-production", "acme", "staging", false},
		{"onward-acme-staging", "../acme", "staging", false},
		{"onward-acme-development", "acme", "development", false},
		{"", "acme", "staging", false},
	} {
		t.Run(fmt.Sprint(tc), func(t *testing.T) {
			t.Setenv("RHD_INSTANCE_ID", tc.instance)
			t.Setenv("RHD_DEPLOYMENT_PROJECT_ID", tc.project)
			t.Setenv("RHD_PROJECT_ENVIRONMENT", tc.environment)
			t.Setenv("RHD_PROJECT_CONFIG_TENANT_ID", "7")
			id, err := FromEnvironment()
			if tc.valid {
				require.NoError(t, err)
				require.Equal(t, int64(7), id.TenantID)
			} else {
				require.Error(t, err)
			}
		})
	}
	t.Setenv("RHD_INSTANCE_ID", "")
	t.Setenv("RHD_DEPLOYMENT_PROJECT_ID", "")
	id, err := FromEnvironment()
	require.NoError(t, err)
	require.Nil(t, id)
}

func identityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	return db
}

func TestInstanceDatabaseBindingIsReadOnlyAndCannotBeRepurposed(t *testing.T) {
	db := identityTestDB(t)
	id := &Identity{"onward-acme-staging", "acme", "staging", 7}
	require.NoError(t, Check(db, nil, false))
	require.Error(t, Check(db, id, false))
	require.NoError(t, Check(db, id, true))
	require.False(t, db.Migrator().HasTable(&Record{}), "preflight created schema")
	require.NoError(t, db.AutoMigrate(&Record{}))
	require.Error(t, Check(db, id, false))
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return Bind(tx, id) }))
	var original Record
	require.NoError(t, db.First(&original).Error)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return Bind(tx, id) }))
	require.NoError(t, Check(db, id, false))
	for _, other := range []*Identity{nil, {"onward-acme-production", "acme", "production", 7}, {"onward-other-staging", "other", "staging", 7}, {"onward-acme-staging", "acme", "staging", 8}} {
		require.Error(t, Check(db, other, false))
		require.Error(t, Check(db, other, true))
		require.Error(t, db.Transaction(func(tx *gorm.DB) error { return Bind(tx, other) }))
	}
	var after Record
	require.NoError(t, db.First(&after).Error)
	require.Equal(t, original, after, "retry or rejection changed persistent identity")
	require.Error(t, db.Create(&Record{ID: 2, InstanceID: "other"}).Error, "singleton constraint missing")
}

func TestInstanceBindingRollsBackWithConfigurationTransaction(t *testing.T) {
	db := identityTestDB(t)
	require.NoError(t, db.AutoMigrate(&Record{}))
	id := &Identity{"onward-acme-production", "acme", "production", 7}
	err := db.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, Bind(tx, id))
		return errors.New("synthetic configuration failure")
	})
	require.Error(t, err)
	var count int64
	require.NoError(t, db.Model(&Record{}).Count(&count).Error)
	require.Zero(t, count)
}
