# Deployment Infrastructure Guide

This guide covers the complete deployment infrastructure for aixgo with HuggingFace + MCP integration.

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Prerequisites](#prerequisites)
4. [Local Development](#local-development)
5. [Cloud Run Deployment](#cloud-run-deployment)
6. [Kubernetes Deployment](#kubernetes-deployment)
7. [CI/CD Pipeline](#cicd-pipeline)
8. [Monitoring & Observability](#monitoring--observability)
9. [Security](#security)
10. [Troubleshooting](#troubleshooting)

## Overview

The deployment infrastructure provides:

- **gRPC Implementation**: Full MCP over gRPC with TLS support
- **Cloud Run**: Serverless deployment with auto-scaling
- **Kubernetes**: Production-grade container orchestration
- **CI/CD**: Automated build, test, and deployment via GitHub Actions
- **Observability**: Prometheus metrics, health checks, and distributed tracing

## Architecture

```text
┌────────────────────────────────────────────────────────────────┐
│                      Internet / Load Balancer                  │
└────────────────────────┬───────────────────────────────────────┘
                         │
         ┌───────────────┴────────────────┐
         │                                │
    ┌────▼─────────┐              ┌──────▼──────────┐
    │   Aixgo      │              │   MCP Server    │
    │ Orchestrator │◄────────────►│  (HuggingFace)  │
    │              │   gRPC       │                 │
    │ - Agents     │              │ - Tools         │
    │ - Routing    │              │ - gRPC/HTTP     │
    │ - Metrics    │              │ - Metrics       │
    └────┬─────────┘              └─────────────────┘
         │
         │ HTTP
         │
    ┌────▼─────────┐
    │   Ollama     │
    │   Service    │
    │              │
    │ - LLM Models │
    │ - Inference  │
    └──────────────┘
```

### Components

1. **Aixgo Orchestrator** (`cmd/aixgo/`)

   - Agent coordination and message routing
   - HTTP API (port 8080) and gRPC (port 9090)
   - Health checks and metrics

2. **MCP Server** (`cmd/mcp-server/`)

   - Model Context Protocol implementation
   - HuggingFace integration
   - gRPC and HTTP APIs

3. **Ollama Service**
   - Local LLM runtime
   - Model serving (port 11434)
   - Persistent storage for models

## Prerequisites

### Required Tools

- **Docker**: Container runtime
- **kubectl**: Kubernetes CLI (for K8s deployment)
- **gcloud**: Google Cloud SDK (for GCP deployment)
- **Go 1.23+**: For local development
- **protoc**: Protocol buffer compiler

### Install Dependencies

```bash
# Install Go dependencies
go mod download

# Install protoc plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate protobuf code (using Go tool in the future)
go run proto/mcp/generate.go
```

## Local Development

### 1. Run with Docker Compose

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

Services will be available at:

- Aixgo: http://localhost:8080
- Ollama: http://localhost:11434

### 2. Run Locally

```bash
# Terminal 1: Start Ollama
docker run -p 11434:11434 ollama/ollama:latest

# Terminal 2: Start MCP Server
go run cmd/mcp-server/main.go

# Terminal 3: Start Orchestrator
go run cmd/aixgo/main.go -config examples/huggingface-mcp/config.yaml
```

### 3. Test Endpoints

```bash
# Health checks
curl http://localhost:8080/health/live
curl http://localhost:8080/health/ready
curl http://localhost:8080/health

# Metrics
curl http://localhost:8080/metrics
```

## Cloud Run Deployment

### Quick Deploy

```bash
# Set environment variables
export GCP_PROJECT_ID="your-project-id"
export GCP_REGION="us-central1"
export XAI_API_KEY="your-xai-key"
export OPENAI_API_KEY="your-openai-key"
export HUGGINGFACE_API_KEY="your-hf-key"

# Deploy using gcloud
gcloud run deploy aixgo-mcp \
  --image us-central1-docker.pkg.dev/$GCP_PROJECT_ID/aixgo/mcp-server:latest \
  --region $GCP_REGION \
  --platform managed \
  --allow-unauthenticated
```

The deployment process:

1. Enable required GCP APIs
2. Create Artifact Registry repository
3. Build and push Docker images
4. Create service account with permissions
5. Store secrets in Secret Manager
6. Deploy to Cloud Run
7. Run health checks

See [deploy/cloudrun/README.md](../deploy/cloudrun/README.md) for detailed instructions.

### Manual Deployment Steps

#### 1. Build and Push Image

```bash
# Configure Docker for Artifact Registry
gcloud auth configure-docker us-central1-docker.pkg.dev

# Build image
docker build -t us-central1-docker.pkg.dev/${PROJECT_ID}/aixgo/mcp-server:latest \
  -f docker/aixgo.Dockerfile .

# Push image
docker push us-central1-docker.pkg.dev/${PROJECT_ID}/aixgo/mcp-server:latest
```

#### 2. Create Secrets

```bash
# Store API keys in Secret Manager
echo -n "${XAI_API_KEY}" | gcloud secrets create xai-api-key --data-file=-
echo -n "${OPENAI_API_KEY}" | gcloud secrets create openai-api-key --data-file=-
echo -n "${HUGGINGFACE_API_KEY}" | gcloud secrets create huggingface-api-key --data-file=-
```

#### 3. Deploy Service

**Security Warning**: The `--allow-unauthenticated` flag grants **public access** to your service and should **only be used for local/development/test deployments**. For production
deployments, **remove this flag** or use `--no-allow-unauthenticated` and configure IAM authentication.

```bash
gcloud run deploy aixgo-mcp \
  --image=us-central1-docker.pkg.dev/${PROJECT_ID}/aixgo/mcp-server:latest \
  --platform=managed \
  --region=us-central1 \
  --allow-unauthenticated \
  --min-instances=0 \
  --max-instances=100 \
  --cpu=2 \
  --memory=2Gi \
  --timeout=300 \
  --set-secrets="XAI_API_KEY=xai-api-key:latest,OPENAI_API_KEY=openai-api-key:latest,HUGGINGFACE_API_KEY=huggingface-api-key:latest"
```

**For production deployments**, use authenticated access instead:

```bash
# Deploy with authentication required
gcloud run deploy aixgo-mcp \
  --image=us-central1-docker.pkg.dev/${PROJECT_ID}/aixgo/mcp-server:latest \
  --no-allow-unauthenticated \
  --platform=managed \
  --region=us-central1 \
  --min-instances=0 \
  --max-instances=100 \
  --cpu=2 \
  --memory=2Gi \
  --timeout=300 \
  --set-secrets="XAI_API_KEY=xai-api-key:latest,OPENAI_API_KEY=openai-api-key:latest,HUGGINGFACE_API_KEY=huggingface-api-key:latest"

# Grant access to specific service account or user
gcloud run services add-iam-policy-binding aixgo-mcp \
  --region=us-central1 \
  --member="serviceAccount:caller@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/run.invoker"
```

See the [Authentication section (line 522)](#authentication) for comprehensive authentication configuration, IAM setup, and service account management.

### Cloud Run Configuration

See `deploy/cloudrun/service.yaml` for full configuration including:

- Resource limits (CPU, memory)
- Scaling configuration
- Health checks
- Environment variables
- Secret mounting

## Kubernetes Deployment

### GKE Setup

#### 1. Create Cluster

```bash
gcloud container clusters create aixgo-cluster \
  --region=us-central1 \
  --num-nodes=3 \
  --machine-type=n1-standard-4 \
  --enable-autoscaling \
  --min-nodes=3 \
  --max-nodes=10 \
  --enable-stackdriver-kubernetes \
  --enable-ip-alias
```

#### 2. Get Credentials

```bash
gcloud container clusters get-credentials aixgo-cluster --region=us-central1
```

### Deploy to Staging

```bash
# Set environment variables
export GCP_PROJECT_ID="your-project-id"
export GKE_CLUSTER="aixgo-cluster"
export GKE_ZONE="us-central1-a"
export XAI_API_KEY="your-xai-key"
export OPENAI_API_KEY="your-openai-key"
export HUGGINGFACE_API_KEY="your-hf-key"

# Get cluster credentials
gcloud container clusters get-credentials $GKE_CLUSTER --zone $GKE_ZONE

# Deploy using kubectl
kubectl apply -k deploy/k8s/overlays/staging
```

### Deploy to Production

```bash
# Get cluster credentials
gcloud container clusters get-credentials $GKE_CLUSTER --zone $GKE_ZONE

# Deploy using kubectl
kubectl apply -k deploy/k8s/overlays/production
```

The deployment process:

1. Authenticate to GCP and get cluster credentials
2. Build and push Docker images
3. Create namespace and secrets
4. Apply Kubernetes manifests via kustomize
5. Wait for rollout completion

See [deploy/k8s/README.md](../deploy/k8s/README.md) for detailed instructions.

### Kubernetes Resources

The deployment includes:

- **Deployments**: Aixgo orchestrator, MCP server, Ollama
- **Services**: ClusterIP services for internal communication
- **HPA**: Horizontal Pod Autoscaling based on CPU/memory
- **Ingress**: External access with TLS termination
- **ConfigMaps**: Configuration management
- **Secrets**: API keys and certificates
- **RBAC**: Service accounts and permissions
- **PVC**: Persistent storage for Ollama models

## CI/CD Pipeline

### GitHub Actions Workflows

#### 1. CI Workflow (`.github/workflows/ci.yml`)

Triggered on push and pull requests:

```yaml
- Lint (golangci-lint)
- Test (unit tests, race detection, coverage)
- Build (multi-platform binaries)
- Docker build (without push)
- Security scan (Trivy, Gosec)
```

#### 2. Release Workflow (`.github/workflows/release.yml`)

Triggered on version tags (`v*.*.*`):

```yaml
- Build multi-platform binaries
- Build and push Docker images (amd64, arm64)
- Create GitHub release with artifacts
- Push to GitHub Container Registry and Docker Hub
```

#### 3. Cloud Run Deploy (`.github/workflows/deploy-cloudrun.yml`)

Triggered on main branch changes:

```yaml
- Authenticate to GCP
- Build and push Docker image
- Deploy to Cloud Run
- Run health checks
- Create deployment summary
```

#### 4. Kubernetes Deploy (`.github/workflows/deploy-k8s.yml`)

Triggered on main branch changes:

```yaml
- Authenticate to GCP
- Build and push Docker images
- Update Kubernetes manifests
- Deploy with kustomize
- Run smoke tests
- Send notifications
```

### Required GitHub Secrets

Configure in repository settings → Secrets and variables → Actions:

```bash
GCP_PROJECT_ID: Your GCP project ID
WIF_PROVIDER: Workload Identity Federation provider
WIF_SERVICE_ACCOUNT: Service account for WIF
CLOUD_RUN_SA: Cloud Run service account
XAI_API_KEY: xAI API key (optional)
OPENAI_API_KEY: OpenAI API key (optional)
HUGGINGFACE_API_KEY: HuggingFace API key (optional)
SLACK_WEBHOOK_URL: Slack webhook for notifications (optional)
```

### Creating a Release

```bash
# Tag a new version
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# GitHub Actions will automatically:
# 1. Build binaries for all platforms
# 2. Build and push Docker images
# 3. Create GitHub release with artifacts
```

## Monitoring & Observability

### Health Checks

All services expose three health check endpoints:

```bash
# Liveness: Is the process running?
curl http://service:8080/health/live

# Readiness: Can it accept traffic?
curl http://service:8080/health/ready

# Detailed health status
curl http://service:8080/health
```

Response format:

```json
{
  "status": "healthy",
  "timestamp": "2025-11-20T12:00:00Z",
  "version": "1.0.0",
  "uptime": "1h30m",
  "checks": {
    "ping": {
      "status": "healthy",
      "message": "OK",
      "last_checked": "2025-11-20T12:00:00Z"
    }
  },
  "system": {
    "num_goroutines": 42,
    "num_cpu": 4,
    "mem_alloc_mb": 128,
    "mem_sys_mb": 256
  }
}
```

### Prometheus Metrics

Metrics endpoint: `http://service:8080/metrics`

#### Key Metrics

**HTTP Metrics**:

```
aixgo_http_requests_total{method="GET",path="/health",status="200"}
aixgo_http_request_duration_seconds{method="GET",path="/health"}
```

**MCP Metrics**:

```
aixgo_mcp_tool_calls_total{tool="echo",status="success"}
aixgo_mcp_tool_call_duration_seconds{tool="echo"}
```

**gRPC Metrics**:

```
aixgo_grpc_requests_total{method="/mcp.MCPService/CallTool",status="OK"}
aixgo_grpc_request_duration_seconds{method="/mcp.MCPService/CallTool"}
```

**Agent Metrics**:

```
aixgo_agent_messages_total{agent="analyzer",type="request"}
aixgo_agent_execution_duration_seconds{agent="analyzer"}
```

**System Metrics**:

```
aixgo_active_connections
aixgo_memory_usage_bytes
aixgo_goroutines
```

### Setting up Prometheus

#### Kubernetes

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: aixgo
  namespace: aixgo
spec:
  selector:
    matchLabels:
      app: aixgo
  endpoints:
    - port: http
      path: /metrics
      interval: 30s
```

#### Cloud Run

Use Cloud Monitoring integration or deploy a Prometheus instance that scrapes the metrics endpoint.

### Distributed Tracing

OpenTelemetry is integrated by default:

```go
// Traces are automatically created for:
// - HTTP requests
// - gRPC calls
// - Agent executions
// - MCP tool calls
```

Configure trace exporter via environment variables:

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4318
OTEL_SERVICE_NAME=aixgo-orchestrator
OTEL_TRACES_SAMPLER=parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG=0.1
```

## Security

### Authentication & Authorization

#### Cloud Run

Enable authentication:

```bash
gcloud run services update aixgo-mcp --no-allow-unauthenticated
```

Access with service account:

```bash
TOKEN=$(gcloud auth print-identity-token)
curl -H "Authorization: Bearer ${TOKEN}" https://service-url/health
```

#### Kubernetes

Use Ingress with authentication:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/auth-type: basic
    nginx.ingress.kubernetes.io/auth-secret: basic-auth
```

### TLS/SSL

#### Cloud Run

Automatic TLS termination. For custom domains:

```bash
gcloud run domain-mappings create \
  --service=aixgo-mcp \
  --domain=api.example.com
```

#### Kubernetes

Use cert-manager for automatic certificates:

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.3/cert-manager.yaml
```

### Secret Management

#### Development

Use `.env` file (never commit):

```bash
cp .env.example .env
# Edit .env with your keys
```

#### Production

**GCP Secret Manager**:

```bash
# Create secret
gcloud secrets create api-key --data-file=-

# Grant access
gcloud secrets add-iam-policy-binding api-key \
  --member="serviceAccount:service@project.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"
```

**Kubernetes Secrets**:

```bash
kubectl create secret generic api-keys \
  --from-literal=key=value \
  -n aixgo
```

### Network Security

#### Cloud Run

- Use VPC connector for private resources
- Configure ingress settings (internal, internal-and-cloud-load-balancing, all)

#### Kubernetes

Apply network policies:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: aixgo-netpol
spec:
  podSelector:
    matchLabels:
      app: aixgo
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app: ingress-nginx
  egress:
    - to:
        - podSelector:
            matchLabels:
              app: ollama
```

## Troubleshooting

### Service Not Starting

**Check logs**:

```bash
# Cloud Run
gcloud run services logs read aixgo-mcp --limit=50

# Kubernetes
kubectl logs -l app=aixgo -n aixgo --tail=50
```

**Common issues**:

- Missing environment variables
- Invalid API keys
- Insufficient resources
- Image pull errors

### High Latency

**Diagnose**:

```bash
# Check metrics
curl http://service:8080/metrics | grep duration

# Check resource usage
kubectl top pods -n aixgo
```

**Solutions**:

- Scale up resources
- Increase replica count
- Optimize Ollama model loading
- Enable request caching

### Connection Issues

**Test connectivity**:

```bash
# Port forward for testing
kubectl port-forward svc/aixgo-service 8080:8080 -n aixgo

# Test from within cluster
kubectl run test --rm -it --image=curlimages/curl -- sh
curl http://aixgo-service:8080/health
```

**Check**:

- Service endpoints: `kubectl get endpoints -n aixgo`
- Network policies
- Firewall rules
- DNS resolution

### Memory Issues

**Monitor memory**:

```bash
# Check memory usage
curl http://service:8080/metrics | grep memory

# Kubernetes
kubectl top pods -n aixgo
```

**Solutions**:

- Increase memory limits
- Use smaller Ollama models
- Enable memory profiling
- Check for memory leaks

### gRPC Errors

**Common issues**:

1. **Connection refused**: Check gRPC port (9090) is exposed
2. **TLS errors**: Verify certificate configuration
3. **Deadline exceeded**: Increase timeout values
4. **Unavailable**: Check service health and connectivity

**Debug**:

```bash
# Test gRPC endpoint with grpcurl
grpcurl -plaintext localhost:9090 list
grpcurl -plaintext localhost:9090 mcp.MCPService/Ping
```

## Additional Resources

- [Cloud Run Documentation](https://cloud.google.com/run/docs)
- [GKE Documentation](https://cloud.google.com/kubernetes-engine/docs)
- [Prometheus Documentation](https://prometheus.io/docs/)
- [gRPC Documentation](https://grpc.io/docs/)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)

## Support

- **Issues**: [GitHub Issues](https://github.com/aixgo-dev/aixgo/issues)
- **Discussions**: [GitHub Discussions](https://github.com/aixgo-dev/aixgo/discussions)
- **Documentation**: [docs/](../docs/)
