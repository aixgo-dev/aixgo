# Multi-stage build for secure, minimal Docker image
# Stage 1: Builder
FROM golang:1.26-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary with security flags
# -trimpath: Remove file system paths from binary
# -ldflags: Strip debug information and set build info
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev') -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /build/aixgo \
    ./cmd/aixgo

# Stage 2: Runtime (minimal)
FROM scratch

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy SSL certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Create non-root user and group
# Note: In scratch, we need to create passwd/group files
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Copy the binary
COPY --from=builder /build/aixgo /usr/local/bin/aixgo

# Create app directory and set permissions
WORKDIR /app

# Use non-root user (UID 1000)
# Note: We'll use the 'nobody' user from the base image
USER nobody:nogroup

# Set security labels
LABEL org.opencontainers.image.title="AIxGo"
LABEL org.opencontainers.image.description="AIxGo Multi-Agent Framework"
LABEL org.opencontainers.image.vendor="AIxGo"
LABEL org.opencontainers.image.url="https://github.com/aixgo-dev/aixgo"
LABEL org.opencontainers.image.source="https://github.com/aixgo-dev/aixgo"
LABEL org.opencontainers.image.licenses="MIT"

# Security: Run as non-root user (nobody)
# Security: No shell available (scratch image)
# Security: Read-only root filesystem (set at runtime)

# Expose default port (if needed)
EXPOSE 8080

# Health check (optional, adjust based on your app)
# Note: scratch doesn't have shell, so health check must be done externally
# HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
#   CMD ["/usr/local/bin/aixgo", "health"]

# Run the application
ENTRYPOINT ["/usr/local/bin/aixgo"]
CMD ["--help"]
