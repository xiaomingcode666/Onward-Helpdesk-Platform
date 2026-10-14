package services

import (
	"fmt"
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

// createProductDefaultKnowledgeBase 创建产品默认知识库并写入按语言展示的名称和描述。
func createProductDefaultKnowledgeBase(product *models.Product, operator *dto.AuthPrincipal) error {
	locales := []string{strings.TrimSpace(product.DefaultLocale)}
	if locales[0] == "" {
		locales[0] = "en"
	}
	nameText := productKnowledgeBaseNameText(product.Name)
	descriptionText := productKnowledgeBaseDescriptionText(product.Name)
	if _, err := ProductCenterService.CreateProductKnowledgeBase(product.TenantID, product.ID, dto.EnterpriseProductKnowledgeBaseCreateRequest{
		Name:           nameText.Render(i18nx.LocaleZhCN),
		Description:    descriptionText.Render(i18nx.LocaleZhCN),
		SupportLocales: locales,
	}, operator); err != nil {
		return err
	}
	profile := repositories.ProductServiceProfileRepository.GetByProductID(sqls.DB(), product.ID)
	if profile == nil || profile.TenantID != product.TenantID || profile.DefaultKnowledgeBaseID <= 0 {
		return fmt.Errorf("product knowledge base was not created")
	}
	return repositories.KnowledgeBaseRepository.Updates(sqls.DB(), profile.DefaultKnowledgeBaseID, map[string]any{
		"name_text":        nameText,
		"description_text": descriptionText,
	})
}

// syncProductDerivedDisplayTextDB 在产品改名后刷新派生知识库和机器人展示文本中的产品名。
// 用户改过名称（展示文本为空或已是其他资源键）的数据保持不变。
func syncProductDerivedDisplayTextDB(db *gorm.DB, product *models.Product) error {
	if db == nil || product == nil || product.ID <= 0 {
		return nil
	}
	var agents []models.AIAgent
	if err := db.Where("tenant_id = ? AND product_id = ? AND status <> ?", product.TenantID, product.ID, enums.StatusDeleted).
		Find(&agents).Error; err != nil {
		return err
	}
	for _, agent := range agents {
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
// 供按名称筛选和查找使用；未匹配到系统生成名称时原样返回。
func resolveKnowledgeBaseStoredName(db *gorm.DB, tenantID int64, name string) string {
	name = strings.TrimSpace(name)
	if db == nil || tenantID <= 0 || name == "" {
		return name
	}
	var bases []models.KnowledgeBase
	if err := db.Where("tenant_id = ? AND status <> ? AND name_text <> ''", tenantID, enums.StatusDeleted).
		Find(&bases).Error; err != nil {
		return name
	}
	for _, base := range bases {
		if base.Name == name {
			return name
		}
		for _, locale := range i18nx.SupportedLocales {
			if base.DisplayName(locale) == name {
				return base.Name
			}
		}
	}
	return name
}

// knowledgeBaseDisplayName 当 storedName 仍是知识库存储的名称时返回其当前语言展示名，否则原样返回。
func knowledgeBaseDisplayName(knowledgeBaseID int64, storedName, locale string) string {
	if knowledgeBaseID <= 0 || storedName == "" {
		return storedName
	}
	kb := repositories.KnowledgeBaseRepository.Get(sqls.DB(), knowledgeBaseID)
	if kb == nil || strings.TrimSpace(kb.Name) != storedName {
		return storedName
	}
	return kb.DisplayName(locale)
}

func knowledgeBaseDisplayDescription(knowledgeBaseID int64, storedDescription, locale string) string {
	if knowledgeBaseID <= 0 || storedDescription == "" {
		return storedDescription
	}
	kb := repositories.KnowledgeBaseRepository.Get(sqls.DB(), knowledgeBaseID)
	if kb == nil || strings.TrimSpace(kb.Description) != storedDescription {
		return storedDescription
	}
	return kb.DisplayDescription(locale)
}

// LocalizeProductProfile 把产品档案中的系统生成知识库名称和描述换成当前语言。
func LocalizeProductProfile(profile *dto.EnterpriseProductProfileDTO, locale string) {
	if profile == nil || profile.ServiceProfile.KnowledgeBaseID == nil {
		return
	}
	id := *profile.ServiceProfile.KnowledgeBaseID
	profile.ServiceProfile.KnowledgeBaseName = knowledgeBaseDisplayName(id, profile.ServiceProfile.KnowledgeBaseName, locale)
	profile.ServiceProfile.KnowledgeBaseDescription = knowledgeBaseDisplayDescription(id, profile.ServiceProfile.KnowledgeBaseDescription, locale)
}

// LocalizeProductManual 把产品手册挂载中的系统生成知识库名称换成当前语言。
func LocalizeProductManual(manual *dto.EnterpriseProductManualDTO, locale string) {
	if manual == nil {
		return
	}
	manual.KnowledgeBaseName = knowledgeBaseDisplayName(manual.KnowledgeBaseID, manual.KnowledgeBaseName, locale)
}

func LocalizeProductManuals(manuals []dto.EnterpriseProductManualDTO, locale string) {
	for i := range manuals {
		LocalizeProductManual(&manuals[i], locale)
	}
}

// LocalizeProductResources 把产品资源中的系统生成知识库名称换成当前语言。
func LocalizeProductResources(resources *dto.EnterpriseProductResourcesDTO, locale string) {
	if resources == nil {
		return
	}
	resources.Knowledge.KnowledgeBaseName = knowledgeBaseDisplayName(resources.Knowledge.KnowledgeBaseID, resources.Knowledge.KnowledgeBaseName, locale)
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
	if err := db.AutoMigrate(&models.KnowledgeBase{}, &models.AIAgent{}); err != nil {
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
