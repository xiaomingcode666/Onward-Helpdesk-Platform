package services

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestAIWorkflowServiceValidateDefinitionReportsErrors(t *testing.T) {
	setupAIWorkflowTestDB(t)
	result := AIWorkflowService.ValidateDefinition(dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: "start"},
			{ID: "create_1", Type: "create_ticket"},
			{ID: "end_1", Type: "end"},
		},
		Edges: []dsl.Edge{
			{ID: "e1", Source: "start_1", Target: "create_1"},
			{ID: "e2", Source: "create_1", Target: "end_1"},
		},
	})

	if result.Valid {
		t.Fatalf("expected invalid workflow definition")
	}
	if len(result.Errors) == 0 {
		t.Fatalf("expected validation errors")
	}
}

func TestAIWorkflowServicePublishCreatesImmutableVersion(t *testing.T) {
	setupAIWorkflowTestDB(t)
	operator := aiWorkflowTestOperator()
	workflow, err := AIWorkflowService.CreateWorkflow(request.CreateAIWorkflowRequest{
		Name:        "support flow",
		Description: "customer service flow",
		AgentID:     12,
		Definition:  validAIWorkflowDefinition(),
	}, operator)
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}

	version, err := AIWorkflowService.PublishWorkflow(request.PublishAIWorkflowRequest{
		WorkflowID: workflow.ID,
		Definition: validAIWorkflowDefinition(),
	}, operator)
	if err != nil {
		t.Fatalf("PublishWorkflow() error = %v", err)
	}

	if version.WorkflowID != workflow.ID {
		t.Fatalf("expected workflow id %d, got %d", workflow.ID, version.WorkflowID)
	}
	if version.Version != 1 {
		t.Fatalf("expected first version to be 1, got %d", version.Version)
	}
	if version.DefinitionHash == "" {
		t.Fatalf("expected definition hash")
	}
	if version.PublishedAt == nil {
		t.Fatalf("expected published timestamp")
	}

	var stored dsl.Definition
	if err := json.Unmarshal([]byte(version.Definition), &stored); err != nil {
		t.Fatalf("unmarshal stored definition: %v", err)
	}
	if stored.EntryNodeID != "start_1" {
		t.Fatalf("unexpected stored definition: %+v", stored)
	}
}

func TestAIWorkflowServicePublishesCustomCredentialChainWithoutTemplateCode(t *testing.T) {
	setupAIWorkflowTestDB(t)
	operator := aiWorkflowTestOperator()
	definition := validAIWorkflowDefinition()
	want := []string{dsl.ModelCredentialScopeTenantDefault, dsl.ModelCredentialScopeProduct}
	definition.ModelPolicy = &dsl.ModelPolicy{CredentialChain: want}
	workflow, err := AIWorkflowService.CreateWorkflow(request.CreateAIWorkflowRequest{
		Name:       "custom credential order",
		AgentID:    12,
		Definition: definition,
	}, operator)
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}

	version, err := AIWorkflowService.PublishWorkflow(request.PublishAIWorkflowRequest{
		WorkflowID: workflow.ID,
		Definition: definition,
	}, operator)
	if err != nil {
		t.Fatalf("PublishWorkflow() error = %v", err)
	}
	var stored dsl.Definition
	if err := json.Unmarshal([]byte(version.Definition), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.ModelPolicy == nil || !reflect.DeepEqual(stored.ModelPolicy.CredentialChain, want) {
		t.Fatalf("stored credential chain = %#v, want %#v", stored.ModelPolicy, want)
	}
}

func TestAIWorkflowServicePublishIncrementsVersionWhenDefinitionChanges(t *testing.T) {
	setupAIWorkflowTestDB(t)
	operator := aiWorkflowTestOperator()
	workflow, err := AIWorkflowService.CreateWorkflow(request.CreateAIWorkflowRequest{
		Name:       "support flow versions",
		AgentID:    99,
		Definition: validAIWorkflowDefinition(),
	}, operator)
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}

	first, err := AIWorkflowService.PublishWorkflow(request.PublishAIWorkflowRequest{
		WorkflowID: workflow.ID,
		Definition: validAIWorkflowDefinition(),
	}, operator)
	if err != nil {
		t.Fatalf("PublishWorkflow() first error = %v", err)
	}
	changed := validAIWorkflowDefinition()
	changed.Nodes[1].Config = json.RawMessage(`{"text":"changed"}`)
	second, err := AIWorkflowService.PublishWorkflow(request.PublishAIWorkflowRequest{
		WorkflowID: workflow.ID,
		Definition: changed,
	}, operator)
	if err != nil {
		t.Fatalf("PublishWorkflow() second error = %v", err)
	}

	if first.Version != 1 || second.Version != 2 {
		t.Fatalf("expected versions 1 and 2, got %d and %d", first.Version, second.Version)
	}
}

func TestAIWorkflowServicePublishReusesIdenticalStableVersion(t *testing.T) {
	setupAIWorkflowTestDB(t)
	operator := aiWorkflowTestOperator()
	definition := validAIWorkflowDefinition()
	workflow, err := AIWorkflowService.CreateWorkflow(request.CreateAIWorkflowRequest{
		Name:       "idempotent publish",
		AgentID:    99,
		Definition: definition,
	}, operator)
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}

	first, err := AIWorkflowService.PublishWorkflow(request.PublishAIWorkflowRequest{WorkflowID: workflow.ID, Definition: definition}, operator)
	if err != nil {
		t.Fatalf("PublishWorkflow() first error = %v", err)
	}
	second, err := AIWorkflowService.PublishWorkflow(request.PublishAIWorkflowRequest{WorkflowID: workflow.ID, Definition: definition}, operator)
	if err != nil {
		t.Fatalf("PublishWorkflow() second error = %v", err)
	}
	if second.ID != first.ID || second.Version != 1 {
		t.Fatalf("identical publish created another version: first=%+v second=%+v", first, second)
	}
	versions := repositories.AIWorkflowVersionRepository.Find(sqls.DB(), sqls.NewCnd().Eq("workflow_id", workflow.ID))
	if len(versions) != 1 {
		t.Fatalf("version count = %d, want 1", len(versions))
	}
}

func TestAIWorkflowServiceRollbackCreatesNewImmutableStableVersion(t *testing.T) {
	setupAIWorkflowTestDB(t)
	operator := aiWorkflowTestOperator()
	firstDefinition := validAIWorkflowDefinition()
	firstJSON, err := json.Marshal(firstDefinition)
	if err != nil {
		t.Fatal(err)
	}
	workflow := models.AIWorkflow{
		TenantID:        operator.TenantID,
		Code:            "tenant_rollback_test",
		Scope:           models.AIWorkflowScopeTenant,
		Name:            "rollback test",
		Status:          enums.StatusOk,
		DraftDefinition: string(firstJSON),
	}
	if err := sqls.DB().Create(&workflow).Error; err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	first, err := AIWorkflowService.PublishWorkflow(request.PublishAIWorkflowRequest{
		WorkflowID: workflow.ID,
		Definition: firstDefinition,
	}, operator)
	if err != nil {
		t.Fatalf("publish first version: %v", err)
	}
	secondDefinition := validAIWorkflowDefinition()
	secondDefinition.Nodes[1].Config = json.RawMessage(`{"text":"changed"}`)
	second, err := AIWorkflowService.PublishWorkflow(request.PublishAIWorkflowRequest{
		WorkflowID: workflow.ID,
		Definition: secondDefinition,
	}, operator)
	if err != nil {
		t.Fatalf("publish second version: %v", err)
	}
	if err := sqls.DB().Model(&models.AIAgent{}).Where("id = ?", 12).Updates(map[string]any{
		"workflow_id":         workflow.ID,
		"workflow_version_id": second.ID,
	}).Error; err != nil {
		t.Fatalf("bind agent to second version: %v", err)
	}

	rolledBack, err := AIWorkflowService.RollbackWorkflowVersion(workflow.ID, first.ID, operator)
	if err != nil {
		t.Fatalf("RollbackWorkflowVersion() error = %v", err)
	}
	if rolledBack.Version != 3 || rolledBack.SourceVersionID != first.ID {
		t.Fatalf("rollback version = %+v, want V3 sourced from %d", rolledBack, first.ID)
	}
	if rolledBack.DefinitionHash != first.DefinitionHash || rolledBack.ChangeSummary != "回滚至 V1" {
		t.Fatalf("rollback did not preserve source definition: %+v", rolledBack)
	}
	storedWorkflow := repositories.AIWorkflowRepository.Get(sqls.DB(), workflow.ID)
	if storedWorkflow == nil || storedWorkflow.CurrentStableVersionID != rolledBack.ID || storedWorkflow.DraftDefinition != first.Definition {
		t.Fatalf("workflow was not advanced to rollback version: %+v", storedWorkflow)
	}
	boundAgent := repositories.AIAgentRepository.Get(sqls.DB(), 12)
	if boundAgent == nil || boundAgent.WorkflowVersionID != second.ID {
		t.Fatalf("rollback must not update product bindings: %+v", boundAgent)
	}
	storedFirst := repositories.AIWorkflowVersionRepository.Get(sqls.DB(), first.ID)
	storedSecond := repositories.AIWorkflowVersionRepository.Get(sqls.DB(), second.ID)
	if storedFirst == nil || storedSecond == nil || storedFirst.Status != enums.StatusOk || storedSecond.Status != enums.StatusOk {
		t.Fatalf("historical versions were mutated: first=%+v second=%+v", storedFirst, storedSecond)
	}
	if _, err := AIWorkflowService.RollbackWorkflowVersion(workflow.ID, rolledBack.ID, operator); err == nil {
		t.Fatal("expected rollback to current stable version to fail")
	}
}

func TestAIWorkflowServicePublishRejectsInvalidDSL(t *testing.T) {
	setupAIWorkflowTestDB(t)
	operator := aiWorkflowTestOperator()
	workflow, err := AIWorkflowService.CreateWorkflow(request.CreateAIWorkflowRequest{
		Name:       "invalid publish flow",
		AgentID:    23,
		Definition: validAIWorkflowDefinition(),
	}, operator)
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}

	_, err = AIWorkflowService.PublishWorkflow(request.PublishAIWorkflowRequest{
		WorkflowID: workflow.ID,
		Definition: dsl.Definition{
			SchemaVersion: 1,
			EntryNodeID:   "start_1",
			Nodes: []dsl.Node{
				{ID: "start_1", Type: "start"},
				{ID: "create_1", Type: "create_ticket"},
				{ID: "end_1", Type: "end"},
			},
			Edges: []dsl.Edge{
				{ID: "e1", Source: "start_1", Target: "create_1"},
				{ID: "e2", Source: "create_1", Target: "end_1"},
			},
		},
	}, operator)
	if err == nil {
		t.Fatalf("expected invalid publish to fail")
	}
	if versions := repositories.AIWorkflowVersionRepository.Find(sqls.DB(), sqls.NewCnd().Eq("workflow_id", workflow.ID)); len(versions) != 0 {
		t.Fatalf("expected no versions after invalid publish, got %d", len(versions))
	}
}

func TestAIWorkflowServiceRejectsPublishingEmbeddedPlatformWorkflow(t *testing.T) {
	setupAIWorkflowTestDB(t)
	workflow, _, err := AIWorkflowService.EnsurePlatformDefaultWorkflowDB(sqls.DB())
	if err != nil {
		t.Fatalf("EnsurePlatformDefaultWorkflowDB() error = %v", err)
	}
	operator := &dto.AuthPrincipal{UserID: 1, Username: "platform-admin", DomainType: models.DomainTypePlatform}
	if _, err := AIWorkflowService.PublishWorkflow(request.PublishAIWorkflowRequest{
		WorkflowID: workflow.ID,
		Definition: AIWorkflowService.DefaultAgentWorkflowDefinition(),
	}, operator); err == nil {
		t.Fatal("expected embedded platform workflow publish to be rejected")
	}
}

func TestEmbeddedPlatformWorkflowUpgradeKeepsAgentReleaseOnReviewedVersion(t *testing.T) {
	setupAIWorkflowTestDB(t)
	workflow, firstVersion, err := AIWorkflowService.EnsurePlatformDefaultWorkflowDB(sqls.DB())
	if err != nil {
		t.Fatalf("EnsurePlatformDefaultWorkflowDB() error = %v", err)
	}
	agent := repositories.AIAgentRepository.Get(sqls.DB(), 12)
	if agent == nil {
		t.Fatal("test agent is missing")
	}
	agent.ProductID = 101
	agent.WorkflowID = workflow.ID
	agent.WorkflowVersionID = firstVersion.ID
	release := &models.AIAgentRelease{
		TenantID:          agent.TenantID,
		ProductID:         agent.ProductID,
		AgentID:           agent.ID,
		ReleaseNo:         1,
		WorkflowID:        workflow.ID,
		WorkflowVersionID: firstVersion.ID,
		ReviewStatus:      enums.AIAgentReviewStatusApproved,
		DeploymentStatus:  models.AIAgentReleaseDeploymentActive,
		Status:            enums.StatusOk,
	}
	if err := sqls.DB().Create(release).Error; err != nil {
		t.Fatalf("create reviewed release: %v", err)
	}
	if err := sqls.DB().Model(&models.AIAgent{}).Where("id = ?", agent.ID).Updates(map[string]any{
		"product_id":          agent.ProductID,
		"workflow_id":         workflow.ID,
		"workflow_version_id": firstVersion.ID,
		"active_release_id":   release.ID,
	}).Error; err != nil {
		t.Fatalf("bind reviewed release: %v", err)
	}

	spec, ok, err := platformBuiltInWorkflowSpec(workflow.Code)
	if err != nil || !ok {
		t.Fatalf("platformBuiltInWorkflowSpec() = ok %v error %v", ok, err)
	}
	spec.definition.Nodes[0].Name = spec.definition.Nodes[0].Name + " manifest upgrade"
	secondWorkflow, secondVersion, err := AIWorkflowService.ensurePlatformBuiltInWorkflowDB(sqls.DB(), spec)
	if err != nil {
		t.Fatalf("ensure manifest upgrade: %v", err)
	}
	if secondWorkflow.ID != workflow.ID || secondVersion.ID == firstVersion.ID || secondVersion.Version != firstVersion.Version+1 {
		t.Fatalf("unexpected manifest version upgrade: workflow=%+v version=%+v", secondWorkflow, secondVersion)
	}
	storedAgent := repositories.AIAgentRepository.Get(sqls.DB(), agent.ID)
	storedRelease := repositories.AIAgentReleaseRepository.Get(sqls.DB(), release.ID)
	if storedAgent == nil || storedAgent.ActiveReleaseID != release.ID || storedAgent.WorkflowVersionID != firstVersion.ID {
		t.Fatalf("manifest upgrade changed agent deployment: %+v", storedAgent)
	}
	if storedRelease == nil || storedRelease.DeploymentStatus != models.AIAgentReleaseDeploymentActive || storedRelease.WorkflowVersionID != firstVersion.ID {
		t.Fatalf("manifest upgrade changed reviewed release: %+v", storedRelease)
	}
	_, repeatedVersion, err := AIWorkflowService.ensurePlatformBuiltInWorkflowDB(sqls.DB(), spec)
	if err != nil {
		t.Fatalf("repeat manifest upgrade: %v", err)
	}
	if repeatedVersion.ID != secondVersion.ID {
		t.Fatalf("idempotent manifest sync created another version: first=%d repeated=%d", secondVersion.ID, repeatedVersion.ID)
	}
}

func TestAIWorkflowServiceDeleteWorkflowRetainsImmutableVersions(t *testing.T) {
	setupAIWorkflowTestDB(t)
	operator := aiWorkflowTestOperator()
	now := time.Now()
	workflow := models.AIWorkflow{
		TenantID:        operator.TenantID,
		Code:            "tenant_delete_test",
		Scope:           models.AIWorkflowScopeTenant,
		Name:            "待删除流程",
		Status:          enums.StatusOk,
		DraftDefinition: `{}`,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := sqls.DB().Create(&workflow).Error; err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	version := models.AIWorkflowVersion{
		WorkflowID:     workflow.ID,
		Version:        1,
		Status:         enums.StatusOk,
		Definition:     `{}`,
		DefinitionHash: "delete-version-hash",
		ReleaseChannel: models.AIWorkflowReleaseChannelStable,
		SchemaVersion:  1,
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := sqls.DB().Create(&version).Error; err != nil {
		t.Fatalf("create workflow version: %v", err)
	}
	release := models.AIAgentRelease{
		TenantID: 1, AgentID: 12, ReleaseNo: 1, WorkflowID: workflow.ID, WorkflowVersionID: version.ID,
		DeploymentStatus: models.AIAgentReleaseDeploymentRetired, Status: enums.StatusOk,
	}
	if err := sqls.DB().Create(&release).Error; err != nil {
		t.Fatalf("create retired workflow release: %v", err)
	}

	if err := AIWorkflowService.DeleteWorkflow(workflow.ID, operator); err != nil {
		t.Fatalf("DeleteWorkflow() error = %v", err)
	}

	var refreshedWorkflow models.AIWorkflow
	if err := sqls.DB().First(&refreshedWorkflow, workflow.ID).Error; err != nil {
		t.Fatalf("load workflow: %v", err)
	}
	if refreshedWorkflow.Status != enums.StatusDeleted {
		t.Fatalf("workflow status = %d, want deleted", refreshedWorkflow.Status)
	}
	var refreshedVersion models.AIWorkflowVersion
	if err := sqls.DB().First(&refreshedVersion, version.ID).Error; err != nil {
		t.Fatalf("load workflow version: %v", err)
	}
	if refreshedVersion.Status != enums.StatusOk {
		t.Fatalf("workflow version status = %d, want immutable history to remain available", refreshedVersion.Status)
	}
	var refreshedRelease models.AIAgentRelease
	if err := sqls.DB().First(&refreshedRelease, release.ID).Error; err != nil {
		t.Fatalf("load retired workflow release: %v", err)
	}
	if refreshedRelease.Status != enums.StatusOk || refreshedRelease.DeploymentStatus != models.AIAgentReleaseDeploymentRetired {
		t.Fatalf("retired workflow release was mutated: %+v", refreshedRelease)
	}
}

func TestAIWorkflowServiceDeleteWorkflowRejectsOperationalReferences(t *testing.T) {
	for _, testCase := range []struct {
		name string
		seed func(t *testing.T, workflowID, versionID int64)
	}{
		{
			name: "agent draft",
			seed: func(t *testing.T, workflowID, versionID int64) {
				t.Helper()
				if err := sqls.DB().Model(&models.AIAgent{}).Where("id = ?", 12).Updates(map[string]any{
					"workflow_id": workflowID, "workflow_version_id": versionID,
				}).Error; err != nil {
					t.Fatalf("bind agent draft: %v", err)
				}
			},
		},
		{
			name: "agent release",
			seed: func(t *testing.T, workflowID, versionID int64) {
				t.Helper()
				if err := sqls.DB().Create(&models.AIAgentRelease{
					TenantID: 1, AgentID: 12, ReleaseNo: 1, WorkflowID: workflowID, WorkflowVersionID: versionID,
					DeploymentStatus: models.AIAgentReleaseDeploymentActive, Status: enums.StatusOk,
				}).Error; err != nil {
					t.Fatalf("create agent release: %v", err)
				}
			},
		},
		{
			name: "product default",
			seed: func(t *testing.T, workflowID, versionID int64) {
				t.Helper()
				if err := sqls.DB().Create(&models.ProductServiceProfile{
					TenantID: 1, ProductID: 88, DefaultFlowTemplateID: workflowID, Status: enums.StatusOk,
				}).Error; err != nil {
					t.Fatalf("create product service profile: %v", err)
				}
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			setupAIWorkflowTestDB(t)
			operator := aiWorkflowTestOperator()
			now := time.Now()
			workflow := models.AIWorkflow{
				TenantID: 1, Code: "referenced_delete_test", Scope: models.AIWorkflowScopeTenant,
				Name: "被引用流程", Status: enums.StatusOk, DraftDefinition: `{}`,
				AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
			}
			if err := sqls.DB().Create(&workflow).Error; err != nil {
				t.Fatalf("create workflow: %v", err)
			}
			version := models.AIWorkflowVersion{
				WorkflowID: workflow.ID, Version: 1, Status: enums.StatusOk, Definition: `{}`,
				DefinitionHash: "reference-version-hash", ReleaseChannel: models.AIWorkflowReleaseChannelStable,
				SchemaVersion: 1, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
			}
			if err := sqls.DB().Create(&version).Error; err != nil {
				t.Fatalf("create workflow version: %v", err)
			}
			testCase.seed(t, workflow.ID, version.ID)

			if err := AIWorkflowService.DeleteWorkflow(workflow.ID, operator); err == nil {
				t.Fatal("expected referenced workflow retirement to fail")
			}
			stored := repositories.AIWorkflowRepository.Get(sqls.DB(), workflow.ID)
			if stored == nil || stored.Status != enums.StatusOk {
				t.Fatalf("referenced workflow was retired: %+v", stored)
			}
		})
	}
}

func TestAIWorkflowServiceReusableTemplateAndAgentBindingPreserveProductionRelease(t *testing.T) {
	setupAIWorkflowTestDB(t)
	operator := aiWorkflowTestOperator()
	definitionJSON, err := json.Marshal(validAIWorkflowDefinition())
	if err != nil {
		t.Fatalf("marshal definition: %v", err)
	}
	platformWorkflow := models.AIWorkflow{
		TenantID:        0,
		Code:            "platform_default_test",
		Scope:           models.AIWorkflowScopePlatform,
		Name:            "平台默认流程",
		Status:          enums.StatusOk,
		DraftDefinition: string(definitionJSON),
		Locked:          true,
	}
	if err := sqls.DB().Create(&platformWorkflow).Error; err != nil {
		t.Fatalf("create platform workflow: %v", err)
	}
	now := time.Now()
	platformVersion := models.AIWorkflowVersion{
		WorkflowID:     platformWorkflow.ID,
		Version:        1,
		Status:         enums.StatusOk,
		Definition:     string(definitionJSON),
		DefinitionHash: "platform-hash",
		ReleaseChannel: models.AIWorkflowReleaseChannelStable,
		PublishedAt:    &now,
	}
	if err := sqls.DB().Create(&platformVersion).Error; err != nil {
		t.Fatalf("create platform version: %v", err)
	}
	if err := sqls.DB().Model(&platformWorkflow).Updates(map[string]any{
		"published_version_id":      platformVersion.ID,
		"current_stable_version_id": platformVersion.ID,
	}).Error; err != nil {
		t.Fatalf("mark platform stable version: %v", err)
	}

	template, err := AIWorkflowService.CreateReusableTemplateFromVersion(request.CreateAIWorkflowTemplateRequest{
		Name:            "企业售后流程",
		Description:     "tenant reusable workflow",
		SourceVersionID: platformVersion.ID,
	}, operator)
	if err != nil {
		t.Fatalf("CreateReusableTemplateFromVersion() error = %v", err)
	}
	if template.TenantID != operator.TenantID || template.Scope != models.AIWorkflowScopeTenant || template.AgentID != 0 || template.Locked {
		t.Fatalf("unexpected reusable template: %#v", template)
	}
	if template.SourceWorkflowID != platformWorkflow.ID || template.SourceVersionID != platformVersion.ID {
		t.Fatalf("source lineage was not preserved: %#v", template)
	}

	tenantVersion, err := AIWorkflowService.PublishWorkflow(request.PublishAIWorkflowRequest{
		WorkflowID: template.ID,
		Definition: validAIWorkflowDefinition(),
	}, operator)
	if err != nil {
		t.Fatalf("PublishWorkflow() error = %v", err)
	}
	if tenantVersion.ReleaseChannel != models.AIWorkflowReleaseChannelStable {
		t.Fatalf("release channel = %q, want stable", tenantVersion.ReleaseChannel)
	}

	if err := sqls.DB().Model(&models.AIAgent{}).Where("id = ?", 12).Updates(map[string]any{
		"active_release_id": 777,
		"draft_revision":    4,
		"review_status":     enums.AIAgentReviewStatusApproved,
	}).Error; err != nil {
		t.Fatalf("seed agent production state: %v", err)
	}
	if err := AIWorkflowService.BindAgentWorkflowVersion(12, tenantVersion.ID, operator); err != nil {
		t.Fatalf("BindAgentWorkflowVersion() error = %v", err)
	}
	bound := repositories.AIAgentRepository.Get(sqls.DB(), 12)
	if bound == nil {
		t.Fatal("bound agent missing")
	}
	if bound.WorkflowID != template.ID || bound.WorkflowVersionID != tenantVersion.ID {
		t.Fatalf("unexpected binding: workflow=%d version=%d", bound.WorkflowID, bound.WorkflowVersionID)
	}
	if bound.ActiveReleaseID != 777 {
		t.Fatalf("active release changed: got %d, want 777", bound.ActiveReleaseID)
	}
	if bound.DraftRevision != 5 || bound.ReviewStatus != enums.AIAgentReviewStatusUnreviewed {
		t.Fatalf("draft governance not reset: revision=%d review=%q", bound.DraftRevision, bound.ReviewStatus)
	}
}

func TestAIWorkflowServiceReusableTemplateClassificationDoesNotDependOnCode(t *testing.T) {
	operator := aiWorkflowTestOperator()
	platformTemplate := &models.AIWorkflow{
		TenantID: 0,
		Code:     "future_platform_template",
		Scope:    models.AIWorkflowScopePlatform,
		AgentID:  0,
		Status:   enums.StatusOk,
		Locked:   true,
	}
	if !AIWorkflowService.IsReusableTemplateForOperator(platformTemplate, operator) {
		t.Fatal("expected any locked platform template to be reusable without code coupling")
	}
	platformTemplate.Locked = false
	if AIWorkflowService.IsReusableTemplateForOperator(platformTemplate, operator) {
		t.Fatal("expected unlocked platform draft to remain unavailable")
	}
	tenantTemplate := &models.AIWorkflow{
		TenantID: operator.TenantID,
		Code:     "customer_defined_flow",
		Scope:    models.AIWorkflowScopeTenant,
		AgentID:  0,
		Status:   enums.StatusOk,
	}
	if !AIWorkflowService.IsReusableTemplateForOperator(tenantTemplate, operator) {
		t.Fatal("expected tenant-owned customer workflow to be reusable")
	}
}

func TestAIWorkflowServiceBindingRejectsCrossTenantAgent(t *testing.T) {
	setupAIWorkflowTestDB(t)
	definitionJSON, err := json.Marshal(validAIWorkflowDefinition())
	if err != nil {
		t.Fatalf("marshal definition: %v", err)
	}
	workflow := models.AIWorkflow{
		TenantID:        1,
		Code:            "tenant_shared_test",
		Scope:           models.AIWorkflowScopeTenant,
		Name:            "租户流程",
		Status:          enums.StatusOk,
		DraftDefinition: string(definitionJSON),
	}
	if err := sqls.DB().Create(&workflow).Error; err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	version := models.AIWorkflowVersion{
		WorkflowID:     workflow.ID,
		Version:        1,
		Status:         enums.StatusOk,
		Definition:     string(definitionJSON),
		DefinitionHash: "tenant-hash",
		ReleaseChannel: models.AIWorkflowReleaseChannelStable,
	}
	if err := sqls.DB().Create(&version).Error; err != nil {
		t.Fatalf("create version: %v", err)
	}
	otherTenant := &dto.AuthPrincipal{
		UserID:     2,
		Username:   "tenant-2",
		TenantID:   2,
		DomainType: "enterprise",
		Domain:     "enterprise",
	}
	if err := AIWorkflowService.BindAgentWorkflowVersion(12, version.ID, otherTenant); err == nil {
		t.Fatal("expected cross-tenant binding to fail")
	}
}

func TestAIWorkflowServiceRunListAndDetail(t *testing.T) {
	setupAIWorkflowTestDB(t)
	now := time.Now()
	agent := models.AIAgent{Name: "售后 Agent", Status: enums.StatusOk}
	if err := sqls.DB().Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	workflow := models.AIWorkflow{Name: "售后流程", AgentID: agent.ID, Status: enums.StatusOk}
	if err := sqls.DB().Create(&workflow).Error; err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	versionDefinition := validAIWorkflowDefinition()
	versionDefinition.Nodes[1].Name = "运行时回复"
	versionDefinitionJSON, err := json.Marshal(versionDefinition)
	if err != nil {
		t.Fatalf("marshal version definition: %v", err)
	}
	version := models.AIWorkflowVersion{
		WorkflowID: workflow.ID,
		Version:    7,
		Status:     enums.StatusOk,
		Definition: string(versionDefinitionJSON),
	}
	if err := sqls.DB().Create(&version).Error; err != nil {
		t.Fatalf("create workflow version: %v", err)
	}
	run := models.AIWorkflowRun{
		WorkflowID:        workflow.ID,
		WorkflowVersionID: version.ID,
		ConversationID:    303,
		AIAgentID:         agent.ID,
		MessageID:         404,
		Status:            1,
		StartedAt:         now,
		EndedAt:           &now,
	}
	if err := sqls.DB().Create(&run).Error; err != nil {
		t.Fatalf("create workflow run: %v", err)
	}
	otherRun := models.AIWorkflowRun{
		WorkflowID:        workflow.ID,
		WorkflowVersionID: version.ID,
		ConversationID:    999,
		AIAgentID:         agent.ID,
		MessageID:         505,
		Status:            1,
		StartedAt:         now,
	}
	if err := sqls.DB().Create(&otherRun).Error; err != nil {
		t.Fatalf("create other workflow run: %v", err)
	}
	nodes := []models.AIWorkflowNodeRun{
		{
			WorkflowRunID: run.ID,
			NodeID:        "start_1",
			NodeType:      "start",
			Status:        1,
			InputPreview:  `{"inputs":{}}`,
			OutputPreview: `{"messageId":404}`,
			StartedAt:     now,
			EndedAt:       &now,
		},
		{
			WorkflowRunID: run.ID,
			NodeID:        "reply_1",
			NodeType:      "llm_reply",
			Status:        1,
			OutputPreview: `{"replyText":"hello"}`,
			StartedAt:     now,
			EndedAt:       &now,
			DurationMS:    8,
		},
	}
	if err := sqls.DB().Create(&nodes).Error; err != nil {
		t.Fatalf("create workflow node runs: %v", err)
	}

	list, paging := AIWorkflowService.FindRunPageByCnd(sqls.NewCnd().Eq("conversation_id", 303).Desc("id").Page(1, 20))
	if paging.Total != 1 || len(list) != 1 || list[0].ID != run.ID {
		t.Fatalf("unexpected run list: total=%d list=%#v", paging.Total, list)
	}
	auditItems := AIWorkflowService.BuildRunAuditItems(list)
	if len(auditItems) != 1 {
		t.Fatalf("unexpected audit item count: %d", len(auditItems))
	}
	if auditItems[0].Workflow == nil || auditItems[0].Workflow.Name != workflow.Name {
		t.Fatalf("expected workflow context, got %#v", auditItems[0].Workflow)
	}
	if auditItems[0].Version == nil || auditItems[0].Version.Version != version.Version {
		t.Fatalf("expected version context, got %#v", auditItems[0].Version)
	}
	if auditItems[0].Agent == nil || auditItems[0].Agent.Name != agent.Name {
		t.Fatalf("expected agent context, got %#v", auditItems[0].Agent)
	}

	detail, nodeRuns := AIWorkflowService.GetRunDetail(run.ID)
	if detail == nil || detail.ID != run.ID {
		t.Fatalf("unexpected detail run: %#v", detail)
	}
	if len(nodeRuns) != 2 || nodeRuns[0].NodeID != "start_1" || nodeRuns[1].NodeID != "reply_1" {
		t.Fatalf("unexpected detail nodes: %#v", nodeRuns)
	}
	if missing, missingNodes := AIWorkflowService.GetRunDetail(999999); missing != nil || len(missingNodes) != 0 {
		t.Fatalf("expected missing detail to be empty, got run=%#v nodes=%#v", missing, missingNodes)
	}
}

func TestAIWorkflowServiceBuildRunHumanHandlingAudit(t *testing.T) {
	setupAIWorkflowTestDB(t)
	handoffAt := time.Date(2026, 7, 30, 21, 4, 10, 0, time.Local)
	firstReplyAt := handoffAt.Add(4 * time.Second)
	conversation := models.Conversation{
		Status:            enums.IMConversationStatusClosed,
		CurrentAssigneeID: 25,
		HandoffAt:         &handoffAt,
		HandoffReason:     "客户请求人工",
	}
	if err := sqls.DB().Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	sourceMessage := models.Message{
		ConversationID: conversation.ID,
		ClientMsgID:    "workflow-source",
		SenderType:     enums.IMSenderTypeCustomer,
		MessageType:    enums.IMMessageTypeText,
		Content:        "转人工",
		SendStatus:     enums.IMMessageStatusSent,
	}
	if err := sqls.DB().Create(&sourceMessage).Error; err != nil {
		t.Fatalf("create source message: %v", err)
	}
	failedReply := models.Message{
		ConversationID: conversation.ID,
		ClientMsgID:    "failed-agent-reply",
		SenderType:     enums.IMSenderTypeAgent,
		SenderID:       24,
		MessageType:    enums.IMMessageTypeText,
		Content:        "发送失败",
		SendStatus:     enums.IMMessageStatusFailed,
	}
	if err := sqls.DB().Create(&failedReply).Error; err != nil {
		t.Fatalf("create failed agent reply: %v", err)
	}
	firstReply := models.Message{
		ConversationID: conversation.ID,
		ClientMsgID:    "first-agent-reply",
		SenderType:     enums.IMSenderTypeAgent,
		SenderID:       25,
		MessageType:    enums.IMMessageTypeText,
		Content:        "已接入，正在处理",
		SendStatus:     enums.IMMessageStatusSent,
		SentAt:         &firstReplyAt,
	}
	if err := sqls.DB().Create(&firstReply).Error; err != nil {
		t.Fatalf("create first agent reply: %v", err)
	}
	if err := sqls.DB().Create(&models.AgentProfile{
		TenantID:    1,
		UserID:      25,
		AgentCode:   "agent-25",
		DisplayName: "产品测试1值班工程师",
		Status:      enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create agent profile: %v", err)
	}

	audit := AIWorkflowService.BuildRunHumanHandlingAudit(&models.AIWorkflowRun{
		ConversationID: conversation.ID,
		MessageID:      sourceMessage.ID,
		StartedAt:      handoffAt.Add(-time.Second),
	})
	if audit == nil {
		t.Fatal("expected human handling audit")
	}
	if !audit.HandoffOccurred || !audit.HandledByHuman {
		t.Fatalf("expected completed human takeover, got %#v", audit)
	}
	if audit.HandlerUserID != 25 || audit.HandlerName != "产品测试1值班工程师" {
		t.Fatalf("unexpected handler: %#v", audit)
	}
	if audit.FirstHumanReplyMessageID != firstReply.ID || !audit.FirstHumanReplyAt.Equal(firstReplyAt) {
		t.Fatalf("unexpected first human reply: %#v", audit)
	}
	if audit.ConversationStatus != enums.IMConversationStatusClosed || audit.ConversationStatusName != "已关闭" {
		t.Fatalf("unexpected conversation status: %#v", audit)
	}
	if audit.HandoffReason != "客户请求人工" || !audit.HandoffAt.Equal(handoffAt) {
		t.Fatalf("unexpected handoff context: %#v", audit)
	}
}

func TestAIWorkflowServiceBuildRunHumanHandlingAuditDoesNotUseReplyAfterNextRun(t *testing.T) {
	setupAIWorkflowTestDB(t)
	startedAt := time.Date(2026, 7, 30, 20, 0, 0, 0, time.Local)
	nextStartedAt := startedAt.Add(time.Minute)
	conversation := models.Conversation{
		Status:        enums.IMConversationStatusClosed,
		HandoffAt:     &nextStartedAt,
		HandoffReason: "下一轮消息请求人工",
	}
	if err := sqls.DB().Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	sourceMessage := models.Message{
		ConversationID: conversation.ID,
		ClientMsgID:    "earlier-workflow-source",
		SenderType:     enums.IMSenderTypeCustomer,
		MessageType:    enums.IMMessageTypeText,
		Content:        "先排查故障",
		SendStatus:     enums.IMMessageStatusSent,
	}
	if err := sqls.DB().Create(&sourceMessage).Error; err != nil {
		t.Fatalf("create earlier source message: %v", err)
	}
	currentRun := models.AIWorkflowRun{
		ConversationID: conversation.ID,
		MessageID:      sourceMessage.ID,
		StartedAt:      startedAt,
		Status:         1,
	}
	if err := sqls.DB().Create(&currentRun).Error; err != nil {
		t.Fatalf("create current workflow run: %v", err)
	}
	nextSourceMessage := models.Message{
		ConversationID: conversation.ID,
		ClientMsgID:    "next-workflow-source",
		SenderType:     enums.IMSenderTypeCustomer,
		MessageType:    enums.IMMessageTypeText,
		Content:        "请转人工",
		SendStatus:     enums.IMMessageStatusSent,
	}
	if err := sqls.DB().Create(&nextSourceMessage).Error; err != nil {
		t.Fatalf("create next source message: %v", err)
	}
	if err := sqls.DB().Create(&models.AIWorkflowRun{
		ConversationID: conversation.ID,
		MessageID:      nextSourceMessage.ID,
		StartedAt:      nextStartedAt,
		Status:         1,
	}).Error; err != nil {
		t.Fatalf("create next workflow run: %v", err)
	}
	firstReplyAt := nextStartedAt.Add(4 * time.Second)
	if err := sqls.DB().Create(&models.Message{
		ConversationID: conversation.ID,
		ClientMsgID:    "reply-for-next-run",
		SenderType:     enums.IMSenderTypeAgent,
		SenderID:       25,
		MessageType:    enums.IMMessageTypeText,
		Content:        "已接入",
		SendStatus:     enums.IMMessageStatusSent,
		SentAt:         &firstReplyAt,
	}).Error; err != nil {
		t.Fatalf("create reply for next run: %v", err)
	}

	if audit := AIWorkflowService.BuildRunHumanHandlingAudit(&currentRun); audit != nil {
		t.Fatalf("expected later human handling to belong to the next run, got %#v", audit)
	}
}

func TestAIWorkflowServiceBuildRunHumanHandlingAuditUsesLinkedHandledTicket(t *testing.T) {
	setupAIWorkflowTestDB(t)
	handoffAt := time.Date(2026, 8, 12, 12, 10, 0, 0, time.Local)
	acceptedAt := handoffAt.Add(3 * time.Minute)
	conversation := models.Conversation{
		Status:            enums.IMConversationStatusClosed,
		CurrentAssigneeID: 25,
		HandoffAt:         &handoffAt,
		HandoffReason:     "工单已进入人工处理",
	}
	if err := sqls.DB().Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	sourceMessage := models.Message{
		ConversationID: conversation.ID,
		ClientMsgID:    "workflow-ticket-source",
		SenderType:     enums.IMSenderTypeCustomer,
		MessageType:    enums.IMMessageTypeText,
		Content:        "需要人工工程师接入",
		SendStatus:     enums.IMMessageStatusSent,
	}
	if err := sqls.DB().Create(&sourceMessage).Error; err != nil {
		t.Fatalf("create source message: %v", err)
	}
	if err := sqls.DB().Create(&models.AgentProfile{
		TenantID:    1,
		UserID:      25,
		AgentCode:   "agent-25",
		DisplayName: "产品测试1值班工程师",
		Status:      enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create agent profile: %v", err)
	}
	if err := sqls.DB().Create(&models.Ticket{
		TenantID:          1,
		TicketNo:          "WORKFLOW-HUMAN-TICKET",
		Title:             "workflow downstream ticket",
		ConversationID:    conversation.ID,
		Status:            enums.TicketStatusProcessing,
		CurrentAssigneeID: 25,
		AcceptedAt:        &acceptedAt,
		AuditFields:       models.AuditFields{CreatedAt: handoffAt, UpdatedAt: acceptedAt},
	}).Error; err != nil {
		t.Fatalf("create linked ticket: %v", err)
	}

	audit := AIWorkflowService.BuildRunHumanHandlingAudit(&models.AIWorkflowRun{
		ConversationID: conversation.ID,
		MessageID:      sourceMessage.ID,
		StartedAt:      handoffAt.Add(-time.Second),
	})
	if audit == nil || !audit.HandoffOccurred || !audit.HandledByHuman {
		t.Fatalf("expected linked ticket human audit, got %#v", audit)
	}
	if audit.HandlerUserID != 25 || audit.HandlerName != "产品测试1值班工程师" || !audit.FirstHumanReplyAt.Equal(acceptedAt) {
		t.Fatalf("unexpected linked ticket audit handler: %#v", audit)
	}
}

func setupAIWorkflowTestDB(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "ai-workflow-test.db")
	db, err := gorm.Open(sqlite.Open("file:"+dbPath+"?_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(
		&models.Tenant{},
		&models.AIAgent{},
		&models.AIWorkflow{},
		&models.AIWorkflowVersion{},
		&models.AIWorkflowRun{},
		&models.AIWorkflowNodeRun{},
		&models.AIAgentRelease{},
		&models.ProductServiceProfile{},
		&models.Conversation{},
		&models.Message{},
		&models.User{},
		&models.AgentProfile{},
		&models.Ticket{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	if err := sqls.DB().Create(&models.Tenant{ID: 1, Name: "workflow tenant", Status: enums.StatusOk, AIEnabled: boolRef(true)}).Error; err != nil {
		t.Fatalf("create workflow tenant: %v", err)
	}
	for _, id := range []int64{12, 23, 99} {
		if err := sqls.DB().Create(&models.AIAgent{ID: id, TenantID: 1, Name: "agent", Status: enums.StatusOk}).Error; err != nil {
			t.Fatalf("create ai agent: %v", err)
		}
	}
}

func validAIWorkflowDefinition() dsl.Definition {
	return dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: "start"},
			{ID: "reply_1", Type: "send_reply", Config: json.RawMessage(`{"text":"hello"}`), Inputs: map[string]dsl.VariableSelector{
				"replyText": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "end_1", Type: "end"},
		},
		Edges: []dsl.Edge{
			{ID: "e1", Source: "start_1", Target: "reply_1"},
			{ID: "e2", Source: "reply_1", Target: "end_1"},
		},
	}
}

func aiWorkflowTestOperator() *dto.AuthPrincipal {
	return &dto.AuthPrincipal{
		UserID:     1,
		Username:   "workflow-tester",
		Nickname:   "workflow-tester",
		TenantID:   1,
		DomainType: "enterprise",
		Domain:     "enterprise",
	}
}
