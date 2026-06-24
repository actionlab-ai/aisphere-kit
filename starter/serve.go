package starter

import (
	"context"
	"os"
	"time"

	"github.com/actionlab-ai/aisphere-kit/config"
	"github.com/actionlab-ai/aisphere-kit/logx"
)

type RuntimeFactoryOptions struct {
	ConfigPaths    []string
	RuntimeOptions []RuntimeOption
}

func LoadConfig(paths []string) (*config.Config, error) {
	if len(paths) == 0 {
		paths = []string{envOr("AISPHERE_CONFIG", "configs/config.yaml")}
	}
	return config.Load(paths...)
}

func NewRuntimeFromConfig(ctx context.Context, paths []string, opts ...RuntimeOption) (*config.Config, *Runtime, func() error, error) {
	cfg, err := LoadConfig(paths)
	if err != nil {
		return nil, nil, nil, err
	}
	bootstrapLogger := logx.New(cfg.Log).With("app", cfg.App.Name, "env", cfg.App.Env)
	bootstrapLogger.Info("runtime config loaded", "paths", paths)
	rt, cleanup, err := NewRuntime(logx.NewContext(ctx, bootstrapLogger), cfg, opts...)
	if err != nil {
		bootstrapLogger.Error("runtime init failed", "error", err)
		return nil, nil, nil, err
	}
	bootstrapLogger.Info("runtime ready")
	return cfg, rt, cleanup, nil
}

func envOr(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

func CleanupWithTimeout(fn func() error, timeout time.Duration) error {
	return cleanupWithTimeout(fn, timeout)
}
func cleanupWithTimeout(fn func() error, timeout time.Duration) error {
	if fn == nil {
		return nil
	}
	ch := make(chan error, 1)
	go func() { ch <- fn() }()
	select {
	case err := <-ch:
		return err
	case <-time.After(timeout):
		return context.DeadlineExceeded
	}
}
