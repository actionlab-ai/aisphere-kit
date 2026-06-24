package starter

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/actionlab-ai/aisphere-kit/audit"
	"github.com/actionlab-ai/aisphere-kit/authn"
	"github.com/actionlab-ai/aisphere-kit/authz"
	"github.com/actionlab-ai/aisphere-kit/cache"
	"github.com/actionlab-ai/aisphere-kit/casdoor"
	"github.com/actionlab-ai/aisphere-kit/config"
	"github.com/actionlab-ai/aisphere-kit/db"
	"github.com/actionlab-ai/aisphere-kit/logx"
	"github.com/actionlab-ai/aisphere-kit/metrics"
	"github.com/actionlab-ai/aisphere-kit/objectstore"
	"github.com/actionlab-ai/aisphere-kit/permission"
	"github.com/actionlab-ai/aisphere-kit/session"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Runtime struct {
	Config   *config.Config
	Logger   *slog.Logger
	DB       *gorm.DB
	Database db.Database
	// Deprecated: use Database. Kept for compatibility with older services.
	SQL          db.Database
	Redis        redis.UniversalClient
	RedisRuntime *cache.Redis
	S3           objectstore.Client
	Casdoor      *casdoor.Adapter
	Authn        authn.Authenticator
	Authz        authz.Authorizer
	Audit        audit.Recorder
	Session      *session.Manager
	Tx           *db.TxManager
	Permission   permission.Manager
	Metrics      *metrics.Metrics
}

type RuntimeOption func(*Runtime)

func NewRuntime(ctx context.Context, cfg *config.Config, opts ...RuntimeOption) (*Runtime, func() error, error) {
	cleanups := &CleanupStack{}
	logger := logx.New(cfg.Log).With("app", cfg.App.Name, "env", cfg.App.Env, "version", cfg.App.Version)
	rt := &Runtime{Config: cfg, Logger: logger}
	for _, opt := range opts {
		if opt != nil {
			opt(rt)
		}
	}
	ctx = logx.NewContext(ctx, rt.Logger)
	startedAll := time.Now()
	rt.Logger.Info("aisphere runtime init started", "features", cfg.Features)
	defer func() {
		rt.Logger.Debug("aisphere runtime init defer reached", "elapsed", time.Since(startedAll).String())
	}()
	if cfg.Features.Metrics {
		started := time.Now()
		rt.Logger.Debug("metrics init started")
		m, err := metrics.New(cfg.Metrics, cfg.App.Name)
		if err != nil {
			_ = cleanups.Close()
			rt.Logger.Error("metrics init failed", "error", err)
			return nil, nil, fmt.Errorf("init metrics: %w", err)
		}
		rt.Metrics = m
		rt.Logger.Info("metrics init completed", "elapsed", time.Since(started).String())
	} else {
		rt.Logger.Info("metrics disabled")
	}
	if cfg.Features.DB {
		sqldb, err := db.NewDatabase(ctx, cfg.Database, db.WithLogger(rt.Logger), db.WithMetrics(rt.Metrics))
		if err != nil {
			_ = cleanups.Close()
			return nil, nil, err
		}
		rt.Database = sqldb
		rt.SQL = sqldb
		rt.DB = sqldb.GORM()
		rt.Tx = db.NewTxManager(sqldb.GORM(), rt.Logger)
		cleanups.Add(sqldb.Close)
	} else {
		rt.Logger.Info("database disabled")
	}
	if cfg.Features.Cache {
		r, err := cache.NewRedis(ctx, cfg.Redis, cache.WithLogger(rt.Logger), cache.WithMetrics(rt.Metrics))
		if err != nil {
			_ = cleanups.Close()
			return nil, nil, err
		}
		rt.RedisRuntime = r
		rt.Redis = r.Client
		if cfg.Features.Session {
			rt.Session = session.NewManager(r.Client, cfg.Session, session.WithLogger(rt.Logger))
		}
		cleanups.Add(r.Close)
	} else {
		rt.Logger.Info("redis disabled")
	}
	if cfg.Features.S3 {
		s3, err := objectstore.NewMinIO(ctx, cfg.ObjectStore, objectstore.WithLogger(rt.Logger), objectstore.WithMetrics(rt.Metrics))
		if err != nil {
			_ = cleanups.Close()
			return nil, nil, err
		}
		rt.S3 = s3
	} else {
		rt.Logger.Info("objectstore disabled")
	}
	if cfg.Features.Authn || cfg.Features.Authz || cfg.Features.Audit || cfg.Features.Permission {
		rt.Casdoor = casdoor.New(cfg.Casdoor, casdoor.WithLogger(rt.Logger), casdoor.WithMetrics(rt.Metrics))
		rt.Authn = rt.Casdoor
		rt.Authz = rt.Casdoor
		rt.Audit = rt.Casdoor
		if cfg.Features.Permission {
			rt.Permission = rt.Casdoor
		}
	} else {
		rt.Logger.Info("casdoor disabled")
	}
	rt.Logger.Info("aisphere runtime init completed", "elapsed", time.Since(startedAll).String())
	return rt, cleanups.Close, nil
}

func (rt *Runtime) WithTx(ctx context.Context, fn func(ctx context.Context, tx *gorm.DB) error) error {
	if rt == nil || rt.Tx == nil {
		return fmt.Errorf("runtime transaction manager is nil")
	}
	return rt.Tx.WithTx(ctx, fn)
}

func (rt *Runtime) DBFromContext(ctx context.Context) *gorm.DB {
	if rt == nil || rt.Tx == nil {
		return nil
	}
	return rt.Tx.DB(ctx)
}
