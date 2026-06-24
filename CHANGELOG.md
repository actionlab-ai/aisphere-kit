# Changelog

## v0.1.2-casdoor-cert-fix

- Added `casdoor.certificate_file` and certificate normalization for Casdoor JWT verification.
- Added fail-fast startup validation when `features.authn=true`; missing/invalid Casdoor public certificate now stops startup.
- Added `casdoor.NewChecked()` and wired starter runtime to use it for Authn.
- Improved Casdoor adapter startup logs and rejected private-key configuration.


## v0.6.0

- Added scenario-driven APIs for login/logout URL construction, OAuth state storage, token blacklist, DB transaction manager, SQL exec helper, pagination helper, Redis JSON cache helpers, key builder, and Casdoor/Casbin permission grants.
- Runtime now exposes `Authn`, `Authz`, `Audit`, `Session`, `Tx`, `Database`, and `Permission` so components do not reassemble common dependencies.
- Added `docs/SCENARIO_COVERAGE.md` to map business scenarios to packages.

## v0.3.0

- Hardened authn/authz/audit middleware with deny-by-default authz behavior.
- Added slog-based logging, health endpoints, Kratos timeout wiring, and tracing middleware wiring.
- Fixed MySQL slow-query threshold wiring and added context-aware init checks.
- Added Redis single/cluster client starter and safer lock helpers with ownership checks.
- Added MinIO bucket existence checks, object listing/copy, ranged reads, and presigned URLs.
- Added Prometheus dependency metrics and duplicate registration handling.
- Added table-driven tests, GitHub Actions CI, lint config, Makefile targets, and verification docs.

## v0.2.0

- Split authn/authz/audit/objectstore/resource/action/starter packages.

## v0.5.1

- Fix Casdoor SDK v1.46 Enforce signature compatibility.
- Normalize Casdoor authz selector handling: permission_id > model_id > resource_id > enforcer_id > owner.
- Fix MinIO ObjectInfo mapping for ObjectInfo values that do not expose Bucket.
