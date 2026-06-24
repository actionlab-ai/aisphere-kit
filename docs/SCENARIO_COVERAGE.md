# AI Sphere Kit Scenario Coverage

This document maps common AI Sphere component scenarios to packages in `github.com/actionlab-ai/aisphere-kit`.

## Identity and access

| Scenario | Package | Provided API |
|---|---|---|
| Browser login redirect | `authn`, `casdoor` | `casdoor.Adapter.LoginURL(authn.LoginRequest)` |
| Browser logout redirect | `authn`, `casdoor` | `casdoor.Adapter.LogoutURL(authn.LogoutRequest)` |
| Bearer token authentication | `authn`, `casdoor` | `authn.Authenticator`, `casdoor.Adapter.Authenticate` |
| Principal propagation | `principal` | `NewContext`, `RequireFromContext`, `MustFromContext` |
| Token logout / blacklist | `session` | `RevokeToken`, `IsRevoked` backed by Redis |
| OAuth state anti-CSRF | `session` | `NewState`, `VerifyState` backed by Redis |
| Permission check | `authz`, `casdoor` | `authz.Require`, `casdoor.Adapter.Authorize` |
| Resource sharing / ACL | `permission` + `casdoor` | `Grant`, `GrantRole`, `Share`, `Revoke`, `Check`, `DeleteResourcePolicies` |
| Permission authority | `permission.Manager` implemented by `casdoor.Adapter` | policies are stored and enforced in Casdoor/Casbin, not local tables |
| Audit record | `audit`, `casdoor` | `audit.Record`, `casdoor.Adapter.Record` |

## Data and cache

| Scenario | Package | Provided API |
|---|---|---|
| MySQL connection pool | `db` | `NewMySQL`, pool config, ping, close |
| GORM access | `starter` / `db` | `rt.DB`, `rt.DBFromContext(ctx)` |
| SQL exec | `db` | `Exec(ctx, db, sql, args...)` |
| Transaction | `db`, `starter` | `TxManager.WithTx`, `rt.WithTx` |
| Transaction propagation | `db` | `NewTxContext`, `TxFromContext` |
| Pagination | `db` | `Paginate[T]`, `PageRequest`, `PageResult[T]` |
| Redis single/cluster | `cache` | `NewRedis`, `rt.Redis` |
| Redis JSON cache | `cache` | `SetJSON`, `GetJSON`, `RememberJSON` |
| Redis key building | `cache` | `Key(parts...)` |
| Distributed lock | `cache` | `TryLock`, lock ownership check, unlock |

## Object storage

| Scenario | Package | Provided API |
|---|---|---|
| MinIO client | `objectstore` | `NewMinIO`, `rt.S3` |
| Upload/download/delete | `objectstore` | `PutObject`, `GetObject`, `DeleteObject` |
| Metadata/stat/list/copy | `objectstore` | `StatObject`, `ListObjects`, `CopyObject` |
| Presigned upload/download | `objectstore` | `PresignPut`, `PresignGet` |
| Bucket readiness | `objectstore`, `health` | `BucketExists`; health does not create buckets |

## Runtime composition

| Scenario | Package | Provided API |
|---|---|---|
| Load config | `config` | `Load(paths...)`, env override `AISPHERE_*` |
| Build component runtime | `starter` | `NewRuntime`, `NewRuntimeFromConfig` |
| Unified resources | `starter.Runtime` | `DB`, `Redis`, `S3`, `Authn`, `Authz`, `Audit`, `Session`, `Shares` |
| Health check | `health` | readiness/liveness checks for initialized dependencies |
| Logging | `logx` | `slog` JSON/text handlers, context logger |
| Metrics | `metrics` | dependency latency/errors and request counters/duration |

## What remains component-specific

The kit intentionally does not implement Hub/Runtime/Sandbox business tables or controllers. Components still own:

- domain models like Skill, Agent, Workflow, RuntimeSession, SandboxInstance
- business validation
- HTTP/gRPC/proto definitions
- usecase orchestration
- migrations beyond the common `resource_shares` table
- component-specific cache keys and object storage prefixes
