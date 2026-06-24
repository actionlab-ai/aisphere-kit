package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrStateMismatch = errors.New("oauth state mismatch")

type Config struct {
	KeyPrefix       string `json:"key_prefix" yaml:"key_prefix"`
	StateTTL        string `json:"state_ttl" yaml:"state_ttl"`
	BlacklistPrefix string `json:"blacklist_prefix" yaml:"blacklist_prefix"`
}

type Manager struct {
	rdb    redis.UniversalClient
	cfg    Config
	logger *slog.Logger
}

type Option func(*Manager)

func WithLogger(l *slog.Logger) Option { return func(m *Manager) { m.logger = l } }

func NewManager(rdb redis.UniversalClient, cfg Config, opts ...Option) *Manager {
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "aisphere:session"
	}
	if cfg.BlacklistPrefix == "" {
		cfg.BlacklistPrefix = cfg.KeyPrefix + ":blacklist"
	}
	if cfg.StateTTL == "" {
		cfg.StateTTL = "10m"
	}
	m := &Manager{rdb: rdb, cfg: cfg, logger: slog.Default().With("component", "session")}
	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}
	return m
}

func (m *Manager) NewState(ctx context.Context, subject string) (string, error) {
	if m == nil || m.rdb == nil {
		return "", fmt.Errorf("session redis is nil")
	}
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := hex.EncodeToString(b)
	ttl := parseDurationOr(m.cfg.StateTTL, 10*time.Minute)
	key := m.key("oauth_state", state)
	if err := m.rdb.Set(ctx, key, subject, ttl).Err(); err != nil {
		return "", err
	}
	m.logger.Debug("oauth state created", "subject", subject, "ttl", ttl.String())
	return state, nil
}

func (m *Manager) VerifyState(ctx context.Context, state string) (string, error) {
	if m == nil || m.rdb == nil {
		return "", fmt.Errorf("session redis is nil")
	}
	if strings.TrimSpace(state) == "" {
		return "", ErrStateMismatch
	}
	key := m.key("oauth_state", state)
	v, err := m.rdb.GetDel(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrStateMismatch
	}
	if err != nil {
		return "", err
	}
	m.logger.Debug("oauth state verified")
	return v, nil
}

func (m *Manager) RevokeToken(ctx context.Context, tokenOrID string, ttl time.Duration) error {
	if m == nil || m.rdb == nil {
		return fmt.Errorf("session redis is nil")
	}
	if strings.TrimSpace(tokenOrID) == "" {
		return nil
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	key := m.blacklistKey(tokenOrID)
	if err := m.rdb.Set(ctx, key, "1", ttl).Err(); err != nil {
		return err
	}
	m.logger.Info("token revoked", "ttl", ttl.String())
	return nil
}

func (m *Manager) IsRevoked(ctx context.Context, tokenOrID string) (bool, error) {
	if m == nil || m.rdb == nil {
		return false, nil
	}
	if strings.TrimSpace(tokenOrID) == "" {
		return false, nil
	}
	v, err := m.rdb.Exists(ctx, m.blacklistKey(tokenOrID)).Result()
	if err != nil {
		return false, err
	}
	return v > 0, nil
}

func (m *Manager) key(parts ...string) string {
	return strings.Join(append([]string{strings.TrimSuffix(m.cfg.KeyPrefix, ":")}, parts...), ":")
}
func (m *Manager) blacklistKey(tokenOrID string) string {
	return strings.TrimSuffix(m.cfg.BlacklistPrefix, ":") + ":" + hashToken(tokenOrID)
}
func hashToken(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }
func parseDurationOr(s string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}
