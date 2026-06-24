# Changelog

## v0.1.3-access-guard

- Added reusable `access.Guard` package.
- Added `starter.Runtime.Access` initialized from runtime Authz/Audit adapters.
- Preserved v0.1.2 Casdoor JWT certificate startup validation.

## v0.1.2-casdoor-cert-fix

- Added `casdoor.certificate_file`.
- Added startup validation for Casdoor JWT public certificate/public key when Authn is enabled.
- Added `casdoor.NewChecked()`.
