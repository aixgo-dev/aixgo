package supervisor

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/sashabaranov/go-openai"
)

// RoutingStrategy defines how the supervisor routes tasks to agents
type RoutingStrategy string

const (
	StrategyRoundRobin RoutingStrategy = "round_robin"
	StrategyBestMatch  RoutingStrategy = "best_match"
	StrategyManual     RoutingStrategy = "manual"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskInProgress TaskStatus = "in_progress"
	TaskCompleted  TaskStatus = "completed"
	TaskFailed     TaskStatus = "failed"
)

// Task represents a unit of work assigned to an agent
type Task struct {
	ID          string
	Description string
	AssignedTo  string
	Status      TaskStatus
	Result      string
	Round       int
}

// AgentResponse represents a response from an agent
type AgentResponse struct {
	AgentName string
	Content   string
	NextAgent string // Suggested next agent for handoff
	Complete  bool   // Whether the task is complete
}

// Supervisor orchestrates multiple agents
type Supervisor struct {
	def      SupervisorDef
	client   *openai.Client
	agents   map[string]agent.Agent
	rt       agent.Runtime
	tasks    map[string]*Task
	tasksMu  sync.RWMutex
	round    int
	roundMu  sync.Mutex
	messages []Message
	msgMu    sync.RWMutex
}

// SupervisorDef defines the configuration for a supervisor
type SupervisorDef struct {
	Name            string          `yaml:"name"`
	Model           string          `yaml:"model"`
	MaxRounds       int             `yaml:"max_rounds"`
	RoutingStrategy RoutingStrategy `yaml:"routing_strategy,omitempty"`
	SystemPrompt    string          `yaml:"system_prompt,omitempty"`
}

// Message represents a message in the conversation
type Message struct {
	Role    string
	Content string
	Agent   string
}

// New creates a new Supervisor instance
func New(def SupervisorDef, agents map[string]agent.Agent, rt agent.Runtime) (*Supervisor, error) {
	apiKey := getAPIKeyFromEnv(def.Model)
	if apiKey == "" {
		return nil, fmt.Errorf("supervisor API key not found: please set the appropriate environment variable (XAI_API_KEY, OPENAI_API_KEY, ANTHROPIC_API_KEY, or HUGGINGFACE_API_KEY)")
	}

	if def.MaxRounds <= 0 {
		def.MaxRounds = 10 // Default max rounds
	}

	if def.RoutingStrategy == "" {
		def.RoutingStrategy = StrategyBestMatch
	}

	return &Supervisor{
		def:      def,
		client:   openai.NewClient(apiKey),
		agents:   agents,
		rt:       rt,
		tasks:    make(map[string]*Task),
		messages: make([]Message, 0),
	}, nil
}

// Start initializes the supervisor
func (s *Supervisor) Start(ctx context.Context) error {
	log.Printf("[SUPERVISOR] %s online (model: %s, strategy: %s)", s.def.Name, s.def.Model, s.def.RoutingStrategy)
	return nil
}

// Run executes the orchestration loop for a given input
func (s *Supervisor) Run(ctx context.Context, input string) (string, error) {
	s.roundMu.Lock()
	s.round = 0
	s.roundMu.Unlock()

	s.addMessage("user", input, "")

	for {
		s.roundMu.Lock()
		currentRound := s.round
		s.roundMu.Unlock()

		if currentRound >= s.def.MaxRounds {
			return s.generateSummary(), nil
		}

		// Select the best agent for the current state
		selectedAgent := s.selectAgent(ctx, input)
		if selectedAgent == "" {
			return s.generateSummary(), nil
		}

		// Route message to the selected agent
		response, err := s.routeToAgent(ctx, selectedAgent, input)
		if err != nil {
			log.Printf("[SUPERVISOR] Error routing to agent %s: %v", selectedAgent, err)
			s.roundMu.Lock()
			s.round++
			s.roundMu.Unlock()
			continue
		}

		s.addMessage("assistant", response.Content, selectedAgent)

		// Check if task is complete
		if response.Complete {
			return s.generateSummary(), nil
		}

		// Handle handoff if suggested
		if response.NextAgent != "" {
			input = response.Content // Pass the response as input to next agent
		}

		s.roundMu.Lock()
		s.round++
		s.roundMu.Unlock()
	}
}

// selectAgent chooses the best agent based on routing strategy
func (s *Supervisor) selectAgent(ctx context.Context, input string) string {
	if len(s.agents) == 0 {
		return ""
	}

	switch s.def.RoutingStrategy {
	case StrategyRoundRobin:
		return s.selectRoundRobin()
	case StrategyBestMatch:
		return s.selectBestMatch(ctx, input)
	case StrategyManual:
		return s.selectManual(input)
	default:
		return s.selectBestMatch(ctx, input)
	}
}

// selectRoundRobin selects agents in round-robin order
func (s *Supervisor) selectRoundRobin() string {
	agentNames := s.getAgentNames()
	if len(agentNames) == 0 {
		return ""
	}

	s.roundMu.Lock()
	defer s.roundMu.Unlock()
	return agentNames[s.round%len(agentNames)]
}

// selectBestMatch uses LLM to select the best agent
func (s *Supervisor) selectBestMatch(ctx context.Context, input string) string {
	agentNames := s.getAgentNames()
	if len(agentNames) == 0 {
		return ""
	}
	if len(agentNames) == 1 {
		return agentNames[0]
	}

	// Build prompt for agent selection
	prompt := fmt.Sprintf(`Given the following task: %q

Available agents: %v

Which agent should handle this task? Respond with just the agent name.`, input, agentNames)

	resp, err := s.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: s.def.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "You are a routing assistant. Select the best agent for the task. Respond with only the agent name."},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		MaxTokens: 50,
	})
	if err != nil {
		log.Printf("[SUPERVISOR] Error selecting agent: %v, using first agent", err)
		return agentNames[0]
	}

	selected := strings.TrimSpace(resp.Choices[0].Message.Content)
	// Validate the selection
	for _, name := range agentNames {
		if strings.EqualFold(name, selected) {
			return name
		}
	}

	return agentNames[0]
}

// selectManual parses agent name from input (format: @agent_name message)
func (s *Supervisor) selectManual(input string) string {
	if strings.HasPrefix(input, "@") {
		parts := strings.SplitN(input, " ", 2)
		if len(parts) > 0 {
			agentName := strings.TrimPrefix(parts[0], "@")
			if _, exists := s.agents[agentName]; exists {
				return agentName
			}
		}
	}

	// Fall back to first agent
	agentNames := s.getAgentNames()
	if len(agentNames) > 0 {
		return agentNames[0]
	}
	return ""
}

// routeToAgent sends a message to an agent and gets a response
func (s *Supervisor) routeToAgent(ctx context.Context, agentName, input string) (*AgentResponse, error) {
	if s.rt == nil {
		// Direct mode without runtime - simulate response
		return &AgentResponse{
			AgentName: agentName,
			Content:   fmt.Sprintf("Agent %s processed: %s", agentName, input),
			Complete:  true,
		}, nil
	}

	// Send message via runtime
	msg := &agent.Message{}
	if err := s.rt.Send(agentName, msg); err != nil {
		return nil, fmt.Errorf("failed to send to agent %s: %w", agentName, err)
	}

	// Receive response
	ch, err := s.rt.Recv(agentName)
	if err != nil {
		return nil, fmt.Errorf("failed to receive from agent %s: %w", agentName, err)
	}

	select {
	case resp := <-ch:
		content := ""
		if resp != nil && resp.Payload != "" {
			content = resp.Payload
		}
		return &AgentResponse{
			AgentName: agentName,
			Content:   content,
			Complete:  true,
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// AssignTask creates and assigns a task to an agent
func (s *Supervisor) AssignTask(taskID, description, agentName string) error {
	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()

	if _, exists := s.agents[agentName]; !exists {
		return fmt.Errorf("agent %q not found", agentName)
	}

	s.tasks[taskID] = &Task{
		ID:          taskID,
		Description: description,
		AssignedTo:  agentName,
		Status:      TaskPending,
		Round:       s.round,
	}

	return nil
}

// CompleteTask marks a task as completed
func (s *Supervisor) CompleteTask(taskID, result string) error {
	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %q not found", taskID)
	}

	task.Status = TaskCompleted
	task.Result = result
	return nil
}

// FailTask marks a task as failed
func (s *Supervisor) FailTask(taskID, reason string) error {
	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %q not found", taskID)
	}

	task.Status = TaskFailed
	task.Result = reason
	return nil
}

// GetTask returns a task by ID
func (s *Supervisor) GetTask(taskID string) (*Task, bool) {
	s.tasksMu.RLock()
	defer s.tasksMu.RUnlock()
	task, exists := s.tasks[taskID]
	return task, exists
}

// GetPendingTasks returns all pending tasks
func (s *Supervisor) GetPendingTasks() []*Task {
	s.tasksMu.RLock()
	defer s.tasksMu.RUnlock()

	var pending []*Task
	for _, task := range s.tasks {
		if task.Status == TaskPending || task.Status == TaskInProgress {
			pending = append(pending, task)
		}
	}
	return pending
}

// Handoff transfers control from one agent to another
func (s *Supervisor) Handoff(ctx context.Context, fromAgent, toAgent, message string) (*AgentResponse, error) {
	if _, exists := s.agents[toAgent]; !exists {
		return nil, fmt.Errorf("target agent %q not found", toAgent)
	}

	s.addMessage("system", fmt.Sprintf("Handoff from %s to %s", fromAgent, toAgent), "")

	return s.routeToAgent(ctx, toAgent, message)
}

// GetCurrentRound returns the current orchestration round
func (s *Supervisor) GetCurrentRound() int {
	s.roundMu.Lock()
	defer s.roundMu.Unlock()
	return s.round
}

// GetMessages returns the conversation history
func (s *Supervisor) GetMessages() []Message {
	s.msgMu.RLock()
	defer s.msgMu.RUnlock()
	return append([]Message{}, s.messages...)
}

// addMessage adds a message to the conversation history
func (s *Supervisor) addMessage(role, content, agentName string) {
	s.msgMu.Lock()
	defer s.msgMu.Unlock()
	s.messages = append(s.messages, Message{
		Role:    role,
		Content: content,
		Agent:   agentName,
	})
}

// generateSummary creates a summary of the orchestration
func (s *Supervisor) generateSummary() string {
	s.msgMu.RLock()
	defer s.msgMu.RUnlock()

	if len(s.messages) == 0 {
		return ""
	}

	// Return the last assistant message
	for i := len(s.messages) - 1; i >= 0; i-- {
		if s.messages[i].Role == "assistant" {
			return s.messages[i].Content
		}
	}

	return ""
}

// getAgentNames returns a sorted list of agent names
func (s *Supervisor) getAgentNames() []string {
	names := make([]string, 0, len(s.agents))
	for name := range s.agents {
		names = append(names, name)
	}
	return names
}

// GetAgents returns the agents map
func (s *Supervisor) GetAgents() map[string]agent.Agent {
	return s.agents
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
