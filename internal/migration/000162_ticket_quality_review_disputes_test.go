package migration

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestBackfillTicketQualityReviewSnapshotsKeepsExistingValues(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true}})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Ticket{}, &models.TicketQualityReview{}))
	require.NoError(t, db.Create(&models.Ticket{ID: 1, TenantID: 1, TicketNo: "Q-1", CurrentAssigneeID: 21, CurrentTeamID: 7}).Error)
	require.NoError(t, db.Create(&models.Ticket{ID: 2, TenantID: 2, TicketNo: "Q-2", CurrentAssigneeID: 22, CurrentTeamID: 8}).Error)
	now := time.Now().UTC()
	reviews := []models.TicketQualityReview{
		{ID: 1, TenantID: 1, TicketID: 1, ScoreSnapshotJSON: "{}", Result: "fail", Outcome: "fail", ReviewerID: 9, ReviewedAt: now},
		{ID: 2, TenantID: 1, TicketID: 1, ScoreSnapshotJSON: "{}", Result: "pass", ReviewerID: 9, ReviewedAt: now},
		{ID: 3, TenantID: 1, TicketID: 1, ScoreSnapshotJSON: "{}", Result: "fail", Outcome: "fail", ReviewerID: 9, ReviewedAt: now, ReviewedAgentID: 30, ReviewedTeamID: 3, FinalOutcome: "pass"},
		// A ticket ID from another tenant must not attribute the review to that tenant's agent.
		{ID: 4, TenantID: 1, TicketID: 2, ScoreSnapshotJSON: "{}", Result: "pass", Outcome: "pass", ReviewerID: 9, ReviewedAt: now},
	}
	require.NoError(t, db.Create(&reviews).Error)

	require.NoError(t, backfillTicketQualityReviewSnapshots(db))

	var got []models.TicketQualityReview
	require.NoError(t, db.Order("id").Find(&got).Error)
	require.Equal(t, int64(21), got[0].ReviewedAgentID)
	require.Equal(t, int64(7), got[0].ReviewedTeamID)
	require.Equal(t, "fail", got[0].FinalOutcome)
	require.Equal(t, "pass", got[1].FinalOutcome, "falls back to result when outcome is empty")
	require.Equal(t, int64(30), got[2].ReviewedAgentID, "existing snapshot is kept")
	require.Equal(t, "pass", got[2].FinalOutcome, "existing final outcome is kept")
	require.Zero(t, got[3].ReviewedAgentID)
	require.True(t, db.Migrator().HasTable(&models.TicketQualityDispute{}))
}
