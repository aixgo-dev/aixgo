package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/llm/provider"
	"github.com/aixgo-dev/aixgo/internal/observability"
	"github.com/aixgo-dev/aixgo/pkg/security"
	pb "github.com/aixgo-dev/aixgo/proto"
)

// PlannerConfig holds AI-specific configuration for planning
type PlannerConfig struct {
	PlanningStrategy     string          `yaml:"planning_strategy"`
	MaxSteps            int             `yaml:"max_steps"`
	StepDetailLevel     string          `yaml:"step_detail_level"`
	EnableBacktracking  bool            `yaml:"enable_backtracking"`
	EnableSelfCritique  bool            `yaml:"enable_self_critique"`
	ReasoningDepth      int             `yaml:"reasoning_depth"`
	ParallelizableSteps bool            `yaml:"parallelizable_steps"`
	IncludeAlternatives bool            `yaml:"include_alternatives"`
	Temperature         float64         `yaml:"temperature"`
	MaxTokens           int             `yaml:"max_tokens"`
	ExamplePlans        []ExamplePlan   `yaml:"example_plans"`
}

// ExamplePlan for few-shot planning
type ExamplePlan struct {
	Problem     string   `yaml:"problem"`
	Steps       []string `yaml:"steps"`
	Explanation string   `yaml:"explanation"`
}

// PlanStep represents a single step in the reasoning chain
type PlanStep struct {
	StepNumber       int                `json:"step_number"`
	Action           string             `json:"action"`
	Reasoning        string             `json:"reasoning"`
	Prerequisites    []int              `json:"prerequisites,omitempty"`
	ExpectedOutcome  string             `json:"expected_outcome"`
	Complexity       string             `json:"complexity"`
	CanParallelize   bool               `json:"can_parallelize"`
	Alternatives     []AlternativeStep  `json:"alternatives,omitempty"`
	Confidence       float64            `json:"confidence"`
	EstimatedTokens  int                `json:"estimated_tokens,omitempty"`
}

// AlternativeStep represents an alternative approach
type AlternativeStep struct {
	Action    string  `json:"action"`
	Reasoning string  `json:"reasoning"`
	TradeOffs string  `json:"trade_offs"`
}

// ReasoningPlan with Chain-of-Thought structure
type ReasoningPlan struct {
	Problem             string            `json:"problem"`
	Analysis           ProblemAnalysis    `json:"analysis"`
	Steps              []PlanStep         `json:"steps"`
	ExecutionStrategy  string             `json:"execution_strategy"`
	CriticalPath       []int              `json:"critical_path"`
	ParallelGroups     [][]int            `json:"parallel_groups,omitempty"`
	BackupPlans        []BackupPlan       `json:"backup_plans,omitempty"`
	SuccessCriteria    []string           `json:"success_criteria"`
	RiskAssessment     RiskAssessment     `json:"risk_assessment"`
	TotalComplexity    string             `json:"total_complexity"`
	EstimatedDuration  string             `json:"estimated_duration"`
	TokensUsed         int                `json:"tokens_used"`
	PlanningStrategy   string             `json:"planning_strategy"`
	SelfCritique       string             `json:"self_critique,omitempty"`
}

// ProblemAnalysis breaks down the problem
type ProblemAnalysis struct {
	Type           string   `json:"problem_type"`
	Domain         string   `json:"domain"`
	Constraints    []string `json:"constraints"`
	Resources      []string `json:"available_resources"`
	KeyChallenges  []string `json:"key_challenges"`
	Assumptions    []string `json:"assumptions"`
}

// BackupPlan for contingency planning
type BackupPlan struct {
	TriggerCondition string     `json:"trigger_condition"`
	AlternativeSteps []PlanStep `json:"alternative_steps"`
	Description      string     `json:"description"`
}

// RiskAssessment evaluates plan risks
type RiskAssessment struct {
	OverallRisk      string       `json:"overall_risk"`
	RiskFactors      []RiskFactor `json:"risk_factors"`
	MitigationSteps  []string     `json:"mitigation_steps"`
}

type RiskFactor struct {
	Factor      string  `json:"factor"`
	Severity    string  `json:"severity"`
	Likelihood  float64 `json:"likelihood"`
	Impact      string  `json:"impact"`
}

// PlannerAgent implements AI-powered Chain-of-Thought planning
type PlannerAgent struct {
	*BaseAgent
	def      agent.AgentDef
	provider provider.Provider
	config   PlannerConfig
	rt       agent.Runtime

	// AI-specific planning fields
	planCache       map[string]*ReasoningPlan
	planHistory     []PlanExecutionHistory
	reasoningDepth  int
	metacognition   MetacognitionModule
}

// PlanExecutionHistory tracks plan performance
type PlanExecutionHistory struct {
	PlanID          string
	Problem         string
	StepsCompleted  int
	TotalSteps      int
	Success         bool
	ExecutionTime   time.Duration
	TokensUsed      int
}

// MetacognitionModule for self-reflection and improvement
type MetacognitionModule struct {
	SuccessPatterns   []string
	FailurePatterns   []string
	LearningInsights  map[string]float64
}

// Planning strategies
const (
	StrategyChainOfThought    = "chain_of_thought"
	StrategyTreeOfThought     = "tree_of_thought"
	StrategyReActPlanning     = "react_planning"
	StrategyMonteCarlo        = "monte_carlo"
	StrategyBackwardChaining  = "backward_chaining"
	StrategyHierarchicalPlan  = "hierarchical_plan" // Renamed to avoid conflict
)

func init() {
	agent.Register("planner", NewPlannerAgent)
}

// NewPlannerAgent creates a new AI-powered planner agent
func NewPlannerAgent(def agent.AgentDef, rt agent.Runtime) (agent.Agent, error) {
	var config PlannerConfig
	if err := def.UnmarshalKey("planner_config", &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal planner config: %w", err)
	}

	// Set AI-optimized defaults for planning
	if config.Temperature == 0 {
		config.Temperature = 0.7 // Higher for creative problem-solving
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = 2000 // More tokens for detailed planning
	}
	if config.MaxSteps == 0 {
		config.MaxSteps = 20
	}
	if config.ReasoningDepth == 0 {
		config.ReasoningDepth = 3
	}
	if config.PlanningStrategy == "" {
		config.PlanningStrategy = StrategyChainOfThought
	}
	if config.StepDetailLevel == "" {
		config.StepDetailLevel = "detailed"
	}

	// Initialize provider
	prov, err := initializeProvider(def.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LLM provider: %w", err)
	}

	return &PlannerAgent{
		def:            def,
		provider:       prov,
		config:         config,
		rt:             rt,
		planCache:      make(map[string]*ReasoningPlan),
		planHistory:    make([]PlanExecutionHistory, 0, 100),
		reasoningDepth: config.ReasoningDepth,
		metacognition: MetacognitionModule{
			LearningInsights: make(map[string]float64),
		},
	}, nil
}

// Execute performs synchronous planning
func (p *PlannerAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	if !p.Ready() {
		return nil, fmt.Errorf("agent not ready")
	}

	// Perform planning and return result
	plan, err := p.createPlan(ctx, input.Payload)
	if err != nil {
		return nil, err
	}

	// Convert plan to message
	return p.planToMessage(plan, input.Message)
}

// planToMessage converts a ReasoningPlan to an agent.Message
func (p *PlannerAgent) planToMessage(plan *ReasoningPlan, input *pb.Message) (*agent.Message, error) {
	// Marshal plan to JSON
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal plan: %w", err)
	}

	return &agent.Message{
		Message: &pb.Message{
			Type:    "plan",
			Payload: string(planJSON),
		},
	}, nil
}

// Start begins the planner agent's processing loop
func (p *PlannerAgent) Start(ctx context.Context) error {
	p.InitContext(ctx)
	if len(p.def.Inputs) == 0 {
		return fmt.Errorf("no inputs defined for PlannerAgent")
	}

	ch, err := p.rt.Recv(p.def.Inputs[0].Source)
	if err != nil {
		return fmt.Errorf("failed to receive from %s: %w", p.def.Inputs[0].Source, err)
	}

	validator := &security.StringValidator{
		MaxLength:            100000,
		DisallowNullBytes:    true,
		DisallowControlChars: true,
	}

	for m := range ch {
		if err := validator.Validate(m.Payload); err != nil {
			log.Printf("Planner input validation error: %v", err)
			continue
		}

		span := observability.StartSpan("planner.plan", map[string]any{
			"problem_length": len(m.Payload),
			"strategy":       p.config.PlanningStrategy,
		})

		plan, err := p.createPlan(ctx, m.Payload)
		span.End()

		if err != nil {
			log.Printf("Planning error: %v", err)
			continue
		}

		p.sendPlan(plan, m)
		p.recordPlanHistory(plan)
	}
	return nil
}

// createPlan generates a Chain-of-Thought reasoning plan
func (p *PlannerAgent) createPlan(ctx context.Context, problem string) (*ReasoningPlan, error) {
	// Check cache first
	if cached, exists := p.planCache[problem]; exists {
		log.Printf("Using cached plan for problem")
		return cached, nil
	}

	startTime := time.Now()

	// Select planning strategy
	var plan *ReasoningPlan
	var err error

	switch p.config.PlanningStrategy {
	case StrategyChainOfThought:
		plan, err = p.planWithChainOfThought(ctx, problem)
	case StrategyTreeOfThought:
		plan, err = p.planWithTreeOfThought(ctx, problem)
	case StrategyReActPlanning:
		plan, err = p.planWithReAct(ctx, problem)
	case StrategyBackwardChaining:
		plan, err = p.planWithBackwardChaining(ctx, problem)
	case StrategyHierarchicalPlan:
		plan, err = p.planHierarchically(ctx, problem)
	case StrategyMonteCarlo:
		plan, err = p.planWithMonteCarlo(ctx, problem)
	default:
		plan, err = p.planWithChainOfThought(ctx, problem)
	}

	if err != nil {
		return nil, err
	}

	// Apply self-critique if enabled
	if p.config.EnableSelfCritique {
		critique, err := p.selfCritique(ctx, plan)
		if err == nil {
			plan.SelfCritique = critique
		}
	}

	// Optimize plan based on learning
	p.optimizePlanWithLearning(plan)

	// Cache the plan
	p.planCache[problem] = plan

	// Record planning time
	plan.EstimatedDuration = fmt.Sprintf("%v", time.Since(startTime))

	return plan, nil
}

// planWithChainOfThought implements Chain-of-Thought reasoning
func (p *PlannerAgent) planWithChainOfThought(ctx context.Context, problem string) (*ReasoningPlan, error) {
	prompt := p.buildChainOfThoughtPrompt(problem)
	schema := p.buildPlanSchema()

	req := provider.StructuredRequest{
		CompletionRequest: provider.CompletionRequest{
			Messages: []provider.Message{
				{Role: "system", Content: p.getChainOfThoughtSystemPrompt()},
				{Role: "user", Content: prompt},
			},
			Model:       p.def.Model,
			Temperature: p.config.Temperature,
			MaxTokens:   p.config.MaxTokens,
		},
		ResponseSchema: schema,
		StrictSchema:   true,
	}

	resp, err := p.provider.CreateStructured(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Chain-of-Thought planning failed: %w", err)
	}

	var plan ReasoningPlan
	if err := json.Unmarshal(resp.Data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan: %w", err)
	}

	plan.TokensUsed = resp.Usage.TotalTokens
	plan.PlanningStrategy = StrategyChainOfThought

	// Identify critical path and parallel groups
	p.analyzePlanStructure(&plan)

	return &plan, nil
}

// planWithTreeOfThought explores multiple reasoning paths
func (p *PlannerAgent) planWithTreeOfThought(ctx context.Context, problem string) (*ReasoningPlan, error) {
	// TODO: Implement proper Tree-of-Thought planning
	// This should:
	// 1. Generate multiple reasoning paths/branches
	// 2. Evaluate each branch with scoring
	// 3. Potentially backtrack and explore alternatives
	// 4. Select the best path based on evaluation scores
	// Current implementation is a simplified stub

	// Generate multiple reasoning branches
	branches := p.generateReasoningBranches(ctx, problem, 3)

	// Evaluate and select best branch
	bestBranch := p.selectBestBranch(ctx, branches)

	// Convert to plan format
	return p.branchToPlan(bestBranch), nil
}

// planWithReAct combines reasoning with action planning
func (p *PlannerAgent) planWithReAct(ctx context.Context, problem string) (*ReasoningPlan, error) {
	prompt := fmt.Sprintf(`Problem: %s

Create a ReAct-style plan that alternates between:
1. Thought: Reasoning about the current state
2. Action: Concrete step to take
3. Observation: Expected result

Continue this cycle until the problem is solved.`, problem)

	req := provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "system", Content: p.getReActSystemPrompt()},
			{Role: "user", Content: prompt},
		},
		Model:         p.def.Model,
		Temperature:   p.config.Temperature,
		MaxTokens:     p.config.MaxTokens,
		MaxIterations: p.config.MaxSteps,
	}

	resp, err := p.provider.CreateCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ReAct planning failed: %w", err)
	}

	// Parse ReAct output into plan structure
	return p.parseReActToPlan(resp.Content, resp.Usage.TotalTokens), nil
}

// planWithBackwardChaining starts from goal and works backward
func (p *PlannerAgent) planWithBackwardChaining(ctx context.Context, problem string) (*ReasoningPlan, error) {
	// TODO: Implement proper backward chaining with goal decomposition
	// This should:
	// 1. Parse the goal state from the problem
	// 2. Recursively decompose goals into subgoals
	// 3. Identify preconditions for each subgoal
	// 4. Build a dependency graph
	// 5. Reverse the chain to create an executable plan
	// Current implementation uses a simple prompt-based approach

	prompt := fmt.Sprintf(`Problem: %s

Use backward chaining to create a plan:
1. Start with the desired end state
2. Identify what must be true immediately before
3. Continue backward until reaching the current state
4. Reverse the chain to create forward steps`, problem)

	req := provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "system", Content: p.getBackwardChainingSystemPrompt()},
			{Role: "user", Content: prompt},
		},
		Model:       p.def.Model,
		Temperature: p.config.Temperature,
		MaxTokens:   p.config.MaxTokens,
	}

	resp, err := p.provider.CreateCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("backward chaining failed: %w", err)
	}

	return p.parseBackwardChainToPlan(resp.Content, resp.Usage.TotalTokens), nil
}

// planHierarchically creates multi-level plans
func (p *PlannerAgent) planHierarchically(ctx context.Context, problem string) (*ReasoningPlan, error) {
	// First, create high-level plan
	highLevel, err := p.createHighLevelPlan(ctx, problem)
	if err != nil {
		return nil, err
	}

	// Then decompose each high-level step
	detailedSteps := []PlanStep{}
	for _, hlStep := range highLevel.Steps {
		subSteps, err := p.decomposeStep(ctx, hlStep)
		if err != nil {
			log.Printf("Failed to decompose step: %v", err)
			detailedSteps = append(detailedSteps, hlStep)
		} else {
			detailedSteps = append(detailedSteps, subSteps...)
		}
	}

	highLevel.Steps = detailedSteps
	highLevel.PlanningStrategy = StrategyHierarchicalPlan
	return highLevel, nil
}

// planWithMonteCarlo simulates multiple plan paths
func (p *PlannerAgent) planWithMonteCarlo(ctx context.Context, problem string) (*ReasoningPlan, error) {
	// TODO: Implement proper Monte Carlo Tree Search (MCTS) planning
	// This should:
	// 1. Build a tree of possible action sequences
	// 2. Use random simulations to evaluate paths
	// 3. Apply UCB1 or similar algorithm for exploration/exploitation balance
	// 4. Backpropagate results to update node statistics
	// Current implementation is a simplified stub that just generates multiple plans

	numSimulations := 5
	plans := make([]*ReasoningPlan, 0, numSimulations)

	// Generate multiple plan simulations
	for i := 0; i < numSimulations; i++ {
		plan, err := p.simulatePlan(ctx, problem, i)
		if err != nil {
			log.Printf("Simulation %d failed: %v", i, err)
			continue
		}
		plans = append(plans, plan)
	}

	if len(plans) == 0 {
		return nil, fmt.Errorf("all Monte Carlo simulations failed")
	}

	// Select best plan based on success probability
	return p.selectBestPlan(plans), nil
}

// Helper methods for prompt building

func (p *PlannerAgent) buildChainOfThoughtPrompt(problem string) string {
	examplesStr := ""
	if len(p.config.ExamplePlans) > 0 {
		examplesStr = "\n\nExample plans for reference:\n"
		for i, ex := range p.config.ExamplePlans {
			if i >= 2 { // Limit examples to save tokens
				break
			}
			examplesStr += fmt.Sprintf("\nProblem: %s\nSteps: %v\nExplanation: %s\n",
				ex.Problem, ex.Steps, ex.Explanation)
		}
	}

	return fmt.Sprintf(`Create a detailed Chain-of-Thought reasoning plan for the following problem:

"%s"

%s

Requirements:
1. Break down the problem systematically
2. Create logical, sequential steps
3. For each step, provide:
   - Clear action description
   - Reasoning behind the step
   - Expected outcome
   - Complexity assessment
   - Whether it can be parallelized
4. Identify prerequisites and dependencies
5. Consider alternative approaches where applicable
6. Assess risks and provide contingencies
7. Define clear success criteria

Think step by step, showing your reasoning process explicitly.`, problem, examplesStr)
}

func (p *PlannerAgent) getChainOfThoughtSystemPrompt() string {
	return `You are an expert AI planning system specializing in Chain-of-Thought reasoning.

Your capabilities:
- Systematic problem decomposition
- Logical step sequencing
- Dependency analysis
- Risk assessment
- Parallel execution identification
- Alternative path generation

Create comprehensive, executable plans that:
1. Are logically sound and complete
2. Account for edge cases and failures
3. Optimize for efficiency
4. Provide clear success metrics
5. Include reasoning transparency

Always think step-by-step and show your work.`
}

func (p *PlannerAgent) getReActSystemPrompt() string {
	return `You are a ReAct planning agent. Create plans that interleave:
- Thought: Reasoning about the current situation
- Action: Concrete steps to take
- Observation: Expected results or feedback

This creates a dynamic planning process that adapts based on observations.`
}

func (p *PlannerAgent) getBackwardChainingSystemPrompt() string {
	return `You are a backward chaining planner. Start from the goal state and work backward to the current state, then reverse the chain to create executable forward steps.`
}

func (p *PlannerAgent) buildPlanSchema() json.RawMessage {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"problem": map[string]any{"type": "string"},
			"analysis": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"problem_type":  map[string]any{"type": "string"},
					"domain":        map[string]any{"type": "string"},
					"constraints":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"key_challenges": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				},
			},
			"steps": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"step_number":      map[string]any{"type": "number"},
						"action":           map[string]any{"type": "string"},
						"reasoning":        map[string]any{"type": "string"},
						"expected_outcome": map[string]any{"type": "string"},
						"complexity":       map[string]any{"type": "string"},
						"confidence":       map[string]any{"type": "number"},
						"can_parallelize":  map[string]any{"type": "boolean"},
					},
				},
			},
			"success_criteria": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"total_complexity": map[string]any{"type": "string"},
		},
		"required": []string{"problem", "steps", "success_criteria"},
	}

	data, _ := json.Marshal(schema)
	return data
}

// Analysis and optimization methods

func (p *PlannerAgent) analyzePlanStructure(plan *ReasoningPlan) {
	// Identify critical path (longest dependency chain)
	criticalPath := p.findCriticalPath(plan.Steps)
	plan.CriticalPath = criticalPath

	// Identify parallel execution groups
	if p.config.ParallelizableSteps {
		parallelGroups := p.identifyParallelGroups(plan.Steps)
		plan.ParallelGroups = parallelGroups
	}

	// Set execution strategy
	if len(plan.ParallelGroups) > 0 {
		plan.ExecutionStrategy = "parallel_optimized"
	} else {
		plan.ExecutionStrategy = "sequential"
	}
}

func (p *PlannerAgent) findCriticalPath(steps []PlanStep) []int {
	// Simplified critical path - in production, use proper graph algorithms
	path := make([]int, 0, len(steps))
	for i := range steps {
		if len(steps[i].Prerequisites) == 0 || i == 0 {
			path = append(path, steps[i].StepNumber)
		}
	}
	return path
}

func (p *PlannerAgent) identifyParallelGroups(steps []PlanStep) [][]int {
	var groups [][]int
	processed := make(map[int]bool)

	for _, step := range steps {
		if processed[step.StepNumber] {
			continue
		}

		if step.CanParallelize {
			group := []int{step.StepNumber}
			processed[step.StepNumber] = true

			// Find other steps that can run in parallel
			for _, other := range steps {
				if other.StepNumber != step.StepNumber &&
					other.CanParallelize &&
					!processed[other.StepNumber] &&
					!p.hasDependency(step, other) {
					group = append(group, other.StepNumber)
					processed[other.StepNumber] = true
				}
			}

			if len(group) > 1 {
				groups = append(groups, group)
			}
		}
	}

	return groups
}

func (p *PlannerAgent) hasDependency(step1, step2 PlanStep) bool {
	for _, prereq := range step2.Prerequisites {
		if prereq == step1.StepNumber {
			return true
		}
	}
	for _, prereq := range step1.Prerequisites {
		if prereq == step2.StepNumber {
			return true
		}
	}
	return false
}

func (p *PlannerAgent) selfCritique(ctx context.Context, plan *ReasoningPlan) (string, error) {
	critiquePrompt := fmt.Sprintf(`Critically evaluate this plan:

Problem: %s
Steps: %d
Strategy: %s

Assess:
1. Completeness - Does it solve the problem fully?
2. Efficiency - Are there redundant or unnecessary steps?
3. Risks - What could go wrong?
4. Improvements - How could it be better?

Provide a concise critique.`, plan.Problem, len(plan.Steps), plan.PlanningStrategy)

	req := provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "system", Content: "You are a critical planning analyst. Identify weaknesses and suggest improvements."},
			{Role: "user", Content: critiquePrompt},
		},
		Model:       p.def.Model,
		Temperature: 0.3, // Lower temperature for critical analysis
		MaxTokens:   500,
	}

	resp, err := p.provider.CreateCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

func (p *PlannerAgent) optimizePlanWithLearning(plan *ReasoningPlan) {
	// Apply learned insights to optimize the plan
	for insight, weight := range p.metacognition.LearningInsights {
		if weight > 0.7 && strings.Contains(plan.Problem, insight) {
			log.Printf("Applying learned insight: %s (weight: %.2f)", insight, weight)
			// Adjust plan confidence based on past success
			for i := range plan.Steps {
				plan.Steps[i].Confidence *= (1 + weight/10)
			}
		}
	}
}

func (p *PlannerAgent) recordPlanHistory(plan *ReasoningPlan) {
	history := PlanExecutionHistory{
		PlanID:     fmt.Sprintf("plan_%d", time.Now().UnixNano()),
		Problem:    plan.Problem,
		TotalSteps: len(plan.Steps),
		TokensUsed: plan.TokensUsed,
	}

	p.planHistory = append(p.planHistory, history)

	// Keep only last 100 records
	if len(p.planHistory) > 100 {
		p.planHistory = p.planHistory[len(p.planHistory)-100:]
	}

	// Update learning insights periodically
	if len(p.planHistory)%10 == 0 {
		p.updateLearningInsights()
	}
}

func (p *PlannerAgent) updateLearningInsights() {
	// Analyze patterns in successful plans
	for _, history := range p.planHistory {
		if history.Success {
			// Extract problem features and update weights
			features := p.extractProblemFeatures(history.Problem)
			for _, feature := range features {
				current := p.metacognition.LearningInsights[feature]
				p.metacognition.LearningInsights[feature] = current*0.9 + 0.1 // Exponential moving average
			}
		}
	}
}

func (p *PlannerAgent) extractProblemFeatures(problem string) []string {
	// Simple feature extraction - in production, use NLP techniques
	features := []string{}
	keywords := []string{"optimize", "analyze", "implement", "design", "debug", "refactor"}
	for _, kw := range keywords {
		if strings.Contains(strings.ToLower(problem), kw) {
			features = append(features, kw)
		}
	}
	return features
}

// Utility methods for alternative planning strategies

func (p *PlannerAgent) generateReasoningBranches(ctx context.Context, problem string, numBranches int) []string {
	// TODO: Implement proper branch generation for Tree-of-Thought
	// This is a simplified stub - should actually generate diverse reasoning paths
	branches := make([]string, numBranches)
	for i := 0; i < numBranches; i++ {
		branches[i] = fmt.Sprintf("Branch %d for: %s", i+1, problem)
	}
	return branches
}

func (p *PlannerAgent) selectBestBranch(ctx context.Context, branches []string) string {
	// TODO: Implement proper branch evaluation and selection
	// Should score branches based on viability, completeness, etc.
	// This is a simplified stub - just selects first branch
	if len(branches) > 0 {
		return branches[0]
	}
	return ""
}

func (p *PlannerAgent) branchToPlan(branch string) *ReasoningPlan {
	// Convert branch to plan format
	return &ReasoningPlan{
		Problem:          branch,
		PlanningStrategy: StrategyTreeOfThought,
		Steps:            []PlanStep{{StepNumber: 1, Action: "Execute branch"}},
	}
}

func (p *PlannerAgent) parseReActToPlan(content string, tokens int) *ReasoningPlan {
	// Parse ReAct format to plan structure
	return &ReasoningPlan{
		Problem:          "ReAct problem",
		PlanningStrategy: StrategyReActPlanning,
		Steps:            []PlanStep{{StepNumber: 1, Action: content}},
		TokensUsed:       tokens,
	}
}

func (p *PlannerAgent) parseBackwardChainToPlan(content string, tokens int) *ReasoningPlan {
	return &ReasoningPlan{
		Problem:          "Backward chain problem",
		PlanningStrategy: StrategyBackwardChaining,
		Steps:            []PlanStep{{StepNumber: 1, Action: content}},
		TokensUsed:       tokens,
	}
}

func (p *PlannerAgent) createHighLevelPlan(ctx context.Context, problem string) (*ReasoningPlan, error) {
	// Create high-level abstract plan
	return p.planWithChainOfThought(ctx, "High-level: "+problem)
}

func (p *PlannerAgent) decomposeStep(ctx context.Context, step PlanStep) ([]PlanStep, error) {
	// Decompose a high-level step into sub-steps
	subSteps := []PlanStep{
		{
			StepNumber: step.StepNumber,
			Action:     step.Action + " (decomposed)",
			Reasoning:  "Detailed implementation",
		},
	}
	return subSteps, nil
}

func (p *PlannerAgent) simulatePlan(ctx context.Context, problem string, seed int) (*ReasoningPlan, error) {
	// TODO: Implement proper Monte Carlo simulation with randomization
	// Should use the seed to create variations in the planning process
	// Current implementation just reuses Chain-of-Thought with a label

	// Simulate a plan execution with variation
	plan, err := p.planWithChainOfThought(ctx, problem)
	if err != nil {
		return nil, err
	}
	// Add variation based on seed
	plan.PlanningStrategy = fmt.Sprintf("%s_simulation_%d", StrategyMonteCarlo, seed)
	return plan, nil
}

func (p *PlannerAgent) selectBestPlan(plans []*ReasoningPlan) *ReasoningPlan {
	if len(plans) == 0 {
		return nil
	}
	// Select plan with highest average confidence
	bestPlan := plans[0]
	bestScore := p.calculatePlanScore(bestPlan)

	for _, plan := range plans[1:] {
		score := p.calculatePlanScore(plan)
		if score > bestScore {
			bestPlan = plan
			bestScore = score
		}
	}
	// Reset strategy to monte_carlo (from monte_carlo_simulation_N)
	bestPlan.PlanningStrategy = StrategyMonteCarlo
	return bestPlan
}

func (p *PlannerAgent) calculatePlanScore(plan *ReasoningPlan) float64 {
	if len(plan.Steps) == 0 {
		return 0
	}
	var totalConfidence float64
	for _, step := range plan.Steps {
		totalConfidence += step.Confidence
	}
	return totalConfidence / float64(len(plan.Steps))
}

func (p *PlannerAgent) sendPlan(plan *ReasoningPlan, originalMsg *agent.Message) {
	planJSON, _ := json.Marshal(plan)

	out := &agent.Message{Message: &pb.Message{
		Id:        originalMsg.Id,
		Type:      "reasoning_plan",
		Payload:   string(planJSON),
		Timestamp: time.Now().Format(time.RFC3339),
	}}

	for _, o := range p.def.Outputs {
		if err := p.rt.Send(o.Target, out); err != nil {
			log.Printf("Error sending plan to %s: %v", o.Target, err)
		}
	}
}