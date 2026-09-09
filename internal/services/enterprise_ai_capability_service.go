package services

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var EnterpriseAICapabilityService = &enterpriseAICapabilityService{}

type EnterpriseAICapabilityState struct {
	Type      enums.AIModelType
	Available bool
	ModelName string
	Models    []string
	Dimension int
}

type EnterpriseAICapabilityAggregate struct {
	GeneratedAt       time.Time
	Capabilities      []EnterpriseAICapabilityState
	DefaultCredential *EnterpriseAIDefaultCredentialState
}

type EnterpriseAIDefaultCredentialState struct {
	KeyName         string
	KeyStatus       string
	ProvisionStatus string
	Ready           bool
}

type enterpriseAICapabilityService struct{}

func (s *enterpriseAICapabilityService) GetCapabilities(ctx context.Context, tenantID int64) *EnterpriseAICapabilityAggregate {
	ret := &EnterpriseAICapabilityAggregate{
		GeneratedAt:  time.Now(),
		Capabilities: make([]EnterpriseAICapabilityState, 0, 2),
	}
	if tenantID > 0 && sqls.DB().Migrator().HasTable("sub2api_tenant_accounts") {
		if account := repositories.PlatformIAMRepository.FindSub2APIAccountByTenantID(sqls.DB(), tenantID); account != nil {
			ret.DefaultCredential = &EnterpriseAIDefaultCredentialState{
				KeyName:         strings.TrimSpace(account.DefaultKeyName),
				KeyStatus:       strings.TrimSpace(account.DefaultKeyStatus),
				ProvisionStatus: strings.TrimSpace(account.ProvisionStatus),
				Ready: strings.EqualFold(strings.TrimSpace(account.ProvisionStatus), "active") &&
					strings.EqualFold(strings.TrimSpace(account.DefaultKeyStatus), "active") &&
					strings.TrimSpace(account.DefaultKeyCiphertext) != "",
			}
		}
	}
	for _, modelType := range []enums.AIModelType{enums.AIModelTypeLLM, enums.AIModelTypeEmbedding} {
		state := EnterpriseAICapabilityState{Type: modelType, Models: []string{}}
		route, err := ai.ResolveCapabilityRouteForScope(ctx, modelType, ai.CapabilityScope{TenantID: tenantID})
		if err != nil {
			slog.Warn("enterprise AI capability is unavailable", "tenant_id", tenantID, "model_type", modelType, "error", err)
		} else if route != nil {
			state.Available = true
			state.ModelName = route.Config.ModelName
			state.Dimension = route.Config.Dimension
			if modelType == enums.AIModelTypeLLM {
				models, listErr := ai.ListCapabilityModels(ctx, route)
				state.Models = models
				if listErr != nil {
					slog.Warn("enterprise AI model catalog is unavailable", "tenant_id", tenantID, "error", listErr)
				}
			}
		}
		ret.Capabilities = append(ret.Capabilities, state)
	}
	return ret
}

func (s *enterpriseAICapabilityService) UpdateDefaultLLMModel(ctx context.Context, tenantID int64, modelName string, operator *dto.AuthPrincipal) (*EnterpriseAICapabilityAggregate, error) {
	modelName = strings.TrimSpace(modelName)
	if tenantID <= 0 {
		return nil, errors.New("tenant is required")
	}
	if modelName == "" {
		return nil, errors.New("modelName is required")
	}
	account := repositories.PlatformIAMRepository.FindSub2APIAccountByTenantID(sqls.DB(), tenantID)
	if account == nil || strings.TrimSpace(account.DefaultKeyCiphertext) == "" {
		return nil, errors.New("tenant sub2api account is not configured")
	}
	if !strings.EqualFold(strings.TrimSpace(account.ProvisionStatus), "active") || !strings.EqualFold(strings.TrimSpace(account.DefaultKeyStatus), "active") {
		return nil, errors.New("tenant sub2api default key is not active")
	}
	route, err := ai.ResolveCapabilityRouteForScope(ctx, enums.AIModelTypeLLM, ai.CapabilityScope{TenantID: tenantID})
	if err != nil {
		return nil, err
	}
	if route == nil {
		return nil, errors.New("llm capability is not available")
	}
	models, err := ai.ListCapabilityModels(ctx, route)
	if err != nil {
		return nil, err
	}
	if len(models) > 0 && !containsStringFold(models, modelName) {
		return nil, errors.New("modelName is not in the available model list")
	}
	audit := utils.BuildAuditFields(operator)
	now := time.Now()
	if err := repositories.PlatformIAMRepository.UpdateSub2APIAccount(sqls.DB(), account.ID, map[string]any{
		"default_llm_model": modelName,
		"updated_at":        now,
		"update_user_id":    audit.UpdateUserID,
		"update_user_name":  audit.UpdateUserName,
	}); err != nil {
		return nil, err
	}
	return s.GetCapabilities(ctx, tenantID), nil
}

func containsStringFold(items []string, value string) bool {
	value = strings.TrimSpace(value)
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), value) {
			return true
		}
	}
	return false
}
