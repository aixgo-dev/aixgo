package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aixgo-dev/aixgo/internal/llm/parser"
	"github.com/aixgo-dev/aixgo/internal/llm/provider"
)

// TestCase represents a single test case for evaluation
type TestCase struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Input        string         `json:"input"`
	ExpectedTool string         `json:"expected_tool,omitempty"`
	ExpectedArgs map[string]any `json:"expected_args,omitempty"`
	ExpectedType string         `json:"expected_type"` // "tool_call" or "direct_answer"
	Category     string         `json:"category"`
	Difficulty   string         `json:"difficulty"` // easy, medium, hard
	Tags         []string       `json:"tags"`
}

// TestSuite represents a collection of test cases
type TestSuite struct {
	Name      string     `json:"name"`
	Version   string     `json:"version"`
	TestCases []TestCase `json:"test_cases"`
}

// BenchmarkResult represents the result of a single test
type BenchmarkResult struct {
	TestCase     TestCase            `json:"test_case"`
	Success      bool                `json:"success"`
	ActualOutput string              `json:"actual_output"`
	ParsedResult *parser.ParseResult `json:"parsed_result,omitempty"`
	Latency      time.Duration       `json:"latency"`
	TokensUsed   int                 `json:"tokens_used"`
	Iterations   int                 `json:"iterations"`
	ErrorMessage string              `json:"error_message,omitempty"`
	Timestamp    time.Time           `json:"timestamp"`
}

// ModelBenchmark represents benchmark results for a model
type ModelBenchmark struct {
	ModelName string            `json:"model_name"`
	TestSuite string            `json:"test_suite"`
	Results   []BenchmarkResult `json:"results"`
	Summary   BenchmarkSummary  `json:"summary"`
	StartTime time.Time         `json:"start_time"`
	EndTime   time.Time         `json:"end_time"`
}

// BenchmarkSummary provides summary statistics
type BenchmarkSummary struct {
	TotalTests          int                         `json:"total_tests"`
	PassedTests         int                         `json:"passed_tests"`
	FailedTests         int                         `json:"failed_tests"`
	SuccessRate         float64                     `json:"success_rate"`
	AverageLatency      time.Duration               `json:"average_latency"`
	MedianLatency       time.Duration               `json:"median_latency"`
	P95Latency          time.Duration               `json:"p95_latency"`
	TotalTokens         int                         `json:"total_tokens"`
	AverageIterations   float64                     `json:"average_iterations"`
	CategoryBreakdown   map[string]*CategoryStats   `json:"category_breakdown"`
	DifficultyBreakdown map[string]*DifficultyStats `json:"difficulty_breakdown"`
}

// CategoryStats provides statistics per category
type CategoryStats struct {
	Total       int     `json:"total"`
	Passed      int     `json:"passed"`
	SuccessRate float64 `json:"success_rate"`
}

// DifficultyStats provides statistics per difficulty level
type DifficultyStats struct {
	Total       int           `json:"total"`
	Passed      int           `json:"passed"`
	SuccessRate float64       `json:"success_rate"`
	AvgLatency  time.Duration `json:"avg_latency"`
}

// Evaluator runs benchmarks on LLM providers
type Evaluator struct {
	provider provider.Provider
	parser   *parser.ReActParser
	timeout  time.Duration
}

// NewEvaluator creates a new evaluator
func NewEvaluator(provider provider.Provider, modelName string) *Evaluator {
	return &Evaluator{
		provider: provider,
		parser:   parser.NewReActParser(modelName, false),
		timeout:  30 * time.Second,
	}
}

// GetDefaultTestSuite returns a comprehensive test suite
func GetDefaultTestSuite() *TestSuite {
	return &TestSuite{
		Name:    "ReAct Tool Calling Benchmark",
		Version: "1.0.0",
		TestCases: []TestCase{
			// Easy test cases
			{
				ID:           "easy_weather_1",
				Name:         "Simple Weather Query",
				Description:  "Basic weather information request",
				Input:        "What's the weather in Paris?",
				ExpectedTool: "get_weather",
				ExpectedArgs: map[string]any{
					"location": "Paris",
				},
				ExpectedType: "tool_call",
				Category:     "weather",
				Difficulty:   "easy",
				Tags:         []string{"weather", "location"},
			},
			{
				ID:           "easy_calc_1",
				Name:         "Simple Addition",
				Description:  "Basic mathematical calculation",
				Input:        "Calculate 25 plus 37",
				ExpectedTool: "calculate",
				ExpectedArgs: map[string]any{
					"operation": "add",
					"a":         25.0,
					"b":         37.0,
				},
				ExpectedType: "tool_call",
				Category:     "math",
				Difficulty:   "easy",
				Tags:         []string{"calculation", "addition"},
			},
			{
				ID:           "easy_direct_1",
				Name:         "Direct Answer",
				Description:  "Question that doesn't need tools",
				Input:        "What is the capital of France?",
				ExpectedType: "direct_answer",
				Category:     "knowledge",
				Difficulty:   "easy",
				Tags:         []string{"geography", "direct"},
			},

			// Medium test cases
			{
				ID:           "medium_weather_1",
				Name:         "Weather with Units",
				Description:  "Weather query with specific units",
				Input:        "Give me the temperature in Tokyo in Celsius",
				ExpectedTool: "get_weather",
				ExpectedArgs: map[string]any{
					"location": "Tokyo",
					"units":    "celsius",
				},
				ExpectedType: "tool_call",
				Category:     "weather",
				Difficulty:   "medium",
				Tags:         []string{"weather", "units"},
			},
			{
				ID:           "medium_calc_1",
				Name:         "Complex Calculation",
				Description:  "Multi-step calculation",
				Input:        "What's 150 multiplied by 3.5?",
				ExpectedTool: "calculate",
				ExpectedArgs: map[string]any{
					"operation": "multiply",
					"a":         150.0,
					"b":         3.5,
				},
				ExpectedType: "tool_call",
				Category:     "math",
				Difficulty:   "medium",
				Tags:         []string{"calculation", "multiplication"},
			},
			{
				ID:           "medium_search_1",
				Name:         "Search Query",
				Description:  "Information search request",
				Input:        "Search for information about quantum computing",
				ExpectedTool: "search",
				ExpectedArgs: map[string]any{
					"query": "quantum computing",
				},
				ExpectedType: "tool_call",
				Category:     "search",
				Difficulty:   "medium",
				Tags:         []string{"search", "technology"},
			},

			// Hard test cases
			{
				ID:           "hard_weather_1",
				Name:         "Ambiguous Location",
				Description:  "Weather query with ambiguous location",
				Input:        "How's the weather in Springfield? I mean the one in Illinois",
				ExpectedTool: "get_weather",
				ExpectedArgs: map[string]any{
					"location": "Springfield, Illinois",
				},
				ExpectedType: "tool_call",
				Category:     "weather",
				Difficulty:   "hard",
				Tags:         []string{"weather", "disambiguation"},
			},
			{
				ID:           "hard_multi_1",
				Name:         "Implicit Tool Need",
				Description:  "Query that implicitly requires a tool",
				Input:        "Is it warmer in Miami or Seattle right now?",
				ExpectedTool: "get_weather",
				ExpectedType: "tool_call",
				Category:     "weather",
				Difficulty:   "hard",
				Tags:         []string{"weather", "comparison"},
			},
			{
				ID:           "hard_context_1",
				Name:         "Context-Dependent",
				Description:  "Query requiring context understanding",
				Input:        "If I have 3 boxes with 12 items each, how many items total?",
				ExpectedTool: "calculate",
				ExpectedArgs: map[string]any{
					"operation": "multiply",
					"a":         3.0,
					"b":         12.0,
				},
				ExpectedType: "tool_call",
				Category:     "math",
				Difficulty:   "hard",
				Tags:         []string{"calculation", "word_problem"},
			},
		},
	}
}

// RunBenchmark runs a complete benchmark suite
func (e *Evaluator) RunBenchmark(ctx context.Context, suite *TestSuite, modelName string) (*ModelBenchmark, error) {
	benchmark := &ModelBenchmark{
		ModelName: modelName,
		TestSuite: suite.Name,
		StartTime: time.Now(),
		Results:   make([]BenchmarkResult, 0, len(suite.TestCases)),
	}

	// Run tests sequentially to avoid overwhelming the model
	for _, testCase := range suite.TestCases {
		result := e.runSingleTest(ctx, testCase)
		benchmark.Results = append(benchmark.Results, result)

		// Small delay between tests
		time.Sleep(100 * time.Millisecond)
	}

	benchmark.EndTime = time.Now()
	benchmark.Summary = e.calculateSummary(benchmark.Results)

	return benchmark, nil
}

// runSingleTest runs a single test case
func (e *Evaluator) runSingleTest(ctx context.Context, testCase TestCase) BenchmarkResult {
	result := BenchmarkResult{
		TestCase:  testCase,
		Timestamp: time.Now(),
	}

	// Create timeout context
	testCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	startTime := time.Now()

	// Create completion request
	req := provider.CompletionRequest{
		Messages: []provider.Message{
			{
				Role:    "user",
				Content: testCase.Input,
			},
		},
		MaxTokens:   256,
		Temperature: 0.3,
		Tools:       e.getTestTools(),
	}

	// Execute completion
	resp, err := e.provider.CreateCompletion(testCtx, req)
	result.Latency = time.Since(startTime)

	if err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result
	}

	// Parse response
	parseResult, err := e.parser.Parse(resp.Content)
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("parse error: %v", err)
		return result
	}

	result.ActualOutput = resp.Content
	result.ParsedResult = parseResult
	result.TokensUsed = resp.Usage.TotalTokens

	// Note: Iterations tracking would require provider support for metadata
	// For now, this is set to 0 by default in BenchmarkResult initialization

	// Evaluate success
	result.Success = e.evaluateResult(testCase, parseResult)

	return result
}

// evaluateResult checks if the result matches expectations
func (e *Evaluator) evaluateResult(testCase TestCase, result *parser.ParseResult) bool {
	switch testCase.ExpectedType {
	case "tool_call":
		if result.ToolCall == nil {
			return false
		}

		// Check tool name
		if testCase.ExpectedTool != "" && result.ToolCall.Action != testCase.ExpectedTool {
			// Allow some flexibility for similar tool names
			if !e.fuzzyMatchToolName(result.ToolCall.Action, testCase.ExpectedTool) {
				return false
			}
		}

		// Check arguments if specified
		if testCase.ExpectedArgs != nil {
			return e.validateArguments(testCase.ExpectedArgs, result.ToolCall.ActionInput)
		}

		return true

	case "direct_answer":
		// Should have final answer, not tool call
		return result.FinalAnswer != "" && result.ToolCall == nil

	default:
		// Unknown type, check if we got something reasonable
		return result.FinalAnswer != "" || result.ToolCall != nil
	}
}

// fuzzyMatchToolName allows for slight variations in tool names
func (e *Evaluator) fuzzyMatchToolName(actual, expected string) bool {
	actual = strings.ToLower(actual)
	expected = strings.ToLower(expected)

	// Exact match
	if actual == expected {
		return true
	}

	// Check if one contains the other
	if strings.Contains(actual, expected) || strings.Contains(expected, actual) {
		return true
	}

	// Common variations
	variations := map[string][]string{
		"calculate": {"calc", "compute", "math"},
		"search":    {"find", "query", "lookup"},
		"weather":   {"get_weather", "check_weather", "weather_info"},
	}

	for base, vars := range variations {
		if strings.Contains(expected, base) || strings.Contains(actual, base) {
			for _, v := range vars {
				if strings.Contains(actual, v) || strings.Contains(expected, v) {
					return true
				}
			}
		}
	}

	return false
}

// validateArguments checks if arguments match expectations
func (e *Evaluator) validateArguments(expected, actual map[string]any) bool {
	for key, expectedValue := range expected {
		actualValue, exists := actual[key]
		if !exists {
			// Check for common key variations
			if altValue := e.findAlternativeKey(key, actual); altValue != nil {
				actualValue = altValue
			} else {
				return false
			}
		}

		// Normalize and compare values
		if !e.valuesMatch(expectedValue, actualValue) {
			return false
		}
	}

	return true
}

// findAlternativeKey looks for common key variations
func (e *Evaluator) findAlternativeKey(key string, args map[string]any) any {
	alternatives := map[string][]string{
		"location": {"city", "place", "loc"},
		"query":    {"q", "search", "text"},
		"a":        {"first", "num1", "operand1"},
		"b":        {"second", "num2", "operand2"},
	}

	if alts, exists := alternatives[key]; exists {
		for _, alt := range alts {
			if val, ok := args[alt]; ok {
				return val
			}
		}
	}

	return nil
}

// valuesMatch checks if two values are equivalent
func (e *Evaluator) valuesMatch(expected, actual any) bool {
	// Try direct comparison
	if expected == actual {
		return true
	}

	// Convert to strings and compare
	expectedStr := fmt.Sprintf("%v", expected)
	actualStr := fmt.Sprintf("%v", actual)

	// Case-insensitive string comparison
	if strings.EqualFold(expectedStr, actualStr) {
		return true
	}

	// Number comparison with tolerance
	if expectedNum, ok := toFloat64(expected); ok {
		if actualNum, ok := toFloat64(actual); ok {
			tolerance := 0.001
			return abs(expectedNum-actualNum) < tolerance
		}
	}

	// Partial string matching for locations
	if strings.Contains(strings.ToLower(actualStr), strings.ToLower(expectedStr)) {
		return true
	}

	return false
}

// calculateSummary generates summary statistics
func (e *Evaluator) calculateSummary(results []BenchmarkResult) BenchmarkSummary {
	summary := BenchmarkSummary{
		TotalTests:          len(results),
		CategoryBreakdown:   make(map[string]*CategoryStats),
		DifficultyBreakdown: make(map[string]*DifficultyStats),
	}

	var latencies []time.Duration
	var totalIterations int

	for _, result := range results {
		if result.Success {
			summary.PassedTests++
		} else {
			summary.FailedTests++
		}

		summary.TotalTokens += result.TokensUsed
		totalIterations += result.Iterations
		latencies = append(latencies, result.Latency)

		// Update category stats
		if _, exists := summary.CategoryBreakdown[result.TestCase.Category]; !exists {
			summary.CategoryBreakdown[result.TestCase.Category] = &CategoryStats{}
		}
		summary.CategoryBreakdown[result.TestCase.Category].Total++
		if result.Success {
			summary.CategoryBreakdown[result.TestCase.Category].Passed++
		}

		// Update difficulty stats
		if _, exists := summary.DifficultyBreakdown[result.TestCase.Difficulty]; !exists {
			summary.DifficultyBreakdown[result.TestCase.Difficulty] = &DifficultyStats{}
		}
		summary.DifficultyBreakdown[result.TestCase.Difficulty].Total++
		if result.Success {
			summary.DifficultyBreakdown[result.TestCase.Difficulty].Passed++
		}
	}

	// Calculate rates
	if summary.TotalTests > 0 {
		summary.SuccessRate = float64(summary.PassedTests) / float64(summary.TotalTests)
		summary.AverageIterations = float64(totalIterations) / float64(summary.TotalTests)
	}

	// Calculate latency statistics
	if len(latencies) > 0 {
		summary.AverageLatency = calculateAverage(latencies)
		summary.MedianLatency = calculateMedian(latencies)
		summary.P95Latency = calculatePercentile(latencies, 95)
	}

	// Calculate category success rates
	for _, stats := range summary.CategoryBreakdown {
		if stats.Total > 0 {
			stats.SuccessRate = float64(stats.Passed) / float64(stats.Total)
		}
	}

	// Calculate difficulty success rates and latencies
	for difficulty, stats := range summary.DifficultyBreakdown {
		if stats.Total > 0 {
			stats.SuccessRate = float64(stats.Passed) / float64(stats.Total)

			// Calculate average latency for this difficulty
			var diffLatencies []time.Duration
			for _, result := range results {
				if result.TestCase.Difficulty == difficulty {
					diffLatencies = append(diffLatencies, result.Latency)
				}
			}
			if len(diffLatencies) > 0 {
				stats.AvgLatency = calculateAverage(diffLatencies)
			}
		}
	}

	return summary
}

// getTestTools returns a standard set of test tools
func (e *Evaluator) getTestTools() []provider.Tool {
	// Helper to marshal schema to JSON - returns empty JSON object on error
	// This is safe because tool schemas are static/hardcoded and errors indicate
	// a programming bug rather than runtime condition
	safeMarshal := func(schema map[string]any) json.RawMessage {
		data, err := json.Marshal(schema)
		if err != nil {
			// Log the error and return empty object to prevent crash
			// In practice, this should never happen with valid static schemas
			fmt.Printf("ERROR: failed to marshal tool schema: %v\n", err)
			return json.RawMessage("{}")
		}
		return data
	}

	return []provider.Tool{
		{
			Name:        "get_weather",
			Description: "Get current weather information for a location",
			Parameters: safeMarshal(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"location": map[string]any{"type": "string"},
					"units":    map[string]any{"type": "string", "enum": []string{"celsius", "fahrenheit"}},
				},
				"required": []string{"location"},
			}),
		},
		{
			Name:        "calculate",
			Description: "Perform mathematical calculations",
			Parameters: safeMarshal(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"operation": map[string]any{"type": "string", "enum": []string{"add", "subtract", "multiply", "divide"}},
					"a":         map[string]any{"type": "number"},
					"b":         map[string]any{"type": "number"},
				},
				"required": []string{"operation", "a", "b"},
			}),
		},
		{
			Name:        "search",
			Description: "Search for information on the internet",
			Parameters: safeMarshal(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
				},
				"required": []string{"query"},
			}),
		},
	}
}

// Utility functions

func toFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		var f float64
		_, err := fmt.Sscanf(val, "%f", &f)
		return f, err == nil
	default:
		return 0, false
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func calculateAverage(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	return sum / time.Duration(len(durations))
}

func calculateMedian(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	// Simple bubble sort for small arrays
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func calculatePercentile(durations []time.Duration, percentile int) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	// Simple bubble sort for small arrays
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}
	index := (percentile * len(sorted)) / 100
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

// ExportResults exports benchmark results to JSON
func ExportResults(benchmark *ModelBenchmark, filename string) error {
	data, err := json.MarshalIndent(benchmark, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal results: %w", err)
	}
	// Note: File writing would be handled by the caller
	fmt.Printf("Benchmark results:\n%s\n", string(data))
	return nil
}

// CompareModels compares benchmark results between models
func CompareModels(benchmarks ...*ModelBenchmark) map[string]any {
	comparison := make(map[string]any)

	for _, benchmark := range benchmarks {
		modelStats := map[string]any{
			"success_rate":       benchmark.Summary.SuccessRate,
			"average_latency_ms": benchmark.Summary.AverageLatency.Milliseconds(),
			"total_tokens":       benchmark.Summary.TotalTokens,
			"average_iterations": benchmark.Summary.AverageIterations,
		}

		// Add difficulty breakdown
		difficultyStats := make(map[string]any)
		for diff, stats := range benchmark.Summary.DifficultyBreakdown {
			difficultyStats[diff] = map[string]any{
				"success_rate":   stats.SuccessRate,
				"avg_latency_ms": stats.AvgLatency.Milliseconds(),
			}
		}
		modelStats["by_difficulty"] = difficultyStats

		comparison[benchmark.ModelName] = modelStats
	}

	return comparison
}
