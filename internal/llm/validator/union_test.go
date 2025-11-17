package validator

import (
	"reflect"
	"testing"
)

func TestSimpleUnion(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		types    []any
		wantType int
		wantErr  bool
	}{
		{
			name:     "string match",
			value:    "hello",
			types:    []any{"", 0, false},
			wantType: 0,
			wantErr:  false,
		},
		{
			name:     "int match",
			value:    42,
			types:    []any{"", 0, false},
			wantType: 1,
			wantErr:  false,
		},
		{
			name:     "bool match",
			value:    true,
			types:    []any{"", 0, false},
			wantType: 2,
			wantErr:  false,
		},
		{
			name:     "type coercion - string to int",
			value:    "42",
			types:    []any{0, ""},
			wantType: 0, // Should match int first via coercion
			wantErr:  false,
		},
		{
			name:     "no match",
			value:    []int{1, 2, 3},
			types:    []any{"", 0, false},
			wantType: -1,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			union := NewUnion(tt.types...)
			ctx := NewValidationContext()

			err := union.Validate(ctx, tt.value)

			if (err != nil) != tt.wantErr {
				t.Errorf("Union.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && union.GetType() != tt.wantType {
				t.Errorf("Union.GetType() = %d, want %d", union.GetType(), tt.wantType)
			}
		})
	}
}

func TestUnionWithStructs(t *testing.T) {
	type Person struct {
		Name string `json:"name" validate:"required"`
		Age  int    `json:"age" validate:"gte=0"`
	}

	type Company struct {
		Name      string `json:"name" validate:"required"`
		Employees int    `json:"employees" validate:"gt=0"`
	}

	tests := []struct {
		name     string
		value    map[string]any
		wantType int
		wantErr  bool
	}{
		{
			name: "valid person",
			value: map[string]any{
				"name": "John",
				"age":  30,
			},
			wantType: 0,
			wantErr:  false,
		},
		{
			name: "valid company",
			value: map[string]any{
				"name":      "TechCorp",
				"employees": 100,
			},
			wantType: 1,
			wantErr:  false,
		},
		{
			name: "invalid - missing required fields",
			value: map[string]any{
				"invalid": "data",
			},
			wantType: -1,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			union := NewUnion(Person{}, Company{})
			ctx := NewValidationContext()

			err := union.Validate(ctx, tt.value)

			if (err != nil) != tt.wantErr {
				t.Errorf("Union.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && union.GetType() != tt.wantType {
				t.Errorf("Union.GetType() = %d, want %d", union.GetType(), tt.wantType)
			}
		})
	}
}

func TestDiscriminatedUnion(t *testing.T) {
	type Dog struct {
		Species string `json:"species" validate:"required"`
		Breed   string `json:"breed" validate:"required"`
	}

	type Cat struct {
		Species      string `json:"species" validate:"required"`
		WhiskerCount int    `json:"whisker_count" validate:"gt=0"`
	}

	type Bird struct {
		Species  string  `json:"species" validate:"required"`
		Wingspan float64 `json:"wingspan" validate:"gt=0"`
	}

	tests := []struct {
		name    string
		value   map[string]any
		wantKey string
		wantErr bool
	}{
		{
			name: "valid dog",
			value: map[string]any{
				"species": "dog",
				"breed":   "Golden Retriever",
			},
			wantKey: "dog",
			wantErr: false,
		},
		{
			name: "valid cat",
			value: map[string]any{
				"species":       "cat",
				"whisker_count": 24,
			},
			wantKey: "cat",
			wantErr: false,
		},
		{
			name: "valid bird",
			value: map[string]any{
				"species":  "bird",
				"wingspan": 2.5,
			},
			wantKey: "bird",
			wantErr: false,
		},
		{
			name: "missing discriminator",
			value: map[string]any{
				"breed": "Golden Retriever",
			},
			wantKey: "",
			wantErr: true,
		},
		{
			name: "unknown discriminator value",
			value: map[string]any{
				"species": "fish",
				"scales":  100,
			},
			wantKey: "",
			wantErr: true,
		},
		{
			name: "invalid data for type",
			value: map[string]any{
				"species":       "cat",
				"whisker_count": -5, // Invalid - must be > 0
			},
			wantKey: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			union := NewDiscriminatedUnion("species", map[string]any{
				"dog":  Dog{},
				"cat":  Cat{},
				"bird": Bird{},
			})
			ctx := NewValidationContext()

			err := union.Validate(ctx, tt.value)

			if (err != nil) != tt.wantErr {
				t.Errorf("DiscriminatedUnion.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && union.GetKey() != tt.wantKey {
				t.Errorf("DiscriminatedUnion.GetKey() = %s, want %s", union.GetKey(), tt.wantKey)
			}

			if !tt.wantErr && union.Get() == nil {
				t.Error("DiscriminatedUnion.Get() returned nil for successful validation")
			}
		})
	}
}

func TestDiscriminatedUnionWithValidation(t *testing.T) {
	type Circle struct {
		Type   string  `json:"type" validate:"eq=circle"`
		Radius float64 `json:"radius" validate:"gt=0"`
	}

	type Rectangle struct {
		Type   string  `json:"type" validate:"eq=rectangle"`
		Width  float64 `json:"width" validate:"gt=0"`
		Height float64 `json:"height" validate:"gt=0"`
	}

	type Triangle struct {
		Type   string  `json:"type" validate:"eq=triangle"`
		Base   float64 `json:"base" validate:"gt=0"`
		Height float64 `json:"height" validate:"gt=0"`
	}

	tests := []struct {
		name    string
		value   map[string]any
		wantKey string
		wantErr bool
	}{
		{
			name: "valid circle",
			value: map[string]any{
				"type":   "circle",
				"radius": 5.0,
			},
			wantKey: "circle",
			wantErr: false,
		},
		{
			name: "valid rectangle",
			value: map[string]any{
				"type":   "rectangle",
				"width":  10.0,
				"height": 20.0,
			},
			wantKey: "rectangle",
			wantErr: false,
		},
		{
			name: "invalid circle - negative radius",
			value: map[string]any{
				"type":   "circle",
				"radius": -5.0,
			},
			wantKey: "",
			wantErr: true,
		},
		{
			name: "invalid rectangle - missing height",
			value: map[string]any{
				"type":  "rectangle",
				"width": 10.0,
			},
			wantKey: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			union := NewDiscriminatedUnion("type", map[string]any{
				"circle":    Circle{},
				"rectangle": Rectangle{},
				"triangle":  Triangle{},
			})
			ctx := NewValidationContext()

			err := union.Validate(ctx, tt.value)

			if (err != nil) != tt.wantErr {
				t.Errorf("DiscriminatedUnion.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && union.GetKey() != tt.wantKey {
				t.Errorf("DiscriminatedUnion.GetKey() = %s, want %s", union.GetKey(), tt.wantKey)
			}
		})
	}
}

func TestValidateUnionHelper(t *testing.T) {
	ctx := NewValidationContext()

	// Test successful validation
	value, err := ValidateUnion(ctx, "hello", reflect.TypeOf(""), reflect.TypeOf(0))
	if err != nil {
		t.Errorf("ValidateUnion() error = %v, want nil", err)
	}
	if value != "hello" {
		t.Errorf("ValidateUnion() value = %v, want 'hello'", value)
	}

	// Test with integer
	value, err = ValidateUnion(ctx, 42, reflect.TypeOf(0), reflect.TypeOf(""))
	if err != nil {
		t.Errorf("ValidateUnion() error = %v, want nil", err)
	}
	if value != 42 {
		t.Errorf("ValidateUnion() value = %v, want 42", value)
	}

	// Test with type coercion
	value, err = ValidateUnion(ctx, "123", reflect.TypeOf(0), reflect.TypeOf(""))
	if err != nil {
		t.Errorf("ValidateUnion() error = %v, want nil", err)
	}
	if value != 123 {
		t.Errorf("ValidateUnion() value = %v, want 123", value)
	}
}

func TestValidateDiscriminatedUnionHelper(t *testing.T) {
	type Success struct {
		Status string `json:"status" validate:"eq=success"`
		Data   string `json:"data" validate:"required"`
	}

	type Error struct {
		Status  string `json:"status" validate:"eq=error"`
		Message string `json:"message" validate:"required"`
	}

	ctx := NewValidationContext()
	mapping := map[string]reflect.Type{
		"success": reflect.TypeOf(Success{}),
		"error":   reflect.TypeOf(Error{}),
	}

	// Test successful validation
	value := map[string]any{
		"status": "success",
		"data":   "Operation completed",
	}

	result, err := ValidateDiscriminatedUnion(ctx, value, "status", mapping)
	if err != nil {
		t.Errorf("ValidateDiscriminatedUnion() error = %v, want nil", err)
	}

	success, ok := result.(Success)
	if !ok {
		t.Errorf("ValidateDiscriminatedUnion() result type = %T, want Success", result)
	}
	if success.Data != "Operation completed" {
		t.Errorf("ValidateDiscriminatedUnion() success.Data = %s, want 'Operation completed'", success.Data)
	}

	// Test error case
	errorValue := map[string]any{
		"status":  "error",
		"message": "Something went wrong",
	}

	result, err = ValidateDiscriminatedUnion(ctx, errorValue, "status", mapping)
	if err != nil {
		t.Errorf("ValidateDiscriminatedUnion() error = %v, want nil", err)
	}

	errorResult, ok := result.(Error)
	if !ok {
		t.Errorf("ValidateDiscriminatedUnion() result type = %T, want Error", result)
	}
	if errorResult.Message != "Something went wrong" {
		t.Errorf("ValidateDiscriminatedUnion() errorResult.Message = %s, want 'Something went wrong'", errorResult.Message)
	}
}

func TestDiscriminatedUnionNonStringDiscriminator(t *testing.T) {
	type TypeA struct {
		ID int `json:"id"`
	}

	union := NewDiscriminatedUnion("id", map[string]any{
		"1": TypeA{},
	})
	ctx := NewValidationContext()

	// Discriminator value is not a string
	value := map[string]any{
		"id": 123, // Integer instead of string
	}

	err := union.Validate(ctx, value)
	if err == nil {
		t.Error("Expected error for non-string discriminator, got nil")
	}
}

func TestDiscriminatedUnionNonObjectValue(t *testing.T) {
	type TypeA struct {
		Value string `json:"value"`
	}

	union := NewDiscriminatedUnion("type", map[string]any{
		"a": TypeA{},
	})
	ctx := NewValidationContext()

	// Value is not an object
	err := union.Validate(ctx, "not an object")
	if err == nil {
		t.Error("Expected error for non-object value, got nil")
	}
}

func TestUnionEmptyTypes(t *testing.T) {
	union := &Union{
		Types:        []reflect.Type{},
		SelectedType: -1,
	}
	ctx := NewValidationContext()

	err := union.Validate(ctx, "value")
	if err == nil {
		t.Error("Expected error for union with no types, got nil")
	}
}
