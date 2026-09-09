package factory

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	runtimeinstruction "remotehelpdesk/internal/ai/runtime/instruction"
	runtimetooling "remotehelpdesk/internal/ai/runtime/tooling"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/services"

	einoskill "github.com/cloudwego/eino/adk/middlewares/skill"
)

type runtimeSkillMetadata struct {
	ID               int64
	Name             string
	Description      string
	AllowedToolCodes []string
}

type databaseSkillBackend struct {
	toolDefinitions []runtimetooling.MCPToolDefinition
	skillsByID      map[string]models.SkillDefinition
	order           []string
}

func newDatabaseSkillBackend(aiAgent models.AIAgent, toolDefinitions []runtimetooling.MCPToolDefinition) (*databaseSkillBackend, error) {
	visibleSkills := loadVisibleSkills(aiAgent)
	if len(visibleSkills) == 0 {
		return nil, fmt.Errorf("no visible skills available")
	}
	ret := &databaseSkillBackend{
		toolDefinitions: append([]runtimetooling.MCPToolDefinition(nil), toolDefinitions...),
		skillsByID:      make(map[string]models.SkillDefinition, len(visibleSkills)),
		order:           make([]string, 0, len(visibleSkills)),
	}
	for _, item := range visibleSkills {
		id := strconv.FormatInt(item.ID, 10)
		if id == "" {
			continue
		}
		ret.skillsByID[id] = item
		ret.order = append(ret.order, id)
	}
	if len(ret.skillsByID) == 0 {
		return nil, fmt.Errorf("no visible skills available")
	}
	return ret, nil
}

func (b *databaseSkillBackend) List(_ context.Context) ([]einoskill.FrontMatter, error) {
	if b == nil || len(b.order) == 0 {
		return nil, nil
	}
	ret := make([]einoskill.FrontMatter, 0, len(b.order))
	for _, id := range b.order {
		item, ok := b.skillsByID[id]
		if !ok {
			continue
		}
		ret = append(ret, einoskill.FrontMatter{
			Name:        strconv.FormatInt(item.ID, 10),
			Description: skillListDescription(item),
		})
	}
	return ret, nil
}

func (b *databaseSkillBackend) Get(_ context.Context, name string) (einoskill.Skill, error) {
	if b == nil {
		return einoskill.Skill{}, fmt.Errorf("database skill backend is nil")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return einoskill.Skill{}, fmt.Errorf("skill name is empty")
	}
	item, ok := b.skillsByID[name]
	if !ok {
		return einoskill.Skill{}, fmt.Errorf("skill %q not found", name)
	}
	return einoskill.Skill{
		FrontMatter: einoskill.FrontMatter{
			Name:        strconv.FormatInt(item.ID, 10),
			Description: skillListDescription(item),
		},
		Content:       runtimeinstruction.BuildSkillDocument(&item, filterSkillToolDefinitions(b.toolDefinitions, &item)),
		BaseDirectory: "",
	}, nil
}

func loadVisibleSkills(aiAgent models.AIAgent) []models.SkillDefinition {
	ids := utils.SplitInt64s(strings.TrimSpace(aiAgent.SkillIDs))
	if len(ids) == 0 {
		return nil
	}
	byID := services.SkillDefinitionService.GetByIDs(ids)
	if len(byID) == 0 {
		return nil
	}
	ret := make([]models.SkillDefinition, 0, len(ids))
	for _, id := range ids {
		item, ok := byID[id]
		if !ok || item.Status != enums.StatusOk || item.ID <= 0 {
			continue
		}
		if item.TenantID != 0 && item.TenantID != aiAgent.TenantID {
			continue
		}
		ret = append(ret, item)
	}
	return ret
}

func buildRuntimeSkillMetadataMap(aiAgent models.AIAgent) map[string]runtimeSkillMetadata {
	visibleSkills := loadVisibleSkills(aiAgent)
	if len(visibleSkills) == 0 {
		return nil
	}
	ret := make(map[string]runtimeSkillMetadata, len(visibleSkills))
	for _, item := range visibleSkills {
		if item.ID <= 0 {
			continue
		}
		id := strconv.FormatInt(item.ID, 10)
		ret[id] = runtimeSkillMetadata{
			ID:               item.ID,
			Name:             strings.TrimSpace(item.Name),
			Description:      skillListDescription(item),
			AllowedToolCodes: parseSkillToolWhitelist(item.ToolWhitelist),
		}
	}
	return ret
}

func HasVisibleSkills(aiAgent models.AIAgent) bool {
	return len(buildRuntimeSkillMetadataMap(aiAgent)) > 0
}

func skillListDescription(item models.SkillDefinition) string {
	if desc := strings.TrimSpace(item.Description); desc != "" {
		return desc
	}
	if name := strings.TrimSpace(item.Name); name != "" {
		return name
	}
	return fmt.Sprintf("Skill %d", item.ID)
}

func parseSkillToolWhitelist(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	ret := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		ret = append(ret, item)
	}
	return ret
}

func filterSkillToolDefinitions(defs []runtimetooling.MCPToolDefinition, skill *models.SkillDefinition) []runtimetooling.MCPToolDefinition {
	allowed := parseSkillToolWhitelist(skill.ToolWhitelist)
	if len(allowed) == 0 {
		return nil
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, item := range allowed {
		allowedSet[item] = struct{}{}
	}
	ret := make([]runtimetooling.MCPToolDefinition, 0, len(defs))
	for _, item := range defs {
		if _, ok := allowedSet[strings.TrimSpace(item.ToolCode)]; ok {
			ret = append(ret, item)
		}
	}
	return ret
}
