# Multi-stage build for minimal, secure image
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary with security flags
# -trimpath: Remove file system paths from binary
# -ldflags: Strip debug information
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-w -s" \
    -a -installsuffix cgo \
    -o aixgo ./cmd/orchestrator

# Final stage - minimal and secure
FROM alpine:3.19

# Install runtime dependencies only
RUN apk add --no-cache ca-certificates tzdata && \
    # Create non-root user and group
    addgroup -g 1000 -S aixgo && \
    adduser -u 1000 -S aixgo -G aixgo -h /app -s /sbin/nologin && \
    # Remove apk cache
    rm -rf /var/cache/apk/*

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/aixgo /usr/local/bin/aixgo

# Copy config directory (if needed)
# COPY config/ ./config/

# Change ownership
RUN chown -R aixgo:aixgo /app

# Security labels
LABEL org.opencontainers.image.title="AIxGo"
LABEL org.opencontainers.image.description="AIxGo Multi-Agent Framework"
LABEL org.opencontainers.image.vendor="AIxGo"
LABEL org.opencontainers.image.source="https://github.com/aixgo-dev/aixgo"

# Drop to non-root user
USER aixgo:aixgo

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/usr/local/bin/aixgo", "health"] || exit 1

# Run as non-root
ENTRYPOINT ["/usr/local/bin/aixgo"]
CMD ["config/agents.yaml"]
