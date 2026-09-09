package services

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"remotehelpdesk/internal/ai/rag/vectordb"
	"remotehelpdesk/internal/pkg/cache"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services/storage"

	"github.com/mlogclub/simple/sqls"
)

type PlatformOpsInfrastructureAggregate struct {
	GeneratedAt        time.Time
	TotalMeasuredBytes int64
	Dependencies       []PlatformInfrastructureDependencyAggregate
	DatabaseTables     []repositories.PlatformDatabaseTableStorageRow
}

type PlatformInfrastructureDependencyAggregate struct {
	Key             string
	Name            string
	Provider        string
	Status          string
	Endpoint        string
	LatencyMs       int64
	StoredBytes     int64
	AllocatedBytes  int64
	CapacityBytes   int64
	SharePercent    float64
	CapacityPercent float64
	ItemCount       int64
	ItemUnit        string
	SecondaryValue  int64
	SecondaryLabel  string
	Estimated       bool
	Measurement     string
	Error           string
}

func (s *platformConsoleService) GetOpsInfrastructure(now time.Time) *PlatformOpsInfrastructureAggregate {
	dependencies := make([]PlatformInfrastructureDependencyAggregate, 4)
	var databaseTables []repositories.PlatformDatabaseTableStorageRow
	var wait sync.WaitGroup
	wait.Add(4)
	go func() {
		defer wait.Done()
		dependencies[0], databaseTables = probePlatformDatabaseInfrastructure()
	}()
	go func() {
		defer wait.Done()
		dependencies[1] = probePlatformRedisInfrastructure()
	}()
	go func() {
		defer wait.Done()
		dependencies[2] = probePlatformVectorInfrastructure()
	}()
	go func() {
		defer wait.Done()
		dependencies[3] = probePlatformObjectStorageInfrastructure()
	}()
	wait.Wait()

	var total int64
	for i := range dependencies {
		if dependencies[i].StoredBytes > 0 {
			total += dependencies[i].StoredBytes
		}
		if dependencies[i].CapacityBytes > 0 && dependencies[i].AllocatedBytes >= 0 {
			dependencies[i].CapacityPercent = clampPercent(float64(dependencies[i].AllocatedBytes) / float64(dependencies[i].CapacityBytes) * 100)
		}
	}
	if total > 0 {
		for i := range dependencies {
			dependencies[i].SharePercent = clampPercent(float64(dependencies[i].StoredBytes) / float64(total) * 100)
		}
	}
	return &PlatformOpsInfrastructureAggregate{
		GeneratedAt: now, TotalMeasuredBytes: total,
		Dependencies: dependencies, DatabaseTables: databaseTables,
	}
}

func probePlatformDatabaseInfrastructure() (PlatformInfrastructureDependencyAggregate, []repositories.PlatformDatabaseTableStorageRow) {
	startedAt := time.Now()
	cfg := config.CurrentOrDefault()
	ret := PlatformInfrastructureDependencyAggregate{
		Key: "database", Name: "业务数据库", Provider: databaseProviderLabel(cfg.DB.Type),
		Status: "unhealthy", Endpoint: databaseEndpointLabel(cfg.DB.Type), ItemUnit: "张表",
		SecondaryLabel: "打开连接", Measurement: "数据库物理占用",
	}
	database, err := repositories.PlatformConsoleRepository.PingDatabase(sqls.DB())
	ret.LatencyMs = time.Since(startedAt).Milliseconds()
	if err != nil {
		ret.Error = err.Error()
		return ret, nil
	}
	ret.Status = "healthy"
	ret.SecondaryValue = int64(database.Stats.OpenConnections)
	storageSnapshot, err := repositories.PlatformConsoleRepository.GetDatabaseStorageSnapshot(sqls.DB(), 8)
	if storageSnapshot != nil {
		ret.StoredBytes = storageSnapshot.SizeBytes
		ret.AllocatedBytes = storageSnapshot.SizeBytes
		ret.ItemCount = storageSnapshot.TableCount
	}
	if err != nil {
		ret.Status = "degraded"
		ret.Error = err.Error()
		if storageSnapshot != nil {
			return ret, storageSnapshot.Tables
		}
		return ret, nil
	}
	return ret, storageSnapshot.Tables
}

func probePlatformRedisInfrastructure() PlatformInfrastructureDependencyAggregate {
	startedAt := time.Now()
	cfg := config.CurrentOrDefault().Redis
	endpoint := strings.TrimSpace(cfg.Addr)
	if endpoint == "" {
		endpoint = "127.0.0.1:6379"
	}
	ret := PlatformInfrastructureDependencyAggregate{
		Key: "redis", Name: "Redis 缓存与队列", Provider: "Redis", Status: "unhealthy",
		Endpoint: endpoint, ItemUnit: "个键", SecondaryLabel: "客户端", Measurement: "Redis 数据集内存",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client := cache.Client()
	if err := client.Ping(ctx).Err(); err != nil {
		ret.LatencyMs = time.Since(startedAt).Milliseconds()
		ret.Error = err.Error()
		return ret
	}
	ret.Status = "healthy"
	ret.LatencyMs = time.Since(startedAt).Milliseconds()
	if count, err := client.DBSize(ctx).Result(); err == nil {
		ret.ItemCount = count
	} else {
		ret.Status = "degraded"
		ret.Error = err.Error()
	}
	infoSections := make([]string, 0, 3)
	for _, section := range []string{"memory", "clients", "server"} {
		info, infoErr := client.Info(ctx, section).Result()
		if infoErr != nil {
			ret.Status = "degraded"
			ret.Error = joinInfrastructureError(ret.Error, infoErr.Error())
			continue
		}
		infoSections = append(infoSections, info)
	}
	values := parseRedisInfo(strings.Join(infoSections, "\n"))
	ret.StoredBytes = redisInfoInt64(values, "used_memory_dataset")
	ret.AllocatedBytes = redisInfoInt64(values, "used_memory")
	ret.CapacityBytes = redisInfoInt64(values, "maxmemory")
	ret.SecondaryValue = redisInfoInt64(values, "connected_clients")
	if ret.StoredBytes == 0 {
		ret.StoredBytes = ret.AllocatedBytes
	}
	if version := strings.TrimSpace(values["redis_version"]); version != "" {
		ret.Provider = "Redis " + version
	}
	return ret
}

func probePlatformVectorInfrastructure() PlatformInfrastructureDependencyAggregate {
	startedAt := time.Now()
	cfg := config.CurrentOrDefault().VectorDB
	providerType := strings.ToLower(strings.TrimSpace(cfg.Type))
	ret := PlatformInfrastructureDependencyAggregate{
		Key: "vector", Name: "向量数据库", Provider: vectorProviderLabel(providerType), Status: "unhealthy",
		Endpoint: vectorEndpointLabel(cfg), ItemUnit: "个向量", SecondaryLabel: "集合",
		Estimated: true, Measurement: "Float32 原始向量估算",
	}
	provider := vectordb.GetProvider()
	if provider == nil {
		ret.Error = "vector database provider not initialized"
		return ret
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	collections, err := provider.ListCollections(ctx)
	ret.LatencyMs = time.Since(startedAt).Milliseconds()
	if err != nil {
		ret.Error = err.Error()
		return ret
	}
	ret.Status = "healthy"
	ret.SecondaryValue = int64(len(collections))
	for _, name := range collections {
		info, infoErr := provider.GetCollection(ctx, name)
		if infoErr != nil {
			ret.Status = "degraded"
			ret.Error = joinInfrastructureError(ret.Error, fmt.Sprintf("%s: %v", name, infoErr))
			continue
		}
		points := int64(info.PointCount)
		ret.ItemCount += points
		if points > 0 && info.Dimension > 0 {
			ret.StoredBytes += points * int64(info.Dimension) * 4
		}
	}
	ret.AllocatedBytes = ret.StoredBytes
	ret.LatencyMs = time.Since(startedAt).Milliseconds()
	return ret
}

func probePlatformObjectStorageInfrastructure() PlatformInfrastructureDependencyAggregate {
	startedAt := time.Now()
	cfg := config.CurrentOrDefault().Storage
	providerType := cfg.Default
	if providerType == "" {
		providerType = enums.AssetProviderLocal
	}
	ret := PlatformInfrastructureDependencyAggregate{
		Key: "object_storage", Name: "对象存储", Provider: storageProviderLabel(providerType), Status: "unhealthy",
		Endpoint: storageEndpointLabel(cfg), ItemUnit: "个对象", SecondaryLabel: "存储桶",
		Measurement: "业务文件登记值",
	}
	provider, err := storage.GetDefault()
	if err != nil {
		ret.Error = err.Error()
		return ret
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = provider.HealthCheck(ctx); err != nil {
		ret.LatencyMs = time.Since(startedAt).Milliseconds()
		ret.Error = err.Error()
		return ret
	}
	ret.Status = "healthy"
	ret.LatencyMs = time.Since(startedAt).Milliseconds()
	rows, err := repositories.PlatformConsoleRepository.GetAssetStorageRows(sqls.DB())
	if err != nil {
		ret.Status = "degraded"
		ret.Error = err.Error()
		return ret
	}
	for _, row := range rows {
		if strings.EqualFold(strings.TrimSpace(row.Provider), string(providerType)) {
			ret.ItemCount += row.ObjectCount
			ret.StoredBytes += row.SizeBytes
		}
	}
	ret.AllocatedBytes = ret.StoredBytes
	if providerType == enums.AssetProviderMinIO || providerType == enums.AssetProviderOSS {
		ret.SecondaryValue = 1
	}
	return ret
}

func parseRedisInfo(raw string) map[string]string {
	ret := make(map[string]string)
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if ok {
			ret[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return ret
}

func redisInfoInt64(values map[string]string, key string) int64 {
	value, _ := strconv.ParseInt(strings.TrimSpace(values[key]), 10, 64)
	return value
}

func joinInfrastructureError(current, next string) string {
	current = strings.TrimSpace(current)
	next = strings.TrimSpace(next)
	if current == "" {
		return next
	}
	if next == "" {
		return current
	}
	return current + "; " + next
}

func clampPercent(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return math.Round(value*10) / 10
}

func databaseProviderLabel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "postgres", "postgresql":
		return "PostgreSQL"
	case "sqlite", "sqlite3":
		return "SQLite"
	default:
		return strings.TrimSpace(value)
	}
}

func databaseEndpointLabel(value string) string {
	provider := databaseProviderLabel(value)
	if provider == "" {
		return "默认数据源"
	}
	return provider + " 主数据源"
}

func vectorProviderLabel(value string) string {
	switch value {
	case "qdrant":
		return "Qdrant"
	case "lancedb":
		return "LanceDB"
	default:
		return value
	}
}

func vectorEndpointLabel(cfg config.VectorDBConfig) string {
	if strings.EqualFold(strings.TrimSpace(cfg.Type), "lancedb") {
		return strings.TrimSpace(cfg.LanceDB.Path)
	}
	host := strings.TrimSpace(cfg.Qdrant.Host)
	if host == "" {
		host = "localhost"
	}
	port := cfg.Qdrant.GrpcPort
	if port <= 0 {
		port = 6334
	}
	return fmt.Sprintf("%s:%d", host, port)
}

func storageProviderLabel(value enums.AssetProvider) string {
	switch value {
	case enums.AssetProviderMinIO:
		return "MinIO"
	case enums.AssetProviderOSS:
		return "OSS"
	case enums.AssetProviderLocal:
		return "本地文件"
	default:
		return string(value)
	}
}

func storageEndpointLabel(cfg config.StorageConfig) string {
	switch cfg.Default {
	case enums.AssetProviderMinIO:
		return strings.TrimRight(strings.TrimSpace(cfg.MinIO.Endpoint), "/") + "/" + strings.TrimSpace(cfg.MinIO.Bucket)
	case enums.AssetProviderOSS:
		return strings.TrimRight(strings.TrimSpace(cfg.OSS.Endpoint), "/") + "/" + strings.TrimSpace(cfg.OSS.Bucket)
	default:
		return strings.TrimSpace(cfg.Local.Root)
	}
}
