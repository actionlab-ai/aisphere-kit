# Logging

`aisphere-kit` uses the standard library `log/slog` only.

## Log levels

- debug: dependency init details, health check success, middleware decisions, MinIO object operations, Redis/MySQL ping success.
- info: runtime startup, dependency initialized, object uploaded/copied/deleted, authentication success.
- warn: authz denied, dependency health failure, audit queue full, non-fatal close/record failures.
- error: invalid config, dependency init failure, runtime initialization failure.

## Context logger

Use:

```go
ctx = logx.NewContext(ctx, logger)
logger := logx.FromContext(ctx)
```

The Kratos adapter attaches request-scoped attributes such as middleware name and request ID when available.
