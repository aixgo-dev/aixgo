# RAG Agent Example

This example demonstrates building a production-ready Retrieval-Augmented
Generation (RAG) system using Aixgo's vector database and embeddings
integration.

## Table of Contents

- [What is RAG?](#what-is-rag)
- [Architecture](#architecture)
- [Features](#features)
- [Quick Start](#quick-start)
- [Configuration Options](#configuration-options)
- [Firestore Setup](#firestore-setup)
- [Usage Examples](#usage-examples)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)
- [Performance Tuning](#performance-tuning)
- [Next Steps](#next-steps)

## What is RAG?

Retrieval-Augmented Generation (RAG) enhances Large Language Models (LLMs) by
providing them with relevant context from your knowledge base. This approach:

- **Reduces Hallucinations**: Grounds responses in factual data
- **Enables Domain Knowledge**: Lets LLMs answer questions about your specific data
- **Stays Current**: Update knowledge without retraining models
- **Provides Citations**: Track which documents informed each response

### How RAG Works

1. **Index Phase**: Convert documents to embeddings and store in vector database
2. **Retrieval Phase**: Find relevant documents using semantic similarity
3. **Generation Phase**: LLM generates response using retrieved context

## Architecture

```text
┌─────────────┐      ┌──────────────┐      ┌─────────────┐
│   Query     │─────>│  Embeddings  │─────>│   Vector    │
│             │      │   Service    │      │   Search    │
└─────────────┘      └──────────────┘      └─────────────┘
                                                   │
                                                   ▼
┌─────────────┐      ┌──────────────┐      ┌─────────────┐
│  Response   │<─────│  LLM (GPT-4) │<─────│  Retrieved  │
│             │      │              │      │   Context   │
└─────────────┘      └──────────────┘      └─────────────┘
```

## Features

- **Multiple Embedding Providers**: HuggingFace (free), OpenAI, or self-hosted TEI
- **Flexible Vector Stores**: In-memory (development) or Firestore (production)
- **Hybrid Search**: Combine vector similarity with metadata filtering
- **Production Ready**: Built-in observability, error handling, and retry logic
- **Cost Efficient**: Free tier available with HuggingFace models
- **Type Safe**: Leverages Go's type system for reliability

## Quick Start

### Prerequisites

- Go 1.21 or later
- For production: Google Cloud account (Firestore) OR OpenAI API key

### Option 1: HuggingFace Embeddings (Free, No Setup)

Perfect for getting started quickly with zero cost:

```bash
# Clone the repository
cd examples/rag-agent

# Run with default config (HuggingFace + In-Memory)
go run main.go --config config.yaml
```

**What this does:**

- Uses HuggingFace's free Inference API
- Stores embeddings in memory (data lost on restart)
- No API keys or credentials needed

### Option 2: OpenAI Embeddings (Paid, Best Quality)

For production-quality embeddings:

```bash
# Set your OpenAI API key
export OPENAI_API_KEY=sk-...

# Run with OpenAI config
cd examples/rag-agent
go run main.go --config config-openai.yaml
```

**Cost:** ~$0.02 per 1M tokens (text-embedding-3-small)

### Option 3: Production with Firestore (Persistent Storage)

For production deployments with persistent storage:

```bash
# Set up Google Cloud authentication
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json

# Optional: HuggingFace API key for higher rate limits
export HUGGINGFACE_API_KEY=hf_...

# Run with Firestore config
cd examples/rag-agent
go run main.go --config config-firestore.yaml
```

**Setup Required:** See [Firestore Setup](#firestore-setup) section below

## Configuration Options

### Embedding Providers

Choose an embedding provider based on your requirements:

#### HuggingFace (Recommended for Development)

**Free, no setup required:**

```yaml
embeddings:
  provider: huggingface
  huggingface:
    model: sentence-transformers/all-MiniLM-L6-v2  # 384 dims, fast
    wait_for_model: true
    use_cache: true
    api_key: ${HUGGINGFACE_API_KEY}  # Optional, for higher rate limits
```

**Alternative Models:**

```yaml
# Higher quality (1024 dimensions)
model: BAAI/bge-large-en-v1.5

# Multilingual support
model: thenlper/gte-large

# General purpose
model: BAAI/bge-small-en-v1.5
```

**Model Comparison:**

| Model | Dimensions | Speed | Quality | Best For |
|-------|------------|-------|---------|----------|
| `all-MiniLM-L6-v2` | 384 | Very Fast | Good | Development, prototyping |
| `BAAI/bge-small-en-v1.5` | 384 | Fast | Good | General purpose |
| `BAAI/bge-large-en-v1.5` | 1024 | Medium | Excellent | Production |
| `thenlper/gte-large` | 1024 | Medium | Excellent | Multilingual |

#### OpenAI (Recommended for Production)

**Best quality, managed service:**

```yaml
embeddings:
  provider: openai
  openai:
    api_key: ${OPENAI_API_KEY}
    model: text-embedding-3-small  # 1536 dims, $0.02/1M tokens
    # model: text-embedding-3-large  # 3072 dims, $0.13/1M tokens
```

**Cost Comparison (1M tokens):**

| Provider | Model | Cost | Dimensions | Quality |
|----------|-------|------|------------|---------|
| HuggingFace | all-MiniLM-L6-v2 | **FREE** | 384 | Good |
| HuggingFace | bge-large-en-v1.5 | **FREE** | 1024 | Excellent |
| OpenAI | text-embedding-3-small | $0.02 | 1536 | Excellent |
| OpenAI | text-embedding-3-large | $0.13 | 3072 | Best |

### Vector Store Providers

Choose a vector store based on your deployment requirements:

#### Memory (Development)

**Fast, zero setup:**

```yaml
vectorstore:
  provider: memory
  embedding_dimensions: 384  # Must match your embedding model
  default_top_k: 10
  memory:
    max_documents: 10000
```

**Pros:**

- No setup required
- Fast for small datasets
- Perfect for testing

**Cons:**

- Data lost on restart
- Limited capacity (10K documents default)
- Not suitable for production

**Use Cases:** Development, unit tests, prototyping

#### Firestore (Production)

**Persistent, scalable:**

```yaml
vectorstore:
  provider: firestore
  embedding_dimensions: 384
  default_top_k: 10
  firestore:
    project_id: my-project
    collection: knowledge_base
    credentials_file: /path/to/key.json  # Optional, uses ADC if not set
    database_id: "(default)"  # Optional
```

**Pros:**

- Fully managed (serverless)
- Automatic scaling
- Persistent storage
- Real-time sync capabilities

**Cons:**

- Requires GCP account
- Setup complexity
- Costs based on operations
- Index creation time (5-10 min)

**Use Cases:** Production deployments, serverless architectures

**Cost Estimate:** ~$0.06 per 100K reads + storage costs

#### Future Providers

**Coming Soon:**

- **Qdrant**: High-performance dedicated vector database
- **pgvector**: PostgreSQL extension for existing databases
- **Pinecone**: Managed vector database service

See the [Extending Aixgo Guide](../../web/content/guides/extending-aixgo.md) for implementing custom providers.

## Firestore Setup

### 1. Create a GCP Project

```bash
gcloud projects create my-rag-project
gcloud config set project my-rag-project
```

### 2. Enable Firestore

```bash
gcloud services enable firestore.googleapis.com
gcloud firestore databases create --location=us-central1
```

### 3. Create Vector Index

```bash
# For 384-dimensional embeddings (all-MiniLM-L6-v2)
gcloud firestore indexes composite create \
  --collection-group=knowledge_base \
  --query-scope=COLLECTION \
  --field-config=field-path=embedding,vector-config='{"dimension":"384","flat":{}}' \
  --project=my-project

# For 1536-dimensional embeddings (OpenAI text-embedding-3-small)
gcloud firestore indexes composite create \
  --collection-group=knowledge_base \
  --query-scope=COLLECTION \
  --field-config=field-path=embedding,vector-config='{"dimension":"1536","flat":{}}' \
  --project=my-project
```

### 4. Setup Authentication

```bash
# Create service account
gcloud iam service-accounts create aixgo-rag \
  --display-name="Aixgo RAG Agent"

# Grant permissions
gcloud projects add-iam-policy-binding my-project \
  --member="serviceAccount:aixgo-rag@my-project.iam.gserviceaccount.com" \
  --role="roles/datastore.user"

# Create key
gcloud iam service-accounts keys create key.json \
  --iam-account=aixgo-rag@my-project.iam.gserviceaccount.com

export GOOGLE_APPLICATION_CREDENTIALS=$(pwd)/key.json
```

## Usage Examples

### Indexing Documents

```go
package main

import (
    "context"
    "github.com/aixgo-dev/aixgo/pkg/embeddings"
    "github.com/aixgo-dev/aixgo/pkg/vectorstore"
    "github.com/aixgo-dev/aixgo/pkg/vectorstore/memory"
)

func indexDocuments() error {
    // Initialize embedding service
    embeddingCfg := embeddings.Config{
        Provider: "huggingface",
        HuggingFace: &embeddings.HuggingFaceConfig{
            Model: "sentence-transformers/all-MiniLM-L6-v2",
        },
    }
    embSvc, err := embeddings.New(embeddingCfg)
    if err != nil {
        return err
    }
    defer embSvc.Close()

    // Initialize vector store
    store, err := memory.New()
    if err != nil {
        return err
    }
    defer store.Close()

    // Get collection
    coll := store.Collection("knowledge_base")

    // Prepare documents
    docs := []string{
        "Aixgo is a production-grade AI agent framework for Go.",
        "It supports multiple LLM providers including OpenAI, Anthropic, and Gemini.",
        "Aixgo includes built-in support for vector databases and RAG.",
    }

    // Generate embeddings and store
    ctx := context.Background()
    for i, content := range docs {
        // Generate embedding
        embedding, err := embSvc.Embed(ctx, content)
        if err != nil {
            return err
        }

        // Create document
        doc := &vectorstore.Document{
            ID:      fmt.Sprintf("doc-%d", i),
            Content: vectorstore.NewTextContent(content),
            Embedding: vectorstore.NewEmbedding(
                embedding,
                embSvc.ModelName(),
            ),
            Metadata: map[string]any{
                "source": "documentation",
                "index":  i,
            },
        }

        // Store in vector database
        if _, err := coll.Upsert(ctx, doc); err != nil {
            return err
        }
    }

    return nil
}
```

### Searching Documents

```go
func searchDocuments(query string) error {
    // ... initialize services as above ...

    // Get collection
    coll := store.Collection("knowledge_base")

    // Generate query embedding
    ctx := context.Background()
    queryEmbedding, err := embSvc.Embed(ctx, query)
    if err != nil {
        return err
    }

    // Query the collection
    results, err := coll.Query(ctx, &vectorstore.Query{
        Embedding: vectorstore.NewEmbedding(
            queryEmbedding,
            embSvc.ModelName(),
        ),
        Limit:    5,
        MinScore: 0.7,
        Filters: vectorstore.Eq("source", "documentation"),
    })
    if err != nil {
        return err
    }

    // Process results
    for _, match := range results.Matches {
        fmt.Printf("Score: %.3f - %s\n", match.Score, match.Document.Content.String())
    }

    return nil
}
```

## Best Practices

### Chunking Strategy

Break documents into semantic chunks (500-1000 tokens):

```go
func chunkDocument(text string, chunkSize int) []string {
    // Split by sentences or paragraphs
    // Maintain context overlap between chunks
    // Include metadata (source document, chapter, etc.)
}
```

### Metadata Design

Store useful filters:

```yaml
metadata:
  source: "user-manual"
  version: "2.1"
  category: "installation"
  date: "2025-01-20"
  author: "docs-team"
  language: "en"
```

### Query Optimization

- **Rewrite queries**: Expand abbreviations, fix typos
- **Multiple retrievals**: Try different query formulations
- **Reranking**: Use a second model to re-score results
- **Hybrid search**: Combine vector search with keyword filters

## Troubleshooting

### Firestore Permission Denied

```
Error: rpc error: code = PermissionDenied
```

**Solution**: Verify service account has `roles/datastore.user` role

### Index Not Ready

```
Error: index not found or not ready
```

**Solution**: Wait for index creation (can take 5-10 minutes)

```bash
gcloud firestore indexes composite list --format=table
```

### HuggingFace Rate Limit

```text
Error: rate limit exceeded
```

**Solution**: Add your HuggingFace API key:

```yaml
embeddings:
  huggingface:
    api_key: ${HUGGINGFACE_API_KEY}
```

Get a free key at: <https://huggingface.co/settings/tokens>

### Embedding Dimension Mismatch

```text
Error: embedding dimension mismatch: expected 384, got 1536
```

**Solution**: Ensure `embedding_dimensions` matches your model:

```text
all-MiniLM-L6-v2: 384
text-embedding-3-small: 1536
text-embedding-3-large: 3072
```

## Performance Tuning

### Batch Processing

```gogo
// Faster than individual Embed() calls
embeddings, err := embSvc.EmbedBatch(ctx, []string{"text1", "text2", "text3"})
```

### Connection Pooling

Firestore client manages connection pooling automatically.

### Caching

Consider caching embeddings for frequently queried content:

```gogo
type EmbeddingCache struct {
    cache map[string][]float32
    mu    sync.RWMutex
}
```

## Next Steps

### Enhance Your RAG System

1. **Add Custom Providers**
   - See [Extending Aixgo Guide](../../web/content/guides/extending-aixgo.md)
   - Implement Qdrant or pgvector support

2. **Production Deployment**
   - See [Production Deployment Guide](../../web/content/guides/production-deployment.md)
   - Set up monitoring and alerts
   - Configure auto-scaling

3. **Improve Retrieval Quality**
   - Experiment with different embedding models
   - Optimize chunk size and overlap
   - Add reranking for better precision
   - Implement hybrid search (keywords + vectors)

4. **Add Observability**
   - Monitor embedding latency
   - Track search quality metrics
   - Log retrieval accuracy
   - Set up error alerting

5. **Evaluation & Testing**
   - Create test queries with expected results
   - Measure precision and recall
   - A/B test different models
   - Benchmark performance

### Learning Resources

- **Official Documentation**
  - [Aixgo Vector Store Package](../../pkg/vectorstore/README.md)
  - [Aixgo Embeddings Package](../../pkg/embeddings/README.md)
  - [Vector Databases Guide](../../web/content/guides/vector-databases.md)

- **External Resources**
  - [Firestore Vector Search Documentation](https://firebase.google.com/docs/firestore/vector-search)
  - [HuggingFace Sentence Transformers](https://www.sbert.net/)
  - [OpenAI Embeddings Guide](https://platform.openai.com/docs/guides/embeddings)
  - [RAG Best Practices](https://www.pinecone.io/learn/retrieval-augmented-generation/)

### Community & Support

- **GitHub Issues**: [Report bugs or request features](https://github.com/aixgo-dev/aixgo/issues)
- **Discussions**: [Ask questions and share ideas](https://github.com/aixgo-dev/aixgo/discussions)
- **Documentation**: [Full Aixgo documentation](https://aixgo.dev)

## Contributing

We welcome contributions! Here's how you can help:

1. **Report Issues**: Found a bug? [Open an issue](https://github.com/aixgo-dev/aixgo/issues)
2. **Improve Documentation**: Submit PRs to improve this example
3. **Add Examples**: Share your RAG implementations
4. **Add Providers**: Implement support for new vector databases

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for guidelines.
