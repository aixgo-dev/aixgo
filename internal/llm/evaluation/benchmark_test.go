package evaluation

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/aixgo-dev/aixgo/internal/llm/parser"
)

func TestGetDefaultTestSuite(t *testing.T) {
	suite := GetDefaultTestSuite()

	if suite.Name == "" {
		t.Error("TestSuite should have a name")
	}
	if len(suite.TestCases) == 0 {
		t.Error("TestSuite should have test cases")
	}

	// Check test case distribution
	difficulties := make(map[string]int)
	categories := make(map[string]int)
	for _, tc := range suite.TestCases {
		difficulties[tc.Difficulty]++
		categories[tc.Category]++
	}

	if len(difficulties) < 2 {
		t.Error("TestSuite should have multiple difficulty levels")
	}
	if len(categories) < 2 {
		t.Error("TestSuite should have multiple categories")
	}
}

func TestEvaluator_EvaluateResult(t *testing.T) {
	e := &Evaluator{}

	tests := []struct {
		name     string
		testCase TestCase
		result   *parser.ParseResult
		want     bool
	}{
		{
			name: "correct tool call",
			testCase: TestCase{
				ExpectedType: "tool_call",
				ExpectedTool: "get_weather",
			},
			result: &parser.ParseResult{
				ToolCall: &parser.ToolCall{Action: "get_weather"},
			},
			want: true,
		},
		{
			name: "no tool call when expected",
			testCase: TestCase{
				ExpectedType: "tool_call",
				ExpectedTool: "get_weather",
			},
			result: &parser.ParseResult{
				FinalAnswer: "some answer",
			},
			want: false,
		},
		{
			name: "direct answer expected",
			testCase: TestCase{
				ExpectedType: "direct_answer",
			},
			result: &parser.ParseResult{
				FinalAnswer: "Paris is the capital of France",
			},
			want: true,
		},
		{
			name: "direct answer but got tool call",
			testCase: TestCase{
				ExpectedType: "direct_answer",
			},
			result: &parser.ParseResult{
				ToolCall: &parser.ToolCall{Action: "search"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.evaluateResult(tt.testCase, tt.result)
			if got != tt.want {
				t.Errorf("evaluateResult() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluator_FuzzyMatchToolName(t *testing.T) {
	e := &Evaluator{}

	tests := []struct {
		actual   string
		expected string
		want     bool
	}{
		{"get_weather", "get_weather", true},
		{"weather", "get_weather", true},
		{"calc", "calculate", true},
		{"search", "find", true},
		{"xyz123", "abc456", false},
	}

	for _, tt := range tests {
		t.Run(tt.actual+"_"+tt.expected, func(t *testing.T) {
			got := e.fuzzyMatchToolName(tt.actual, tt.expected)
			if got != tt.want {
				t.Errorf("fuzzyMatchToolName(%q, %q) = %v, want %v", tt.actual, tt.expected, got, tt.want)
			}
		})
	}
}

func TestEvaluator_ValidateArguments(t *testing.T) {
	e := &Evaluator{}

	tests := []struct {
		name     string
		expected map[string]any
		actual   map[string]any
		want     bool
	}{
		{
			name:     "exact match",
			expected: map[string]any{"location": "Paris"},
			actual:   map[string]any{"location": "Paris"},
			want:     true,
		},
		{
			name:     "case insensitive",
			expected: map[string]any{"location": "paris"},
			actual:   map[string]any{"location": "PARIS"},
			want:     true,
		},
		{
			name:     "numeric tolerance",
			expected: map[string]any{"a": 25.0},
			actual:   map[string]any{"a": 25.0001},
			want:     true,
		},
		{
			name:     "alternative key",
			expected: map[string]any{"location": "Tokyo"},
			actual:   map[string]any{"city": "Tokyo"},
			want:     true,
		},
		{
			name:     "missing key",
			expected: map[string]any{"location": "Paris", "units": "celsius"},
			actual:   map[string]any{"location": "Paris"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.validateArguments(tt.expected, tt.actual)
			if got != tt.want {
				t.Errorf("validateArguments() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateSummary(t *testing.T) {
	e := &Evaluator{}

	results := []BenchmarkResult{
		{
			TestCase:   TestCase{Category: "weather", Difficulty: "easy"},
			Success:    true,
			Latency:    100 * time.Millisecond,
			TokensUsed: 50,
		},
		{
			TestCase:   TestCase{Category: "weather", Difficulty: "medium"},
			Success:    true,
			Latency:    200 * time.Millisecond,
			TokensUsed: 75,
		},
		{
			TestCase:   TestCase{Category: "math", Difficulty: "easy"},
			Success:    false,
			Latency:    150 * time.Millisecond,
			TokensUsed: 60,
		},
	}

	summary := e.calculateSummary(results)

	if summary.TotalTests != 3 {
		t.Errorf("TotalTests = %d, want 3", summary.TotalTests)
	}
	if summary.PassedTests != 2 {
		t.Errorf("PassedTests = %d, want 2", summary.PassedTests)
	}
	if summary.FailedTests != 1 {
		t.Errorf("FailedTests = %d, want 1", summary.FailedTests)
	}
	if summary.TotalTokens != 185 {
		t.Errorf("TotalTokens = %d, want 185", summary.TotalTokens)
	}

	// Check category breakdown
	if summary.CategoryBreakdown["weather"].Total != 2 {
		t.Errorf("weather.Total = %d, want 2", summary.CategoryBreakdown["weather"].Total)
	}
	if summary.CategoryBreakdown["weather"].Passed != 2 {
		t.Errorf("weather.Passed = %d, want 2", summary.CategoryBreakdown["weather"].Passed)
	}
}

func TestCompareModels(t *testing.T) {
	b1 := &ModelBenchmark{
		ModelName: "model-a",
		Summary: BenchmarkSummary{
			SuccessRate:    0.8,
			AverageLatency: 100 * time.Millisecond,
			TotalTokens:    500,
			DifficultyBreakdown: map[string]*DifficultyStats{
				"easy": {SuccessRate: 0.9, AvgLatency: 50 * time.Millisecond},
			},
		},
	}
	b2 := &ModelBenchmark{
		ModelName: "model-b",
		Summary: BenchmarkSummary{
			SuccessRate:    0.9,
			AverageLatency: 150 * time.Millisecond,
			TotalTokens:    600,
			DifficultyBreakdown: map[string]*DifficultyStats{
				"easy": {SuccessRate: 0.95, AvgLatency: 60 * time.Millisecond},
			},
		},
	}

	comparison := CompareModels(b1, b2)

	if len(comparison) != 2 {
		t.Errorf("comparison should have 2 models, got %d", len(comparison))
	}

	modelA := comparison["model-a"].(map[string]any)
	if modelA["success_rate"] != 0.8 {
		t.Errorf("model-a success_rate = %v, want 0.8", modelA["success_rate"])
	}
}

func TestLatencyCalculations(t *testing.T) {
	durations := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
	}

	avg := calculateAverage(durations)
	if avg != 30*time.Millisecond {
		t.Errorf("calculateAverage() = %v, want 30ms", avg)
	}

	median := calculateMedian(durations)
	if median != 30*time.Millisecond {
		t.Errorf("calculateMedian() = %v, want 30ms", median)
	}

	p95 := calculatePercentile(durations, 95)
	if p95 != 50*time.Millisecond {
		t.Errorf("calculatePercentile(95) = %v, want 50ms", p95)
	}
}

func TestGetTestTools(t *testing.T) {
	e := &Evaluator{}
	tools := e.getTestTools()

	if len(tools) == 0 {
		t.Error("getTestTools() should return tools")
	}

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true

		// Verify parameters are valid JSON
		var params map[string]any
		if err := json.Unmarshal(tool.Parameters, &params); err != nil {
			t.Errorf("tool %s has invalid parameters: %v", tool.Name, err)
		}
	}

	expectedTools := []string{"get_weather", "calculate", "search"}
	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("missing expected tool: %s", name)
		}
	}
}

