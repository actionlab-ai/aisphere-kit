package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// PostgresConfig is an alias for DatabaseConfig with postgres semantics.
type PostgresConfig = DatabaseConfig

// Postgres wraps a *gorm.DB opened against a Postgres backend.
type Postgres struct {
	DB     *gorm.DB
	SQLDB  *sql.DB
	logger *slog.Logger
	opts   options // carries metrics; logger kept separately for clarity
}

// NewPostgres opens a Postgres connection via gorm.io/driver/postgres
// (which itself uses jackc/pgx under the hood).
func NewPostgres(ctx context.Context, cfg PostgresConfig, opts ...Option) (*Postgres, error) {
	o := buildOptions(opts)
	l := o.logger.With("component", "db", "driver", "postgres")
	started := time.Now()

	l.Info("postgres init started", "max_open_conns", cfg.MaxOpenConns, "max_idle_conns", cfg.MaxIdleConns, "auto_create_database", cfg.AutoCreateDatabase || cfg.AutoCreate)
	if cfg.DSN == "" {
		err := fmt.Errorf("database.dsn is required")
		l.Error("postgres config invalid", "error", err)
		return nil, err
	}

	slow := 500 * time.Millisecond
	if cfg.SlowThreshold != "" {
		d, err := time.ParseDuration(cfg.SlowThreshold)
		if err != nil {
			l.Error("postgres slow threshold parse failed", "error", err, "value", cfg.SlowThreshold)
			return nil, fmt.Errorf("parse database.slow_threshold: %w", err)
		}
		if d > 0 {
			slow = d
		}
	}

	if cfg.AutoCreateDatabase || cfg.AutoCreate {
		if err := EnsurePostgresDatabase(ctx, cfg, opts...); err != nil {
			if o.metrics != nil {
				o.metrics.ObserveDependency("postgres", "ensure_database", started, err)
			}
			l.Error("postgres ensure database failed", "error", err)
			return nil, err
		}
	}

	dsn := normalizePostgresDSN(cfg.DSN, cfg.SSLMode, cfg.SearchPath)

	gcfg := &gorm.Config{
		Logger: logger.New(slogWriter{logger: l}, logger.Config{
			SlowThreshold:             slow,
			LogLevel:                  parseLogLevel(cfg.LogLevel),
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		}),
	}
	l.Debug("postgres opening connection", "slow_threshold", slow.String(), "gorm_log_level", firstNonEmpty(cfg.LogLevel, "warn"), "search_path", cfg.SearchPath)

	db, err := gorm.Open(postgres.Open(dsn), gcfg)
	if err != nil {
		if o.metrics != nil {
			o.metrics.ObserveDependency("postgres", "open", started, err)
		}
		l.Error("postgres open failed", "error", err)
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		if o.metrics != nil {
			o.metrics.ObserveDependency("postgres", "db", started, err)
		}
		l.Error("postgres sql db failed", "error", err)
		return nil, fmt.Errorf("postgres sql db: %w", err)
	}

	if err := applyPool(sqlDB, cfg); err != nil {
		_ = sqlDB.Close()
		l.Error("postgres pool config failed", "error", err)
		return nil, err
	}

	l.Debug("postgres ping started")
	if err := pingWithDeps(ctx, sqlDB, "postgres", started, o); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	if o.metrics != nil {
		o.metrics.ObserveDependency("postgres", "init", started, nil)
	}
	l.Info("postgres init completed", "elapsed", time.Since(started).String())
	return &Postgres{DB: db, SQLDB: sqlDB, logger: l, opts: o}, nil
}

// EnsurePostgresDatabase creates the configured target database if it does not
// already exist. It connects to cfg.MaintenanceDB, defaulting to "postgres".
func EnsurePostgresDatabase(ctx context.Context, cfg PostgresConfig, opts ...Option) error {
	o := buildOptions(opts)
	l := o.logger.With("component", "db", "driver", "postgres", "operation", "ensure_database")
	started := time.Now()

	targetDB, maintenanceDSN, err := postgresMaintenanceDSN(cfg)
	if err != nil {
		return err
	}
	if targetDB == "" {
		return fmt.Errorf("postgres target database name is required for auto_create_database")
	}

	l.Info("postgres ensure database started", "database", targetDB, "maintenance_db", firstNonEmpty(cfg.MaintenanceDB, "postgres"))
	db, err := gorm.Open(postgres.Open(normalizePostgresDSN(maintenanceDSN, cfg.SSLMode, "")), &gorm.Config{
		Logger: logger.New(slogWriter{logger: l}, logger.Config{
			SlowThreshold:             500 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		}),
	})
	if err != nil {
		return fmt.Errorf("open postgres maintenance database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("postgres maintenance sql db: %w", err)
	}
	defer sqlDB.Close()

	var exists bool
	err = sqlDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`, targetDB).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check postgres database existence: %w", err)
	}
	if exists {
		l.Debug("postgres database already exists", "database", targetDB)
		return nil
	}

	if _, err := sqlDB.ExecContext(ctx, `CREATE DATABASE `+quoteIdent(targetDB)); err != nil {
		return fmt.Errorf("create postgres database %q: %w", targetDB, err)
	}
	if o.metrics != nil {
		o.metrics.ObserveDependency("postgres", "create_database", started, nil)
	}
	l.Info("postgres database created", "database", targetDB, "elapsed", time.Since(started).String())
	return nil
}

// normalizePostgresDSN ensures sslmode is present and optionally encodes a
// search_path into the Postgres runtime options. Encoding the search_path in the
// DSN is preferred over running SET search_path once because it applies to every
// pooled connection.
func normalizePostgresDSN(dsn, sslMode, searchPath string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err == nil {
			q := u.Query()
			if q.Get("sslmode") == "" {
				mode := sslMode
				if mode == "" {
					mode = "disable"
				}
				q.Set("sslmode", mode)
			}
			if sp := strings.TrimSpace(searchPath); sp != "" && !strings.Contains(q.Get("options"), "search_path") {
				q.Set("options", strings.TrimSpace(q.Get("options")+" -c search_path="+sp))
			}
			u.RawQuery = q.Encode()
			return u.String()
		}
	}

	if !containsKeyword(dsn, "sslmode") {
		mode := sslMode
		if mode == "" {
			mode = "disable"
		}
		dsn = strings.TrimSpace(dsn) + " sslmode=" + mode
	}
	if sp := strings.TrimSpace(searchPath); sp != "" && !containsKeyword(dsn, "options") && !containsKeyword(dsn, "search_path") {
		dsn = strings.TrimSpace(dsn) + " options='-c search_path=" + escapeSingleQuoted(sp) + "'"
	}
	return dsn
}

func postgresMaintenanceDSN(cfg PostgresConfig) (targetDB string, maintenanceDSN string, err error) {
	maintenanceDB := strings.TrimSpace(cfg.MaintenanceDB)
	if maintenanceDB == "" {
		maintenanceDB = "postgres"
	}
	dsn := strings.TrimSpace(cfg.DSN)
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return "", "", fmt.Errorf("parse postgres dsn: %w", err)
		}
		target := strings.TrimPrefix(u.EscapedPath(), "/")
		if target == "" {
			return "", "", errors.New("postgres dsn path must include database name for auto_create_database")
		}
		dbName, err := url.PathUnescape(target)
		if err != nil {
			return "", "", fmt.Errorf("decode postgres database name: %w", err)
		}
		u.Path = "/" + url.PathEscape(maintenanceDB)
		return dbName, u.String(), nil
	}

	parts := splitKeywordDSN(dsn)
	target := parts["dbname"]
	if target == "" {
		return "", "", errors.New("postgres keyword dsn must include dbname for auto_create_database")
	}
	parts["dbname"] = maintenanceDB
	return target, joinKeywordDSN(parts), nil
}

func splitKeywordDSN(dsn string) map[string]string {
	parts := make(map[string]string)
	for _, f := range strings.Fields(dsn) {
		k, v, ok := strings.Cut(f, "=")
		if !ok || k == "" {
			continue
		}
		parts[k] = strings.Trim(v, "'")
	}
	return parts
}

func joinKeywordDSN(parts map[string]string) string {
	order := []string{"host", "port", "user", "password", "dbname", "sslmode", "connect_timeout", "timezone", "TimeZone"}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(parts))
	for _, k := range order {
		v, ok := parts[k]
		if !ok {
			continue
		}
		out = append(out, k+"="+quoteKeywordValue(v))
		seen[k] = struct{}{}
	}
	for k, v := range parts {
		if _, ok := seen[k]; ok {
			continue
		}
		out = append(out, k+"="+quoteKeywordValue(v))
	}
	return strings.Join(out, " ")
}

func quoteKeywordValue(v string) string {
	if v == "" || strings.ContainsAny(v, " \t'\\") {
		return "'" + strings.ReplaceAll(strings.ReplaceAll(v, `\`, `\\`), `'`, `\'`) + "'"
	}
	return v
}

func containsKeyword(dsn, key string) bool {
	return strings.Contains(dsn, key+"=")
}

func escapeSingleQuoted(s string) string {
	return strings.ReplaceAll(s, `'`, `''`)
}

// quoteIdent wraps an identifier in double quotes, escaping inner quotes.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// GORM returns the underlying *gorm.DB.
func (p *Postgres) GORM() *gorm.DB { return p.DB }

// SQL returns the underlying *sql.DB.
func (p *Postgres) SQL() *sql.DB { return p.SQLDB }

// Driver returns "postgres".
func (p *Postgres) Driver() string { return "postgres" }

// Close releases the underlying connection pool.
func (p *Postgres) Close() error {
	if p == nil || p.SQLDB == nil {
		return nil
	}
	started := time.Now()
	err := p.SQLDB.Close()
	if p.logger != nil {
		if err != nil {
			p.logger.Warn("postgres close failed", "error", err)
		} else {
			p.logger.Info("postgres closed", "elapsed", time.Since(started).String())
		}
	}
	return err
}

// Ping runs a health-check.
func (p *Postgres) Ping(ctx context.Context) error {
	if p == nil || p.SQLDB == nil {
		return fmt.Errorf("postgres not initialized")
	}
	return pingWithDeps(ctx, p.SQLDB, "postgres", time.Now(), p.opts)
}
