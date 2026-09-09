package repositories

import (
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var AIWorkflowRepository = newAIWorkflowRepository()

func newAIWorkflowRepository() *aiWorkflowRepository {
	return &aiWorkflowRepository{}
}

type aiWorkflowRepository struct{}

type AIWorkflowRetirementReferences struct {
	Agents   []models.AIAgent
	Releases []models.AIAgentRelease
	Profiles []models.ProductServiceProfile
	Forks    []models.AIWorkflow
}

func (r *aiWorkflowRepository) Get(db *gorm.DB, id int64) *models.AIWorkflow {
	ret := &models.AIWorkflow{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *aiWorkflowRepository) GetForUpdate(db *gorm.DB, id int64) *models.AIWorkflow {
	ret := &models.AIWorkflow{}
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *aiWorkflowRepository) Take(db *gorm.DB, where ...interface{}) *models.AIWorkflow {
	ret := &models.AIWorkflow{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *aiWorkflowRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.AIWorkflow) {
	cnd.Find(db, &list)
	return
}

func (r *aiWorkflowRepository) FindPageByParams(db *gorm.DB, params *params.QueryParams) (list []models.AIWorkflow, paging *sqls.Paging) {
	return r.FindPageByCnd(db, &params.Cnd)
}

func (r *aiWorkflowRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.AIWorkflow, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.AIWorkflow{})
	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *aiWorkflowRepository) Create(db *gorm.DB, t *models.AIWorkflow) error {
	return db.Create(t).Error
}

func (r *aiWorkflowRepository) Updates(db *gorm.DB, id int64, columns map[string]interface{}) error {
	return db.Model(&models.AIWorkflow{}).Where("id = ?", id).Updates(columns).Error
}

func (r *aiWorkflowRepository) GetByScopeCode(db *gorm.DB, tenantID int64, scope, code string) *models.AIWorkflow {
	return r.Take(db, "tenant_id = ? AND scope = ? AND code = ? AND status <> ?", tenantID, scope, code, enums.StatusDeleted)
}

func (r *aiWorkflowRepository) GetAnyByScopeCode(db *gorm.DB, tenantID int64, scope, code string) *models.AIWorkflow {
	return r.Take(db, "tenant_id = ? AND scope = ? AND code = ?", tenantID, scope, code)
}

func (r *aiWorkflowRepository) FindRetirementReferences(db *gorm.DB, workflowID int64) (*AIWorkflowRetirementReferences, error) {
	ret := &AIWorkflowRetirementReferences{}
	if db.Migrator().HasTable(&models.AIAgent{}) {
		if err := db.Select("id", "tenant_id", "product_id").
			Where("workflow_id = ? AND status <> ?", workflowID, enums.StatusDeleted).
			Order("id ASC").Find(&ret.Agents).Error; err != nil {
			return nil, err
		}
	}
	if db.Migrator().HasTable(&models.AIAgentRelease{}) {
		if err := db.Select("id", "tenant_id", "product_id", "agent_id").
			Where("workflow_id = ? AND deployment_status = ? AND status <> ?", workflowID, models.AIAgentReleaseDeploymentActive, enums.StatusDeleted).
			Order("id ASC").Find(&ret.Releases).Error; err != nil {
			return nil, err
		}
	}
	if db.Migrator().HasTable(&models.ProductServiceProfile{}) {
		if err := db.Select("id", "tenant_id", "product_id").
			Where("default_flow_template_id = ? AND status <> ?", workflowID, enums.StatusDeleted).
			Order("id ASC").Find(&ret.Profiles).Error; err != nil {
			return nil, err
		}
	}
	if err := db.Select("id", "tenant_id").
		Where("scope = ? AND source_workflow_id = ? AND status <> ?", models.AIWorkflowScopeTenant, workflowID, enums.StatusDeleted).
		Order("id ASC").Find(&ret.Forks).Error; err != nil {
		return nil, err
	}
	return ret, nil
}

// RebindBuiltInReferences moves live configuration away from a retired
// platform template without manufacturing a reviewed deployment. Historical
// releases keep their original workflow IDs for auditability.
func (r *aiWorkflowRepository) RebindBuiltInReferences(db *gorm.DB, workflowID, replacementWorkflowID, replacementVersionID int64, now time.Time) error {
	return db.Transaction(func(tx *gorm.DB) error {
		agentIDs := make([]int64, 0)
		if tx.Migrator().HasTable(&models.AIAgent{}) {
			var agents []models.AIAgent
			if err := tx.Select("id").
				Where("workflow_id = ? AND status <> ?", workflowID, enums.StatusDeleted).
				Find(&agents).Error; err != nil {
				return err
			}
			for i := range agents {
				agentIDs = append(agentIDs, agents[i].ID)
			}
		}

		if tx.Migrator().HasTable(&models.AIAgentRelease{}) {
			var releases []models.AIAgentRelease
			query := tx.Select("id", "agent_id").Where(
				"workflow_id = ? AND deployment_status = ? AND status <> ?",
				workflowID,
				models.AIAgentReleaseDeploymentActive,
				enums.StatusDeleted,
			)
			if err := query.Find(&releases).Error; err != nil {
				return err
			}
			for i := range releases {
				agentIDs = appendUniqueInt64(agentIDs, releases[i].AgentID)
			}
			if len(agentIDs) > 0 {
				if err := tx.Model(&models.AIAgentRelease{}).
					Where("agent_id IN ? AND deployment_status = ? AND status <> ?", agentIDs, models.AIAgentReleaseDeploymentActive, enums.StatusDeleted).
					Updates(map[string]any{
						"deployment_status": models.AIAgentReleaseDeploymentRetired,
						"update_user_name":  "system",
						"updated_at":        now,
					}).Error; err != nil {
					return err
				}
			}
		}

		if tx.Migrator().HasTable(&models.AIAgent{}) {
			if err := tx.Model(&models.AIAgent{}).
				Where("workflow_id = ? AND status <> ?", workflowID, enums.StatusDeleted).
				Updates(map[string]any{
					"workflow_id":         replacementWorkflowID,
					"workflow_version_id": replacementVersionID,
					"active_release_id":   0,
					"draft_revision":      gorm.Expr("draft_revision + 1"),
					"service_mode":        enums.IMConversationServiceModeAIOnly,
					"status":              enums.StatusDisabled,
					"review_status":       enums.AIAgentReviewStatusUnreviewed,
					"review_comment":      "旧平台模板已合并到 AI 智能问诊，请基于最新稳定版重新审核部署",
					"reviewed_at":         nil,
					"reviewed_by_id":      0,
					"reviewed_by_name":    "",
					"update_user_name":    "system",
					"updated_at":          now,
				}).Error; err != nil {
				return err
			}
			if len(agentIDs) > 0 {
				if err := tx.Model(&models.AIAgent{}).
					Where("id IN ? AND status <> ?", agentIDs, enums.StatusDeleted).
					Updates(map[string]any{
						"active_release_id": 0,
						"status":            enums.StatusDisabled,
						"review_status":     enums.AIAgentReviewStatusUnreviewed,
						"review_comment":    "旧平台模板已退役，请基于最新稳定版重新审核部署",
						"reviewed_at":       nil,
						"reviewed_by_id":    0,
						"reviewed_by_name":  "",
						"update_user_name":  "system",
						"updated_at":        now,
					}).Error; err != nil {
					return err
				}
			}
		}

		if tx.Migrator().HasTable(&models.ProductServiceProfile{}) {
			if err := tx.Model(&models.ProductServiceProfile{}).
				Where("default_flow_template_id = ? AND status <> ?", workflowID, enums.StatusDeleted).
				Updates(map[string]any{
					"default_flow_template_id": replacementWorkflowID,
					"update_user_name":         "system",
					"updated_at":               now,
				}).Error; err != nil {
				return err
			}
		}

		return tx.Model(&models.AIWorkflow{}).
			Where("scope = ? AND source_workflow_id = ? AND status <> ?", models.AIWorkflowScopeTenant, workflowID, enums.StatusDeleted).
			Updates(map[string]any{
				"source_workflow_id": replacementWorkflowID,
				"source_version_id":  replacementVersionID,
				"update_user_name":   "system",
				"updated_at":         now,
			}).Error
	})
}

func appendUniqueInt64(values []int64, value int64) []int64 {
	if value <= 0 {
		return values
	}
	for _, item := range values {
		if item == value {
			return values
		}
	}
	return append(values, value)
}

func (r *aiWorkflowRepository) RetireBuiltIn(db *gorm.DB, workflowID int64, now time.Time) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.AIWorkflowVersion{}).
			Where("workflow_id = ? AND release_channel <> ?", workflowID, models.AIWorkflowReleaseChannelRevoked).
			Updates(map[string]any{
				"release_channel":  models.AIWorkflowReleaseChannelDeprecated,
				"update_user_name": "system",
				"updated_at":       now,
			}).Error; err != nil {
			return err
		}
		return tx.Model(&models.AIWorkflow{}).Where("id = ?", workflowID).Updates(map[string]any{
			"status":           enums.StatusDeleted,
			"update_user_name": "system",
			"updated_at":       now,
		}).Error
	})
}
