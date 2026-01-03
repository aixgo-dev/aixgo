---
title: 'Embeddings in Aixgo'
description: 'Complete guide to choosing and using embedding providers for RAG, semantic search, and similarity matching'
weight: 5
---

# Embeddings in Aixgo

This guide provides a comprehensive reference for working with embeddings in Aixgo, covering provider selection, configuration, optimization, and production deployment strategies.

## Overview

### What Are Embeddings?

Embeddings are dense numerical representations that capture the semantic meaning of text. Similar texts produce similar vectors, enabling powerful AI applications:

- **Semantic Search**: Find content by meaning, not keywords
- **RAG Systems**: Power retrieval-augmented generation
- **Similarity Matching**: Compare and cluster documents
- **Duplicate Detection**: Identify similar or redundant content
- **Recommendation Systems**: Suggest related items

**Example:**
```text
"machine learning" → [0.42, 0.78, 0.11, ...]
"deep learning"    → [0.41, 0.79, 0.12, ...] (similar vector)
"cooking recipe"   → [0.89, 0.12, 0.05, ...] (different vector)
```

### Why Embeddings Matter

Embeddings bridge the gap between human language and machine computation, enabling:

1. **Context-Aware Search**: Find "automobile" when searching for "car"
2. **Cross-Language Understanding**: Match concepts across languages
3. **Efficient Storage**: Compress semantic information into fixed-size vectors
4. **Fast Similarity Computation**: Use cosine similarity for quick comparisons
5. **Knowledge Retrieval**: Power RAG systems with relevant context

### Quick Comparison

| Provider | Cost | Quality | Speed | Best For |
|----------|------|---------|-------|----------|
| **HuggingFace API** | Free (rate limited) | Good-Excellent | Medium | Development, prototyping |
| **HuggingFace TEI** | Free (self-host) | Good-Excellent | Very Fast | High-volume production |
| **OpenAI** | $0.02-0.13/1M tokens | Excellent | Fast | Production quality |

## Provider Comparison

### HuggingFace Inference API

The HuggingFace Inference API provides free access to thousands of embedding models, perfect for development and prototyping.

**Key Features:**
- Free tier with rate limits (varies by model)
- Access to state-of-the-art models
- No infrastructure required
- Automatic model loading and caching

**Popular Models:**
```go
// Fast and efficient (384 dimensions)
Model: "sentence-transformers/all-MiniLM-L6-v2"
// Excellent quality/speed balance

// High quality (1024 dimensions)
Model: "BAAI/bge-large-en-v1.5"
// Top performance on benchmarks

// Multilingual support (1024 dimensions)
Model: "thenlper/gte-large"
// Supports 100+ languages
```

**Configuration Example:**
```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/aixgo-dev/aixgo/pkg/embeddings"
)

func main() {
    ctx := context.Background()

    // Configure HuggingFace API
    config := embeddings.Config{
        Provider: "huggingface",
        HuggingFace: &embeddings.HuggingFaceConfig{
            Model:        "sentence-transformers/all-MiniLM-L6-v2",
            APIKey:       os.Getenv("HUGGINGFACE_API_KEY"), // Optional
            WaitForModel: true,  // Wait if model needs loading
            UseCache:     true,  // Use HF's server-side cache
        },
    }

    // Create embedding service
    embSvc, err := embeddings.New(config)
    if err != nil {
        log.Fatal(err)
    }
    defer embSvc.Close()

    // Generate embedding
    text := "Aixgo is a production-grade AI framework for Go"
    embedding, err := embSvc.Embed(ctx, text)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Dimensions: %d\n", len(embedding))
    fmt.Printf("First 5 values: %v\n", embedding[:5])
}
```

**Rate Limits:**
- Without API key: ~100 requests/hour
- With free API key: ~1000 requests/hour
- Pro accounts: Higher limits available

**Best For:**
- Development and testing
- Prototyping new features
- Small-scale applications
- Evaluating different models

### HuggingFace TEI (Self-Hosted)

Text Embeddings Inference (TEI) is a high-performance embedding server optimized for production workloads.

**Key Features:**
- 10x faster than API calls
- No rate limits
- GPU acceleration support
- Automatic batching and caching
- Production-grade performance

**Docker Deployment:**
```bash
# CPU deployment
docker run -d \
  --name tei \
  -p 8080:8080 \
  -v $PWD/data:/data \
  ghcr.io/huggingface/text-embeddings-inference:latest \
  --model-id BAAI/bge-large-en-v1.5 \
  --max-batch-size 128 \
  --max-client-batch-size 32

# GPU deployment (NVIDIA)
docker run -d \
  --name tei-gpu \
  -p 8080:8080 \
  -v $PWD/data:/data \
  --gpus all \
  ghcr.io/huggingface/text-embeddings-inference:latest \
  --model-id BAAI/bge-large-en-v1.5 \
  --max-batch-size 256 \
  --max-client-batch-size 64
```

**Docker Compose Example:**
```yaml
version: '3.8'
services:
  tei:
    image: ghcr.io/huggingface/text-embeddings-inference:latest
    ports:
      - "8080:8080"
    volumes:
      - ./models:/data
    environment:
      - MODEL_ID=BAAI/bge-large-en-v1.5
    command:
      - --model-id=BAAI/bge-large-en-v1.5
      - --max-batch-size=128
      - --max-client-batch-size=32
      - --max-batch-tokens=16384
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
```

**Configuration Example:**
```go
config := embeddings.Config{
    Provider: "huggingface_tei",
    HuggingFaceTEI: &embeddings.HuggingFaceTEIConfig{
        Endpoint:  "http://localhost:8080",
        Model:     "BAAI/bge-large-en-v1.5",
        Normalize: true,  // L2 normalize vectors
        Truncate:  true,  // Auto-truncate long texts
    },
}

embSvc, err := embeddings.New(config)
if err != nil {
    log.Fatal(err)
}

// Batch processing for efficiency
texts := []string{
    "First document",
    "Second document",
    "Third document",
}

embeddings, err := embSvc.EmbedBatch(ctx, texts)
if err != nil {
    log.Fatal(err)
}
```

**Performance Benefits:**
- Latency: 5-10ms per request (vs 50-100ms for API)
- Throughput: 1000+ embeddings/second with GPU
- Batch processing: Automatic optimization
- No network overhead to external APIs

**Best For:**
- High-volume production workloads
- Low-latency requirements
- Cost-sensitive applications
- Data privacy requirements (on-premise)

### OpenAI

OpenAI provides state-of-the-art embedding models with excellent quality and flexible pricing.

**Model Comparison:**

| Model | Dimensions | Price/1M tokens | Quality | Use Case |
|-------|------------|-----------------|---------|-----------|
| text-embedding-3-small | 1536 | $0.02 | Very Good | General purpose |
| text-embedding-3-large | 3072 | $0.13 | Excellent | High accuracy |
| text-embedding-ada-002 | 1536 | $0.10 | Good | Legacy support |

**Custom Dimensions:**
OpenAI's new models support dimension reduction:
```go
config := embeddings.Config{
    Provider: "openai",
    OpenAI: &embeddings.OpenAIConfig{
        APIKey:     os.Getenv("OPENAI_API_KEY"),
        Model:      "text-embedding-3-small",
        Dimensions: 512,  // Reduce from 1536 to 512
    },
}
```

**Configuration Example:**
```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/aixgo-dev/aixgo/pkg/embeddings"
)

func main() {
    ctx := context.Background()

    // Configure OpenAI embeddings
    config := embeddings.Config{
        Provider: "openai",
        OpenAI: &embeddings.OpenAIConfig{
            APIKey: os.Getenv("OPENAI_API_KEY"),
            Model:  "text-embedding-3-small",
            // Optional: custom dimensions
            // Dimensions: 768,
        },
    }

    embSvc, err := embeddings.New(config)
    if err != nil {
        log.Fatal(err)
    }
    defer embSvc.Close()

    // Single embedding
    embedding, err := embSvc.Embed(ctx, "Sample text")
    if err != nil {
        log.Fatal(err)
    }

    // Batch embeddings (more efficient)
    texts := []string{
        "First document about AI",
        "Second document about machine learning",
        "Third document about deep learning",
    }

    embeddings, err := embSvc.EmbedBatch(ctx, texts)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Generated %d embeddings\n", len(embeddings))
}
```

**Cost Optimization:**
```go
// Calculate embedding costs
func calculateCost(texts []string, model string) float64 {
    totalTokens := 0
    for _, text := range texts {
        // Rough estimate: 1 token ≈ 4 characters
        totalTokens += len(text) / 4
    }

    costPerMillion := map[string]float64{
        "text-embedding-3-small": 0.02,
        "text-embedding-3-large": 0.13,
        "text-embedding-ada-002": 0.10,
    }

    return float64(totalTokens) * costPerMillion[model] / 1_000_000
}

// Example usage
texts := make([]string, 10000)
// ... populate texts
cost := calculateCost(texts, "text-embedding-3-small")
fmt.Printf("Estimated cost: $%.4f\n", cost)
```

**Best For:**
- Production applications requiring high quality
- Applications with moderate volume
- When consistency and reliability are critical
- Multi-language support requirements

## Choosing the Right Provider

### Decision Matrix

Use this matrix to select the optimal provider based on your requirements:

| Requirement | HuggingFace API | HuggingFace TEI | OpenAI |
|-------------|-----------------|-----------------|---------|
| **Budget: Free** | ✅ Best | ✅ Best (self-host) | ❌ |
| **Budget: <$100/month** | ✅ | ✅ | ✅ Good |
| **Budget: >$100/month** | ✅ | ✅ | ✅ Best |
| **Quality: Good** | ✅ | ✅ | ✅ |
| **Quality: Excellent** | ✅ (bge-large) | ✅ (bge-large) | ✅ Best |
| **Latency: <10ms** | ❌ | ✅ Best | ❌ |
| **Latency: <50ms** | ❌ | ✅ | ✅ |
| **Latency: <100ms** | ✅ | ✅ | ✅ |
| **Volume: <1K/day** | ✅ Best | ✅ | ✅ |
| **Volume: 1K-100K/day** | ❌ | ✅ Best | ✅ |
| **Volume: >100K/day** | ❌ | ✅ Best | ✅ |
| **Infrastructure: None** | ✅ Best | ❌ | ✅ Best |
| **Infrastructure: Docker** | ❌ | ✅ | ❌ |
| **Data Privacy** | ❌ | ✅ Best | ❌ |

### Practical Recommendations

**For Development:**
```go
// Use HuggingFace API - free and easy
config := embeddings.Config{
    Provider: "huggingface",
    HuggingFace: &embeddings.HuggingFaceConfig{
        Model: "sentence-transformers/all-MiniLM-L6-v2",
    },
}
```

**For Production (Budget-Conscious):**
```go
// Deploy TEI with Docker
config := embeddings.Config{
    Provider: "huggingface_tei",
    HuggingFaceTEI: &embeddings.HuggingFaceTEIConfig{
        Endpoint: "http://tei-service:8080",
        Model:    "BAAI/bge-large-en-v1.5",
    },
}
```

**For Production (Quality-First):**
```go
// Use OpenAI for best quality
config := embeddings.Config{
    Provider: "openai",
    OpenAI: &embeddings.OpenAIConfig{
        APIKey: os.Getenv("OPENAI_API_KEY"),
        Model:  "text-embedding-3-large",
    },
}
```

**For Hybrid Approach:**
```go
// Use multiple providers with fallback
type EmbeddingServiceWithFallback struct {
    primary   embeddings.EmbeddingService
    fallback  embeddings.EmbeddingService
}

func (e *EmbeddingServiceWithFallback) Embed(ctx context.Context, text string) ([]float32, error) {
    emb, err := e.primary.Embed(ctx, text)
    if err != nil {
        // Fallback to secondary provider
        return e.fallback.Embed(ctx, text)
    }
    return emb, nil
}
```

## Integration with Vectorstore

Embeddings seamlessly integrate with Aixgo's vectorstore for building RAG systems and semantic search.

### Matching Dimensions

Ensure your vectorstore and embeddings use compatible dimensions:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/aixgo-dev/aixgo/pkg/embeddings"
    "github.com/aixgo-dev/aixgo/pkg/vectorstore"
    "github.com/aixgo-dev/aixgo/pkg/vectorstore/memory"
)

func main() {
    ctx := context.Background()

    // Setup embeddings
    embConfig := embeddings.Config{
        Provider: "huggingface",
        HuggingFace: &embeddings.HuggingFaceConfig{
            Model: "sentence-transformers/all-MiniLM-L6-v2", // 384 dims
        },
    }
    embSvc, err := embeddings.New(embConfig)
    if err != nil {
        log.Fatal(err)
    }
    defer embSvc.Close()

    // Get embedding dimensions
    dims := embSvc.Dimensions()
    fmt.Printf("Embedding dimensions: %d\n", dims)

    // Create vectorstore with matching dimensions
    store, err := memory.New(
        memory.WithEmbeddingDimensions(dims),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer store.Close()

    // Create collection
    docs := store.Collection("documents")

    // Index document
    text := "Aixgo provides powerful embedding capabilities"
    embedding, err := embSvc.Embed(ctx, text)
    if err != nil {
        log.Fatal(err)
    }

    doc := &vectorstore.Document{
        ID:        "doc1",
        Content:   vectorstore.NewTextContent(text),
        Embedding: vectorstore.NewEmbedding(embedding, embSvc.Model()),
    }

    _, err = docs.Upsert(ctx, doc)
    if err != nil {
        log.Fatal(err)
    }
}
```

### Batch Processing for Efficiency

Process multiple documents efficiently:

```go
func batchIndexDocuments(
    ctx context.Context,
    collection vectorstore.Collection,
    embSvc embeddings.EmbeddingService,
    texts []string,
) error {
    const batchSize = 100

    for i := 0; i < len(texts); i += batchSize {
        end := i + batchSize
        if end > len(texts) {
            end = len(texts)
        }

        batch := texts[i:end]

        // Batch embed
        embeddings, err := embSvc.EmbedBatch(ctx, batch)
        if err != nil {
            return fmt.Errorf("batch embed failed: %w", err)
        }

        // Create documents
        docs := make([]*vectorstore.Document, len(batch))
        for j, text := range batch {
            docs[j] = &vectorstore.Document{
                ID:        fmt.Sprintf("doc-%d", i+j),
                Content:   vectorstore.NewTextContent(text),
                Embedding: vectorstore.NewEmbedding(embeddings[j], embSvc.Model()),
            }
        }

        // Batch upsert
        result, err := collection.UpsertBatch(ctx, docs)
        if err != nil {
            return fmt.Errorf("batch upsert failed: %w", err)
        }

        fmt.Printf("Indexed batch %d-%d: %d succeeded, %d failed\n",
            i, end, result.Succeeded, result.Failed)
    }

    return nil
}
```

### Caching Strategies

Implement embedding cache to reduce API calls and costs:

```go
package main

import (
    "context"
    "crypto/md5"
    "encoding/hex"
    "sync"
    "time"

    "github.com/aixgo-dev/aixgo/pkg/embeddings"
)

type CachedEmbeddingService struct {
    service embeddings.EmbeddingService
    cache   map[string]cacheEntry
    mu      sync.RWMutex
    ttl     time.Duration
}

type cacheEntry struct {
    embedding []float32
    timestamp time.Time
}

func NewCachedEmbeddingService(service embeddings.EmbeddingService, ttl time.Duration) *CachedEmbeddingService {
    return &CachedEmbeddingService{
        service: service,
        cache:   make(map[string]cacheEntry),
        ttl:     ttl,
    }
}

func (c *CachedEmbeddingService) Embed(ctx context.Context, text string) ([]float32, error) {
    // Generate cache key
    hash := md5.Sum([]byte(text))
    key := hex.EncodeToString(hash[:])

    // Check cache
    c.mu.RLock()
    if entry, ok := c.cache[key]; ok {
        if time.Since(entry.timestamp) < c.ttl {
            c.mu.RUnlock()
            return entry.embedding, nil
        }
    }
    c.mu.RUnlock()

    // Generate embedding
    embedding, err := c.service.Embed(ctx, text)
    if err != nil {
        return nil, err
    }

    // Update cache
    c.mu.Lock()
    c.cache[key] = cacheEntry{
        embedding: embedding,
        timestamp: time.Now(),
    }
    c.mu.Unlock()

    return embedding, nil
}

func (c *CachedEmbeddingService) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
    results := make([][]float32, len(texts))
    uncached := make([]int, 0)
    uncachedTexts := make([]string, 0)

    // Check cache for each text
    c.mu.RLock()
    for i, text := range texts {
        hash := md5.Sum([]byte(text))
        key := hex.EncodeToString(hash[:])

        if entry, ok := c.cache[key]; ok && time.Since(entry.timestamp) < c.ttl {
            results[i] = entry.embedding
        } else {
            uncached = append(uncached, i)
            uncachedTexts = append(uncachedTexts, text)
        }
    }
    c.mu.RUnlock()

    // Generate embeddings for uncached texts
    if len(uncachedTexts) > 0 {
        embeddings, err := c.service.EmbedBatch(ctx, uncachedTexts)
        if err != nil {
            return nil, err
        }

        // Update results and cache
        c.mu.Lock()
        for i, idx := range uncached {
            results[idx] = embeddings[i]

            hash := md5.Sum([]byte(uncachedTexts[i]))
            key := hex.EncodeToString(hash[:])
            c.cache[key] = cacheEntry{
                embedding: embeddings[i],
                timestamp: time.Now(),
            }
        }
        c.mu.Unlock()
    }

    return results, nil
}

// Usage
func main() {
    // Create base service
    config := embeddings.Config{
        Provider: "openai",
        OpenAI: &embeddings.OpenAIConfig{
            APIKey: os.Getenv("OPENAI_API_KEY"),
            Model:  "text-embedding-3-small",
        },
    }
    baseSvc, _ := embeddings.New(config)

    // Wrap with cache (1 hour TTL)
    cachedSvc := NewCachedEmbeddingService(baseSvc, time.Hour)

    // Use cached service
    ctx := context.Background()
    embedding, _ := cachedSvc.Embed(ctx, "Sample text")
    // First call: generates embedding

    embedding2, _ := cachedSvc.Embed(ctx, "Sample text")
    // Second call: returns from cache
}
```

### Error Handling

Implement robust error handling for production:

```go
func embedWithRetry(
    ctx context.Context,
    svc embeddings.EmbeddingService,
    text string,
    maxRetries int,
) ([]float32, error) {
    var lastErr error

    for i := 0; i < maxRetries; i++ {
        embedding, err := svc.Embed(ctx, text)
        if err == nil {
            return embedding, nil
        }

        lastErr = err

        // Check if error is retryable
        if !isRetryableError(err) {
            return nil, err
        }

        // Exponential backoff
        backoff := time.Duration(1<<uint(i)) * time.Second
        select {
        case <-time.After(backoff):
            // Continue to next retry
        case <-ctx.Done():
            return nil, ctx.Err()
        }
    }

    return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

func isRetryableError(err error) bool {
    errStr := err.Error()
    return strings.Contains(errStr, "rate limit") ||
           strings.Contains(errStr, "timeout") ||
           strings.Contains(errStr, "temporary")
}
```

## Best Practices

### Performance Optimization

**Use Batch Operations:**
```go
// Inefficient: Individual calls
for _, text := range texts {
    emb, _ := embSvc.Embed(ctx, text)
    // Process embedding
}

// Efficient: Batch call
embeddings, _ := embSvc.EmbedBatch(ctx, texts)
for i, emb := range embeddings {
    // Process embedding
}
```

**Implement Connection Pooling:**
```go
// For TEI deployments
type TEIPool struct {
    endpoints []string
    current   int
    mu        sync.Mutex
}

func (p *TEIPool) GetEndpoint() string {
    p.mu.Lock()
    defer p.mu.Unlock()

    endpoint := p.endpoints[p.current]
    p.current = (p.current + 1) % len(p.endpoints)
    return endpoint
}

// Use round-robin load balancing
pool := &TEIPool{
    endpoints: []string{
        "http://tei-1:8080",
        "http://tei-2:8080",
        "http://tei-3:8080",
    },
}
```

**Choose Appropriate Batch Sizes:**
```go
// Optimal batch sizes by provider
batchSizes := map[string]int{
    "huggingface":     50,   // API rate limits
    "huggingface_tei": 128,  // TEI optimization
    "openai":          100,  // API limits
}
```

### Quality Considerations

**Pick Models Matching Your Domain:**

```go
// General text
models := []string{
    "sentence-transformers/all-MiniLM-L6-v2",
    "text-embedding-3-small",
}

// High quality requirements
models := []string{
    "BAAI/bge-large-en-v1.5",
    "text-embedding-3-large",
}

// Multilingual
models := []string{
    "thenlper/gte-large",
    "intfloat/multilingual-e5-large",
}

// Code search
models := []string{
    "microsoft/codebert-base",
    "huggingface/CodeBERTa-small-v1",
}
```

**Test Quality with Your Data:**
```go
func evaluateEmbeddingQuality(
    svc embeddings.EmbeddingService,
    testPairs [][]string,
) float64 {
    ctx := context.Background()
    totalScore := 0.0

    for _, pair := range testPairs {
        emb1, _ := svc.Embed(ctx, pair[0])
        emb2, _ := svc.Embed(ctx, pair[1])

        similarity := cosineSimilarity(emb1, emb2)
        totalScore += similarity
    }

    return totalScore / float64(len(testPairs))
}

func cosineSimilarity(a, b []float32) float64 {
    var dotProduct, normA, normB float64
    for i := range a {
        dotProduct += float64(a[i] * b[i])
        normA += float64(a[i] * a[i])
        normB += float64(b[i] * b[i])
    }
    return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
```

**Monitor Embedding Drift:**
```go
type EmbeddingMonitor struct {
    baseline  map[string][]float32
    threshold float64
}

func (m *EmbeddingMonitor) CheckDrift(text string, current []float32) bool {
    if baseline, ok := m.baseline[text]; ok {
        similarity := cosineSimilarity(baseline, current)
        if similarity < m.threshold {
            log.Printf("Drift detected for '%s': similarity %.3f", text, similarity)
            return true
        }
    }
    return false
}
```

### Cost Optimization

**Use HuggingFace for Development:**
```go
func getEmbeddingConfig(env string) embeddings.Config {
    switch env {
    case "development":
        return embeddings.Config{
            Provider: "huggingface",
            HuggingFace: &embeddings.HuggingFaceConfig{
                Model: "sentence-transformers/all-MiniLM-L6-v2",
            },
        }
    case "production":
        return embeddings.Config{
            Provider: "huggingface_tei",
            HuggingFaceTEI: &embeddings.HuggingFaceTEIConfig{
                Endpoint: os.Getenv("TEI_ENDPOINT"),
                Model:    "BAAI/bge-large-en-v1.5",
            },
        }
    default:
        panic("unknown environment")
    }
}
```

**Cache Frequently Used Embeddings:**
```go
// Implement LRU cache for embeddings
type LRUEmbeddingCache struct {
    capacity int
    cache    map[string]*list.Element
    lru      *list.List
    mu       sync.Mutex
}

type cacheItem struct {
    key       string
    embedding []float32
}

func (c *LRUEmbeddingCache) Get(key string) ([]float32, bool) {
    c.mu.Lock()
    defer c.mu.Unlock()

    if elem, ok := c.cache[key]; ok {
        c.lru.MoveToFront(elem)
        return elem.Value.(*cacheItem).embedding, true
    }
    return nil, false
}

func (c *LRUEmbeddingCache) Set(key string, embedding []float32) {
    c.mu.Lock()
    defer c.mu.Unlock()

    if elem, ok := c.cache[key]; ok {
        c.lru.MoveToFront(elem)
        elem.Value.(*cacheItem).embedding = embedding
        return
    }

    if c.lru.Len() >= c.capacity {
        oldest := c.lru.Back()
        if oldest != nil {
            c.lru.Remove(oldest)
            delete(c.cache, oldest.Value.(*cacheItem).key)
        }
    }

    elem := c.lru.PushFront(&cacheItem{key, embedding})
    c.cache[key] = elem
}
```

**Consider Dimension Reduction:**
```go
// OpenAI supports custom dimensions
config := embeddings.Config{
    Provider: "openai",
    OpenAI: &embeddings.OpenAIConfig{
        APIKey:     os.Getenv("OPENAI_API_KEY"),
        Model:      "text-embedding-3-large",
        Dimensions: 1024, // Reduce from 3072 to 1024
        // Minimal quality loss, 66% cost reduction
    },
}
```

## Model Selection Guide

### By Use Case

| Use Case | Recommended Models | Dimensions | Provider |
|----------|-------------------|------------|----------|
| **General Text** | all-MiniLM-L6-v2, text-embedding-3-small | 384, 1536 | HF, OpenAI |
| **High Quality** | bge-large-en-v1.5, text-embedding-3-large | 1024, 3072 | HF, OpenAI |
| **Multilingual** | gte-large, multilingual-e5-large | 1024 | HF |
| **Code Search** | codebert-base, codegen-embeddings | 768 | HF |
| **Long Documents** | jina-embeddings-v2-base-en | 768 | HF |
| **Low Latency** | all-MiniLM-L6-v2 | 384 | HF TEI |
| **Budget** | all-MiniLM-L6-v2 | 384 | HF API |

### By Dimensions

**384 Dimensions (Fast, Good Quality):**
```go
// Best for real-time applications
Model: "sentence-transformers/all-MiniLM-L6-v2"
// 120M parameters, 5ms inference
```

**768 Dimensions (Balanced):**
```go
// Good balance of speed and quality
Model: "sentence-transformers/all-mpnet-base-v2"
// 110M parameters, 10ms inference
```

**1024 Dimensions (High Quality):**
```go
// Excellent quality for most use cases
Model: "BAAI/bge-large-en-v1.5"
// 335M parameters, 15ms inference
```

**1536 Dimensions (OpenAI Standard):**
```go
// OpenAI's default dimension
Model: "text-embedding-3-small"
// Good quality, managed service
```

**3072 Dimensions (Best Quality):**
```go
// Maximum quality available
Model: "text-embedding-3-large"
// Best for critical applications
```

### Performance Benchmarks

```go
// Benchmark different models
func benchmarkModels() {
    models := []struct {
        name string
        dims int
        config embeddings.Config
    }{
        {
            name: "MiniLM",
            dims: 384,
            config: embeddings.Config{
                Provider: "huggingface",
                HuggingFace: &embeddings.HuggingFaceConfig{
                    Model: "sentence-transformers/all-MiniLM-L6-v2",
                },
            },
        },
        {
            name: "BGE-Large",
            dims: 1024,
            config: embeddings.Config{
                Provider: "huggingface",
                HuggingFace: &embeddings.HuggingFaceConfig{
                    Model: "BAAI/bge-large-en-v1.5",
                },
            },
        },
    }

    text := "Sample text for benchmarking"

    for _, model := range models {
        svc, _ := embeddings.New(model.config)

        start := time.Now()
        _, err := svc.Embed(context.Background(), text)
        elapsed := time.Since(start)

        fmt.Printf("%s (%d dims): %v\n", model.name, model.dims, elapsed)
    }
}
```

## Common Issues & Troubleshooting

### Dimension Mismatches

**Problem:**
```text
Error: dimension mismatch: expected 384, got 1536
```

**Solution:**
```go
// Ensure consistent dimensions across your pipeline
embSvc, _ := embeddings.New(config)
dims := embSvc.Dimensions()

// Configure vectorstore with matching dimensions
store, _ := memory.New(
    memory.WithEmbeddingDimensions(dims),
)

// For Firestore, create index with correct dimensions
// gcloud firestore indexes composite create \
//   --field-config=field-path=embedding.vector,vector-config='{"dimension":"384","flat":{}}'
```

### Rate Limits

**Problem:**
```text
Error: rate limit exceeded
```

**Solutions:**
```go
// 1. Add API key for higher limits
config := embeddings.Config{
    Provider: "huggingface",
    HuggingFace: &embeddings.HuggingFaceConfig{
        Model:  "sentence-transformers/all-MiniLM-L6-v2",
        APIKey: os.Getenv("HUGGINGFACE_API_KEY"),
    },
}

// 2. Implement rate limiting
type RateLimitedEmbedding struct {
    service embeddings.EmbeddingService
    limiter *rate.Limiter
}

func (r *RateLimitedEmbedding) Embed(ctx context.Context, text string) ([]float32, error) {
    if err := r.limiter.Wait(ctx); err != nil {
        return nil, err
    }
    return r.service.Embed(ctx, text)
}

// 3. Deploy TEI for unlimited requests
// See TEI deployment section above
```

### Model Loading Errors

**Problem:**
```text
Error: model is currently loading
```

**Solution:**
```go
config := embeddings.Config{
    Provider: "huggingface",
    HuggingFace: &embeddings.HuggingFaceConfig{
        Model:        "BAAI/bge-large-en-v1.5",
        WaitForModel: true,  // Wait for model to load
        UseCache:     false, // Force fresh model load
    },
}
```

### Embedding Quality Issues

**Problem:** Poor search results or similarity scores

**Debugging:**
```go
func debugEmbeddingQuality(svc embeddings.EmbeddingService) {
    ctx := context.Background()

    // Test similar texts
    similar := []string{
        "machine learning",
        "deep learning",
        "artificial intelligence",
    }

    embeddings := make([][]float32, len(similar))
    for i, text := range similar {
        embeddings[i], _ = svc.Embed(ctx, text)
    }

    // Check similarities
    for i := 0; i < len(similar); i++ {
        for j := i + 1; j < len(similar); j++ {
            sim := cosineSimilarity(embeddings[i], embeddings[j])
            fmt.Printf("%s <-> %s: %.3f\n", similar[i], similar[j], sim)
        }
    }

    // Test dissimilar texts
    dissimilar := "cooking recipes"
    dissimilarEmb, _ := svc.Embed(ctx, dissimilar)

    for i, text := range similar {
        sim := cosineSimilarity(embeddings[i], dissimilarEmb)
        fmt.Printf("%s <-> %s: %.3f\n", text, dissimilar, sim)
    }
}
```

**Solutions:**
1. Try a larger model (bge-large vs all-MiniLM)
2. Fine-tune on domain-specific data
3. Adjust text preprocessing
4. Use hybrid search (semantic + keyword)

### Memory Issues with Large Batches

**Problem:**
```text
Error: out of memory
```

**Solution:**
```go
func processLargeDataset(
    svc embeddings.EmbeddingService,
    texts []string,
) error {
    const maxBatchSize = 50

    for i := 0; i < len(texts); i += maxBatchSize {
        end := i + maxBatchSize
        if end > len(texts) {
            end = len(texts)
        }

        batch := texts[i:end]

        // Process batch with timeout
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        embeddings, err := svc.EmbedBatch(ctx, batch)
        cancel()

        if err != nil {
            // Log error and continue
            log.Printf("Batch %d failed: %v", i/maxBatchSize, err)
            continue
        }

        // Process embeddings
        for j, emb := range embeddings {
            // Store or process embedding
            _ = emb
            _ = j
        }

        // Optional: garbage collection hint
        if i%1000 == 0 {
            runtime.GC()
        }
    }

    return nil
}
```

## Production Deployment

### HuggingFace TEI Setup

**Full Production Setup:**

```yaml
# docker-compose.yml
version: '3.8'

services:
  tei:
    image: ghcr.io/huggingface/text-embeddings-inference:latest
    ports:
      - "8080:8080"
    volumes:
      - ./models:/data
    environment:
      - HUGGING_FACE_HUB_TOKEN=${HUGGING_FACE_HUB_TOKEN}
    command:
      - --model-id=BAAI/bge-large-en-v1.5
      - --max-batch-size=256
      - --max-client-batch-size=64
      - --max-batch-tokens=16384
      - --max-concurrent-requests=512
    deploy:
      replicas: 3
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf
    depends_on:
      - tei
```

**Nginx Load Balancer Configuration:**
```nginx
upstream tei_backend {
    least_conn;
    server tei_1:8080 max_fails=3 fail_timeout=30s;
    server tei_2:8080 max_fails=3 fail_timeout=30s;
    server tei_3:8080 max_fails=3 fail_timeout=30s;
}

server {
    listen 80;

    location / {
        proxy_pass http://tei_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;

        # Timeouts
        proxy_connect_timeout 10s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;

        # Buffering
        proxy_buffering off;

        # Health checks
        proxy_next_upstream error timeout http_502 http_503;
        proxy_next_upstream_tries 3;
    }
}
```

**Monitoring and Scaling:**
```go
// Health check endpoint
func healthCheckTEI(endpoint string) error {
    resp, err := http.Get(endpoint + "/health")
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("unhealthy: status %d", resp.StatusCode)
    }
    return nil
}

// Auto-scaling based on latency
type TEIAutoScaler struct {
    targetLatency time.Duration
    minInstances  int
    maxInstances  int
}

func (s *TEIAutoScaler) CheckScaling(avgLatency time.Duration, currentInstances int) int {
    if avgLatency > s.targetLatency && currentInstances < s.maxInstances {
        return currentInstances + 1 // Scale up
    }
    if avgLatency < s.targetLatency/2 && currentInstances > s.minInstances {
        return currentInstances - 1 // Scale down
    }
    return currentInstances // No change
}
```

### OpenAI Best Practices

**Rate Limit Handling:**
```go
type OpenAIRateLimiter struct {
    service   embeddings.EmbeddingService
    limiter   *rate.Limiter
    semaphore chan struct{}
}

func NewOpenAIRateLimiter(service embeddings.EmbeddingService, rps int, maxConcurrent int) *OpenAIRateLimiter {
    return &OpenAIRateLimiter{
        service:   service,
        limiter:   rate.NewLimiter(rate.Limit(rps), rps),
        semaphore: make(chan struct{}, maxConcurrent),
    }
}

func (r *OpenAIRateLimiter) Embed(ctx context.Context, text string) ([]float32, error) {
    // Acquire semaphore
    r.semaphore <- struct{}{}
    defer func() { <-r.semaphore }()

    // Rate limit
    if err := r.limiter.Wait(ctx); err != nil {
        return nil, err
    }

    return r.service.Embed(ctx, text)
}
```

**Retry Strategies:**
```go
func embedWithExponentialBackoff(
    ctx context.Context,
    svc embeddings.EmbeddingService,
    text string,
) ([]float32, error) {
    maxRetries := 5
    baseDelay := time.Second

    for i := 0; i < maxRetries; i++ {
        embedding, err := svc.Embed(ctx, text)
        if err == nil {
            return embedding, nil
        }

        // Check for rate limit error
        if strings.Contains(err.Error(), "rate_limit") {
            delay := baseDelay * time.Duration(math.Pow(2, float64(i)))
            if delay > 30*time.Second {
                delay = 30 * time.Second
            }

            select {
            case <-time.After(delay):
                continue
            case <-ctx.Done():
                return nil, ctx.Err()
            }
        }

        // Non-retryable error
        return nil, err
    }

    return nil, fmt.Errorf("max retries exceeded")
}
```

**Cost Monitoring:**
```go
type CostTracker struct {
    mu         sync.Mutex
    tokenCount int64
    model      string
}

func (ct *CostTracker) Track(text string) {
    // Estimate tokens (rough: 1 token ≈ 4 characters)
    tokens := len(text) / 4

    ct.mu.Lock()
    ct.tokenCount += int64(tokens)
    ct.mu.Unlock()
}

func (ct *CostTracker) GetCost() float64 {
    ct.mu.Lock()
    defer ct.mu.Unlock()

    costPerMillion := map[string]float64{
        "text-embedding-3-small": 0.02,
        "text-embedding-3-large": 0.13,
        "text-embedding-ada-002": 0.10,
    }

    return float64(ct.tokenCount) * costPerMillion[ct.model] / 1_000_000
}

func (ct *CostTracker) Reset() {
    ct.mu.Lock()
    ct.tokenCount = 0
    ct.mu.Unlock()
}
```

**Fallback Providers:**
```go
type FallbackEmbeddingService struct {
    providers []embeddings.EmbeddingService
}

func (f *FallbackEmbeddingService) Embed(ctx context.Context, text string) ([]float32, error) {
    var lastErr error

    for i, provider := range f.providers {
        embedding, err := provider.Embed(ctx, text)
        if err == nil {
            if i > 0 {
                log.Printf("Using fallback provider %d", i)
            }
            return embedding, nil
        }
        lastErr = err
    }

    return nil, fmt.Errorf("all providers failed: %w", lastErr)
}

// Usage
fallbackSvc := &FallbackEmbeddingService{
    providers: []embeddings.EmbeddingService{
        openaiSvc,    // Primary
        teiSvc,       // Fallback 1
        huggingfaceSvc, // Fallback 2
    },
}
```

## Complete Examples

### Building a Semantic Search System

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/aixgo-dev/aixgo/pkg/embeddings"
    "github.com/aixgo-dev/aixgo/pkg/vectorstore"
    "github.com/aixgo-dev/aixgo/pkg/vectorstore/memory"
)

type SemanticSearch struct {
    embeddings embeddings.EmbeddingService
    collection vectorstore.Collection
}

func NewSemanticSearch() (*SemanticSearch, error) {
    // Setup embeddings
    embConfig := embeddings.Config{
        Provider: "huggingface",
        HuggingFace: &embeddings.HuggingFaceConfig{
            Model: "BAAI/bge-large-en-v1.5",
        },
    }
    embSvc, err := embeddings.New(embConfig)
    if err != nil {
        return nil, err
    }

    // Setup vector store
    store, err := memory.New()
    if err != nil {
        return nil, err
    }

    // Create collection
    collection := store.Collection("documents",
        vectorstore.WithDeduplication(true),
        vectorstore.WithMaxDocuments(100000),
    )

    return &SemanticSearch{
        embeddings: embSvc,
        collection: collection,
    }, nil
}

func (ss *SemanticSearch) IndexDocuments(ctx context.Context, documents map[string]string) error {
    // Batch process documents
    ids := make([]string, 0, len(documents))
    texts := make([]string, 0, len(documents))

    for id, text := range documents {
        ids = append(ids, id)
        texts = append(texts, text)
    }

    // Generate embeddings
    embeddings, err := ss.embeddings.EmbedBatch(ctx, texts)
    if err != nil {
        return fmt.Errorf("embedding generation failed: %w", err)
    }

    // Create document objects
    docs := make([]*vectorstore.Document, len(texts))
    for i, text := range texts {
        docs[i] = &vectorstore.Document{
            ID:        ids[i],
            Content:   vectorstore.NewTextContent(text),
            Embedding: vectorstore.NewEmbedding(embeddings[i], ss.embeddings.Model()),
        }
    }

    // Batch upsert
    result, err := ss.collection.UpsertBatch(ctx, docs)
    if err != nil {
        return fmt.Errorf("upsert failed: %w", err)
    }

    fmt.Printf("Indexed %d documents (%d succeeded, %d failed)\n",
        len(docs), result.Succeeded, result.Failed)

    return nil
}

func (ss *SemanticSearch) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
    // Generate query embedding
    queryEmb, err := ss.embeddings.Embed(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("query embedding failed: %w", err)
    }

    // Search
    result, err := ss.collection.Query(ctx, &vectorstore.Query{
        Embedding: vectorstore.NewEmbedding(queryEmb, ss.embeddings.Model()),
        Limit:     limit,
        MinScore:  0.5,
    })
    if err != nil {
        return nil, fmt.Errorf("search failed: %w", err)
    }

    // Convert results
    results := make([]SearchResult, 0, len(result.Matches))
    for _, match := range result.Matches {
        results = append(results, SearchResult{
            ID:      match.Document.ID,
            Content: match.Document.Content.String(),
            Score:   match.Score,
        })
    }

    return results, nil
}

type SearchResult struct {
    ID      string
    Content string
    Score   float64
}

func (ss *SemanticSearch) Close() error {
    return ss.embeddings.Close()
}

func main() {
    ctx := context.Background()

    // Initialize search system
    search, err := NewSemanticSearch()
    if err != nil {
        log.Fatal(err)
    }
    defer search.Close()

    // Index sample documents
    documents := map[string]string{
        "doc1": "Aixgo is a production-grade AI framework for Go",
        "doc2": "Machine learning enables computers to learn from data",
        "doc3": "Go is a statically typed, compiled programming language",
        "doc4": "Neural networks are inspired by biological neural networks",
        "doc5": "Docker containers provide consistent deployment environments",
    }

    if err := search.IndexDocuments(ctx, documents); err != nil {
        log.Fatal(err)
    }

    // Perform searches
    queries := []string{
        "AI framework for golang",
        "deep learning and neural networks",
        "container deployment",
    }

    for _, query := range queries {
        fmt.Printf("\nSearching for: '%s'\n", query)
        results, err := search.Search(ctx, query, 3)
        if err != nil {
            log.Printf("Search failed: %v", err)
            continue
        }

        for i, result := range results {
            fmt.Printf("%d. [%.3f] %s: %s\n",
                i+1, result.Score, result.ID, result.Content)
        }
    }
}
```

### Multi-Provider Embedding Pipeline

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "sync"

    "github.com/aixgo-dev/aixgo/pkg/embeddings"
)

type MultiProviderPipeline struct {
    providers map[string]embeddings.EmbeddingService
    mu        sync.RWMutex
}

func NewMultiProviderPipeline() (*MultiProviderPipeline, error) {
    providers := make(map[string]embeddings.EmbeddingService)

    // Setup HuggingFace API
    hfConfig := embeddings.Config{
        Provider: "huggingface",
        HuggingFace: &embeddings.HuggingFaceConfig{
            Model: "sentence-transformers/all-MiniLM-L6-v2",
        },
    }
    hfSvc, err := embeddings.New(hfConfig)
    if err == nil {
        providers["huggingface"] = hfSvc
    }

    // Setup OpenAI (if API key available)
    if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
        openaiConfig := embeddings.Config{
            Provider: "openai",
            OpenAI: &embeddings.OpenAIConfig{
                APIKey: apiKey,
                Model:  "text-embedding-3-small",
            },
        }
        openaiSvc, err := embeddings.New(openaiConfig)
        if err == nil {
            providers["openai"] = openaiSvc
        }
    }

    // Setup TEI (if available)
    if endpoint := os.Getenv("TEI_ENDPOINT"); endpoint != "" {
        teiConfig := embeddings.Config{
            Provider: "huggingface_tei",
            HuggingFaceTEI: &embeddings.HuggingFaceTEIConfig{
                Endpoint: endpoint,
                Model:    "BAAI/bge-large-en-v1.5",
            },
        }
        teiSvc, err := embeddings.New(teiConfig)
        if err == nil {
            providers["tei"] = teiSvc
        }
    }

    if len(providers) == 0 {
        return nil, fmt.Errorf("no embedding providers available")
    }

    return &MultiProviderPipeline{
        providers: providers,
    }, nil
}

func (mp *MultiProviderPipeline) CompareProviders(ctx context.Context, text string) {
    mp.mu.RLock()
    defer mp.mu.RUnlock()

    fmt.Printf("Comparing embeddings for: '%s'\n\n", text)

    embeddings := make(map[string][]float32)

    // Generate embeddings with each provider
    for name, svc := range mp.providers {
        emb, err := svc.Embed(ctx, text)
        if err != nil {
            fmt.Printf("%s: Error - %v\n", name, err)
            continue
        }
        embeddings[name] = emb
        fmt.Printf("%s: Generated %d dimensions\n", name, len(emb))
    }

    // Compare similarities between providers
    fmt.Println("\nCross-provider similarities:")
    providers := make([]string, 0, len(embeddings))
    for name := range embeddings {
        providers = append(providers, name)
    }

    for i := 0; i < len(providers); i++ {
        for j := i + 1; j < len(providers); j++ {
            emb1 := embeddings[providers[i]]
            emb2 := embeddings[providers[j]]

            if len(emb1) != len(emb2) {
                fmt.Printf("%s <-> %s: Different dimensions (%d vs %d)\n",
                    providers[i], providers[j], len(emb1), len(emb2))
                continue
            }

            similarity := cosineSimilarity(emb1, emb2)
            fmt.Printf("%s <-> %s: %.3f\n",
                providers[i], providers[j], similarity)
        }
    }
}

func (mp *MultiProviderPipeline) BenchmarkProviders(ctx context.Context, texts []string) {
    mp.mu.RLock()
    defer mp.mu.RUnlock()

    fmt.Printf("Benchmarking with %d texts\n\n", len(texts))

    for name, svc := range mp.providers {
        start := time.Now()

        // Single embedding
        _, err := svc.Embed(ctx, texts[0])
        singleTime := time.Since(start)

        if err != nil {
            fmt.Printf("%s: Error - %v\n", name, err)
            continue
        }

        // Batch embedding
        start = time.Now()
        _, err = svc.EmbedBatch(ctx, texts)
        batchTime := time.Since(start)

        if err != nil {
            fmt.Printf("%s: Batch error - %v\n", name, err)
            continue
        }

        fmt.Printf("%s:\n", name)
        fmt.Printf("  Single: %v\n", singleTime)
        fmt.Printf("  Batch (%d): %v\n", len(texts), batchTime)
        fmt.Printf("  Avg per text: %v\n\n", batchTime/time.Duration(len(texts)))
    }
}

func (mp *MultiProviderPipeline) Close() {
    mp.mu.Lock()
    defer mp.mu.Unlock()

    for _, svc := range mp.providers {
        svc.Close()
    }
}

func main() {
    ctx := context.Background()

    pipeline, err := NewMultiProviderPipeline()
    if err != nil {
        log.Fatal(err)
    }
    defer pipeline.Close()

    // Compare providers
    pipeline.CompareProviders(ctx, "Artificial intelligence is transforming technology")

    // Benchmark providers
    texts := []string{
        "Machine learning algorithms",
        "Natural language processing",
        "Computer vision applications",
        "Deep learning neural networks",
        "Reinforcement learning agents",
    }

    fmt.Println("\n" + strings.Repeat("=", 50))
    pipeline.BenchmarkProviders(ctx, texts)
}
```

## Next Steps

- **Vector Databases**: [Build RAG systems with embeddings](./vector-databases.md)
- **Provider Integration**: [Configure LLM and embedding providers](./provider-integration.md)
- **Production Deployment**: [Deploy embeddings at scale](./production-deployment.md)
- **API Reference**: [Embeddings Package Documentation](https://pkg.go.dev/github.com/aixgo-dev/aixgo/pkg/embeddings)

## Resources

- [HuggingFace Model Hub](https://huggingface.co/models?pipeline_tag=sentence-similarity)
- [OpenAI Embeddings Guide](https://platform.openai.com/docs/guides/embeddings)
- [Text Embeddings Inference (TEI)](https://github.com/huggingface/text-embeddings-inference)
- [Embedding Model Leaderboard](https://huggingface.co/spaces/mteb/leaderboard)
- [MTEB Benchmark](https://github.com/embeddings-benchmark/mteb)
