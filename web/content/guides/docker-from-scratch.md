---
title: 'Building Docker Images from Scratch'
description: "Create minimal 8MB production containers leveraging Aixgo's single binary advantage."
breadcrumb: 'Deployment'
category: 'Deployment'
weight: 7
---

One of Aixgo's most compelling advantages is deployment size. While Python AI frameworks produce 1GB+ containers, Aixgo agents compile to 8MB binaries. This guide shows how to
build minimal production containers.

## The Container Size Problem

Python-based AI frameworks create massive containers:

```dockerfile
# Python AI service - typical Dockerfile
FROM python:3.11
COPY requirements.txt .
RUN pip install -r requirements.txt

# Result: 1.2GB+ container
# - Python runtime: 900MB
# - Dependencies: 300MB+
# - Your code: <1MB
```

**Problems:**

- Slow deployments (minutes to pull 1GB)
- High storage costs
- Large attack surface
- Slow cold starts (30-45 seconds)

## Aixgo's Solution: FROM scratch

Go compiles to static binaries with zero runtime dependencies:

```dockerfile
# Aixgo service - minimal Dockerfile
FROM scratch
COPY aixgo-agent /
CMD ["/aixgo-agent"]

# Result: 8MB total container
# - Go binary: 8MB
# - No runtime needed
# - No dependencies
```

**Benefits:**

- Fast deployments (seconds to pull 8MB)
- Minimal storage costs
- Tiny attack surface
- Instant cold starts (<100ms)

## Basic Dockerfile: Single Binary

The simplest production Dockerfile:

```dockerfile
FROM golang:1.21 AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o agent main.go

# Runtime stage
FROM scratch

# Copy binary from builder
COPY --from=builder /app/agent /agent

# Copy config (optional)
COPY --from=builder /app/config/ /config/

CMD ["/agent"]
```

**Build and run:**

```bash
docker build -t aixgo-agent:latest .
docker run aixgo-agent:latest
```

**Result:** ~8MB container

## Optimized Dockerfile: Smallest Possible Image

Further optimization with build flags:

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -trimpath \
    -o agent \
    main.go

# Runtime: scratch (0MB base)
FROM scratch

# Copy CA certificates (for HTTPS)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /app/agent /agent

# Copy config
COPY --from=builder /app/config/ /config/

CMD ["/agent"]
```

**Build flags explained:**

- `CGO_ENABLED=0` - Disable C dependencies (pure Go)
- `-ldflags="-w -s"` - Strip debug info and symbol table
- `-trimpath` - Remove file system paths from binary
- Result: ~6-8MB container

## Multi-Stage Build Best Practices

### Stage 1: Builder (golang:alpine)

Use Alpine for smaller build stage:

```dockerfile
FROM golang:1.21-alpine AS builder

# Install build dependencies (if needed)
RUN apk add --no-cache git

WORKDIR /app

# Layer caching: dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Then copy source (changes more frequently)
COPY . .

# Build
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o agent main.go
```

### Stage 2: Runtime (scratch)

Minimal runtime with only essentials:

```dockerfile
FROM scratch

# Copy CA certificates for HTTPS API calls
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data (if needed)
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy binary
COPY --from=builder /app/agent /agent

# Copy config
COPY config/ /config/

# Non-root user (security best practice)
USER 65534:65534

CMD ["/agent"]
```

## Handling External Dependencies

### CA Certificates (for HTTPS)

If your agent calls external APIs:

```dockerfile
FROM scratch

# Required for HTTPS calls to LLM providers
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /app/agent /agent
CMD ["/agent"]
```

### Timezone Data

If your agent uses timezone-aware date/time:

```dockerfile
FROM scratch

# For time.LoadLocation()
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
ENV TZ=UTC

COPY --from=builder /app/agent /agent
CMD ["/agent"]
```

### Configuration Files

Mount configs as volumes or copy at build time:

```dockerfile
# Option 1: Copy at build time
COPY config/agents.yaml /config/agents.yaml

# Option 2: Mount as volume (more flexible)
# docker run -v ./config:/config aixgo-agent
```

## Size Comparison: Python vs Aixgo

Real-world example: data analysis agent

### Python (LangChain)

```dockerfile
FROM python:3.11

COPY requirements.txt .
RUN pip install -r requirements.txt

COPY . .

CMD ["python", "main.py"]
```

**requirements.txt:**

```text
langchain==0.1.0
openai==1.0.0
pandas==2.1.0
numpy==1.24.0
# ... 50+ more dependencies
```

**Result:**

- Base image: 900MB
- Dependencies: 400MB
- Total: **1.3GB**

### Aixgo

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o agent main.go

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/agent /agent
COPY --from=builder /app/config/ /config/
CMD ["/agent"]
```

**Result:**

- Base image: 0MB (scratch)
- Binary: 8MB
- Total: **8MB**

**Improvement: 150x smaller**

## Security Hardening

### Run as Non-Root User

```dockerfile
FROM scratch

# Copy binary
COPY --from=builder /app/agent /agent

# Copy passwd file for non-root user
COPY --from=builder /etc/passwd /etc/passwd

# Run as nobody (UID 65534)
USER 65534:65534

CMD ["/agent"]
```

### Read-Only Filesystem

```dockerfile
# Dockerfile
FROM scratch
COPY --from=builder /app/agent /agent
USER 65534:65534
CMD ["/agent"]
```

```bash
# Run with read-only root filesystem
docker run --read-only aixgo-agent:latest
```

### Minimal Attack Surface

`FROM scratch` has:

- No shell
- No package manager
- No utilities
- No OS files

Only your binary exists. Nothing else to exploit.

## Cloud-Specific Optimizations

### Google Cloud Run

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o agent main.go

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/agent /agent
COPY config/ /config/

# Cloud Run sets PORT env var
CMD ["/agent"]
```

**Deploy:**

```bash
gcloud run deploy aixgo-agent \
  --image gcr.io/my-project/aixgo-agent \
  --platform managed \
  --memory 512Mi \
  --max-instances 10
```

### AWS Lambda

For Lambda, use AWS Lambda Go runtime:

```dockerfile
FROM golang:1.21 AS builder
WORKDIR /app
COPY . .
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-w -s" -o bootstrap main.go

FROM scratch
COPY --from=builder /app/bootstrap /bootstrap
COPY config/ /config/
ENTRYPOINT ["/bootstrap"]
```

**Package and deploy:**

```bash
zip function.zip bootstrap config/*
aws lambda create-function \
  --function-name aixgo-agent \
  --runtime provided.al2 \
  --handler bootstrap \
  --zip-file fileb://function.zip
```

## Build Optimization Techniques

### Layer Caching

Order Dockerfile commands from least to most frequently changed:

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app

# 1. Dependencies (cached unless go.mod changes)
COPY go.mod go.sum ./
RUN go mod download

# 2. Source code (changes frequently)
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o agent main.go
```

### Multi-Platform Builds

Build for multiple architectures:

```bash
docker buildx create --use
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t my-registry/aixgo-agent:latest \
  --push \
  .
```

### Build-Time Variables

Inject version info at build time:

```dockerfile
ARG VERSION=dev
ARG GIT_COMMIT=unknown

RUN CGO_ENABLED=0 go build \
  -ldflags="-w -s -X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT}" \
  -o agent \
  main.go
```

```bash
docker build \
  --build-arg VERSION=1.0.0 \
  --build-arg GIT_COMMIT=$(git rev-parse HEAD) \
  -t aixgo-agent:1.0.0 \
  .
```

## Debugging Minimal Containers

### Problem: No Shell in scratch

You can't `docker exec` into a `scratch` container (no shell).

**Solution 1: Debug build with shell**

```dockerfile
# Production
FROM scratch AS production
COPY --from=builder /app/agent /agent
CMD ["/agent"]

# Debug (with shell)
FROM alpine:latest AS debug
COPY --from=builder /app/agent /agent
CMD ["/bin/sh"]
```

Build debug version:

```bash
docker build --target debug -t aixgo-agent:debug .
docker run -it aixgo-agent:debug /bin/sh
```

**Solution 2: Logging**

Use structured logging to stdout:

```go
import "log/slog"

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    slog.SetDefault(logger)

    slog.Info("Starting agent", "version", Version)
    // ...
}
```

View logs:

```bash
docker logs -f <container-id>
```

## Performance Impact

Container size affects:

| Metric               | 1.2GB (Python)     | 8MB (Aixgo)       | Impact          |
| -------------------- | ------------------ | ----------------- | --------------- |
| **Pull time**        | 2-5 minutes        | 5-10 seconds      | 24-60x faster   |
| **Cold start**       | 30-45 seconds      | <100ms            | 300-450x faster |
| **Storage cost**     | $0.10/GB/month     | $0.001/GB/month   | 100x cheaper    |
| **Deploy frequency** | Slow (discouraged) | Fast (encouraged) | Higher velocity |

## Real-World Example: Minimal Production Dockerfile

Complete production-ready Dockerfile:

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

# Install certificates
RUN apk add --no-cache ca-certificates git

WORKDIR /app

# Dependencies
COPY go.mod go.sum ./
RUN go mod download

# Source
COPY . .

# Build with all optimizations
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.Version=${VERSION:-dev}" \
    -trimpath \
    -o agent \
    main.go

# Runtime stage
FROM scratch

# Metadata
LABEL maintainer="your-team@company.com"
LABEL version="1.0.0"

# Copy certificates for HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy binary
COPY --from=builder /app/agent /agent

# Copy config
COPY config/ /config/

# Non-root user
USER 65534:65534

# Health check (if HTTP server)
HEALTHCHECK --interval=30s --timeout=3s \
  CMD ["/agent", "--health-check"]

CMD ["/agent"]
```

**Build:**

```bash
docker build -t my-registry/aixgo-agent:1.0.0 .
docker push my-registry/aixgo-agent:1.0.0
```

## Key Takeaways

1. **FROM scratch** - Smallest possible base (0MB)
2. **Multi-stage builds** - Keep builder separate from runtime
3. **Static binaries** - `CGO_ENABLED=0` for zero dependencies
4. **Strip symbols** - `-ldflags="-w -s"` reduces size
5. **Layer caching** - Dependencies before source code
6. **150x smaller** - 8MB vs 1.2GB Python containers

## Next Steps

- **[Production Deployment](/guides/production-deployment)** - Deploy minimal containers to production
- **[Single Binary vs Distributed](/guides/single-vs-distributed)** - Understand scaling patterns
- **[Observability & Monitoring](/guides/observability)** - Monitor containerized agents
