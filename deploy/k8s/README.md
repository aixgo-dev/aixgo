# Kubernetes Deployment Guide

This directory contains Kubernetes manifests for deploying aixgo to a Kubernetes cluster (GKE, EKS, AKS, or self-managed).

## ⚠️ Important: Configuration Required

**Before deploying**, you MUST configure the image registry placeholders in the kustomization files:

- `REGION` → Your actual region (e.g., `us`, `europe-west1`)
- `PROJECT_ID` → Your GCP project ID (e.g., `my-project`)
- `latest` → Specific version tag (e.g., `v1.0.0`, `sha-abc123`)

See the [Configuration](#configuration) section below for details.

## Architecture

The deployment consists of three main components:

1. **Ollama**: LLM runtime for local models
2. **Aixgo Orchestrator**: Main agent orchestration service
3. **MCP Server**: Model Context Protocol server for HuggingFace integration

## Directory Structure

```
k8s/
├── base/                          # Base manifests
│   ├── namespace.yaml            # aixgo namespace
│   ├── rbac.yaml                 # Service account and RBAC
│   ├── configmap.yaml            # Configuration
│   ├── secrets.yaml              # API keys and certificates
│   ├── ollama-deployment.yaml    # Ollama deployment
│   ├── aixgo-deployment.yaml     # Aixgo orchestrator
│   ├── mcp-server-deployment.yaml # MCP server
│   ├── ingress.yaml              # Ingress configuration
│   └── kustomization.yaml        # Base kustomization
└── overlays/
    ├── staging/                  # Staging environment
    │   ├── kustomization.yaml
    │   └── deployment-patch.yaml
    └── production/               # Production environment
        ├── kustomization.yaml
        └── deployment-patch.yaml
```

## Prerequisites

- Kubernetes cluster (1.24+)
- kubectl configured
- Docker installed
- gcloud CLI (for GKE)

## Quick Start

### 1. Set Environment Variables

```bash
export GCP_PROJECT_ID="your-project-id"
export GKE_CLUSTER="aixgo-cluster"
export GKE_ZONE="us-central1-a"
export XAI_API_KEY="your-xai-key"
export OPENAI_API_KEY="your-openai-key"
export HUGGINGFACE_API_KEY="your-huggingface-key"
```

### 2. Get Cluster Credentials (GKE)

```bash
gcloud container clusters get-credentials $GKE_CLUSTER --zone $GKE_ZONE
```

### 3. Deploy to Staging

```bash
kubectl apply -k deploy/k8s/overlays/staging
```

### 4. Deploy to Production

```bash
kubectl apply -k deploy/k8s/overlays/production
```

## Deployment Options

### Using kubectl with Kustomize

```bash
# Deploy to staging
make deploy-k8s-staging

# Deploy to production
make deploy-k8s-production

# Custom environment
make deploy-k8s ENVIRONMENT=staging
```

### Manual Deployment (kubectl)

If you need to deploy manually without the Go tool:

#### Option 1: Using kubectl with kustomize

```bash
# Preview what will be applied
kubectl kustomize overlays/production

# Apply configuration
kubectl apply -k overlays/production

# Watch deployment status
kubectl rollout status deployment/aixgo-orchestrator -n aixgo
```

#### Option 2: Using kustomize CLI

```bash
kustomize build overlays/production | kubectl apply -f -
```

#### Option 3: Manual step-by-step

```bash
kubectl apply -f base/namespace.yaml
kubectl apply -f base/rbac.yaml
kubectl apply -f base/configmap.yaml
kubectl apply -f base/secrets.yaml
kubectl apply -f base/ollama-deployment.yaml
kubectl apply -f base/aixgo-deployment.yaml
kubectl apply -f base/mcp-server-deployment.yaml
kubectl apply -f base/ingress.yaml
```

## Configuration

### Image Registry Configuration (Required)

**Step 1: Edit base/kustomization.yaml**

Replace placeholder values in the `images` section:

```yaml
# BEFORE (placeholder values)
images:
- name: REGION-docker.pkg.dev/PROJECT_ID/aixgo/orchestrator
  newTag: latest

# AFTER (your actual values)
images:
- name: us-docker.pkg.dev/my-project/aixgo/orchestrator
  newTag: v1.0.0  # Use specific version, NOT 'latest'
- name: us-docker.pkg.dev/my-project/aixgo/mcp-server
  newTag: v1.0.0
```

**Step 2: Pin Image Versions**

Always use specific version tags in production:
- ✅ Good: `v1.0.0`, `v2.1.3`, `sha-abc123def`
- ❌ Bad: `latest` (non-deterministic, can break deployments)

**Step 3: Verify Registry Access**

```bash
# Test image pull
docker pull us-docker.pkg.dev/my-project/aixgo/orchestrator:v1.0.0

# If using private registry, configure image pull secrets
kubectl create secret docker-registry gcr-json-key \
  --docker-server=us-docker.pkg.dev \
  --docker-username=_json_key \
  --docker-password="$(cat keyfile.json)" \
  --namespace=aixgo
```

### Environment Variables

Configure via `base/configmap.yaml`:

- `environment`: Deployment environment (staging, production)
- `log_level`: Logging level (debug, info, warn, error)
- `ollama_url`: Ollama service endpoint
- `mcp_server_url`: MCP server gRPC endpoint
- `max_rounds`: Maximum agent execution rounds
- `enable_tracing`: Enable OpenTelemetry tracing
- `enable_metrics`: Enable Prometheus metrics

### Resource Configuration

Adjust resources in overlay patches:

**Staging** (`overlays/staging/deployment-patch.yaml`):
- Lower resource limits for cost savings
- Fewer replicas

**Production** (`overlays/production/deployment-patch.yaml`):
- Higher resource limits for performance
- More replicas for high availability

### Scaling

#### Manual Scaling

```bash
kubectl scale deployment/aixgo-orchestrator --replicas=5 -n aixgo
```

#### Horizontal Pod Autoscaling

HPA is configured in deployment manifests:

```yaml
minReplicas: 2
maxReplicas: 10
targetCPUUtilizationPercentage: 70
```

Adjust in `base/aixgo-deployment.yaml` or via overlays.

## Monitoring

### Health Checks

- Liveness: `/health/live`
- Readiness: `/health/ready`
- Health: `/health`

### Prometheus Metrics

Metrics are exposed on port 8080 at `/metrics`:

```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8080"
  prometheus.io/path: "/metrics"
```

### Viewing Logs

```bash
# Aixgo orchestrator logs
kubectl logs -l app=aixgo,component=orchestrator -n aixgo --tail=100 -f

# MCP server logs
kubectl logs -l app=mcp-server -n aixgo --tail=100 -f

# Ollama logs
kubectl logs -l app=ollama -n aixgo --tail=100 -f
```

## Networking

### Services

- `aixgo-service`: Aixgo orchestrator (HTTP: 8080, gRPC: 9090)
- `mcp-server-service`: MCP server (HTTP: 8080, gRPC: 9090)
- `ollama-service`: Ollama LLM runtime (HTTP: 11434)

### Ingress

Configure in `base/ingress.yaml`:

- `api.aixgo.example.com` → Aixgo service
- `mcp.aixgo.example.com` → MCP server

Update DNS records to point to ingress IP:

```bash
kubectl get ingress aixgo-ingress -n aixgo
```

### TLS/SSL

#### Using cert-manager

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: your-email@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx
EOF
```

#### Using GKE Managed Certificates

Already configured in `base/ingress.yaml`:

```yaml
apiVersion: networking.gke.io/v1
kind: ManagedCertificate
metadata:
  name: aixgo-cert
spec:
  domains:
  - api.aixgo.example.com
  - mcp.aixgo.example.com
```

## Storage

### Ollama Persistent Volume

Ollama requires persistent storage for model files:

```yaml
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 50Gi
  storageClassName: standard-rwo
```

Adjust storage size based on models:
- Small models (2-7B): 10-20Gi
- Medium models (13-34B): 20-40Gi
- Large models (70B+): 50Gi+

## Security

### RBAC

Service account `aixgo-sa` has minimal permissions:
- Read ConfigMaps and Secrets
- List and watch Pods
- Get Services

### Network Policies

Add network policies to restrict traffic:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: aixgo-netpol
  namespace: aixgo
spec:
  podSelector:
    matchLabels:
      app: aixgo
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: ollama
    ports:
    - protocol: TCP
      port: 11434
```

### Pod Security Standards

Apply pod security standards:

```bash
kubectl label namespace aixgo pod-security.kubernetes.io/enforce=baseline
```

## Troubleshooting

### Check Pod Status

```bash
kubectl get pods -n aixgo
kubectl describe pod <pod-name> -n aixgo
```

### Check Events

```bash
kubectl get events -n aixgo --sort-by='.lastTimestamp'
```

### Debug Container

```bash
kubectl exec -it <pod-name> -n aixgo -- /bin/sh
```

### Port Forwarding

```bash
# Forward Aixgo service
kubectl port-forward svc/aixgo-service 8080:8080 -n aixgo

# Forward MCP server
kubectl port-forward svc/mcp-server-service 9090:9090 -n aixgo
```

### View Resource Usage

```bash
kubectl top pods -n aixgo
kubectl top nodes
```

## Updating Deployment

### Rolling Update

```bash
# Update image tag in kustomization.yaml
kubectl apply -k overlays/production

# Watch rollout
kubectl rollout status deployment/aixgo-orchestrator -n aixgo
```

### Rollback

```bash
kubectl rollout undo deployment/aixgo-orchestrator -n aixgo
kubectl rollout history deployment/aixgo-orchestrator -n aixgo
```

## Clean Up

```bash
# Delete specific environment
kubectl delete -k overlays/staging

# Delete entire namespace
kubectl delete namespace aixgo
```

## GKE-Specific Setup

### Create GKE Cluster

```bash
gcloud container clusters create aixgo-cluster \
  --region=us-central1 \
  --num-nodes=3 \
  --machine-type=n1-standard-4 \
  --enable-autoscaling \
  --min-nodes=3 \
  --max-nodes=10 \
  --enable-stackdriver-kubernetes \
  --enable-ip-alias \
  --enable-autoupgrade \
  --enable-autorepair
```

### Configure kubectl

```bash
gcloud container clusters get-credentials aixgo-cluster --region=us-central1
```

### Enable Workload Identity (Recommended)

```bash
gcloud iam service-accounts create aixgo-gsa

gcloud projects add-iam-policy-binding PROJECT_ID \
  --member="serviceAccount:aixgo-gsa@PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"

gcloud iam service-accounts add-iam-policy-binding aixgo-gsa@PROJECT_ID.iam.gserviceaccount.com \
  --role roles/iam.workloadIdentityUser \
  --member "serviceAccount:PROJECT_ID.svc.id.goog[aixgo/aixgo-sa]"

kubectl annotate serviceaccount aixgo-sa \
  iam.gke.io/gcp-service-account=aixgo-gsa@PROJECT_ID.iam.gserviceaccount.com \
  -n aixgo
```

## Additional Resources

- [Kubernetes Documentation](https://kubernetes.io/docs/home/)
- [Kustomize Documentation](https://kustomize.io/)
- [GKE Best Practices](https://cloud.google.com/kubernetes-engine/docs/best-practices)
