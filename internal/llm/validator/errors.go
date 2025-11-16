package validator

import (
	"fmt"
	"strings"
)

// ValidationError represents a single validation error
type ValidationError struct {
	Field      []string // Field path, e.g., ["user", "address", "zip"]
	Message    string   // Human-readable error message
	Type       string   // Machine-readable error type (e.g., "min_length", "email")
	Value      any      // The actual value that failed validation
	Constraint any      // The constraint that was violated (e.g., min=5)
}

// Error returns a formatted error message
func (v *ValidationError) Error() string {
	fieldPath := strings.Join(v.Field, ".")
	if fieldPath == "" {
		fieldPath = "value"
	}

	msg := fmt.Sprintf("%s: %s", fieldPath, v.Message)

	// Add type information if available
	if v.Type != "" {
		msg += fmt.Sprintf(" (type: %s)", v.Type)
	}

	// Add value information for debugging
	if v.Value != nil {
		msg += fmt.Sprintf(", value: %v", v.Value)
	}

	// Add constraint information
	if v.Constraint != nil {
		msg += fmt.Sprintf(", constraint: %v", v.Constraint)
	}

	return msg
}

// ValidationErrors represents multiple validation errors
type ValidationErrors struct {
	Errors []ValidationError
}

// Error returns a formatted string of all validation errors
func (v *ValidationErrors) Error() string {
	if len(v.Errors) == 0 {
		return "no validation errors"
	}

	if len(v.Errors) == 1 {
		return fmt.Sprintf("ValidationError: %s", v.Errors[0].Error())
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("ValidationError: %d errors\n", len(v.Errors)))

	for i, err := range v.Errors {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err.Error()))
	}

	return sb.String()
}

// Add adds a new validation error
func (v *ValidationErrors) Add(err ValidationError) {
	v.Errors = append(v.Errors, err)
}

// HasErrors returns true if there are any validation errors
func (v *ValidationErrors) HasErrors() bool {
	return len(v.Errors) > 0
}

// NewValidationError creates a new validation error
func NewValidationError(field []string, message, errorType string, value, constraint any) ValidationError {
	return ValidationError{
		Field:      field,
		Message:    message,
		Type:       errorType,
		Value:      value,
		Constraint: constraint,
	}
}

// NewFieldError creates a validation error for a specific field
func NewFieldError(fieldName, message, errorType string, value, constraint any) ValidationError {
	return ValidationError{
		Field:      []string{fieldName},
		Message:    message,
		Type:       errorType,
		Value:      value,
		Constraint: constraint,
	}
}

// WithFieldPrefix adds a field prefix to all errors (for nested validation)
func (v *ValidationErrors) WithFieldPrefix(prefix string) *ValidationErrors {
	newErrors := &ValidationErrors{
		Errors: make([]ValidationError, len(v.Errors)),
	}

	for i, err := range v.Errors {
		newField := make([]string, 0, len(err.Field)+1)
		newField = append(newField, prefix)
		newField = append(newField, err.Field...)
		newErrors.Errors[i] = ValidationError{
			Field:      newField,
			Message:    err.Message,
			Type:       err.Type,
			Value:      err.Value,
			Constraint: err.Constraint,
		}
	}

	return newErrors
}

// Merge combines multiple ValidationErrors into one
func (v *ValidationErrors) Merge(other *ValidationErrors) {
	if other == nil {
		return
	}
	v.Errors = append(v.Errors, other.Errors...)
}

// ToError converts ValidationErrors to error interface, or nil if no errors
func (v *ValidationErrors) ToError() error {
	if !v.HasErrors() {
		return nil
	}
	return v
}

// Common error types
const (
	ErrorTypeRequired      = "required"
	ErrorTypeType          = "type"
	ErrorTypeMinLength     = "min_length"
	ErrorTypeMaxLength     = "max_length"
	ErrorTypeMin           = "min"
	ErrorTypeMax           = "max"
	ErrorTypePattern       = "pattern"
	ErrorTypeEmail         = "email"
	ErrorTypeURL           = "url"
	ErrorTypeUUID          = "uuid"
	ErrorTypeEnum          = "enum"
	ErrorTypeUnique        = "unique"
	ErrorTypeCustom        = "custom"
	ErrorTypeModel         = "model"
	ErrorTypeUnknownField  = "unknown_field"
	ErrorTypeDiscriminator = "discriminator"
)
