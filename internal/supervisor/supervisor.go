package supervisor

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/sashabaranov/go-openai"
	"log"
)

type Supervisor struct {
	def    SupervisorDef
	client *openai.Client
	agents map[string]agent.Agent
	rt     agent.Runtime
}

type SupervisorDef struct {
	Name      string `yaml:"name"`
	Model     string `yaml:"model"`
	MaxRounds int    `yaml:"max_rounds"`
}

func New(def SupervisorDef, agents map[string]agent.Agent, rt agent.Runtime) (*Supervisor, error) {
	apiKey := getAPIKeyFromEnv(def.Model)
	if apiKey == "" {
		return nil, fmt.Errorf("supervisor API key not found: please set the appropriate environment variable (XAI_API_KEY, OPENAI_API_KEY, ANTHROPIC_API_KEY, or HUGGINGFACE_API_KEY)")
	}

	return &Supervisor{
		def:    def,
		client: openai.NewClient(apiKey),
		agents: agents,
		rt:     rt,
	}, nil
}

func (s *Supervisor) Start(ctx context.Context) error {
	// In production: intercept all messages via runtime
	// Here: simple loop for demo
	log.Printf("[SUPERVISOR] %s online (model: %s)", s.def.Name, s.def.Model)
	return nil
}

// getAPIKeyFromEnv returns the appropriate API key from environment variables based on model name
func getAPIKeyFromEnv(model string) string {
	modelLower := strings.ToLower(model)

	// Try model-specific keys first
	if strings.Contains(modelLower, "grok") || strings.Contains(modelLower, "xai") {
		if key := os.Getenv("XAI_API_KEY"); key != "" {
			return key
		}
	}

	if strings.Contains(modelLower, "gpt") || strings.Contains(modelLower, "openai") {
		if key := os.Getenv("OPENAI_API_KEY"); key != "" {
			return key
		}
	}

	if strings.Contains(modelLower, "claude") || strings.Contains(modelLower, "anthropic") {
		if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
			return key
		}
	}

	// For HuggingFace models (check patterns)
	hfPatterns := []string{
		"meta-llama/", "mistralai/", "tiiuae/", "EleutherAI/",
		"bigscience/", "facebook/", "google/", "microsoft/",
	}
	for _, pattern := range hfPatterns {
		if strings.HasPrefix(model, pattern) {
			if key := os.Getenv("HUGGINGFACE_API_KEY"); key != "" {
				return key
			}
			break
		}
	}

	// Fall back to generic keys
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("XAI_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("HUGGINGFACE_API_KEY"); key != "" {
		return key
	}

	return ""
}
