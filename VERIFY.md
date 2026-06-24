# Verify aisphere-kit

```bash
go mod tidy
go test ./...
go vet ./...
go build ./...
```

Kratos v3 is currently published as an untagged v3 module snapshot on pkg.go.dev. If Go cannot resolve the pseudo-version pinned in `go.mod`, run:

```bash
go get github.com/go-kratos/kratos/v3@latest
go mod tidy
```

The package intentionally keeps Kratos lifecycle, transport and middleware as the service skeleton. `kratosx` only wires Aisphere defaults around Kratos; it does not replace Kratos.


## PostgreSQL DB checks

Run locally with module download access:

```bash
go mod tidy
go test ./...
```

This package includes unit coverage for driver normalization, PostgreSQL DSN normalization, maintenance DSN derivation, JSONB scan/value behavior, and framework-independent permission/resource/auth packages.
