# Aixgo Development and Deployment Tools

This directory contains Go-based development and deployment tools that replace shell scripts for improved cross-platform compatibility, maintainability, and type safety.

## Available Tools

### 0. Proto Code Generation Tool

**Location**: `cmd/tools/generate-proto/main.go`

**Replaces**: `proto/mcp/generate.sh`

Automates Go code generation from Protocol Buffer definitions.

#### Features

- Automatic installation of required protoc plugins
- Project root detection
- Cross-platform compatibility
- Verbose and dry-run modes
- Proper error handling and validation

#### Usage

```bash
# Basic usage
go run cmd/tools/generate-proto/main.go

# Custom proto file
go run cmd/tools/generate-proto/main.go -proto proto/custom/myproto.proto

# Verbose mode with dry-run
go run cmd/tools/generate-proto/main.go -verbose -dry-run

# Skip plugin installation
go run cmd/tools/generate-proto/main.go -install=false
```

#### Flags

| Flag       | Default               | Description                       |
| ---------- | --------------------- | --------------------------------- |
| `-proto`   | `proto/mcp/mcp.proto` | Path to proto file                |
| `-install` | `true`                | Install protoc plugins if missing |
| `-verbose` | `false`               | Enable verbose output             |
| `-dry-run` | `false`               | Show commands without executing   |

#### Prerequisites

- **protoc**: Protocol Buffers compiler
- **Go 1.23+**: For running the tool

The tool will automatically install:

- `protoc-gen-go`
- `protoc-gen-go-grpc`

#### Examples

```bash
# Generate with defaults
go run cmd/tools/generate-proto/main.go

# Test without executing
go run cmd/tools/generate-proto/main.go -dry-run -verbose

# Build binary
go build -o bin/generate-proto cmd/tools/generate-proto/main.go
./bin/generate-proto
```

---

### 1. Cloud Run Deployment Tool

**Location**: `cmd/deploy/cloudrun/main.go`

Automates deployment of aixgo services to Google Cloud Run.

#### Features

- Automated GCP project setup and API enablement
- Artifact Registry repository creation
- Docker image building and pushing
- Service account creation with proper IAM roles
- Secret Manager integration for API keys
- Cloud Run service deployment
- Health check validation

#### Usage

```bash
# Basic deployment
go run cmd/deploy/cloudrun/main.go \
  -project my-gcp-project \
  -region us-central1

# With environment variables
export GCP_PROJECT_ID="my-project"
export GCP_REGION="us-central1"
export XAI_API_KEY="your-key"
export OPENAI_API_KEY="your-key"
export HUGGINGFACE_API_KEY="your-key"

go run cmd/deploy/cloudrun/main.go

# Skip certain steps
go run cmd/deploy/cloudrun/main.go \
  -project my-project \
  -skip-build \
  -skip-secrets

# Deploy to staging
go run cmd/deploy/cloudrun/main.go \
  -project my-project \
  -env staging \
  -service aixgo-mcp-staging
```

#### Flags

| Flag                     | Environment Variable  | Default       | Description                       |
| ------------------------ | --------------------- | ------------- | --------------------------------- |
| `-project`               | `GCP_PROJECT_ID`      | (required)    | GCP Project ID                    |
| `-region`                | `GCP_REGION`          | `us-central1` | GCP Region                        |
| `-service`               | -                     | `aixgo-mcp`   | Cloud Run service name            |
| `-image`                 | -                     | `mcp-server`  | Container image name              |
| `-repository`            | -                     | `aixgo`       | Artifact Registry repository      |
| `-env`                   | -                     | `production`  | Environment (staging, production) |
| `-xai-key`               | `XAI_API_KEY`         | -             | XAI API Key                       |
| `-openai-key`            | `OPENAI_API_KEY`      | -             | OpenAI API Key                    |
| `-huggingface-key`       | `HUGGINGFACE_API_KEY` | -             | HuggingFace API Key               |
| `-skip-build`            | -                     | `false`       | Skip Docker build and push        |
| `-skip-secrets`          | -                     | `false`       | Skip secret creation              |
| `-skip-deploy`           | -                     | `false`       | Skip Cloud Run deployment         |
| `-allow-unauthenticated` | -                     | `true`        | Allow unauthenticated access      |

#### Examples

**Production Deployment**:

```bash
go run cmd/deploy/cloudrun/main.go \
  -project my-production-project \
  -region us-central1 \
  -env production \
  -xai-key "${XAI_API_KEY}" \
  -openai-key "${OPENAI_API_KEY}" \
  -huggingface-key "${HUGGINGFACE_API_KEY}"
```

**Staging Deployment**:

```bash
go run cmd/deploy/cloudrun/main.go \
  -project my-staging-project \
  -region us-east1 \
  -env staging \
  -service aixgo-mcp-staging
```

**Build Only** (no deployment):

```bash
go run cmd/deploy/cloudrun/main.go \
  -project my-project \
  -skip-deploy
```

**Deploy Only** (using existing image):

```bash
go run cmd/deploy/cloudrun/main.go \
  -project my-project \
  -skip-build \
  -skip-secrets
```

---

### 2. Kubernetes Deployment Tool

**Location**: `cmd/deploy/k8s/main.go`

Automates deployment of aixgo services to Kubernetes (GKE, EKS, AKS, or self-managed).

#### Features

- GCP authentication and GKE cluster credential management
- Multi-platform Docker image building (amd64, arm64)
- Artifact Registry integration
- Kubernetes namespace and secret creation
- Kustomize-based deployment with environment overlays
- Deployment rollout monitoring
- Automated smoke tests with port forwarding
- Deployment verification

#### Usage

```bash
# Deploy to staging
go run cmd/deploy/k8s/main.go \
  -project my-gcp-project \
  -cluster aixgo-cluster \
  -env staging

# Deploy to production
go run cmd/deploy/k8s/main.go \
  -project my-gcp-project \
  -cluster aixgo-cluster \
  -env production

# With custom image tag
go run cmd/deploy/k8s/main.go \
  -project my-project \
  -cluster my-cluster \
  -tag v1.2.3 \
  -env production

# Skip build and tests
go run cmd/deploy/k8s/main.go \
  -project my-project \
  -cluster my-cluster \
  -skip-build \
  -skip-tests
```

#### Flags

| Flag               | Environment Variable  | Default         | Description                       |
| ------------------ | --------------------- | --------------- | --------------------------------- |
| `-project`         | `GCP_PROJECT_ID`      | (required)      | GCP Project ID                    |
| `-region`          | `GCP_REGION`          | `us-central1`   | GCP Region                        |
| `-zone`            | `GKE_ZONE`            | `us-central1`   | GKE Zone                          |
| `-cluster`         | `GKE_CLUSTER`         | `aixgo-cluster` | GKE Cluster name                  |
| `-env`             | -                     | `staging`       | Environment (staging, production) |
| `-registry`        | -                     | (auto-detected) | Container registry URL            |
| `-tag`             | `IMAGE_TAG`           | `latest`        | Container image tag               |
| `-xai-key`         | `XAI_API_KEY`         | -               | XAI API Key                       |
| `-openai-key`      | `OPENAI_API_KEY`      | -               | OpenAI API Key                    |
| `-huggingface-key` | `HUGGINGFACE_API_KEY` | -               | HuggingFace API Key               |
| `-skip-build`      | -                     | `false`         | Skip Docker build and push        |
| `-skip-secrets`    | -                     | `false`         | Skip secret creation              |
| `-skip-deploy`     | -                     | `false`         | Skip Kubernetes deployment        |
| `-skip-tests`      | -                     | `false`         | Skip smoke tests                  |

#### Examples

**Full Production Deployment**:

```bash
go run cmd/deploy/k8s/main.go \
  -project my-production-project \
  -cluster production-cluster \
  -zone us-central1-a \
  -env production \
  -tag v1.2.3 \
  -xai-key "${XAI_API_KEY}" \
  -openai-key "${OPENAI_API_KEY}" \
  -huggingface-key "${HUGGINGFACE_API_KEY}"
```

**Staging Deployment**:

```bash
go run cmd/deploy/k8s/main.go \
  -project my-staging-project \
  -cluster staging-cluster \
  -env staging \
  -tag latest
```

**Update Deployment** (using existing images):

```bash
go run cmd/deploy/k8s/main.go \
  -project my-project \
  -cluster my-cluster \
  -env production \
  -skip-build \
  -skip-secrets
```

**Build and Push Only** (no deployment):

```bash
go run cmd/deploy/k8s/main.go \
  -project my-project \
  -tag v1.2.3 \
  -skip-deploy
```

**Quick Deploy** (skip tests):

```bash
go run cmd/deploy/k8s/main.go \
  -project my-project \
  -cluster my-cluster \
  -skip-tests
```

---

## Building the Tools

### Build for Current Platform

```bash
# Proto generation tool
go build -o bin/generate-proto cmd/tools/generate-proto/main.go

# Cloud Run tool
go build -o bin/deploy-cloudrun cmd/deploy/cloudrun/main.go

# Kubernetes tool
go build -o bin/deploy-k8s cmd/deploy/k8s/main.go
```

### Build All Tools

```bash
# Build all development and deployment tools
go build -o bin/generate-proto cmd/tools/generate-proto/main.go
go build -o bin/deploy-cloudrun cmd/deploy/cloudrun/main.go
go build -o bin/deploy-k8s cmd/deploy/k8s/main.go
```

### Build for All Platforms

```bash
# Proto generation tool
GOOS=linux GOARCH=amd64 go build -o bin/generate-proto-linux-amd64 cmd/tools/generate-proto/main.go
GOOS=linux GOARCH=arm64 go build -o bin/generate-proto-linux-arm64 cmd/tools/generate-proto/main.go
GOOS=darwin GOARCH=amd64 go build -o bin/generate-proto-darwin-amd64 cmd/tools/generate-proto/main.go
GOOS=darwin GOARCH=arm64 go build -o bin/generate-proto-darwin-arm64 cmd/tools/generate-proto/main.go
GOOS=windows GOARCH=amd64 go build -o bin/generate-proto-windows-amd64.exe cmd/tools/generate-proto/main.go

# Cloud Run deployment tool
GOOS=linux GOARCH=amd64 go build -o bin/deploy-cloudrun-linux-amd64 cmd/deploy/cloudrun/main.go
GOOS=linux GOARCH=arm64 go build -o bin/deploy-cloudrun-linux-arm64 cmd/deploy/cloudrun/main.go
GOOS=darwin GOARCH=amd64 go build -o bin/deploy-cloudrun-darwin-amd64 cmd/deploy/cloudrun/main.go
GOOS=darwin GOARCH=arm64 go build -o bin/deploy-cloudrun-darwin-arm64 cmd/deploy/cloudrun/main.go
GOOS=windows GOARCH=amd64 go build -o bin/deploy-cloudrun-windows-amd64.exe cmd/deploy/cloudrun/main.go

# Kubernetes deployment tool
GOOS=linux GOARCH=amd64 go build -o bin/deploy-k8s-linux-amd64 cmd/deploy/k8s/main.go
GOOS=linux GOARCH=arm64 go build -o bin/deploy-k8s-linux-arm64 cmd/deploy/k8s/main.go
GOOS=darwin GOARCH=amd64 go build -o bin/deploy-k8s-darwin-amd64 cmd/deploy/k8s/main.go
GOOS=darwin GOARCH=arm64 go build -o bin/deploy-k8s-darwin-arm64 cmd/deploy/k8s/main.go
GOOS=windows GOARCH=amd64 go build -o bin/deploy-k8s-windows-amd64.exe cmd/deploy/k8s/main.go
```

---

## Integration with CI/CD

### GitHub Actions

The tools are integrated into GitHub Actions workflows for automated deployments:

#### Cloud Run Deployment

```yaml
- name: Deploy to Cloud Run
  run: |
    go run cmd/deploy/cloudrun/main.go \
      -project ${{ secrets.GCP_PROJECT_ID }} \
      -region us-central1 \
      -env ${{ github.event.inputs.environment }} \
      -xai-key ${{ secrets.XAI_API_KEY }} \
      -openai-key ${{ secrets.OPENAI_API_KEY }} \
      -huggingface-key ${{ secrets.HUGGINGFACE_API_KEY }}
```

#### Kubernetes Deployment

```yaml
- name: Deploy to Kubernetes
  run: |
    go run cmd/deploy/k8s/main.go \
      -project ${{ secrets.GCP_PROJECT_ID }} \
      -cluster ${{ env.GKE_CLUSTER }} \
      -zone ${{ env.GKE_ZONE }} \
      -env ${{ github.event.inputs.environment }} \
      -tag ${{ github.sha }} \
      -xai-key ${{ secrets.XAI_API_KEY }} \
      -openai-key ${{ secrets.OPENAI_API_KEY }} \
      -huggingface-key ${{ secrets.HUGGINGFACE_API_KEY }}
```

### Makefile Integration

Add these targets to your Makefile:

```makefile
.PHONY: generate-proto build-tools deploy-cloudrun deploy-k8s

# Development tools
generate-proto: ## Generate Go code from protobuf
	@go run cmd/tools/generate-proto/main.go

# Build all tools
build-tools: ## Build all development and deployment tools
	@echo "Building development tools..."
	@mkdir -p bin
	@go build -o bin/generate-proto cmd/tools/generate-proto/main.go
	@echo "Building deployment tools..."
	@go build -o bin/deploy-cloudrun cmd/deploy/cloudrun/main.go
	@go build -o bin/deploy-k8s cmd/deploy/k8s/main.go
	@echo "All tools built successfully!"

# Cloud Run deployment
deploy-cloudrun: ## Deploy to Cloud Run
	@go run cmd/deploy/cloudrun/main.go \
		-project $(GCP_PROJECT_ID) \
		-region $(GCP_REGION)

deploy-cloudrun-staging: ## Deploy to Cloud Run staging
	@go run cmd/deploy/cloudrun/main.go \
		-project $(GCP_PROJECT_ID) \
		-region $(GCP_REGION) \
		-env staging

deploy-cloudrun-production: ## Deploy to Cloud Run production
	@go run cmd/deploy/cloudrun/main.go \
		-project $(GCP_PROJECT_ID) \
		-region $(GCP_REGION) \
		-env production

# Kubernetes deployment
deploy-k8s: ## Deploy to Kubernetes
	@go run cmd/deploy/k8s/main.go \
		-project $(GCP_PROJECT_ID) \
		-cluster $(GKE_CLUSTER) \
		-env $(ENVIRONMENT)

deploy-k8s-staging: ## Deploy to Kubernetes staging
	@go run cmd/deploy/k8s/main.go \
		-project $(GCP_PROJECT_ID) \
		-cluster $(GKE_CLUSTER) \
		-env staging

deploy-k8s-production: ## Deploy to Kubernetes production
	@go run cmd/deploy/k8s/main.go \
		-project $(GCP_PROJECT_ID) \
		-cluster $(GKE_CLUSTER) \
		-env production
```

---

## Prerequisites

### Required Tools

#### Core Requirements (All Tools)

- **Go 1.23+**: For running the tools

#### Proto Code Generation Tool

- **protoc**: Protocol Buffers compiler

The tool will automatically install these Go packages:

- `protoc-gen-go`
- `protoc-gen-go-grpc`

#### Deployment Tools

All deployment tools require:

- **Docker**: For building container images
- **gcloud**: Google Cloud SDK for GCP operations

#### Cloud Run Tool Additional Requirements

- **curl**: For health check testing

#### Kubernetes Tool Additional Requirements

- **kubectl**: Kubernetes CLI
- **kubectl kustomize**: For kustomize support (built into kubectl 1.14+)

### Installation

#### macOS

```bash
# Install Go
brew install go

# Install protoc (for proto code generation)
brew install protobuf

# Install Docker
brew install --cask docker

# Install gcloud
brew install --cask google-cloud-sdk

# Install kubectl
brew install kubectl

# Install curl (usually pre-installed)
brew install curl
```

#### Linux (Ubuntu/Debian)

```bash
# Install Go
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Install protoc (for proto code generation)
sudo apt-get update
sudo apt-get install -y protobuf-compiler

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Install gcloud
curl https://sdk.cloud.google.com | bash
exec -l $SHELL

# Install kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# Install curl
sudo apt-get install curl
```

#### Windows

```powershell
# Install Go using installer from https://go.dev/dl/

# Install protoc (for proto code generation)
# Download from https://github.com/protocolbuffers/protobuf/releases
# Or use Chocolatey:
choco install protoc

# Install Docker Desktop from https://www.docker.com/products/docker-desktop

# Install gcloud using installer from https://cloud.google.com/sdk/docs/install

# Install kubectl
gcloud components install kubectl

# curl is available in PowerShell 7+ or install from https://curl.se/windows/
```

---

## Environment Variables

Both tools support environment variables for sensitive data:

### Common Variables

```bash
# Required
export GCP_PROJECT_ID="my-gcp-project"

# Optional
export GCP_REGION="us-central1"
export XAI_API_KEY="your-xai-api-key"
export OPENAI_API_KEY="your-openai-api-key"
export HUGGINGFACE_API_KEY="your-huggingface-api-key"
```

### Kubernetes-Specific Variables

```bash
export GKE_CLUSTER="aixgo-cluster"
export GKE_ZONE="us-central1-a"
export IMAGE_TAG="v1.2.3"
```

### Using .env File

Create a `.env` file (never commit this):

```bash
# .env
GCP_PROJECT_ID=my-project
GCP_REGION=us-central1
XAI_API_KEY=your-key-here
OPENAI_API_KEY=your-key-here
HUGGINGFACE_API_KEY=your-key-here
GKE_CLUSTER=aixgo-cluster
GKE_ZONE=us-central1-a
```

Load environment variables:

```bash
# Bash/Zsh
source .env
go run cmd/deploy/cloudrun/main.go

# Or use a tool like direnv
echo 'source .env' > .envrc
direnv allow
```

---

## Error Handling

Both tools include comprehensive error handling and validation:

### Common Error Messages

**"Project ID is required"**

- Solution: Set `-project` flag or `GCP_PROJECT_ID` environment variable

**"gcloud CLI not found"**

- Solution: Install Google Cloud SDK

**"docker not found"**

- Solution: Install Docker

**"Repository already exists"**

- This is a warning, not an error. The tool continues with the existing repository.

**"Health check failed"**

- Solution: Check Cloud Run logs or Kubernetes pod logs for application errors

### Debugging

Enable verbose output by examining stdout/stderr. The tools output colored logs:

- **[INFO]** (Green): Normal operation
- **[WARN]** (Yellow): Non-fatal issues
- **[ERROR]** (Red): Fatal errors

---

## Security Best Practices

1. **Never commit API keys** to version control
2. **Use Secret Manager** for production secrets (Cloud Run tool does this automatically)
3. **Use Workload Identity** for GKE deployments
4. **Rotate secrets regularly**
5. **Use least-privilege IAM roles**
6. **Enable authentication** for production deployments:
   ```bash
   go run cmd/deploy/cloudrun/main.go -allow-unauthenticated=false
   ```

---

## Troubleshooting

### Cloud Run Tool

**Issue**: Image build fails

```bash
# Check Docker daemon is running
docker ps

# Check Dockerfile exists
ls -la docker/aixgo.Dockerfile
```

**Issue**: Deployment succeeds but health check fails

```bash
# Check Cloud Run logs
gcloud run services logs read aixgo-mcp --region us-central1 --limit 50

# Test locally
docker build -t test -f docker/aixgo.Dockerfile .
docker run -p 8080:8080 test
curl http://localhost:8080/health/live
```

### Kubernetes Tool

**Issue**: Cannot connect to cluster

```bash
# Verify cluster credentials
kubectl cluster-info

# Re-authenticate
gcloud container clusters get-credentials CLUSTER_NAME --zone ZONE
```

**Issue**: Pods not starting

```bash
# Check pod status
kubectl get pods -n aixgo
kubectl describe pod POD_NAME -n aixgo
kubectl logs POD_NAME -n aixgo
```

**Issue**: Image pull errors

```bash
# Verify image exists
gcloud artifacts docker images list REGION-docker.pkg.dev/PROJECT/REPO

# Check service account permissions
kubectl describe serviceaccount aixgo-sa -n aixgo
```

---

## Migration from Shell Scripts

All shell scripts have been replaced with Go tools for better cross-platform compatibility and maintainability.

### Proto Code Generation

**Before (Shell Script)**:

```bash
cd proto/mcp
./generate.sh
```

**After (Go Tool)**:

```bash
go run cmd/tools/generate-proto/main.go
```

### Cloud Run Deployment

**Before (Shell Script)**:

```bash
cd deploy/cloudrun
./deploy.sh
```

**After (Go Tool)**:

```bash
go run cmd/deploy/cloudrun/main.go -project my-project
```

### Migration Summary

| Shell Script                | Go Tool                            | Status      |
| --------------------------- | ---------------------------------- | ----------- |
| `proto/mcp/generate.sh`     | `cmd/tools/generate-proto/main.go` | ✅ Complete |
| `deploy/cloudrun/deploy.sh` | `cmd/deploy/cloudrun/main.go`      | ✅ Complete |

### Benefits

1. **Cross-platform**: Works on Windows, macOS, and Linux
2. **Type safety**: Compile-time error checking
3. **Better error handling**: Structured error messages with context
4. **Easier testing**: Can be unit tested
5. **Maintainability**: Easier to extend and modify
6. **No shell dependencies**: Only requires Go and specific tools (protoc, gcloud, etc.)
7. **Consistent UX**: All tools follow the same flag patterns and output formatting
8. **Dry-run mode**: Preview commands before execution
9. **Verbose mode**: Debug output when needed

---

## Contributing

When adding new deployment tools:

1. Create a new directory under `cmd/deploy/`
2. Follow the existing pattern for flags and error handling
3. Include colored logging for better UX
4. Support environment variables for all sensitive data
5. Add comprehensive flag documentation
6. Update this README with usage examples

---

## Additional Resources

- [Cloud Run Documentation](https://cloud.google.com/run/docs)
- [GKE Documentation](https://cloud.google.com/kubernetes-engine/docs)
- [Artifact Registry Documentation](https://cloud.google.com/artifact-registry/docs)
- [Kustomize Documentation](https://kustomize.io/)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
