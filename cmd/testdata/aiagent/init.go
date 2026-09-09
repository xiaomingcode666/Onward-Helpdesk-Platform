package aiagent

import (
	"remotehelpdesk/cmd/testdata/seedlang"
	"remotehelpdesk/cmd/testdata/seeds"
	"remotehelpdesk/cmd/testdata/skill"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"
	"fmt"
	"time"

	"github.com/mlogclub/simple/sqls"
)

type InitResult struct {
	Created int
	Updated int
}

// Init 初始化 AI Agent 测试数据
// 依赖于 AI Config 和 Knowledge Base 已初始化
func Init(lang seedlang.Language) (*InitResult, error) {
	result := &InitResult{}

	aiConfigID, err := getDefaultAIConfigID()
	if err != nil {
		return result, fmt.Errorf("get default ai config id failed: %w", err)
	}
	if aiConfigID == 0 {
		return result, fmt.Errorf("no default ai config found, please init ai config first")
	}

	knowledgeIDs, err := getDefaultKnowledgeIDs()
	if err != nil {
		return result, fmt.Errorf("get default knowledge ids failed: %w", err)
	}

	defaultTeamIDs := getDefaultTeamIDs()
	defaultSkillIDs, err := getDefaultSkillIDs()
	if err != nil {
		return result, fmt.Errorf("get default skill ids failed: %w", err)
	}

	seedItems := buildModels(lang, aiConfigID, knowledgeIDs, defaultTeamIDs, defaultSkillIDs)
	for _, item := range seedItems {
		itemCopy := item
		if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			existing := repositories.AIAgentRepository.Take(ctx.Tx, "name = ?", itemCopy.Name)
			if existing != nil {
				// 更新
				if err := ctx.Tx.Model(existing).Updates(&itemCopy).Error; err != nil {
					return err
				}
				result.Updated++
			} else {
				// 创建
				if err := ctx.Tx.Create(&itemCopy).Error; err != nil {
					return err
				}
				result.Created++
			}
			return nil
		}); err != nil {
			return nil, fmt.Errorf("upsert ai agent failed: %w", err)
		}
	}

	return result, nil
}

func buildModels(lang seedlang.Language, aiConfigID int64, knowledgeIDs []int64, defaultTeamIDs string, defaultSkillIDs string) []models.AIAgent {
	now := time.Now()
	seedItems := seeds.AIAgentSeeds(lang)
	items := make([]models.AIAgent, 0, len(seedItems))
	for _, seed := range seedItems {
		items = append(items, models.AIAgent{
			Name:                seed.Name,
			Description:         seed.Description,
			Status:              enums.StatusOk,
			AIConfigID:          aiConfigID,
			ServiceMode:         seed.ServiceMode,
			SystemPrompt:        seed.SystemPrompt,
			WelcomeMessage:      seed.WelcomeMessage,
			ReplyTimeoutSeconds: seed.ReplyTimeoutSeconds,
			TeamIDs:             defaultTeamIDs,
			HandoffMode:         seed.HandoffMode,
			FallbackMode:        seed.FallbackMode,
			FallbackMessage:     seed.FallbackMessage,
			KnowledgeIDs:        utils.JoinInt64s(knowledgeIDs),
			SkillIDs:            defaultSkillIDs,
			SortNo:              seed.SortNo,
			AuditFields: models.AuditFields{
				CreatedAt:      now,
				CreateUserID:   0,
				CreateUserName: "System",
				UpdatedAt:      now,
				UpdateUserID:   0,
				UpdateUserName: "System",
			},
		})
	}
	return items
}

func getDefaultAIConfigID() (int64, error) {
	aiConfig := repositories.AIConfigRepository.Take(
		sqls.DB(),
		"model_type = ? AND status = ?",
		string(enums.AIModelTypeLLM),
		enums.StatusOk,
	)
	if aiConfig == nil {
		return 0, nil
	}
	return aiConfig.ID, nil
}

func getDefaultKnowledgeIDs() ([]int64, error) {
	knowledges := repositories.KnowledgeBaseRepository.Find(
		sqls.DB(),
		sqls.NewCnd().Where("status = ?", enums.StatusOk),
	)
	ids := make([]int64, 0, len(knowledges))
	for _, knowledge := range knowledges {
		ids = append(ids, knowledge.ID)
	}
	return ids, nil
}

func getDefaultTeamIDs() string {
	teams := repositories.AgentTeamRepository.Find(
		sqls.DB(),
		sqls.NewCnd().Where("status = ?", enums.StatusOk),
	)
	teamIDs := make([]int64, 0, len(teams))
	for _, team := range teams {
		teamIDs = append(teamIDs, team.ID)
	}
	return utils.JoinInt64s(teamIDs)
}

func getDefaultSkillIDs() (string, error) {
	skillItem := repositories.SkillDefinitionRepository.FindOne(
		sqls.DB(),
		sqls.NewCnd().Where("status = ?", enums.StatusOk).Desc("id"),
	)
	if skillItem == nil {
		return "", fmt.Errorf("default test skill not found")
	}
	return utils.JoinInt64s([]int64{skillItem.ID}), nil
}
