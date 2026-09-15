package services

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
)

func TestTicketDuplicateParentPreservesIdentityClocksAndAudit(t *testing.T) {
	db, op, child, parent := duplicateFixture(t)
	require.NoError(t, db.AutoMigrate(&models.Conversation{}, &models.Asset{}))
	conversation := models.Conversation{TenantID: child.TenantID, CustomerID: child.CustomerID, Status: enums.IMConversationStatusActive}
	require.NoError(t, db.Create(&conversation).Error)
	asset := models.Asset{TenantID: child.TenantID, ConversationID: conversation.ID, AssetID: "original-asset", StorageKey: "original-key", Filename: "错误截图.png"}
	require.NoError(t, db.Create(&asset).Error)
	child.ConversationID = conversation.ID
	deadline := time.Now().Add(-time.Hour)
	child.SLADueAt, child.AcceptDeadlineAt = &deadline, &deadline
	require.NoError(t, db.Save(&child).Error)
	progress := models.TicketProgress{TenantID: child.TenantID, TicketID: child.ID, Content: "原始复现步骤", EventType: "progress", CreatedAt: time.Now()}
	require.NoError(t, db.Create(&progress).Error)
	require.NoError(t, db.First(&child, child.ID).Error)
	require.NoError(t, db.First(&parent, parent.ID).Error)
	require.NoError(t, db.First(&progress, progress.ID).Error)
	require.NoError(t, db.First(&asset, asset.ID).Error)
	cmd := duplicateCommand(t, db, child, parent)
	first := executeGovernance(t, child.ID, cmd, op)
	require.Equal(t, first, executeGovernance(t, child.ID, cmd, op))
	for _, original := range []models.Ticket{child, parent} {
		stored := repositories.TicketRepository.Get(db, original.ID)
		// The only ticket fields changed by association are revision and update time.
		stored.GovernanceRevision = original.GovernanceRevision
		stored.UpdatedAt = original.UpdatedAt
		require.Equal(t, original, *stored)
		view, err := GetTicketGovernance(original.ID, op)
		require.NoError(t, err)
		require.Len(t, view.Relations, 1)
		require.Equal(t, "parent", view.Relations[0].Kind)
		require.Equal(t, parent.ID, view.Relations[0].SourceID)
		require.Equal(t, child.ID, view.Relations[0].TargetID)
		require.False(t, view.Merge.CanMerge)
		require.Len(t, view.History, 1)
		var audit map[string]any
		require.NoError(t, json.Unmarshal(view.History[0].Details, &audit))
		require.Equal(t, false, audit["clock_reset"])
		require.Equal(t, "unchanged", audit["sla_effect"])
		snapshot := audit["sla_snapshot"].(map[string]any)
		require.Equal(t, snapshot["previous_sla_deadline"], snapshot["sla_deadline"])
		require.Equal(t, snapshot["previous_accept_deadline"], snapshot["accept_deadline"])
		if original.ID == child.ID {
			require.Equal(t, true, snapshot["resolution_overdue"])
			require.Equal(t, true, snapshot["accept_overdue"])
		}
	}
	for role, expected := range map[string]int64{"parent": parent.ID, "child": child.ID} {
		page, err := EnterpriseTicketService.List(child.TenantID, EnterpriseTicketQuery{RelationRole: role})
		require.NoError(t, err)
		require.Len(t, page.Items, 1)
		require.Equal(t, expected, page.Items[0].ID)
	}
	standalone, err := EnterpriseTicketService.List(child.TenantID, EnterpriseTicketQuery{RelationRole: "standalone"})
	require.NoError(t, err)
	require.Empty(t, standalone.Items)
	var unchangedAsset models.Asset
	require.NoError(t, db.First(&unchangedAsset, asset.ID).Error)
	require.Equal(t, asset, unchangedAsset)
	var unchangedProgress models.TicketProgress
	require.NoError(t, db.First(&unchangedProgress, progress.ID).Error)
	require.Equal(t, progress, unchangedProgress)
	for _, key := range []string{"", "duplicate-conversation-retry"} {
		created, err := TicketService.CreateFromConversation(request.CreateTicketFromConversationRequest{ConversationID: conversation.ID, IdempotencyKey: key}, op)
		require.NoError(t, err)
		require.Equal(t, child.ID, created.ID, "conversation must retain its own child ticket")
	}
	resolved, err := ResolveWorkflowTicket(child.TenantID, child.ID)
	require.NoError(t, err)
	require.Equal(t, child.ID, resolved.ID)
	// Parents cannot be closed while a child still needs handling.
	require.Error(t, repositories.TicketRepository.Updates(db, parent.ID, map[string]any{"status": "closed", "case_status": "closed"}))
	view, err := GetTicketGovernance(child.ID, op)
	require.NoError(t, err)
	unlink := governanceCommand(t, db, child.ID, "unlink")
	unlink.RelationID = view.Relations[0].ID
	executeGovernance(t, child.ID, unlink, op)
	after, err := GetTicketGovernance(child.ID, op)
	require.NoError(t, err)
	require.Empty(t, after.Relations)
	require.Len(t, after.History, 2)
	require.JSONEq(t, string(view.History[0].Details), string(after.History[1].Details))
}

func TestTicketDuplicateParentValidationAndAtomicRollback(t *testing.T) {
	for _, scenario := range []string{"self", "later_parent", "tenant", "source_permission", "target_permission", "target_revision", "source_revision", "already_child", "nested_parent", "has_children", "parent_closed", "draft", "legacy_merge", "audit_failure", "receipt_failure"} {
		t.Run(scenario, func(t *testing.T) {
			db, op, child, parent := duplicateFixture(t)
			cmd := duplicateCommand(t, db, child, parent)
			switch scenario {
			case "self":
				cmd.TargetID = child.ID
			case "later_parent":
				require.NoError(t, db.Model(&parent).Update("created_at", child.CreatedAt.Add(time.Hour)).Error)
			case "tenant":
				require.NoError(t, db.Model(&parent).Update("tenant_id", 999).Error)
			case "source_permission":
				op.Permissions = []string{constants.PermissionTicketView.Code}
			case "target_permission":
				op.Roles = []string{EnterpriseRoleEngineer}
				op.Permissions = []string{constants.PermissionTicketView.Code, constants.PermissionTicketUpdate.Code}
				require.NoError(t, db.Model(&child).Update("current_assignee_id", op.UserID).Error)
			case "target_revision":
				wrong := int64(999)
				cmd.TargetRevision = &wrong
			case "source_revision":
				wrong := int64(999)
				cmd.ExpectedRevision = &wrong
			case "parent_closed":
				require.NoError(t, db.Model(&parent).Updates(map[string]any{"status": "closed", "case_status": "closed"}).Error)
			case "draft":
				require.NoError(t, db.Model(&child).Update("status", "draft").Error)
			case "legacy_merge":
				cmd.Action = "merge"
			case "already_child", "nested_parent", "has_children":
				source, target := int64(999), child.ID
				if scenario == "nested_parent" {
					target = parent.ID
				}
				if scenario == "has_children" {
					source, target = child.ID, 999
				}
				key := scenario
				require.NoError(t, db.Create(&models.TicketRelation{TenantID: child.TenantID, SourceID: source, TargetID: target, Kind: "parent", ActiveKey: &key, ChildKey: &target}).Error)
			case "audit_failure", "receipt_failure":
				table := "t_ticket_progress"
				if scenario == "receipt_failure" {
					table = "t_ticket_governance_operation"
				}
				require.NoError(t, db.Exec("CREATE TRIGGER fail_duplicate_write BEFORE INSERT ON "+table+" BEGIN SELECT RAISE(ABORT, 'injected failure'); END").Error)
			}
			before := repositories.TicketRepository.Get(db, child.ID)
			_, err := ExecuteTicketGovernance(child.ID, cmd, op)
			require.Error(t, err)
			if scenario == "target_revision" || scenario == "source_revision" {
				require.True(t, errors.Is(err, ErrTicketCaseConflict))
			}
			require.Equal(t, before, repositories.TicketRepository.Get(db, child.ID))
			var count int64
			require.NoError(t, db.Model(&models.TicketRelation{}).Where("source_id = ? AND target_id = ?", parent.ID, child.ID).Count(&count).Error)
			require.Zero(t, count)
		})
	}
}

func TestTicketDuplicateCandidatesAndHistoricalClosedChild(t *testing.T) {
	db, op, child, parent := duplicateFixture(t)
	// A common incident can span customers. It still requires staff confirmation.
	require.NoError(t, db.Model(&parent).Update("customer_id", parent.CustomerID+1).Error)
	foreign := parent
	foreign.ID = 0
	foreign.TenantID++
	foreign.TicketNo = "FOREIGN"
	require.NoError(t, db.Create(&foreign).Error)
	later := parent
	later.ID = 0
	later.CreatedAt = child.CreatedAt.Add(time.Hour)
	later.TicketNo = "LATER"
	require.NoError(t, db.Create(&later).Error)
	items, err := SearchTicketDuplicateCandidates(child.ID, "", op)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, parent.ID, items[0].OtherID)
	require.Equal(t, "same_title", items[0].Match)
	// Historical cancellation is not undone or used to restart a clock.
	handled := time.Now().Add(-time.Hour)
	due := handled.Add(-time.Hour)
	require.NoError(t, db.Model(&child).Updates(map[string]any{"status": "cancelled", "case_status": "cancelled", "handled_at": handled, "sla_due_at": due}).Error)
	executeGovernance(t, child.ID, duplicateCommand(t, db, child, parent), op)
	stored := repositories.TicketRepository.Get(db, child.ID)
	require.Equal(t, "cancelled", stored.CaseStatus)
	require.True(t, stored.HandledAt.Equal(handled))
	require.True(t, stored.SLADueAt.Equal(due))
	items, err = SearchTicketDuplicateCandidates(child.ID, "", op)
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestTicketDuplicateLegacyMergeIsReadOnly(t *testing.T) {
	db, op, source, main := duplicateFixture(t)
	require.NoError(t, db.AutoMigrate(&models.Conversation{}, &models.Message{}, &models.Asset{}))
	// Seed an existing historical merge; new writes are rejected by the command API.
	require.NoError(t, db.Model(&source).Updates(map[string]any{"merged_into_id": main.ID, "status": "cancelled", "case_status": "cancelled"}).Error)
	view, err := GetTicketGovernance(main.ID, op)
	require.NoError(t, err)
	require.False(t, view.Merge.CanMerge)
	require.Len(t, view.Merge.Sources, 1)
	require.Equal(t, source.Description, view.Merge.Sources[0].Description)
	resolved, err := ResolveWorkflowTicket(source.TenantID, source.ID)
	require.NoError(t, err)
	require.Equal(t, main.ID, resolved.ID)
}

func TestTicketRelationsRetiredTypesRemainReadableAndRemovable(t *testing.T) {
	for _, kind := range []string{"related", "causes"} {
		t.Run(kind, func(t *testing.T) {
			db, op, child, parent := duplicateFixture(t)
			cmd := governanceCommand(t, db, child.ID, "link")
			cmd.RelationKind, cmd.TargetID = kind, parent.ID
			_, err := ExecuteTicketGovernance(child.ID, cmd, op)
			require.Error(t, err)
			var count int64
			require.NoError(t, db.Model(&models.TicketRelation{}).Count(&count).Error)
			require.Zero(t, count)
			key := "legacy-" + kind
			row := models.TicketRelation{TenantID: child.TenantID, SourceID: child.ID, TargetID: parent.ID, Kind: kind, ActiveKey: &key, Reason: "historical relationship"}
			require.NoError(t, db.Create(&row).Error)
			view, err := GetTicketGovernance(child.ID, op)
			require.NoError(t, err)
			require.Len(t, view.Relations, 1)
			require.Equal(t, kind, view.Relations[0].Kind)
			unlink := governanceCommand(t, db, child.ID, "unlink")
			unlink.RelationID = row.ID
			executeGovernance(t, child.ID, unlink, op)
			require.NoError(t, db.First(&row, row.ID).Error)
			require.Nil(t, row.ActiveKey)
			require.NotNil(t, row.RemovedAt)
		})
	}
}
