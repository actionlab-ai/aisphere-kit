# Configuration

Use YAML for local development and environment variables for secrets in production. Environment overrides use `AISPHERE_` and `__` as nested separators, for example `AISPHERE_DATABASE__DSN`.

```yaml
app:
  name: aisphere-hub
  env: dev
  version: dev

server:
  http:
    addr: 0.0.0.0:8101
    timeout: 10s
  grpc:
    addr: 0.0.0.0:9101
    timeout: 10s

features:
  db: true
  cache: true
  s3: true
  authn: true
  authz: true
  audit: true
  metrics: true
  tracing: true

log:
  level: info
  format: json

database:
  driver: mysql
  dsn: ${AISPHERE_DATABASE__DSN}
  max_open_conns: 50
  max_idle_conns: 10
  conn_max_lifetime: 1h
  slow_threshold: 500ms
  log_level: warn

redis:
  mode: single
  addr: ${AISPHERE_REDIS__ADDR}
  password: ${AISPHERE_REDIS__PASSWORD}
  db: 0

objectstore:
  provider: minio
  endpoint: ${AISPHERE_OBJECTSTORE__ENDPOINT}
  access_key: ${AISPHERE_OBJECTSTORE__ACCESS_KEY}
  secret_key: ${AISPHERE_OBJECTSTORE__SECRET_KEY}
  bucket: aisphere-hub
  use_ssl: false
  auto_create_bucket: false

casdoor:
  endpoint: ${AISPHERE_CASDOOR__ENDPOINT}
  client_id: ${AISPHERE_CASDOOR__CLIENT_ID}
  client_secret: ${AISPHERE_CASDOOR__CLIENT_SECRET}
  certificate: ${AISPHERE_CASDOOR__CERTIFICATE}
  organization: aisphere
  application: aisphere-hub
  permission_id: aisphere/hub-permission
  model_id: aisphere/hub-model
  resource_id: aisphere/hub-resource
  allow_anonymous: false
  audit_async: true
  audit_queue: 1024
  audit_timeout: 5s
  retry_attempts: 2
  retry_backoff: 100ms

metrics:
  namespace: aisphere
  subsystem: hub

health:
  timeout: 2s

shutdown:
  timeout: 15s
```
