package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/go-redis/redis/v8"
	"github.com/mlogclub/simple/sqls"
)

const (
	customerPortalCachePrefix = "remotehelpdesk:customer-portal:v1:"

	customerPortalCacheDialTimeout = 500 * time.Millisecond
	customerPortalCacheIOTimeout   = 500 * time.Millisecond

	customerPortalProductCacheTTL      = 30 * time.Minute
	customerPortalProductModelCacheTTL = 30 * time.Minute
	customerPortalUserCacheTTL         = 5 * time.Minute
	customerPortalManualCountCacheTTL  = 10 * time.Minute
	customerPortalWarrantyCacheTTL     = 10 * time.Minute
)

const (
	customerPortalCacheKindProduct      = "product"
	customerPortalCacheKindProductModel = "product-model"
	customerPortalCacheKindUser         = "user"
	customerPortalCacheKindManualCount  = "manual-count"
	customerPortalCacheKindWarranty     = "warranty"
)

var (
	customerPortalRedisOnce   sync.Once
	customerPortalRedisClient *redis.Client
)

type customerPortalCachedProduct struct {
	ID       int64  `json:"id"`
	TenantID int64  `json:"tenant_id"`
	Name     string `json:"name"`
	Code     string `json:"code"`
}

type customerPortalCachedProductModel struct {
	ID        int64  `json:"id"`
	TenantID  int64  `json:"tenant_id"`
	ProductID int64  `json:"product_id"`
	Name      string `json:"name"`
}

type customerPortalCachedUser struct {
	ID       int64  `json:"id"`
	Nickname string `json:"nickname"`
	Username string `json:"username"`
}

type customerPortalCachedWarranty struct {
	DeviceID  int64     `json:"device_id"`
	HasRecord bool      `json:"has_record"`
	EndAt     time.Time `json:"end_at"`
}

func customerPortalRedisCacheClient() *redis.Client {
	customerPortalRedisOnce.Do(func() {
		cfg := config.CurrentOrDefault().Redis
		addr := strings.TrimSpace(cfg.Addr)
		if addr == "" {
			addr = ":6379"
		}
		client := redis.NewClient(&redis.Options{
			Addr:         addr,
			Password:     cfg.Password,
			DB:           cfg.DB,
			MaxRetries:   1,
			DialTimeout:  customerPortalCacheDialTimeout,
			ReadTimeout:  customerPortalCacheIOTimeout,
			WriteTimeout: customerPortalCacheIOTimeout,
		})
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := client.Ping(ctx).Err(); err != nil {
			slog.Warn("customer portal redis cache disabled", "error", err)
			return
		}
		customerPortalRedisClient = client
	})
	return customerPortalRedisClient
}

func customerPortalCacheKey(kind string, id int64) string {
	return customerPortalCachePrefix + kind + ":" + strconv.FormatInt(id, 10)
}

func customerPortalRedisValueBytes(value any) []byte {
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		return v
	case string:
		return []byte(v)
	default:
		return []byte(fmt.Sprint(v))
	}
}

func customerPortalRedisDecodeJSON(value any, target any) bool {
	raw := customerPortalRedisValueBytes(value)
	if len(raw) == 0 {
		return false
	}
	return json.Unmarshal(raw, target) == nil
}

func loadCustomerPortalProductsByIDCached(productIDs []int64) map[int64]*models.Product {
	productIDs = uniqueCustomerPortalInt64s(productIDs)
	result := make(map[int64]*models.Product, len(productIDs))
	if len(productIDs) == 0 {
		return result
	}
	cached, misses := customerPortalLoadProductsByIDFromCache(productIDs)
	for id, item := range cached {
		result[id] = item
	}
	if len(misses) == 0 {
		return result
	}
	items := repositories.ProductRepository.FindByIDs(sqls.DB(), misses)
	if len(items) == 0 {
		return result
	}
	customerPortalStoreProductsInCache(items)
	for i := range items {
		result[items[i].ID] = &items[i]
	}
	return result
}

func customerPortalLoadProductsByIDFromCache(productIDs []int64) (map[int64]*models.Product, []int64) {
	result := make(map[int64]*models.Product, len(productIDs))
	misses := make([]int64, 0, len(productIDs))
	client := customerPortalRedisCacheClient()
	if client == nil {
		return result, append(misses, productIDs...)
	}
	keys := make([]string, len(productIDs))
	for i, id := range productIDs {
		keys[i] = customerPortalCacheKey(customerPortalCacheKindProduct, id)
	}
	ctx, cancel := context.WithTimeout(context.Background(), customerPortalCacheIOTimeout)
	defer cancel()
	rawValues, err := client.MGet(ctx, keys...).Result()
	if err != nil {
		return result, append(misses, productIDs...)
	}
	for i, raw := range rawValues {
		id := productIDs[i]
		var cached customerPortalCachedProduct
		if raw == nil || !customerPortalRedisDecodeJSON(raw, &cached) || cached.ID <= 0 {
			misses = append(misses, id)
			continue
		}
		result[id] = &models.Product{
			ID:       cached.ID,
			TenantID: cached.TenantID,
			Name:     cached.Name,
			Code:     cached.Code,
		}
	}
	return result, misses
}

func customerPortalStoreProductsInCache(items []models.Product) {
	client := customerPortalRedisCacheClient()
	if client == nil || len(items) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), customerPortalCacheIOTimeout)
	defer cancel()
	pipe := client.Pipeline()
	for i := range items {
		cached := customerPortalCachedProduct{
			ID:       items[i].ID,
			TenantID: items[i].TenantID,
			Name:     items[i].Name,
			Code:     items[i].Code,
		}
		payload, err := json.Marshal(cached)
		if err != nil {
			continue
		}
		pipe.Set(ctx, customerPortalCacheKey(customerPortalCacheKindProduct, items[i].ID), payload, customerPortalProductCacheTTL)
	}
	_, _ = pipe.Exec(ctx)
}

func loadCustomerPortalProductModelsByIDCached(productModelIDs []int64) map[int64]*models.ProductModel {
	productModelIDs = uniqueCustomerPortalInt64s(productModelIDs)
	result := make(map[int64]*models.ProductModel, len(productModelIDs))
	if len(productModelIDs) == 0 {
		return result
	}
	cached, misses := customerPortalLoadProductModelsByIDFromCache(productModelIDs)
	for id, item := range cached {
		result[id] = item
	}
	if len(misses) == 0 {
		return result
	}
	items := repositories.ProductModelRepository.FindByIDs(sqls.DB(), misses)
	if len(items) == 0 {
		return result
	}
	customerPortalStoreProductModelsInCache(items)
	for i := range items {
		result[items[i].ID] = &items[i]
	}
	return result
}

func customerPortalLoadProductModelsByIDFromCache(productModelIDs []int64) (map[int64]*models.ProductModel, []int64) {
	result := make(map[int64]*models.ProductModel, len(productModelIDs))
	misses := make([]int64, 0, len(productModelIDs))
	client := customerPortalRedisCacheClient()
	if client == nil {
		return result, append(misses, productModelIDs...)
	}
	keys := make([]string, len(productModelIDs))
	for i, id := range productModelIDs {
		keys[i] = customerPortalCacheKey(customerPortalCacheKindProductModel, id)
	}
	ctx, cancel := context.WithTimeout(context.Background(), customerPortalCacheIOTimeout)
	defer cancel()
	rawValues, err := client.MGet(ctx, keys...).Result()
	if err != nil {
		return result, append(misses, productModelIDs...)
	}
	for i, raw := range rawValues {
		id := productModelIDs[i]
		var cached customerPortalCachedProductModel
		if raw == nil || !customerPortalRedisDecodeJSON(raw, &cached) || cached.ID <= 0 {
			misses = append(misses, id)
			continue
		}
		result[id] = &models.ProductModel{
			ID:        cached.ID,
			TenantID:  cached.TenantID,
			ProductID: cached.ProductID,
			Name:      cached.Name,
		}
	}
	return result, misses
}

func customerPortalStoreProductModelsInCache(items []models.ProductModel) {
	client := customerPortalRedisCacheClient()
	if client == nil || len(items) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), customerPortalCacheIOTimeout)
	defer cancel()
	pipe := client.Pipeline()
	for i := range items {
		cached := customerPortalCachedProductModel{
			ID:        items[i].ID,
			TenantID:  items[i].TenantID,
			ProductID: items[i].ProductID,
			Name:      items[i].Name,
		}
		payload, err := json.Marshal(cached)
		if err != nil {
			continue
		}
		pipe.Set(ctx, customerPortalCacheKey(customerPortalCacheKindProductModel, items[i].ID), payload, customerPortalProductModelCacheTTL)
	}
	_, _ = pipe.Exec(ctx)
}

func loadCustomerPortalUsersByIDCached(userIDs []int64) map[int64]*models.User {
	userIDs = uniqueCustomerPortalInt64s(userIDs)
	result := make(map[int64]*models.User, len(userIDs))
	if len(userIDs) == 0 {
		return result
	}
	cached, misses := customerPortalLoadUsersByIDFromCache(userIDs)
	for id, item := range cached {
		result[id] = item
	}
	if len(misses) == 0 {
		return result
	}
	items := repositories.UserRepository.FindByIds(sqls.DB(), misses)
	if len(items) == 0 {
		return result
	}
	customerPortalStoreUsersInCache(items)
	for i := range items {
		result[items[i].ID] = &items[i]
	}
	return result
}

func customerPortalLoadUsersByIDFromCache(userIDs []int64) (map[int64]*models.User, []int64) {
	result := make(map[int64]*models.User, len(userIDs))
	misses := make([]int64, 0, len(userIDs))
	client := customerPortalRedisCacheClient()
	if client == nil {
		return result, append(misses, userIDs...)
	}
	keys := make([]string, len(userIDs))
	for i, id := range userIDs {
		keys[i] = customerPortalCacheKey(customerPortalCacheKindUser, id)
	}
	ctx, cancel := context.WithTimeout(context.Background(), customerPortalCacheIOTimeout)
	defer cancel()
	rawValues, err := client.MGet(ctx, keys...).Result()
	if err != nil {
		return result, append(misses, userIDs...)
	}
	for i, raw := range rawValues {
		id := userIDs[i]
		var cached customerPortalCachedUser
		if raw == nil || !customerPortalRedisDecodeJSON(raw, &cached) || cached.ID <= 0 {
			misses = append(misses, id)
			continue
		}
		result[id] = &models.User{
			ID:       cached.ID,
			Nickname: cached.Nickname,
			Username: cached.Username,
		}
	}
	return result, misses
}

func customerPortalStoreUsersInCache(items []models.User) {
	client := customerPortalRedisCacheClient()
	if client == nil || len(items) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), customerPortalCacheIOTimeout)
	defer cancel()
	pipe := client.Pipeline()
	for i := range items {
		cached := customerPortalCachedUser{
			ID:       items[i].ID,
			Nickname: items[i].Nickname,
			Username: items[i].Username,
		}
		payload, err := json.Marshal(cached)
		if err != nil {
			continue
		}
		pipe.Set(ctx, customerPortalCacheKey(customerPortalCacheKindUser, items[i].ID), payload, customerPortalUserCacheTTL)
	}
	_, _ = pipe.Exec(ctx)
}

func customerPortalInvalidateUsers(userIDs ...int64) {
	client := customerPortalRedisCacheClient()
	if client == nil {
		return
	}
	ids := uniqueCustomerPortalInt64s(userIDs)
	if len(ids) == 0 {
		return
	}
	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, customerPortalCacheKey(customerPortalCacheKindUser, id))
	}
	ctx, cancel := context.WithTimeout(context.Background(), customerPortalCacheIOTimeout)
	defer cancel()
	_, _ = client.Del(ctx, keys...).Result()
}

func loadCustomerPortalManualCountByProductIDCached(productIDs []int64) map[int64]int {
	productIDs = uniqueCustomerPortalInt64s(productIDs)
	result := make(map[int64]int, len(productIDs))
	if len(productIDs) == 0 {
		return result
	}
	cached, misses := customerPortalLoadManualCountsByProductIDFromCache(productIDs)
	for id, count := range cached {
		result[id] = count
	}
	if len(misses) == 0 {
		return result
	}
	items := repositories.ProductManualFileRepository.Find(sqls.DB(), sqls.NewCnd().
		Where("product_id IN ?", misses).
		Where("(visibility = ? OR visibility = '')", "public").
		Where("status <> ?", enums.StatusDeleted))
	for _, item := range items {
		if item.ProductID > 0 {
			result[item.ProductID]++
		}
	}
	for _, id := range misses {
		if _, ok := result[id]; !ok {
			result[id] = 0
		}
	}
	customerPortalStoreManualCountsInCache(result)
	return result
}

func customerPortalLoadManualCountsByProductIDFromCache(productIDs []int64) (map[int64]int, []int64) {
	result := make(map[int64]int, len(productIDs))
	misses := make([]int64, 0, len(productIDs))
	client := customerPortalRedisCacheClient()
	if client == nil {
		return result, append(misses, productIDs...)
	}
	keys := make([]string, len(productIDs))
	for i, id := range productIDs {
		keys[i] = customerPortalCacheKey(customerPortalCacheKindManualCount, id)
	}
	ctx, cancel := context.WithTimeout(context.Background(), customerPortalCacheIOTimeout)
	defer cancel()
	rawValues, err := client.MGet(ctx, keys...).Result()
	if err != nil {
		return result, append(misses, productIDs...)
	}
	for i, raw := range rawValues {
		id := productIDs[i]
		if raw == nil {
			misses = append(misses, id)
			continue
		}
		var count int
		if !customerPortalRedisDecodeJSON(raw, &count) {
			misses = append(misses, id)
			continue
		}
		result[id] = count
	}
	return result, misses
}

func customerPortalStoreManualCountsInCache(countByProductID map[int64]int) {
	client := customerPortalRedisCacheClient()
	if client == nil || len(countByProductID) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), customerPortalCacheIOTimeout)
	defer cancel()
	pipe := client.Pipeline()
	for productID, count := range countByProductID {
		payload, err := json.Marshal(count)
		if err != nil {
			continue
		}
		pipe.Set(ctx, customerPortalCacheKey(customerPortalCacheKindManualCount, productID), payload, customerPortalManualCountCacheTTL)
	}
	_, _ = pipe.Exec(ctx)
}

func loadCustomerPortalLatestWarrantyByDeviceIDCached(deviceIDs []int64) map[int64]*models.DeviceWarrantyRecord {
	deviceIDs = uniqueCustomerPortalInt64s(deviceIDs)
	result := make(map[int64]*models.DeviceWarrantyRecord, len(deviceIDs))
	if len(deviceIDs) == 0 {
		return result
	}
	cached, misses := customerPortalLoadLatestWarrantyByDeviceIDFromCache(deviceIDs)
	for id, item := range cached {
		result[id] = item
	}
	if len(misses) > 0 {
		items := repositories.DeviceWarrantyRecordRepository.FindByDeviceIDs(sqls.DB(), misses)
		for i := range items {
			record := &items[i]
			if record.DeviceID <= 0 {
				continue
			}
			if _, exists := result[record.DeviceID]; !exists || result[record.DeviceID] == nil || result[record.DeviceID].EndAt.IsZero() {
				result[record.DeviceID] = record
			}
		}
		for _, id := range misses {
			if _, ok := result[id]; !ok {
				result[id] = &models.DeviceWarrantyRecord{DeviceID: id}
			}
		}
		customerPortalStoreLatestWarrantiesInCache(result)
	}
	return result
}

func customerPortalLoadLatestWarrantyByDeviceIDFromCache(deviceIDs []int64) (map[int64]*models.DeviceWarrantyRecord, []int64) {
	result := make(map[int64]*models.DeviceWarrantyRecord, len(deviceIDs))
	misses := make([]int64, 0, len(deviceIDs))
	client := customerPortalRedisCacheClient()
	if client == nil {
		return result, append(misses, deviceIDs...)
	}
	keys := make([]string, len(deviceIDs))
	for i, id := range deviceIDs {
		keys[i] = customerPortalCacheKey(customerPortalCacheKindWarranty, id)
	}
	ctx, cancel := context.WithTimeout(context.Background(), customerPortalCacheIOTimeout)
	defer cancel()
	rawValues, err := client.MGet(ctx, keys...).Result()
	if err != nil {
		return result, append(misses, deviceIDs...)
	}
	for i, raw := range rawValues {
		id := deviceIDs[i]
		var cached customerPortalCachedWarranty
		if raw == nil || !customerPortalRedisDecodeJSON(raw, &cached) {
			misses = append(misses, id)
			continue
		}
		if !cached.HasRecord {
			result[id] = &models.DeviceWarrantyRecord{DeviceID: id}
			continue
		}
		result[id] = &models.DeviceWarrantyRecord{
			DeviceID: id,
			EndAt:    cached.EndAt,
		}
	}
	return result, misses
}

func customerPortalStoreLatestWarrantiesInCache(warrantyByDeviceID map[int64]*models.DeviceWarrantyRecord) {
	client := customerPortalRedisCacheClient()
	if client == nil || len(warrantyByDeviceID) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), customerPortalCacheIOTimeout)
	defer cancel()
	pipe := client.Pipeline()
	for deviceID, warranty := range warrantyByDeviceID {
		cached := customerPortalCachedWarranty{DeviceID: deviceID}
		if warranty != nil && !warranty.EndAt.IsZero() {
			cached.HasRecord = true
			cached.EndAt = warranty.EndAt
		}
		payload, err := json.Marshal(cached)
		if err != nil {
			continue
		}
		pipe.Set(ctx, customerPortalCacheKey(customerPortalCacheKindWarranty, deviceID), payload, customerPortalWarrantyCacheTTL)
	}
	_, _ = pipe.Exec(ctx)
}
