# Docker Security Best Practices

This document outlines the security measures implemented in AIxGo Docker images and how to deploy them securely.

## Security Features Implemented

### 1. Non-Root User

All Docker images run as non-root user `aixgo:1000` to minimize attack surface.

**Implementation:**
```dockerfile
# Create non-root user
RUN addgroup -g 1000 -S aixgo && \
    adduser -u 1000 -S aixgo -G aixgo -h /app -s /sbin/nologin

# Drop privileges
USER aixgo:aixgo
```

**Benefits:**
- Limits damage from container escape vulnerabilities
- Prevents privilege escalation attacks
- Follows principle of least privilege

### 2. Minimal Base Image

Uses `alpine:3.19` (5MB) instead of full Ubuntu/Debian images (100MB+).

**Benefits:**
- Smaller attack surface (fewer installed packages)
- Faster image pulls and deployments
- Reduced vulnerability exposure

### 3. Multi-Stage Builds

Separates build environment from runtime environment.

**Benefits:**
- No build tools in final image
- No source code in final image
- Minimal runtime dependencies

### 4. Security Flags in Build

```dockerfile
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-w -s" \
    -o aixgo
```

**Flags:**
- `-trimpath`: Removes file system paths from binary (prevents info disclosure)
- `-ldflags="-w -s"`: Strips debug info and symbol table (reduces size and info disclosure)
- `CGO_ENABLED=0`: Creates fully static binary (no dynamic linking vulnerabilities)

### 5. Read-Only Root Filesystem

Configured in docker-compose.yml:

```yaml
read_only: true
tmpfs:
  - /tmp:noexec,nosuid,nodev,size=100m
```

**Benefits:**
- Prevents malware from persisting
- Prevents unauthorized file modifications
- Limits attack vectors

### 6. Capability Dropping

Removes all Linux capabilities, only adding back what's necessary:

```yaml
cap_drop:
  - ALL
# cap_add:
#   - NET_BIND_SERVICE  # Only if needed
```

**Benefits:**
- Minimizes kernel attack surface
- Prevents privilege escalation
- Follows least privilege principle

### 7. No New Privileges

```yaml
security_opt:
  - no-new-privileges:true
```

**Benefits:**
- Prevents setuid/setgid privilege escalation
- Blocks malicious privilege elevation

### 8. Resource Limits

```yaml
deploy:
  resources:
    limits:
      cpus: '2.0'
      memory: 2G
    reservations:
      cpus: '0.5'
      memory: 512M
```

**Benefits:**
- Prevents DoS attacks via resource exhaustion
- Ensures fair resource sharing
- Protects host system

### 9. Network Isolation

```yaml
networks:
  aixgo-network:
    driver: bridge
```

**Benefits:**
- Containers isolated from host network
- Internal-only communication possible
- Controlled exposure

### 10. Health Checks

```dockerfile
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/usr/local/bin/aixgo", "health"] || exit 1
```

**Benefits:**
- Automatic detection of unhealthy containers
- Automatic restart of failed services
- Improved availability

---

## Available Dockerfiles

### 1. `Dockerfile` (Scratch-based - Most Secure)

**Use when**: Maximum security is required, no debugging needed

**Features:**
- FROM scratch (0 bytes base image)
- Absolutely no shell or utilities
- Smallest possible attack surface
- Cannot be accessed with `docker exec`

**Limitations:**
- No shell for debugging
- No shell-form health check support (exec-form `HEALTHCHECK CMD ["/binary", "health"]` works, but cannot use shell commands or `||` syntax)
- Requires external monitoring or exec-form healthchecks

### 2. `Dockerfile.alpine` (Alpine-based - Recommended)

**Use when**: Need debugging tools and shell access

**Features:**
- FROM alpine:3.19 (5MB base image)
- Includes shell and basic utilities
- Health check support
- Can use `docker exec` for debugging

**Security:**
- Non-root user with no shell login
- Minimal package installation
- Security updates available

### 3. `docker/aixgo.Dockerfile` (Project-specific)

**Use when**: Building via docker-compose

**Features:**
- Optimized for docker-compose workflow
- Includes health checks
- Non-root user
- Security hardened

---

## Deployment Recommendations

### Production Deployment

```bash
# Build with Alpine (recommended)
docker build -f Dockerfile.alpine -t aixgo:latest .

# Run with security options
docker run -d \
  --name aixgo \
  --user 1000:1000 \
  --read-only \
  --tmpfs /tmp:noexec,nosuid,nodev,size=100m \
  --cap-drop=ALL \
  --security-opt=no-new-privileges:true \
  --memory=2g \
  --cpus=2.0 \
  -p 8080:8080 \
  aixgo:latest
```

### Docker Compose Deployment

```bash
# Use the provided docker-compose.yml
docker-compose up -d

# All security options are pre-configured
```

---

## Security Scanning

### Scan Images for Vulnerabilities

```bash
# Using Docker Scout (built-in)
docker scout cves aixgo:latest

# Using Trivy
trivy image aixgo:latest

# Using Snyk
snyk container test aixgo:latest
```

### Continuous Scanning

Add to CI/CD pipeline:

```yaml
# .github/workflows/security-scan.yml
- name: Run Trivy vulnerability scanner
  uses: aquasecurity/trivy-action@master
  with:
    image-ref: 'aixgo:latest'
    format: 'sarif'
    output: 'trivy-results.sarif'
    severity: 'CRITICAL,HIGH'
```

---

## Runtime Security Best Practices

### 1. Use Docker Content Trust

```bash
# Enable Docker Content Trust
export DOCKER_CONTENT_TRUST=1

# Pull and run images
docker pull aixgo:latest
```

### 2. Limit Container Restarts

```yaml
restart: unless-stopped  # Not "always"
```

### 3. Use Secrets Management

**Don't:**
```yaml
environment:
  - API_KEY=sk-1234567890abcdef  # BAD!
```

**Do (Docker Compose - Recommended):**
```yaml
# docker-compose.yml
version: "3.8"
services:
  aixgo:
    image: aixgo:latest
    secrets:
      - api_key
    environment:
      - API_KEY_FILE=/run/secrets/api_key

secrets:
  api_key:
    file: ./secrets/api_key.txt  # Local file (dev)
    # Or use external secret for production:
    # external: true
```

**Do (Docker Swarm):**
```bash
# Use Docker secrets (Swarm mode only)
echo "sk-1234567890abcdef" | docker secret create api_key -

docker service create \
  --secret api_key \
  aixgo:latest
```

### 4. Enable Logging

```yaml
logging:
  driver: "json-file"
  options:
    max-size: "10m"
    max-file: "3"
```

### 5. Network Policies

```yaml
networks:
  aixgo-network:
    driver: bridge
    internal: true  # No external access
```

---

## Security Checklist

Before deploying to production, verify:

- [ ] Image runs as non-root user
- [ ] Read-only root filesystem enabled
- [ ] All capabilities dropped (except required ones)
- [ ] No new privileges flag set
- [ ] Resource limits configured
- [ ] Health checks implemented
- [ ] Network isolation configured
- [ ] Secrets managed securely (not in environment variables)
- [ ] Logging enabled
- [ ] Image scanned for vulnerabilities
- [ ] Security updates applied
- [ ] Monitoring and alerting configured

---

## Vulnerability Response

### Regular Updates

```bash
# Rebuild images regularly (weekly)
docker build --no-cache -f Dockerfile.alpine -t aixgo:latest .

# Update base images
docker pull alpine:3.19
docker build -f Dockerfile.alpine -t aixgo:latest .
```

### Emergency Patching

If a critical vulnerability is discovered:

1. Update base image immediately
2. Rebuild all images
3. Run security scan
4. Deploy to staging
5. Test thoroughly
6. Deploy to production

---

## Monitoring and Alerting

### Recommended Metrics

- Container CPU usage
- Container memory usage
- Container restart count
- Failed health checks
- Security scan results
- Unauthorized access attempts

### Recommended Tools

- **Prometheus + Grafana**: Metrics and dashboards
- **Falco**: Runtime security monitoring
- **Sysdig**: Container security platform
- **Aqua Security**: Container security

---

## Additional Resources

- [Docker Security Best Practices](https://docs.docker.com/engine/security/)
- [CIS Docker Benchmark](https://www.cisecurity.org/benchmark/docker)
- [OWASP Docker Security Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Docker_Security_Cheat_Sheet.html)
- [Snyk Container Security](https://snyk.io/learn/container-security/)

---

## Contact

For security issues or questions:
- **Security Contact**: Create a confidential security issue via GitHub's [private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability) or email the project maintainers listed in CODEOWNERS
- **Response Time**: Security issues are typically triaged within 48 hours
- **Disclosure Policy**: We follow [responsible disclosure](https://cheatsheetseries.owasp.org/cheatsheets/Vulnerability_Disclosure_Cheat_Sheet.html) practices
