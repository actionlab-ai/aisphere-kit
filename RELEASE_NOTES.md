# aisphere-kit v0.7.0

This release removes local sharing as an authorization source and moves resource sharing / permission mutation to Casdoor + Casbin.

## Key changes

- Added `permission.Manager` as the platform permission facade.
- Added `permission.Grant`, `ShareRequest`, `CheckRequest`, `DeleteResourceRequest`.
- Added role expansion helpers for `viewer`, `editor`, and `owner`.
- Added Casdoor-backed policy management in `casdoor.Adapter` using Casdoor SDK policy APIs:
  - `AddPolicy`
  - `RemovePolicy`
  - `GetPolicies`
  - `Enforce`
- Added `DeleteResourcePolicies` for resource lifecycle cleanup when a resource such as `aihub:skill:skill-a` is deleted.
- Updated `starter.Runtime` to expose `Permission permission.Manager`.
- Removed local `sharing.Store` from runtime assembly. `features.sharing` is now deprecated and validation returns an error.
- Added `docs/PERMISSION_LIFECYCLE.md`.

## Design boundary

- Casdoor/Casbin is the permission authority.
- Business components own resource lifecycle.
- Local component DB tables may store share link metadata, invitations, and outbox retry jobs, but not final ACL decisions.

## Local verification

The sandbox environment cannot fetch external modules, so full `go test ./...` must be run locally after `go mod tidy`.

```bash
go mod tidy
go test ./...
```
