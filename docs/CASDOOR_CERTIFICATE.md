# Casdoor JWT certificate setup

When `features.authn=true`, `aisphere-kit` validates Casdoor JWT verification configuration during startup.

## Why this is required

Casdoor signs OAuth/OIDC access tokens with the JWT certificate selected by the Casdoor Application. Services such as Hub must verify those JWTs before trusting the user identity. A missing or wrong public key should fail during startup instead of causing the first protected request to return an `invalid key` error.

## Recommended setup

1. Open Casdoor.
2. Go to **Certs**.
3. Open the cert selected by your Casdoor Application as the JWT certificate.
4. Copy the **public certificate** or **public key**.
5. Configure it in the component YAML.

Inline config:

```yaml
features:
  authn: true

casdoor:
  endpoint: "http://localhost:18000"
  client_id: "aisphere-auth"
  client_secret: "change-me"
  organization: "aisphere"
  application: "aisphere"
  certificate: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
```

Mounted file config:

```yaml
casdoor:
  certificate_file: ./certs/casdoor-jwt-public.pem
```

Inline `certificate` takes precedence over `certificate_file`.

## Accepted PEM types

- `-----BEGIN CERTIFICATE-----`
- `-----BEGIN PUBLIC KEY-----`
- `-----BEGIN RSA PUBLIC KEY-----`

Private keys are rejected. Do not put Casdoor private keys in service config.

## Token `kid` check

The JWT header should reference the same certificate:

```json
{
  "alg": "RS256",
  "kid": "cert_aisphere",
  "typ": "JWT"
}
```

The configured public certificate/key must correspond to that `kid`.
