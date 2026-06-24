package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Lock struct {
	client redis.UniversalClient
	key    string
	token  string
}

func TryLock(ctx context.Context, client redis.UniversalClient, key string, ttl time.Duration) (*Lock, bool, error) {
	if client == nil {
		return nil, false, fmt.Errorf("redis client is nil")
	}
	if key == "" {
		return nil, false, fmt.Errorf("lock key is required")
	}
	if ttl <= 0 {
		return nil, false, fmt.Errorf("lock ttl must be positive")
	}
	token, err := randomToken()
	if err != nil {
		return nil, false, err
	}
	ok, err := client.SetNX(ctx, key, token, ttl).Result()
	if err != nil || !ok {
		return nil, ok, err
	}
	return &Lock{client: client, key: key, token: token}, true, nil
}

func LockWithWait(ctx context.Context, client redis.UniversalClient, key string, ttl, wait, interval time.Duration) (*Lock, error) {
	deadline := time.Now().Add(wait)
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	for {
		l, ok, err := TryLock(ctx, client, key, ttl)
		if err != nil {
			return nil, err
		}
		if ok {
			return l, nil
		}
		if wait <= 0 || time.Now().After(deadline) {
			return nil, fmt.Errorf("lock %s not acquired", key)
		}
		t := time.NewTimer(interval)
		select {
		case <-t.C:
		case <-ctx.Done():
			if !t.Stop() {
				<-t.C
			}
			return nil, ctx.Err()
		}
	}
}

func (l *Lock) Token() string {
	if l == nil {
		return ""
	}
	return l.token
}
func (l *Lock) Key() string {
	if l == nil {
		return ""
	}
	return l.key
}

func (l *Lock) Refresh(ctx context.Context, ttl time.Duration) (bool, error) {
	if l == nil {
		return false, nil
	}
	if ttl <= 0 {
		return false, fmt.Errorf("lock ttl must be positive")
	}
	const script = `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("PEXPIRE", KEYS[1], ARGV[2]) else return 0 end`
	res, err := l.client.Eval(ctx, script, []string{l.key}, l.token, ttl.Milliseconds()).Int()
	return res == 1, err
}

func (l *Lock) Unlock(ctx context.Context) error {
	if l == nil {
		return nil
	}
	const script = `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) else return 0 end`
	res, err := l.client.Eval(ctx, script, []string{l.key}, l.token).Int()
	if err != nil {
		return err
	}
	if res == 0 {
		return fmt.Errorf("lock %s is not owned by current token", l.key)
	}
	return nil
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
