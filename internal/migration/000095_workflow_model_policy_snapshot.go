package migration

import (
	"encoding/json"
	"strings"

	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(95, "freeze legacy workflow model policies", func() error {
		return freezeLegacyWorkflowModelPolicies(sqls.DB())
	})
}

func freezeLegacyWorkflowModelPolicies(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.AIWorkflow{}) || !db.Migrator().HasTable(&models.AIWorkflowVersion{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&models.AIWorkflowVersion{}, "ModelPolicySnapshot") {
		if err := db.Migrator().AddColumn(&models.AIWorkflowVersion{}, "ModelPolicySnapshot"); err != nil {
			return err
		}
	}

	return db.Transaction(func(tx *gorm.DB) error {
		workflows := make([]models.AIWorkflow, 0)
		if err := tx.Find(&workflows).Error; err != nil {
			return err
		}
		workflowByID := make(map[int64]models.AIWorkflow, len(workflows))
		for i := range workflows {
			workflowByID[workflows[i].ID] = workflows[i]
		}

		versions := make([]models.AIWorkflowVersion, 0)
		if err := tx.Order("id ASC").Find(&versions).Error; err != nil {
			return err
		}
		for i := range versions {
			if workflowModelPolicyFromJSON(versions[i].ModelPolicySnapshot) != nil {
				continue
			}
			policy := workflowDefinitionModelPolicy(versions[i].Definition)
			if policy == nil {
				workflow := workflowByID[versions[i].WorkflowID]
				policy = workflowDefinitionModelPolicy(workflow.DraftDefinition)
			}
			if policy == nil {
				policy = &dsl.ModelPolicy{CredentialChain: []string{
					dsl.ModelCredentialScopeProduct,
					dsl.ModelCredentialScopeTenantDefault,
				}}
			}
			raw, err := json.Marshal(policy)
			if err != nil {
				return err
			}
			if err := tx.Model(&models.AIWorkflowVersion{}).Where("id = ?", versions[i].ID).
				Update("model_policy_snapshot", string(raw)).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func workflowDefinitionModelPolicy(raw string) *dsl.ModelPolicy {
	definition := dsl.Definition{}
	if json.Unmarshal([]byte(strings.TrimSpace(raw)), &definition) != nil {
		return nil
	}
	if definition.ModelPolicy == nil || len(definition.ModelPolicy.CredentialChain) == 0 {
		return nil
	}
	return &dsl.ModelPolicy{CredentialChain: append([]string(nil), definition.ModelPolicy.CredentialChain...)}
}

func workflowModelPolicyFromJSON(raw string) *dsl.ModelPolicy {
	policy := dsl.ModelPolicy{}
	if json.Unmarshal([]byte(strings.TrimSpace(raw)), &policy) != nil || len(policy.CredentialChain) == 0 {
		return nil
	}
	return &policy
}
