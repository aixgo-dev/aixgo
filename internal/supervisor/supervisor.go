package supervisor

import (
	"context"
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

func New(def SupervisorDef, agents map[string]agent.Agent, rt agent.Runtime) *Supervisor {
	return &Supervisor{
		def:    def,
		client: openai.NewClient(getAPIKey()),
		agents: agents,
		rt:     rt,
	}
}

func (s *Supervisor) Start(ctx context.Context) error {
	// In production: intercept all messages via runtime
	// Here: simple loop for demo
	log.Printf("[SUPERVISOR] %s online (model: %s)", s.def.Name, s.def.Model)
	return nil
}

func getAPIKey() string {
	// Replace with your key or env var
	return "xai-api-key-placeholder"
}
