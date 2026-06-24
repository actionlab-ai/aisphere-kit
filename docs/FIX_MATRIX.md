# Fix matrix for v0.3.0

## P0

1. `db/mysql.go`: slow threshold now feeds GORM logger config instead of being discarded.
2. `kratosx/server.go`: HTTP/gRPC timeout config is wired into Kratos server options.
3. `health/health.go`: readiness uses `BucketExists`, never `EnsureBucket`.
4. `objectstore`: `GetOptions` now supports ranged reads; MinIO implementation applies it.
5. `authz`: middleware and helper are deny-by-default for missing resolver/resource/action.
6. `casdoor`: calls are wrapped with caller context and retry policy; no per-request package-level client mutation.
7. `principal`: `RequireFromContext` returns error; `MustFromContext` panics; zero principal is unauthenticated.
8. `metrics`: registration errors are handled; duplicate registration is accepted explicitly.
9. `audit`: async audit is bounded, preserves context values with timeout, logs errors, and separates operation from URI.
10. `casdoor`: audit event is serialized into Casdoor `Record.Object`, with status/message/action/resource context preserved.

## P1

- Added tests for principal/authn/authz/cache lock/cleanup/resource.
- Added GitHub Actions CI with lint/test/build matrix.
- Added slog logging and request audit logs.
- Added tracing middleware wiring through Kratos when enabled.
- Added dependency metric collectors.
- Added Redis client interface alias and lock helpers.
- Added retry config for Casdoor calls.
- Added cleanup timeout and ordered cleanup stack.
- Added env override via `AISPHERE_*`.
- Added `/livez` and `/readyz` HTTP handlers through `kratosx`.

## P2

- Added functional options to DB starter.
- Added stronger objectstore facade: list/copy/stat/ranged read/presign.
- Added lock wait/refresh/token ownership checks.
- Added real example module under `examples/kratos-service`.
- Added `CHANGELOG.md`, `VERIFY.md`, CI and Makefile targets.
- Added `version` package using build info.
- Changed resource helpers to return typed `resource.Name`.
