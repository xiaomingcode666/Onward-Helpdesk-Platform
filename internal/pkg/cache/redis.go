package cache

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"remotehelpdesk/internal/pkg/config"

	"github.com/go-redis/redis/v8"
)

var (
	redisClientOnce sync.Once
	redisClient     *redis.Client
)

func Client() *redis.Client {
	redisClientOnce.Do(func() {
		cfg := config.CurrentOrDefault().Redis
		addr := strings.TrimSpace(cfg.Addr)
		if addr == "" {
			addr = "127.0.0.1:6379"
		}
		redisClient = redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: cfg.Password,
			DB:       cfg.DB,
		})
	})
	return redisClient
}

func Ping(ctx context.Context) error {
	return Client().Ping(ctx).Err()
}

func SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return Client().Set(ctx, key, raw, ttl).Err()
}

func GetString(ctx context.Context, key string) (string, bool, error) {
	value, err := Client().Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func GetJSON(ctx context.Context, key string, target any) (bool, error) {
	value, err := Client().Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(value, target)
}

func Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return Client().Del(ctx, keys...).Err()
}
