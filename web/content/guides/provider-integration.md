---
title: 'Provider Integration Guide'
description: 'Integrate Aixgo with OpenAI, Anthropic, Google Vertex AI, HuggingFace, Ollama (local), and vector databases.'
breadcrumb: 'Reference'
category: 'Reference'
weight: 11
---

## Provider Status

### LLM Providers

| Provider           | Status                                  | Notes                                                     |
| ------------------ | --------------------------------------- | --------------------------------------------------------- |
| OpenAI             | {{< status-badge status="available" >}} | Chat, streaming SSE, function calling, JSON mode          |
| Anthropic (Claude) | {{< status-badge status="available" >}} | Messages API, streaming SSE, tool use                     |
| Google Gemini      | {{< status-badge status="available" >}} | GenerateContent API, streaming SSE, function calling      |
| xAI (Grok)         | {{< status-badge status="available" >}} | Chat, streaming SSE, function calling (OpenAI-compatible) |
| Vertex AI          | {{< status-badge status="available" >}} | Google Cloud AI Platform, streaming SSE, function calling |
| HuggingFace        | {{< status-badge status="available" >}} | Free Inference API, cloud backends                        |
| Ollama             | {{< status-badge status="available" >}} | Local models, zero costs, enterprise SSRF protection, hybrid fallback |

### Vector Databases

| Provider  | Status                                  | Notes                                 |
| --------- | --------------------------------------- | ------------------------------------- |
| Firestore | {{< status-badge status="available" >}} | Google Cloud serverless vector search |
| In-Memory | {{< status-badge status="available" >}} | Development and testing               |
| Qdrant    | {{< status-badge status="planned" >}}   | Planned for v0.2                      |
| pgvector  | {{< status-badge status="planned" >}}   | Planned for v0.2                      |

### Embedding Providers

| Provider        | Status                                  | Notes                                          |
| --------------- | --------------------------------------- | ---------------------------------------------- |
| OpenAI          | {{< status-badge status="available" >}} | text-embedding-3-small, text-embedding-3-large |
| HuggingFace API | {{< status-badge status="available" >}} | Free inference API, 100+ models                |
| HuggingFace TEI | {{< status-badge status="available" >}} | Self-hosted high-performance server            |

---

## LLM Providers

### OpenAI (GPT-4, GPT-3.5)

**Supported models:**

- `gpt-4` - Most capable, higher cost
- `gpt-4-turbo` - Faster, lower cost than GPT-4
- `gpt-3.5-turbo` - Fast, cost-effective

**Configuration:**

```yaml
# config/agents.yaml
agents:
  - name: analyzer
    role: react
    model: gpt-4-turbo
    provider: openai
    api_key: ${OPENAI_API_KEY}
    temperature: 0.7
    max_tokens: 1000
```

**Environment variables:**

```bash
export OPENAI_API_KEY=sk-...
```

**Go code:**

```go
import "github.com/aixgo-dev/aixgo/providers/openai"

agent := aixgo.NewAgent(
    aixgo.WithName("analyzer"),
    aixgo.WithModel("gpt-4-turbo"),
    aixgo.WithProvider(openai.Provider{
        APIKey: os.Getenv("OPENAI_API_KEY"),
    }),
)
```

**Features:**

- ✅ Chat completions
- ✅ Function calling (tools)
- ✅ Streaming SSE responses
- ✅ JSON mode
- ✅ Token usage tracking

**Pricing (as of 2025):**

- GPT-4 Turbo: $0.01 per 1K input tokens, $0.03 per 1K output tokens
- GPT-3.5 Turbo: $0.0005 per 1K input tokens, $0.0015 per 1K output tokens

### Anthropic (Claude)

**Supported models:**

- `claude-3-opus` - Most capable
- `claude-3-sonnet` - Balanced performance/cost
- `claude-3-haiku` - Fastest, lowest cost

**Configuration:**

```yaml
agents:
  - name: analyst
    role: react
    model: claude-3-sonnet
    provider: anthropic
    api_key: ${ANTHROPIC_API_KEY}
    temperature: 0.5
    max_tokens: 2000
```

**Environment variables:**

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

**Go code:**

```go
import "github.com/aixgo-dev/aixgo/providers/anthropic"

agent := aixgo.NewAgent(
    aixgo.WithName("analyst"),
    aixgo.WithModel("claude-3-sonnet"),
    aixgo.WithProvider(anthropic.Provider{
        APIKey: os.Getenv("ANTHROPIC_API_KEY"),
    }),
)
```

**Features:**

- ✅ Long context window (200K tokens supported by API)
- ✅ Tool use
- ✅ Streaming SSE responses
- 🚧 Vision support (Planned)

**Pricing:**

- Claude 3 Opus: $0.015 per 1K input tokens, $0.075 per 1K output tokens
- Claude 3 Sonnet: $0.003 per 1K input tokens, $0.015 per 1K output tokens
- Claude 3 Haiku: $0.00025 per 1K input tokens, $0.00125 per 1K output tokens

### Google Vertex AI (Gemini)

**Supported models:**

- `gemini-1.5-pro` - Most capable
- `gemini-1.5-flash` - Fast, cost-effective

**Configuration:**

```yaml
agents:
  - name: processor
    role: react
    model: gemini-1.5-flash
    provider: vertexai
    project_id: ${GCP_PROJECT_ID}
    location: us-central1
    temperature: 0.8
```

**Authentication:**

```bash
# Service account key
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json

# Or use gcloud default credentials
gcloud auth application-default login
```

**Go code:**

```go
import "github.com/aixgo-dev/aixgo/providers/vertexai"

agent := aixgo.NewAgent(
    aixgo.WithName("processor"),
    aixgo.WithModel("gemini-1.5-flash"),
    aixgo.WithProvider(vertexai.Provider{
        ProjectID: os.Getenv("GCP_PROJECT_ID"),
        Location:  "us-central1",
    }),
)
```

**Features:**

- ✅ Long context (2M tokens for Gemini 1.5)
- ✅ Multimodal (text, images, video)
- ✅ Function calling
- ✅ Grounding with Google Search

**Pricing:**

- Gemini 1.5 Pro: $0.00125 per 1K input chars, $0.005 per 1K output chars
- Gemini 1.5 Flash: $0.000125 per 1K input chars, $0.000375 per 1K output chars

### HuggingFace Inference API

**Supported backends:**

- HuggingFace Inference API (cloud)
- Ollama (local)
- vLLM (self-hosted)

**Supported models:**

- Any model on HuggingFace with Inference API enabled
- Popular: `meta-llama/Llama-2-70b-chat-hf`, `mistralai/Mixtral-8x7B-Instruct-v0.1`

**Configuration:**

```yaml
agents:
  - name: classifier
    role: react
    model: meta-llama/Llama-2-70b-chat-hf
    provider: huggingface
    api_key: ${HUGGINGFACE_API_KEY}
    endpoint: https://api-inference.huggingface.co
```

**Environment variables:**

```bash
export HUGGINGFACE_API_KEY=hf_...
```

**Go code:**

```go
import "github.com/aixgo-dev/aixgo/providers/huggingface"

agent := aixgo.NewAgent(
    aixgo.WithName("classifier"),
    aixgo.WithModel("meta-llama/Llama-2-70b-chat-hf"),
    aixgo.WithProvider(huggingface.Provider{
        APIKey:   os.Getenv("HUGGINGFACE_API_KEY"),
        Endpoint: "https://api-inference.huggingface.co",
    }),
)
```

**Features:**

- ✅ Open-source models
- ✅ Self-hosted option (Ollama, vLLM)
- ✅ Cloud backends
- ✅ Streaming support
- ✅ Custom fine-tuned models
- ⚠️ Tool calling support (model-dependent)

**Pricing:**

- Pay-per-request or dedicated endpoints
- Varies by model size and usage

---

### Ollama (Local Models)

**Run production AI models on your own infrastructure with zero API costs and complete data privacy.**

Ollama enables you to run state-of-the-art open-source models locally or on-premises. Aixgo provides enterprise-grade Ollama integration with hardened security, automatic fallback to cloud APIs, and production-ready deployment templates.

**Supported models:**

Any model from the [Ollama library](https://ollama.com/library):

- `phi3.5:3.8b-mini-instruct-q4_K_M` - Fast, efficient (3.8B parameters)
- `gemma2:2b-instruct-q4_0` - Google's lightweight model (2B)
- `llama3.1:8b` - Meta's Llama 3.1 (8B)
- `mistral:7b` - Mistral 7B
- `codellama:7b` - Code-focused model
- Custom quantized models (int4, int8, fp16)

**Quick Start:**

1. Install Ollama:

```bash
# macOS
brew install ollama

# Linux
curl -fsSL https://ollama.com/install.sh | sh

# Windows - download from https://ollama.com/download
```

1. Pull a model:

```bash
ollama pull phi3.5:3.8b-mini-instruct-q4_K_M
```

1. Configure Aixgo:

```yaml
# config/agents.yaml
model_services:
  - name: phi-local
    provider: huggingface
    model: phi3.5:3.8b-mini-instruct-q4_K_M
    runtime: ollama
    transport: local
    config:
      address: http://localhost:11434  # Default, can be omitted
      quantization: int4

agents:
  - name: local-assistant
    role: react
    model: phi-local
    prompt: |
      You are a helpful AI assistant running locally.
    temperature: 0.7
    max_tokens: 1000
```

**Go SDK:**

```go
import (
    "context"
    "github.com/aixgo-dev/aixgo/internal/llm/inference"
)

// Create Ollama service
ollama := inference.NewOllamaService("http://localhost:11434")

// Check availability
if !ollama.Available() {
    log.Fatal("Ollama not running")
}

// List available models
models, err := ollama.ListModels(context.Background())
if err != nil {
    log.Fatal(err)
}
for _, model := range models {
    fmt.Printf("Model: %s (Size: %d bytes)\n", model.Name, model.Size)
}

// Generate text
req := inference.GenerateRequest{
    Model:       "phi3.5:3.8b-mini-instruct-q4_K_M",
    Prompt:      "Explain quantum computing in simple terms.",
    MaxTokens:   500,
    Temperature: 0.7,
    Stop:        []string{"\n\n"},
}
resp, err := ollama.Generate(context.Background(), req)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Response: %s\n", resp.Text)
fmt.Printf("Tokens used: %d prompt + %d completion = %d total\n",
    resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)

// Chat completions
chatResp, err := ollama.Chat(context.Background(), "phi3.5:3.8b-mini-instruct-q4_K_M", []inference.ChatMessage{
    {Role: "user", Content: "What is Aixgo?"},
})
```

**Configuration Options:**

```yaml
model_services:
  - name: my-ollama-service
    provider: huggingface
    model: llama3.1:8b
    runtime: ollama
    transport: local
    config:
      # Ollama server address (optional, defaults to http://localhost:11434)
      address: http://localhost:11434

      # Quantization level (optional)
      quantization: int4  # Options: int4, int8, fp16

      # Request timeout (optional, defaults to 5 minutes)
      timeout: 300s
```

**Hybrid Inference with Automatic Fallback:**

Aixgo can automatically fall back to cloud APIs if Ollama is unavailable:

```yaml
agents:
  - name: resilient-agent
    role: react
    providers:
      # Try local Ollama first
      - model: phi-local
        provider: huggingface
        runtime: ollama

      # Fallback to cloud if local unavailable
      - model: gpt-4-turbo
        provider: openai
        api_key: ${OPENAI_API_KEY}

      - model: claude-3-haiku
        provider: anthropic
        api_key: ${ANTHROPIC_API_KEY}

    fallback_strategy: cascade  # Try each in order
    prompt: |
      You are a resilient assistant with automatic failover.
```

**Security Features:**

Aixgo's Ollama integration includes enterprise-grade security:

- **SSRF Protection**: Strict host allowlist (localhost, 127.0.0.1, ::1, ollama)
- **No Redirects**: Prevents redirect-based SSRF attacks
- **IP Validation**: Blocks private ranges, link-local, multicast, cloud metadata endpoints
- **DNS Rebinding Protection**: Per-connection hostname validation
- **40+ Security Test Cases**: Comprehensive security validation

**Production Deployment:**

**Docker Compose:**

```yaml
# docker-compose.yaml
version: '3.8'
services:
  ollama:
    image: ollama/ollama:0.5.4
    ports:
      - "11434:11434"
    volumes:
      - ollama-data:/root/.ollama
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:11434/api/tags"]
      interval: 10s
      timeout: 5s
      retries: 3

  aixgo:
    build: .
    depends_on:
      ollama:
        condition: service_healthy
    environment:
      - OLLAMA_HOST=http://ollama:11434
    ports:
      - "8080:8080"

volumes:
  ollama-data:
```

**Kubernetes:**

Aixgo provides production-ready Kubernetes manifests at `deploy/k8s/base/ollama-deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ollama
  namespace: aixgo
spec:
  replicas: 1
  template:
    spec:
      # Security: Non-root user, seccomp, capabilities dropped
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        seccompProfile:
          type: RuntimeDefault

      containers:
      - name: ollama
        image: ollama/ollama:0.5.4
        ports:
        - containerPort: 11434

        resources:
          requests:
            cpu: 2
            memory: 4Gi
          limits:
            cpu: 4
            memory: 8Gi

        # Health checks
        livenessProbe:
          httpGet:
            path: /api/tags
            port: 11434
          initialDelaySeconds: 30
          periodSeconds: 10

        readinessProbe:
          httpGet:
            path: /api/tags
            port: 11434
          initialDelaySeconds: 10
          periodSeconds: 5

        volumeMounts:
        - name: ollama-data
          mountPath: /.ollama

      volumes:
      - name: ollama-data
        persistentVolumeClaim:
          claimName: ollama-pvc
```

Deploy with:

```bash
kubectl apply -f deploy/k8s/base/ollama-deployment.yaml
```

**Custom Docker Image:**

Build a security-hardened Ollama image with pre-loaded models:

```dockerfile
# docker/ollama.Dockerfile
FROM ollama/ollama:0.5.4

# Pre-pull models at build time (optional)
ARG MODELS="phi3.5:3.8b-mini-instruct-q4_K_M gemma2:2b-instruct-q4_0"
RUN ollama serve & sleep 5 && \
    for model in $MODELS; do ollama pull $model; done && \
    pkill ollama

# Run as non-root user
USER 1000

EXPOSE 11434
CMD ["serve"]
```

Build and run:

```bash
docker build -f docker/ollama.Dockerfile -t my-ollama:latest .
docker run -d -p 11434:11434 -v ollama-data:/root/.ollama my-ollama:latest
```

**API Endpoints:**

Aixgo supports these Ollama API endpoints:

| Endpoint | Method | Purpose | Supported |
|----------|--------|---------|-----------|
| `/api/generate` | POST | Text generation | ✅ Yes |
| `/api/chat` | POST | Chat completions | ✅ Yes |
| `/api/tags` | GET | List models / health check | ✅ Yes |
| `/` | GET | Health check | ✅ Yes |

**Environment Variables:**

```bash
# Ollama server address (optional, defaults to http://localhost:11434)
export OLLAMA_HOST=http://localhost:11434

# For custom deployments
export OLLAMA_HOST=http://ollama-service.aixgo.svc.cluster.local:11434
```

**Features:**

- ✅ Zero API costs
- ✅ Complete data privacy
- ✅ Any Ollama-compatible model
- ✅ Text generation and chat completions
- ✅ Token usage tracking
- ✅ Model listing and health checks
- ✅ Hybrid inference with cloud fallback
- ✅ Enterprise security (SSRF protection)
- ✅ Production Kubernetes manifests
- ✅ Docker and Docker Compose support
- ✅ Non-root container execution
- ❌ Streaming (not yet supported)
- ❌ Function calling (model-dependent)

**Performance:**

| Model | Size | Speed (tokens/sec) | Memory | Best For |
|-------|------|-------------------|--------|----------|
| **phi3.5:3.8b-q4** | 2.2GB | ~50-100 | 4GB | General purpose, fast responses |
| **gemma2:2b-q4** | 1.6GB | ~80-150 | 3GB | Lightweight, edge deployment |
| **llama3.1:8b** | 4.7GB | ~30-60 | 8GB | Higher quality, reasoning |
| **mistral:7b** | 4.1GB | ~40-80 | 6GB | Balanced performance/quality |

**Model Selection Guide:**

- **Development/Testing**: `gemma2:2b-q4_0` - Fastest, smallest
- **Production (CPU)**: `phi3.5:3.8b-mini-instruct-q4_K_M` - Best quality/speed balance
- **Production (GPU)**: `llama3.1:8b` or `mistral:7b` - Higher quality
- **Code Generation**: `codellama:7b` - Specialized for code
- **Edge Devices**: `gemma2:2b-q4_0` - Smallest memory footprint

**Troubleshooting:**

**Ollama not available:**

```bash
# Check if Ollama is running
curl http://localhost:11434/api/tags

# Start Ollama
ollama serve

# Check logs
ollama logs
```

**Model not found:**

```bash
# List available models
ollama list

# Pull missing model
ollama pull phi3.5:3.8b-mini-instruct-q4_K_M
```

**Connection refused in Kubernetes:**

```bash
# Check service
kubectl get svc ollama-service -n aixgo

# Check pod
kubectl get pods -n aixgo -l app=ollama

# View logs
kubectl logs -n aixgo -l app=ollama

# Port forward for testing
kubectl port-forward -n aixgo svc/ollama-service 11434:11434
```

**High memory usage:**

- Use quantized models (q4_K_M, q4_0)
- Reduce `num_ctx` parameter
- Limit concurrent requests
- Use smaller models (2B-7B vs 13B+)

**Learn More:**

- [Ollama Documentation](https://github.com/ollama/ollama)
- [Available Models](https://ollama.com/library)
- [Aixgo Security Documentation](https://github.com/aixgo-dev/aixgo/blob/main/docs/SECURITY_STATUS.md)
- [Kubernetes Deployment Guide](https://github.com/aixgo-dev/aixgo/tree/main/deploy/k8s)

### xAI (Grok)

**Supported models:**

- `gpt-4-turbo` - Latest Grok model

**Configuration:**

```yaml
agents:
  - name: researcher
    role: react
    model: gpt-4-turbo
    provider: xai
    api_key: ${XAI_API_KEY}
```

**Environment variables:**

```bash
export XAI_API_KEY=xai-...
```

**Features:**

- ✅ Real-time web access
- ✅ Tool calling
- ✅ Long context window

## Provider Comparison

| Provider          | Best For                          | Context Length | Tool Support | Cost      |
| ----------------- | --------------------------------- | -------------- | ------------ | --------- |
| **OpenAI**        | General purpose, function calling | 128K tokens    | ✅ Excellent | $$$       |
| **Anthropic**     | Long documents, safety            | 200K tokens    | ✅ Excellent | $$$$      |
| **Google Vertex** | Multimodal, grounding             | 2M tokens      | ✅ Good      | $$        |
| **HuggingFace**   | Open source, custom models        | Varies         | ⚠️ Limited   | $         |
| **xAI**           | Real-time info, research          | 128K tokens    | ✅ Good      | $$$       |
| **Ollama**        | Local inference, data privacy     | Varies (4K-32K)| ⚠️ Limited   | Free      |

## Multi-Provider Strategy

### Fallback Configuration

Use multiple providers with automatic fallback:

```yaml
agents:
  - name: resilient-analyzer
    role: react
    providers:
      - model: gpt-4-turbo
        provider: openai
        api_key: ${OPENAI_API_KEY}
      - model: claude-3-sonnet
        provider: anthropic
        api_key: ${ANTHROPIC_API_KEY}
      - model: gemini-1.5-flash
        provider: vertexai
        project_id: ${GCP_PROJECT_ID}
    fallback_strategy: cascade # Try each in order
```

If OpenAI fails, automatically try Anthropic, then Google.

### Cost Optimization

Route based on complexity:

```yaml
# Simple tasks: cheap model
- name: simple-classifier
  role: react
  model: gpt-3.5-turbo
  provider: openai

# Complex reasoning: capable model
- name: complex-analyzer
  role: react
  model: gpt-4-turbo
  provider: openai
```

### Region-Specific Routing

```yaml
# US region: Vertex AI (low latency)
- name: us-agent
  role: react
  model: gemini-1.5-flash
  provider: vertexai
  location: us-central1

# EU region: OpenAI EU endpoint
- name: eu-agent
  role: react
  model: gpt-4-turbo
  provider: openai
  endpoint: https://api.openai.com/v1 # or EU-specific endpoint
```

## Vector Databases & Embeddings

### Overview

Aixgo provides integrated support for vector databases and embeddings, enabling Retrieval-Augmented Generation (RAG) systems. The architecture separates embedding generation from
vector storage for maximum flexibility.

**Architecture:**

```text
Documents → Embeddings Service → Vector Database → Semantic Search
```

### Embedding Providers

#### OpenAI Embeddings

**Best for:** Production deployments, highest quality

**Configuration:**

```yaml
embeddings:
  provider: openai
  openai:
    api_key: ${OPENAI_API_KEY}
    model: text-embedding-3-small # or text-embedding-3-large
```

**Go code:**

```go
import "github.com/aixgo-dev/aixgo/pkg/embeddings"

config := embeddings.Config{
    Provider: "openai",
    OpenAI: &embeddings.OpenAIConfig{
        APIKey: os.Getenv("OPENAI_API_KEY"),
        Model:  "text-embedding-3-small",
    },
}

embSvc, err := embeddings.New(config)
if err != nil {
    log.Fatal(err)
}
defer embSvc.Close()

// Generate embedding
embedding, err := embSvc.Embed(ctx, "Your text here")
```

**Models:**

- `text-embedding-3-small`: 1536 dimensions, $0.02 per 1M tokens
- `text-embedding-3-large`: 3072 dimensions, $0.13 per 1M tokens
- `text-embedding-ada-002`: 1536 dimensions (legacy)

#### HuggingFace Inference API

**Best for:** Development, cost-sensitive deployments

**Configuration:**

```yaml
embeddings:
  provider: huggingface
  huggingface:
    model: sentence-transformers/all-MiniLM-L6-v2
    api_key: ${HUGGINGFACE_API_KEY} # Optional
    wait_for_model: true
    use_cache: true
```

**Popular models:**

- `sentence-transformers/all-MiniLM-L6-v2`: 384 dims, fast
- `BAAI/bge-large-en-v1.5`: 1024 dims, excellent quality
- `thenlper/gte-large`: 1024 dims, multilingual

**Pricing:** FREE (Inference API) with rate limits

#### HuggingFace TEI (Self-Hosted)

**Best for:** High-throughput production workloads

**Docker setup:**

```bash
docker run -d \
  --name tei \
  -p 8080:8080 \
  --gpus all \
  ghcr.io/huggingface/text-embeddings-inference:latest \
  --model-id BAAI/bge-large-en-v1.5
```

**Configuration:**

```yaml
embeddings:
  provider: huggingface_tei
  huggingface_tei:
    endpoint: http://localhost:8080
    model: BAAI/bge-large-en-v1.5
    normalize: true
```

### Vector Store Providers

#### Firestore Vector Search

**Best for:** Serverless production deployments on GCP

**Setup:**

```bash
# Enable Firestore
gcloud services enable firestore.googleapis.com

# Create vector index
gcloud firestore indexes composite create \
  --collection-group=embeddings \
  --query-scope=COLLECTION \
  --field-config=field-path=embedding,vector-config='{"dimension":"384","flat":{}}'
```

**Configuration:**

```yaml
vectorstore:
  provider: firestore
  embedding_dimensions: 384
  firestore:
    project_id: ${GCP_PROJECT_ID}
    collection: embeddings
    credentials_file: /path/to/key.json # Optional
```

**Go code:**

```go
import "github.com/aixgo-dev/aixgo/pkg/vectorstore"

config := vectorstore.Config{
    Provider:            "firestore",
    EmbeddingDimensions: 384,
    Firestore: &vectorstore.FirestoreConfig{
        ProjectID:  os.Getenv("GCP_PROJECT_ID"),
        Collection: "embeddings",
    },
}

store, err := vectorstore.New(config)
if err != nil {
    log.Fatal(err)
}
defer store.Close()

// Upsert documents
doc := vectorstore.Document{
    ID:        "doc-1",
    Content:   "Your document content",
    Embedding: embedding,
    Metadata: map[string]interface{}{
        "category": "documentation",
    },
}
store.Upsert(ctx, []vectorstore.Document{doc})

// Search
results, err := store.Search(ctx, vectorstore.SearchQuery{
    Embedding: queryEmbedding,
    TopK:      5,
    MinScore:  0.7,
})
```

**Features:**

- ✅ Serverless, auto-scaling
- ✅ Persistent storage
- ✅ Real-time updates
- ✅ ACID transactions

**Pricing:** ~$0.06 per 100K reads + storage

#### In-Memory Vector Store

**Best for:** Development, testing, prototyping

**Configuration:**

```yaml
vectorstore:
  provider: memory
  embedding_dimensions: 384
  memory:
    max_documents: 10000
```

**Features:**

- ✅ Zero setup
- ✅ Fast for small datasets
- ❌ Data lost on restart
- ❌ Limited capacity

#### Qdrant (Planned - v0.2)

High-performance dedicated vector database:

```yaml
# Coming soon
vectorstore:
  provider: qdrant
  embedding_dimensions: 384
  qdrant:
    host: localhost
    port: 6333
    collection: knowledge_base
```

#### pgvector (Planned - v0.2)

PostgreSQL extension for vector search:

```yaml
# Coming soon
vectorstore:
  provider: pgvector
  embedding_dimensions: 384
  pgvector:
    connection_string: postgresql://user:pass@localhost/db
    table: embeddings
```

### Complete RAG Example

```go
package main

import (
    "context"
    "log"

    "github.com/aixgo-dev/aixgo/pkg/embeddings"
    "github.com/aixgo-dev/aixgo/pkg/vectorstore"
)

func main() {
    ctx := context.Background()

    // Setup embeddings
    embConfig := embeddings.Config{
        Provider: "huggingface",
        HuggingFace: &embeddings.HuggingFaceConfig{
            Model: "sentence-transformers/all-MiniLM-L6-v2",
        },
    }
    embSvc, _ := embeddings.New(embConfig)
    defer embSvc.Close()

    // Setup vector store
    storeConfig := vectorstore.Config{
        Provider:            "firestore",
        EmbeddingDimensions: embSvc.Dimensions(),
        Firestore: &vectorstore.FirestoreConfig{
            ProjectID:  "my-project",
            Collection: "knowledge_base",
        },
    }
    store, _ := vectorstore.New(storeConfig)
    defer store.Close()

    // Index documents
    docs := []string{
        "Aixgo is a production-grade AI framework",
        "RAG combines retrieval with generation",
    }

    for i, content := range docs {
        emb, _ := embSvc.Embed(ctx, content)
        doc := vectorstore.Document{
            ID:        fmt.Sprintf("doc-%d", i),
            Content:   content,
            Embedding: emb,
        }
        store.Upsert(ctx, []vectorstore.Document{doc})
    }

    // Search
    query := "What is Aixgo?"
    queryEmb, _ := embSvc.Embed(ctx, query)
    results, _ := store.Search(ctx, vectorstore.SearchQuery{
        Embedding: queryEmb,
        TopK:      3,
    })

    for _, result := range results {
        fmt.Printf("Score: %.2f - %s\n", result.Score, result.Document.Content)
    }
}
```

### Provider Comparison: Embeddings

| Provider            | Cost                 | Quality        | Speed     | Best For    |
| ------------------- | -------------------- | -------------- | --------- | ----------- |
| **OpenAI**          | $0.02-0.13/1M tokens | Excellent      | Fast      | Production  |
| **HuggingFace API** | Free                 | Good-Excellent | Medium    | Development |
| **HuggingFace TEI** | Free (self-host)     | Good-Excellent | Very Fast | High-volume |

### Provider Comparison: Vector Stores

| Provider               | Persistence | Scalability | Setup  | Cost      |
| ---------------------- | ----------- | ----------- | ------ | --------- |
| **Memory**             | No          | Low         | None   | Free      |
| **Firestore**          | Yes         | Unlimited   | Medium | $$        |
| **Qdrant** (planned)   | Yes         | Very High   | Medium | Self-host |
| **pgvector** (planned) | Yes         | High        | Medium | Self-host |

### Learn More

- **[Vector Databases Guide](/guides/vector-databases)** - Complete RAG implementation guide
- **[Extending Aixgo](/guides/extending-aixgo)** - Add custom vector store providers
- **[RAG Agent Example](https://github.com/aixgo-dev/aixgo/tree/main/examples/rag-agent)** - Full working example

## API Key Management

### Environment Variables

```bash
# .env file
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GCP_PROJECT_ID=my-project
HUGGINGFACE_API_KEY=hf_...
```

Load with:

```bash
export $(cat .env | xargs)
```

### Kubernetes Secrets

```bash
kubectl create secret generic llm-keys \
  --from-literal=OPENAI_API_KEY=sk-... \
  --from-literal=ANTHROPIC_API_KEY=sk-ant-...
```

**Reference in deployment:**

```yaml
env:
  - name: OPENAI_API_KEY
    valueFrom:
      secretKeyRef:
        name: llm-keys
        key: OPENAI_API_KEY
```

### Cloud Secret Managers

**Google Secret Manager:**

```go
import "cloud.google.com/go/secretmanager/apiv1"

func getAPIKey(ctx context.Context, secretName string) (string, error) {
    client, _ := secretmanager.NewClient(ctx)
    result, _ := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
        Name: secretName,
    })
    return string(result.Payload.Data), nil
}
```

## Rate Limiting & Retries

### Provider Rate Limits

| Provider  | Tier        | Requests/Min | Tokens/Min |
| --------- | ----------- | ------------ | ---------- |
| OpenAI    | Free        | 3            | 40,000     |
| OpenAI    | Paid Tier 1 | 500          | 90,000     |
| Anthropic | Free        | 5            | 25,000     |
| Anthropic | Paid        | 50           | 100,000    |
| Vertex AI | Default     | 60           | 60,000     |

### Retry Configuration

```yaml
agents:
  - name: resilient-agent
    role: react
    model: gpt-4-turbo
    provider: openai
    retry:
      max_attempts: 3
      initial_backoff: 1s
      max_backoff: 10s
      multiplier: 2
      retry_on:
        - rate_limit
        - timeout
        - server_error
```

## Monitoring Provider Performance

### Track Latency by Provider

```go
import "github.com/prometheus/client_golang/prometheus"

var providerLatency = prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "llm_provider_latency_seconds",
        Help: "LLM API call latency by provider",
    },
    []string{"provider", "model"},
)

// Aixgo tracks this automatically
```

### Cost Tracking

```yaml
observability:
  cost_tracking: true
  cost_alert_threshold: 100 # Alert if daily cost > $100
```

## Best Practices

### 1. Use Environment-Specific Keys

```yaml
# Development
OPENAI_API_KEY=sk-dev-...

# Production
OPENAI_API_KEY=sk-prod-...
```

### 2. Implement Fallback Providers

Always have a backup provider to avoid single point of failure.

### 3. Monitor Token Usage

Track and alert on unexpected token consumption:

```yaml
observability:
  llm_observability:
    enabled: true
    track_tokens: true
    daily_token_limit: 1000000
```

### 4. Choose Models Strategically

- **Simple tasks:** gpt-3.5-turbo, gemini-flash, claude-haiku
- **Complex reasoning:** gpt-4-turbo, claude-3-opus
- **Long documents:** claude-3-opus (200K), gemini-pro (2M)
- **Cost-sensitive:** gemini-flash, gpt-3.5-turbo

### 5. Use Caching

Cache LLM responses for repeated queries:

```go
import "github.com/aixgo-dev/aixgo/cache"

agent := aixgo.NewAgent(
    aixgo.WithName("cached-analyzer"),
    aixgo.WithCache(cache.NewRedisCache("localhost:6379")),
    aixgo.WithCacheTTL(1 * time.Hour),
)
```

## Troubleshooting

### Authentication Errors

**Error:** `401 Unauthorized`

**Solution:**

- Verify API key is correct
- Check key has not expired
- Ensure environment variable is loaded

### Rate Limit Exceeded

**Error:** `429 Too Many Requests`

**Solution:**

- Implement exponential backoff
- Reduce request rate
- Upgrade to higher tier
- Add multiple API keys for rotation

### Timeout Errors

**Error:** `Request timeout`

**Solution:**

```yaml
agents:
  - name: patient-agent
    role: react
    model: gpt-4-turbo
    timeout: 60s # Increase timeout
```

## Next Steps

- **[Type Safety & LLM Integration](/guides/type-safety)** - Type-safe provider usage
- **[Observability & Monitoring](/guides/observability)** - Monitor provider performance
- **[Production Deployment](/guides/production-deployment)** - Deploy with secrets management
