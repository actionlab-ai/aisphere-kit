package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrCacheMiss = errors.New("cache miss")

func SetJSON(ctx context.Context, rdb redis.UniversalClient, key string, value any, ttl time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return rdb.Set(ctx, key, b, ttl).Err()
}

func GetJSON[T any](ctx context.Context, rdb redis.UniversalClient, key string) (T, error) {
	var zero T
	b, err := rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return zero, ErrCacheMiss
	}
	if err != nil {
		return zero, err
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		return zero, err
	}
	return out, nil
}

func RememberJSON[T any](ctx context.Context, rdb redis.UniversalClient, key string, ttl time.Duration, load func(context.Context) (T, error)) (T, error) {
	v, err := GetJSON[T](ctx, rdb, key)
	if err == nil {
		return v, nil
	}
	if !errors.Is(err, ErrCacheMiss) {
		return v, err
	}
	v, err = load(ctx)
	if err != nil {
		return v, err
	}
	return v, SetJSON(ctx, rdb, key, v, ttl)
}
