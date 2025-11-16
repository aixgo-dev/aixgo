package validator

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/aixgo-dev/aixgo/internal/llm/schema"
)

// ValidatorFunc is a function that validates a value
type ValidatorFunc func(ctx *ValidationContext, value any) (any, error)

// ValidatorMode represents when a validator runs
type ValidatorMode int

const (
	// BeforeValidator runs before type parsing/coercion
	BeforeValidator ValidatorMode = iota
	// AfterValidator runs after type parsing (most common)
	AfterValidator
	// WrapValidator wraps the validation process
	WrapValidator
)

// FieldValidatorConfig configures a field validator
type FieldValidatorConfig struct {
	Mode      ValidatorMode
	Validator ValidatorFunc
}

// Validator handles struct validation based on tags
type Validator struct {
	strictMode bool
	coerce     bool
}

// New creates a new validator with default settings
func New() *Validator {
	return &Validator{
		strictMode: false,
		coerce:     true,
	}
}

// NewStrict creates a validator in strict mode (no type coercion)
func NewStrict() *Validator {
	return &Validator{
		strictMode: true,
		coerce:     false,
	}
}

// Validate validates data against a struct type
func Validate[T any](data map[string]any) (*T, error) {
	v := New()
	return ValidateWithValidator[T](v, data)
}

// ValidateStrict validates data in strict mode
func ValidateStrict[T any](data map[string]any) (*T, error) {
	v := NewStrict()
	return ValidateWithValidator[T](v, data)
}

// ValidateWithValidator validates using a specific validator instance
func ValidateWithValidator[T any](v *Validator, data map[string]any) (*T, error) {
	var result T
	ctx := NewValidationContext()
	ctx.RawData = data

	err := v.validateStruct(ctx, data, reflect.ValueOf(&result).Elem())
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// validateStruct validates a struct value
func (v *Validator) validateStruct(ctx *ValidationContext, data map[string]any, value reflect.Value) error {
	errors := &ValidationErrors{}

	typ := value.Type()

	// Process each field
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := value.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get JSON tag for field name
		jsonTag := field.Tag.Get("json")
		fieldName := field.Name
		if jsonTag != "" && jsonTag != "-" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}

		// Create field context
		fieldCtx := ctx.WithField(fieldName)

		// Get validate tag
		validateTag := field.Tag.Get("validate")

		// Get field data
		fieldData, exists := data[fieldName]

		// Check required
		if v.isRequired(validateTag) && !exists {
			errors.Add(NewFieldError(
				fieldName,
				"field is required",
				ErrorTypeRequired,
				nil,
				"required",
			))
			continue
		}

		// If field doesn't exist, check if we should validate the zero value
		if !exists {
			// Check for omitempty in JSON tag
			hasOmitEmpty := false
			if jsonTag != "" {
				parts := strings.Split(jsonTag, ",")
				for _, part := range parts {
					if part == "omitempty" {
						hasOmitEmpty = true
						break
					}
				}
			}

			// If omitempty, skip validation entirely
			if hasOmitEmpty {
				continue
			}

			// If there are validation tags other than "required", validate the zero value
			if validateTag != "" && validateTag != "required" {
				// Use the zero value
				fieldData = fieldValue.Interface()
			} else {
				// No validation needed, skip
				continue
			}
		}

		// Validate field
		validatedValue, err := v.validateField(fieldCtx, field, validateTag, fieldData)
		if err != nil {
			if valErr, ok := err.(*ValidationErrors); ok {
				errors.Merge(valErr.WithFieldPrefix(fieldName))
			} else {
				errors.Add(NewFieldError(
					fieldName,
					err.Error(),
					ErrorTypeCustom,
					fieldData,
					nil,
				))
			}
			continue
		}

		// Set the value
		if validatedValue != nil {
			if err := v.setValue(fieldValue, validatedValue); err != nil {
				errors.Add(NewFieldError(
					fieldName,
					fmt.Sprintf("failed to set value: %v", err),
					ErrorTypeType,
					validatedValue,
					nil,
				))
			}
		}

		// Store validated value in context
		ctx.SetValidatedValue(fieldName, validatedValue)
	}

	// Run model-level validation if the type implements Validatable
	if errors.HasErrors() {
		return errors
	}

	// Check if struct implements Validatable interface
	if validatable, ok := value.Addr().Interface().(schema.Validatable); ok {
		if err := validatable.Validate(); err != nil {
			errors.Add(ValidationError{
				Field:   []string{},
				Message: err.Error(),
				Type:    ErrorTypeModel,
			})
		}
	}

	return errors.ToError()
}

// validateField validates a single field
func (v *Validator) validateField(ctx *ValidationContext, field reflect.StructField, validateTag string, value any) (any, error) {
	// First, coerce the type if needed
	targetType := field.Type
	coercedValue, err := v.coerceType(value, targetType)
	if err != nil {
		return nil, fmt.Errorf("type coercion failed: %w", err)
	}

	// Parse and apply validation rules from tag
	if validateTag != "" {
		rules := parseValidateTag(validateTag)
		for _, rule := range rules {
			if err := v.applyRule(ctx, rule, coercedValue, targetType); err != nil {
				return nil, err
			}
		}
	}

	// If the type implements Validatable, call its Validate method
	if validatable, ok := coercedValue.(schema.Validatable); ok {
		if err := validatable.Validate(); err != nil {
			return nil, err
		}
	}

	return coercedValue, nil
}

// coerceType converts a value to the target type
func (v *Validator) coerceType(value any, targetType reflect.Type) (any, error) {
	if !v.coerce {
		// Strict mode - no coercion
		valueType := reflect.TypeOf(value)
		if valueType != targetType {
			return nil, fmt.Errorf("type mismatch: expected %s, got %s", targetType, valueType)
		}
		return value, nil
	}

	// Handle nil
	if value == nil {
		return reflect.Zero(targetType).Interface(), nil
	}

	valueReflect := reflect.ValueOf(value)
	valueType := valueReflect.Type()

	// If types match, no coercion needed
	if valueType.AssignableTo(targetType) {
		return value, nil
	}

	// Try to convert
	if valueType.ConvertibleTo(targetType) {
		return valueReflect.Convert(targetType).Interface(), nil
	}

	// Special handling for common conversions
	switch targetType.Kind() {
	case reflect.String:
		return fmt.Sprintf("%v", value), nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.toInt(value, targetType)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.toUint(value, targetType)

	case reflect.Float32, reflect.Float64:
		return v.toFloat(value, targetType)

	case reflect.Bool:
		return v.toBool(value)

	case reflect.Slice:
		return v.toSlice(value, targetType)

	case reflect.Struct:
		// Handle nested struct validation
		if valueMap, ok := value.(map[string]any); ok {
			return v.structFromMap(valueMap, targetType)
		}
	}

	return nil, fmt.Errorf("cannot convert %T to %s", value, targetType)
}

// Type conversion helpers

func (v *Validator) toInt(value any, targetType reflect.Type) (any, error) {
	var i64 int64

	switch val := value.(type) {
	case int:
		i64 = int64(val)
	case int64:
		i64 = val
	case float64:
		i64 = int64(val)
	case string:
		parsed, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot parse '%s' as integer: %w", val, err)
		}
		i64 = parsed
	case json.Number:
		parsed, err := val.Int64()
		if err != nil {
			return nil, err
		}
		i64 = parsed
	default:
		return nil, fmt.Errorf("cannot convert %T to int", value)
	}

	// Convert to the target type
	switch targetType.Kind() {
	case reflect.Int:
		return int(i64), nil
	case reflect.Int8:
		return int8(i64), nil
	case reflect.Int16:
		return int16(i64), nil
	case reflect.Int32:
		return int32(i64), nil
	case reflect.Int64:
		return i64, nil
	default:
		return i64, nil
	}
}

func (v *Validator) toUint(value any, targetType reflect.Type) (any, error) {
	switch val := value.(type) {
	case uint:
		return val, nil
	case uint64:
		return val, nil
	case int:
		if val < 0 {
			return nil, fmt.Errorf("cannot convert negative int to uint")
		}
		return uint64(val), nil
	case float64:
		if val < 0 {
			return nil, fmt.Errorf("cannot convert negative float to uint")
		}
		return uint64(val), nil
	case string:
		u, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot parse '%s' as uint: %w", val, err)
		}
		return u, nil
	default:
		return nil, fmt.Errorf("cannot convert %T to uint", value)
	}
}

func (v *Validator) toFloat(value any, targetType reflect.Type) (any, error) {
	switch val := value.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot parse '%s' as float: %w", val, err)
		}
		return f, nil
	case json.Number:
		return val.Float64()
	default:
		return nil, fmt.Errorf("cannot convert %T to float", value)
	}
}

func (v *Validator) toBool(value any) (any, error) {
	switch val := value.(type) {
	case bool:
		return val, nil
	case string:
		return strconv.ParseBool(val)
	case int, int64:
		return val != 0, nil
	case float64:
		return val != 0, nil
	default:
		return nil, fmt.Errorf("cannot convert %T to bool", value)
	}
}

func (v *Validator) toSlice(value any, targetType reflect.Type) (any, error) {
	valueReflect := reflect.ValueOf(value)
	if valueReflect.Kind() != reflect.Slice {
		return nil, fmt.Errorf("expected slice, got %T", value)
	}

	// Create new slice of target element type
	elemType := targetType.Elem()
	result := reflect.MakeSlice(targetType, valueReflect.Len(), valueReflect.Len())

	// Convert each element
	for i := 0; i < valueReflect.Len(); i++ {
		elem := valueReflect.Index(i).Interface()
		converted, err := v.coerceType(elem, elemType)
		if err != nil {
			return nil, fmt.Errorf("element %d: %w", i, err)
		}
		result.Index(i).Set(reflect.ValueOf(converted))
	}

	return result.Interface(), nil
}

func (v *Validator) structFromMap(data map[string]any, targetType reflect.Type) (any, error) {
	result := reflect.New(targetType).Elem()
	ctx := NewValidationContext()
	ctx.RawData = data

	if err := v.validateStruct(ctx, data, result); err != nil {
		return nil, err
	}

	return result.Interface(), nil
}

// setValue sets a reflect.Value with proper type handling
func (v *Validator) setValue(target reflect.Value, value any) error {
	if !target.CanSet() {
		return fmt.Errorf("cannot set value")
	}

	valueReflect := reflect.ValueOf(value)

	// Handle nil
	if value == nil {
		target.Set(reflect.Zero(target.Type()))
		return nil
	}

	// Direct assignment if types match
	if valueReflect.Type().AssignableTo(target.Type()) {
		target.Set(valueReflect)
		return nil
	}

	// Try conversion
	if valueReflect.Type().ConvertibleTo(target.Type()) {
		target.Set(valueReflect.Convert(target.Type()))
		return nil
	}

	return fmt.Errorf("cannot assign %T to %s", value, target.Type())
}

// Validation rule application

type validationRule struct {
	name  string
	param string
}

func parseValidateTag(tag string) []validationRule {
	if tag == "" {
		return nil
	}

	parts := strings.Split(tag, ",")
	rules := make([]validationRule, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Split on '=' for parameterized rules
		if idx := strings.Index(part, "="); idx > 0 {
			rules = append(rules, validationRule{
				name:  part[:idx],
				param: part[idx+1:],
			})
		} else {
			rules = append(rules, validationRule{
				name: part,
			})
		}
	}

	return rules
}

func (v *Validator) isRequired(validateTag string) bool {
	rules := parseValidateTag(validateTag)
	for _, rule := range rules {
		if rule.name == "required" {
			return true
		}
	}
	return false
}

func (v *Validator) applyRule(ctx *ValidationContext, rule validationRule, value any, targetType reflect.Type) error {
	switch rule.name {
	case "required":
		// Already handled earlier
		return nil

	case "min":
		return v.validateMin(value, rule.param)

	case "max":
		return v.validateMax(value, rule.param)

	case "gte":
		return v.validateGTE(value, rule.param)

	case "gt":
		return v.validateGT(value, rule.param)

	case "lte":
		return v.validateLTE(value, rule.param)

	case "lt":
		return v.validateLT(value, rule.param)

	case "oneof":
		return v.validateOneOf(value, strings.Split(rule.param, " "))

	case "email":
		return v.validateEmail(value)

	case "url":
		return v.validateURL(value)

	case "uuid":
		return v.validateUUID(value)

	case "pattern":
		return v.validatePattern(value, rule.param)

	case "alpha":
		return v.validateAlpha(value)

	case "alphanum":
		return v.validateAlphaNum(value)

	case "numeric":
		return v.validateNumeric(value)

	default:
		// Unknown rule - ignore or return error?
		return nil
	}
}

// Validation rule implementations

func (v *Validator) validateMin(value any, param string) error {
	switch val := value.(type) {
	case string:
		minLen, _ := strconv.Atoi(param)
		if len(val) < minLen {
			return fmt.Errorf("must be at least %d characters, got %d", minLen, len(val))
		}
	case int, int64:
		minVal, _ := strconv.ParseInt(param, 10, 64)
		intVal := reflect.ValueOf(val).Int()
		if intVal < minVal {
			return fmt.Errorf("must be at least %d, got %d", minVal, intVal)
		}
	case float64, float32:
		minVal, _ := strconv.ParseFloat(param, 64)
		floatVal := reflect.ValueOf(val).Float()
		if floatVal < minVal {
			return fmt.Errorf("must be at least %f, got %f", minVal, floatVal)
		}
	}
	return nil
}

func (v *Validator) validateMax(value any, param string) error {
	switch val := value.(type) {
	case string:
		maxLen, _ := strconv.Atoi(param)
		if len(val) > maxLen {
			return fmt.Errorf("must be at most %d characters, got %d", maxLen, len(val))
		}
	case int, int64:
		maxVal, _ := strconv.ParseInt(param, 10, 64)
		intVal := reflect.ValueOf(val).Int()
		if intVal > maxVal {
			return fmt.Errorf("must be at most %d, got %d", maxVal, intVal)
		}
	case float64, float32:
		maxVal, _ := strconv.ParseFloat(param, 64)
		floatVal := reflect.ValueOf(val).Float()
		if floatVal > maxVal {
			return fmt.Errorf("must be at most %f, got %f", maxVal, floatVal)
		}
	}
	return nil
}

func (v *Validator) validateGTE(value any, param string) error {
	switch val := value.(type) {
	case int, int64:
		minVal, _ := strconv.ParseInt(param, 10, 64)
		intVal := reflect.ValueOf(val).Int()
		if intVal < minVal {
			return fmt.Errorf("must be greater than or equal to %d, got %d", minVal, intVal)
		}
	case float64, float32:
		minVal, _ := strconv.ParseFloat(param, 64)
		floatVal := reflect.ValueOf(val).Float()
		if floatVal < minVal {
			return fmt.Errorf("must be greater than or equal to %f, got %f", minVal, floatVal)
		}
	}
	return nil
}

func (v *Validator) validateGT(value any, param string) error {
	switch val := value.(type) {
	case int, int64:
		minVal, _ := strconv.ParseInt(param, 10, 64)
		intVal := reflect.ValueOf(val).Int()
		if intVal <= minVal {
			return fmt.Errorf("must be greater than %d, got %d", minVal, intVal)
		}
	case float64, float32:
		minVal, _ := strconv.ParseFloat(param, 64)
		floatVal := reflect.ValueOf(val).Float()
		if floatVal <= minVal {
			return fmt.Errorf("must be greater than %f, got %f", minVal, floatVal)
		}
	}
	return nil
}

func (v *Validator) validateLTE(value any, param string) error {
	switch val := value.(type) {
	case int, int64:
		maxVal, _ := strconv.ParseInt(param, 10, 64)
		intVal := reflect.ValueOf(val).Int()
		if intVal > maxVal {
			return fmt.Errorf("must be less than or equal to %d, got %d", maxVal, intVal)
		}
	case float64, float32:
		maxVal, _ := strconv.ParseFloat(param, 64)
		floatVal := reflect.ValueOf(val).Float()
		if floatVal > maxVal {
			return fmt.Errorf("must be less than or equal to %f, got %f", maxVal, floatVal)
		}
	}
	return nil
}

func (v *Validator) validateLT(value any, param string) error {
	switch val := value.(type) {
	case int, int64:
		maxVal, _ := strconv.ParseInt(param, 10, 64)
		intVal := reflect.ValueOf(val).Int()
		if intVal >= maxVal {
			return fmt.Errorf("must be less than %d, got %d", maxVal, intVal)
		}
	case float64, float32:
		maxVal, _ := strconv.ParseFloat(param, 64)
		floatVal := reflect.ValueOf(val).Float()
		if floatVal >= maxVal {
			return fmt.Errorf("must be less than %f, got %f", maxVal, floatVal)
		}
	}
	return nil
}

func (v *Validator) validateOneOf(value any, options []string) error {
	str := fmt.Sprintf("%v", value)
	for _, opt := range options {
		if str == opt {
			return nil
		}
	}
	return fmt.Errorf("must be one of [%s], got '%s'", strings.Join(options, ", "), str)
}

func (v *Validator) validateEmail(value any) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("email must be a string")
	}
	email := schema.EmailStr(str)
	return email.Validate()
}

func (v *Validator) validateURL(value any) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("URL must be a string")
	}
	url := schema.HttpUrl(str)
	return url.Validate()
}

func (v *Validator) validateUUID(value any) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("UUID must be a string")
	}
	uuid := schema.UUID(str)
	return uuid.Validate()
}

func (v *Validator) validatePattern(value any, pattern string) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("pattern validation requires a string")
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}

	if !re.MatchString(str) {
		return fmt.Errorf("must match pattern '%s'", pattern)
	}

	return nil
}

func (v *Validator) validateAlpha(value any) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("alpha validation requires a string")
	}
	alpha := schema.AlphaStr(str)
	return alpha.Validate()
}

func (v *Validator) validateAlphaNum(value any) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("alphanumeric validation requires a string")
	}
	alphaNum := schema.AlphaNumStr(str)
	return alphaNum.Validate()
}

func (v *Validator) validateNumeric(value any) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("numeric validation requires a string")
	}
	numeric := schema.NumericStr(str)
	return numeric.Validate()
}
