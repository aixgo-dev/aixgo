---
title: 'Extending Aixgo'
description: 'Guide to adding custom LLM providers, vector databases, and embedding services'
weight: 7
---

# Extending Aixgo

Aixgo is designed to be extensible. This guide shows you how to add custom providers for LLMs, vector databases, and embeddings.

## Overview

Aixgo uses a registry pattern that allows you to:

- Add custom LLM providers
- Implement new vector database backends
- Integrate custom embedding services
- Extend existing providers with additional features

### Provider Architecture

```text
┌─────────────────────────────────────────┐
│         Application Code                │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│      Provider-Agnostic Interface        │
│  (LLM, VectorStore, EmbeddingService)   │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│          Provider Registry              │
│   {"openai": Factory, "custom": ...}    │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│       Concrete Implementations          │
│  OpenAI, Anthropic, Custom, etc.        │
└─────────────────────────────────────────┘
```

## Adding a Custom Vector Store

Let's implement a Qdrant vector store as an example using Aixgo's Collection-based architecture.

### Step 1: Implement the VectorStore Interface

```go
// pkg/vectorstore/qdrant/qdrant.go
package qdrant

import (
    "context"
    "fmt"
    "sync"

    qdrantclient "github.com/qdrant/go-client/qdrant"
    "github.com/aixgo-dev/aixgo/pkg/vectorstore"
)

// QdrantStore implements the vectorstore.VectorStore interface
type QdrantStore struct {
    client      *qdrantclient.Client
    collections sync.Map // thread-safe collection cache
}

// Option configures the Qdrant store
type Option func(*QdrantStore) error

// WithHost sets the Qdrant server address
func WithHost(host string) Option {
    return func(q *QdrantStore) error {
        q.host = host
        return nil
    }
}

// WithAPIKey sets the API key for authentication
func WithAPIKey(apiKey string) Option {
    return func(q *QdrantStore) error {
        q.apiKey = apiKey
        return nil
    }
}

// New creates a new Qdrant vector store
func New(opts ...Option) (*QdrantStore, error) {
    store := &QdrantStore{
        host:   "localhost:6333",
        apiKey: "",
    }

    // Apply options
    for _, opt := range opts {
        if err := opt(store); err != nil {
            return nil, err
        }
    }

    // Create Qdrant client
    client, err := qdrantclient.NewClient(&qdrantclient.Config{
        Host:   store.host,
        APIKey: store.apiKey,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create qdrant client: %w", err)
    }

    store.client = client
    return store, nil
}

// Collection returns or creates a collection
func (q *QdrantStore) Collection(name string, opts ...vectorstore.CollectionOption) vectorstore.Collection {
    // Check cache
    if coll, ok := q.collections.Load(name); ok {
        return coll.(vectorstore.Collection)
    }

    // Create new collection
    config := vectorstore.NewCollectionConfig(opts...)
    coll := &QdrantCollection{
        store:  q,
        name:   name,
        config: config,
    }

    // Ensure Qdrant collection exists
    if err := coll.ensureExists(context.Background()); err != nil {
        // Log error but return collection anyway - it will fail on use
        fmt.Printf("Warning: failed to ensure collection exists: %v\n", err)
    }

    // Cache and return
    q.collections.Store(name, coll)
    return coll
}

// Close closes the Qdrant client
func (q *QdrantStore) Close() error {
    return q.client.Close()
}
```

### Step 2: Implement the Collection Interface

```go
// QdrantCollection implements vectorstore.Collection
type QdrantCollection struct {
    store  *QdrantStore
    name   string
    config *vectorstore.CollectionConfig
}

// ensureExists creates the Qdrant collection if it doesn't exist
func (c *QdrantCollection) ensureExists(ctx context.Context) error {
    exists, err := c.store.client.CollectionExists(ctx, c.name)
    if err != nil {
        return fmt.Errorf("failed to check collection: %w", err)
    }

    if !exists {
        // Create collection with vector configuration
        dims := c.config.Dimensions
        if dims == 0 {
            dims = 384 // Default dimension
        }

        err = c.store.client.CreateCollection(ctx, &qdrantclient.CreateCollection{
            CollectionName: c.name,
            VectorsConfig: &qdrantclient.VectorsConfig{
                Params: &qdrantclient.VectorParams{
                    Size:     uint64(dims),
                    Distance: qdrantclient.Distance_Cosine,
                },
            },
        })
        if err != nil {
            return fmt.Errorf("failed to create collection: %w", err)
        }
    }

    return nil
}

// Upsert inserts or updates a document
func (c *QdrantCollection) Upsert(ctx context.Context, doc *vectorstore.Document) (*vectorstore.UpsertResult, error) {
    // Convert to Qdrant point
    point := &qdrantclient.PointStruct{
        Id: &qdrantclient.PointId{
            PointIdOptions: &qdrantclient.PointId_Uuid{
                Uuid: doc.ID,
            },
        },
        Vectors: &qdrantclient.Vectors{
            VectorsOptions: &qdrantclient.Vectors_Vector{
                Vector: &qdrantclient.Vector{
                    Data: doc.Embedding.Vector,
                },
            },
        },
        Payload: buildPayload(doc),
    }

    // Upsert to Qdrant
    _, err := c.store.client.Upsert(ctx, &qdrantclient.UpsertPoints{
        CollectionName: c.name,
        Points:         []*qdrantclient.PointStruct{point},
    })
    if err != nil {
        return nil, fmt.Errorf("failed to upsert: %w", err)
    }

    return &vectorstore.UpsertResult{Inserted: 1}, nil
}

// Query performs similarity search
func (c *QdrantCollection) Query(ctx context.Context, query *vectorstore.Query) (*vectorstore.QueryResult, error) {
    // Build Qdrant search request
    searchParams := &qdrantclient.SearchPoints{
        CollectionName: c.name,
        Vector:         query.Embedding.Vector,
        Limit:          uint64(query.Limit),
        WithPayload:    &qdrantclient.WithPayloadSelector{
            SelectorOptions: &qdrantclient.WithPayloadSelector_Enable{Enable: true},
        },
    }

    if query.MinScore > 0 {
        searchParams.ScoreThreshold = &query.MinScore
    }

    // Add filters if provided
    if query.Filters != nil {
        searchParams.Filter = buildQdrantFilter(query.Filters)
    }

    // Execute search
    results, err := c.store.client.Search(ctx, searchParams)
    if err != nil {
        return nil, fmt.Errorf("search failed: %w", err)
    }

    // Convert results
    matches := make([]*vectorstore.Match, 0, len(results))
    for _, hit := range results {
        doc := parseQdrantPoint(hit)
        matches = append(matches, &vectorstore.Match{
            Document: doc,
            Score:    hit.Score,
        })
    }

    return &vectorstore.QueryResult{Matches: matches}, nil
}

// Delete removes documents by IDs
func (c *QdrantCollection) Delete(ctx context.Context, ids ...string) error {
    if len(ids) == 0 {
        return nil
    }

    // Convert string IDs to Qdrant point IDs
    pointIds := make([]*qdrantclient.PointId, len(ids))
    for i, id := range ids {
        pointIds[i] = &qdrantclient.PointId{
            PointIdOptions: &qdrantclient.PointId_Uuid{Uuid: id},
        }
    }

    // Delete from Qdrant
    _, err := c.store.client.Delete(ctx, &qdrantclient.DeletePoints{
        CollectionName: c.name,
        Points: &qdrantclient.PointsSelector{
            PointsSelectorOneOf: &qdrantclient.PointsSelector_Points{
                Points: &qdrantclient.PointsIdsList{Ids: pointIds},
            },
        },
    })

    return err
}

// Helper functions

func buildPayload(doc *vectorstore.Document) map[string]interface{} {
    payload := make(map[string]interface{})

    // Store content based on type
    switch content := doc.Content.(type) {
    case *vectorstore.TextContent:
        payload["content_type"] = "text"
        payload["content_text"] = content.Text
    case *vectorstore.ImageContent:
        payload["content_type"] = "image"
        payload["content_url"] = content.URL
    }

    // Store metadata
    for k, v := range doc.Metadata {
        payload[k] = v
    }

    // Store scope if present
    if doc.Scope != nil {
        if doc.Scope.Tenant != "" {
            payload["scope_tenant"] = doc.Scope.Tenant
        }
        if doc.Scope.User != "" {
            payload["scope_user"] = doc.Scope.User
        }
    }

    // Store tags
    if len(doc.Tags) > 0 {
        payload["tags"] = doc.Tags
    }

    return payload
}

func parseQdrantPoint(point *qdrantclient.ScoredPoint) *vectorstore.Document {
    doc := &vectorstore.Document{
        ID:       point.Id.GetUuid(),
        Metadata: make(map[string]any),
    }

    // Parse content
    if contentType, ok := point.Payload["content_type"].(string); ok {
        switch contentType {
        case "text":
            if text, ok := point.Payload["content_text"].(string); ok {
                doc.Content = vectorstore.NewTextContent(text)
            }
        case "image":
            if url, ok := point.Payload["content_url"].(string); ok {
                doc.Content = vectorstore.NewImageURL(url)
            }
        }
    }

    // Parse scope
    if tenant, ok := point.Payload["scope_tenant"].(string); ok {
        if doc.Scope == nil {
            doc.Scope = &vectorstore.Scope{}
        }
        doc.Scope.Tenant = tenant
    }
    if user, ok := point.Payload["scope_user"].(string); ok {
        if doc.Scope == nil {
            doc.Scope = &vectorstore.Scope{}
        }
        doc.Scope.User = user
    }

    // Parse tags
    if tags, ok := point.Payload["tags"].([]interface{}); ok {
        doc.Tags = make([]string, len(tags))
        for i, tag := range tags {
            doc.Tags[i] = tag.(string)
        }
    }

    // Parse metadata (exclude system fields)
    for k, v := range point.Payload {
        if !isSystemField(k) {
            doc.Metadata[k] = v
        }
    }

    return doc
}

func isSystemField(key string) bool {
    systemFields := map[string]bool{
        "content_type": true,
        "content_text": true,
        "content_url":  true,
        "scope_tenant": true,
        "scope_user":   true,
        "tags":         true,
    }
    return systemFields[key]
}

func buildQdrantFilter(filter vectorstore.Filter) *qdrantclient.Filter {
    // Implement filter translation from Aixgo's Filter to Qdrant's Filter
    // This is a simplified example - production code needs full implementation
    return &qdrantclient.Filter{
        // Add filter conditions based on filter type
    }
}
```

### Step 3: Usage Example

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/aixgo-dev/aixgo/pkg/vectorstore"
    "github.com/aixgo-dev/aixgo/pkg/vectorstore/qdrant"
)

func main() {
    ctx := context.Background()

    // Create Qdrant store with options
    store, err := qdrant.New(
        qdrant.WithHost("localhost:6333"),
        qdrant.WithAPIKey(os.Getenv("QDRANT_API_KEY")),
    )
    if err != nil {
        panic(err)
    }
    defer store.Close()

    // Create a collection for documents
    docs := store.Collection("documents",
        vectorstore.WithDimensions(384),
        vectorstore.WithDeduplication(true),
    )

    // Upsert a document
    doc := &vectorstore.Document{
        ID:      "doc1",
        Content: vectorstore.NewTextContent("Aixgo is a Go framework for AI agents"),
        Embedding: vectorstore.NewEmbedding(
            []float32{0.1, 0.2, 0.3}, // ... 384 dimensions
            "all-MiniLM-L6-v2",
        ),
        Tags:     []string{"documentation"},
        Metadata: map[string]any{"source": "readme"},
    }

    result, err := docs.Upsert(ctx, doc)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Inserted %d documents\n", result.Inserted)

    // Query the collection
    queryResult, err := docs.Query(ctx, &vectorstore.Query{
        Embedding: vectorstore.NewEmbedding(
            []float32{0.11, 0.21, 0.31}, // ... 384 dimensions
            "all-MiniLM-L6-v2",
        ),
        Limit:    5,
        MinScore: 0.7,
    })
    if err != nil {
        panic(err)
    }

    // Display results
    for _, match := range queryResult.Matches {
        fmt.Printf("Score: %.2f - %s\n",
            match.Score,
            match.Document.Content.String(),
        )
    }
}
```

## Adding a Custom Embedding Provider

Let's implement a Cohere embeddings provider.

### Step 1: Define Configuration

```go
// pkg/embeddings/embeddings.go
package embeddings

type CohereConfig struct {
    APIKey string `yaml:"api_key" json:"api_key"`
    Model  string `yaml:"model" json:"model"`
}

func (cc *CohereConfig) Validate() error {
    if cc.APIKey == "" {
        return fmt.Errorf("cohere api_key is required")
    }
    if cc.Model == "" {
        cc.Model = "embed-english-v3.0" // Default
    }
    return nil
}

// Add to Config
type Config struct {
    // ... existing fields ...
    Cohere *CohereConfig `yaml:"cohere,omitempty" json:"cohere,omitempty"`
}
```

### Step 2: Implement the Interface

```go
// pkg/embeddings/cohere.go
package embeddings

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

type CohereEmbeddings struct {
    apiKey string
    model  string
    client *http.Client
}

type cohereRequest struct {
    Texts []string `json:"texts"`
    Model string   `json:"model"`
}

type cohereResponse struct {
    Embeddings [][]float32 `json:"embeddings"`
}

func init() {
    Register("cohere", NewCohere)
}

func NewCohere(config Config) (EmbeddingService, error) {
    if config.Cohere == nil {
        return nil, fmt.Errorf("cohere configuration is required")
    }

    return &CohereEmbeddings{
        apiKey: config.Cohere.APIKey,
        model:  config.Cohere.Model,
        client: &http.Client{
            Timeout: 30 * time.Second,
        },
    }, nil
}

func (c *CohereEmbeddings) Embed(ctx context.Context, text string) ([]float32, error) {
    embeddings, err := c.EmbedBatch(ctx, []string{text})
    if err != nil {
        return nil, err
    }
    if len(embeddings) == 0 {
        return nil, fmt.Errorf("no embeddings returned")
    }
    return embeddings[0], nil
}

func (c *CohereEmbeddings) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
    reqBody := cohereRequest{
        Texts: texts,
        Model: c.model,
    }

    jsonData, err := json.Marshal(reqBody)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", "https://api.cohere.ai/v1/embed", bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, err
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+c.apiKey)

    resp, err := c.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
    }

    var cohereResp cohereResponse
    if err := json.Unmarshal(body, &cohereResp); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }

    return cohereResp.Embeddings, nil
}

func (c *CohereEmbeddings) Dimensions() int {
    // Cohere embed-english-v3.0 returns 1024 dimensions
    return 1024
}

func (c *CohereEmbeddings) ModelName() string {
    return c.model
}

func (c *CohereEmbeddings) Close() error {
    c.client.CloseIdleConnections()
    return nil
}
```

## Adding a Custom LLM Provider

Example: Adding Mistral AI support.

### Implementation

```go
// pkg/llm/mistral.go
package llm

import (
    "context"
    "fmt"
    // ... imports
)

type MistralLLM struct {
    apiKey      string
    model       string
    temperature float32
    client      *http.Client
}

func init() {
    Register("mistral", NewMistral)
}

func NewMistral(config Config) (LLM, error) {
    if config.Mistral == nil {
        return nil, fmt.Errorf("mistral configuration required")
    }

    return &MistralLLM{
        apiKey:      config.Mistral.APIKey,
        model:       config.Mistral.Model,
        temperature: config.Mistral.Temperature,
        client: &http.Client{
            Timeout: 60 * time.Second,
        },
    }, nil
}

func (m *MistralLLM) Complete(ctx context.Context, prompt string) (string, error) {
    // Implementation similar to OpenAI
    // Use Mistral's API endpoints and format
}

func (m *MistralLLM) Chat(ctx context.Context, messages []Message) (Message, error) {
    // Implement chat completion
}

func (m *MistralLLM) Close() error {
    m.client.CloseIdleConnections()
    return nil
}
```

## Best Practices

### 1. Configuration Validation

Always validate configuration early:

```go
func (c *CustomConfig) Validate() error {
    if c.RequiredField == "" {
        return fmt.Errorf("required_field must be set")
    }
    if c.Port < 1 || c.Port > 65535 {
        return fmt.Errorf("invalid port: %d", c.Port)
    }
    // Set defaults
    if c.Timeout == 0 {
        c.Timeout = 30 * time.Second
    }
    return nil
}
```

### 2. Error Handling

Provide clear, actionable error messages:

```go
// Bad
return fmt.Errorf("error: %w", err)

// Good
return fmt.Errorf("failed to connect to Qdrant at %s:%d: %w",
    q.host, q.port, err)
```

### 3. Context Support

Always respect context cancellation:

```go
func (c *CustomProvider) Search(ctx context.Context, query Query) (Results, error) {
    // Check context before expensive operations
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    // Pass context to HTTP requests
    req, err := http.NewRequestWithContext(ctx, "POST", url, body)
}
```

### 4. Resource Cleanup

Implement proper cleanup:

```go
func (c *CustomProvider) Close() error {
    // Close connections
    if c.client != nil {
        c.client.Close()
    }

    // Close channels
    if c.done != nil {
        close(c.done)
    }

    return nil
}
```

### 5. Testing

Add comprehensive tests:

```go
func TestCustomProvider(t *testing.T) {
    // Test configuration validation
    t.Run("ValidateConfig", func(t *testing.T) {
        config := CustomConfig{}
        err := config.Validate()
        if err == nil {
            t.Error("expected validation error for empty config")
        }
    })

    // Test provider creation
    t.Run("NewProvider", func(t *testing.T) {
        config := CustomConfig{
            Host: "localhost",
            Port: 1234,
        }
        provider, err := New(config)
        if err != nil {
            t.Fatalf("failed to create provider: %v", err)
        }
        defer provider.Close()
    })

    // Test operations
    t.Run("Operations", func(t *testing.T) {
        // Test upsert, search, delete, get
    })
}
```

## Complete Example: pgvector Provider

Here's a complete implementation for PostgreSQL with pgvector extension:

```go
// pkg/vectorstore/pgvector/pgvector.go
package pgvector

import (
    "context"
    "database/sql"
    "fmt"

    _ "github.com/lib/pq"
    "github.com/pgvector/pgvector-go"
    "github.com/aixgo-dev/aixgo/pkg/vectorstore"
)

type PgVectorStore struct {
    db                  *sql.DB
    table               string
    embeddingDimensions int
    defaultTopK         int
}

func init() {
    vectorstore.Register("pgvector", New)
}

func New(config vectorstore.Config) (vectorstore.VectorStore, error) {
    if config.PgVector == nil {
        return nil, fmt.Errorf("pgvector configuration required")
    }

    // Connect to PostgreSQL
    db, err := sql.Open("postgres", config.PgVector.ConnectionString)
    if err != nil {
        return nil, fmt.Errorf("failed to connect: %w", err)
    }

    // Test connection
    if err := db.Ping(); err != nil {
        return nil, fmt.Errorf("failed to ping database: %w", err)
    }

    // Create table if not exists
    createTableSQL := fmt.Sprintf(`
        CREATE TABLE IF NOT EXISTS %s (
            id TEXT PRIMARY KEY,
            content TEXT NOT NULL,
            embedding vector(%d),
            metadata JSONB,
            created_at TIMESTAMP DEFAULT NOW(),
            updated_at TIMESTAMP DEFAULT NOW()
        )
    `, config.PgVector.Table, config.EmbeddingDimensions)

    if _, err := db.Exec(createTableSQL); err != nil {
        return nil, fmt.Errorf("failed to create table: %w", err)
    }

    // Create vector index
    indexSQL := fmt.Sprintf(`
        CREATE INDEX IF NOT EXISTS %s_embedding_idx
        ON %s USING ivfflat (embedding vector_cosine_ops)
        WITH (lists = 100)
    `, config.PgVector.Table, config.PgVector.Table)

    if _, err := db.Exec(indexSQL); err != nil {
        return nil, fmt.Errorf("failed to create index: %w", err)
    }

    return &PgVectorStore{
        db:                  db,
        table:               config.PgVector.Table,
        embeddingDimensions: config.EmbeddingDimensions,
        defaultTopK:         config.DefaultTopK,
    }, nil
}

func (p *PgVectorStore) Upsert(ctx context.Context, documents []vectorstore.Document) error {
    if len(documents) == 0 {
        return nil
    }

    tx, err := p.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    stmt, err := tx.PrepareContext(ctx, fmt.Sprintf(`
        INSERT INTO %s (id, content, embedding, metadata, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (id) DO UPDATE SET
            content = EXCLUDED.content,
            embedding = EXCLUDED.embedding,
            metadata = EXCLUDED.metadata,
            updated_at = EXCLUDED.updated_at
    `, p.table))
    if err != nil {
        return err
    }
    defer stmt.Close()

    for _, doc := range documents {
        // Convert metadata to JSONB
        metadataJSON, _ := json.Marshal(doc.Metadata)

        // Convert embedding to pgvector format
        embedding := pgvector.NewVector(doc.Embedding)

        _, err := stmt.ExecContext(ctx,
            doc.ID,
            doc.Content,
            embedding,
            metadataJSON,
            doc.CreatedAt,
            doc.UpdatedAt,
        )
        if err != nil {
            return err
        }
    }

    return tx.Commit()
}

func (p *PgVectorStore) Search(ctx context.Context, query vectorstore.SearchQuery) ([]vectorstore.SearchResult, error) {
    if query.TopK == 0 {
        query.TopK = p.defaultTopK
    }

    embedding := pgvector.NewVector(query.Embedding)

    querySQL := fmt.Sprintf(`
        SELECT id, content, embedding, metadata,
               1 - (embedding <=> $1) as score
        FROM %s
        WHERE 1 - (embedding <=> $1) >= $2
        ORDER BY embedding <=> $1
        LIMIT $3
    `, p.table)

    rows, err := p.db.QueryContext(ctx, querySQL, embedding, query.MinScore, query.TopK)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var results []vectorstore.SearchResult
    for rows.Next() {
        var doc vectorstore.Document
        var embeddingData pgvector.Vector
        var metadataJSON []byte
        var score float32

        err := rows.Scan(&doc.ID, &doc.Content, &embeddingData, &metadataJSON, &score)
        if err != nil {
            return nil, err
        }

        // Parse metadata
        json.Unmarshal(metadataJSON, &doc.Metadata)
        doc.Embedding = embeddingData.Slice()

        results = append(results, vectorstore.SearchResult{
            Document: doc,
            Score:    score,
        })
    }

    return results, rows.Err()
}

func (p *PgVectorStore) Delete(ctx context.Context, ids []string) error {
    if len(ids) == 0 {
        return nil
    }

    placeholders := make([]string, len(ids))
    args := make([]interface{}, len(ids))
    for i, id := range ids {
        placeholders[i] = fmt.Sprintf("$%d", i+1)
        args[i] = id
    }

    query := fmt.Sprintf("DELETE FROM %s WHERE id IN (%s)",
        p.table, strings.Join(placeholders, ","))

    _, err := p.db.ExecContext(ctx, query, args...)
    return err
}

func (p *PgVectorStore) Get(ctx context.Context, ids []string) ([]vectorstore.Document, error) {
    // Similar to Delete, but with SELECT
}

func (p *PgVectorStore) Close() error {
    return p.db.Close()
}
```

## Deployment and Distribution

### Creating a Separate Module

For external providers, create a separate module:

```text
github.com/yourorg/aixgo-qdrant/
├── go.mod
├── go.sum
├── README.md
├── qdrant.go
└── qdrant_test.go
```

**go.mod:**

```go
module github.com/yourorg/aixgo-qdrant

go 1.21

require (
    github.com/aixgo-dev/aixgo v0.2.4
    github.com/qdrant/go-client v1.0.0
)
```

**Usage:**

```go
import (
    "github.com/aixgo-dev/aixgo/pkg/vectorstore"
    _ "github.com/yourorg/aixgo-qdrant"  // Register provider
)

config := vectorstore.Config{
    Provider: "qdrant",
    // ...
}
```

## Testing Custom Providers

### Integration Tests

```go
func TestQdrantIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // Start Qdrant with Docker
    ctx := context.Background()
    container := startQdrantContainer(t)
    defer container.Stop(ctx)

    // Create provider
    config := vectorstore.Config{
        Provider:            "qdrant",
        EmbeddingDimensions: 384,
        Qdrant: &vectorstore.QdrantConfig{
            Host:       "localhost",
            Port:       6333,
            Collection: "test",
        },
    }

    store, err := vectorstore.New(config)
    if err != nil {
        t.Fatal(err)
    }
    defer store.Close()

    // Run tests
    testUpsert(t, store)
    testSearch(t, store)
    testDelete(t, store)
}
```

## Contributing Back

To contribute your provider to Aixgo:

1. Fork the repository
2. Create a feature branch
3. Add your provider implementation
4. Add tests (unit + integration)
5. Update documentation
6. Submit a pull request

See [CONTRIBUTING.md](https://github.com/aixgo-dev/aixgo/blob/main/CONTRIBUTING.md) for details.

## Resources

- [Qdrant Go Client](https://github.com/qdrant/go-client)
- [pgvector Go](https://github.com/pgvector/pgvector-go)
- [Cohere API](https://docs.cohere.ai/)
- [Mistral API](https://docs.mistral.ai/)

## Next Steps

- Try the [RAG Agent Example](../../examples/rag-agent)
- Read [Vector Databases Guide](./vector-databases.md)
- Explore [Provider Integration](./provider-integration.md)
