package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/actionlab-ai/aisphere-kit/metrics"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type MySQLConfig struct {
	Driver          string `json:"driver" yaml:"driver"`
	DSN             string `json:"dsn" yaml:"dsn"`
	MaxOpenConns    int    `json:"max_open_conns" yaml:"max_open_conns"`
	MaxIdleConns    int    `json:"max_idle_conns" yaml:"max_idle_conns"`
	ConnMaxLifetime string `json:"conn_max_lifetime" yaml:"conn_max_lifetime"`
	SlowThreshold   string `json:"slow_threshold" yaml:"slow_threshold"`
	LogLevel        string `json:"log_level" yaml:"log_level"`
}

type MySQL struct {
	DB      *gorm.DB
	SQLDB   *sql.DB
	logger  *slog.Logger
	metrics *metrics.Metrics
}

type Option func(*options)

type options struct {
	logger  *slog.Logger
	metrics *metrics.Metrics
}

func WithLogger(l *slog.Logger) Option      { return func(o *options) { o.logger = l } }
func WithMetrics(m *metrics.Metrics) Option { return func(o *options) { o.metrics = m } }

func NewMySQL(ctx context.Context, cfg MySQLConfig, opts ...Option) (*MySQL, error) {
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
	l = l.With("component", "db", "driver", firstNonEmpty(cfg.Driver, "mysql"))
	started := time.Now()
	l.Info("mysql init started", "max_open_conns", cfg.MaxOpenConns, "max_idle_conns", cfg.MaxIdleConns)
	if cfg.Driver != "" && cfg.Driver != "mysql" {
		err := fmt.Errorf("unsupported database.driver %q", cfg.Driver)
		l.Error("mysql config invalid", "error", err)
		return nil, err
	}
	if cfg.DSN == "" {
		err := fmt.Errorf("database.dsn is required")
		l.Error("mysql config invalid", "error", err)
		return nil, err
	}
	slow := 500 * time.Millisecond
	if cfg.SlowThreshold != "" {
		d, err := time.ParseDuration(cfg.SlowThreshold)
		if err != nil {
			l.Error("mysql slow threshold parse failed", "error", err, "value", cfg.SlowThreshold)
			return nil, fmt.Errorf("parse database.slow_threshold: %w", err)
		}
		if d > 0 {
			slow = d
		}
	}
	gcfg := &gorm.Config{Logger: logger.New(slogWriter{logger: l}, logger.Config{SlowThreshold: slow, LogLevel: parseLogLevel(cfg.LogLevel), IgnoreRecordNotFoundError: true, Colorful: false})}
	l.Debug("mysql opening connection", "slow_threshold", slow.String(), "gorm_log_level", firstNonEmpty(cfg.LogLevel, "warn"))
	db, err := gorm.Open(mysql.Open(cfg.DSN), gcfg)
	if err != nil {
		if opt.metrics != nil {
			opt.metrics.ObserveDependency("mysql", "open", started, err)
		}
		l.Error("mysql open failed", "error", err)
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		if opt.metrics != nil {
			opt.metrics.ObserveDependency("mysql", "db", started, err)
		}
		l.Error("mysql sql db failed", "error", err)
		return nil, fmt.Errorf("mysql sql db: %w", err)
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != "" {
		d, err := time.ParseDuration(cfg.ConnMaxLifetime)
		if err != nil {
			_ = sqlDB.Close()
			l.Error("mysql conn max lifetime parse failed", "error", err, "value", cfg.ConnMaxLifetime)
			return nil, fmt.Errorf("parse database.conn_max_lifetime: %w", err)
		}
		sqlDB.SetConnMaxLifetime(d)
	}
	l.Debug("mysql ping started")
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		if opt.metrics != nil {
			opt.metrics.ObserveDependency("mysql", "ping", started, err)
		}
		l.Error("mysql ping failed", "error", err, "elapsed", time.Since(started).String())
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	if opt.metrics != nil {
		opt.metrics.ObserveDependency("mysql", "init", started, nil)
	}
	l.Info("mysql init completed", "elapsed", time.Since(started).String())
	return &MySQL{DB: db, SQLDB: sqlDB, logger: l, metrics: opt.metrics}, nil
}

type slogWriter struct{ logger *slog.Logger }

func (w slogWriter) Printf(format string, args ...any) {
	l := w.logger
	if l == nil {
		l = slog.Default()
	}
	l.Debug("gorm", "message", fmt.Sprintf(format, args...))
}

func (m *MySQL) Close() error {
	if m == nil || m.SQLDB == nil {
		return nil
	}
	started := time.Now()
	err := m.SQLDB.Close()
	if m.logger != nil {
		if err != nil {
			m.logger.Warn("mysql close failed", "error", err)
		} else {
			m.logger.Info("mysql closed", "elapsed", time.Since(started).String())
		}
	}
	return err
}
func (m *MySQL) Ping(ctx context.Context) error {
	if m == nil || m.SQLDB == nil {
		return fmt.Errorf("mysql not initialized")
	}
	started := time.Now()
	err := m.SQLDB.PingContext(ctx)
	if m.metrics != nil {
		m.metrics.ObserveDependency("mysql", "ping", started, err)
	}
	if m.logger != nil {
		if err != nil {
			m.logger.Warn("mysql health check failed", "error", err)
		} else {
			m.logger.Debug("mysql health check ok", "elapsed", time.Since(started).String())
		}
	}
	return err
}

func Transaction(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error) error {
	if db == nil {
		return fmt.Errorf("gorm db is nil")
	}
	return db.WithContext(ctx).Transaction(fn)
}

func parseLogLevel(level string) logger.LogLevel {
	switch level {
	case "silent":
		return logger.Silent
	case "info":
		return logger.Info
	case "error":
		return logger.Error
	default:
		return logger.Warn
	}
}
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
