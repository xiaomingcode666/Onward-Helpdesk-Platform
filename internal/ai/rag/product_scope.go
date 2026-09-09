package rag

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const (
	knowledgeTenantScopePrefix  = "tenant:"
	knowledgeProductScopePrefix = "product:"
)

type KnowledgeEntryProductScope struct {
	ProductIDs      []int64
	ProductModelIDs []int64
	ScopeKeys       []string
	Fingerprint     string
}

func BuildKnowledgeRuntimeScopeKeys(tenantID, productID int64) []string {
	if tenantID <= 0 || productID <= 0 {
		return nil
	}
	return []string{
		buildKnowledgeTenantScopeKey(tenantID),
		buildKnowledgeProductScopeKey(productID),
	}
}

func ResolveKnowledgeEntryProductScope(db *gorm.DB, tenantID, knowledgeBaseID, entryID int64, entryType string) KnowledgeEntryProductScope {
	if db == nil {
		db = sqls.DB()
	}
	knowledgeBaseType := ""
	if knowledgeBase := repositories.KnowledgeBaseRepository.Get(db, knowledgeBaseID); knowledgeBase != nil && knowledgeBase.TenantID == tenantID {
		knowledgeBaseType = knowledgeBase.KnowledgeType
	}
	links := repositories.ProductKnowledgeLinkRepository.Find(db, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("knowledge_base_id", knowledgeBaseID).
		Eq("knowledge_entry_id", entryID).
		Eq("status", enums.StatusOk))

	productIDs := make([]int64, 0, len(links))
	productModelIDs := make([]int64, 0, len(links))
	seenProducts := make(map[int64]struct{}, len(links))
	seenModels := make(map[int64]struct{}, len(links))
	hasProductLinks := false
	for _, link := range links {
		if !knowledgeScopeLinkMatchesEntryType(link.LinkType, entryType, knowledgeBaseType) || link.ProductID <= 0 {
			continue
		}
		// An active link keeps the entry product-scoped even when its referenced
		// product is no longer active. Broken links must never widen access.
		hasProductLinks = true
		product := repositories.ProductRepository.GetByTenant(db, link.ProductID, tenantID)
		if product == nil || product.Status != enums.StatusOk {
			continue
		}
		if strings.TrimSpace(link.PublishStatus) != "published" {
			continue
		}
		if _, ok := seenProducts[product.ID]; !ok {
			seenProducts[product.ID] = struct{}{}
			productIDs = append(productIDs, product.ID)
		}
		if link.ProductModelID <= 0 {
			continue
		}
		model := repositories.ProductModelRepository.Get(db, link.ProductModelID)
		if model == nil || model.TenantID != tenantID || model.ProductID != product.ID || model.Status != enums.StatusOk {
			continue
		}
		if _, ok := seenModels[model.ID]; !ok {
			seenModels[model.ID] = struct{}{}
			productModelIDs = append(productModelIDs, model.ID)
		}
	}

	sort.Slice(productIDs, func(i, j int) bool { return productIDs[i] < productIDs[j] })
	sort.Slice(productModelIDs, func(i, j int) bool { return productModelIDs[i] < productModelIDs[j] })
	scopeKeys := make([]string, 0, max(1, len(productIDs)))
	if !hasProductLinks {
		scopeKeys = append(scopeKeys, buildKnowledgeTenantScopeKey(tenantID))
	} else {
		for _, productID := range productIDs {
			scopeKeys = append(scopeKeys, buildKnowledgeProductScopeKey(productID))
		}
	}

	return KnowledgeEntryProductScope{
		ProductIDs:      productIDs,
		ProductModelIDs: productModelIDs,
		ScopeKeys:       scopeKeys,
		Fingerprint:     knowledgeScopeFingerprint(scopeKeys, productModelIDs),
	}
}

func knowledgeScopeLinkMatchesEntryType(linkType, entryType, knowledgeBaseType string) bool {
	linkType = strings.ToLower(strings.TrimSpace(linkType))
	entryType = strings.ToLower(strings.TrimSpace(entryType))
	knowledgeBaseType = strings.ToLower(strings.TrimSpace(knowledgeBaseType))
	isFAQEntry := entryType == "faq" || entryType == "knowledge_faq"
	if knowledgeBaseType != "" {
		return (knowledgeBaseType == "faq") == isFAQEntry
	}
	return (linkType == "faq") == isFAQEntry
}

func buildKnowledgeTenantScopeKey(tenantID int64) string {
	return fmt.Sprintf("%s%d", knowledgeTenantScopePrefix, tenantID)
}

func buildKnowledgeProductScopeKey(productID int64) string {
	return fmt.Sprintf("%s%d", knowledgeProductScopePrefix, productID)
}

func knowledgeScopeFingerprint(scopeKeys []string, productModelIDs []int64) string {
	parts := append([]string(nil), scopeKeys...)
	for _, modelID := range productModelIDs {
		parts = append(parts, fmt.Sprintf("model:%d", modelID))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, ",")))
	return hex.EncodeToString(sum[:6])
}
