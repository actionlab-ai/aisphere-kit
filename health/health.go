package health

import (
	"context"
	"fmt"
	"time"

	"github.com/actionlab-ai/aisphere-kit/config"
	"github.com/actionlab-ai/aisphere-kit/logx"
	"github.com/actionlab-ai/aisphere-kit/starter"
)

type Result struct {
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
	Duration string `json:"duration,omitempty"`
}

type Checker interface {
	Ping(ctx context.Context) error
}

func Check(ctx context.Context, rt *starter.Runtime) []Result {
	if rt == nil {
		return []Result{{Name: "runtime", OK: false, Error: "runtime is nil"}}
	}
	timeout := config.ParseDurationOr(rt.Config.Health.Timeout, 2*time.Second)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	logx.FromContext(ctx).Debug("readiness check started", "timeout", timeout.String())
	var out []Result
	if rt.SQL != nil {
		name := "database"
		if rt.Database != nil && rt.Database.Driver() != "" {
			name = rt.Database.Driver()
		} else if rt.SQL != nil && rt.SQL.Driver() != "" {
			name = rt.SQL.Driver()
		}
		out = append(out, result(name, func() error { return rt.SQL.Ping(ctx) }))
	}
	if rt.RedisRuntime != nil {
		out = append(out, result("redis", func() error { return rt.RedisRuntime.Ping(ctx) }))
	}
	if rt.S3 != nil {
		out = append(out, result("objectstore", func() error { _, err := rt.S3.BucketExists(ctx); return err }))
	}
	if rt.Casdoor != nil {
		out = append(out, result("casdoor", func() error { return rt.Casdoor.Ping(ctx) }))
	}
	if err := IsHealthy(out); err != nil {
		logx.FromContext(ctx).Warn("readiness check failed", "error", err)
	} else {
		logx.FromContext(ctx).Debug("readiness check passed", "checks", len(out))
	}
	return out
}

func Live(ctx context.Context, rt *starter.Runtime) []Result {
	if rt == nil {
		return []Result{{Name: "runtime", OK: false, Error: "runtime is nil"}}
	}
	logx.FromContext(ctx).Debug("liveness check passed")
	return []Result{{Name: "runtime", OK: true}}
}

func result(name string, fn func() error) Result {
	started := time.Now()
	err := fn()
	d := time.Since(started)
	if err != nil {
		return Result{Name: name, OK: false, Error: err.Error(), Duration: d.String()}
	}
	return Result{Name: name, OK: true, Duration: d.String()}
}
func IsHealthy(results []Result) error {
	for _, r := range results {
		if !r.OK {
			return fmt.Errorf("%s unhealthy: %s", r.Name, r.Error)
		}
	}
	return nil
}
