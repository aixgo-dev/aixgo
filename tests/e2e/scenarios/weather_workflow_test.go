package scenarios

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aixgo-dev/aixgo/internal/llm/provider"
	"github.com/aixgo-dev/aixgo/pkg/mcp"
	"github.com/aixgo-dev/aixgo/tests/e2e"
)

// TestWeatherWorkflow_SimpleQuery tests a simple weather query workflow
func TestWeatherWorkflow_SimpleQuery(t *testing.T) {
	env := e2e.NewTestEnvironment(t)
	defer env.Cleanup()

	// Register weather tool
	env.MCPServer().RegisterTool("get_weather", "Get weather for a location", func(ctx context.Context, args mcp.Args) (any, error) {
		location := args.String("location")
		return map[string]any{
			"location":    location,
			"temperature": 22,
			"condition":   "sunny",
			"humidity":    65,
		}, nil
	})

	// Setup provider to call the weather tool
	env.Provider().AddToolCallResponse("get_weather", map[string]any{"location": "Tokyo"})
	env.Provider().AddTextResponse("The weather in Tokyo is sunny with a temperature of 22 degrees Celsius.")

	// Step 1: User asks about weather
	resp1, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "What's the weather like in Tokyo?"},
		},
	})

	e2e.AssertNoError(t, err, "first completion")
	e2e.AssertEqual(t, 1, len(resp1.ToolCalls), "should request tool call")
	e2e.AssertEqual(t, "get_weather", resp1.ToolCalls[0].Function.Name, "tool name")

	// Step 2: Execute the tool
	var toolArgs map[string]any
	if err := json.Unmarshal(resp1.ToolCalls[0].Function.Arguments, &toolArgs); err != nil {
		t.Fatalf("unmarshal tool args: %v", err)
	}

	toolResult, err := env.MCPServer().CallTool(env.Context(), "get_weather", toolArgs)
	e2e.AssertNoError(t, err, "tool execution")

	// Step 3: Get final response
	toolResultJSON, _ := json.Marshal(toolResult)
	resp2, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "What's the weather like in Tokyo?"},
			{Role: "assistant", Content: ""},
			{Role: "tool", Content: string(toolResultJSON)},
		},
	})

	e2e.AssertNoError(t, err, "second completion")
	e2e.AssertContains(t, resp2.Content, "Tokyo", "response mentions location")
	e2e.AssertContains(t, resp2.Content, "sunny", "response mentions condition")
}

// TestWeatherWorkflow_MultipleLocations tests weather queries for multiple locations
func TestWeatherWorkflow_MultipleLocations(t *testing.T) {
	env := e2e.NewTestEnvironment(t)
	defer env.Cleanup()

	weatherData := map[string]map[string]any{
		"Tokyo":    {"temperature": 22, "condition": "sunny"},
		"London":   {"temperature": 15, "condition": "cloudy"},
		"New York": {"temperature": 28, "condition": "humid"},
	}

	env.MCPServer().RegisterTool("get_weather", "Get weather for a location", func(ctx context.Context, args mcp.Args) (any, error) {
		location := args.String("location")
		if data, ok := weatherData[location]; ok {
			return data, nil
		}
		return map[string]any{"error": "location not found"}, nil
	})

	locations := []string{"Tokyo", "London", "New York"}

	for _, loc := range locations {
		t.Run(loc, func(t *testing.T) {
			result, err := env.MCPServer().CallTool(env.Context(), "get_weather", map[string]any{"location": loc})
			e2e.AssertNoError(t, err, "tool call for "+loc)

			resultMap, ok := result.(map[string]any)
			if !ok {
				t.Fatalf("expected map result for %s", loc)
			}

			if _, hasError := resultMap["error"]; hasError {
				t.Errorf("unexpected error for %s", loc)
			}
		})
	}

	// Verify all calls were recorded
	calls := env.MCPServer().GetCalls()
	e2e.AssertEqual(t, len(locations), len(calls), "number of tool calls")
}

// TestWeatherWorkflow_InvalidLocation tests handling of invalid location
func TestWeatherWorkflow_InvalidLocation(t *testing.T) {
	env := e2e.NewTestEnvironment(t)
	defer env.Cleanup()

	env.MCPServer().RegisterTool("get_weather", "Get weather for a location", func(ctx context.Context, args mcp.Args) (any, error) {
		location := args.String("location")
		if location == "" {
			return map[string]any{"error": "location is required"}, nil
		}
		return map[string]any{"temperature": 20, "condition": "unknown"}, nil
	})

	// Test with empty location
	result, err := env.MCPServer().CallTool(env.Context(), "get_weather", map[string]any{"location": ""})
	e2e.AssertNoError(t, err, "tool call should not error")

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}

	if _, hasError := resultMap["error"]; !hasError {
		t.Error("expected error for empty location")
	}
}

// TestWeatherWorkflow_FullConversation tests a full weather conversation
func TestWeatherWorkflow_FullConversation(t *testing.T) {
	env := e2e.NewTestEnvironment(t)
	defer env.Cleanup()

	// Setup conversation flow
	env.Provider().AddTextResponse("I can help you with weather information. Which city are you interested in?")
	env.Provider().AddToolCallResponse("get_weather", map[string]any{"location": "Paris"})
	env.Provider().AddTextResponse("The weather in Paris is currently 18 degrees and partly cloudy. Would you like to know about another city?")
	env.Provider().AddTextResponse("You're welcome! Have a great day!")

	env.MCPServer().RegisterTool("get_weather", "Get weather for a location", func(ctx context.Context, args mcp.Args) (any, error) {
		return map[string]any{"temperature": 18, "condition": "partly cloudy"}, nil
	})

	conversation := []struct {
		user     string
		checkFn  func(resp *provider.CompletionResponse)
	}{
		{
			user: "Can you help me with weather?",
			checkFn: func(resp *provider.CompletionResponse) {
				e2e.AssertContains(t, resp.Content, "weather", "should mention weather")
			},
		},
		{
			user: "What's it like in Paris?",
			checkFn: func(resp *provider.CompletionResponse) {
				e2e.AssertEqual(t, 1, len(resp.ToolCalls), "should call weather tool")
			},
		},
	}

	for i, turn := range conversation {
		resp, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
			Messages: []provider.Message{
				{Role: "user", Content: turn.user},
			},
		})

		e2e.AssertNoError(t, err, "turn "+string(rune('0'+i)))
		turn.checkFn(resp)
	}
}

// TestWeatherWorkflow_SchemaValidation tests weather tool schema validation
func TestWeatherWorkflow_SchemaValidation(t *testing.T) {
	env := e2e.NewTestEnvironment(t)
	defer env.Cleanup()

	// Define tool with schema
	schema := mcp.Schema{
		"location": mcp.SchemaField{
			Type:        "string",
			Required:    true,
			Description: "The city name",
			MinLength:   1,
			MaxLength:   100,
		},
		"units": mcp.SchemaField{
			Type:        "string",
			Required:    false,
			Description: "Temperature units",
			Enum:        []any{"celsius", "fahrenheit"},
		},
	}

	testCases := []struct {
		name    string
		args    mcp.Args
		wantErr bool
	}{
		{
			name:    "valid args",
			args:    mcp.Args{"location": "Tokyo", "units": "celsius"},
			wantErr: false,
		},
		{
			name:    "missing required",
			args:    mcp.Args{"units": "celsius"},
			wantErr: true,
		},
		{
			name:    "invalid enum",
			args:    mcp.Args{"location": "Tokyo", "units": "kelvin"},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := schema.ValidateArgs(tc.args)
			if tc.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
