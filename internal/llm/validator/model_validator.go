package validator

import (
	"fmt"
	"reflect"
)

// ModelValidator is an interface for types that can validate themselves
// This allows for cross-field validation logic
type ModelValidator interface {
	ValidateModel() error
}

// ValidateModel runs model-level validation on a struct
// This is called after all field validations have passed
func ValidateModel(v any) error {
	if validator, ok := v.(ModelValidator); ok {
		return validator.ValidateModel()
	}
	return nil
}

// WithModelValidator wraps a validator function for use in validation
type WithModelValidator struct {
	Validator func(any) error
}

// BeforeValidatorFunc creates a before-validator from a function
func BeforeValidatorFunc(fn func(*ValidationContext, any) (any, error)) FieldValidatorConfig {
	return FieldValidatorConfig{
		Mode:      BeforeValidator,
		Validator: fn,
	}
}

// AfterValidatorFunc creates an after-validator from a function
func AfterValidatorFunc(fn func(*ValidationContext, any) (any, error)) FieldValidatorConfig {
	return FieldValidatorConfig{
		Mode:      AfterValidator,
		Validator: fn,
	}
}

// WrapValidatorFunc creates a wrap-validator from a function
func WrapValidatorFunc(fn func(*ValidationContext, any, ValidatorFunc) (any, error)) FieldValidatorConfig {
	return FieldValidatorConfig{
		Mode: WrapValidator,
		Validator: func(ctx *ValidationContext, value any) (any, error) {
			// Create a default handler that just returns the value
			handler := func(ctx *ValidationContext, v any) (any, error) {
				return v, nil
			}
			return fn(ctx, value, handler)
		},
	}
}

// Custom validator functions for common patterns

// ValidateFieldMatch validates that two fields match (e.g., password confirmation)
func ValidateFieldMatch(field1, field2 string) func(any) error {
	return func(v any) error {
		val := reflect.ValueOf(v)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}

		if val.Kind() != reflect.Struct {
			return fmt.Errorf("ValidateFieldMatch requires a struct")
		}

		f1 := val.FieldByName(field1)
		f2 := val.FieldByName(field2)

		if !f1.IsValid() || !f2.IsValid() {
			return fmt.Errorf("fields %s or %s not found", field1, field2)
		}

		if f1.Interface() != f2.Interface() {
			return fmt.Errorf("%s and %s must match", field1, field2)
		}

		return nil
	}
}

// ValidateDateRange validates that start date is before end date
func ValidateDateRange(startField, endField string, message string) func(any) error {
	return func(v any) error {
		val := reflect.ValueOf(v)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}

		if val.Kind() != reflect.Struct {
			return fmt.Errorf("ValidateDateRange requires a struct")
		}

		start := val.FieldByName(startField)
		end := val.FieldByName(endField)

		if !start.IsValid() || !end.IsValid() {
			return fmt.Errorf("fields %s or %s not found", startField, endField)
		}

		// Use reflection to compare - assuming comparable types
		if start.Kind() == reflect.Struct && end.Kind() == reflect.Struct {
			// Try to call After/Before methods if available
			afterMethod := start.MethodByName("After")
			if afterMethod.IsValid() {
				results := afterMethod.Call([]reflect.Value{end})
				if len(results) > 0 && results[0].Kind() == reflect.Bool {
					if results[0].Bool() {
						if message == "" {
							message = fmt.Sprintf("%s must be before %s", startField, endField)
						}
						return fmt.Errorf("%s", message)
					}
				}
			}
		}

		return nil
	}
}

// ValidateConditionalRequired validates that a field is required based on another field's value
func ValidateConditionalRequired(conditionField string, conditionValue any, requiredField string) func(any) error {
	return func(v any) error {
		val := reflect.ValueOf(v)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}

		if val.Kind() != reflect.Struct {
			return fmt.Errorf("ValidateConditionalRequired requires a struct")
		}

		condition := val.FieldByName(conditionField)
		required := val.FieldByName(requiredField)

		if !condition.IsValid() || !required.IsValid() {
			return fmt.Errorf("fields not found")
		}

		// Check if condition matches
		if condition.Interface() == conditionValue {
			// Check if required field is zero/empty
			if required.IsZero() {
				return fmt.Errorf("%s is required when %s is %v", requiredField, conditionField, conditionValue)
			}
		}

		return nil
	}
}

// ValidateAtLeastOne validates that at least one of the specified fields is non-zero
func ValidateAtLeastOne(fields ...string) func(any) error {
	return func(v any) error {
		val := reflect.ValueOf(v)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}

		if val.Kind() != reflect.Struct {
			return fmt.Errorf("ValidateAtLeastOne requires a struct")
		}

		for _, fieldName := range fields {
			field := val.FieldByName(fieldName)
			if field.IsValid() && !field.IsZero() {
				return nil // Found at least one non-zero field
			}
		}

		return fmt.Errorf("at least one of [%v] must be provided", fields)
	}
}

// ValidateMutuallyExclusive validates that only one of the specified fields is set
func ValidateMutuallyExclusive(fields ...string) func(any) error {
	return func(v any) error {
		val := reflect.ValueOf(v)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}

		if val.Kind() != reflect.Struct {
			return fmt.Errorf("ValidateMutuallyExclusive requires a struct")
		}

		setCount := 0
		var setField string

		for _, fieldName := range fields {
			field := val.FieldByName(fieldName)
			if field.IsValid() && !field.IsZero() {
				setCount++
				if setField == "" {
					setField = fieldName
				}
			}
		}

		if setCount > 1 {
			return fmt.Errorf("only one of [%v] can be set", fields)
		}

		return nil
	}
}
