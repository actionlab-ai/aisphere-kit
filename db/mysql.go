package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// MySQLConfig is retained for backward compatibility. It is now an alias
// for DatabaseConfig so existing YAML and code paths keep working.
//
// Deprecated: prefer DatabaseConfig (which is the same type under a clearer
// name) for new code. New postgres-specific fields are accessible through
// the alias as well.
type MySQLConfig = DatabaseConfig

// MySQL wraps a *gorm.DB opened against a MySQL backend.
//
// It satisfies the Database interface so callers can keep using the
// concrete type or migrate to the interface uniformly.
type MySQL struct {
	DB     *gorm.DB
	SQLDB  *sql.DB
	logger *slog.Logger
	opts   options // carries metrics; logger kept separately for clarity
}

// NewMySQL opens a MySQL connection via gorm.io/driver/mysql.
//
// Behavior is unchanged from v0.1.0: cfg.Driver must be empty or "mysql",
// otherwise an error is returned. Use NewDatabase for driver dispatch.
func NewMySQL(ctx context.Context, cfg MySQLConfig, opts ...Option) (*MySQL, error) {
	o := buildOptions(opts)
	l := o.logger.With("component", "db", "driver", "mysql")
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

	gcfg := &gorm.Config{
		Logger: logger.New(slogWriter{logger: l}, logger.Config{
			SlowThreshold:             slow,
			LogLevel:                  parseLogLevel(cfg.LogLevel),
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		}),
	}
	l.Debug("mysql opening connection", "slow_threshold", slow.String(), "gorm_log_level", firstNonEmpty(cfg.LogLevel, "warn"))

	db, err := gorm.Open(mysql.Open(cfg.DSN), gcfg)
	if err != nil {
		if o.metrics != nil {
			o.metrics.ObserveDependency("mysql", "open", started, err)
		}
		l.Error("mysql open failed", "error", err)
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		if o.metrics != nil {
			o.metrics.ObserveDependency("mysql", "db", started, err)
		}
		l.Error("mysql sql db failed", "error", err)
		return nil, fmt.Errorf("mysql sql db: %w", err)
	}

	if err := applyPool(sqlDB, cfg); err != nil {
		_ = sqlDB.Close()
		l.Error("mysql pool config failed", "error", err)
		return nil, err
	}

	l.Debug("mysql ping started")
	if err := pingWithDeps(ctx, sqlDB, "mysql", started, o); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	if o.metrics != nil {
		o.metrics.ObserveDependency("mysql", "init", started, nil)
	}
	l.Info("mysql init completed", "elapsed", time.Since(started).String())
	return &MySQL{DB: db, SQLDB: sqlDB, logger: l, opts: o}, nil
}

type slogWriter struct{ logger *slog.Logger }

func (w slogWriter) Printf(format string, args ...any) {
	l := w.logger
	if l == nil {
		l = slog.Default()
	}
	l.Debug("gorm", "message", fmt.Sprintf(format, args...))
}

// GORM returns the underlying *gorm.DB.
func (m *MySQL) GORM() *gorm.DB { return m.DB }

// SQL returns the underlying *sql.DB.
func (m *MySQL) SQL() *sql.DB { return m.SQLDB }

// Driver returns "mysql".
func (m *MySQL) Driver() string { return "mysql" }

// Close releases the underlying connection pool.
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

// Ping runs a health-check.
func (m *MySQL) Ping(ctx context.Context) error {
	if m == nil || m.SQLDB == nil {
		return fmt.Errorf("mysql not initialized")
	}
	return pingWithDeps(ctx, m.SQLDB, "mysql", time.Now(), m.opts)
}

// Transaction wraps gorm.Transaction with a context-bound db.
func Transaction(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error) error {
	if db == nil {
		return fmt.Errorf("gorm db is nil")
	}
	return db.WithContext(ctx).Transaction(fn)
}

// parseLogLevel maps the string config to a gorm logger.LogLevel.
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

// firstNonEmpty returns the first non-empty argument, or "" if all are empty.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// Compile-time interface checks.
var (
	_ Database = (*MySQL)(nil)
	_ Database = (*Postgres)(nil)
)
