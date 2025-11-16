package validator

import (
	"testing"

	"github.com/aixgo-dev/aixgo/internal/llm/schema"
)

// Example struct for testing
type User struct {
	Name     string  `json:"name" validate:"required,min=3,max=50"`
	Email    string  `json:"email" validate:"required,email"`
	Age      int     `json:"age" validate:"gte=0,lte=120"`
	Score    float64 `json:"score,omitempty" validate:"gt=0,lte=100"`
	Username string  `json:"username" validate:"required,alphanum,min=3"`
}

func TestValidateBasic(t *testing.T) {
	data := map[string]any{
		"name":     "John Doe",
		"email":    "john@example.com",
		"age":      25,
		"score":    95.5,
		"username": "johndoe123",
	}

	user, err := Validate[User](data)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if user.Name != "John Doe" {
		t.Errorf("Expected name 'John Doe', got '%s'", user.Name)
	}

	if user.Age != 25 {
		t.Errorf("Expected age 25, got %d", user.Age)
	}
}

func TestValidateRequired(t *testing.T) {
	data := map[string]any{
		"email": "john@example.com",
		"age":   25,
		// Missing required "name" field
	}

	_, err := Validate[User](data)
	if err == nil {
		t.Fatal("Expected validation error for missing required field")
	}

	valErr, ok := err.(*ValidationErrors)
	if !ok {
		t.Fatalf("Expected ValidationErrors, got %T", err)
	}

	if len(valErr.Errors) == 0 {
		t.Fatal("Expected at least one error")
	}

	// Check that it's a required field error
	found := false
	for _, e := range valErr.Errors {
		if e.Type == ErrorTypeRequired {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected a required field error")
	}
}

func TestValidateMin(t *testing.T) {
	data := map[string]any{
		"name":     "Jo", // Too short (min=3)
		"email":    "john@example.com",
		"age":      25,
		"username": "jd", // Too short (min=3)
	}

	_, err := Validate[User](data)
	if err == nil {
		t.Fatal("Expected validation error for short strings")
	}

	valErr, ok := err.(*ValidationErrors)
	if !ok {
		t.Fatalf("Expected ValidationErrors, got %T", err)
	}

	if len(valErr.Errors) < 2 {
		t.Errorf("Expected at least 2 errors (name and username), got %d", len(valErr.Errors))
	}
}

func TestValidateEmail(t *testing.T) {
	data := map[string]any{
		"name":     "John Doe",
		"email":    "invalid-email", // Invalid email
		"age":      25,
		"username": "johndoe",
	}

	_, err := Validate[User](data)
	if err == nil {
		t.Fatal("Expected validation error for invalid email")
	}
}

func TestValidateAge(t *testing.T) {
	tests := []struct {
		name    string
		age     int
		wantErr bool
	}{
		{"valid age", 25, false},
		{"zero age", 0, false},
		{"max age", 120, false},
		{"negative age", -1, true},
		{"too old", 121, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]any{
				"name":     "John Doe",
				"email":    "john@example.com",
				"age":      tt.age,
				"username": "johndoe",
			}

			_, err := Validate[User](data)
			if (err != nil) != tt.wantErr {
				t.Errorf("age=%d: wantErr=%v, got error=%v", tt.age, tt.wantErr, err)
			}
		})
	}
}

func TestTypeCoercion(t *testing.T) {
	// Test that string numbers are coerced to int
	data := map[string]any{
		"name":     "John Doe",
		"email":    "john@example.com",
		"age":      "25", // String instead of int
		"score":    "95.5", // String instead of float
		"username": "johndoe",
	}

	user, err := Validate[User](data)
	if err != nil {
		t.Fatalf("Type coercion failed: %v", err)
	}

	if user.Age != 25 {
		t.Errorf("Expected age 25, got %d", user.Age)
	}

	if user.Score != 95.5 {
		t.Errorf("Expected score 95.5, got %f", user.Score)
	}
}

func TestStrictMode(t *testing.T) {
	// In strict mode, type coercion should fail
	data := map[string]any{
		"name":     "John Doe",
		"email":    "john@example.com",
		"age":      "25", // String - should fail in strict mode
		"username": "johndoe",
	}

	_, err := ValidateStrict[User](data)
	if err == nil {
		t.Fatal("Expected error in strict mode with type mismatch")
	}
}

// Test constrained types
type Product struct {
	ID    schema.UUID         `json:"id" validate:"required"`
	Name  string              `json:"name" validate:"required"`
	Price schema.PositiveFloat `json:"price" validate:"required"`
}

func TestConstrainedTypes(t *testing.T) {
	data := map[string]any{
		"id":    "550e8400-e29b-41d4-a716-446655440000",
		"name":  "Widget",
		"price": 19.99,
	}

	product, err := Validate[Product](data)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	// Validate the constrained types
	if err := product.ID.Validate(); err != nil {
		t.Errorf("UUID validation failed: %v", err)
	}

	if err := product.Price.Validate(); err != nil {
		t.Errorf("PositiveFloat validation failed: %v", err)
	}
}

func TestConstrainedTypesInvalid(t *testing.T) {
	data := map[string]any{
		"id":    "invalid-uuid",
		"name":  "Widget",
		"price": -5.0, // Negative price
	}

	product, err := Validate[Product](data)
	if err != nil {
		// Type coercion might not detect UUID invalidity immediately
		// The Validate() method on UUID type will catch it
		t.Logf("Validation error (expected): %v", err)
	}

	if product != nil {
		// Even if initial validation passes, constrained type validation should fail
		if err := product.ID.Validate(); err == nil {
			t.Error("Expected UUID validation to fail for invalid UUID")
		}

		if err := product.Price.Validate(); err == nil {
			t.Error("Expected PositiveFloat validation to fail for negative price")
		}
	}
}

func TestNestedStructs(t *testing.T) {
	type Address struct {
		Street string `json:"street" validate:"required"`
		City   string `json:"city" validate:"required"`
		Zip    string `json:"zip" validate:"required,numeric,min=5,max=5"`
	}

	type Person struct {
		Name    string  `json:"name" validate:"required"`
		Address Address `json:"address" validate:"required"`
	}

	data := map[string]any{
		"name": "John Doe",
		"address": map[string]any{
			"street": "123 Main St",
			"city":   "Springfield",
			"zip":    "12345",
		},
	}

	person, err := Validate[Person](data)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if person.Address.City != "Springfield" {
		t.Errorf("Expected city 'Springfield', got '%s'", person.Address.City)
	}
}

func TestValidationErrors(t *testing.T) {
	data := map[string]any{
		"name":  "AB",              // Too short
		"email": "not-an-email",    // Invalid email
		"age":   150,               // Too old
		// Missing username (required)
	}

	_, err := Validate[User](data)
	if err == nil {
		t.Fatal("Expected multiple validation errors")
	}

	valErr, ok := err.(*ValidationErrors)
	if !ok {
		t.Fatalf("Expected ValidationErrors, got %T", err)
	}

	t.Logf("Validation errors:\n%s", valErr.Error())

	// Should have multiple errors
	if len(valErr.Errors) < 3 {
		t.Errorf("Expected at least 3 errors, got %d", len(valErr.Errors))
	}
}
