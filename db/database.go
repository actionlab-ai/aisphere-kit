package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/actionlab-ai/aisphere-kit/metrics"
	"gorm.io/gorm"
)

// Database is the backend-agnostic interface every concrete driver
// (MySQL, Postgres, ...) must satisfy. Runtime holds this interface and
// downstream code should depend on it, not on *MySQL or *Postgres directly.
type Database interface {
	// GORM returns the underlying *gorm.DB for query building.
	GORM() *gorm.DB
	// SQL returns the underlying *sql.DB for ping/close/pool control.
	SQL() *sql.DB
	// Driver returns the configured driver name, e.g. "mysql" or "postgres".
	Driver() string
	// Ping runs a health-check against the underlying connection.
	Ping(ctx context.Context) error
	// Close releases the underlying connection pool.
	Close() error
}

// DatabaseConfig is the backend-agnostic configuration. It is field-compatible
// with the legacy MySQLConfig so existing YAML/env still parses; new fields are
// added for postgres-specific options.
//
// Driver selects the underlying gorm dialect. Supported values:
//   - "" / "mysql" -> gorm.io/driver/mysql
//   - "postgres" / "postgresql" / "pgx" -> gorm.io/driver/postgres
//
// The DSN format follows the chosen driver's convention:
//   - mysql:    "user:pass@tcp(host:3306)/dbname?charset=utf8mb4&parseTime=True"
//   - postgres: "host=localhost user=postgres password=postgres dbname=aihub port=5432 sslmode=disable"
//     or a URL: "postgres://user:pass@host:5432/dbname?sslmode=disable"
type DatabaseConfig struct {
	Driver          string `json:"driver" yaml:"driver"`
	DSN             string `json:"dsn" yaml:"dsn"`
	MaxOpenConns    int    `json:"max_open_conns" yaml:"max_open_conns"`
	MaxIdleConns    int    `json:"max_idle_conns" yaml:"max_idle_conns"`
	ConnMaxLifetime string `json:"conn_max_lifetime" yaml:"conn_max_lifetime"`
	SlowThreshold   string `json:"slow_threshold" yaml:"slow_threshold"`
	LogLevel        string `json:"log_level" yaml:"log_level"`

	// AutoCreateDatabase creates the target PostgreSQL database when it does not
	// exist. It is intentionally implemented only for Postgres because MySQL DSN
	// permissions and creation semantics vary widely between deployments.
	AutoCreateDatabase bool `json:"auto_create_database" yaml:"auto_create_database"`
	// AutoCreate is a legacy alias used by older Hub configs.
	AutoCreate bool `json:"autoCreate" yaml:"autoCreate"`
	// MaintenanceDB is the database used to run CREATE DATABASE for Postgres.
	// Defaults to "postgres".
	MaintenanceDB string `json:"maintenance_db" yaml:"maintenance_db"`

	// Postgres-only options. Ignored when Driver != postgres.
	// SearchPath is encoded into the connection DSN via the Postgres "options"
	// runtime parameter so every pooled connection receives the same setting.
	// Leave empty to use the server/default schema resolution, usually public.
	SearchPath string `json:"search_path" yaml:"search_path"`
	// SSLMode overrides the sslmode query-param. Only applied when the DSN does
	// not already contain sslmode. Common values: disable, require, verify-ca,
	// verify-full. Defaults to "disable" for dev convenience.
	SSLMode string `json:"ssl_mode" yaml:"ssl_mode"`
}

// Normalize validates the driver name and returns the canonical form.
func (c DatabaseConfig) Normalize() (string, error) {
	switch canon := canonicalDriver(c.Driver); canon {
	case "mysql", "postgres":
		return canon, nil
	default:
		return "", fmt.Errorf("unsupported database.driver %q (supported: mysql, postgres)", c.Driver)
	}
}

// Options shared by every driver constructor.
type options struct {
	logger  *slog.Logger
	metrics *metrics.Metrics
}

// Option configures a Database constructor.
type Option func(*options)

// WithLogger attaches a slog logger.
func WithLogger(l *slog.Logger) Option { return func(o *options) { o.logger = l } }

// WithMetrics attaches a metrics recorder for dependency observations.
func WithMetrics(m *metrics.Metrics) Option { return func(o *options) { o.metrics = m } }

func buildOptions(opts []Option) options {
	var o options
	for _, fn := range opts {
		if fn != nil {
			fn(&o)
		}
	}
	if o.logger == nil {
		o.logger = slog.Default()
	}
	return o
}

// NewDatabase is the backend-agnostic factory. It dispatches to NewMySQL or
// NewPostgres based on cfg.Driver and returns a Database interface.
func NewDatabase(ctx context.Context, cfg DatabaseConfig, opts ...Option) (Database, error) {
	driver, err := cfg.Normalize()
	if err != nil {
		return nil, err
	}
	switch driver {
	case "mysql":
		return NewMySQL(ctx, MySQLConfig(cfg), opts...)
	case "postgres":
		return NewPostgres(ctx, PostgresConfig(cfg), opts...)
	default:
		return nil, fmt.Errorf("unsupported database.driver %q", cfg.Driver)
	}
}

// canonicalDriver normalizes the driver string.
//   - "" defaults to "mysql" (legacy behavior).
//   - "postgres", "postgresql", "pgx" -> "postgres".
func canonicalDriver(s string) string {
	switch s {
	case "", "mysql":
		return "mysql"
	case "postgres", "postgresql", "pgx":
		return "postgres"
	default:
		return s
	}
}

// applyPool configures connection-pool knobs shared by both drivers.
func applyPool(sqlDB *sql.DB, cfg DatabaseConfig) error {
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != "" {
		d, err := time.ParseDuration(cfg.ConnMaxLifetime)
		if err != nil {
			return fmt.Errorf("parse database.conn_max_lifetime: %w", err)
		}
		sqlDB.SetConnMaxLifetime(d)
	}
	return nil
}

// pingWithDeps runs PingContext with logger + metrics observation.
func pingWithDeps(ctx context.Context, sqlDB *sql.DB, driver string, started time.Time, opt options) error {
	err := sqlDB.PingContext(ctx)
	if opt.metrics != nil {
		opt.metrics.ObserveDependency(driver, "ping", started, err)
	}
	if err != nil {
		opt.logger.Error("db health check failed", "driver", driver, "error", err)
		return err
	}
	opt.logger.Debug("db health check ok", "driver", driver, "elapsed", time.Since(started).String())
	return nil
}
