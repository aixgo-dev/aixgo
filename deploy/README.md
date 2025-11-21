# Deployment Guide

This directory contains deployment configurations and scripts for aixgo across different platforms.

## Available Deployment Options

### 1. Cloud Run (Serverless)

Deploy to Google Cloud Run for serverless, auto-scaling infrastructure.

**Directory**: `cloudrun/`

**Best for**:
- Variable workloads
- Quick prototypes
- Cost-sensitive deployments
- Zero-ops maintenance

**Quick Start**:
```bash
cd cloudrun
./deploy.sh
```

See [Cloud Run README](cloudrun/README.md) for detailed instructions.

### 2. Kubernetes (GKE, EKS, AKS)

Deploy to any Kubernetes cluster with full control over infrastructure.

**Directory**: `k8s/`

**Best for**:
- Production workloads
- Multi-region deployments
- Advanced networking requirements
- Custom resource requirements

**Quick Start**:
```bash
# Staging
kubectl apply -k k8s/overlays/staging

# Production
kubectl apply -k k8s/overlays/production
```

See [Kubernetes README](k8s/README.md) for detailed instructions.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      Load Balancer / Ingress                │
└─────────────────────┬───────────────────────┬───────────────┘
                      │                       │
         ┌────────────▼──────────┐  ┌────────▼─────────────┐
         │  Aixgo Orchestrator   │  │    MCP Server        │
         │                       │  │  (HuggingFace)       │
         │  - Agent coordination │  │  - Tool serving      │
         │  - Message routing    │  │  - gRPC/HTTP APIs    │
         │  - Observability      │  │  - Health checks     │
         └────────────┬──────────┘  └──────────────────────┘
                      │
         ┌────────────▼──────────┐
         │   Ollama Service      │
         │                       │
         │  - Local LLM runtime  │
         │  - Model serving      │
         └───────────────────────┘
```

## Components

### Aixgo Orchestrator

Main service that coordinates agent workflows.

- **HTTP API**: Port 8080
- **gRPC API**: Port 9090
- **Health checks**: `/health/live`, `/health/ready`
- **Metrics**: `/metrics` (Prometheus format)

### MCP Server

Model Context Protocol server for HuggingFace integration.

- **HTTP API**: Port 8080
- **gRPC API**: Port 9090
- **Tools**: HuggingFace inference, embeddings, etc.
- **Health checks**: `/health/live`, `/health/ready`

### Ollama Service

Local LLM runtime for open-source models.

- **API**: Port 11434
- **Models**: Configurable via environment
- **Storage**: Persistent volume for model data

## Environment Variables

### Common Configuration

```bash
# Server Configuration
PORT=8080                    # HTTP server port
GRPC_PORT=9090              # gRPC server port
LOG_LEVEL=info              # Logging level
ENVIRONMENT=production       # Environment name

# LLM Configuration
OLLAMA_URL=http://ollama-service:11434

# API Keys (use Secret Manager in production)
XAI_API_KEY=your-xai-key
OPENAI_API_KEY=your-openai-key
HUGGINGFACE_API_KEY=your-hf-key

# Observability
ENABLE_TRACING=true
ENABLE_METRICS=true
```

## Monitoring and Observability

### Health Checks

All services expose standard health check endpoints:

- **Liveness**: `GET /health/live` - Is the service running?
- **Readiness**: `GET /health/ready` - Is the service ready to accept traffic?
- **Health**: `GET /health` - Detailed health status

### Prometheus Metrics

Metrics are exposed at `GET /metrics`:

#### HTTP Metrics
- `aixgo_http_requests_total`: Total HTTP requests
- `aixgo_http_request_duration_seconds`: Request duration histogram

#### MCP Metrics
- `aixgo_mcp_tool_calls_total`: Total tool calls
- `aixgo_mcp_tool_call_duration_seconds`: Tool call duration

#### Agent Metrics
- `aixgo_agent_messages_total`: Total agent messages
- `aixgo_agent_execution_duration_seconds`: Agent execution duration

#### System Metrics
- `aixgo_active_connections`: Active connections
- `aixgo_memory_usage_bytes`: Memory usage
- `aixgo_goroutines`: Number of goroutines

### Logging

Structured JSON logs are written to stdout/stderr:

```json
{
  "level": "info",
  "timestamp": "2025-11-20T12:00:00Z",
  "service": "aixgo-orchestrator",
  "message": "Agent execution started",
  "agent": "analyzer",
  "trace_id": "abc123"
}
```

### Tracing

OpenTelemetry distributed tracing is enabled by default:

- **Trace context**: Propagated via HTTP headers and gRPC metadata
- **Exporters**: OTLP, Jaeger, Zipkin
- **Sampling**: Configurable sampling rate

## Security

### API Key Management

**Development**:
- Use `.env` file (never commit!)
- Set environment variables directly

**Production**:
- Use Secret Manager (GCP Secret Manager, AWS Secrets Manager, etc.)
- Mount secrets as environment variables
- Rotate keys regularly

### TLS/SSL

**Cloud Run**:
- Automatic TLS termination
- Custom domains supported

**Kubernetes**:
- Use cert-manager for automatic certificate management
- Configure TLS in Ingress
- Enable mTLS with service mesh (Istio, Linkerd)

### Network Security

**Cloud Run**:
- VPC connector for private resources
- Identity-aware proxy for authentication

**Kubernetes**:
- Network policies to restrict traffic
- Service mesh for mTLS
- Ingress with WAF

## Cost Optimization

### Cloud Run

- Use min instances = 0 for development
- Set min instances = 1+ for production (avoid cold starts)
- Optimize container size (<100MB)
- Use appropriate CPU/memory allocation

**Estimated costs** (100K requests/month):
- Small workload: $5-10/month
- Medium workload: $20-50/month
- Large workload: $100-200/month

### Kubernetes

- Use horizontal pod autoscaling
- Configure resource requests/limits appropriately
- Use spot/preemptible instances for non-critical workloads
- Enable cluster autoscaling

**Estimated costs** (3-node cluster):
- Development: $150-300/month
- Production: $500-1500/month

## CI/CD Integration

Automated deployments are configured via GitHub Actions:

- `.github/workflows/ci.yml` - Continuous integration
- `.github/workflows/release.yml` - Release builds
- `.github/workflows/deploy-cloudrun.yml` - Cloud Run deployment
- `.github/workflows/deploy-k8s.yml` - Kubernetes deployment

### Required Secrets

Configure these in your GitHub repository settings:

**GCP Authentication**:
- `GCP_PROJECT_ID`: GCP project ID
- `WIF_PROVIDER`: Workload Identity Federation provider
- `WIF_SERVICE_ACCOUNT`: Service account for WIF
- `CLOUD_RUN_SA`: Cloud Run service account

**API Keys** (optional, if not using Secret Manager):
- `XAI_API_KEY`
- `OPENAI_API_KEY`
- `HUGGINGFACE_API_KEY`

**Notifications** (optional):
- `SLACK_WEBHOOK_URL`: Slack webhook for deployment notifications

## Troubleshooting

### Service Not Starting

1. Check logs:
   ```bash
   # Cloud Run
   gcloud run services logs read aixgo-mcp --limit=50

   # Kubernetes
   kubectl logs -l app=aixgo -n aixgo --tail=50
   ```

2. Verify environment variables are set correctly
3. Check health endpoints respond

### High Latency

1. Check resource utilization:
   ```bash
   # Kubernetes
   kubectl top pods -n aixgo
   ```

2. Review metrics in Prometheus/Grafana
3. Enable debug logging temporarily
4. Check Ollama model loading time

### Out of Memory

1. Increase memory limits:
   - Cloud Run: `--memory=4Gi`
   - Kubernetes: Update resource limits in manifests

2. Monitor memory usage:
   ```bash
   # Check /metrics endpoint
   curl http://service-url/metrics | grep memory
   ```

3. Optimize model selection (smaller models for Ollama)

### Connection Refused

1. Verify service is running:
   ```bash
   # Kubernetes
   kubectl get pods -n aixgo
   kubectl describe service aixgo-service -n aixgo
   ```

2. Check network policies
3. Verify firewall rules
4. Test internal connectivity

## Support and Contributing

- **Documentation**: See individual deployment READMEs
- **Issues**: [GitHub Issues](https://github.com/aixgo-dev/aixgo/issues)
- **Discussions**: [GitHub Discussions](https://github.com/aixgo-dev/aixgo/discussions)
- **Contributing**: See [CONTRIBUTING.md](../docs/CONTRIBUTING.md)

## Additional Resources

- [Cloud Run Documentation](https://cloud.google.com/run/docs)
- [Kubernetes Documentation](https://kubernetes.io/docs/home/)
- [GCP Best Practices](https://cloud.google.com/architecture/best-practices)
- [Prometheus Monitoring](https://prometheus.io/docs/introduction/overview/)
- [OpenTelemetry](https://opentelemetry.io/docs/)
