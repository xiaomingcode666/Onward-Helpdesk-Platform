package services

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var ProductKnowledgeResolver = newProductKnowledgeResolver()

func newProductKnowledgeResolver() *productKnowledgeResolver {
	return &productKnowledgeResolver{}
}

type productKnowledgeResolver struct{}

// ResolveTenantScope resolves only tenant-shared knowledge. It is used by the
// tenant default agent when a customer starts a conversation without a product
// or device context.
func (s *productKnowledgeResolver) ResolveTenantScope(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error) {
	if scope.TenantID <= 0 {
		return nil, errorsx.InvalidParam("tenantId is required")
	}
	tenant, err := requireActiveTenant(scope.TenantID)
	if err != nil {
		return nil, err
	}

	scope.ProductID = 0
	scope.ProductModelID = 0
	scope.DeviceID = 0
	scope.ServiceCodeID = 0
	scope.Locale = normalizeScopeLocale(scope.Locale)
	scope.RegionCode = normalizeScopeRegion(scope.RegionCode)
	scope.Audience = normalizeKnowledgeAudience(scope.Audience)
	languages := buildKnowledgeScopeLanguageFilter(scope.Locale, tenant.DefaultLocale)

	knowledgeBases := repositories.KnowledgeBaseRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", scope.TenantID).
		Eq("access_scope", string(enums.KnowledgeBaseAccessScopeTenant)).
		Eq("status", enums.StatusOk).
		Asc("sort_no").
		Asc("id"))
	knowledgeBaseIDs := make([]int64, 0, len(knowledgeBases))
	entryKeys := make([]string, 0)
	revisionIDs := make([]int64, 0)
	for _, knowledgeBase := range knowledgeBases {
		knowledgeBaseIDs = append(knowledgeBaseIDs, knowledgeBase.ID)
		for _, document := range repositories.KnowledgeDocumentRepository.Find(sqls.DB(), sqls.NewCnd().
			Eq("tenant_id", scope.TenantID).
			Eq("knowledge_base_id", knowledgeBase.ID).
			Eq("status", enums.StatusOk).
			Eq("review_status", "published")) {
			if !matchesScopeLanguages(languages, document.Language) || document.PublishedRevisionID <= 0 {
				continue
			}
			entryKeys = append(entryKeys, buildKnowledgeDocumentEntryKey(document.ID))
			revisionIDs = append(revisionIDs, document.PublishedRevisionID)
		}
		for _, faq := range repositories.KnowledgeFAQRepository.Find(sqls.DB(), sqls.NewCnd().
			Eq("tenant_id", scope.TenantID).
			Eq("knowledge_base_id", knowledgeBase.ID).
			Eq("status", enums.StatusOk).
			Eq("review_status", "published")) {
			if !matchesScopeLanguages(languages, faq.Language) || faq.PublishedRevisionID <= 0 {
				continue
			}
			entryKeys = append(entryKeys, buildKnowledgeFAQEntryKey(faq.ID))
			revisionIDs = append(revisionIDs, faq.PublishedRevisionID)
		}
	}

	sort.Strings(entryKeys)
	sort.SliceStable(revisionIDs, func(i, j int) bool { return revisionIDs[i] < revisionIDs[j] })
	sort.SliceStable(knowledgeBaseIDs, func(i, j int) bool { return knowledgeBaseIDs[i] < knowledgeBaseIDs[j] })
	trace := []dto.KnowledgeScopeTraceItem{
		{
			Step:    "binding",
			Reason:  "resolved tenant-shared knowledge bases",
			Applied: len(knowledgeBaseIDs) > 0,
			Meta:    map[string]any{"tenantSharedKnowledgeBaseIds": knowledgeBaseIDs},
		},
		{
			Step:    "result",
			Reason:  "resolved published tenant-shared entry scope",
			Applied: len(entryKeys) > 0,
			Meta: map[string]any{
				"entryKeys":    entryKeys,
				"revisionIds":  revisionIDs,
				"knowledgeIds": knowledgeBaseIDs,
				"audience":     scope.Audience,
				"languages":    languages,
			},
		},
	}
	return &dto.ResolvedKnowledgeScope{
		Context:                      scope,
		Languages:                    languages,
		KnowledgeBaseIDs:             knowledgeBaseIDs,
		TenantSharedKnowledgeBaseIDs: append([]int64(nil), knowledgeBaseIDs...),
		EntryKeys:                    entryKeys,
		RevisionIDs:                  revisionIDs,
		ResolutionTrace:              trace,
	}, nil
}

func (s *productKnowledgeResolver) ResolveScope(scope dto.KnowledgeScopeContext) (*dto.ResolvedKnowledgeScope, error) {
	if scope.TenantID <= 0 {
		return nil, errorsx.InvalidParam("tenantId is required")
	}
	if _, err := requireActiveTenant(scope.TenantID); err != nil {
		return nil, err
	}

	scope.Locale = normalizeScopeLocale(scope.Locale)
	scope.RegionCode = normalizeScopeRegion(scope.RegionCode)
	scope.Audience = normalizeKnowledgeAudience(scope.Audience)

	resolvedProduct, resolvedModel, resolvedDevice, resolvedServiceCode, trace, err := s.resolveContext(scope)
	if err != nil {
		return nil, err
	}
	if resolvedProduct == nil {
		return nil, errorsx.InvalidParam("product context is required")
	}

	scope.ProductID = resolvedProduct.ID
	languages := buildKnowledgeScopeLanguageFilter(scope.Locale, resolvedProduct.DefaultLocale)
	if resolvedModel != nil {
		scope.ProductModelID = resolvedModel.ID
	}
	if resolvedDevice != nil {
		scope.DeviceID = resolvedDevice.ID
		if scope.RegionCode == "" {
			scope.RegionCode = normalizeScopeRegion(resolvedDevice.RegionCode)
		}
	}
	if resolvedServiceCode != nil {
		scope.ServiceCodeID = resolvedServiceCode.ID
	}

	productKnowledgeBases, kbErr := s.resolveKnowledgeBasesForLanguages(scope, languages)
	if kbErr != nil {
		return nil, kbErr
	}
	tenantSharedKnowledgeBases := repositories.KnowledgeBaseRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", scope.TenantID).
		Eq("access_scope", string(enums.KnowledgeBaseAccessScopeTenant)).
		Eq("status", enums.StatusOk).
		Asc("sort_no").
		Asc("id"))
	kbs := mergeResolvedKnowledgeBases(productKnowledgeBases, tenantSharedKnowledgeBases)
	kbMap := make(map[int64]models.KnowledgeBase, len(kbs))
	kbIDs := make([]int64, 0, len(kbs))
	productKnowledgeBaseIDs := make([]int64, 0, len(productKnowledgeBases))
	tenantSharedKnowledgeBaseIDs := make([]int64, 0, len(tenantSharedKnowledgeBases))
	for _, kb := range productKnowledgeBases {
		if kb.TenantID == scope.TenantID && kb.Status == enums.StatusOk {
			productKnowledgeBaseIDs = appendUniqueInt64(productKnowledgeBaseIDs, kb.ID)
		}
	}
	for _, kb := range tenantSharedKnowledgeBases {
		if kb.TenantID == scope.TenantID && kb.Status == enums.StatusOk {
			tenantSharedKnowledgeBaseIDs = appendUniqueInt64(tenantSharedKnowledgeBaseIDs, kb.ID)
		}
	}
	for _, kb := range kbs {
		if kb.TenantID != scope.TenantID || kb.Status != enums.StatusOk {
			continue
		}
		kbMap[kb.ID] = kb
		kbIDs = append(kbIDs, kb.ID)
	}
	if len(kbIDs) > 0 {
		trace = append(trace, dto.KnowledgeScopeTraceItem{
			Step:    "binding",
			Reason:  "resolved product and tenant-shared knowledge bases",
			Applied: true,
			Meta: map[string]any{
				"knowledgeBaseIds":             kbIDs,
				"productKnowledgeBaseIds":      productKnowledgeBaseIDs,
				"tenantSharedKnowledgeBaseIds": tenantSharedKnowledgeBaseIDs,
			},
		})
	}

	entryKeys := make([]string, 0)
	revisionIDs := make([]int64, 0)
	seenEntryKeys := make(map[string]struct{})
	seenRevisionIDs := make(map[int64]struct{})
	seenKnowledgeBaseIDs := make(map[int64]struct{}, len(kbIDs))
	for _, id := range kbIDs {
		seenKnowledgeBaseIDs[id] = struct{}{}
	}
	appendEntry := func(knowledgeBaseID int64, entryKey string, revisionID int64) {
		if strings.TrimSpace(entryKey) == "" || revisionID <= 0 {
			return
		}
		if knowledgeBaseID > 0 {
			if _, ok := seenKnowledgeBaseIDs[knowledgeBaseID]; !ok {
				seenKnowledgeBaseIDs[knowledgeBaseID] = struct{}{}
				kbIDs = append(kbIDs, knowledgeBaseID)
			}
		}
		if _, ok := seenEntryKeys[entryKey]; !ok {
			seenEntryKeys[entryKey] = struct{}{}
			entryKeys = append(entryKeys, entryKey)
		}
		if _, ok := seenRevisionIDs[revisionID]; !ok {
			seenRevisionIDs[revisionID] = struct{}{}
			revisionIDs = append(revisionIDs, revisionID)
		}
	}

	links := repositories.ProductKnowledgeLinkRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", scope.TenantID).
		Eq("product_id", scope.ProductID).
		Eq("status", enums.StatusOk).
		Eq("publish_status", "published"))
	sort.SliceStable(links, func(i, j int) bool {
		left := resolverLinkRank(links[i], scope)
		right := resolverLinkRank(links[j], scope)
		if left != right {
			return left > right
		}
		if links[i].SortNo != links[j].SortNo {
			return links[i].SortNo < links[j].SortNo
		}
		return links[i].ID < links[j].ID
	})

	for _, link := range links {
		if link.ProductModelID > 0 && link.ProductModelID != scope.ProductModelID {
			continue
		}
		if !s.linkEntryVisibleInScope(scope, link) {
			continue
		}
		if !matchesScopeLanguages(languages, link.Language) {
			continue
		}
		if !matchesAudienceVisibility(scope.Audience, link.Visibility) {
			continue
		}
		if len(kbMap) > 0 {
			if _, ok := kbMap[link.KnowledgeBaseID]; !ok {
				continue
			}
		}
		entryKey, revisionID, ok := s.resolveLinkEntry(scope.TenantID, link)
		if !ok {
			continue
		}
		appendEntry(link.KnowledgeBaseID, entryKey, revisionID)
	}

	// Unlinked entries inherit the database scope: tenant-shared bases are
	// available to every product in the tenant, while product bases enter this
	// set only through a binding to the current product. Entries linked to a
	// different product remain excluded in both cases.
	if len(kbMap) > 0 {
		linkedEntryKeys := make(map[string]struct{})
		for _, link := range repositories.ProductKnowledgeLinkRepository.Find(sqls.DB(), sqls.NewCnd().
			Eq("tenant_id", scope.TenantID).
			In("knowledge_base_id", kbIDs).
			Eq("status", enums.StatusOk)) {
			if link.KnowledgeEntryID <= 0 {
				continue
			}
			entryKey := buildKnowledgeDocumentEntryKey(link.KnowledgeEntryID)
			if s.isFAQKnowledgeLink(scope.TenantID, link) {
				entryKey = buildKnowledgeFAQEntryKey(link.KnowledgeEntryID)
			}
			linkedEntryKeys[entryKey] = struct{}{}
		}
		for _, kb := range kbs {
			for _, doc := range repositories.KnowledgeDocumentRepository.Find(sqls.DB(), sqls.NewCnd().
				Eq("tenant_id", scope.TenantID).
				Eq("knowledge_base_id", kb.ID).
				Eq("status", enums.StatusOk).
				Eq("review_status", "published")) {
				if !matchesScopeLanguages(languages, doc.Language) {
					continue
				}
				if !s.candidateDocumentVisibleInScope(scope, doc) {
					continue
				}
				entryKey := buildKnowledgeDocumentEntryKey(doc.ID)
				if _, linked := linkedEntryKeys[entryKey]; linked {
					continue
				}
				appendEntry(doc.KnowledgeBaseID, entryKey, resolvePublishedDocumentRevisionID(doc))
			}
			for _, faq := range repositories.KnowledgeFAQRepository.Find(sqls.DB(), sqls.NewCnd().
				Eq("tenant_id", scope.TenantID).
				Eq("knowledge_base_id", kb.ID).
				Eq("status", enums.StatusOk).
				Eq("review_status", "published")) {
				if !matchesScopeLanguages(languages, faq.Language) {
					continue
				}
				entryKey := buildKnowledgeFAQEntryKey(faq.ID)
				if _, linked := linkedEntryKeys[entryKey]; linked {
					continue
				}
				appendEntry(faq.KnowledgeBaseID, entryKey, resolvePublishedFAQRevisionID(faq))
			}
		}
	}

	sort.Strings(entryKeys)
	sort.SliceStable(revisionIDs, func(i, j int) bool { return revisionIDs[i] < revisionIDs[j] })
	sort.SliceStable(kbIDs, func(i, j int) bool { return kbIDs[i] < kbIDs[j] })
	sort.SliceStable(productKnowledgeBaseIDs, func(i, j int) bool { return productKnowledgeBaseIDs[i] < productKnowledgeBaseIDs[j] })
	sort.SliceStable(tenantSharedKnowledgeBaseIDs, func(i, j int) bool { return tenantSharedKnowledgeBaseIDs[i] < tenantSharedKnowledgeBaseIDs[j] })

	trace = append(trace, dto.KnowledgeScopeTraceItem{
		Step:    "result",
		Reason:  "resolved published entry scope",
		Applied: len(entryKeys) > 0,
		Meta: map[string]any{
			"entryKeys":    entryKeys,
			"revisionIds":  revisionIDs,
			"knowledgeIds": kbIDs,
			"audience":     scope.Audience,
			"languages":    languages,
		},
	})

	return &dto.ResolvedKnowledgeScope{
		Context:                      scope,
		Languages:                    languages,
		KnowledgeBaseIDs:             kbIDs,
		ProductKnowledgeBaseIDs:      productKnowledgeBaseIDs,
		TenantSharedKnowledgeBaseIDs: tenantSharedKnowledgeBaseIDs,
		EntryKeys:                    entryKeys,
		RevisionIDs:                  revisionIDs,
		ResolutionTrace:              trace,
	}, nil
}

func mergeResolvedKnowledgeBases(groups ...[]models.KnowledgeBase) []models.KnowledgeBase {
	result := make([]models.KnowledgeBase, 0)
	seen := make(map[int64]struct{})
	for _, group := range groups {
		for _, knowledgeBase := range group {
			if _, ok := seen[knowledgeBase.ID]; ok {
				continue
			}
			seen[knowledgeBase.ID] = struct{}{}
			result = append(result, knowledgeBase)
		}
	}
	return result
}

func appendUniqueInt64(values []int64, value int64) []int64 {
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}

func (s *productKnowledgeResolver) resolveKnowledgeBasesForLanguages(scope dto.KnowledgeScopeContext, languages []string) ([]models.KnowledgeBase, error) {
	result := make([]models.KnowledgeBase, 0)
	seen := make(map[int64]struct{})
	for _, language := range languages {
		resolved, err := ProductKnowledgeBindingService.Resolve(request.ResolveProductKnowledgeBindingRequest{
			TenantID:       scope.TenantID,
			ProductID:      scope.ProductID,
			ProductModelID: scope.ProductModelID,
			Locale:         language,
			RegionCode:     scope.RegionCode,
		})
		if err != nil {
			return nil, err
		}
		for _, knowledgeBase := range resolved {
			if _, ok := seen[knowledgeBase.ID]; ok {
				continue
			}
			seen[knowledgeBase.ID] = struct{}{}
			result = append(result, knowledgeBase)
		}
	}
	return result, nil
}

func (s *productKnowledgeResolver) resolveContext(scope dto.KnowledgeScopeContext) (*models.Product, *models.ProductModel, *models.Device, *models.ServiceCode, []dto.KnowledgeScopeTraceItem, error) {
	trace := make([]dto.KnowledgeScopeTraceItem, 0, 4)
	var (
		product     *models.Product
		model       *models.ProductModel
		device      *models.Device
		serviceCode *models.ServiceCode
	)

	if scope.ServiceCodeID > 0 {
		serviceCode = repositories.ServiceCodeRepository.Get(sqls.DB(), scope.ServiceCodeID)
		if serviceCode == nil || serviceCode.TenantID != scope.TenantID {
			return nil, nil, nil, nil, trace, errorsx.InvalidParam("service code not found")
		}
		if serviceCode.ProductID > 0 {
			product = repositories.ProductRepository.Get(sqls.DB(), serviceCode.ProductID)
		}
		if serviceCode.ProductModelID > 0 {
			model = repositories.ProductModelRepository.Get(sqls.DB(), serviceCode.ProductModelID)
		}
		if serviceCode.DeviceID > 0 {
			device = repositories.DeviceRepository.Get(sqls.DB(), serviceCode.DeviceID)
		}
		trace = append(trace, dto.KnowledgeScopeTraceItem{
			Step:    "service_code",
			Reason:  "resolved product context from service code",
			Applied: true,
			Meta: map[string]any{
				"serviceCodeId":  serviceCode.ID,
				"productId":      serviceCode.ProductID,
				"productModelId": serviceCode.ProductModelID,
				"deviceId":       serviceCode.DeviceID,
			},
		})
	}

	if scope.DeviceID > 0 {
		device = repositories.DeviceRepository.Get(sqls.DB(), scope.DeviceID)
		if device == nil || device.TenantID != scope.TenantID || device.Status != enums.StatusOk {
			return nil, nil, nil, nil, trace, errorsx.InvalidParam("device not found")
		}
		if device.ProductID > 0 {
			product = repositories.ProductRepository.Get(sqls.DB(), device.ProductID)
		}
		if device.ProductModelID > 0 {
			model = repositories.ProductModelRepository.Get(sqls.DB(), device.ProductModelID)
		}
		trace = append(trace, dto.KnowledgeScopeTraceItem{
			Step:    "device",
			Reason:  "resolved product context from device",
			Applied: true,
			Meta: map[string]any{
				"deviceId":       device.ID,
				"productId":      device.ProductID,
				"productModelId": device.ProductModelID,
			},
		})
	}

	if scope.ProductID > 0 {
		product = repositories.ProductRepository.Get(sqls.DB(), scope.ProductID)
		if product == nil || product.TenantID != scope.TenantID || product.Status != enums.StatusOk {
			return nil, nil, nil, nil, trace, errorsx.InvalidParam("product not found or disabled")
		}
	}
	if scope.ProductModelID > 0 {
		model = repositories.ProductModelRepository.Get(sqls.DB(), scope.ProductModelID)
		if model == nil || model.TenantID != scope.TenantID || model.Status != enums.StatusOk {
			return nil, nil, nil, nil, trace, errorsx.InvalidParam("product model not found or disabled")
		}
	}
	if product != nil && model != nil && model.ProductID != product.ID {
		return nil, nil, nil, nil, trace, errorsx.InvalidParam("product model does not belong to product")
	}
	if device != nil && product != nil && device.ProductID != product.ID {
		return nil, nil, nil, nil, trace, errorsx.InvalidParam("device does not belong to product")
	}
	if device != nil && model != nil && device.ProductModelID > 0 && device.ProductModelID != model.ID {
		return nil, nil, nil, nil, trace, errorsx.InvalidParam("device does not belong to product model")
	}
	return product, model, device, serviceCode, trace, nil
}

func (s *productKnowledgeResolver) resolveLinkEntry(tenantID int64, link models.ProductKnowledgeLink) (string, int64, bool) {
	if s.isFAQKnowledgeLink(tenantID, link) {
		faq := repositories.KnowledgeFAQRepository.Get(sqls.DB(), link.KnowledgeEntryID)
		if faq == nil || faq.TenantID != tenantID || faq.KnowledgeBaseID != link.KnowledgeBaseID || faq.Status != enums.StatusOk || faq.ReviewStatus != "published" {
			return "", 0, false
		}
		return buildKnowledgeFAQEntryKey(faq.ID), resolvePublishedFAQRevisionID(*faq), true
	}
	doc := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), link.KnowledgeEntryID)
	if doc == nil || doc.TenantID != tenantID || doc.KnowledgeBaseID != link.KnowledgeBaseID || doc.Status != enums.StatusOk || doc.ReviewStatus != "published" {
		return "", 0, false
	}
	return buildKnowledgeDocumentEntryKey(doc.ID), resolvePublishedDocumentRevisionID(*doc), true
}

func (s *productKnowledgeResolver) linkEntryVisibleInScope(scope dto.KnowledgeScopeContext, link models.ProductKnowledgeLink) bool {
	if s.isFAQKnowledgeLink(scope.TenantID, link) {
		return true
	}
	doc := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), link.KnowledgeEntryID)
	if doc == nil || doc.TenantID != scope.TenantID || doc.KnowledgeBaseID != link.KnowledgeBaseID {
		return false
	}
	return s.candidateDocumentVisibleInScope(scope, *doc)
}

func (s *productKnowledgeResolver) candidateDocumentVisibleInScope(scope dto.KnowledgeScopeContext, doc models.KnowledgeDocument) bool {
	sourceType := strings.ToLower(strings.TrimSpace(doc.SourceType))
	if sourceType != "knowledge_candidate" && sourceType != "knowledge_entry" {
		return true
	}
	if doc.SourceReferenceID <= 0 {
		return sourceType == "knowledge_entry"
	}
	candidate := repositories.KnowledgeCandidateRepository.Get(sqls.DB(), doc.SourceReferenceID)
	if candidate == nil && sourceType == "knowledge_entry" {
		return true
	}
	if candidate == nil || candidate.TenantID != scope.TenantID || candidate.Status == enums.StatusDeleted ||
		candidate.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusApproved) || candidate.RequiresReassessment {
		return false
	}
	if candidate.ProductID > 0 && candidate.ProductID != scope.ProductID {
		return false
	}
	if candidate.ProductModelID > 0 && candidate.ProductModelID != scope.ProductModelID {
		return false
	}
	return true
}

func (s *productKnowledgeResolver) isFAQKnowledgeLink(tenantID int64, link models.ProductKnowledgeLink) bool {
	knowledgeBase := repositories.KnowledgeBaseRepository.Get(sqls.DB(), link.KnowledgeBaseID)
	if knowledgeBase != nil && knowledgeBase.TenantID == tenantID {
		return strings.EqualFold(strings.TrimSpace(knowledgeBase.KnowledgeType), "faq")
	}
	return strings.EqualFold(strings.TrimSpace(link.LinkType), "faq")
}

func resolverLinkRank(link models.ProductKnowledgeLink, scope dto.KnowledgeScopeContext) int {
	score := 0
	if link.ProductModelID > 0 && link.ProductModelID == scope.ProductModelID {
		score += 4
	}
	if sameScopeLanguage(scope.Locale, link.Language) {
		score += 2
	} else if strings.TrimSpace(link.Language) == "" || strings.EqualFold(strings.TrimSpace(link.Language), "default") {
		score++
	}
	if matchesAudienceVisibility(scope.Audience, link.Visibility) {
		score++
	}
	return score
}

func normalizeScopeLocale(locale string) string {
	locale = strings.TrimSpace(strings.ReplaceAll(locale, "_", "-"))
	if locale == "" {
		return "default"
	}
	parts := strings.Split(locale, "-")
	if len(parts) == 1 {
		return strings.ToLower(parts[0])
	}
	parts[0] = strings.ToLower(parts[0])
	parts[1] = strings.ToUpper(parts[1])
	return strings.Join(parts, "-")
}

func buildKnowledgeScopeLanguageFilter(locales ...string) []string {
	result := make([]string, 0, len(locales)*2+1)
	seen := make(map[string]struct{})
	hasDefaultLocale := false
	appendLanguage := func(language string) {
		language = normalizeScopeLocale(language)
		if strings.EqualFold(language, "default") {
			hasDefaultLocale = true
		}
		key := strings.ToLower(language)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		result = append(result, language)
		if separator := strings.Index(language, "-"); separator > 0 {
			base := strings.ToLower(language[:separator])
			if _, ok := seen[base]; !ok {
				seen[base] = struct{}{}
				result = append(result, base)
			}
		}
	}
	for _, locale := range locales {
		appendLanguage(locale)
	}
	if hasDefaultLocale {
		appendLanguage("zh-CN")
		appendLanguage("zh")
		appendLanguage("en")
	}
	appendLanguage("default")
	return result
}

func normalizeScopeRegion(region string) string {
	return strings.ToUpper(strings.TrimSpace(region))
}

func normalizeKnowledgeAudience(audience string) string {
	switch strings.ToLower(strings.TrimSpace(audience)) {
	case "customer":
		return "customer"
	case "engineer", "internal":
		return "engineer"
	default:
		return "agent"
	}
}

func matchesScopeLanguage(requested, candidate string) bool {
	if sameScopeLanguage(requested, candidate) {
		return true
	}
	candidate = strings.TrimSpace(strings.ToLower(candidate))
	return candidate == "" || candidate == "default"
}

func matchesScopeLanguages(languages []string, candidate string) bool {
	candidate = normalizeScopeLocale(candidate)
	for _, language := range languages {
		if strings.EqualFold(language, candidate) {
			return true
		}
	}
	return false
}

func sameScopeLanguage(requested, candidate string) bool {
	return strings.EqualFold(strings.TrimSpace(requested), strings.TrimSpace(candidate)) && strings.TrimSpace(candidate) != ""
}

func matchesAudienceVisibility(audience, visibility string) bool {
	visibility = strings.ToLower(strings.TrimSpace(visibility))
	switch audience {
	case "customer":
		switch visibility {
		case "customer", "public", "external":
			return true
		default:
			return false
		}
	case "engineer":
		return visibility != ""
	default:
		switch visibility {
		case "", "public", "external", "customer", "team", "agent", "engineer", "internal":
			return true
		default:
			return false
		}
	}
}

func buildKnowledgeDocumentEntryKey(documentID int64) string {
	return "document:" + strconv.FormatInt(documentID, 10)
}

func buildKnowledgeFAQEntryKey(faqID int64) string {
	return "faq:" + strconv.FormatInt(faqID, 10)
}

func resolvePublishedDocumentRevisionID(document models.KnowledgeDocument) int64 {
	return document.PublishedRevisionID
}

func resolvePublishedFAQRevisionID(faq models.KnowledgeFAQ) int64 {
	return faq.PublishedRevisionID
}

func formatScopeTraceMeta(meta map[string]any) string {
	if len(meta) == 0 {
		return ""
	}
	return fmt.Sprint(meta)
}
