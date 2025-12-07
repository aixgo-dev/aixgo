package mcp

import (
	"context"
	"testing"
)

type WeatherInput struct {
	City    string `json:"city" jsonschema:"required"`
	Country string `json:"country" description:"Country code"`
}

type WeatherOutput struct {
	Temperature float64 `json:"temperature"`
	Humidity    int     `json:"humidity"`
	Condition   string  `json:"condition"`
}

func TestNewTypedTool(t *testing.T) {
	handler := func(ctx context.Context, in WeatherInput) (WeatherOutput, error) {
		return WeatherOutput{
			Temperature: 72.0,
			Humidity:    50,
			Condition:   "sunny",
		}, nil
	}

	tool := NewTypedTool("get_weather", "Get weather for a city", handler)

	if tool.Name() != "get_weather" {
		t.Errorf("expected name 'get_weather', got '%s'", tool.Name())
	}

	if tool.Description() != "Get weather for a city" {
		t.Errorf("expected description 'Get weather for a city', got '%s'", tool.Description())
	}

	schema := tool.Schema()
	if len(schema) != 2 {
		t.Errorf("expected 2 schema fields, got %d", len(schema))
	}

	cityField, ok := schema["city"]
	if !ok {
		t.Error("expected 'city' field in schema")
	}
	if cityField.Type != "string" {
		t.Errorf("expected city type 'string', got '%s'", cityField.Type)
	}
	if !cityField.Required {
		t.Error("expected city field to be required")
	}
}

func TestTypedToolExecution(t *testing.T) {
	handler := func(ctx context.Context, in WeatherInput) (WeatherOutput, error) {
		return WeatherOutput{
			Temperature: 72.0,
			Humidity:    50,
			Condition:   "sunny for " + in.City,
		}, nil
	}

	typedTool := NewTypedTool("get_weather", "Get weather", handler)
	tool := typedTool.ToTool()

	ctx := context.Background()
	args := Args{"city": "London", "country": "UK"}

	result, err := tool.Handler(ctx, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output, ok := result.(WeatherOutput)
	if !ok {
		t.Fatalf("expected WeatherOutput, got %T", result)
	}

	if output.Temperature != 72.0 {
		t.Errorf("expected temperature 72.0, got %f", output.Temperature)
	}
	if output.Condition != "sunny for London" {
		t.Errorf("expected condition 'sunny for London', got '%s'", output.Condition)
	}
}

func TestServerRegisterTypedTool(t *testing.T) {
	server := NewServer("test")

	handler := func(ctx context.Context, in WeatherInput) (WeatherOutput, error) {
		return WeatherOutput{Temperature: 72.0}, nil
	}

	typedTool := NewTypedTool("get_weather", "Get weather", handler)

	err := server.RegisterTypedTool(typedTool)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tools := server.ListTools()
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}

	if tools[0].Name != "get_weather" {
		t.Errorf("expected tool name 'get_weather', got '%s'", tools[0].Name)
	}
}

func TestGenerateSchema(t *testing.T) {
	type TestInput struct {
		Name     string            `json:"name" jsonschema:"required,minLength=1,maxLength=100"`
		Age      int               `json:"age" jsonschema:"minimum=0,maximum=150"`
		Score    float64           `json:"score"`
		Active   bool              `json:"active"`
		Tags     []string          `json:"tags"`
		Metadata map[string]string `json:"metadata"`
	}

	schema := generateSchema[TestInput]()

	// Check name field
	nameField := schema["name"]
	if nameField.Type != "string" {
		t.Errorf("expected name type 'string', got '%s'", nameField.Type)
	}
	if !nameField.Required {
		t.Error("expected name to be required")
	}
	if nameField.MinLength != 1 {
		t.Errorf("expected minLength 1, got %d", nameField.MinLength)
	}
	if nameField.MaxLength != 100 {
		t.Errorf("expected maxLength 100, got %d", nameField.MaxLength)
	}

	// Check age field
	ageField := schema["age"]
	if ageField.Type != "integer" {
		t.Errorf("expected age type 'integer', got '%s'", ageField.Type)
	}
	if ageField.Minimum == nil || *ageField.Minimum != 0 {
		t.Error("expected minimum 0")
	}
	if ageField.Maximum == nil || *ageField.Maximum != 150 {
		t.Error("expected maximum 150")
	}

	// Check score field
	scoreField := schema["score"]
	if scoreField.Type != "number" {
		t.Errorf("expected score type 'number', got '%s'", scoreField.Type)
	}

	// Check active field
	activeField := schema["active"]
	if activeField.Type != "boolean" {
		t.Errorf("expected active type 'boolean', got '%s'", activeField.Type)
	}

	// Check tags field
	tagsField := schema["tags"]
	if tagsField.Type != "array" {
		t.Errorf("expected tags type 'array', got '%s'", tagsField.Type)
	}

	// Check metadata field
	metadataField := schema["metadata"]
	if metadataField.Type != "object" {
		t.Errorf("expected metadata type 'object', got '%s'", metadataField.Type)
	}
}

func TestTypedToolWithError(t *testing.T) {
	handler := func(ctx context.Context, in WeatherInput) (WeatherOutput, error) {
		return WeatherOutput{}, context.Canceled
	}

	typedTool := NewTypedTool("get_weather", "Get weather", handler)
	tool := typedTool.ToTool()

	ctx := context.Background()
	args := Args{"city": "London"}

	_, err := tool.Handler(ctx, args)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}
