package llm

import (
	"fmt"
	"reflect"
)

type Validator struct{ Schema map[string]any }

func NewValidator(schema map[string]any) *Validator { return &Validator{Schema: schema} }

func (v *Validator) Validate(input map[string]any) error {
	props, ok := v.Schema["properties"].(map[string]any)
	if !ok {
		return fmt.Errorf("missing 'properties' in schema")
	}

	required, _ := v.Schema["required"].([]any)
	reqMap := make(map[string]bool)
	for _, r := range required {
		if str, ok := r.(string); ok {
			reqMap[str] = true
		}
	}

	for k, val := range input {
		sch, ok := props[k].(map[string]any)
		if !ok {
			return fmt.Errorf("unknown field: %s", k)
		}

		typ, _ := sch["type"].(string)
		if err := checkType(val, typ); err != nil {
			return fmt.Errorf("field %s: %v", k, err)
		}

		// Validate numeric constraints
		if err := validateNumericConstraints(k, val, sch); err != nil {
			return err
		}
	}

	for r := range reqMap {
		if _, ok := input[r]; !ok {
			return fmt.Errorf("missing required field: %s", r)
		}
	}
	return nil
}

func validateNumericConstraints(fieldName string, val any, schema map[string]any) error {
	if val == nil {
		return nil
	}

	valType := reflect.TypeOf(val)
	if valType == nil {
		return nil
	}

	kind := valType.Kind()

	// Handle minimum constraint
	if min, ok := schema["minimum"].(float64); ok {
		switch kind {
		case reflect.Float64:
			if val.(float64) < min {
				return fmt.Errorf("%s below minimum %v", fieldName, min)
			}
		case reflect.Int:
			if float64(val.(int)) < min {
				return fmt.Errorf("%s below minimum %v", fieldName, min)
			}
		}
	}

	// Handle maximum constraint
	if max, ok := schema["maximum"].(float64); ok {
		switch kind {
		case reflect.Float64:
			if val.(float64) > max {
				return fmt.Errorf("%s above maximum %v", fieldName, max)
			}
		case reflect.Int:
			if float64(val.(int)) > max {
				return fmt.Errorf("%s above maximum %v", fieldName, max)
			}
		}
	}

	return nil
}

func checkType(val any, typ string) error {
	if val == nil {
		return fmt.Errorf("value cannot be nil")
	}

	valType := reflect.TypeOf(val)
	if valType == nil {
		return fmt.Errorf("value cannot be nil")
	}

	kind := valType.Kind()

	switch typ {
	case "string":
		if kind != reflect.String {
			return fmt.Errorf("must be string")
		}
	case "number":
		if kind != reflect.Float64 && kind != reflect.Int {
			return fmt.Errorf("must be number")
		}
	case "boolean":
		if kind != reflect.Bool {
			return fmt.Errorf("must be boolean")
		}
	case "object":
		if kind != reflect.Map {
			return fmt.Errorf("must be object")
		}
	case "array":
		if kind != reflect.Slice {
			return fmt.Errorf("must be array")
		}
	}
	return nil
}
