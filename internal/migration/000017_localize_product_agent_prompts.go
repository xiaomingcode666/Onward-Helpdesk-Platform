package migration

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(17, "localize untouched default product ai agent prompts", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			return localizeDefaultProductAgentPrompts(ctx.Tx)
		})
	})
}

func localizeDefaultProductAgentPrompts(db *gorm.DB) error {
	var agents []models.AIAgent
	if err := db.Where("product_id > 0 AND status <> ?", enums.StatusDeleted).Find(&agents).Error; err != nil {
		return err
	}

	for i := range agents {
		agent := &agents[i]
		var product models.Product
		if err := db.First(&product, "id = ? AND tenant_id = ?", agent.ProductID, agent.TenantID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}
		if strings.TrimSpace(agent.SystemPrompt) != legacyDefaultProductAgentSystemPrompt(&product) {
			continue
		}
		if err := db.Model(&models.AIAgent{}).
			Where("id = ?", agent.ID).
			Updates(map[string]any{
				"system_prompt": localizedDefaultProductAgentSystemPrompt(&product),
				"updated_at":    time.Now(),
			}).Error; err != nil {
			return err
		}
	}
	return nil
}

func legacyDefaultProductAgentSystemPrompt(product *models.Product) string {
	return fmt.Sprintf(`You are the after-sales support assistant for %s (%s).
Use the bound product knowledge base as the primary source of truth.
Ask for the device model, serial number, symptoms, fault codes, region, and troubleshooting already attempted when context is incomplete.
Never invent specifications, safety instructions, warranty terms, or repair procedures.
For electrical, hydraulic, pressure, motion, or other safety-critical operations, state the required isolation and escalation steps before troubleshooting.
When evidence is insufficient or the issue requires authorization, hand off to a human service engineer with a concise summary.`, product.Name, product.Code)
}

func localizedDefaultProductAgentSystemPrompt(product *models.Product) string {
	return fmt.Sprintf(`你是 %s（%s）的产品售后服务助手。
以绑定的产品知识库作为主要事实来源。
当上下文不完整时，先追问设备型号、序列号、故障现象、故障码、所在地区和已尝试的排障步骤。
不要编造规格参数、安全说明、保修条款或维修流程。
涉及电气、液压、压力、运动部件等安全关键操作时，先说明隔离防护和升级处理要求，再继续排障。
证据不足或问题需要授权处理时，转交人工工程师，并附上简洁的问题摘要。`, product.Name, product.Code)
}
