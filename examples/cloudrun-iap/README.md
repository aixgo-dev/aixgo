# Cloud Run with IAP Example

This example demonstrates deploying aixgo to Cloud Run with Identity-Aware Proxy (IAP) protection.

## Overview

IAP provides:
- Context-aware access control based on Google identity
- Zero-trust security model
- Centralized access management
- Audit logging for all access attempts

## Prerequisites

- GCP project with billing enabled
- Custom domain (IAP requires a domain, not Cloud Run URLs)
- OAuth consent screen configured
- `gcloud` CLI installed and configured

## Quick Start

1. **Set environment variables**:
   ```bash
   export PROJECT_ID="your-project-id"
   export REGION="us-central1"
   export DOMAIN="api.example.com"
   ```

2. **Deploy Cloud Run service** (authenticated):
   ```bash
   gcloud run deploy aixgo-mcp \
     --image=${REGION}-docker.pkg.dev/${PROJECT_ID}/aixgo/mcp-server:latest \
     --platform=managed \
     --region=${REGION} \
     --no-allow-unauthenticated
   ```

3. **Enable IAP**:
   ```bash
   gcloud services enable iap.googleapis.com
   ```

4. **Set up Load Balancer with IAP** (see Terraform below or manual steps in main README)

5. **Configure IAP audience** in your service:
   ```bash
   gcloud run services update aixgo-mcp \
     --set-env-vars="IAP_AUDIENCE=/projects/${PROJECT_NUMBER}/global/backendServices/aixgo-backend"
   ```

## Configuration Files

- `config.yaml` - Example configuration with IAP settings
- `iap.tf` - Terraform configuration for IAP setup (optional)

## IAP JWT Verification

The service automatically verifies IAP JWTs using `pkg/security/iap.go`:

```go
// In your middleware or handler
keyCache := security.NewIAPKeyCache()

func iapMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        jwt := r.Header.Get("X-Goog-IAP-JWT-Assertion")
        if jwt == "" {
            http.Error(w, "Missing IAP JWT", http.StatusUnauthorized)
            return
        }

        claims, err := security.VerifyIAPJWT(r.Context(), jwt, os.Getenv("IAP_AUDIENCE"), keyCache)
        if err != nil {
            http.Error(w, "Invalid IAP JWT", http.StatusUnauthorized)
            return
        }

        // Add claims to context
        ctx := context.WithValue(r.Context(), "iap_claims", claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

## Testing IAP Locally

For local development, you can bypass IAP verification:

```bash
# Set environment variable to disable JWT verification
export IAP_VERIFY_JWT=false
```

To simulate IAP headers locally:
```bash
curl -H "X-Goog-Authenticated-User-Email: accounts.google.com:test@example.com" \
     -H "X-Goog-Authenticated-User-Id: 123456789" \
     http://localhost:8080/api/v1/health
```

## Security Considerations

1. **Always verify JWTs in production** - The IAP headers can be spoofed if requests bypass IAP
2. **Use VPC ingress controls** - Restrict Cloud Run to only accept traffic from the load balancer
3. **Monitor access logs** - IAP provides detailed audit logs in Cloud Logging
4. **Rotate OAuth credentials** - Periodically rotate OAuth client secrets
