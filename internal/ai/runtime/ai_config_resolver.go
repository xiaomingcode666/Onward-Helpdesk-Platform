package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"
	svc "remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func resolveAgentAIConfig(ctx context.Context, agent models.AIAgent, tenantID, productID int64, requestedCredentialChain []string) (context.Context, *models.AIConfig, error) {
	if tenantID <= 0 {
		tenantID = agent.TenantID
	}
	if tenantID <= 0 {
		tenantID = inferAgentTenantID(agent)
	}
	if productID <= 0 {
		productID = agent.ProductID
	}
	credentialChain := append([]string(nil), requestedCredentialChain...)
	if len(credentialChain) == 0 {
		credentialChain = resolveAgentWorkflowCredentialChain(agent)
	}
	ctx = ai.WithCapabilityScope(ctx, ai.CapabilityScope{
		TenantID:        tenantID,
		ProductID:       productID,
		CredentialChain: credentialChain,
	})
	// A published workflow credential policy is authoritative. AIConfigID is a
	// legacy provider override and is only used when the workflow has no policy.
	if len(credentialChain) == 0 && agent.AIConfigID > 0 {
		item := svc.AIConfigService.Get(agent.AIConfigID)
		if item == nil {
			return ctx, nil, fmt.Errorf("ai config is nil")
		}
		return ctx, item, nil
	}
	item, err := ai.GetEnabledAIConfigWithContext(ctx, enums.AIModelTypeLLM)
	if err != nil {
		return ctx, nil, fmt.Errorf("resolve platform LLM capability: %w", err)
	}
	if item == nil {
		return ctx, nil, fmt.Errorf("platform LLM capability is unavailable")
	}
	return ctx, applyAgentLLMModel(item, agent.LLMModelName), nil
}

func resolveAgentWorkflowCredentialChain(agent models.AIAgent) []string {
	if agent.WorkflowVersionID <= 0 {
		return nil
	}
	version := repositories.AIWorkflowVersionRepository.Get(sqls.DB(), agent.WorkflowVersionID)
	if version == nil {
		return nil
	}
	if chain := definitionCredentialChain(version.Definition); len(chain) > 0 {
		return chain
	}
	return modelPolicySnapshotCredentialChain(version.ModelPolicySnapshot)
}

func definitionCredentialChain(raw string) []string {
	var definition dsl.Definition
	if json.Unmarshal([]byte(raw), &definition) != nil || definition.ModelPolicy == nil {
		return nil
	}
	return append([]string(nil), definition.ModelPolicy.CredentialChain...)
}

func modelPolicySnapshotCredentialChain(raw string) []string {
	var policy dsl.ModelPolicy
	if json.Unmarshal([]byte(raw), &policy) != nil {
		return nil
	}
	return append([]string(nil), policy.CredentialChain...)
}

func applyAgentLLMModel(item *models.AIConfig, modelName string) *models.AIConfig {
	if item == nil || strings.TrimSpace(modelName) == "" {
		return item
	}
	ret := *item
	ret.ModelName = strings.TrimSpace(modelName)
	return &ret
}

func inferAgentTenantID(agent models.AIAgent) int64 {
	var tenantID int64
	for _, knowledgeBaseID := range utils.SplitInt64s(agent.KnowledgeIDs) {
		knowledgeBase := svc.KnowledgeBaseService.Get(knowledgeBaseID)
		if knowledgeBase == nil || knowledgeBase.TenantID <= 0 {
			continue
		}
		if tenantID == 0 {
			tenantID = knowledgeBase.TenantID
			continue
		}
		if tenantID != knowledgeBase.TenantID {
			return 0
		}
	}
	return tenantID
}
