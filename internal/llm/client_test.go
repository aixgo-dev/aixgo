package llm

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aixgo-dev/aixgo/internal/llm/provider"
)

func TestCreateStructured(t *testing.T) {
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
		Age   int    `json:"age" validate:"gte=0,lte=120"`
	}

	ctx := context.Background()

	// Setup mock provider
	mock := provider.NewMockProvider("test")
	userData := map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
		"age":   30,
	}
	mock.AddStructuredResponse(provider.MockStructuredResponse(userData))

	// Create client
	client := NewClient(mock, ClientConfig{
		DefaultModel:       "test-model",
		DefaultTemperature: 0.7,
		MaxRetries:         3,
	})

	// Test successful creation
	user, err := CreateStructured[User](ctx, client, "Create a user", nil)
	if err != nil {
		t.Fatalf("CreateStructured() error = %v", err)
	}

	if user.Name != "Alice" {
		t.Errorf("User.Name = %s, want 'Alice'", user.Name)
	}

	if user.Email != "alice@example.com" {
		t.Errorf("User.Email = %s, want 'alice@example.com'", user.Email)
	}

	if user.Age != 30 {
		t.Errorf("User.Age = %d, want 30", user.Age)
	}

	// Verify the mock provider was called
	if len(mock.StructuredCalls) != 1 {
		t.Errorf("Provider calls = %d, want 1", len(mock.StructuredCalls))
	}
}

func TestCreateStructured_ValidationError(t *testing.T) {
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
		Age   int    `json:"age" validate:"gte=0,lte=120"`
	}

	ctx := context.Background()

	// Setup mock provider with invalid data
	mock := provider.NewMockProvider("test")
	invalidData := map[string]any{
		"name":  "Bob",
		"email": "not-an-email", // Invalid email
		"age":   30,
	}
	mock.AddStructuredResponse(provider.MockStructuredResponse(invalidData))

	client := NewClient(mock, ClientConfig{
		DefaultModel: "test-model",
	})

	// Test validation error
	_, err := CreateStructured[User](ctx, client, "Create a user", nil)
	if err == nil {
		t.Fatal("CreateStructured() error = nil, want validation error")
	}
}

func TestCreateStructured_WithOptions(t *testing.T) {
	type Product struct {
		Name  string  `json:"name" validate:"required"`
		Price float64 `json:"price" validate:"gt=0"`
	}

	ctx := context.Background()

	// Setup mock provider
	mock := provider.NewMockProvider("test")
	productData := map[string]any{
		"name":  "Widget",
		"price": 19.99,
	}
	mock.AddStructuredResponse(provider.MockStructuredResponse(productData))

	client := NewClient(mock, ClientConfig{
		DefaultModel: "default-model",
	})

	// Test with options
	product, err := CreateStructured[Product](ctx, client, "Create a product", &CreateOptions{
		Model:        "custom-model",
		Temperature:  0.5,
		MaxTokens:    1000,
		SystemPrompt: "You are a helpful assistant.",
	})

	if err != nil {
		t.Fatalf("CreateStructured() error = %v", err)
	}

	if product.Name != "Widget" {
		t.Errorf("Product.Name = %s, want 'Widget'", product.Name)
	}

	// Verify options were used
	if len(mock.StructuredCalls) != 1 {
		t.Fatal("Provider was not called")
	}

	call := mock.StructuredCalls[0]
	if call.Model != "custom-model" {
		t.Errorf("Request.Model = %s, want 'custom-model'", call.Model)
	}

	if call.Temperature != 0.5 {
		t.Errorf("Request.Temperature = %f, want 0.5", call.Temperature)
	}

	if call.MaxTokens != 1000 {
		t.Errorf("Request.MaxTokens = %d, want 1000", call.MaxTokens)
	}

	// Check system prompt was added
	if len(call.Messages) < 2 {
		t.Fatal("Messages length < 2")
	}

	if call.Messages[0].Role != "system" {
		t.Errorf("First message role = %s, want 'system'", call.Messages[0].Role)
	}
}

func TestCreateList(t *testing.T) {
	type Item struct {
		Name string `json:"name" validate:"required"`
		ID   int    `json:"id" validate:"gt=0"`
	}

	ctx := context.Background()

	// Setup mock provider
	mock := provider.NewMockProvider("test")
	listData := []map[string]any{
		{"name": "Item1", "id": 1},
		{"name": "Item2", "id": 2},
		{"name": "Item3", "id": 3},
	}
	mock.AddStructuredResponse(provider.MockStructuredResponse(listData))

	client := NewClient(mock, ClientConfig{
		DefaultModel: "test-model",
	})

	// Test successful list creation
	items, err := CreateList[Item](ctx, client, "Create a list of items", nil)
	if err != nil {
		t.Fatalf("CreateList() error = %v", err)
	}

	if len(items) != 3 {
		t.Errorf("Items length = %d, want 3", len(items))
	}

	if items[0].Name != "Item1" {
		t.Errorf("items[0].Name = %s, want 'Item1'", items[0].Name)
	}

	if items[1].ID != 2 {
		t.Errorf("items[1].ID = %d, want 2", items[1].ID)
	}
}

func TestCreateCompletion(t *testing.T) {
	ctx := context.Background()

	// Setup mock provider
	mock := provider.NewMockProvider("test")
	mock.AddCompletionResponse(&provider.CompletionResponse{
		Content:      "This is a test response",
		FinishReason: "stop",
	})

	client := NewClient(mock, ClientConfig{
		DefaultModel: "test-model",
	})

	// Test completion
	response, err := CreateCompletion(ctx, client, "Say hello", nil)
	if err != nil {
		t.Fatalf("CreateCompletion() error = %v", err)
	}

	if response != "This is a test response" {
		t.Errorf("Response = %s, want 'This is a test response'", response)
	}

	// Verify the mock provider was called
	if len(mock.CompletionCalls) != 1 {
		t.Errorf("Provider calls = %d, want 1", len(mock.CompletionCalls))
	}
}

func TestCreateStructured_TypeCoercion(t *testing.T) {
	type Data struct {
		Count int    `json:"count"`
		Value string `json:"value"`
	}

	ctx := context.Background()

	// Setup mock provider - return string "42" for count (will be coerced to int)
	mock := provider.NewMockProvider("test")
	responseData := map[string]any{
		"count": "42", // String instead of int
		"value": "test",
	}
	mock.AddStructuredResponse(provider.MockStructuredResponse(responseData))

	// Create client WITHOUT strict validation
	client := NewClient(mock, ClientConfig{
		DefaultModel:     "test-model",
		StrictValidation: false,
	})

	// Test with type coercion (should succeed)
	data, err := CreateStructured[Data](ctx, client, "Create data", nil)
	if err != nil {
		t.Fatalf("CreateStructured() error = %v (type coercion should work)", err)
	}

	if data.Count != 42 {
		t.Errorf("Data.Count = %d, want 42", data.Count)
	}

	// Test with strict validation (should fail)
	mock.Reset()
	mock.AddStructuredResponse(provider.MockStructuredResponse(responseData))

	clientStrict := NewClient(mock, ClientConfig{
		DefaultModel:     "test-model",
		StrictValidation: true,
	})

	_, err = CreateStructured[Data](ctx, clientStrict, "Create data", nil)
	if err == nil {
		t.Error("CreateStructured() with strict validation error = nil, want error for type mismatch")
	}
}

func TestNewClientWithProvider(t *testing.T) {
	// Register a mock provider
	mock := provider.NewMockProvider("test-provider")
	provider.Register("test-provider", mock)

	// Test creating client with provider name
	client, err := NewClientWithProvider("test-provider", ClientConfig{
		DefaultModel: "test-model",
	})

	if err != nil {
		t.Fatalf("NewClientWithProvider() error = %v", err)
	}

	if client == nil {
		t.Fatal("Client is nil")
	}

	// Test with non-existent provider
	_, err = NewClientWithProvider("non-existent", ClientConfig{})
	if err == nil {
		t.Error("NewClientWithProvider() with non-existent provider error = nil, want error")
	}
}

func TestCreateStructured_ProviderError(t *testing.T) {
	type User struct {
		Name string `json:"name"`
	}

	ctx := context.Background()

	// Setup mock provider with error
	mock := provider.NewMockProvider("test")
	mock.AddError(provider.NewProviderError("test", provider.ErrorCodeRateLimit, "Rate limited", nil))

	client := NewClient(mock, ClientConfig{
		DefaultModel: "test-model",
	})

	// Test provider error propagation
	_, err := CreateStructured[User](ctx, client, "Create a user", nil)
	if err == nil {
		t.Fatal("CreateStructured() error = nil, want provider error")
	}
}

func TestCreateList_InvalidItem(t *testing.T) {
	type Item struct {
		Name string `json:"name" validate:"required"`
		ID   int    `json:"id" validate:"gt=0"`
	}

	ctx := context.Background()

	// Setup mock provider with one invalid item
	mock := provider.NewMockProvider("test")
	listData := []map[string]any{
		{"name": "Item1", "id": 1},
		{"name": "Item2", "id": -5}, // Invalid - ID must be > 0
	}
	mock.AddStructuredResponse(provider.MockStructuredResponse(listData))

	client := NewClient(mock, ClientConfig{
		DefaultModel: "test-model",
	})

	// Test validation error in list
	_, err := CreateList[Item](ctx, client, "Create items", nil)
	if err == nil {
		t.Fatal("CreateList() error = nil, want validation error for item 1")
	}
}

func TestCreateList_NotAnArray(t *testing.T) {
	type Item struct {
		Name string `json:"name"`
	}

	ctx := context.Background()

	// Setup mock provider returning an object instead of an array
	mock := provider.NewMockProvider("test")
	notArrayData := map[string]any{
		"name": "Not an array",
	}

	jsonData, _ := json.Marshal(notArrayData)
	mock.AddStructuredResponse(&provider.StructuredResponse{
		Data: jsonData,
	})

	client := NewClient(mock, ClientConfig{
		DefaultModel: "test-model",
	})

	// Test error when response is not an array
	_, err := CreateList[Item](ctx, client, "Create items", nil)
	if err == nil {
		t.Fatal("CreateList() error = nil, want error for non-array response")
	}
}
