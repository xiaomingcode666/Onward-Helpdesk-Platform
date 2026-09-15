package migration

import (
	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
	"testing"
)

func TestTicketStatusWorkflowMigrationRetainsLegacyAndPinnedFacts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqls.SetDB(db)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	statement := &gorm.Statement{DB: db}
	require.NoError(t, statement.Parse(&models.Ticket{}))
	legacy := struct {
		ID          int64 `gorm:"primaryKey"`
		CaseStatus  string
		CaseOwnerID int64
	}{ID: 1, CaseStatus: "waiting", CaseOwnerID: 18}
	require.NoError(t, db.Table(statement.Table).AutoMigrate(&legacy))
	require.NoError(t, db.Table(statement.Table).Create(&legacy).Error)
	require.NoError(t, migrationFuncs[141].Fn())
	var ticket models.Ticket
	require.NoError(t, db.First(&ticket, 1).Error)
	require.Equal(t, "waiting", ticket.CaseStatus)
	require.EqualValues(t, 18, ticket.CaseOwnerID)
	require.Zero(t, ticket.CaseWorkflowVersionID)
	require.Empty(t, ticket.CaseWorkflowKey)
	require.NoError(t, db.Model(&ticket).UpdateColumns(map[string]any{"case_workflow_version_id": 37, "case_workflow_key": "*"}).Error)
	require.NoError(t, migrationFuncs[141].Fn())
	require.NoError(t, db.First(&ticket, 1).Error)
	require.EqualValues(t, 37, ticket.CaseWorkflowVersionID)
	require.Equal(t, "*", ticket.CaseWorkflowKey)
}
