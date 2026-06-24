package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/actionlab-ai/aisphere-kit/cache"
	"github.com/actionlab-ai/aisphere-kit/casdoor"
	"github.com/actionlab-ai/aisphere-kit/db"
	"github.com/actionlab-ai/aisphere-kit/logx"
	"github.com/actionlab-ai/aisphere-kit/metrics"
	"github.com/actionlab-ai/aisphere-kit/objectstore"
	"github.com/actionlab-ai/aisphere-kit/session"
	"gopkg.in/yaml.v3"
)

type Config struct {
	App         AppConfig               `json:"app" yaml:"app"`
	Server      ServerConfig            `json:"server" yaml:"server"`
	Features    FeatureConfig           `json:"features" yaml:"features"`
	Log         logx.Config             `json:"log" yaml:"log"`
	Database    db.DatabaseConfig       `json:"database" yaml:"database"`
	Redis       cache.RedisConfig       `json:"redis" yaml:"redis"`
	ObjectStore objectstore.MinIOConfig `json:"objectstore" yaml:"objectstore"`
	Casdoor     casdoor.Config          `json:"casdoor" yaml:"casdoor"`
	Metrics     metrics.Config          `json:"metrics" yaml:"metrics"`
	Session     session.Config          `json:"session" yaml:"session"`
	Health      HealthConfig            `json:"health" yaml:"health"`
	Shutdown    ShutdownConfig          `json:"shutdown" yaml:"shutdown"`
}

type AppConfig struct {
	Name    string `json:"name" yaml:"name"`
	Env     string `json:"env" yaml:"env"`
	Version string `json:"version" yaml:"version"`
}
type ServerConfig struct {
	HTTP EndpointConfig `json:"http" yaml:"http"`
	GRPC EndpointConfig `json:"grpc" yaml:"grpc"`
}
type EndpointConfig struct {
	Addr    string `json:"addr" yaml:"addr"`
	Timeout string `json:"timeout" yaml:"timeout"`
}
type FeatureConfig struct {
	DB      bool `json:"db" yaml:"db"`
	Cache   bool `json:"cache" yaml:"cache"`
	S3      bool `json:"s3" yaml:"s3"`
	Authn   bool `json:"authn" yaml:"authn"`
	Authz   bool `json:"authz" yaml:"authz"`
	Audit   bool `json:"audit" yaml:"audit"`
	Metrics bool `json:"metrics" yaml:"metrics"`
	Tracing bool `json:"tracing" yaml:"tracing"`
	Session bool `json:"session" yaml:"session"`
	// Permission enables Casdoor/Casbin policy management through permission.Manager.
	Permission bool `json:"permission" yaml:"permission"`
	// Sharing is deprecated. Sharing/ACL must be represented as Casdoor/Casbin policies via Permission.
	Sharing bool `json:"sharing" yaml:"sharing"`
}
type HealthConfig struct {
	Timeout string `json:"timeout" yaml:"timeout"`
}
type ShutdownConfig struct {
	Timeout string `json:"timeout" yaml:"timeout"`
}

func Default() *Config {
	return &Config{
		App:         AppConfig{Env: "dev", Version: "dev"},
		Server:      ServerConfig{HTTP: EndpointConfig{Addr: "0.0.0.0:8000", Timeout: "10s"}, GRPC: EndpointConfig{Addr: "0.0.0.0:9000", Timeout: "10s"}},
		Features:    FeatureConfig{DB: true, Cache: true, S3: true, Authn: true, Authz: true, Audit: true, Metrics: true, Tracing: true, Session: true, Permission: true, Sharing: false},
		Log:         logx.Config{Level: "info", Format: "json"},
		Database:    db.DatabaseConfig{Driver: "mysql", MaxOpenConns: 50, MaxIdleConns: 10, ConnMaxLifetime: "1h", SlowThreshold: "500ms", LogLevel: "warn", MaintenanceDB: "postgres"},
		Redis:       cache.RedisConfig{Mode: "single", Addr: "127.0.0.1:6379", DialTimeout: "5s", ReadTimeout: "3s", WriteTimeout: "3s"},
		ObjectStore: objectstore.MinIOConfig{Provider: "minio", Endpoint: "127.0.0.1:9000", Bucket: "aisphere", UseSSL: false},
		Metrics:     metrics.Config{Namespace: "aisphere"},
		Session:     session.Config{KeyPrefix: "aisphere:session", StateTTL: "10m"},
		Health:      HealthConfig{Timeout: "2s"},
		Shutdown:    ShutdownConfig{Timeout: "15s"},
	}
}

func Load(paths ...string) (*Config, error) {
	merged := map[string]any{}
	base, _ := yaml.Marshal(Default())
	_ = yaml.Unmarshal(base, &merged)
	for _, path := range paths {
		if path == "" {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("load config %s: %w", path, err)
		}
		current := map[string]any{}
		if err := yaml.Unmarshal(b, &current); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", path, err)
		}
		mergeMap(merged, current)
	}
	applyEnv(merged, "AISPHERE_")
	b, _ := json.Marshal(merged)
	cfg := &Config{}
	if err := json.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if cfg.Casdoor.HTTPTimeout == "" {
		cfg.Casdoor.HTTPTimeout = "10s"
	}
	if cfg.Casdoor.DefaultScope == "" {
		cfg.Casdoor.DefaultScope = "openid profile email"
	}
	return cfg, cfg.Validate()
}

func mergeMap(dst, src map[string]any) {
	for k, v := range src {
		sm, so := v.(map[string]any)
		dm, do := dst[k].(map[string]any)
		if so && do {
			mergeMap(dm, sm)
			continue
		}
		dst[k] = v
	}
}

func applyEnv(root map[string]any, prefix string) {
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, prefix) {
			continue
		}
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		path := strings.ToLower(strings.TrimPrefix(parts[0], prefix))
		keys := strings.Split(path, "__")
		setNested(root, keys, parts[1])
	}
}

func setNested(m map[string]any, keys []string, v any) {
	if len(keys) == 0 {
		return
	}
	if len(keys) == 1 {
		m[keys[0]] = v
		return
	}
	next, ok := m[keys[0]].(map[string]any)
	if !ok {
		next = map[string]any{}
		m[keys[0]] = next
	}
	setNested(next, keys[1:], v)
}

func (c *Config) Validate() error {
	var errs []error
	if c.App.Name == "" {
		errs = append(errs, fmt.Errorf("app.name is required"))
	}
	if c.Server.HTTP.Timeout != "" {
		if _, err := time.ParseDuration(c.Server.HTTP.Timeout); err != nil {
			errs = append(errs, fmt.Errorf("server.http.timeout: %w", err))
		}
	}
	if c.Server.GRPC.Timeout != "" {
		if _, err := time.ParseDuration(c.Server.GRPC.Timeout); err != nil {
			errs = append(errs, fmt.Errorf("server.grpc.timeout: %w", err))
		}
	}
	if c.Features.DB {
		if _, err := c.Database.Normalize(); err != nil {
			errs = append(errs, err)
		}
		if c.Database.DSN == "" {
			errs = append(errs, fmt.Errorf("database.dsn is required"))
		}
	}
	if c.Features.Cache {
		switch c.Redis.Mode {
		case "", "single":
			if c.Redis.Addr == "" {
				errs = append(errs, fmt.Errorf("redis.addr is required"))
			}
		case "cluster":
			if len(c.Redis.Addrs) == 0 {
				errs = append(errs, fmt.Errorf("redis.addrs is required"))
			}
		default:
			errs = append(errs, fmt.Errorf("redis.mode only supports single or cluster"))
		}
	}
	if c.Features.S3 {
		if c.ObjectStore.Provider != "" && c.ObjectStore.Provider != "minio" {
			errs = append(errs, fmt.Errorf("objectstore.provider only supports minio"))
		}
		if c.ObjectStore.Endpoint == "" || c.ObjectStore.Bucket == "" {
			errs = append(errs, fmt.Errorf("objectstore.endpoint and bucket are required"))
		}
	}
	if c.Features.Session && !c.Features.Cache {
		errs = append(errs, fmt.Errorf("features.session requires features.cache"))
	}
	if c.Features.Sharing {
		errs = append(errs, fmt.Errorf("features.sharing is deprecated; use features.permission with Casdoor/Casbin"))
	}
	if c.Features.Authn || c.Features.Authz || c.Features.Audit || c.Features.Permission {
		if c.Casdoor.Endpoint == "" || c.Casdoor.ClientID == "" || c.Casdoor.ClientSecret == "" || c.Casdoor.Organization == "" || c.Casdoor.Application == "" {
			errs = append(errs, fmt.Errorf("casdoor endpoint/client_id/client_secret/organization/application are required"))
		}
	}
	if c.Features.Authn {
		if _, err := c.Casdoor.NormalizedCertificate(); err != nil {
			errs = append(errs, fmt.Errorf("casdoor jwt certificate: %w", err))
		}
	}
	if c.Features.Authz {
		selectors := 0
		for _, v := range []string{c.Casdoor.PermissionID, c.Casdoor.ModelID, c.Casdoor.ResourceID, c.Casdoor.EnforcerID, c.Casdoor.Owner} {
			if v != "" {
				selectors++
			}
		}
		if selectors == 0 {
			errs = append(errs, fmt.Errorf("one of casdoor permission_id/model_id/resource_id/enforcer_id/owner is required for authz"))
		}
	}

	if c.Features.Permission {
		if c.Casdoor.PolicyEnforcer == "" && c.Casdoor.EnforcerID == "" {
			errs = append(errs, fmt.Errorf("casdoor.policy_enforcer or casdoor.enforcer_id is required for permission management"))
		}
	}

	return errors.Join(errs...)
}

func ParseDurationOr(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return fallback
	}
	return d
}
