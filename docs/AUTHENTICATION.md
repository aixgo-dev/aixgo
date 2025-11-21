# Authentication and Authorization Guide

This document describes how to configure authentication and authorization for aixgo applications.

## Overview

aixgo supports flexible authentication modes to accommodate different deployment scenarios:

| Mode        | Use Case                            | Auth Handled By |
| ----------- | ----------------------------------- | --------------- |
| `disabled`  | Local development                   | N/A             |
| `delegated` | Cloud Run + IAP, Kubernetes + Istio | Infrastructure  |
| `builtin`   | Self-hosted, API services           | Application     |
| `hybrid`    | Mixed human + service auth          | Both            |

## Authentication Modes

### Disabled Mode

For local development only. No authentication is performed.

```yaml
environment: development
auth_mode: disabled
```

**Warning**: This mode is NOT allowed in production. The configuration validation will fail if you attempt to use `auth_mode: disabled` with `environment: production`.

### Delegated Mode

Use this when authentication is handled by infrastructure (e.g., Google Cloud IAP, Istio service mesh).

```yaml
environment: production
auth_mode: delegated

delegated_auth:
  identity_header: X-Goog-Authenticated-User-Email
  iap:
    enabled: true
    verify_jwt: true
    audience: '/projects/123456789/apps/my-project-id'
```

**How it works**:

1. Infrastructure authenticates the user
2. Identity information is passed via HTTP headers
3. aixgo extracts the identity and creates a Principal

**Supported Infrastructure**:

- Google Cloud IAP (Identity-Aware Proxy)
- Istio service mesh
- Any proxy that sets identity headers

### Builtin Mode

Use this when the application needs to validate credentials directly.

```yaml
environment: production
auth_mode: builtin

builtin_auth:
  method: api_key
  api_keys:
    source: environment
    env_prefix: AIXGO_API_KEY_
```

**Setting up API keys**:

Set environment variables with the configured prefix:

```bash
export AIXGO_API_KEY_service1="your-secret-api-key-here"
export AIXGO_API_KEY_admin="admin-api-key-here"
```

The part after the prefix becomes the principal ID.

**Making authenticated requests**:

```bash
curl -H "Authorization: Bearer your-secret-api-key-here" https://your-api/endpoint
```

### Hybrid Mode

Use this when you have both human users (via infrastructure auth) and service accounts (via API keys).

```yaml
environment: production
auth_mode: hybrid

delegated_auth:
  identity_header: X-Goog-Authenticated-User-Email
  iap:
    enabled: true

builtin_auth:
  method: api_key
  api_keys:
    source: environment
    env_prefix: AIXGO_API_KEY_
```

**How it works**:

1. First, try delegated authentication (check for IAP headers)
2. If no IAP headers, fall back to builtin authentication (check Authorization header)
3. If both fail, request is rejected

## Configuration Options

### SecurityConfig

| Field            | Type   | Description                                                |
| ---------------- | ------ | ---------------------------------------------------------- |
| `environment`    | string | Environment name (development, staging, production)        |
| `auth_mode`      | string | Authentication mode (disabled, delegated, builtin, hybrid) |
| `delegated_auth` | object | Delegated authentication configuration                     |
| `builtin_auth`   | object | Builtin authentication configuration                       |
| `authorization`  | object | Authorization configuration                                |
| `audit`          | object | Audit logging configuration                                |

### DelegatedAuthConfig

| Field             | Type   | Description                          |
| ----------------- | ------ | ------------------------------------ |
| `identity_header` | string | HTTP header containing user identity |
| `iap`             | object | IAP-specific configuration           |
| `header_mapping`  | map    | Map fields to custom headers         |

### BuiltinAuthConfig

| Field      | Type   | Description                     |
| ---------- | ------ | ------------------------------- |
| `method`   | string | Authentication method (api_key) |
| `api_keys` | object | API key configuration           |

### APIKeyConfig

| Field        | Type   | Description                 |
| ------------ | ------ | --------------------------- |
| `source`     | string | Key source (environment)    |
| `env_prefix` | string | Environment variable prefix |

### AuthorizationConfig

| Field          | Type   | Description                   |
| -------------- | ------ | ----------------------------- |
| `enabled`      | bool   | Enable authorization checks   |
| `default_deny` | bool   | Deny by default (recommended) |
| `policy_file`  | string | Path to policy file           |

### AuditConfig

| Field                | Type   | Description                        |
| -------------------- | ------ | ---------------------------------- |
| `enabled`            | bool   | Enable audit logging               |
| `backend`            | string | Log backend (memory, json, syslog) |
| `log_auth_decisions` | bool   | Log authentication decisions       |

## Integration

### Using with MCP Server

```go
package main

import (
    "github.com/aixgo-dev/aixgo/pkg/mcp"
    "github.com/aixgo-dev/aixgo/pkg/security"
)

func main() {
    // Load config
    config := &security.SecurityConfig{
        Environment: "production",
        AuthMode:    security.AuthModeBuiltin,
        BuiltinAuth: &security.BuiltinAuthConfig{
            Method: "api_key",
            APIKeys: &security.APIKeyConfig{
                Source:    "environment",
                EnvPrefix: "AIXGO_API_KEY_",
            },
        },
        Authorization: &security.AuthorizationConfig{
            Enabled:     true,
            DefaultDeny: true,
        },
        Audit: &security.AuditConfig{
            Enabled: true,
            Backend: "json",
        },
    }

    // Create server with security config
    server := mcp.NewServer("my-server",
        mcp.WithSecurityConfig(config),
    )

    // Get the auth extractor for HTTP middleware
    extractor := server.GetAuthExtractor()

    // Use in HTTP middleware
    handler := security.ExtractAuthContext(extractor)(yourHandler)
}
```

### HTTP Middleware

```go
// Create middleware from auth extractor
middleware := security.ExtractAuthContext(extractor)

// Apply to your HTTP handler
http.Handle("/api/", middleware(apiHandler))
```

## Security Best Practices

1. **Never disable auth in production**: The configuration validation will prevent this.

2. **Use delegated auth when available**: Cloud IAP and service mesh authentication are more secure than application-level auth.

3. **Rotate API keys regularly**: When using builtin auth, implement key rotation.

4. **Enable audit logging**: Always enable audit logging in production.

5. **Use default-deny authorization**: Set `default_deny: true` to ensure explicit permission grants.

6. **Validate JWT signatures**: When using IAP, set `verify_jwt: true` to validate token signatures.

## Example Configurations

See the `examples/` directory for complete configuration examples:

- `config-local-dev.yaml` - Local development (disabled auth)
- `config-cloudrun-iap.yaml` - Cloud Run with IAP
- `config-self-hosted.yaml` - Self-hosted with API keys
- `config-hybrid.yaml` - Hybrid mode for mixed auth

## Troubleshooting

### "auth_mode=disabled is not allowed in production"

This error occurs when you try to use disabled auth in production. Change either:

- Set `environment: development` (not recommended for production)
- Set `auth_mode: builtin` or `auth_mode: delegated`

### "delegated_auth configuration required"

When using `auth_mode: delegated`, you must provide the `delegated_auth` configuration section.

### "builtin_auth configuration required"

When using `auth_mode: builtin`, you must provide the `builtin_auth` configuration section.

### "missing Authorization header"

When using builtin auth, requests must include an Authorization header:

```
Authorization: Bearer <your-api-key>
```

### "invalid authentication token"

The API key in the Authorization header doesn't match any configured keys. Check that:

1. The environment variable is set correctly
2. The env_prefix matches your config
3. The key value matches exactly
