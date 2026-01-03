---
title: 'Vector Databases in Aixgo'
description: 'Complete guide to building RAG systems with vector databases and embeddings'
weight: 4
---

# Vector Databases in Aixgo

This guide covers everything you need to build production-ready Retrieval-Augmented Generation (RAG) systems using Aixgo's vector database and embeddings integration.

## Overview

Vector databases enable semantic search by storing high-dimensional embeddings alongside your data. Combined with LLMs, they power RAG systems that reduce hallucinations and
provide domain-specific knowledge.

### What You'll Learn

- Vector database fundamentals and the Collection-based architecture
- Choosing the right embedding provider
- Implementing semantic search and RAG systems
- 10 powerful use cases: caching, agent memory, conversations, and more
- Production deployment strategies with Firestore
- Performance optimization and best practices
- Migrating from the old API
- Troubleshooting common issues

## Understanding Vector Databases

### What are Embeddings?

Embeddings are numerical representations of text that capture semantic meaning. Similar texts produce similar vectors, enabling:

- **Semantic Search**: Find by meaning, not keywords
- **Similarity Matching**: Compare documents for relevance
- **Clustering**: Group related content
- **Recommendations**: Suggest similar items

**Example:**

```text
"dog" → [0.2, 0.8, 0.1, ...]
"puppy" → [0.21, 0.79, 0.11, ...] (similar vector)
"car" → [0.9, 0.1, 0.05, ...] (different vector)
```

### How RAG Works

```text
┌──────────────────────────────────────────────────┐
│ 1. INDEXING (One-time)                           │
│                                                  │
│  Documents → Embeddings → Vector Database       │
└──────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────┐
│ 2. RETRIEVAL (Per query)                         │
│                                                  │
│  Query → Embedding → Similarity Search          │
│       → Top K Documents                          │
└──────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────┐
│ 3. GENERATION (Per query)                        │
│                                                  │
│  Retrieved Context + Query → LLM → Response     │
└──────────────────────────────────────────────────┘
```

## Quick Start

### Minimal Example

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

    // 1. Setup embeddings (free HuggingFace)
    embConfig := embeddings.Config{
        Provider: "huggingface",
        HuggingFace: &embeddings.HuggingFaceConfig{
            Model: "sentence-transformers/all-MiniLM-L6-v2",
        },
    }
    embSvc, _ := embeddings.New(embConfig)
    defer embSvc.Close()

    // 2. Setup vector store (in-memory for testing)
    store, _ := memory.New()
    defer store.Close()

    // 3. Create a collection for documents
    docs := store.Collection("documents")

    // 4. Index documents
    contents := []string{
        "Aixgo is a production-grade AI framework for Go",
        "Go is a programming language created at Google",
        "Vector databases enable semantic search",
    }

    for i, content := range contents {
        emb, _ := embSvc.Embed(ctx, content)
        doc := &vectorstore.Document{
            ID:        fmt.Sprintf("doc-%d", i),
            Content:   vectorstore.NewTextContent(content),
            Embedding: vectorstore.NewEmbedding(emb, "all-MiniLM-L6-v2"),
        }
        docs.Upsert(ctx, doc)
    }

    // 5. Search
    query := "What is Aixgo?"
    queryEmb, _ := embSvc.Embed(ctx, query)

    result, _ := docs.Query(ctx, &vectorstore.Query{
        Embedding: vectorstore.NewEmbedding(queryEmb, "all-MiniLM-L6-v2"),
        Limit:     3,
    })

    // 6. Display results
    for _, match := range result.Matches {
        fmt.Printf("Score: %.2f - %s\n", match.Score, match.Document.Content.String())
    }
}
```

## Collection-Based Architecture

Aixgo's vector store uses a **Collection-based architecture** that provides logical isolation for different use cases. Each collection can have its own configuration for TTL,
deduplication, scoping, and capacity limits.

### Core Concepts

**Collections**: Logical containers for related documents with shared configuration.

```go
// Create collections with different purposes
cache := store.Collection("cache",
    vectorstore.WithTTL(5*time.Minute),
    vectorstore.WithDeduplication(true),
)

memory := store.Collection("agent-memory",
    vectorstore.WithScope("user", "session"),
    vectorstore.WithMaxDocuments(1000),
)

docs := store.Collection("documents",
    vectorstore.WithMaxDocuments(100000),
)
```

**Documents**: Enhanced with multi-modal content, scoping, temporal data, and tags.

```go
doc := &vectorstore.Document{
    ID:        "doc1",
    Content:   vectorstore.NewTextContent("text content"),
    Embedding: vectorstore.NewEmbedding(emb, "model-name"),
    Scope:     vectorstore.NewScope("tenant1", "user123", "session456"),
    Tags:      []string{"important", "verified"},
    Temporal:  vectorstore.NewTemporalWithTTL(24*time.Hour),
    Metadata:  map[string]any{"source": "api"},
}
```

**Queries**: Advanced filtering with scope, tags, temporal conditions, and sorting.

```go
result, _ := docs.Query(ctx, &vectorstore.Query{
    Embedding: vectorstore.NewEmbedding(emb, "model"),
    Limit:     10,
    MinScore:  0.7,
    Filters: vectorstore.And(
        vectorstore.UserFilter("user123"),
        vectorstore.TagFilter("important"),
        vectorstore.CreatedAfter(time.Now().Add(-7*24*time.Hour)),
    ),
    SortBy: []vectorstore.SortBy{
        vectorstore.SortByScore(),
        vectorstore.SortByCreatedAt(true),
    },
})
```

## 10 Powerful Use Cases

### 1. Semantic Caching

Cache LLM responses to reduce costs and latency.

```go
// Create cache collection with TTL
cache := store.Collection("llm-cache",
    vectorstore.WithTTL(5*time.Minute),
    vectorstore.WithDeduplication(true),
    vectorstore.WithMaxDocuments(10000),
)

// Cache a query result
cacheDoc := &vectorstore.Document{
    ID:      "query-hash-123",
    Content: vectorstore.NewTextContent("What is the capital of France?"),
    Embedding: vectorstore.NewEmbedding(queryEmb, "text-embedding-3-small"),
    Temporal: vectorstore.NewTemporalWithTTL(5 * time.Minute),
    Tags:    []string{"qa", "geography"},
    Metadata: map[string]any{
        "answer": "Paris",
        "model":  "gpt-4",
    },
}
cache.Upsert(ctx, cacheDoc)

// Lookup cached result by similarity
query := &vectorstore.Query{
    Embedding: vectorstore.NewEmbedding(queryEmb, "text-embedding-3-small"),
    Limit:     1,
    MinScore:  0.98, // High threshold for cache hits
}

result, _ := cache.Query(ctx, query)
if result.HasMatches() {
    answer := result.TopMatch().Document.Metadata["answer"]
    fmt.Printf("Cache hit! Answer: %s\n", answer)
}
```

### 2. Agent Memory with Scope Isolation

Store agent memories with multi-tenant isolation.

```go
// Create memory collection with scope requirements
memory := store.Collection("agent-memory",
    vectorstore.WithScope("tenant", "user", "session"),
    vectorstore.WithMaxDocuments(1000),
)

// Store a memory scoped to tenant, user, and session
memoryDoc := &vectorstore.Document{
    ID:      "memory-1",
    Content: vectorstore.NewTextContent("User prefers dark mode"),
    Embedding: vectorstore.NewEmbedding(emb, "text-embedding-3-small"),
    Scope:   vectorstore.NewScope("tenant1", "user123", "session456"),
    Tags:    []string{"preference", "ui"},
}
memory.Upsert(ctx, memoryDoc)

// Retrieve memories for specific user
result, _ := memory.Query(ctx, &vectorstore.Query{
    Embedding: vectorstore.NewEmbedding(queryEmb, "text-embedding-3-small"),
    Filters: vectorstore.And(
        vectorstore.TenantFilter("tenant1"),
        vectorstore.UserFilter("user123"),
        vectorstore.TagFilter("preference"),
    ),
    Limit: 5,
})
```

### 3. Conversation History

Track conversation context across sessions.

```go
conversations := store.Collection("conversations",
    vectorstore.WithScope("user", "thread"),
    vectorstore.WithMaxDocuments(100000),
)

// Store a conversation turn
turn := &vectorstore.Document{
    ID:      "turn-1",
    Content: vectorstore.NewTextContent("User: What's the weather?\nAssistant: It's sunny today."),
    Embedding: vectorstore.NewEmbedding(emb, "text-embedding-3-small"),
    Scope: &vectorstore.Scope{
        User:   "user123",
        Thread: "thread-abc",
    },
    Temporal: &vectorstore.Temporal{
        CreatedAt: time.Now(),
        EventTime: &[]time.Time{time.Now()}[0],
    },
    Tags: []string{"weather", "conversation"},
    Metadata: map[string]any{"turn_number": 1},
}
conversations.Upsert(ctx, turn)

// Retrieve conversation history chronologically
result, _ := conversations.Query(ctx, &vectorstore.Query{
    Filters: vectorstore.And(
        vectorstore.UserFilter("user123"),
        vectorstore.Eq("thread", "thread-abc"),
    ),
    SortBy: []vectorstore.SortBy{
        vectorstore.SortByCreatedAt(false), // Ascending
    },
    Limit: 20,
})
```

### 4. Content Deduplication

Prevent duplicate content in your knowledge base.

```go
docs := store.Collection("documents",
    vectorstore.WithDeduplicationThreshold(0.95),
)

// Insert multiple documents - duplicates are automatically detected
documents := []*vectorstore.Document{
    {
        ID:        "doc1",
        Content:   vectorstore.NewTextContent("The quick brown fox"),
        Embedding: vectorstore.NewEmbedding(emb1, "model"),
    },
    {
        ID:        "doc2",
        Content:   vectorstore.NewTextContent("The quick brown fox"), // Duplicate
        Embedding: vectorstore.NewEmbedding(emb1, "model"),
    },
}

result, _ := docs.UpsertBatch(ctx, documents)
fmt.Printf("Inserted: %d, Deduplicated: %d\n", result.Inserted, result.Deduplicated)
```

### 5. Multi-Modal Support

Store and search images, URLs, and text.

```go
media := store.Collection("media")

// Store an image with CLIP embeddings
imageDoc := &vectorstore.Document{
    ID:      "img1",
    Content: vectorstore.NewImageURL("https://example.com/photo.jpg"),
    Embedding: vectorstore.NewEmbedding(clipEmb, "clip-vit-base-patch32"),
    Tags:    []string{"photo", "landscape"},
}
media.Upsert(ctx, imageDoc)

// Query with image or text embedding
result, _ := media.Query(ctx, &vectorstore.Query{
    Embedding: vectorstore.NewEmbedding(queryEmb, "clip-vit-base-patch32"),
    Filters:   vectorstore.TagFilter("photo"),
    Limit:     10,
})
```

### 6. Temporal Data with Expiration

Manage time-based data with automatic cleanup.

```go
events := store.Collection("events",
    vectorstore.WithTTL(7*24*time.Hour), // 7 days
)

// Store event with TTL
event := &vectorstore.Document{
    ID:      "event-1",
    Content: vectorstore.NewTextContent("System maintenance scheduled"),
    Embedding: vectorstore.NewEmbedding(emb, "model"),
    Temporal: vectorstore.NewTemporalWithTTL(24 * time.Hour),
    Tags:    []string{"maintenance", "scheduled"},
}
events.Upsert(ctx, event)

// Query for recent, non-expired events
result, _ := events.Query(ctx, &vectorstore.Query{
    Filters: vectorstore.And(
        vectorstore.CreatedAfter(time.Now().Add(-7*24*time.Hour)),
        vectorstore.NotExpired(),
        vectorstore.TagFilter("scheduled"),
    ),
    SortBy: []vectorstore.SortBy{
        vectorstore.SortByCreatedAt(true), // Most recent first
    },
})
```

### 7. Multi-Tenancy via Scope

Isolate data across tenants, users, and sessions.

```go
// Index with tenant isolation
doc := &vectorstore.Document{
    ID:      "doc1",
    Content: vectorstore.NewTextContent("Sensitive company data"),
    Embedding: vectorstore.NewEmbedding(emb, "model"),
    Scope:   vectorstore.NewScope("acme-corp", "user456", ""),
}
docs.Upsert(ctx, doc)

// Search within tenant boundary
result, _ := docs.Query(ctx, &vectorstore.Query{
    Embedding: vectorstore.NewEmbedding(queryEmb, "model"),
    Filters: vectorstore.And(
        vectorstore.TenantFilter("acme-corp"),
        vectorstore.UserFilter("user456"),
    ),
})
```

### 8. Batch Operations

Efficiently process large datasets with progress tracking.

```go
// Create many documents
documents := make([]*vectorstore.Document, 1000)
for i := range documents {
    documents[i] = &vectorstore.Document{
        ID:      fmt.Sprintf("doc-%d", i),
        Content: vectorstore.NewTextContent(fmt.Sprintf("Document %d", i)),
        Embedding: vectorstore.NewEmbedding(embs[i], "model"),
    }
}

// Batch insert with progress tracking
result, _ := docs.UpsertBatch(ctx, documents,
    vectorstore.WithBatchSize(100),
    vectorstore.WithParallelism(4),
    vectorstore.WithProgressCallback(func(processed, total int) {
        pct := float64(processed) / float64(total) * 100
        fmt.Printf("Progress: %.1f%%\n", pct)
    }),
)

fmt.Printf("Inserted: %d, Failed: %d\n", result.Inserted, result.Failed)
```

### 9. Streaming Queries

Process large result sets efficiently.

```go
query := &vectorstore.Query{
    Embedding: vectorstore.NewEmbedding(emb, "model"),
    Limit:     1000, // Large result set
}

// Stream results to avoid loading all at once
iter, _ := docs.QueryStream(ctx, query)
defer iter.Close()

count := 0
for iter.Next() {
    match := iter.Match()
    if match.Score >= 0.8 {
        count++
        // Process high-score match
    }
}

if err := iter.Err(); err != nil {
    log.Printf("Stream error: %v", err)
}
```

### 10. Advanced Filtering

Complex queries with boolean logic.

```go
// Find recent, high-rated electronics in stock
result, _ := docs.Query(ctx, &vectorstore.Query{
    Embedding: vectorstore.NewEmbedding(emb, "model"),
    Filters: vectorstore.And(
        vectorstore.TagFilter("electronics"),
        vectorstore.Gte("rating", 4.5),
        vectorstore.Eq("in_stock", true),
        vectorstore.CreatedAfter(time.Now().Add(-30*24*time.Hour)),
        vectorstore.Or(
            vectorstore.Contains("category", "phone"),
            vectorstore.Contains("category", "laptop"),
        ),
    ),
    SortBy: []vectorstore.SortBy{
        vectorstore.SortByScore(),
        vectorstore.SortByField("rating", true),
    },
    Limit: 20,
})
```

## Choosing Components

### Embedding Providers

#### Embedding Provider Decision Matrix

| Provider            | Cost             | Setup   | Quality        | Speed     | Best For    |
| ------------------- | ---------------- | ------- | -------------- | --------- | ----------- |
| **HuggingFace API** | Free             | None    | Good-Excellent | Medium    | Development |
| **HuggingFace TEI** | Free (self-host) | Docker  | Good-Excellent | Very Fast | Production  |
| **OpenAI**          | $0.02-0.13/1M    | API Key | Excellent      | Fast      | Production  |

#### HuggingFace Models

**Popular choices:**

```go
// Fast, good quality (384 dimensions)
Model: "sentence-transformers/all-MiniLM-L6-v2"

// Best quality (1024 dimensions)
Model: "BAAI/bge-large-en-v1.5"

// Multilingual (1024 dimensions)
Model: "thenlper/gte-large"
```

**Configuration:**

```go
config := embeddings.Config{
    Provider: "huggingface",
    HuggingFace: &embeddings.HuggingFaceConfig{
        Model:        "sentence-transformers/all-MiniLM-L6-v2",
        APIKey:       os.Getenv("HUGGINGFACE_API_KEY"), // Optional
        WaitForModel: true,
        UseCache:     true,
    },
}
```

#### OpenAI Models

**Available models:**

```go
// Recommended: Balance of cost and quality
Model: "text-embedding-3-small"  // 1536 dims, $0.02/1M tokens

// Best quality
Model: "text-embedding-3-large"  // 3072 dims, $0.13/1M tokens

// Legacy (still supported)
Model: "text-embedding-ada-002"  // 1536 dims, $0.10/1M tokens
```

**Configuration:**

```go
config := embeddings.Config{
    Provider: "openai",
    OpenAI: &embeddings.OpenAIConfig{
        APIKey: os.Getenv("OPENAI_API_KEY"),
        Model:  "text-embedding-3-small",
    },
}
```

### Vector Store Providers

#### Decision Matrix

| Provider              | Persistence | Scalability    | Setup  | Best For                 |
| --------------------- | ----------- | -------------- | ------ | ------------------------ |
| **Memory**            | No          | Low (10K docs) | None   | Development, testing     |
| **Firestore**         | Yes         | Unlimited      | Medium | Production, serverless   |
| **Qdrant** (future)   | Yes         | Very High      | Medium | High-performance search  |
| **pgvector** (future) | Yes         | High           | Medium | Existing PostgreSQL apps |

#### Memory Store

**Configuration:**

```go
import "github.com/aixgo-dev/aixgo/pkg/vectorstore/memory"

store, _ := memory.New()
defer store.Close()

// Create collections with different configurations
cache := store.Collection("cache", vectorstore.WithTTL(5*time.Minute))
docs := store.Collection("docs", vectorstore.WithMaxDocuments(10000))
```

**Use Cases:**

- Unit tests
- Local development
- Prototyping
- Small datasets

#### Firestore

**Configuration:**

```go
import "github.com/aixgo-dev/aixgo/pkg/vectorstore/firestore"

store, _ := firestore.New(ctx,
    firestore.WithProjectID("my-gcp-project"),
    firestore.WithCredentialsFile("/path/to/service-account.json"),
)
defer store.Close()

// Create collections
docs := store.Collection("documents")
```

**Use Cases:**

- Production deployments
- Serverless architectures
- Firebase-based apps
- Auto-scaling workloads

## Production Setup

### Firestore Configuration

#### 1. Create GCP Project

```bash
# Create project
gcloud projects create my-rag-project
gcloud config set project my-rag-project

# Enable billing (required)
gcloud beta billing projects link my-rag-project \
  --billing-account=BILLING_ACCOUNT_ID
```

#### 2. Enable Firestore

```bash
# Enable Firestore API
gcloud services enable firestore.googleapis.com

# Create database
gcloud firestore databases create \
  --location=us-central1 \
  --type=firestore-native
```

#### 3. Create Vector Index

**Critical:** Firestore requires vector indexes for similarity search.

```bash
# For 384-dimensional embeddings (all-MiniLM-L6-v2)
gcloud firestore indexes composite create \
  --collection-group=documents \
  --query-scope=COLLECTION \
  --field-config=field-path=embedding.vector,vector-config='{"dimension":"384","flat":{}}' \
  --project=my-rag-project

# For 1536-dimensional embeddings (OpenAI)
gcloud firestore indexes composite create \
  --collection-group=documents \
  --query-scope=COLLECTION \
  --field-config=field-path=embedding.vector,vector-config='{"dimension":"1536","flat":{}}' \
  --project=my-rag-project
```

**Check index status:**

```bash
gcloud firestore indexes composite list --format=table
```

#### 4. Setup Authentication

```bash
# Create service account
gcloud iam service-accounts create aixgo-rag \
  --display-name="Aixgo RAG Service"

# Grant Firestore permissions
gcloud projects add-iam-policy-binding my-rag-project \
  --member="serviceAccount:aixgo-rag@my-rag-project.iam.gserviceaccount.com" \
  --role="roles/datastore.user"

# Create and download key
gcloud iam service-accounts keys create key.json \
  --iam-account=aixgo-rag@my-rag-project.iam.gserviceaccount.com

# Set environment variable
export GOOGLE_APPLICATION_CREDENTIALS=$(pwd)/key.json
```

### HuggingFace TEI (Self-Hosted)

For high-throughput production deployments:

#### Docker Deployment

```bash
# Run TEI server
docker run -d \
  --name tei \
  -p 8080:8080 \
  -v $PWD/data:/data \
  --gpus all \
  ghcr.io/huggingface/text-embeddings-inference:latest \
  --model-id BAAI/bge-large-en-v1.5 \
  --revision main \
  --max-batch-size 128
```

#### Configuration

```go
config := embeddings.Config{
    Provider: "huggingface_tei",
    HuggingFaceTEI: &embeddings.HuggingFaceTEIConfig{
        Endpoint:  "http://localhost:8080",
        Model:     "BAAI/bge-large-en-v1.5",
        Normalize: true,
    },
}
```

## Building RAG Systems

### Complete RAG Implementation

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "strings"

    "github.com/aixgo-dev/aixgo/pkg/embeddings"
    "github.com/aixgo-dev/aixgo/pkg/llm"
    "github.com/aixgo-dev/aixgo/pkg/vectorstore"
    "github.com/aixgo-dev/aixgo/pkg/vectorstore/firestore"
)

type RAGSystem struct {
    embeddings embeddings.EmbeddingService
    vectorDB   vectorstore.VectorStore
    collection vectorstore.Collection
    llm        llm.LLM
}

func NewRAGSystem() (*RAGSystem, error) {
    ctx := context.Background()

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
    store, err := firestore.New(ctx,
        firestore.WithProjectID(os.Getenv("GCP_PROJECT_ID")),
    )
    if err != nil {
        return nil, err
    }

    // Create knowledge base collection
    knowledgeBase := store.Collection("knowledge_base",
        vectorstore.WithDeduplication(true),
        vectorstore.WithMaxDocuments(100000),
    )

    // Setup LLM
    llmInstance, err := llm.New(llm.Config{
        Provider: "openai",
        OpenAI: &llm.OpenAIConfig{
            APIKey: os.Getenv("OPENAI_API_KEY"),
            Model:  "gpt-4-turbo-preview",
        },
    })
    if err != nil {
        return nil, err
    }

    return &RAGSystem{
        embeddings: embSvc,
        vectorDB:   store,
        collection: knowledgeBase,
        llm:        llmInstance,
    }, nil
}

// IndexDocument adds a document to the knowledge base
func (r *RAGSystem) IndexDocument(ctx context.Context, id, content string, metadata map[string]any) error {
    // Generate embedding
    embedding, err := r.embeddings.Embed(ctx, content)
    if err != nil {
        return fmt.Errorf("failed to generate embedding: %w", err)
    }

    // Create document
    doc := &vectorstore.Document{
        ID:        id,
        Content:   vectorstore.NewTextContent(content),
        Embedding: vectorstore.NewEmbedding(embedding, "BAAI/bge-large-en-v1.5"),
        Metadata:  metadata,
        Tags:      []string{"knowledge-base"},
    }

    // Store in vector DB
    _, err = r.collection.Upsert(ctx, doc)
    return err
}

// Query performs RAG: retrieve + generate
func (r *RAGSystem) Query(ctx context.Context, question string, topK int) (string, error) {
    // 1. Generate query embedding
    queryEmb, err := r.embeddings.Embed(ctx, question)
    if err != nil {
        return "", fmt.Errorf("failed to embed query: %w", err)
    }

    // 2. Retrieve relevant documents
    result, err := r.collection.Query(ctx, &vectorstore.Query{
        Embedding: vectorstore.NewEmbedding(queryEmb, "BAAI/bge-large-en-v1.5"),
        Limit:     topK,
        MinScore:  0.7,
    })
    if err != nil {
        return "", fmt.Errorf("search failed: %w", err)
    }

    if !result.HasMatches() {
        return "I don't have enough information to answer that question.", nil
    }

    // 3. Build context from retrieved documents
    var contextParts []string
    for i, match := range result.Matches {
        contextParts = append(contextParts,
            fmt.Sprintf("[Document %d] (score: %.2f)\n%s",
                i+1, match.Score, match.Document.Content.String()))
    }
    context := strings.Join(contextParts, "\n\n")

    // 4. Generate response with LLM
    prompt := fmt.Sprintf(`Based on the following context, please answer the question.

Context:
%s

Question: %s

Answer:`, context, question)

    response, err := r.llm.Complete(ctx, prompt)
    if err != nil {
        return "", fmt.Errorf("LLM generation failed: %w", err)
    }

    return response, nil
}

func (r *RAGSystem) Close() error {
    r.embeddings.Close()
    r.vectorDB.Close()
    return nil
}
```

### Usage Example

```go
func main() {
    ctx := context.Background()

    // Initialize RAG system
    rag, err := NewRAGSystem()
    if err != nil {
        log.Fatal(err)
    }
    defer rag.Close()

    // Index documents
    documents := map[string]string{
        "doc-1": "Aixgo is a production-grade AI agent framework written in Go.",
        "doc-2": "Aixgo supports multiple LLM providers including OpenAI, Anthropic, and Gemini.",
        "doc-3": "Vector databases in Aixgo enable semantic search and RAG systems.",
    }

    for id, content := range documents {
        err := rag.IndexDocument(ctx, id, content, map[string]any{
            "source": "documentation",
        })
        if err != nil {
            log.Printf("Failed to index %s: %v", id, err)
        }
    }

    // Query the system
    answer, err := rag.Query(ctx, "What LLM providers does Aixgo support?", 3)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Answer:", answer)
}
```

## Best Practices

### 1. Document Chunking

Break large documents into optimal chunks:

```go
func chunkDocument(text string, chunkSize int, overlap int) []string {
    var chunks []string
    words := strings.Fields(text)

    for i := 0; i < len(words); i += chunkSize - overlap {
        end := i + chunkSize
        if end > len(words) {
            end = len(words)
        }

        chunk := strings.Join(words[i:end], " ")
        chunks = append(chunks, chunk)

        if end == len(words) {
            break
        }
    }

    return chunks
}

// Usage
chunks := chunkDocument(largeDocument, 500, 50) // 500 words, 50 overlap
for i, chunk := range chunks {
    emb, _ := embSvc.Embed(ctx, chunk)
    doc := &vectorstore.Document{
        ID:        fmt.Sprintf("doc-%s-chunk-%d", docID, i),
        Content:   vectorstore.NewTextContent(chunk),
        Embedding: vectorstore.NewEmbedding(emb, "model"),
        Metadata: map[string]any{
            "chunk_index": i,
            "parent_doc":  docID,
        },
    }
    docs.Upsert(ctx, doc)
}
```

**Recommended sizes:**

- 200-500 words per chunk
- 10-20% overlap between chunks
- Preserve sentence boundaries

### 2. Metadata and Tags Strategy

Use metadata and tags for hybrid search:

```go
doc := &vectorstore.Document{
    ID:      "doc1",
    Content: vectorstore.NewTextContent("Installation guide for Aixgo..."),
    Embedding: vectorstore.NewEmbedding(emb, "model"),
    Tags:    []string{"getting-started", "setup", "installation"},
    Metadata: map[string]any{
        "doc_type":   "user_guide",
        "section":    "installation",
        "version":    "2.0",
        "language":   "en",
        "created_at": time.Now().Format(time.RFC3339),
        "author":     "docs-team",
    },
}

// Filter during search
result, _ := docs.Query(ctx, &vectorstore.Query{
    Embedding: vectorstore.NewEmbedding(queryEmb, "model"),
    Filters: vectorstore.And(
        vectorstore.TagFilter("installation"),
        vectorstore.Eq("doc_type", "user_guide"),
        vectorstore.Eq("version", "2.0"),
    ),
    Limit: 10,
})
```

### 3. Batch Processing

Index efficiently with batch operations:

```go
func indexDocuments(collection vectorstore.Collection, embSvc embeddings.EmbeddingService, docs []string) error {
    ctx := context.Background()
    const batchSize = 100

    for i := 0; i < len(docs); i += batchSize {
        end := i + batchSize
        if end > len(docs) {
            end = len(docs)
        }

        batch := docs[i:end]

        // Batch embed
        embeddings, err := embSvc.EmbedBatch(ctx, batch)
        if err != nil {
            return err
        }

        // Create documents
        var vsDocs []*vectorstore.Document
        for j, content := range batch {
            vsDocs = append(vsDocs, &vectorstore.Document{
                ID:        fmt.Sprintf("doc-%d", i+j),
                Content:   vectorstore.NewTextContent(content),
                Embedding: vectorstore.NewEmbedding(embeddings[j], "model"),
            })
        }

        // Batch upsert
        _, err = collection.UpsertBatch(ctx, vsDocs)
        if err != nil {
            return err
        }
    }

    return nil
}
```

### 4. Error Handling and Retries

Implement robust error handling:

```go
func queryWithRetry(collection vectorstore.Collection, query *vectorstore.Query, maxRetries int) (*vectorstore.QueryResult, error) {
    var lastErr error

    for i := 0; i < maxRetries; i++ {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        result, err := collection.Query(ctx, query)
        if err == nil {
            return result, nil
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
    // Retry on rate limits, timeouts, network errors
    errStr := err.Error()
    return strings.Contains(errStr, "rate limit") ||
           strings.Contains(errStr, "timeout") ||
           strings.Contains(errStr, "connection")
}
```

### 5. Caching Strategies

Implement multi-level caching for frequently accessed content:

```go
type CachedEmbeddings struct {
    cache sync.Map
    svc   embeddings.EmbeddingService
}

func (ce *CachedEmbeddings) Embed(ctx context.Context, text string) ([]float32, error) {
    // Check cache
    if emb, ok := ce.cache.Load(text); ok {
        return emb.([]float32), nil
    }

    // Generate embedding
    emb, err := ce.svc.Embed(ctx, text)
    if err != nil {
        return nil, err
    }

    // Cache result
    ce.cache.Store(text, emb)
    return emb, nil
}
```

## Performance Optimization

### Latency Optimization

**Target latencies:**

- Embedding generation: < 100ms
- Vector search: < 50ms
- Total RAG latency: < 500ms

**Optimization strategies:**

1. **Use batch operations** for bulk indexing
2. **Deploy TEI for embeddings** (10x faster than API)
3. **Reduce query limit** (fewer results = faster)
4. **Add filters** to reduce search space
5. **Cache embeddings** for repeated queries
6. **Use streaming** for large result sets

### Cost Optimization

**For OpenAI embeddings:**

```go
// Calculate costs
const avgTokensPerDoc = 100
const documentsToIndex = 100000
const embeddingCost = 0.00002 / 1000 // text-embedding-3-small per token

totalTokens := avgTokensPerDoc * documentsToIndex
totalCost := totalTokens * embeddingCost

fmt.Printf("Indexing cost: $%.2f\n", totalCost)
// Output: Indexing cost: $0.20
```

**Cost-saving tips:**

1. Use HuggingFace for development (free)
2. Cache embeddings for reused content
3. Choose smaller models if acceptable
4. Use batch operations (no rate limit overhead)
5. Implement semantic caching to reduce LLM calls

### Firestore Costs

**Pricing (as of 2025):**

- Reads: $0.06 per 100K
- Writes: $0.18 per 100K
- Storage: $0.18 per GB/month

**Example cost:**

- 100K documents indexed: ~$18
- 1M searches/month: ~$600
- 10GB storage: ~$1.80/month

## Migration Guide

### From Old API to Collection-Based API

The vector store API was redesigned with Collections. Here's how to migrate:

#### Old API (Before)

```go
// Old: Config-based initialization
config := vectorstore.Config{
    Provider: "memory",
    EmbeddingDimensions: 384,
    Memory: &vectorstore.MemoryConfig{MaxDocuments: 10000},
}
store, _ := vectorstore.New(config)

// Old: Simple document structure
doc := vectorstore.Document{
    ID:        "doc1",
    Content:   "text content",      // Simple string
    Embedding: []float32{...},
    Metadata:  map[string]interface{}{},
}

// Old: Direct store methods
store.Upsert(ctx, []vectorstore.Document{doc})
results, _ := store.Search(ctx, vectorstore.SearchQuery{
    Embedding: emb,
    TopK:      10,
})
```

#### New API (After)

```go
// New: Provider-specific constructors
import "github.com/aixgo-dev/aixgo/pkg/vectorstore/memory"
store, _ := memory.New()

// New: Collection-based isolation
docs := store.Collection("documents",
    vectorstore.WithMaxDocuments(10000),
)

// New: Enhanced document structure
doc := &vectorstore.Document{
    ID:      "doc1",
    Content: vectorstore.NewTextContent("text content"),  // Multi-modal
    Embedding: vectorstore.NewEmbedding([]float32{...}, "model"),
    Tags:    []string{"important"},
    Metadata: map[string]any{"source": "api"},
}

// New: Collection methods
docs.Upsert(ctx, doc)
result, _ := docs.Query(ctx, &vectorstore.Query{
    Embedding: vectorstore.NewEmbedding(emb, "model"),
    Limit:     10,
})
```

#### Key Changes

1. **Provider Initialization**: Use provider-specific packages (`memory.New()`, `firestore.New()`)
2. **Collections**: Always work through collections for logical isolation
3. **Document Structure**: Use typed content (`NewTextContent`, `NewImageURL`)
4. **Embeddings**: Include model name with `NewEmbedding(vector, model)`
5. **Queries**: Use `Query` struct instead of `SearchQuery`, `Limit` instead of `TopK`
6. **Results**: Access via `result.Matches` instead of array directly
7. **Pointers**: Documents are now pointers (`*vectorstore.Document`)

## Troubleshooting

### Common Issues

#### 1. Embedding Dimension Mismatch

```text
Error: embedding dimension mismatch: expected 384, got 1536
```

**Solution:**

```go
// Ensure collection dimensions match your embedding model
embSvc, _ := embeddings.New(config)
dims := embSvc.Dimensions()

// For memory store (auto-detects)
store, _ := memory.New()

// For Firestore, create index with correct dimensions
// See "Create Vector Index" section above
```

#### 2. Firestore Index Not Ready

```text
Error: index not found or not ready
```

**Solution:** Wait 5-10 minutes for index creation, then check:

```bash
gcloud firestore indexes composite list
```

**Note:** The index field path changed to `embedding.vector` in the new API.

#### 3. HuggingFace Rate Limit

```text
Error: rate limit exceeded
```

**Solutions:**

1. Get free API key: <https://huggingface.co/settings/tokens>
2. Use batch operations (`EmbedBatch`)
3. Deploy TEI locally for unlimited requests

#### 4. Low Search Quality

**Debugging steps:**

```go
// 1. Check similarity scores
for _, match := range result.Matches {
    fmt.Printf("Score: %.3f - %s\n", match.Score, match.Document.Content.String())
}

// 2. Lower MinScore threshold
query.MinScore = 0.5 // or 0.0 to see all results

// 3. Increase limit
query.Limit = 20 // see more candidates

// 4. Try different embedding model
// Larger models (1024 dims) often perform better
```

**Improvement strategies:**

1. Use better embedding model (bge-large vs all-MiniLM)
2. Optimize chunk size (test different sizes)
3. Add reranking step with cross-encoder
4. Implement hybrid search (semantic + keyword)

#### 5. Collection Not Found

```text
Error: collection does not exist
```

**Solution:**

```go
// Collections are created on first use
docs := store.Collection("documents") // Creates if doesn't exist

// Or check existing collections
collections, _ := store.ListCollections(ctx)
fmt.Println("Available collections:", collections)
```

## Advanced Topics

### Hybrid Search

Combine vector similarity with filters:

```go
// Semantic search with metadata filters
result, _ := docs.Query(ctx, &vectorstore.Query{
    Embedding: vectorstore.NewEmbedding(queryEmb, "model"),
    Filters: vectorstore.And(
        vectorstore.TagFilter("documentation"),
        vectorstore.Eq("category", "api-reference"),
        vectorstore.Gte("version", "2.0"),
    ),
    Limit: 20,
})

// Post-process with keyword matching if needed
var filtered []*vectorstore.Match
for _, match := range result.Matches {
    if strings.Contains(strings.ToLower(match.Document.Content.String()), "aixgo") {
        filtered = append(filtered, match)
    }
}
```

### Pagination

Handle large result sets with pagination:

```go
pageSize := 20
page := 0

for {
    result, _ := docs.Query(ctx, &vectorstore.Query{
        Embedding: vectorstore.NewEmbedding(emb, "model"),
        Limit:     pageSize,
        Offset:    page * pageSize,
    })

    fmt.Printf("Page %d: %d results\n", page+1, result.Count())

    // Process results
    for _, match := range result.Matches {
        // Process match
    }

    // Check for more pages
    if !result.HasMore() {
        break
    }

    page++
}
```

### Versioning

Handle document versions:

```go
doc := &vectorstore.Document{
    ID:      "user-guide-v2",
    Content: vectorstore.NewTextContent("Updated installation guide..."),
    Embedding: vectorstore.NewEmbedding(emb, "model"),
    Tags:    []string{"latest"},
    Metadata: map[string]any{
        "doc_id":  "user-guide",
        "version": "2.0",
        "latest":  true,
    },
}

// Search latest versions only
result, _ := docs.Query(ctx, &vectorstore.Query{
    Embedding: vectorstore.NewEmbedding(queryEmb, "model"),
    Filters:   vectorstore.TagFilter("latest"),
})
```

## Next Steps

- **Try the Example**: [RAG Agent Example](../../examples/rag-agent)
- **API Reference**: [Vector Store Package](https://pkg.go.dev/github.com/aixgo-dev/aixgo/pkg/vectorstore)
- **Extend Aixgo**: [Adding Custom Providers](./extending-aixgo.md)
- **Production**: [Deployment Guide](./production-deployment.md)

## Resources

- [Firestore Vector Search](https://firebase.google.com/docs/firestore/vector-search)
- [HuggingFace Embeddings](https://huggingface.co/models?pipeline_tag=sentence-similarity)
- [OpenAI Embeddings](https://platform.openai.com/docs/guides/embeddings)
- [RAG Best Practices](https://www.pinecone.io/learn/retrieval-augmented-generation/)
