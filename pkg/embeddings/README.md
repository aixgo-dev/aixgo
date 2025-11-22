# Embeddings Package

The `embeddings` package provides a unified interface for generating text
embeddings from various providers. Embeddings are numerical representations of
text that capture semantic meaning, enabling similarity search and
retrieval-augmented generation (RAG).

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Supported Providers](#supported-providers)
- [API Reference](#api-reference)
- [Configuration](#configuration)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)
- [Examples](#examples)

## Overview

Embeddings transform text into high-dimensional vectors that capture semantic
relationships. Similar texts produce similar vectors, enabling:

- **Semantic Search**: Find documents by meaning, not just keywords
- **Similarity Matching**: Compare documents for relevance
- **Clustering**: Group related content
- **RAG Systems**: Retrieve relevant context for LLM prompts

This package abstracts embedding generation across providers, allowing you to
switch between OpenAI, HuggingFace, and self-hosted models without changing
your application code.

## Features

- **Multiple Providers**: OpenAI, HuggingFace Inference API, HuggingFace TEI (self-hosted)
- **Batch Processing**: Efficiently generate embeddings for multiple texts
- **Provider Agnostic**: Switch providers using configuration
- **Cost Optimization**: Use free HuggingFace models or pay-as-you-go OpenAI
- **Automatic Dimensions**: Provider reports embedding dimensions
- **Extensible**: Easy to add custom embedding providers via registry

## Installation

```bash
go get github.com/aixgo-dev/aixgo/pkg/embeddings
```

No additional dependencies required for the base package. Each provider handles its own HTTP client.

## Quick Start

### Using HuggingFace (Free)

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/aixgo-dev/aixgo/pkg/embeddings"
)

func main() {
    // Configure HuggingFace embeddings (no API key needed for public models)
    config := embeddings.Config{
        Provider: "huggingface",
        HuggingFace: &embeddings.HuggingFaceConfig{
            Model: "sentence-transformers/all-MiniLM-L6-v2",
            WaitForModel: true,
            UseCache: true,
        },
    }

    // Create embedding service
    svc, err := embeddings.New(config)
    if err != nil {
        log.Fatal(err)
    }
    defer svc.Close()

    // Generate embedding
    ctx := context.Background()
    embedding, err := svc.Embed(ctx, "Aixgo is an AI agent framework for Go")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Generated embedding with %d dimensions\n", len(embedding))
    fmt.Printf("Model: %s\n", svc.ModelName())
    fmt.Printf("Dimensions: %d\n", svc.Dimensions())
}
```

### Using OpenAI

```go
config := embeddings.Config{
    Provider: "openai",
    OpenAI: &embeddings.OpenAIConfig{
        APIKey: "sk-...", // Or use env: os.Getenv("OPENAI_API_KEY")
        Model:  "text-embedding-3-small",
    },
}

svc, err := embeddings.New(config)
if err != nil {
    log.Fatal(err)
}
defer svc.Close()
```

### Batch Processing

```go
texts := []string{
    "First document",
    "Second document",
    "Third document",
}

embeddings, err := svc.EmbedBatch(ctx, texts)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Generated %d embeddings\n", len(embeddings))
```

## Supported Providers

### Comparison Table

| Provider | Cost | Setup | Speed | Quality | Dimensions | Best For |
|----------|------|-------|-------|---------|------------|----------|
| **HuggingFace API** | Free | None | Medium | Good-Excellent | 384-1024 | Dev |
| **HuggingFace TEI** | Free (self-host) | Docker | Very Fast | Good-Excellent | 384-1024 | Prod |
| **OpenAI** | $0.02-0.13/1M | API Key | Fast | Excellent | 1536-3072 | Prod |

### HuggingFace Inference API

**Pros:**

- Completely free for public models
- No setup required
- Access to 100+ models
- Automatic model loading

**Cons:**

- Rate limited without API key
- Cold start delays
- Network latency

**Popular Models:**

| Model | Dimensions | Speed | Quality | Use Case |
|-------|-----------|-------|---------|----------|
| `sentence-transformers/all-MiniLM-L6-v2` | 384 | Very Fast | Good | Development, general purpose |
| `BAAI/bge-small-en-v1.5` | 384 | Fast | Good | Efficient search |
| `BAAI/bge-large-en-v1.5` | 1024 | Medium | Excellent | Production quality |
| `thenlper/gte-large` | 1024 | Medium | Excellent | Multilingual |
| `intfloat/e5-large-v2` | 1024 | Medium | Excellent | Retrieval tasks |

### HuggingFace TEI (Text Embeddings Inference)

Self-hosted embedding server optimized for performance.

**Pros:**

- Very fast (GPU acceleration)
- No rate limits
- Complete control
- Batch optimization

**Cons:**

- Requires deployment
- Hardware costs
- Maintenance overhead

**Setup:**

```bash
# Run TEI server with Docker
docker run -d \
  --name tei \
  -p 8080:8080 \
  -v $PWD/data:/data \
  --pull always \
  ghcr.io/huggingface/text-embeddings-inference:latest \
  --model-id sentence-transformers/all-MiniLM-L6-v2
```

### OpenAI

**Pros:**

- State-of-the-art quality
- Reliable API
- Fast response times
- No infrastructure

**Cons:**

- Costs money
- API key required
- Network dependency

**Pricing (as of 2025):**

- `text-embedding-3-small` (1536 dims): $0.02 per 1M tokens
- `text-embedding-3-large` (3072 dims): $0.13 per 1M tokens

## API Reference

### EmbeddingService Interface

```go
type EmbeddingService interface {
    // Embed generates embeddings for a single text
    Embed(ctx context.Context, text string) ([]float32, error)

    // EmbedBatch generates embeddings for multiple texts
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

    // Dimensions returns the dimension size of the embeddings
    Dimensions() int

    // ModelName returns the name of the embedding model
    ModelName() string

    // Close closes any resources held by the service
    Close() error
}
```

### Configuration Structures

#### OpenAI Config

```go
type OpenAIConfig struct {
    APIKey     string // Required: Your OpenAI API key
    Model      string // Required: Model name (e.g., "text-embedding-3-small")
    BaseURL    string // Optional: Custom endpoint (default: https://api.openai.com/v1)
    Dimensions int    // Optional: Reduce dimensions (text-embedding-3 only)
}
```

#### HuggingFace Config

```go
type HuggingFaceConfig struct {
    APIKey       string // Optional: For higher rate limits
    Model        string // Required: Model ID from HuggingFace Hub
    Endpoint     string // Optional: Custom endpoint (default: https://api-inference.huggingface.co)
    WaitForModel bool   // Optional: Wait if model is loading (default: false)
    UseCache     bool   // Optional: Use cached results (default: false)
}
```

#### HuggingFace TEI Config

```go
type HuggingFaceTEIConfig struct {
    Endpoint  string // Required: TEI server URL (e.g., "http://localhost:8080")
    Model     string // Optional: Model name (informational only)
    Normalize bool   // Optional: Return normalized embeddings (default: false)
}
```

## Configuration

### Environment Variables

```bash
# OpenAI
export OPENAI_API_KEY=sk-...

# HuggingFace (optional, for higher rate limits)
export HUGGINGFACE_API_KEY=hf_...
```

### YAML Configuration

```yaml
embeddings:
  provider: huggingface
  huggingface:
    model: sentence-transformers/all-MiniLM-L6-v2
    wait_for_model: true
    use_cache: true
    api_key: ${HUGGINGFACE_API_KEY}  # Optional
```

### Programmatic Configuration

```go
config := embeddings.Config{
    Provider: "openai",
    OpenAI: &embeddings.OpenAIConfig{
        APIKey: os.Getenv("OPENAI_API_KEY"),
        Model:  "text-embedding-3-small",
    },
}
```

## Best Practices

### 1. Choose the Right Provider

```go
// Development/Testing: Use HuggingFace (free)
config.Provider = "huggingface"
config.HuggingFace = &embeddings.HuggingFaceConfig{
    Model: "sentence-transformers/all-MiniLM-L6-v2",
}

// Production (Cost-Sensitive): Use HuggingFace TEI (self-hosted)
config.Provider = "huggingface_tei"
config.HuggingFaceTEI = &embeddings.HuggingFaceTEIConfig{
    Endpoint: "http://tei-service:8080",
}

// Production (Quality-Focused): Use OpenAI
config.Provider = "openai"
config.OpenAI = &embeddings.OpenAIConfig{
    APIKey: os.Getenv("OPENAI_API_KEY"),
    Model:  "text-embedding-3-small",
}
```

### 2. Use Batch Processing

```go
// Good: Batch processing (more efficient)
embeddings, err := svc.EmbedBatch(ctx, []string{
    "Document 1",
    "Document 2",
    "Document 3",
})

// Avoid: Individual calls in a loop
for _, text := range texts {
    emb, err := svc.Embed(ctx, text) // Inefficient
}
```

### 3. Handle Context and Timeouts

```go
// Set appropriate timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

embedding, err := svc.Embed(ctx, text)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        log.Printf("Embedding generation timeout")
    }
    return err
}
```

### 4. Validate Input

```go
func validateText(text string) error {
    if text == "" {
        return fmt.Errorf("text cannot be empty")
    }
    if len(text) > 8192 {
        return fmt.Errorf("text too long: %d chars (max 8192)", len(text))
    }
    return nil
}

if err := validateText(text); err != nil {
    return nil, err
}
```

### 5. Reuse Service Instances

```go
// Good: Reuse service instance
var embeddingService embeddings.EmbeddingService

func init() {
    var err error
    embeddingService, err = embeddings.New(config)
    if err != nil {
        log.Fatal(err)
    }
}

func getEmbedding(text string) ([]float32, error) {
    return embeddingService.Embed(context.Background(), text)
}

// Avoid: Creating new service for each call
func getEmbedding(text string) ([]float32, error) {
    svc, _ := embeddings.New(config) // Inefficient
    defer svc.Close()
    return svc.Embed(context.Background(), text)
}
```

### 6. Error Handling and Retries

```go
func embedWithRetry(svc embeddings.EmbeddingService, text string, maxRetries int) ([]float32, error) {
    var lastErr error
    for i := 0; i < maxRetries; i++ {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        embedding, err := svc.Embed(ctx, text)
        if err == nil {
            return embedding, nil
        }

        lastErr = err
        if !isRetryable(err) {
            break
        }

        // Exponential backoff
        time.Sleep(time.Duration(1<<uint(i)) * time.Second)
    }
    return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

func isRetryable(err error) bool {
    // Retry on network errors, rate limits, timeouts
    return errors.Is(err, context.DeadlineExceeded) ||
           strings.Contains(err.Error(), "rate limit") ||
           strings.Contains(err.Error(), "timeout")
}
```

## Troubleshooting

### HuggingFace Rate Limit

```text
Error: rate limit exceeded
```

**Solutions:**

1. Get a free API key from [HuggingFace](https://huggingface.co/settings/tokens):

```go
config.HuggingFace.APIKey = "hf_..."
```

2. Use batch processing to reduce requests
3. Deploy your own TEI server

### OpenAI Authentication Error

```text
Error: OpenAI API error: invalid API key
```

**Solution:** Verify your API key:

```bash
# Test your API key
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"
```

### Model Loading Timeout

```text
Error: model is currently loading
```

**Solution:** Enable wait for model:

```go
config.HuggingFace.WaitForModel = true
```

### Dimension Mismatch

If you're getting different dimensions than expected:

```go
// Check actual dimensions
fmt.Printf("Expected: %d, Got: %d\n", expectedDims, svc.Dimensions())

// Common dimensions by model:
// - all-MiniLM-L6-v2: 384
// - bge-large-en-v1.5: 1024
// - text-embedding-3-small: 1536
// - text-embedding-3-large: 3072
```

### TEI Connection Error

```text
Error: failed to connect to TEI server
```

**Solution:** Verify TEI is running:

```bash
# Check if TEI is accessible
curl http://localhost:8080/health

# View TEI logs
docker logs tei
```

## Examples

### Example 1: Semantic Search

```go
// Generate embeddings for documents
documents := []string{
    "Aixgo is a production-grade AI framework",
    "Go is a programming language created by Google",
    "Machine learning enables computers to learn from data",
}

docEmbeddings, err := svc.EmbedBatch(ctx, documents)
if err != nil {
    log.Fatal(err)
}

// Generate query embedding
query := "What is Aixgo?"
queryEmbedding, err := svc.Embed(ctx, query)
if err != nil {
    log.Fatal(err)
}

// Find most similar document (using cosine similarity)
bestIdx := 0
bestScore := float32(0.0)
for i, docEmb := range docEmbeddings {
    score := cosineSimilarity(queryEmbedding, docEmb)
    if score > bestScore {
        bestScore = score
        bestIdx = i
    }
}

fmt.Printf("Best match (%.2f): %s\n", bestScore, documents[bestIdx])
```

### Example 2: Caching Embeddings

```go
type EmbeddingCache struct {
    cache map[string][]float32
    mu    sync.RWMutex
    svc   embeddings.EmbeddingService
}

func (ec *EmbeddingCache) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
    // Check cache first
    ec.mu.RLock()
    if emb, ok := ec.cache[text]; ok {
        ec.mu.RUnlock()
        return emb, nil
    }
    ec.mu.RUnlock()

    // Generate embedding
    emb, err := ec.svc.Embed(ctx, text)
    if err != nil {
        return nil, err
    }

    // Cache result
    ec.mu.Lock()
    ec.cache[text] = emb
    ec.mu.Unlock()

    return emb, nil
}
```

### Example 3: Custom Provider

```go
package main

import "github.com/aixgo-dev/aixgo/pkg/embeddings"

func init() {
    // Register custom provider
    embeddings.Register("custom", func(config embeddings.Config) (embeddings.EmbeddingService, error) {
        return NewCustomEmbeddings(config)
    })
}

type CustomEmbeddings struct {
    model string
    dims  int
}

func (c *CustomEmbeddings) Embed(ctx context.Context, text string) ([]float32, error) {
    // Your implementation
    return make([]float32, c.dims), nil
}

func (c *CustomEmbeddings) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
    result := make([][]float32, len(texts))
    for i, text := range texts {
        emb, err := c.Embed(ctx, text)
        if err != nil {
            return nil, err
        }
        result[i] = emb
    }
    return result, nil
}

func (c *CustomEmbeddings) Dimensions() int      { return c.dims }
func (c *CustomEmbeddings) ModelName() string    { return c.model }
func (c *CustomEmbeddings) Close() error         { return nil }
```

### Example 4: Provider Comparison

```go
func compareProviders(text string) {
    providers := []embeddings.Config{
        {
            Provider: "huggingface",
            HuggingFace: &embeddings.HuggingFaceConfig{
                Model: "sentence-transformers/all-MiniLM-L6-v2",
            },
        },
        {
            Provider: "openai",
            OpenAI: &embeddings.OpenAIConfig{
                APIKey: os.Getenv("OPENAI_API_KEY"),
                Model:  "text-embedding-3-small",
            },
        },
    }

    for _, config := range providers {
        svc, err := embeddings.New(config)
        if err != nil {
            continue
        }
        defer svc.Close()

        start := time.Now()
        emb, err := svc.Embed(context.Background(), text)
        duration := time.Since(start)

        if err != nil {
            fmt.Printf("%s: ERROR - %v\n", config.Provider, err)
            continue
        }

        fmt.Printf("%s: %dms, %d dimensions\n",
            config.Provider, duration.Milliseconds(), len(emb))
    }
}
```

## Performance Considerations

### Latency Comparison

| Provider | Latency (avg) | Throughput |
|----------|---------------|------------|
| HuggingFace API | 200-500ms | Low |
| HuggingFace TEI | 10-50ms | Very High |
| OpenAI | 100-300ms | High |

### Optimization Tips

1. **Use batch operations** when processing multiple texts
2. **Cache embeddings** for frequently used content
3. **Deploy TEI locally** for production workloads
4. **Choose smaller models** (384 dims) if quality allows
5. **Set appropriate timeouts** to prevent hanging requests

### Cost Analysis

**Example: 1 million documents, 100 tokens each**

| Provider | Cost | Setup | Ongoing |
|----------|------|-------|---------|
| HuggingFace API | $0 | $0 | $0 |
| HuggingFace TEI | ~$100/month | $500 | $100/month (GPU VM) |
| OpenAI (text-embedding-3-small) | $2,000 | $0 | Pay per use |

## Model Selection Guide

### For Development

Use **HuggingFace API** with `sentence-transformers/all-MiniLM-L6-v2`:

- Free, no setup
- 384 dimensions (fast)
- Good quality for testing

### For Production: Cost-Sensitive

Use **HuggingFace TEI** with `BAAI/bge-large-en-v1.5`:

- Self-hosted (one-time setup)
- 1024 dimensions (excellent quality)
- No ongoing API costs

### For Production: Quality-Focused

Use **OpenAI** with `text-embedding-3-small`:

- Best-in-class quality
- 1536 dimensions
- Managed service
- $0.02 per 1M tokens

## Next Steps

- **Vector Storage**: See [pkg/vectorstore/README.md](../vectorstore/README.md)
- **RAG Implementation**: See [examples/rag-agent](../../examples/rag-agent)
- **Production Deployment**: See [guides/production-deployment.md](../../web/content/guides/production-deployment.md)

## Resources

- [OpenAI Embeddings Guide](https://platform.openai.com/docs/guides/embeddings)
- [HuggingFace Sentence Transformers](https://www.sbert.net/)
- [HuggingFace TEI Documentation](https://github.com/huggingface/text-embeddings-inference)
- [Embedding Comparison Benchmarks](https://huggingface.co/spaces/mteb/leaderboard)

## Contributing

To add a new embedding provider:

1. Implement the `EmbeddingService` interface
2. Register your provider in `init()`
3. Add configuration structs to the Config type
4. Add tests and documentation
5. Submit a pull request

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for details.
