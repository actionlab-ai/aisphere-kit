# PostgreSQL support

`aisphere-kit/db` supports both MySQL and PostgreSQL behind the same `db.Database` interface.

## Runtime fields

When `features.db=true`, `starter.Runtime` exposes:

```go
rt.DB        // *gorm.DB
rt.Database  // db.Database, preferred
rt.SQL       // db.Database, deprecated compatibility alias
rt.Tx        // transaction manager
```

## Config example

```yaml
features:
  db: true

database:
  driver: postgres
  dsn: postgres://postgres:postgres@127.0.0.1:5432/aisphere_hub?sslmode=disable
  auto_create_database: true
  maintenance_db: postgres
  max_open_conns: 50
  max_idle_conns: 10
  conn_max_lifetime: 1h
  slow_threshold: 500ms
  log_level: warn
```

`autoCreate` is also accepted as a legacy alias for older Hub configs.

## Auto-create database

When `auto_create_database=true`, the kit connects to `maintenance_db` first and creates the target database if it does not exist. This keeps Hub, Runtime, Sandbox, and other components from re-implementing PostgreSQL bootstrap logic.

## Search path

Prefer encoding schema selection in the DSN-level `options` runtime parameter. The kit does this automatically when `database.search_path` is set, instead of running `SET search_path` only once on a pooled connection.

```yaml
database:
  driver: postgres
  dsn: postgres://postgres:postgres@127.0.0.1:5432/aisphere_hub?sslmode=disable
  search_path: aihub
```

## JSONB

Use `db.JSONB[T]` for document-style payloads, such as the current Hub `aihub_document.payload` column.

```go
type Document[T any] struct {
    NamespaceID string     `gorm:"primaryKey"`
    Kind        string     `gorm:"primaryKey"`
    ID          string     `gorm:"primaryKey"`
    Payload     db.JSONB[T] `gorm:"type:jsonb"`
}
```

## Safe pagination

Do not pass raw HTTP sort strings into `PageRequest.Sort`. Use `PaginateSafe` with an allowlist:

```go
result, err := db.PaginateSafe[Skill](query, db.PageRequest{
    Page: 1,
    PageSize: 20,
    SortBy: "created_at",
    SortOrder: "desc",
}, map[string]string{
    "created_at": "created_at",
    "name": "name",
})
```
