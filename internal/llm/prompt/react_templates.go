package prompt

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ModelConfig contains model-specific configurations
type ModelConfig struct {
	Name           string
	MaxContextSize int
	Temperature    float32
	TopP           float32
	TopK           int
	RepeatPenalty  float32
	StopSequences  []string
	SystemPrompt   string
}

// ReActTemplate defines a ReAct prompt template
type ReActTemplate struct {
	ModelName       string
	SystemPrompt    string
	FewShotExamples []FewShotExample
	OutputFormat    OutputFormat
}

// FewShotExample represents a few-shot learning example
type FewShotExample struct {
	Query       string
	Thought     string
	Action      string
	ActionInput map[string]any
	Observation string
	FinalAnswer string
}

// OutputFormat defines the expected output structure
type OutputFormat struct {
	ThoughtPrefix     string
	ActionPrefix      string
	ActionInputPrefix string
	ObservationPrefix string
	FinalAnswerPrefix string
	JSONDelimiters    bool
	StrictFormatting  bool
}

// GetModelConfig returns optimized configuration for specific models
func GetModelConfig(modelName string) *ModelConfig {
	configs := map[string]*ModelConfig{
		// Phi-3.5 Mini optimized configuration
		"phi3.5:3.8b-mini-instruct": {
			Name:           "phi3.5:3.8b-mini-instruct",
			MaxContextSize: 4096,
			Temperature:    0.3, // Lower for more deterministic tool calling
			TopP:           0.9,
			TopK:           40,
			RepeatPenalty:  1.1,
			StopSequences: []string{
				"Observation:",
				"User:",
				"<|end|>",
				"<|endoftext|>",
			},
			SystemPrompt: "You are a precise AI assistant that follows instructions exactly. Always use the specified format for your responses.",
		},
		// Gemma 2B optimized configuration
		"gemma:2b-instruct": {
			Name:           "gemma:2b-instruct",
			MaxContextSize: 8192,
			Temperature:    0.4, // Slightly higher for Gemma
			TopP:           0.95,
			TopK:           50,
			RepeatPenalty:  1.15,
			StopSequences: []string{
				"Observation:",
				"User:",
				"<end_of_turn>",
			},
			SystemPrompt: "You are a helpful assistant that uses tools to answer questions accurately.",
		},
		// Default configuration
		"default": {
			Name:           "default",
			MaxContextSize: 4096,
			Temperature:    0.5,
			TopP:           0.9,
			TopK:           40,
			RepeatPenalty:  1.1,
			StopSequences: []string{
				"Observation:",
				"User:",
			},
			SystemPrompt: "You are an AI assistant with tool-calling capabilities.",
		},
	}

	if config, exists := configs[modelName]; exists {
		return config
	}

	// Check for partial matches (e.g., "phi3.5" in "phi3.5:3.8b-mini-instruct-q4_K_M")
	for key, config := range configs {
		if strings.Contains(strings.ToLower(modelName), strings.Split(key, ":")[0]) {
			return config
		}
	}

	return configs["default"]
}

// GetReActTemplate returns an optimized ReAct template for the model
func GetReActTemplate(modelName string) *ReActTemplate {
	config := GetModelConfig(modelName)

	// Common few-shot examples that work well across models
	commonExamples := []FewShotExample{
		{
			Query:   "What's the weather in San Francisco?",
			Thought: "I need to check the current weather conditions in San Francisco using the weather tool.",
			Action:  "get_weather",
			ActionInput: map[string]any{
				"location": "San Francisco",
				"units":    "fahrenheit",
			},
			Observation: "Temperature: 68°F, Conditions: Partly cloudy, Humidity: 65%",
			FinalAnswer: "The current weather in San Francisco is 68°F with partly cloudy conditions and 65% humidity.",
		},
		{
			Query:   "Calculate the sum of 145 and 378",
			Thought: "I need to perform a mathematical calculation. Let me add these numbers.",
			Action:  "calculate",
			ActionInput: map[string]any{
				"operation": "add",
				"a":         145,
				"b":         378,
			},
			Observation: "523",
			FinalAnswer: "The sum of 145 and 378 is 523.",
		},
	}

	// Model-specific templates
	if strings.Contains(modelName, "phi3.5") {
		return &ReActTemplate{
			ModelName:       modelName,
			SystemPrompt:    config.SystemPrompt,
			FewShotExamples: commonExamples,
			OutputFormat: OutputFormat{
				ThoughtPrefix:     "Thought: ",
				ActionPrefix:      "Action: ",
				ActionInputPrefix: "Action Input: ",
				ObservationPrefix: "Observation: ",
				FinalAnswerPrefix: "Final Answer: ",
				JSONDelimiters:    true,
				StrictFormatting:  true,
			},
		}
	} else if strings.Contains(modelName, "gemma") {
		return &ReActTemplate{
			ModelName:       modelName,
			SystemPrompt:    config.SystemPrompt,
			FewShotExamples: commonExamples[:1], // Gemma works better with fewer examples
			OutputFormat: OutputFormat{
				ThoughtPrefix:     "Thought: ",
				ActionPrefix:      "Action: ",
				ActionInputPrefix: "Input: ",
				ObservationPrefix: "Observation: ",
				FinalAnswerPrefix: "Answer: ",
				JSONDelimiters:    false, // Gemma struggles with inline JSON
				StrictFormatting:  false,
			},
		}
	}

	// Default template
	return &ReActTemplate{
		ModelName:       modelName,
		SystemPrompt:    config.SystemPrompt,
		FewShotExamples: commonExamples,
		OutputFormat: OutputFormat{
			ThoughtPrefix:     "Thought: ",
			ActionPrefix:      "Action: ",
			ActionInputPrefix: "Action Input: ",
			ObservationPrefix: "Observation: ",
			FinalAnswerPrefix: "Final Answer: ",
			JSONDelimiters:    true,
			StrictFormatting:  false,
		},
	}
}

// BuildPrompt constructs the full ReAct prompt with tools
func (t *ReActTemplate) BuildPrompt(tools []Tool, messages []Message) string {
	var sb strings.Builder

	// System section with enhanced instructions
	sb.WriteString(t.SystemPrompt)
	sb.WriteString("\n\n")

	// Tools section with structured descriptions
	sb.WriteString("## Available Tools\n\n")
	for _, tool := range tools {
		sb.WriteString(fmt.Sprintf("### %s\n", tool.Name))
		sb.WriteString(fmt.Sprintf("Description: %s\n", tool.Description))
		if len(tool.Schema) > 0 {
			schemaJSON, err := json.MarshalIndent(tool.Schema, "", "  ")
			if err == nil {
				sb.WriteString(fmt.Sprintf("Parameters:\n```json\n%s\n```\n", string(schemaJSON)))
			}
		}
		sb.WriteString("\n")
	}

	// Format instructions with examples
	sb.WriteString("## Response Format\n\n")
	sb.WriteString("You must follow this exact format for each step:\n\n")
	sb.WriteString(fmt.Sprintf("%s[Your reasoning about what to do next]\n", t.OutputFormat.ThoughtPrefix))
	sb.WriteString(fmt.Sprintf("%s[tool_name]\n", t.OutputFormat.ActionPrefix))

	if t.OutputFormat.JSONDelimiters {
		sb.WriteString(fmt.Sprintf("%s```json\n{\"param1\": \"value1\", \"param2\": \"value2\"}\n```\n", t.OutputFormat.ActionInputPrefix))
	} else {
		sb.WriteString(fmt.Sprintf("%sparam1=value1, param2=value2\n", t.OutputFormat.ActionInputPrefix))
	}

	sb.WriteString(fmt.Sprintf("%s[Tool output will appear here]\n", t.OutputFormat.ObservationPrefix))
	sb.WriteString("... (repeat as needed)\n")
	sb.WriteString(fmt.Sprintf("%s[Your final response to the user]\n\n", t.OutputFormat.FinalAnswerPrefix))

	// Few-shot examples
	if len(t.FewShotExamples) > 0 {
		sb.WriteString("## Examples\n\n")
		for i, example := range t.FewShotExamples {
			sb.WriteString(fmt.Sprintf("### Example %d\n", i+1))
			sb.WriteString(fmt.Sprintf("User: %s\n\n", example.Query))
			sb.WriteString(fmt.Sprintf("%s%s\n", t.OutputFormat.ThoughtPrefix, example.Thought))
			sb.WriteString(fmt.Sprintf("%s%s\n", t.OutputFormat.ActionPrefix, example.Action))

			if t.OutputFormat.JSONDelimiters {
				inputJSON, err := json.MarshalIndent(example.ActionInput, "", "  ")
				if err == nil {
					sb.WriteString(fmt.Sprintf("%s```json\n%s\n```\n", t.OutputFormat.ActionInputPrefix, string(inputJSON)))
				}
			} else {
				// Simple key=value format for models that struggle with JSON
				// Sort keys for deterministic output
				keys := make([]string, 0, len(example.ActionInput))
				for k := range example.ActionInput {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				var params []string
				for _, k := range keys {
					params = append(params, fmt.Sprintf("%s=%v", k, example.ActionInput[k]))
				}
				sb.WriteString(fmt.Sprintf("%s%s\n", t.OutputFormat.ActionInputPrefix, strings.Join(params, ", ")))
			}

			sb.WriteString(fmt.Sprintf("%s%s\n", t.OutputFormat.ObservationPrefix, example.Observation))
			sb.WriteString(fmt.Sprintf("%s%s\n\n", t.OutputFormat.FinalAnswerPrefix, example.FinalAnswer))
		}
	}

	// Conversation history
	sb.WriteString("## Conversation\n\n")
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			sb.WriteString(fmt.Sprintf("User: %s\n\n", msg.Content))
		case "assistant":
			sb.WriteString(fmt.Sprintf("Assistant: %s\n\n", msg.Content))
		}
	}

	// Prompt for response
	sb.WriteString("Now, let's solve this step by step.\n\n")
	sb.WriteString(t.OutputFormat.ThoughtPrefix)

	return sb.String()
}

// Tool represents a tool schema
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema,omitempty"`
}

// Message represents a conversation message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
