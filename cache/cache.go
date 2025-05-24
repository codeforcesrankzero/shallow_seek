package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shallowseek/config"
	"github.com/shallowseek/models"
)

var (
	redisClient *redis.Client
	ctx         = context.Background()
)

func Init() error {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     config.GetRedisURL(),
		Password: "",
		DB:       0,
	})

	_, err := redisClient.Ping(ctx).Result()
	return err
}

func CacheSearchResult(query string, results models.SimplifiedSearchResult) error {
	key := "search:" + query
	data, err := json.Marshal(results)
	if err != nil {
		return err
	}

	return redisClient.Set(ctx, key, data, 5*time.Minute).Err()
}

func GetCachedSearchResult(query string) (*models.SimplifiedSearchResult, error) {
	key := "search:" + query
	data, err := redisClient.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var results models.SimplifiedSearchResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}

	return &results, nil
}

func InvalidateCache(query string) error {
	key := "search:" + query
	return redisClient.Del(ctx, key).Err()
} 