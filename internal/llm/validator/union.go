package validator

import (
	"fmt"
	"reflect"
	"strings"
)

// Union represents a type that can be one of several possible types
// Example: Union[string, int, bool] - value can be string, int, or bool
type Union struct {
	// Types holds the possible types for this union
	Types []reflect.Type

	// Value holds the actual validated value
	Value any

	// SelectedType indicates which type was selected (index into Types)
	SelectedType int
}

// NewUnion creates a new union type from a set of possible types
func NewUnion(types ...any) *Union {
	reflectTypes := make([]reflect.Type, len(types))
	for i, t := range types {
		reflectTypes[i] = reflect.TypeOf(t)
	}
	return &Union{
		Types:        reflectTypes,
		SelectedType: -1,
	}
}

// Validate attempts to validate the value against each possible type
// Returns the first successful validation
func (u *Union) Validate(ctx *ValidationContext, value any) error {
	if len(u.Types) == 0 {
		return fmt.Errorf("union has no types defined")
	}

	var errors []string

	// Try each type in order (matching Pydantic behavior)
	for i, targetType := range u.Types {
		// Try to validate against this type
		validator := New()

		// Try to create an instance of the target type
		instance := reflect.New(targetType).Interface()

		// If it's a struct, try to validate as a struct
		if targetType.Kind() == reflect.Struct {
			if mapValue, ok := value.(map[string]any); ok {
				// Check if all fields in the input data are present in the target type
				validFields := make(map[string]bool)
				for j := 0; j < targetType.NumField(); j++ {
					field := targetType.Field(j)
					if !field.IsExported() {
						continue
					}
					jsonTag := field.Tag.Get("json")
					fieldName := field.Name
					if jsonTag != "" {
						parts := strings.Split(jsonTag, ",")
						if parts[0] != "" && parts[0] != "-" {
							fieldName = parts[0]
						}
					}
					validFields[fieldName] = true
				}

				// Check for unknown fields
				hasUnknownFields := false
				for key := range mapValue {
					if !validFields[key] {
						hasUnknownFields = true
						break
					}
				}

				if !hasUnknownFields {
					if err := validator.validateStruct(ctx, mapValue, reflect.ValueOf(instance).Elem()); err == nil {
						u.Value = reflect.ValueOf(instance).Elem().Interface()
						u.SelectedType = i
						return nil
					} else {
						errors = append(errors, fmt.Sprintf("type %d (%s): %v", i, targetType.Name(), err))
					}
				} else {
					errors = append(errors, fmt.Sprintf("type %d (%s): has unknown fields", i, targetType.Name()))
				}
			}
		} else {
			// For primitive types, check type compatibility first
			valueType := reflect.TypeOf(value)

			// If types are exactly the same, use the value as-is
			if valueType == targetType {
				u.Value = value
				u.SelectedType = i
				return nil
			}

			// Check if coercion makes sense
			// Only allow coercion between compatible types (e.g., string to numeric, not vice versa)
			if isCoercionValid(valueType.Kind(), targetType.Kind()) {
				coerced, err := validator.coerceType(value, targetType)
				if err == nil {
					u.Value = coerced
					u.SelectedType = i
					return nil
				} else {
					errors = append(errors, fmt.Sprintf("type %d (%s): %v", i, targetType.Name(), err))
				}
			} else {
				errors = append(errors, fmt.Sprintf("type %d (%s): incompatible types", i, targetType.Name()))
			}
		}
	}

	// None of the types matched
	return fmt.Errorf("value does not match any union type: %v", errors)
}

// Get returns the validated value
func (u *Union) Get() any {
	return u.Value
}

// GetType returns the selected type index
func (u *Union) GetType() int {
	return u.SelectedType
}

// DiscriminatedUnion represents a tagged union where the type is determined
// by a discriminator field (like a "type" or "kind" field)
// Example: Animal with discriminator "species" can be Dog or Cat
type DiscriminatedUnion struct {
	// Discriminator is the field name that determines the type
	Discriminator string

	// Mapping maps discriminator values to types
	Mapping map[string]reflect.Type

	// Value holds the actual validated value
	Value any

	// SelectedKey indicates which discriminator value was used
	SelectedKey string
}

// NewDiscriminatedUnion creates a new discriminated union
func NewDiscriminatedUnion(discriminator string, mapping map[string]any) *DiscriminatedUnion {
	reflectMapping := make(map[string]reflect.Type)
	for key, val := range mapping {
		reflectMapping[key] = reflect.TypeOf(val)
	}
	return &DiscriminatedUnion{
		Discriminator: discriminator,
		Mapping:       reflectMapping,
	}
}

// Validate validates the value as a discriminated union
func (d *DiscriminatedUnion) Validate(ctx *ValidationContext, value any) error {
	// Value must be a map to have a discriminator field
	mapValue, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("discriminated union value must be an object, got %T", value)
	}

	// Get discriminator value
	discriminatorValue, ok := mapValue[d.Discriminator]
	if !ok {
		return fmt.Errorf("missing discriminator field '%s'", d.Discriminator)
	}

	discriminatorStr, ok := discriminatorValue.(string)
	if !ok {
		return fmt.Errorf("discriminator field '%s' must be a string, got %T", d.Discriminator, discriminatorValue)
	}

	// Look up the type based on discriminator
	targetType, ok := d.Mapping[discriminatorStr]
	if !ok {
		validKeys := make([]string, 0, len(d.Mapping))
		for k := range d.Mapping {
			validKeys = append(validKeys, k)
		}
		return fmt.Errorf("unknown discriminator value '%s', valid values: %v", discriminatorStr, validKeys)
	}

	// Validate against the selected type
	validator := New()

	instance := reflect.New(targetType).Elem()
	if err := validator.validateStruct(ctx, mapValue, instance); err != nil {
		return fmt.Errorf("validation failed for discriminator '%s': %w", discriminatorStr, err)
	}

	d.Value = instance.Interface()
	d.SelectedKey = discriminatorStr
	return nil
}

// Get returns the validated value
func (d *DiscriminatedUnion) Get() any {
	return d.Value
}

// GetKey returns the selected discriminator key
func (d *DiscriminatedUnion) GetKey() string {
	return d.SelectedKey
}

// UnionField is a helper for defining union fields in structs
// Use this in your struct tags to indicate a field is a union
type UnionField struct {
	union *Union
	value any
}

// NewUnionField creates a new union field
func NewUnionField(types ...any) *UnionField {
	return &UnionField{
		union: NewUnion(types...),
	}
}

// UnmarshalJSON implements json.Unmarshaler for union fields
func (u *UnionField) UnmarshalJSON(data []byte) error {
	// This will be called during JSON unmarshaling
	// The actual validation happens in the validator
	return nil
}

// Set sets the value after validation
func (u *UnionField) Set(value any) {
	u.value = value
}

// Get returns the current value
func (u *UnionField) Get() any {
	return u.value
}

// ValidateUnion is a helper function to validate a union value
func ValidateUnion(ctx *ValidationContext, value any, types ...reflect.Type) (any, error) {
	union := &Union{
		Types:        types,
		SelectedType: -1,
	}

	if err := union.Validate(ctx, value); err != nil {
		return nil, err
	}

	return union.Get(), nil
}

// ValidateDiscriminatedUnion is a helper function to validate a discriminated union
func ValidateDiscriminatedUnion(ctx *ValidationContext, value any, discriminator string, mapping map[string]reflect.Type) (any, error) {
	union := &DiscriminatedUnion{
		Discriminator: discriminator,
		Mapping:       mapping,
	}

	if err := union.Validate(ctx, value); err != nil {
		return nil, err
	}

	return union.Get(), nil
}

// isCoercionValid checks if coercion between two kinds is valid
// For unions, we want to be selective about which coercions are allowed
func isCoercionValid(from, to reflect.Kind) bool {
	// Same kind - always valid
	if from == to {
		return true
	}

	// String can be coerced to numeric types
	if from == reflect.String {
		switch to {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64, reflect.Bool:
			return true
		}
	}

	// Numeric types can be coerced to other numeric types
	numericKinds := map[reflect.Kind]bool{
		reflect.Int: true, reflect.Int8: true, reflect.Int16: true, reflect.Int32: true, reflect.Int64: true,
		reflect.Uint: true, reflect.Uint8: true, reflect.Uint16: true, reflect.Uint32: true, reflect.Uint64: true,
		reflect.Float32: true, reflect.Float64: true,
	}

	if numericKinds[from] && numericKinds[to] {
		return true
	}

	// No other coercions are valid for unions
	return false
}

// Example usage patterns:
//
// 1. Simple Union:
//    type Response struct {
//        Result Union  // Can be string, int, or bool
//    }
//
// 2. Discriminated Union:
//    type Animal struct {
//        Species string
//        // ... other fields based on species
//    }
//
//    type Dog struct {
//        Species string `json:"species" validate:"eq=dog"`
//        Breed   string `json:"breed"`
//    }
//
//    type Cat struct {
//        Species   string `json:"species" validate:"eq=cat"`
//        WhiskerCount int `json:"whisker_count"`
//    }
//
// 3. Using in validation:
//    union := NewDiscriminatedUnion("species", map[string]any{
//        "dog": Dog{},
//        "cat": Cat{},
//    })
//    err := union.Validate(ctx, data)
