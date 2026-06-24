# aisphere-kit v0.1.3-access-guard

This release keeps the Casdoor certificate startup validation from v0.1.2 and adds a reusable `access.Guard` facade for all AI Sphere services.

## Key changes

- Added `access` package.
- Added `starter.Runtime.Access *access.Guard`.
- Centralized common usecase operations:
  - `Require(ctx, access.Check)` for authz.
  - `Can(ctx, access.Check)` for non-failing permission checks.
  - `Record(ctx, access.Event)` for audit.
  - `RequireAndAudit(ctx, access.Check, access.Event)` for simple guarded operations.
- `access.Guard` composes existing `authz.Authorizer`, `audit.Recorder`, and `principal` context.
- Business modules no longer need to implement per-service access wrappers.

## Recommended use in services

```go
p, err := rt.Access.Require(ctx, access.Check{
    Resource: "aihub:admin",
    Action:   "admin.access",
})

_ = rt.Access.Record(ctx, access.Event{
    Action:   "skill.delete",
    Resource: "aihub:skill:demo",
    Result:   access.ResultSuccess,
})
```

## Config reminder

When `features.authn=true`, Casdoor JWT public certificate configuration is validated at startup:

```yaml
casdoor:
  certificate: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
  # or
  certificate_file: ./certs/casdoor-jwt-public.pem
```
