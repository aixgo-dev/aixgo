package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// TypedTool provides compile-time type safety for MCP tools
type TypedTool[I, O any] struct {
	name        string
	description string
	handler     func(context.Context, I) (O, error)
	schema      Schema
}

// NewTypedTool creates a new type-safe tool with auto-generated schema
func NewTypedTool[I, O any](
	name string,
	description string,
	handler func(context.Context, I) (O, error),
) *TypedTool[I, O] {
	tool := &TypedTool[I, O]{
		name:        name,
		description: description,
		handler:     handler,
	}
	tool.schema = generateSchema[I]()
	return tool
}

// Name returns the tool name
func (t *TypedTool[I, O]) Name() string {
	return t.name
}

// Description returns the tool description
func (t *TypedTool[I, O]) Description() string {
	return t.description
}

// Schema returns the generated JSON schema
func (t *TypedTool[I, O]) Schema() Schema {
	return t.schema
}

// ToTool converts the typed tool to the standard Tool interface
func (t *TypedTool[I, O]) ToTool() Tool {
	return Tool{
		Name:        t.name,
		Description: t.description,
		Schema:      t.schema,
		Handler:     t.createHandler(),
	}
}

// createHandler creates a generic ToolHandler from the typed handler
func (t *TypedTool[I, O]) createHandler() ToolHandler {
	return func(ctx context.Context, args Args) (any, error) {
		var input I

		// Convert args to JSON and then unmarshal into the typed input
		jsonBytes, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal args: %w", err)
		}

		if err := json.Unmarshal(jsonBytes, &input); err != nil {
			return nil, fmt.Errorf("failed to unmarshal args into input type: %w", err)
		}

		// Call the typed handler
		output, err := t.handler(ctx, input)
		if err != nil {
			return nil, err
		}

		return output, nil
	}
}

// RegisterTypedTool registers a TypedTool with the server
func (s *Server) RegisterTypedTool(tool interface{ ToTool() Tool }) error {
	return s.RegisterTool(tool.ToTool())
}

// generateSchema generates a JSON schema from a Go struct type
func generateSchema[T any]() Schema {
	schema := make(Schema)
	var t T
	typ := reflect.TypeOf(t)

	// Handle pointer types
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	// Only process struct types
	if typ.Kind() != reflect.Struct {
		return schema
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get JSON field name
		jsonTag := field.Tag.Get("json")
		fieldName := parseJSONFieldName(jsonTag, field.Name)
		if fieldName == "-" {
			continue
		}

		// Get jsonschema tag for additional metadata
		jsonschemaTag := field.Tag.Get("jsonschema")

		// Create schema field
		schemaField := SchemaField{
			Type:        goTypeToJSONType(field.Type),
			Description: field.Tag.Get("description"),
			Required:    strings.Contains(jsonschemaTag, "required"),
		}

		// Parse additional jsonschema options
		parseJSONSchemaOptions(jsonschemaTag, &schemaField)

		schema[fieldName] = schemaField
	}

	return schema
}

// parseJSONFieldName extracts the field name from a json tag
func parseJSONFieldName(tag string, defaultName string) string {
	if tag == "" {
		return strings.ToLower(defaultName)
	}
	parts := strings.Split(tag, ",")
	if parts[0] == "" {
		return strings.ToLower(defaultName)
	}
	return parts[0]
}

// goTypeToJSONType converts Go types to JSON schema types
func goTypeToJSONType(t reflect.Type) string {
	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map, reflect.Struct:
		return "object"
	default:
		return "string"
	}
}

// parseJSONSchemaOptions parses jsonschema tag options
func parseJSONSchemaOptions(tag string, field *SchemaField) {
	if tag == "" {
		return
	}

	parts := strings.Split(tag, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)

		// Parse key=value pairs
		if strings.Contains(part, "=") {
			kv := strings.SplitN(part, "=", 2)
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])

			switch key {
			case "minLength":
				if n, err := parseIntValue(value); err == nil {
					field.MinLength = n
				}
			case "maxLength":
				if n, err := parseIntValue(value); err == nil {
					field.MaxLength = n
				}
			case "pattern":
				field.Pattern = value
			case "description":
				field.Description = value
			case "minimum":
				if f, err := parseFloatValue(value); err == nil {
					field.Minimum = &f
				}
			case "maximum":
				if f, err := parseFloatValue(value); err == nil {
					field.Maximum = &f
				}
			case "enum":
				// Parse comma-separated enum values
				enumValues := strings.Split(value, "|")
				field.Enum = make([]any, len(enumValues))
				for i, v := range enumValues {
					field.Enum[i] = strings.TrimSpace(v)
				}
			}
		}
	}
}

func parseIntValue(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func parseFloatValue(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
