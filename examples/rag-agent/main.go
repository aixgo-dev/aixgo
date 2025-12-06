package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aixgo-dev/aixgo/pkg/embeddings"
	"github.com/aixgo-dev/aixgo/pkg/llm"
	"github.com/aixgo-dev/aixgo/pkg/vectorstore"
	"github.com/aixgo-dev/aixgo/pkg/vectorstore/firestore"
	"github.com/aixgo-dev/aixgo/pkg/vectorstore/memory"
)

// Config represents the application configuration
type Config struct {
	EmbeddingProvider string
	VectorProvider    string
	LLMProvider       string
	OpenAIKey         string
	HuggingFaceKey    string
	GCPProject        string
	GCPCredentials    string
}

func main() {
	// Parse command-line flags
	configFile := flag.String("config", "config.yaml", "Configuration file path")
	mode := flag.String("mode", "interactive", "Run mode: interactive, index, or query")
	flag.Parse()

	// Load configuration
	cfg, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize services
	ctx := context.Background()
	embSvc, store, llmClient, err := initializeServices(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize services: %v", err)
	}
	defer cleanup(embSvc, store)

	// Run based on mode
	switch *mode {
	case "index":
		if err := runIndexMode(ctx, embSvc, store); err != nil {
			log.Fatalf("Index mode failed: %v", err)
		}
	case "query":
		if err := runQueryMode(ctx, embSvc, store, llmClient); err != nil {
			log.Fatalf("Query mode failed: %v", err)
		}
	case "interactive":
		if err := runInteractiveMode(ctx, embSvc, store, llmClient); err != nil {
			log.Fatalf("Interactive mode failed: %v", err)
		}
	default:
		log.Fatalf("Unknown mode: %s", *mode)
	}
}

// loadConfig loads configuration from environment and file
func loadConfig(filename string) (*Config, error) {
	// For this example, we use environment variables
	// In production, you'd parse the YAML file
	return &Config{
		EmbeddingProvider: getEnv("EMBEDDING_PROVIDER", "huggingface"),
		VectorProvider:    getEnv("VECTOR_PROVIDER", "memory"),
		LLMProvider:       getEnv("LLM_PROVIDER", "openai"),
		OpenAIKey:         os.Getenv("OPENAI_API_KEY"),
		HuggingFaceKey:    os.Getenv("HUGGINGFACE_API_KEY"),
		GCPProject:        os.Getenv("GCP_PROJECT"),
		GCPCredentials:    os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
	}, nil
}

// initializeServices creates and configures all required services
func initializeServices(ctx context.Context, cfg *Config) (embeddings.EmbeddingService, vectorstore.VectorStore, llm.Client, error) {
	// Initialize embedding service
	embCfg := embeddings.Config{
		Provider: cfg.EmbeddingProvider,
	}

	switch cfg.EmbeddingProvider {
	case "huggingface":
		embCfg.HuggingFace = &embeddings.HuggingFaceConfig{
			Model:          "sentence-transformers/all-MiniLM-L6-v2",
			APIKey:         cfg.HuggingFaceKey,
			WaitForModel:   true,
			UseCache:       true,
		}
	case "openai":
		embCfg.OpenAI = &embeddings.OpenAIConfig{
			APIKey: cfg.OpenAIKey,
			Model:  "text-embedding-3-small",
		}
	}

	embSvc, err := embeddings.New(embCfg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create embedding service: %w", err)
	}

	// Initialize vector store
	var store vectorstore.VectorStore
	switch cfg.VectorProvider {
	case "memory":
		store, err = memory.New()
		if err != nil {
			_ = embSvc.Close()
			return nil, nil, nil, fmt.Errorf("failed to create memory store: %w", err)
		}
	case "firestore":
		if cfg.GCPProject == "" {
			_ = embSvc.Close()
			return nil, nil, nil, fmt.Errorf("GCP_PROJECT required for Firestore")
		}
		opts := []firestore.Option{
			firestore.WithProjectID(cfg.GCPProject),
		}
		if cfg.GCPCredentials != "" {
			opts = append(opts, firestore.WithCredentialsFile(cfg.GCPCredentials))
		}
		store, err = firestore.New(ctx, opts...)
		if err != nil {
			_ = embSvc.Close()
			return nil, nil, nil, fmt.Errorf("failed to create Firestore store: %w", err)
		}
	default:
		_ = embSvc.Close()
		return nil, nil, nil, fmt.Errorf("unknown vector provider: %s", cfg.VectorProvider)
	}

	// Initialize LLM client
	var llmClient llm.Client
	if cfg.OpenAIKey != "" {
		llmClient, err = llm.NewClient("openai", cfg.OpenAIKey)
		if err != nil {
			_ = embSvc.Close()
			_ = store.Close()
			return nil, nil, nil, fmt.Errorf("failed to create LLM client: %w", err)
		}
	}

	return embSvc, store, llmClient, nil
}

// runIndexMode indexes sample documents into the vector store
func runIndexMode(ctx context.Context, embSvc embeddings.EmbeddingService, store vectorstore.VectorStore) error {
	fmt.Println("=== Indexing Documents ===")

	// Get collection
	coll := store.Collection("knowledge_base")

	// Sample documents about Aixgo
	documents := []struct {
		id       string
		content  string
		category string
	}{
		{
			id:       "doc-1",
			content:  "Aixgo is a production-grade AI agent framework for Go. It provides a clean, type-safe API for building intelligent agents.",
			category: "overview",
		},
		{
			id:       "doc-2",
			content:  "Aixgo supports multiple LLM providers including OpenAI, Anthropic (Claude), Google (Gemini), Mistral, Cohere, and xAI (Grok).",
			category: "providers",
		},
		{
			id:       "doc-3",
			content:  "The framework includes built-in support for vector databases through the vectorstore package, enabling RAG implementations.",
			category: "features",
		},
		{
			id:       "doc-4",
			content:  "Aixgo provides in-memory and Firestore vector store implementations. Additional providers like Qdrant and pgvector are planned.",
			category: "vectorstore",
		},
		{
			id:       "doc-5",
			content:  "Embeddings in Aixgo can be generated using HuggingFace models (free), OpenAI models, or self-hosted TEI servers.",
			category: "embeddings",
		},
		{
			id:       "doc-6",
			content:  "The supervisor pattern in Aixgo orchestrates multiple agents, handling delegation and coordination automatically.",
			category: "architecture",
		},
		{
			id:       "doc-7",
			content:  "Aixgo agents can use ReAct, Chain-of-Thought, or custom prompting strategies for reasoning and tool usage.",
			category: "agents",
		},
		{
			id:       "doc-8",
			content:  "Production deployments benefit from Aixgo's built-in observability, error handling, retries, and structured logging.",
			category: "production",
		},
	}

	// Index each document
	for i, doc := range documents {
		fmt.Printf("Indexing %d/%d: %s...\n", i+1, len(documents), doc.id)

		// Generate embedding
		embedding, err := embSvc.Embed(ctx, doc.content)
		if err != nil {
			return fmt.Errorf("failed to generate embedding for %s: %w", doc.id, err)
		}

		// Create document
		vDoc := &vectorstore.Document{
			ID:      doc.id,
			Content: vectorstore.NewTextContent(doc.content),
			Embedding: vectorstore.NewEmbedding(
				embedding,
				embSvc.ModelName(),
			),
			Tags: []string{"documentation", doc.category},
			Metadata: map[string]any{
				"source":      "aixgo-docs",
				"category":    doc.category,
				"indexed_at":  time.Now().Format(time.RFC3339),
			},
		}

		// Store in vector database
		result, err := coll.Upsert(ctx, vDoc)
		if err != nil {
			return fmt.Errorf("failed to upsert %s: %w", doc.id, err)
		}

		fmt.Printf("  Stored: %d inserted, %d updated\n", result.Inserted, result.Updated)
	}

	fmt.Printf("\nSuccessfully indexed %d documents!\n", len(documents))
	return nil
}

// runQueryMode performs a single query and displays results
func runQueryMode(ctx context.Context, embSvc embeddings.EmbeddingService, store vectorstore.VectorStore, llmClient llm.Client) error {
	fmt.Println("=== Query Mode ===")

	// Example query
	query := "What LLM providers does Aixgo support?"
	fmt.Printf("Query: %s\n\n", query)

	// Search for relevant documents
	results, err := searchKnowledgeBase(ctx, embSvc, store, query, 3)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	// Display results
	fmt.Println("Retrieved Documents:")
	for i, match := range results.Matches {
		fmt.Printf("\n%d. [Score: %.3f] %s\n", i+1, match.Score, match.Document.ID)
		fmt.Printf("   %s\n", match.Document.Content.String())
		if category, ok := match.Document.Metadata["category"].(string); ok {
			fmt.Printf("   Category: %s\n", category)
		}
	}

	// Generate answer with LLM if available
	if llmClient != nil && results.HasMatches() {
		fmt.Println("\n--- Generating Answer ---")
		answer, err := generateAnswer(ctx, llmClient, query, results)
		if err != nil {
			return fmt.Errorf("answer generation failed: %w", err)
		}
		fmt.Println(answer)
	}

	return nil
}

// runInteractiveMode provides an interactive Q&A session
func runInteractiveMode(ctx context.Context, embSvc embeddings.EmbeddingService, store vectorstore.VectorStore, llmClient llm.Client) error {
	fmt.Println("=== Interactive RAG Assistant ===")
	fmt.Println("Ask questions about Aixgo. Type 'exit' to quit.")
	fmt.Println("Note: Run with --mode=index first to populate the knowledge base.")

	// Simple interactive loop
	for {
		fmt.Print("\nYou: ")
		var query string
		if _, err := fmt.Scanln(&query); err != nil {
			if err.Error() == "unexpected newline" {
				continue
			}
			return err
		}

		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		if query == "exit" || query == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		// Search knowledge base
		results, err := searchKnowledgeBase(ctx, embSvc, store, query, 5)
		if err != nil {
			fmt.Printf("Error searching: %v\n", err)
			continue
		}

		if !results.HasMatches() {
			fmt.Println("Assistant: I couldn't find any relevant information in the knowledge base.")
			continue
		}

		// Generate answer
		if llmClient != nil {
			answer, err := generateAnswer(ctx, llmClient, query, results)
			if err != nil {
				fmt.Printf("Error generating answer: %v\n", err)
				continue
			}
			fmt.Printf("\nAssistant: %s\n", answer)
		} else {
			fmt.Println("\nTop Results:")
			for i, match := range results.Matches {
				if i >= 3 {
					break
				}
				fmt.Printf("%d. [%.3f] %s\n", i+1, match.Score, match.Document.Content.String())
			}
		}
	}

	return nil
}

// searchKnowledgeBase performs semantic search on the knowledge base
func searchKnowledgeBase(ctx context.Context, embSvc embeddings.EmbeddingService, store vectorstore.VectorStore, query string, topK int) (*vectorstore.QueryResult, error) {
	// Generate query embedding
	queryEmbedding, err := embSvc.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Get collection
	coll := store.Collection("knowledge_base")

	// Query with timeout
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	results, err := coll.Query(queryCtx, &vectorstore.Query{
		Embedding: vectorstore.NewEmbedding(
			queryEmbedding,
			embSvc.ModelName(),
		),
		Limit:    topK,
		MinScore: 0.5, // Adjust based on your needs
		Filters:  vectorstore.TagFilter("documentation"),
	})
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return results, nil
}

// generateAnswer uses the LLM to generate an answer based on retrieved context
func generateAnswer(ctx context.Context, client llm.Client, query string, results *vectorstore.QueryResult) (string, error) {
	// Build context from retrieved documents
	var contextParts []string
	for i, match := range results.Matches {
		if i >= 3 { // Limit context to top 3 results
			break
		}
		contextParts = append(contextParts, fmt.Sprintf(
			"[Document %s (score: %.3f)]\n%s",
			match.Document.ID,
			match.Score,
			match.Document.Content.String(),
		))
	}
	contextText := strings.Join(contextParts, "\n\n")

	// Create prompt
	prompt := fmt.Sprintf(`You are a helpful AI assistant with access to a knowledge base about Aixgo, a Go AI agent framework.

Use the following retrieved documents to answer the user's question. Always cite which document(s) you used.
If the documents don't contain enough information to answer the question, say so.

Retrieved Documents:
%s

User Question: %s

Answer:`, contextText, query)

	// Generate response with timeout
	genCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	resp, err := client.Chat(genCtx, messages, llm.WithMaxTokens(500), llm.WithTemperature(0.7))
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	return resp.Content, nil
}

// cleanup closes all services
func cleanup(embSvc embeddings.EmbeddingService, store vectorstore.VectorStore) {
	if embSvc != nil {
		_ = embSvc.Close()
	}
	if store != nil {
		_ = store.Close()
	}
}

// getEnv returns environment variable or default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

