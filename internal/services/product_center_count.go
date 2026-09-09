package services

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

type enterpriseProductCountCacheEntry struct {
	at    time.Time
	total int64
}

var enterpriseProductCountCache sync.Map // key: "tenantID:scope"

const enterpriseProductCountCacheTTL = 30 * time.Second

func (s *productCenterService) CountEnterpriseProductsForOperator(tenantID int64, operator *dto.AuthPrincipal) (dto.EnterpriseProductCountDTO, error) {
	return s.countEnterpriseProducts(tenantID, resolveEnterpriseProductAccessScope(tenantID, operator))
}

func (s *productCenterService) countEnterpriseProducts(tenantID int64, scope enterpriseProductAccessScope) (dto.EnterpriseProductCountDTO, error) {
	if _, err := requireActiveTenant(tenantID); err != nil {
		return dto.EnterpriseProductCountDTO{}, err
	}

	cacheKey := enterpriseProductCountCacheKey(tenantID, scope)
	if entry, ok := enterpriseProductCountCache.Load(cacheKey); ok {
		if cached := entry.(*enterpriseProductCountCacheEntry); time.Since(cached.at) < enterpriseProductCountCacheTTL {
			return dto.EnterpriseProductCountDTO{Total: cached.total}, nil
		}
	}

	cnd := sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Where("status <> ?", enums.StatusDeleted)
	if scope.Restricted {
		if len(scope.ProductIDs) == 0 {
			enterpriseProductCountCache.Store(cacheKey, &enterpriseProductCountCacheEntry{at: time.Now(), total: 0})
			return dto.EnterpriseProductCountDTO{Total: 0}, nil
		}
		cnd.In("id", scope.ProductIDs)
	}
	total := repositories.ProductRepository.Count(sqls.DB(), cnd)
	enterpriseProductCountCache.Store(cacheKey, &enterpriseProductCountCacheEntry{at: time.Now(), total: total})
	return dto.EnterpriseProductCountDTO{Total: total}, nil
}

func (s *productCenterService) InvalidateProductCountCache(tenantID int64) {
	prefix := strconv.FormatInt(tenantID, 10) + ":"
	enterpriseProductCountCache.Range(func(key, _ any) bool {
		if keyText, ok := key.(string); ok && strings.HasPrefix(keyText, prefix) {
			enterpriseProductCountCache.Delete(keyText)
		}
		return true
	})
}

func enterpriseProductCountCacheKey(tenantID int64, scope enterpriseProductAccessScope) string {
	if !scope.Restricted {
		return strconv.FormatInt(tenantID, 10) + ":all"
	}
	productIDs := append([]int64(nil), scope.ProductIDs...)
	sort.Slice(productIDs, func(i, j int) bool { return productIDs[i] < productIDs[j] })
	parts := make([]string, 0, len(productIDs))
	for _, productID := range productIDs {
		parts = append(parts, strconv.FormatInt(productID, 10))
	}
	return strconv.FormatInt(tenantID, 10) + ":restricted:" + strings.Join(parts, ",")
}
