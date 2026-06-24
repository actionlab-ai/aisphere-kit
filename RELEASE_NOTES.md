# aisphere-kit v0.1.2-casdoor-cert-fix

This release hardens Casdoor JWT verification setup so bad public-key configuration fails during startup instead of on the first protected request.

## Key changes

- Added `casdoor.certificate_file` as an alternative to inline `casdoor.certificate`.
- Added `casdoor.Config.NormalizedCertificate()` to trim, normalize CRLF/literal `\n`, and load mounted certificate files.
- Added startup validation for `features.authn=true`: the Casdoor JWT public certificate/public key is now required and must be a PEM encoded RSA public certificate/key.
- Added `casdoor.NewChecked()` and switched `starter.NewRuntime()` to use it when Authn is enabled.
- Improved Casdoor adapter logs with `certificate_present` and `certificate_file` flags.
- Rejects private keys in service config; Hub and other components must only consume Casdoor's public certificate/public key.

## Config example

```yaml
features:
  authn: true

casdoor:
  endpoint: "http://localhost:18000"
  client_id: "aisphere-auth"
  client_secret: "change-me"
  organization: "aisphere"
  application: "aisphere"
  default_scope: "openid profile email"

  # Option A: inline public cert/key copied from Casdoor Cert page
  certificate: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----

  # Option B: mounted file. Inline certificate takes precedence if both are set.
  # certificate_file: "./certs/casdoor-jwt-public.pem"
```

## Verification

```bash
go mod tidy
go test ./...
```

Then restart the component and call a protected endpoint with a Casdoor access token.

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


## PostgreSQL DB baseline fixes

- Added `database.auto_create_database`, `database.maintenance_db`, and legacy `database.autoCreate` alias.
- Added `Runtime.Database` while keeping `Runtime.SQL` as a compatibility alias.
- Health checks now report the actual DB driver name.
- Added safe pagination allowlist support.
- Removed stale local ACL-table references from docs.
