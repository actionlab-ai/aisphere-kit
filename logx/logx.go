package logx

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Level     string `json:"level" yaml:"level"`
	Format    string `json:"format" yaml:"format"`
	AddSource bool   `json:"add_source" yaml:"add_source"`
}

type contextKey struct{}

type RequestIDKey struct{}

func New(cfg Config) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(cfg.Level)) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level, AddSource: cfg.AddSource}
	if strings.EqualFold(cfg.Format, "text") {
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}

func NewContext(ctx context.Context, logger *slog.Logger) context.Context {
	if logger == nil {
		logger = slog.Default()
	}
	return context.WithValue(ctx, contextKey{}, logger)
}

func With(ctx context.Context, attrs ...any) context.Context {
	return NewContext(ctx, FromContext(ctx).With(attrs...))
}

func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if l, ok := ctx.Value(contextKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	ctx = context.WithValue(ctx, RequestIDKey{}, requestID)
	return With(ctx, "request_id", requestID)
}

func RequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(RequestIDKey{}).(string); ok {
		return v
	}
	return ""
}
