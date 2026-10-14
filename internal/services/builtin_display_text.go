package services

import (
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

// 产品派生的知识库和机器人以产品名为参数生成展示文本；存储列保留中文规范值，
// 兼容现有按名称去重和运行时引用。
const (
	productKnowledgeBaseNameKey        = "builtin.knowledgeBase.product.name"
	productKnowledgeBaseDescriptionKey = "builtin.knowledgeBase.product.description"
	productAgentNameKey                = "builtin.aiAgent.product.name"
	productAgentNameWithCodeKey        = "builtin.aiAgent.product.nameWithCode"
	productAgentDescriptionKey         = "builtin.aiAgent.product.description"
)

func productKnowledgeBaseNameText(productName string) i18nx.Text {
	return i18nx.NewText(productKnowledgeBaseNameKey, i18nx.Params{"product": strings.TrimSpace(productName)})
}

func productKnowledgeBaseDescriptionText(productName string) i18nx.Text {
	return i18nx.NewText(productKnowledgeBaseDescriptionKey, i18nx.Params{"product": strings.TrimSpace(productName)})
}

func productAgentNameText(product *models.Product, withCode bool) i18nx.Text {
	if withCode {
		return i18nx.NewText(productAgentNameWithCodeKey, i18nx.Params{
			"product": strings.TrimSpace(product.Name),
			"code":    strings.TrimSpace(product.Code),
		})
	}
	return i18nx.NewText(productAgentNameKey, i18nx.Params{"product": strings.TrimSpace(product.Name)})
}

func productAgentDescriptionText(productName string) i18nx.Text {
	return i18nx.NewText(productAgentDescriptionKey, i18nx.Params{"product": strings.TrimSpace(productName)})
}

// createProductDefaultKnowledgeBase 创建产品默认知识库，并补齐按语言展示的名称和描述。
// 补齐步骤可重复执行：创建报错（例如已存在）或上次补齐失败时，下一次调用会继续补齐。
func createProductDefaultKnowledgeBase(product *models.Product, operator *dto.AuthPrincipal) error {
	locales := []string{strings.TrimSpace(product.DefaultLocale)}
	if locales[0] == "" {
		locales[0] = "en"
	}
	_, createErr := ProductCenterService.CreateProductKnowledgeBase(product.TenantID, product.ID, dto.EnterpriseProductKnowledgeBaseCreateRequest{
		Name:           productKnowledgeBaseNameText(product.Name).Render(i18nx.LocaleZhCN),
		Description:    productKnowledgeBaseDescriptionText(product.Name).Render(i18nx.LocaleZhCN),
		SupportLocales: locales,
	}, operator)
	syncErr := syncProductDerivedDisplayTextDB(sqls.DB(), product)
	if createErr != nil {
		return createErr
	}
	return syncErr
}

// syncProductDerivedDisplayTextDB 补齐并刷新产品派生知识库和机器人的展示文本：
// 存储值仍是系统生成值但展示文本为空时补上；展示文本中的产品名与当前产品不一致时更新。
// 用户改过名称（展示文本为空且存储值不是系统生成值，或已是其他资源键）的数据保持不变。可重复执行。
func syncProductDerivedDisplayTextDB(db *gorm.DB, product *models.Product) error {
	if db == nil || product == nil || product.ID <= 0 {
		return nil
	}
	var agents []models.AIAgent
	if err := db.Where("tenant_id = ? AND product_id = ? AND status <> ?", product.TenantID, product.ID, enums.StatusDeleted).
		Find(&agents).Error; err != nil {
		return err
	}
	agentNameTexts := []i18nx.Text{productAgentNameText(product, false), productAgentNameText(product, true)}
	for i := range agents {
		agent := &agents[i]
		if err := backfillAIAgentDisplayText(db, agent, agentNameTexts, productAgentDescriptionText(product.Name)); err != nil {
			return err
		}
		updates := map[string]any{}
		switch agent.NameText.Key {
		case productAgentNameKey:
			if next := productAgentNameText(product, false); !agent.NameText.Equal(next) {
				updates["name_text"] = next
			}
		case productAgentNameWithCodeKey:
			if next := productAgentNameText(product, true); !agent.NameText.Equal(next) {
				updates["name_text"] = next
			}
		}
		if agent.DescriptionText.Key == productAgentDescriptionKey {
			if next := productAgentDescriptionText(product.Name); !agent.DescriptionText.Equal(next) {
				updates["description_text"] = next
			}
		}
		if len(updates) > 0 {
			if err := repositories.AIAgentRepository.Updates(db, agent.ID, updates); err != nil {
				return err
			}
		}
	}

	profile := repositories.ProductServiceProfileRepository.GetByProductID(db, product.ID)
	if profile == nil || profile.TenantID != product.TenantID || profile.DefaultKnowledgeBaseID <= 0 {
		return nil
	}
	knowledgeBase := repositories.KnowledgeBaseRepository.Get(db, profile.DefaultKnowledgeBaseID)
	if knowledgeBase == nil || knowledgeBase.TenantID != product.TenantID {
		return nil
	}
	if err := backfillKnowledgeBaseDisplayText(db, knowledgeBase, productKnowledgeBaseNameText(product.Name), productKnowledgeBaseDescriptionText(product.Name)); err != nil {
		return err
	}
	updates := map[string]any{}
	if knowledgeBase.NameText.Key == productKnowledgeBaseNameKey {
		if next := productKnowledgeBaseNameText(product.Name); !knowledgeBase.NameText.Equal(next) {
			updates["name_text"] = next
		}
	}
	if knowledgeBase.DescriptionText.Key == productKnowledgeBaseDescriptionKey {
		if next := productKnowledgeBaseDescriptionText(product.Name); !knowledgeBase.DescriptionText.Equal(next) {
			updates["description_text"] = next
		}
	}
	if len(updates) == 0 {
		return nil
	}
	return repositories.KnowledgeBaseRepository.Updates(db, knowledgeBase.ID, updates)
}

// resolveKnowledgeBaseStoredName 把界面提交的知识库名称（可能是任一语言的展示名）解析为数据库中存储的名称，
// 供按名称筛选和查找使用。租户内已有同名知识库时优先按原名匹配，避免把用户自建知识库误解析为内置知识库。
func resolveKnowledgeBaseStoredName(db *gorm.DB, tenantID int64, name string) string {
	name = strings.TrimSpace(name)
	if db == nil || tenantID <= 0 || name == "" {
		return name
	}
	var exact int64
	if err := db.Model(&models.KnowledgeBase{}).
		Where("tenant_id = ? AND status <> ? AND name = ?", tenantID, enums.StatusDeleted, name).
		Count(&exact).Error; err != nil || exact > 0 {
		return name
	}
	var bases []models.KnowledgeBase
	if err := db.Where("tenant_id = ? AND status <> ? AND name_text <> ''", tenantID, enums.StatusDeleted).
		Find(&bases).Error; err != nil {
		return name
	}
	for _, base := range bases {
		for _, locale := range i18nx.SupportedLocales {
			if base.DisplayName(locale) == name {
				return base.Name
			}
		}
	}
	return name
}

// knowledgeBaseDisplay 按当前语言解析知识库展示名和描述，单次请求内缓存查询结果。
// 仅当传入值仍是知识库存储的名称或描述时替换，其他值（如快照、默认分类）原样返回。
type knowledgeBaseDisplay struct {
	locale string
	cache  map[int64]*models.KnowledgeBase
}

func newKnowledgeBaseDisplay(locale string) *knowledgeBaseDisplay {
	return &knowledgeBaseDisplay{locale: locale, cache: map[int64]*models.KnowledgeBase{}}
}

func (d *knowledgeBaseDisplay) base(id int64) *models.KnowledgeBase {
	if id <= 0 {
		return nil
	}
	base, ok := d.cache[id]
	if !ok {
		base = repositories.KnowledgeBaseRepository.Get(sqls.DB(), id)
		d.cache[id] = base
	}
	return base
}

func (d *knowledgeBaseDisplay) name(id int64, stored string) string {
	base := d.base(id)
	if base == nil || stored == "" || strings.TrimSpace(base.Name) != strings.TrimSpace(stored) {
		return stored
	}
	return base.DisplayName(d.locale)
}

func (d *knowledgeBaseDisplay) description(id int64, stored string) string {
	base := d.base(id)
	if base == nil || stored == "" || strings.TrimSpace(base.Description) != strings.TrimSpace(stored) {
		return stored
	}
	return base.DisplayDescription(d.locale)
}

// LocalizeProductProfile 把产品档案中的系统生成知识库名称和描述换成当前语言。
func LocalizeProductProfile(profile *dto.EnterpriseProductProfileDTO, locale string) {
	if profile == nil || profile.ServiceProfile.KnowledgeBaseID == nil {
		return
	}
	display := newKnowledgeBaseDisplay(locale)
	id := *profile.ServiceProfile.KnowledgeBaseID
	profile.ServiceProfile.KnowledgeBaseName = display.name(id, profile.ServiceProfile.KnowledgeBaseName)
	profile.ServiceProfile.KnowledgeBaseDescription = display.description(id, profile.ServiceProfile.KnowledgeBaseDescription)
}

// LocalizeProductManual 把产品手册挂载中的系统生成知识库名称换成当前语言。
func LocalizeProductManual(manual *dto.EnterpriseProductManualDTO, locale string) {
	if manual == nil {
		return
	}
	manual.KnowledgeBaseName = newKnowledgeBaseDisplay(locale).name(manual.KnowledgeBaseID, manual.KnowledgeBaseName)
}

func LocalizeProductManuals(manuals []dto.EnterpriseProductManualDTO, locale string) {
	display := newKnowledgeBaseDisplay(locale)
	for i := range manuals {
		manuals[i].KnowledgeBaseName = display.name(manuals[i].KnowledgeBaseID, manuals[i].KnowledgeBaseName)
	}
}

// LocalizeProductResources 把产品资源中的系统生成知识库名称换成当前语言。
func LocalizeProductResources(resources *dto.EnterpriseProductResourcesDTO, locale string) {
	if resources == nil {
		return
	}
	resources.Knowledge.KnowledgeBaseName = newKnowledgeBaseDisplay(locale).name(resources.Knowledge.KnowledgeBaseID, resources.Knowledge.KnowledgeBaseName)
}

// keepOrReplaceDisplayText 处理编辑表单提交的名称或描述：仍是系统生成内容（任一语言的展示值）时保留存储值和展示文本，
// 否则视为用户自定义，写入提交值并清空展示文本。
func keepOrReplaceDisplayText(text i18nx.Text, stored, submitted string) (string, i18nx.Text) {
	if text.MatchesAnyLocale(submitted) {
		return stored, text
	}
	return submitted, i18nx.Text{}
}

// BackfillBuiltinDisplayTextDB 为存量的系统生成知识库和机器人补充展示文本。只处理展示文本为空、
// 且存储值仍等于系统生成值（中文规范值）的数据；用户改过名称的数据保持不变。
func BackfillBuiltinDisplayTextDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if err := db.AutoMigrate(&models.KnowledgeBase{}, &models.AIAgent{}, &models.AgentTeam{}, &models.Department{}); err != nil {
		return err
	}
	if err := backfillSupportOrganizationDisplayTextDB(db); err != nil {
		return err
	}
	var bases []models.KnowledgeBase
	if err := db.Where("remark = ? AND status <> ?", tenantDefaultKnowledgeBaseRemark, enums.StatusDeleted).Find(&bases).Error; err != nil {
		return err
	}
	for _, base := range bases {
		if err := backfillKnowledgeBaseDisplayText(db, &base, tenantDefaultKnowledgeBaseNameText, tenantDefaultKnowledgeBaseDescriptionText); err != nil {
			return err
		}
	}

	var agents []models.AIAgent
	if err := db.Where("product_id = 0 AND source = ? AND status <> ?", TenantDefaultAIAgentSource, enums.StatusDeleted).Find(&agents).Error; err != nil {
		return err
	}
	for _, agent := range agents {
		tenant := repositories.PlatformIAMRepository.GetTenant(db, agent.TenantID)
		if tenant == nil {
			continue
		}
		if err := backfillAIAgentDisplayText(db, &agent, []i18nx.Text{tenantDefaultAgentNameText}, tenantDefaultAgentDescriptionText(tenant)); err != nil {
			return err
		}
	}

	var products []models.Product
	if err := db.Where("status <> ?", enums.StatusDeleted).Find(&products).Error; err != nil {
		return err
	}
	for i := range products {
		product := &products[i]
		var productAgents []models.AIAgent
		if err := db.Where("tenant_id = ? AND product_id = ? AND status <> ?", product.TenantID, product.ID, enums.StatusDeleted).Find(&productAgents).Error; err != nil {
			return err
		}
		for _, agent := range productAgents {
			names := []i18nx.Text{productAgentNameText(product, false), productAgentNameText(product, true)}
			if err := backfillAIAgentDisplayText(db, &agent, names, productAgentDescriptionText(product.Name)); err != nil {
				return err
			}
		}
		profile := repositories.ProductServiceProfileRepository.GetByProductID(db, product.ID)
		if profile == nil || profile.TenantID != product.TenantID || profile.DefaultKnowledgeBaseID <= 0 {
			continue
		}
		base := repositories.KnowledgeBaseRepository.Get(db, profile.DefaultKnowledgeBaseID)
		if base == nil || base.TenantID != product.TenantID {
			continue
		}
		if err := backfillKnowledgeBaseDisplayText(db, base, productKnowledgeBaseNameText(product.Name), productKnowledgeBaseDescriptionText(product.Name)); err != nil {
			return err
		}
	}
	return nil
}

func backfillKnowledgeBaseDisplayText(db *gorm.DB, base *models.KnowledgeBase, nameText, descriptionText i18nx.Text) error {
	updates := map[string]any{}
	if base.NameText.IsZero() && strings.TrimSpace(base.Name) == nameText.Render(i18nx.LocaleZhCN) {
		updates["name_text"] = nameText
	}
	if base.DescriptionText.IsZero() && strings.TrimSpace(base.Description) == descriptionText.Render(i18nx.LocaleZhCN) {
		updates["description_text"] = descriptionText
	}
	if len(updates) == 0 {
		return nil
	}
	return db.Model(&models.KnowledgeBase{}).Where("id = ?", base.ID).Updates(updates).Error
}

func backfillAIAgentDisplayText(db *gorm.DB, agent *models.AIAgent, nameTexts []i18nx.Text, descriptionText i18nx.Text) error {
	updates := map[string]any{}
	if agent.NameText.IsZero() {
		for _, nameText := range nameTexts {
			if strings.TrimSpace(agent.Name) == nameText.Render(i18nx.LocaleZhCN) {
				updates["name_text"] = nameText
				break
			}
		}
	}
	if agent.DescriptionText.IsZero() && strings.TrimSpace(agent.Description) == descriptionText.Render(i18nx.LocaleZhCN) {
		updates["description_text"] = descriptionText
	}
	if len(updates) == 0 {
		return nil
	}
	return db.Model(&models.AIAgent{}).Where("id = ?", agent.ID).Updates(updates).Error
}

// agentTeamDisplayNames 按当前语言解析团队展示名；仅当 storedName 仍是团队存储的名称时替换。
type agentTeamDisplayNames struct {
	tenantID int64
	locale   string
	cache    map[int64]*models.AgentTeam
}

func newAgentTeamDisplayNames(tenantID int64, operator *dto.AuthPrincipal) *agentTeamDisplayNames {
	locale := ""
	if operator != nil {
		locale = operator.Locale
	}
	return &agentTeamDisplayNames{tenantID: tenantID, locale: locale, cache: map[int64]*models.AgentTeam{}}
}

func (n *agentTeamDisplayNames) name(teamID int64, storedName string) string {
	if n == nil || n.locale == "" || teamID <= 0 || storedName == "" {
		return storedName
	}
	team, ok := n.cache[teamID]
	if !ok {
		team = AgentTeamService.GetForTenant(teamID, n.tenantID)
		n.cache[teamID] = team
	}
	if team == nil || team.Name != storedName {
		return storedName
	}
	return team.DisplayName(n.locale)
}

// operatorLocale 返回本次请求的界面语言（中间件已用 X-Locale 覆盖用户偏好），缺省为服务器默认语言。
func operatorLocale(operator *dto.AuthPrincipal) string {
	if operator == nil || strings.TrimSpace(operator.Locale) == "" {
		return i18nx.DefaultLocale
	}
	return i18nx.NormalizeLocale(operator.Locale)
}

// builtinRoleName 返回内置角色在指定语言下的名称。
func builtinRoleName(code, locale string) string {
	return i18nx.TLocale(locale, builtinRoleKey(code, "name"))
}

func builtinRoleKey(code, field string) string {
	return "builtin.role." + code + "." + field
}

// BuiltinRoleText 返回内置角色名称或描述在指定语言下的文本；仅当存储值仍是系统默认值且存在多语言资源时替换，
// 用户改过的值和没有资源的角色（如平台角色）原样返回。field 为 name 或 description。
func BuiltinRoleText(code string, isBuiltin bool, field, stored, locale string) string {
	key := builtinRoleKey(code, field)
	if !isBuiltin || code == "" || !i18nx.HasKey(key) {
		return stored
	}
	text := i18nx.NewText(key, nil)
	if !text.MatchesAnyLocale(stored) {
		return stored
	}
	return text.Render(locale)
}

// backfillSupportOrganizationDisplayTextDB 为技术团队、产品维修组和系统部门补充展示文本。
func backfillSupportOrganizationDisplayTextDB(db *gorm.DB) error {
	technicalTexts := map[string][2]i18nx.Text{
		TechnicalAfterSalesTeamName:  {technicalAfterSalesTeamNameText, technicalAfterSalesTeamDescriptionText},
		TechnicalMaintenanceTeamName: {technicalMaintenanceTeamNameText, technicalMaintenanceTeamDescriptionText},
	}
	var teams []models.AgentTeam
	if err := db.Where("system_managed = ? AND status <> ?", true, enums.StatusDeleted).Find(&teams).Error; err != nil {
		return err
	}
	for _, team := range teams {
		updates := map[string]any{}
		switch team.TeamType {
		case AgentTeamTypeTechnicalRepair:
			texts, ok := technicalTexts[strings.TrimSpace(team.Name)]
			if !ok {
				continue
			}
			if team.NameText.IsZero() {
				updates["name_text"] = texts[0]
			}
			if team.DescriptionText.IsZero() && strings.TrimSpace(team.Description) == texts[1].Render(i18nx.LocaleZhCN) {
				updates["description_text"] = texts[1]
			}
		case AgentTeamTypeProductRepair:
			product := repositories.ProductRepository.GetByTenant(db, team.ProductID, team.TenantID)
			if product == nil || !team.DescriptionText.IsZero() {
				continue
			}
			if text := productRepairTeamDescriptionText(product.Name); strings.TrimSpace(team.Description) == text.Render(i18nx.LocaleZhCN) {
				updates["description_text"] = text
			}
		}
		if len(updates) > 0 {
			if err := db.Model(&models.AgentTeam{}).Where("id = ?", team.ID).Updates(updates).Error; err != nil {
				return err
			}
		}
	}

	var departments []models.Department
	if err := db.Where("department_code IN ? AND name_text = ''", []string{"root", technicalRepairDepartmentCode}).Find(&departments).Error; err != nil {
		return err
	}
	for _, department := range departments {
		var text i18nx.Text
		switch {
		case department.DepartmentCode == "root" && strings.TrimSpace(department.Name) == rootDepartmentNameText.Render(i18nx.LocaleZhCN):
			text = rootDepartmentNameText
		case department.DepartmentCode == technicalRepairDepartmentCode:
			if texts, ok := technicalTexts[strings.TrimSpace(department.Name)]; ok {
				text = texts[0]
			}
		}
		if text.IsZero() {
			continue
		}
		if err := db.Model(&models.Department{}).Where("id = ?", department.ID).Update("name_text", text).Error; err != nil {
			return err
		}
	}
	return nil
}

// builtinRoleDescription 返回内置角色在指定语言下的描述。
func builtinRoleDescription(code, locale string) string {
	return i18nx.TLocale(locale, builtinRoleKey(code, "description"))
}
