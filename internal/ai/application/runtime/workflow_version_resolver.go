package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	workflowexecutor "remotehelpdesk/internal/ai/runtime/workflow"
	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

type workflowVersionResolver struct{}

func (workflowVersionResolver) ResolveWorkflowVersion(_ context.Context, ref workflowexecutor.WorkflowVersionReference) (workflowexecutor.ResolvedWorkflowVersion, error) {
	version := repositories.AIWorkflowVersionRepository.Get(sqls.DB(), ref.WorkflowVersionID)
	if version == nil || version.Status != enums.StatusOk {
		return workflowexecutor.ResolvedWorkflowVersion{}, fmt.Errorf("published workflow version does not exist: %d", ref.WorkflowVersionID)
	}
	if version.ReleaseChannel == models.AIWorkflowReleaseChannelRevoked {
		return workflowexecutor.ResolvedWorkflowVersion{}, fmt.Errorf("subflow version %d has been revoked", ref.WorkflowVersionID)
	}
	workflow := repositories.AIWorkflowRepository.Get(sqls.DB(), version.WorkflowID)
	if workflow == nil || workflow.Status == enums.StatusDeleted {
		return workflowexecutor.ResolvedWorkflowVersion{}, fmt.Errorf("workflow does not exist for version %d", ref.WorkflowVersionID)
	}
	if workflow.Scope == models.AIWorkflowScopeTenant && workflow.TenantID != ref.TenantID {
		return workflowexecutor.ResolvedWorkflowVersion{}, fmt.Errorf("subflow version %d is outside the tenant scope", ref.WorkflowVersionID)
	}
	if workflow.AgentID > 0 {
		owner := repositories.AIAgentRepository.Get(sqls.DB(), workflow.AgentID)
		if owner == nil || owner.Status == enums.StatusDeleted {
			return workflowexecutor.ResolvedWorkflowVersion{}, fmt.Errorf("workflow owner does not exist for version %d", ref.WorkflowVersionID)
		}
		if owner.TenantID > 0 && owner.TenantID != ref.TenantID {
			return workflowexecutor.ResolvedWorkflowVersion{}, fmt.Errorf("subflow version %d is outside the tenant scope", ref.WorkflowVersionID)
		}
		if owner.ProductID > 0 && owner.ProductID != ref.ProductID {
			return workflowexecutor.ResolvedWorkflowVersion{}, fmt.Errorf("subflow version %d is outside the product scope", ref.WorkflowVersionID)
		}
	}
	definition := dsl.Definition{}
	if err := json.Unmarshal([]byte(version.Definition), &definition); err != nil {
		return workflowexecutor.ResolvedWorkflowVersion{}, fmt.Errorf("invalid subflow definition: %w", err)
	}
	return workflowexecutor.ResolvedWorkflowVersion{
		WorkflowID:     version.WorkflowID,
		VersionID:      version.ID,
		DefinitionHash: strings.TrimSpace(version.DefinitionHash),
		Definition:     definition,
	}, nil
}
