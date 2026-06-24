package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/actionlab-ai/aisphere-kit/metrics"
	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Mode         string   `json:"mode" yaml:"mode"`
	Addr         string   `json:"addr" yaml:"addr"`
	Addrs        []string `json:"addrs" yaml:"addrs"`
	Username     string   `json:"username" yaml:"username"`
	Password     string   `json:"password" yaml:"password"`
	DB           int      `json:"db" yaml:"db"`
	DialTimeout  string   `json:"dial_timeout" yaml:"dial_timeout"`
	ReadTimeout  string   `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout string   `json:"write_timeout" yaml:"write_timeout"`
	PoolSize     int      `json:"pool_size" yaml:"pool_size"`
}

type Client interface{ redis.UniversalClient }

type Redis struct {
	Client  redis.UniversalClient
	logger  *slog.Logger
	metrics *metrics.Metrics
	mode    string
}

type Option func(*options)
type options struct {
	logger  *slog.Logger
	metrics *metrics.Metrics
}

func WithLogger(l *slog.Logger) Option      { return func(o *options) { o.logger = l } }
func WithMetrics(m *metrics.Metrics) Option { return func(o *options) { o.metrics = m } }

func NewRedis(ctx context.Context, cfg RedisConfig, opts ...Option) (*Redis, error) {
	var opt options
	for _, fn := range opts {
		if fn != nil {
			fn(&opt)
		}
	}
	l := opt.logger
	if l == nil {
		l = slog.Default()
	}
	mode := cfg.Mode
	if mode == "" {
		mode = "single"
	}
	l = l.With("component", "cache", "backend", "redis", "mode", mode)
	started := time.Now()
	l.Info("redis init started")
	var client redis.UniversalClient
	switch mode {
	case "single":
		if cfg.Addr == "" {
			err := fmt.Errorf("redis.addr is required")
			l.Error("redis config invalid", "error", err)
			return nil, err
		}
		l.Debug("redis single client creating", "addr", cfg.Addr, "db", cfg.DB, "pool_size", cfg.PoolSize)
		client = redis.NewClient(&redis.Options{Addr: cfg.Addr, Username: cfg.Username, Password: cfg.Password, DB: cfg.DB, DialTimeout: durOrDefault(cfg.DialTimeout, 5*time.Second), ReadTimeout: durOrDefault(cfg.ReadTimeout, 3*time.Second), WriteTimeout: durOrDefault(cfg.WriteTimeout, 3*time.Second), PoolSize: cfg.PoolSize})
	case "cluster":
		if len(cfg.Addrs) == 0 {
			err := fmt.Errorf("redis.addrs is required for cluster")
			l.Error("redis config invalid", "error", err)
			return nil, err
		}
		l.Debug("redis cluster client creating", "addrs", cfg.Addrs, "pool_size", cfg.PoolSize)
		client = redis.NewClusterClient(&redis.ClusterOptions{Addrs: cfg.Addrs, Username: cfg.Username, Password: cfg.Password, DialTimeout: durOrDefault(cfg.DialTimeout, 5*time.Second), ReadTimeout: durOrDefault(cfg.ReadTimeout, 3*time.Second), WriteTimeout: durOrDefault(cfg.WriteTimeout, 3*time.Second), PoolSize: cfg.PoolSize})
	default:
		err := fmt.Errorf("redis.mode only supports single or cluster")
		l.Error("redis config invalid", "error", err, "mode", mode)
		return nil, err
	}
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		if opt.metrics != nil {
			opt.metrics.ObserveDependency("redis", "ping", started, err)
		}
		l.Error("redis ping failed", "error", err, "elapsed", time.Since(started).String())
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	if opt.metrics != nil {
		opt.metrics.ObserveDependency("redis", "init", started, nil)
	}
	l.Info("redis init completed", "elapsed", time.Since(started).String())
	return &Redis{Client: client, logger: l, metrics: opt.metrics, mode: mode}, nil
}

func (r *Redis) Close() error {
	if r == nil || r.Client == nil {
		return nil
	}
	started := time.Now()
	err := r.Client.Close()
	if r.logger != nil {
		if err != nil {
			r.logger.Warn("redis close failed", "error", err)
		} else {
			r.logger.Info("redis closed", "elapsed", time.Since(started).String())
		}
	}
	return err
}
func (r *Redis) Ping(ctx context.Context) error {
	if r == nil || r.Client == nil {
		return fmt.Errorf("redis not initialized")
	}
	started := time.Now()
	err := r.Client.Ping(ctx).Err()
	if r.metrics != nil {
		r.metrics.ObserveDependency("redis", "ping", started, err)
	}
	if r.logger != nil {
		if err != nil {
			r.logger.Warn("redis health check failed", "error", err)
		} else {
			r.logger.Debug("redis health check ok", "elapsed", time.Since(started).String())
		}
	}
	return err
}

func durOrDefault(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return fallback
	}
	return d
}
