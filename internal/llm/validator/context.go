package validator

// ValidationMode represents the validation mode
type ValidationMode string

const (
	// ModeValidation is for validating input data
	ModeValidation ValidationMode = "validation"
	// ModeSerialization is for validating output data
	ModeSerialization ValidationMode = "serialization"
)

// ValidationContext provides context information during validation
type ValidationContext struct {
	// FieldName is the current field being validated
	FieldName string

	// FieldPath is the full path to the current field (for nested structures)
	FieldPath []string

	// ValidatedData contains fields that have already been validated
	// This allows validators to access other field values
	ValidatedData map[string]any

	// RawData is the original input data before validation
	RawData map[string]any

	// UserContext is custom context data provided by the user
	UserContext map[string]any

	// Mode indicates whether this is validation or serialization
	Mode ValidationMode

	// Parent is the parent context for nested validation
	Parent *ValidationContext
}

// NewValidationContext creates a new validation context
func NewValidationContext() *ValidationContext {
	return &ValidationContext{
		FieldPath:     []string{},
		ValidatedData: make(map[string]any),
		RawData:       make(map[string]any),
		UserContext:   make(map[string]any),
		Mode:          ModeValidation,
	}
}

// WithField creates a child context for a specific field
func (c *ValidationContext) WithField(fieldName string) *ValidationContext {
	newPath := make([]string, len(c.FieldPath)+1)
	copy(newPath, c.FieldPath)
	newPath[len(c.FieldPath)] = fieldName

	return &ValidationContext{
		FieldName:     fieldName,
		FieldPath:     newPath,
		ValidatedData: c.ValidatedData,
		RawData:       c.RawData,
		UserContext:   c.UserContext,
		Mode:          c.Mode,
		Parent:        c,
	}
}

// WithMode creates a new context with a different mode
func (c *ValidationContext) WithMode(mode ValidationMode) *ValidationContext {
	newCtx := *c
	newCtx.Mode = mode
	return &newCtx
}

// GetFieldPathString returns the field path as a dot-separated string
func (c *ValidationContext) GetFieldPathString() string {
	if len(c.FieldPath) == 0 {
		return ""
	}
	result := c.FieldPath[0]
	for i := 1; i < len(c.FieldPath); i++ {
		result += "." + c.FieldPath[i]
	}
	return result
}

// SetValidatedValue sets a validated value in the context
func (c *ValidationContext) SetValidatedValue(key string, value any) {
	c.ValidatedData[key] = value
}

// GetValidatedValue retrieves a validated value from the context
func (c *ValidationContext) GetValidatedValue(key string) (any, bool) {
	val, ok := c.ValidatedData[key]
	return val, ok
}

// GetUserContextValue retrieves a user context value
func (c *ValidationContext) GetUserContextValue(key string) (any, bool) {
	val, ok := c.UserContext[key]
	return val, ok
}

// SetUserContextValue sets a user context value
func (c *ValidationContext) SetUserContextValue(key string, value any) {
	c.UserContext[key] = value
}
