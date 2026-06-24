# aisphere-kit

`aisphere-kit` is the framework-neutral SDK for AI Sphere components.

It integrates mature SDKs and libraries instead of reimplementing them:

- MySQL via GORM
- Redis single/cluster via go-redis
- MinIO via minio-go
- Casdoor authn/authz/audit via casdoor-go-sdk
- Casdoor/Casbin permission management via Casdoor policy APIs
- slog-based structured logging
- Prometheus metrics helpers

## Important boundary

`aisphere-kit` does **not** depend on Kratos. Kratos integration belongs in `github.com/actionlab-ai/aisphere-kit-kratos`.

`aisphere-kit` also does **not** keep a local ACL table as the permission authority. Sharing and resource grants are represented as Casdoor/Casbin policies through `permission.Manager`.

## Runtime

```go
cfg, rt, cleanup, err := starter.NewRuntimeFromConfig(ctx, []string{"configs/config.yaml"})
if err != nil {
    return err
}
defer cleanup()

// rt.DB
// rt.Redis
// rt.S3
// rt.Authn
// rt.Authz
// rt.Audit
// rt.Permission
```

## Permission grant example

```go
err := rt.Permission.Share(ctx, permission.ShareRequest{
    Resource:    resource.AIHubSkill(skillID),
    SubjectType: permission.SubjectUser,
    SubjectID:   userID,
    Role:        permission.RoleViewer,
    GrantedBy:   actor.SubjectID,
})
```

## Resource deletion cleanup

```go
err := rt.Permission.DeleteResourcePolicies(ctx, resource.AIHubSkill(skillID))
```

If cleanup fails, the business component should store an outbox job and retry. The business component must always check that the resource still exists before checking authorization.


## PostgreSQL

See [`docs/DB_POSTGRES.md`](docs/DB_POSTGRES.md) for PostgreSQL, auto-create database, JSONB, and safe pagination usage.
