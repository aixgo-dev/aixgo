package parser

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ToolCall represents a parsed tool call
type ToolCall struct {
	Thought     string         `json:"thought"`
	Action      string         `json:"action"`
	ActionInput map[string]any `json:"action_input"`
}

// ParseResult represents the result of parsing LLM output
type ParseResult struct {
	ToolCall    *ToolCall `json:"tool_call,omitempty"`
	FinalAnswer string    `json:"final_answer,omitempty"`
	RawText     string    `json:"raw_text"`
	Confidence  float64   `json:"confidence"`
}

// ReActParser handles parsing of ReAct-style outputs with multiple fallback strategies
type ReActParser struct {
	strictMode    bool
	modelSpecific string
}

// NewReActParser creates a new parser with the specified configuration
func NewReActParser(modelName string, strict bool) *ReActParser {
	return &ReActParser{
		strictMode:    strict,
		modelSpecific: modelName,
	}
}

// Parse attempts to parse the LLM output using multiple strategies
func (p *ReActParser) Parse(text string) (*ParseResult, error) {
	result := &ParseResult{
		RawText:    text,
		Confidence: 0.0,
	}

	// Strategy 1: Check for final answer first
	if answer := p.extractFinalAnswer(text); answer != "" {
		result.FinalAnswer = answer
		result.Confidence = 0.95
		return result, nil
	}

	// Strategy 2: Try structured parsing (highest confidence)
	if toolCall := p.parseStructured(text); toolCall != nil {
		result.ToolCall = toolCall
		result.Confidence = 1.0
		return result, nil
	}

	// Strategy 3: Try regex-based parsing
	if toolCall := p.parseWithRegex(text); toolCall != nil {
		result.ToolCall = toolCall
		result.Confidence = 0.8
		return result, nil
	}

	// Strategy 4: Try fuzzy parsing
	if !p.strictMode {
		if toolCall := p.parseFuzzy(text); toolCall != nil {
			result.ToolCall = toolCall
			result.Confidence = 0.6
			return result, nil
		}
	}

	// Strategy 5: LLM-specific parsing
	if toolCall := p.parseModelSpecific(text); toolCall != nil {
		result.ToolCall = toolCall
		result.Confidence = 0.7
		return result, nil
	}

	// If nothing matched, return the text as a final answer
	result.FinalAnswer = p.cleanupText(text)
	result.Confidence = 0.3
	return result, nil
}

// parseStructured attempts to parse well-formatted ReAct output
func (p *ReActParser) parseStructured(text string) *ToolCall {
	// Look for standard ReAct format
	thoughtRe := regexp.MustCompile(`(?i)Thought:\s*(.+?)(?:\n|$)`)
	actionRe := regexp.MustCompile(`(?i)Action:\s*(\w+)`)

	// Multiple patterns for action input
	inputPatterns := []struct {
		re        *regexp.Regexp
		extractor func(string) map[string]any
	}{
		// JSON in code blocks
		{
			re:        regexp.MustCompile(`(?i)Action Input:\s*` + "```" + `(?:json)?\s*(\{[\s\S]*?\})` + "```"),
			extractor: p.parseJSON,
		},
		// Inline JSON
		{
			re:        regexp.MustCompile(`(?i)Action Input:\s*(\{[^\n]*\})`),
			extractor: p.parseJSON,
		},
		// Key-value pairs
		{
			re:        regexp.MustCompile(`(?i)(?:Action )?Input:\s*([^\n]+)`),
			extractor: p.parseKeyValue,
		},
	}

	thoughtMatch := thoughtRe.FindStringSubmatch(text)
	actionMatch := actionRe.FindStringSubmatch(text)

	if actionMatch == nil {
		return nil
	}

	toolCall := &ToolCall{
		Action: actionMatch[1],
	}

	if thoughtMatch != nil {
		toolCall.Thought = strings.TrimSpace(thoughtMatch[1])
	}

	// Try each input pattern
	for _, pattern := range inputPatterns {
		if match := pattern.re.FindStringSubmatch(text); match != nil {
			if args := pattern.extractor(match[1]); args != nil {
				toolCall.ActionInput = args
				return toolCall
			}
		}
	}

	// If no input found, return with empty input
	toolCall.ActionInput = make(map[string]any)
	return toolCall
}

// parseWithRegex uses regular expressions for more flexible parsing
func (p *ReActParser) parseWithRegex(text string) *ToolCall {
	// More flexible patterns
	patterns := []struct {
		thought string
		action  string
		input   string
	}{
		// Standard format
		{
			thought: `(?i)(?:Thought|Thinking|Reasoning):\s*(.+?)(?:\n|$)`,
			action:  `(?i)(?:Action|Tool|Function):\s*(\w+)`,
			input:   `(?i)(?:Action Input|Input|Parameters|Args):\s*(.+?)(?:\n|$)`,
		},
		// Markdown format
		{
			thought: `(?i)\*\*Thought\*\*:\s*(.+?)(?:\n|$)`,
			action:  `(?i)\*\*Action\*\*:\s*(\w+)`,
			input:   `(?i)\*\*Input\*\*:\s*(.+?)(?:\n|$)`,
		},
		// List format
		{
			thought: `(?i)[-*]\s*Thought:\s*(.+?)(?:\n|$)`,
			action:  `(?i)[-*]\s*Action:\s*(\w+)`,
			input:   `(?i)[-*]\s*Input:\s*(.+?)(?:\n|$)`,
		},
	}

	for _, pattern := range patterns {
		actionRe := regexp.MustCompile(pattern.action)
		actionMatch := actionRe.FindStringSubmatch(text)

		if actionMatch != nil {
			toolCall := &ToolCall{
				Action:      actionMatch[1],
				ActionInput: make(map[string]any),
			}

			// Extract thought if present
			if pattern.thought != "" {
				thoughtRe := regexp.MustCompile(pattern.thought)
				if thoughtMatch := thoughtRe.FindStringSubmatch(text); thoughtMatch != nil {
					toolCall.Thought = strings.TrimSpace(thoughtMatch[1])
				}
			}

			// Extract input if present
			if pattern.input != "" {
				inputRe := regexp.MustCompile(pattern.input)
				if inputMatch := inputRe.FindStringSubmatch(text); inputMatch != nil {
					// Try to parse as JSON first
					if args := p.parseJSON(inputMatch[1]); args != nil {
						toolCall.ActionInput = args
					} else {
						// Fallback to key-value parsing
						toolCall.ActionInput = p.parseKeyValue(inputMatch[1])
					}
				}
			}

			return toolCall
		}
	}

	return nil
}

// parseFuzzy attempts to extract tool calls from poorly formatted text
func (p *ReActParser) parseFuzzy(text string) *ToolCall {
	// Look for function-like patterns
	functionPattern := regexp.MustCompile(`(\w+)\s*\(\s*([^)]*)\s*\)`)
	if match := functionPattern.FindStringSubmatch(text); match != nil {
		toolCall := &ToolCall{
			Action:      match[1],
			ActionInput: make(map[string]any),
		}

		// Parse arguments
		args := strings.Split(match[2], ",")
		for i, arg := range args {
			arg = strings.TrimSpace(arg)
			if strings.Contains(arg, "=") {
				parts := strings.SplitN(arg, "=", 2)
				key := strings.TrimSpace(parts[0])
				value := p.parseValue(strings.TrimSpace(parts[1]))
				toolCall.ActionInput[key] = value
			} else if arg != "" {
				// Positional argument
				toolCall.ActionInput[fmt.Sprintf("arg%d", i)] = p.parseValue(arg)
			}
		}

		return toolCall
	}

	// Look for intent-based patterns
	intentPatterns := map[string]string{
		"get_weather": `(?i)(?:weather|temperature|forecast).*(?:in|at|for)\s+([A-Za-z\s]+)`,
		"calculate":   `(?i)(?:calculate|compute|add|subtract|multiply|divide)`,
		"search":      `(?i)(?:search|find|look up|query).*(?:for|about)\s+(.+)`,
	}

	for action, pattern := range intentPatterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindStringSubmatch(text); match != nil {
			toolCall := &ToolCall{
				Action:      action,
				ActionInput: make(map[string]any),
			}

			if len(match) > 1 {
				// Extract captured groups as parameters
				toolCall.ActionInput["query"] = strings.TrimSpace(match[1])
			}

			return toolCall
		}
	}

	return nil
}

// parseModelSpecific handles model-specific parsing quirks
func (p *ReActParser) parseModelSpecific(text string) *ToolCall {
	switch {
	case strings.Contains(p.modelSpecific, "phi"):
		// Phi models sometimes use different formatting
		re := regexp.MustCompile(`(?i)I (?:need to|will|should) (?:use|call) (?:the )?(\w+)(?: tool| function)?`)
		if match := re.FindStringSubmatch(text); match != nil {
			return &ToolCall{
				Action:      match[1],
				ActionInput: p.extractParametersFromContext(text),
			}
		}

	case strings.Contains(p.modelSpecific, "gemma"):
		// Gemma might use simpler formatting
		re := regexp.MustCompile(`(?i)^(\w+):\s*(.+)$`)
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			if match := re.FindStringSubmatch(line); match != nil {
				// Check if it looks like a tool name
				if p.looksLikeToolName(match[1]) {
					return &ToolCall{
						Action:      match[1],
						ActionInput: p.parseKeyValue(match[2]),
					}
				}
			}
		}
	}

	return nil
}

// Helper functions

func (p *ReActParser) parseJSON(input string) map[string]any {
	input = strings.TrimSpace(input)

	// Clean up common JSON issues
	input = p.fixCommonJSONErrors(input)

	var result map[string]any
	if err := json.Unmarshal([]byte(input), &result); err == nil {
		return result
	}

	// Try to fix and parse again
	fixed := p.aggressiveJSONFix(input)
	if err := json.Unmarshal([]byte(fixed), &result); err == nil {
		return result
	}

	return nil
}

func (p *ReActParser) parseKeyValue(input string) map[string]any {
	result := make(map[string]any)

	// Try comma-separated key=value pairs
	pairs := strings.Split(input, ",")
	for _, pair := range pairs {
		if strings.Contains(pair, "=") {
			parts := strings.SplitN(pair, "=", 2)
			key := strings.TrimSpace(parts[0])
			value := p.parseValue(strings.TrimSpace(parts[1]))
			result[key] = value
		} else if strings.Contains(pair, ":") {
			// Also try colon separator
			parts := strings.SplitN(pair, ":", 2)
			key := strings.TrimSpace(parts[0])
			value := p.parseValue(strings.TrimSpace(parts[1]))
			result[key] = value
		}
	}

	// If no pairs found, treat entire input as single parameter
	if len(result) == 0 && input != "" {
		result["value"] = p.parseValue(input)
	}

	return result
}

func (p *ReActParser) parseValue(value string) any {
	// Remove quotes
	value = strings.Trim(value, `"'`)

	// Try to parse as number
	if num, err := strconv.ParseFloat(value, 64); err == nil {
		return num
	}

	// Try to parse as boolean
	lower := strings.ToLower(value)
	if lower == "true" {
		return true
	}
	if lower == "false" {
		return false
	}

	// Return as string
	return value
}

func (p *ReActParser) fixCommonJSONErrors(s string) string {
	// Remove comments
	s = regexp.MustCompile(`/\*.*?\*/`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`//.*?\n`).ReplaceAllString(s, "")

	// Fix quotes
	s = p.fixQuotes(s)

	// Remove trailing commas
	s = regexp.MustCompile(`,\s*}`).ReplaceAllString(s, "}")
	s = regexp.MustCompile(`,\s*]`).ReplaceAllString(s, "]")

	// Add missing quotes to keys
	s = regexp.MustCompile(`(\w+):`).ReplaceAllString(s, `"$1":`)

	// Fix double quotes that might have been added
	s = strings.ReplaceAll(s, `""`, `"`)

	return s
}

func (p *ReActParser) aggressiveJSONFix(s string) string {
	// More aggressive fixes for badly formatted JSON

	// Extract just the JSON-like part
	if start := strings.Index(s, "{"); start != -1 {
		if end := strings.LastIndex(s, "}"); end != -1 && end > start {
			s = s[start : end+1]
		}
	}

	// Rebuild as valid JSON
	pairs := regexp.MustCompile(`["']?(\w+)["']?\s*[:=]\s*["']?([^,}]+)["']?`).FindAllStringSubmatch(s, -1)
	if len(pairs) > 0 {
		var jsonPairs []string
		for _, pair := range pairs {
			if len(pair) >= 3 {
				key := pair[1]
				value := strings.TrimSpace(pair[2])
				value = strings.Trim(value, `"'}`)

				// Quote the value if it's not a number or boolean
				if _, err := strconv.ParseFloat(value, 64); err != nil {
					if value != "true" && value != "false" && value != "null" {
						value = fmt.Sprintf(`"%s"`, value)
					}
				}

				jsonPairs = append(jsonPairs, fmt.Sprintf(`"%s": %s`, key, value))
			}
		}
		return "{" + strings.Join(jsonPairs, ", ") + "}"
	}

	return s
}

func (p *ReActParser) fixQuotes(s string) string {
	// Convert single quotes to double quotes, but preserve escaped quotes
	inString := false
	escapeNext := false
	var result strings.Builder

	for i, ch := range s {
		if escapeNext {
			result.WriteRune(ch)
			escapeNext = false
			continue
		}

		if ch == '\\' {
			escapeNext = true
			result.WriteRune(ch)
			continue
		}

		if ch == '\'' && !inString {
			// Check if this looks like a string boundary
			if i == 0 || s[i-1] == ':' || s[i-1] == ',' || s[i-1] == '{' || s[i-1] == '[' || s[i-1] == ' ' {
				result.WriteRune('"')
				inString = true
			} else {
				result.WriteRune(ch)
			}
		} else if ch == '\'' && inString {
			result.WriteRune('"')
			inString = false
		} else {
			result.WriteRune(ch)
		}
	}

	return result.String()
}

func (p *ReActParser) extractFinalAnswer(text string) string {
	patterns := []string{
		`(?i)Final Answer:\s*(.+?)(?:\n|$)`,
		`(?i)Answer:\s*(.+?)(?:\n|$)`,
		`(?i)Result:\s*(.+?)(?:\n|$)`,
		`(?i)Therefore,?\s*(.+?)(?:\n|$)`,
		`(?i)In conclusion,?\s*(.+?)(?:\n|$)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindStringSubmatch(text); match != nil {
			return strings.TrimSpace(match[1])
		}
	}

	return ""
}

func (p *ReActParser) extractParametersFromContext(text string) map[string]any {
	params := make(map[string]any)

	// Look for quoted strings that might be parameters
	quotedRe := regexp.MustCompile(`["']([^"']+)["']`)
	matches := quotedRe.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		params["query"] = matches[0][1]
	}

	// Look for numbers
	numberRe := regexp.MustCompile(`\b(\d+(?:\.\d+)?)\b`)
	numbers := numberRe.FindAllStringSubmatch(text, -1)
	for i, match := range numbers {
		if num, err := strconv.ParseFloat(match[1], 64); err == nil {
			params[fmt.Sprintf("number%d", i)] = num
		}
	}

	return params
}

func (p *ReActParser) looksLikeToolName(s string) bool {
	// Check if string looks like a tool/function name
	toolNameRe := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	s = strings.ToLower(s)
	return toolNameRe.MatchString(s) && len(s) > 2 && len(s) < 50
}

func (p *ReActParser) cleanupText(text string) string {
	// Remove formatting artifacts
	text = strings.TrimSpace(text)
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	// Remove incomplete ReAct patterns
	text = regexp.MustCompile(`(?i)^Thought:\s*`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?i)^Action:\s*$`).ReplaceAllString(text, "")

	return text
}
