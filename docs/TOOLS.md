# Aixgo Development Tools Quick Reference

This guide provides quick commands for using the Go-based development tools that replace shell scripts.

## Quick Start

All shell scripts have been replaced with Go tools for better cross-platform compatibility.

### Proto Code Generation

**Old way**:
```bash
cd proto/mcp
./generate.sh
```

**New way**:
```bash
go run cmd/tools/generate-proto/main.go
```

### Cloud Run Deployment

**Old way**:
```bash
cd deploy/cloudrun
./deploy.sh
```

**New way**:
```bash
go run cmd/deploy/cloudrun/main.go -project my-project
```

## Tool Locations

| Tool | Location | Purpose |
|------|----------|---------|
| **generate-proto** | `cmd/tools/generate-proto/main.go` | Generate Go code from protobuf |
| **cloudrun-deploy** | `cmd/deploy/cloudrun/main.go` | Deploy to Google Cloud Run |

## Common Commands

### Proto Code Generation

```bash
# Generate with defaults
go run cmd/tools/generate-proto/main.go

# Verbose mode
go run cmd/tools/generate-proto/main.go -verbose

# Dry-run (preview only)
go run cmd/tools/generate-proto/main.go -dry-run -verbose

# Custom proto file
go run cmd/tools/generate-proto/main.go -proto proto/custom/myproto.proto

# Get help
go run cmd/tools/generate-proto/main.go -h
```

### Cloud Run Deployment

```bash
# Basic deployment
go run cmd/deploy/cloudrun/main.go -project my-project

# With environment
go run cmd/deploy/cloudrun/main.go \
  -project my-project \
  -env production

# Skip build (use existing image)
go run cmd/deploy/cloudrun/main.go \
  -project my-project \
  -skip-build

# Skip secrets
go run cmd/deploy/cloudrun/main.go \
  -project my-project \
  -skip-secrets

# Dry-run mode
go run cmd/deploy/cloudrun/main.go \
  -project my-project \
  -dry-run -verbose

# Get help
go run cmd/deploy/cloudrun/main.go -h
```

## Environment Variables

Set these for easier usage:

```bash
# Required for deployment
export GCP_PROJECT_ID="my-gcp-project"

# Optional
export GCP_REGION="us-central1"
export XAI_API_KEY="your-xai-api-key"
export OPENAI_API_KEY="your-openai-api-key"
export HUGGINGFACE_API_KEY="your-huggingface-api-key"
```

Then run tools without flags:
```bash
go run cmd/deploy/cloudrun/main.go
```

## Building Binaries

Build once, run anywhere:

```bash
# Build all tools
mkdir -p bin
go build -o bin/generate-proto cmd/tools/generate-proto/main.go
go build -o bin/cloudrun-deploy cmd/deploy/cloudrun/main.go

# Use binaries
./bin/generate-proto
./bin/cloudrun-deploy -project my-project
```

## Common Flags

All tools support these flags:

| Flag | Description |
|------|-------------|
| `-verbose` | Show detailed output |
| `-dry-run` | Preview commands without executing |
| `-h` or `-help` | Show help message |

## Prerequisites

### For Proto Generation
- Go 1.23+
- protoc (Protocol Buffers compiler)

Install protoc:
```bash
# macOS
brew install protobuf

# Linux
sudo apt-get install protobuf-compiler

# Windows
choco install protoc
```

### For Cloud Run Deployment
- Go 1.23+
- Docker
- gcloud CLI
- curl

## Troubleshooting

### "protoc not found"
Install Protocol Buffers compiler (see Prerequisites above).

### "Project ID is required"
Set the `-project` flag or `GCP_PROJECT_ID` environment variable.

### "gcloud CLI not found"
Install Google Cloud SDK: https://cloud.google.com/sdk/docs/install

### "docker not found"
Install Docker: https://docs.docker.com/get-docker/

## Full Documentation

For complete documentation, see:
- `./cmd/tools/README.md` - Full tool documentation
- Run any tool with `-h` flag for help

## Migration Notes

### Benefits of Go Tools

1. **Cross-platform**: Works on Windows, macOS, and Linux
2. **Type safety**: Compile-time error checking
3. **Better error handling**: Clear, structured error messages
4. **Testable**: Can be unit tested
5. **Maintainable**: Easier to extend and modify
6. **No shell dependencies**: Only requires Go and specific tools

### Shell Scripts Status

| Shell Script | Status | Replacement |
|--------------|--------|-------------|
| `proto/mcp/generate.sh` | ✅ Replaced | `cmd/tools/generate-proto/main.go` |
| `deploy/cloudrun/deploy.sh` | ✅ Replaced | `cmd/deploy/cloudrun/main.go` |

## Examples

### Complete Proto Generation Workflow

```bash
# 1. Generate protobuf code
go run cmd/tools/generate-proto/main.go -verbose

# 2. Verify generated files
ls -la proto/mcp/*.pb.go
```

### Complete Deployment Workflow

```bash
# 1. Set environment variables
export GCP_PROJECT_ID="my-project"
export XAI_API_KEY="your-key"
export OPENAI_API_KEY="your-key"
export HUGGINGFACE_API_KEY="your-key"

# 2. Deploy to staging (dry-run first)
go run cmd/deploy/cloudrun/main.go \
  -env staging \
  -dry-run -verbose

# 3. Deploy for real
go run cmd/deploy/cloudrun/main.go \
  -env staging

# 4. Deploy to production
go run cmd/deploy/cloudrun/main.go \
  -env production
```

## Tips

1. **Use dry-run mode** to preview commands before execution
2. **Enable verbose mode** for debugging
3. **Build binaries** for faster execution
4. **Set environment variables** to avoid repeating flags
5. **Check help output** with `-h` flag for all options

## Need Help?

- Run any tool with `-h` for detailed help
- Check `./cmd/tools/README.md` for full documentation
- Review tool source code for implementation details
