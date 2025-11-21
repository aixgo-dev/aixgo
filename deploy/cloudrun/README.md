# Cloud Run Deployment Guide

This directory contains configurations for deploying aixgo to Google Cloud Run using the Go deployment tool.

## Prerequisites

- Go 1.23+ installed
- Google Cloud SDK installed and configured
- Docker installed
- GCP project with billing enabled
- Required environment variables set

## Quick Start

1. Set environment variables:

### Required for Cloud Run Deployment

```bash
export GCP_PROJECT_ID="your-project-id"
export GCP_REGION="us-central1"
```

### Optional - Only if using these AI services

```bash
export XAI_API_KEY="your-xai-key"              # Only if using xAI/Grok models
export OPENAI_API_KEY="your-openai-key"        # Only if using OpenAI models
export HUGGINGFACE_API_KEY="your-huggingface-key"  # Only if using HuggingFace models
```

**Note**: You only need to set API keys for the AI services you plan to use. The deployment will work with any combination of these services.

2. Deploy using the Go tool:

```bash
# From project root
go run cmd/deploy/cloudrun/main.go \
  -project $GCP_PROJECT_ID \
  -region $GCP_REGION

# Or use Makefile
make deploy-cloudrun
```

The deployment tool will:
- Enable required GCP APIs
- Create Artifact Registry repository
- Build and push Docker image
- Create service account with required permissions
- Store API keys in Secret Manager
- Deploy service to Cloud Run
- Run health checks

For detailed usage and all available flags, see the [Deployment Tools Documentation](../../cmd/tools/README.md).

## Deployment Options

### Using Go Tool (Recommended)

```bash
# Full deployment
go run cmd/deploy/cloudrun/main.go -project $GCP_PROJECT_ID

# Deploy to staging
go run cmd/deploy/cloudrun/main.go \
  -project $GCP_PROJECT_ID \
  -env staging \
  -service aixgo-mcp-staging

# Skip build (use existing image)
go run cmd/deploy/cloudrun/main.go \
  -project $GCP_PROJECT_ID \
  -skip-build

# Skip secrets (already created)
go run cmd/deploy/cloudrun/main.go \
  -project $GCP_PROJECT_ID \
  -skip-secrets

# Dry run (show commands without executing)
go run cmd/deploy/cloudrun/main.go \
  -project $GCP_PROJECT_ID \
  -dry-run

# Custom resource limits
go run cmd/deploy/cloudrun/main.go \
  -project $GCP_PROJECT_ID \
  -cpu 4 \
  -memory 4Gi \
  -max-instances 200
```

### Using Makefile

```bash
# Deploy to production
make deploy-cloudrun

# Deploy to staging
make deploy-cloudrun-staging

# Deploy to production (explicit)
make deploy-cloudrun-production
```

### Manual Deployment Steps

If you need to deploy manually without the Go tool:

#### 1. Build Docker Image

```bash
docker build -t ${GCP_REGION}-docker.pkg.dev/${GCP_PROJECT_ID}/aixgo/mcp-server:latest \
  -f docker/aixgo.Dockerfile .
docker push ${GCP_REGION}-docker.pkg.dev/${GCP_PROJECT_ID}/aixgo/mcp-server:latest
```

#### 2. Create Secrets

```bash
echo -n "your-api-key" | gcloud secrets create xai-api-key --data-file=-
echo -n "your-api-key" | gcloud secrets create openai-api-key --data-file=-
echo -n "your-api-key" | gcloud secrets create huggingface-api-key --data-file=-
```

#### 3. Deploy Service

**Security Warning**: The `--allow-unauthenticated` flag makes the service **publicly accessible** and is intended **only for testing**. For production, **remove this flag** and configure IAM authentication (see [Authentication section](#authentication) below).

```bash
# Development/Testing deployment (public access)
gcloud run deploy aixgo-mcp \
  --image=${GCP_REGION}-docker.pkg.dev/${GCP_PROJECT_ID}/aixgo/mcp-server:latest \
  --platform=managed \
  --region=${GCP_REGION} \
  --allow-unauthenticated
```

**For production**, deploy with authentication required:

```bash
# Production deployment (authenticated access only)
gcloud run deploy aixgo-mcp \
  --image=${GCP_REGION}-docker.pkg.dev/${GCP_PROJECT_ID}/aixgo/mcp-server:latest \
  --platform=managed \
  --region=${GCP_REGION} \
  --no-allow-unauthenticated

# Grant access to service accounts or users
gcloud run services add-iam-policy-binding aixgo-mcp \
  --region=us-central1 \
  --member="serviceAccount:caller@${GCP_PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/run.invoker"
```

For complete authentication setup including IAM roles, service accounts, and VPC ingress controls, see the [Authentication section](#authentication) below.

#### 4. Update Service (Using YAML)

**Note**: This step is **optional** and only needed if you prefer declarative YAML-based deployments over the imperative `gcloud run deploy` command shown in step 3.

The `service.yaml` file is located at `deploy/cloudrun/service.yaml` in this repository. It provides a complete Cloud Run service specification including resource limits, scaling configuration, health checks, and secret mounting.

To use YAML-based deployment:

```bash
# Update service.yaml with your project details
sed -i "s/PROJECT_ID/${GCP_PROJECT_ID}/g" deploy/cloudrun/service.yaml
sed -i "s/REGION/${GCP_REGION}/g" deploy/cloudrun/service.yaml

# Apply the configuration
gcloud run services replace deploy/cloudrun/service.yaml --region=${GCP_REGION}
```

**When to use YAML-based deployment**:
- You need precise control over all service configuration options
- You want to version control your complete service specification
- You're using GitOps workflows for infrastructure management
- You need to maintain multiple environment configurations (dev/staging/prod)

**When to use imperative deployment** (step 3):
- Quick deployments and testing
- Simple configuration requirements
- Interactive deployment workflows

## Configuration

### Environment Variables

- `PORT`: HTTP server port (default: 8080)
- `GRPC_PORT`: gRPC server port (default: 9090)
- `LOG_LEVEL`: Logging level (debug, info, warn, error)
- `ENVIRONMENT`: Deployment environment (development, staging, production)
- `OLLAMA_URL`: Ollama service URL for local models

### Resource Limits

Default configuration:
- CPU: 2 vCPU
- Memory: 2Gi
- Timeout: 300 seconds
- Concurrency: 80 requests
- Min instances: 0
- Max instances: 100

Adjust in `service.yaml` or via CLI flags.

## Monitoring

### Health Checks

- Liveness: `GET /health/live`
- Readiness: `GET /health/ready`
- Health: `GET /health`

### Metrics

Prometheus metrics available at: `GET /metrics`

Key metrics:
- `aixgo_http_requests_total`: Total HTTP requests
- `aixgo_mcp_tool_calls_total`: Total MCP tool calls
- `aixgo_grpc_requests_total`: Total gRPC requests
- `aixgo_agent_messages_total`: Total agent messages

### Logs

View logs using Cloud Logging:

```bash
gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=aixgo-mcp" --limit 50
```

Or via the Cloud Console:
https://console.cloud.google.com/run/detail/REGION/aixgo-mcp/logs

## Scaling

Cloud Run autoscales based on:
- Number of concurrent requests
- CPU utilization
- Memory usage

Configure scaling:

```bash
gcloud run services update aixgo-mcp \
  --min-instances=1 \
  --max-instances=50 \
  --concurrency=100
```

## Security

### Service Account Permissions

The service account has these IAM roles:
- `roles/secretmanager.secretAccessor`: Access secrets
- `roles/logging.logWriter`: Write logs
- `roles/cloudtrace.agent`: Send traces

### TLS/SSL

Cloud Run automatically provides TLS certificates. Custom domains can be mapped:

```bash
gcloud run domain-mappings create --service=aixgo-mcp --domain=api.example.com
```

### Authentication

Enable authentication:

```bash
gcloud run services update aixgo-mcp --no-allow-unauthenticated
```

## Costs

Estimated costs for Cloud Run:
- 2 vCPU @ $0.00002400/vCPU-second
- 2 GiB memory @ $0.00000250/GiB-second
- 1 million requests/month @ $0.40

Plus:
- Artifact Registry storage
- Secret Manager access
- Cloud Logging
- Network egress

Use the [GCP Pricing Calculator](https://cloud.google.com/products/calculator) for detailed estimates.

## Troubleshooting

### Deployment Fails

Check logs:
```bash
gcloud run services logs read aixgo-mcp --limit=50
```

### Health Checks Failing

Test locally:
```bash
docker run -p 8080:8080 gcr.io/${GCP_PROJECT_ID}/aixgo-mcp:latest
curl http://localhost:8080/health/live
```

### Secret Access Issues

Verify service account permissions:
```bash
gcloud projects get-iam-policy ${GCP_PROJECT_ID} \
  --flatten="bindings[].members" \
  --filter="bindings.members:serviceAccount:aixgo-mcp@${GCP_PROJECT_ID}.iam.gserviceaccount.com"
```

### Performance Issues

Monitor metrics:
```bash
gcloud monitoring dashboards list --filter="displayName:Cloud Run"
```

## CI/CD Integration

The deployment tool is integrated into GitHub Actions. See `.github/workflows/deploy-cloudrun.yml` for the automated deployment workflow.

The workflow uses the Go deployment tool:

```yaml
- name: Deploy to Cloud Run
  run: |
    go run cmd/deploy/cloudrun/main.go \
      -project ${{ env.PROJECT_ID }} \
      -region ${{ env.REGION }} \
      -service ${{ env.SERVICE_NAME }} \
      -env ${{ github.event.inputs.environment || 'staging' }}
```

## Additional Resources

- [Cloud Run Documentation](https://cloud.google.com/run/docs)
- [Cloud Run Best Practices](https://cloud.google.com/run/docs/best-practices)
- [Cloud Run Pricing](https://cloud.google.com/run/pricing)
