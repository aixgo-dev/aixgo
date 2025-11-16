package validator

import (
	"fmt"
	"reflect"
)

// ListOf creates a validator for a list of items of a specific type
// Example: ListOf(User{}) validates []User
type ListOf struct {
	ItemType reflect.Type
	Items    []any
	MinItems int
	MaxItems int
}

// NewListOf creates a new list validator
func NewListOf(itemType any) *ListOf {
	return &ListOf{
		ItemType: reflect.TypeOf(itemType),
		Items:    []any{},
		MinItems: 0,
		MaxItems: -1, // -1 means no limit
	}
}

// WithMinItems sets the minimum number of items
func (l *ListOf) WithMinItems(min int) *ListOf {
	l.MinItems = min
	return l
}

// WithMaxItems sets the maximum number of items
func (l *ListOf) WithMaxItems(max int) *ListOf {
	l.MaxItems = max
	return l
}

// Validate validates a list value
func (l *ListOf) Validate(ctx *ValidationContext, value any) error {
	// Value must be a slice or array
	val := reflect.ValueOf(value)
	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return fmt.Errorf("expected array/list, got %T", value)
	}

	length := val.Len()

	// Check min items
	if l.MinItems > 0 && length < l.MinItems {
		return fmt.Errorf("list must have at least %d items, got %d", l.MinItems, length)
	}

	// Check max items
	if l.MaxItems >= 0 && length > l.MaxItems {
		return fmt.Errorf("list must have at most %d items, got %d", l.MaxItems, length)
	}

	// Validate each item
	validator := New()
	items := make([]any, length)

	for i := 0; i < length; i++ {
		item := val.Index(i).Interface()

		// If item type is a struct, validate it
		if l.ItemType.Kind() == reflect.Struct {
			if mapValue, ok := item.(map[string]any); ok {
				instance := reflect.New(l.ItemType).Elem()
				if err := validator.validateStruct(ctx, mapValue, instance); err != nil {
					return fmt.Errorf("item %d: %w", i, err)
				}
				items[i] = instance.Interface()
			} else {
				return fmt.Errorf("item %d: expected object, got %T", i, item)
			}
		} else {
			// For primitive types, try coercion
			coerced, err := validator.coerceType(item, l.ItemType)
			if err != nil {
				return fmt.Errorf("item %d: %w", i, err)
			}
			items[i] = coerced
		}
	}

	l.Items = items
	return nil
}

// Get returns the validated items
func (l *ListOf) Get() []any {
	return l.Items
}

// DictOf creates a validator for a dictionary/map with specific key and value types
type DictOf struct {
	KeyType   reflect.Type
	ValueType reflect.Type
	Items     map[any]any
	MinItems  int
	MaxItems  int
}

// NewDictOf creates a new dict validator
func NewDictOf(keyType, valueType any) *DictOf {
	return &DictOf{
		KeyType:   reflect.TypeOf(keyType),
		ValueType: reflect.TypeOf(valueType),
		Items:     make(map[any]any),
		MinItems:  0,
		MaxItems:  -1,
	}
}

// WithMinItems sets the minimum number of items
func (d *DictOf) WithMinItems(min int) *DictOf {
	d.MinItems = min
	return d
}

// WithMaxItems sets the maximum number of items
func (d *DictOf) WithMaxItems(max int) *DictOf {
	d.MaxItems = max
	return d
}

// Validate validates a dictionary value
func (d *DictOf) Validate(ctx *ValidationContext, value any) error {
	// Value must be a map
	mapValue, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("expected object/dictionary, got %T", value)
	}

	length := len(mapValue)

	// Check min items
	if d.MinItems > 0 && length < d.MinItems {
		return fmt.Errorf("dictionary must have at least %d items, got %d", d.MinItems, length)
	}

	// Check max items
	if d.MaxItems >= 0 && length > d.MaxItems {
		return fmt.Errorf("dictionary must have at most %d items, got %d", d.MaxItems, length)
	}

	// Validate each key-value pair
	validator := New()
	items := make(map[any]any)

	for key, val := range mapValue {
		// Validate key type (usually string)
		var coercedKey any
		var err error

		if d.KeyType.Kind() == reflect.String {
			coercedKey = key
		} else {
			coercedKey, err = validator.coerceType(key, d.KeyType)
			if err != nil {
				return fmt.Errorf("key '%s': %w", key, err)
			}
		}

		// Validate value type
		var coercedValue any
		if d.ValueType.Kind() == reflect.Struct {
			if mapVal, ok := val.(map[string]any); ok {
				instance := reflect.New(d.ValueType).Elem()
				if err := validator.validateStruct(ctx, mapVal, instance); err != nil {
					return fmt.Errorf("value for key '%s': %w", key, err)
				}
				coercedValue = instance.Interface()
			} else {
				return fmt.Errorf("value for key '%s': expected object, got %T", key, val)
			}
		} else {
			coercedValue, err = validator.coerceType(val, d.ValueType)
			if err != nil {
				return fmt.Errorf("value for key '%s': %w", key, err)
			}
		}

		items[coercedKey] = coercedValue
	}

	d.Items = items
	return nil
}

// Get returns the validated items
func (d *DictOf) Get() map[any]any {
	return d.Items
}

// Optional wraps a type to make it optional (can be nil)
type Optional struct {
	Type     reflect.Type
	Value    any
	HasValue bool
}

// NewOptional creates a new optional validator
func NewOptional(itemType any) *Optional {
	return &Optional{
		Type:     reflect.TypeOf(itemType),
		HasValue: false,
	}
}

// Validate validates an optional value
func (o *Optional) Validate(ctx *ValidationContext, value any) error {
	// nil values are valid for optional
	if value == nil {
		o.HasValue = false
		o.Value = nil
		return nil
	}

	// Validate the value
	validator := New()

	if o.Type.Kind() == reflect.Struct {
		if mapValue, ok := value.(map[string]any); ok {
			instance := reflect.New(o.Type).Elem()
			if err := validator.validateStruct(ctx, mapValue, instance); err != nil {
				return err
			}
			o.Value = instance.Interface()
			o.HasValue = true
			return nil
		}
		return fmt.Errorf("expected object, got %T", value)
	}

	// For primitive types, try coercion
	coerced, err := validator.coerceType(value, o.Type)
	if err != nil {
		return err
	}

	o.Value = coerced
	o.HasValue = true
	return nil
}

// Get returns the value (may be nil)
func (o *Optional) Get() any {
	return o.Value
}

// IsSome returns true if the optional has a value
func (o *Optional) IsSome() bool {
	return o.HasValue
}

// IsNone returns true if the optional is nil
func (o *Optional) IsNone() bool {
	return !o.HasValue
}

// Generic helper functions for common patterns

// ValidateList validates a list of items with a given type
func ValidateList[T any](ctx *ValidationContext, value any, itemValidator func(map[string]any) (*T, error)) ([]*T, error) {
	val := reflect.ValueOf(value)
	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return nil, fmt.Errorf("expected array/list, got %T", value)
	}

	length := val.Len()
	result := make([]*T, length)

	for i := 0; i < length; i++ {
		item := val.Index(i).Interface()

		if mapValue, ok := item.(map[string]any); ok {
			validated, err := itemValidator(mapValue)
			if err != nil {
				return nil, fmt.Errorf("item %d: %w", i, err)
			}
			result[i] = validated
		} else {
			return nil, fmt.Errorf("item %d: expected object, got %T", i, item)
		}
	}

	return result, nil
}

// ValidateDict validates a dictionary with typed values
func ValidateDict[T any](ctx *ValidationContext, value any, valueValidator func(map[string]any) (*T, error)) (map[string]*T, error) {
	mapValue, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected object/dictionary, got %T", value)
	}

	result := make(map[string]*T)

	for key, val := range mapValue {
		if mapVal, ok := val.(map[string]any); ok {
			validated, err := valueValidator(mapVal)
			if err != nil {
				return nil, fmt.Errorf("key '%s': %w", key, err)
			}
			result[key] = validated
		} else {
			return nil, fmt.Errorf("key '%s': expected object, got %T", key, val)
		}
	}

	return result, nil
}

// ValidateOptional validates an optional value
func ValidateOptional[T any](ctx *ValidationContext, value any, validator func(map[string]any) (*T, error)) (*T, error) {
	if value == nil {
		return nil, nil
	}

	if mapValue, ok := value.(map[string]any); ok {
		return validator(mapValue)
	}

	return nil, fmt.Errorf("expected object or nil, got %T", value)
}
