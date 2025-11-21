package validator

import (
	"testing"
)

func TestListOf(t *testing.T) {
	type User struct {
		Name string `json:"name" validate:"required"`
		Age  int    `json:"age" validate:"gte=0"`
	}

	tests := []struct {
		name    string
		list    *ListOf
		value   any
		wantErr bool
		wantLen int
	}{
		{
			name: "valid list of primitives",
			list: NewListOf(""),
			value: []any{
				"hello",
				"world",
			},
			wantErr: false,
			wantLen: 2,
		},
		{
			name: "valid list of structs",
			list: NewListOf(User{}),
			value: []any{
				map[string]any{
					"name": "Alice",
					"age":  30,
				},
				map[string]any{
					"name": "Bob",
					"age":  25,
				},
			},
			wantErr: false,
			wantLen: 2,
		},
		{
			name: "list with min items - valid",
			list: NewListOf("").WithMinItems(2),
			value: []any{
				"hello",
				"world",
			},
			wantErr: false,
			wantLen: 2,
		},
		{
			name: "list with min items - invalid",
			list: NewListOf("").WithMinItems(3),
			value: []any{
				"hello",
			},
			wantErr: true,
		},
		{
			name: "list with max items - valid",
			list: NewListOf("").WithMaxItems(3),
			value: []any{
				"hello",
				"world",
			},
			wantErr: false,
			wantLen: 2,
		},
		{
			name: "list with max items - invalid",
			list: NewListOf("").WithMaxItems(1),
			value: []any{
				"hello",
				"world",
			},
			wantErr: true,
		},
		{
			name:    "not a list",
			list:    NewListOf(""),
			value:   "not a list",
			wantErr: true,
		},
		{
			name: "list with invalid items",
			list: NewListOf(User{}),
			value: []any{
				map[string]any{
					// Missing required name field
					"age": 30,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewValidationContext()
			err := tt.list.Validate(ctx, tt.value)

			if (err != nil) != tt.wantErr {
				t.Errorf("ListOf.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(tt.list.Get()) != tt.wantLen {
				t.Errorf("ListOf.Get() length = %d, want %d", len(tt.list.Get()), tt.wantLen)
			}
		})
	}
}

func TestDictOf(t *testing.T) {
	type Product struct {
		Name  string  `json:"name" validate:"required"`
		Price float64 `json:"price" validate:"gt=0"`
	}

	tests := []struct {
		name    string
		dict    *DictOf
		value   any
		wantErr bool
		wantLen int
	}{
		{
			name: "valid dict of primitives",
			dict: NewDictOf("", 0),
			value: map[string]any{
				"foo": 10,
				"bar": 20,
			},
			wantErr: false,
			wantLen: 2,
		},
		{
			name: "valid dict of structs",
			dict: NewDictOf("", Product{}),
			value: map[string]any{
				"apple": map[string]any{
					"name":  "Apple",
					"price": 1.99,
				},
				"banana": map[string]any{
					"name":  "Banana",
					"price": 0.99,
				},
			},
			wantErr: false,
			wantLen: 2,
		},
		{
			name: "dict with min items - valid",
			dict: NewDictOf("", "").WithMinItems(2),
			value: map[string]any{
				"foo": "bar",
				"baz": "qux",
			},
			wantErr: false,
			wantLen: 2,
		},
		{
			name: "dict with min items - invalid",
			dict: NewDictOf("", "").WithMinItems(3),
			value: map[string]any{
				"foo": "bar",
			},
			wantErr: true,
		},
		{
			name: "dict with max items - valid",
			dict: NewDictOf("", "").WithMaxItems(3),
			value: map[string]any{
				"foo": "bar",
			},
			wantErr: false,
			wantLen: 1,
		},
		{
			name: "dict with max items - invalid",
			dict: NewDictOf("", "").WithMaxItems(1),
			value: map[string]any{
				"foo": "bar",
				"baz": "qux",
			},
			wantErr: true,
		},
		{
			name:    "not a dict",
			dict:    NewDictOf("", ""),
			value:   []any{"not", "a", "dict"},
			wantErr: true,
		},
		{
			name: "dict with invalid values",
			dict: NewDictOf("", Product{}),
			value: map[string]any{
				"apple": map[string]any{
					// Missing required name field
					"price": 1.99,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewValidationContext()
			err := tt.dict.Validate(ctx, tt.value)

			if (err != nil) != tt.wantErr {
				t.Errorf("DictOf.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(tt.dict.Get()) != tt.wantLen {
				t.Errorf("DictOf.Get() length = %d, want %d", len(tt.dict.Get()), tt.wantLen)
			}
		})
	}
}

func TestOptional(t *testing.T) {
	type User struct {
		Name string `json:"name" validate:"required"`
		Age  int    `json:"age" validate:"gte=0"`
	}

	tests := []struct {
		name      string
		optional  *Optional
		value     any
		wantErr   bool
		wantSome  bool
		wantValue any
	}{
		{
			name:      "optional with value - primitive",
			optional:  NewOptional(""),
			value:     "hello",
			wantErr:   false,
			wantSome:  true,
			wantValue: "hello",
		},
		{
			name:     "optional without value",
			optional: NewOptional(""),
			value:    nil,
			wantErr:  false,
			wantSome: false,
		},
		{
			name:     "optional with struct - valid",
			optional: NewOptional(User{}),
			value: map[string]any{
				"name": "Alice",
				"age":  30,
			},
			wantErr:  false,
			wantSome: true,
		},
		{
			name:     "optional with struct - invalid",
			optional: NewOptional(User{}),
			value: map[string]any{
				// Missing required name field
				"age": 30,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewValidationContext()
			err := tt.optional.Validate(ctx, tt.value)

			if (err != nil) != tt.wantErr {
				t.Errorf("Optional.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.optional.IsSome() != tt.wantSome {
				t.Errorf("Optional.IsSome() = %v, want %v", tt.optional.IsSome(), tt.wantSome)
			}

			if !tt.wantErr && tt.wantSome && tt.wantValue != nil {
				if tt.optional.Get() != tt.wantValue {
					t.Errorf("Optional.Get() = %v, want %v", tt.optional.Get(), tt.wantValue)
				}
			}

			if !tt.wantErr && !tt.wantSome {
				if !tt.optional.IsNone() {
					t.Errorf("Optional.IsNone() = false, want true")
				}
			}
		})
	}
}

func TestValidateListGeneric(t *testing.T) {
	type User struct {
		Name string `json:"name" validate:"required"`
		Age  int    `json:"age" validate:"gte=0"`
	}

	ctx := NewValidationContext()
	value := []any{
		map[string]any{
			"name": "Alice",
			"age":  30,
		},
		map[string]any{
			"name": "Bob",
			"age":  25,
		},
	}

	users, err := ValidateList[User](ctx, value, func(data map[string]any) (*User, error) {
		return Validate[User](data)
	})

	if err != nil {
		t.Fatalf("ValidateList() error = %v", err)
	}

	if len(users) != 2 {
		t.Errorf("ValidateList() length = %d, want 2", len(users))
	}

	if users[0].Name != "Alice" {
		t.Errorf("users[0].Name = %s, want 'Alice'", users[0].Name)
	}

	if users[1].Name != "Bob" {
		t.Errorf("users[1].Name = %s, want 'Bob'", users[1].Name)
	}
}

func TestValidateDictGeneric(t *testing.T) {
	type Product struct {
		Name  string  `json:"name" validate:"required"`
		Price float64 `json:"price" validate:"gt=0"`
	}

	ctx := NewValidationContext()
	value := map[string]any{
		"apple": map[string]any{
			"name":  "Apple",
			"price": 1.99,
		},
		"banana": map[string]any{
			"name":  "Banana",
			"price": 0.99,
		},
	}

	products, err := ValidateDict[Product](ctx, value, func(data map[string]any) (*Product, error) {
		return Validate[Product](data)
	})

	if err != nil {
		t.Fatalf("ValidateDict() error = %v", err)
	}

	if len(products) != 2 {
		t.Errorf("ValidateDict() length = %d, want 2", len(products))
	}

	if products["apple"].Name != "Apple" {
		t.Errorf("products['apple'].Name = %s, want 'Apple'", products["apple"].Name)
	}

	if products["banana"].Price != 0.99 {
		t.Errorf("products['banana'].Price = %f, want 0.99", products["banana"].Price)
	}
}

func TestValidateOptionalGeneric(t *testing.T) {
	type User struct {
		Name string `json:"name" validate:"required"`
		Age  int    `json:"age" validate:"gte=0"`
	}

	ctx := NewValidationContext()

	// Test with value
	value := map[string]any{
		"name": "Alice",
		"age":  30,
	}

	user, err := ValidateOptional[User](ctx, value, func(data map[string]any) (*User, error) {
		return Validate[User](data)
	})

	if err != nil {
		t.Fatalf("ValidateOptional() error = %v", err)
	}

	if user == nil {
		t.Fatal("ValidateOptional() returned nil for non-nil input")
		return
	}

	if user.Name != "Alice" {
		t.Errorf("user.Name = %s, want 'Alice'", user.Name)
	}

	// Test with nil
	nilUser, err := ValidateOptional[User](ctx, nil, func(data map[string]any) (*User, error) {
		return Validate[User](data)
	})

	if err != nil {
		t.Fatalf("ValidateOptional() with nil error = %v", err)
	}

	if nilUser != nil {
		t.Errorf("ValidateOptional() with nil = %v, want nil", nilUser)
	}
}
