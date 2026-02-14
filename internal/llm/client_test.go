package llm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aixgo-dev/aixgo/pkg/llm/provider"
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

	// Test with strict validation (should fail immediately, no retry)
	mock.Reset()
	mock.AddStructuredResponse(provider.MockStructuredResponse(responseData))

	clientStrict := NewClient(mock, ClientConfig{
		DefaultModel:           "test-model",
		StrictValidation:       true,
		DisableValidationRetry: true, // Disable retry for this type coercion test
	})

	_, err = CreateStructured[Data](ctx, clientStrict, "Create data", nil)
	if err == nil {
		t.Error("CreateStructured() with strict validation error = nil, want error for type mismatch")
	}

	// Verify provider was called only once (no retry)
	if len(mock.StructuredCalls) != 1 {
		t.Errorf("Provider calls = %d, want 1 (retry disabled)", len(mock.StructuredCalls))
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

// ===== NEW VALIDATION RETRY TESTS =====

func TestCreateStructured_ValidationRetry_Success(t *testing.T) {
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
		Age   int    `json:"age" validate:"gte=0"`
	}

	ctx := context.Background()

	// Setup mock provider - first response invalid, second response valid
	mock := provider.NewMockProvider("test")

	// First attempt: missing email (invalid)
	invalidData := map[string]any{
		"name": "Alice",
		"age":  30,
	}
	mock.AddStructuredResponse(provider.MockStructuredResponse(invalidData))

	// Second attempt: valid
	validData := map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
		"age":   30,
	}
	mock.AddStructuredResponse(provider.MockStructuredResponse(validData))

	client := NewClient(mock, ClientConfig{
		DefaultModel: "test-model",
		MaxRetries:   3,
	})

	// Test automatic retry on validation failure
	user, err := CreateStructured[User](ctx, client, "Create a user", nil)
	if err != nil {
		t.Fatalf("CreateStructured() error = %v, want success after retry", err)
	}

	if user.Email != "alice@example.com" {
		t.Errorf("User.Email = %s, want 'alice@example.com' (should be corrected after retry)", user.Email)
	}

	// Verify provider was called twice (initial + 1 retry)
	if len(mock.StructuredCalls) != 2 {
		t.Errorf("Provider calls = %d, want 2 (initial + 1 retry)", len(mock.StructuredCalls))
	}

	// Verify feedback message was included in second call
	if len(mock.StructuredCalls) >= 2 {
		secondCall := mock.StructuredCalls[1]
		if len(secondCall.Messages) < 3 {
			t.Error("Second call should have at least 3 messages (system, user, assistant, user feedback)")
		}
		// Check that feedback mentions validation error
		lastMsg := secondCall.Messages[len(secondCall.Messages)-1]
		if lastMsg.Role != "user" {
			t.Errorf("Last message role = %s, want 'user' (feedback)", lastMsg.Role)
		}
	}
}

func TestCreateStructured_ValidationRetry_ExhaustedRetries(t *testing.T) {
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	ctx := context.Background()

	// Setup mock provider - always return invalid data
	mock := provider.NewMockProvider("test")
	invalidData := map[string]any{
		"name": "Bob",
		// Missing email - will fail validation
	}

	// Add 3 invalid responses (to exhaust retries)
	for i := 0; i < 3; i++ {
		mock.AddStructuredResponse(provider.MockStructuredResponse(invalidData))
	}

	client := NewClient(mock, ClientConfig{
		DefaultModel: "test-model",
		MaxRetries:   3,
	})

	// Test that error is returned after exhausting retries
	_, err := CreateStructured[User](ctx, client, "Create a user", nil)
	if err == nil {
		t.Fatal("CreateStructured() error = nil, want validation error after exhausting retries")
	}

	// Error message should mention number of attempts
	errMsg := err.Error()
	if !strings.Contains(errMsg, "validation failed after 3 attempts") {
		t.Errorf("Error message = %s, should mention '3 attempts'", errMsg)
	}

	// Verify provider was called 3 times (max retries)
	if len(mock.StructuredCalls) != 3 {
		t.Errorf("Provider calls = %d, want 3 (maxRetries)", len(mock.StructuredCalls))
	}
}

func TestCreateStructured_ValidationRetry_SuccessFirstAttempt(t *testing.T) {
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	ctx := context.Background()

	// Setup mock provider with valid data on first attempt
	mock := provider.NewMockProvider("test")
	validData := map[string]any{
		"name":  "Charlie",
		"email": "charlie@example.com",
	}
	mock.AddStructuredResponse(provider.MockStructuredResponse(validData))

	client := NewClient(mock, ClientConfig{
		DefaultModel: "test-model",
		MaxRetries:   3,
	})

	// Test no retry when first attempt succeeds
	user, err := CreateStructured[User](ctx, client, "Create a user", nil)
	if err != nil {
		t.Fatalf("CreateStructured() error = %v", err)
	}

	if user.Name != "Charlie" {
		t.Errorf("User.Name = %s, want 'Charlie'", user.Name)
	}

	// Verify provider was called only once (no retry needed)
	if len(mock.StructuredCalls) != 1 {
		t.Errorf("Provider calls = %d, want 1 (success on first attempt, no retry)", len(mock.StructuredCalls))
	}
}

func TestCreateStructured_DisableValidationRetry(t *testing.T) {
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	ctx := context.Background()

	// Setup mock provider with invalid data
	mock := provider.NewMockProvider("test")
	invalidData := map[string]any{
		"name": "Dave",
		// Missing email
	}
	mock.AddStructuredResponse(provider.MockStructuredResponse(invalidData))

	// Create client with retry disabled
	client := NewClient(mock, ClientConfig{
		DefaultModel:           "test-model",
		DisableValidationRetry: true,
		MaxRetries:             3, // Should be ignored when DisableValidationRetry is true
	})

	// Test immediate failure when retry is disabled
	_, err := CreateStructured[User](ctx, client, "Create a user", nil)
	if err == nil {
		t.Fatal("CreateStructured() error = nil, want validation error (retry disabled)")
	}

	// Verify provider was called only once (no retry)
	if len(mock.StructuredCalls) != 1 {
		t.Errorf("Provider calls = %d, want 1 (retry disabled)", len(mock.StructuredCalls))
	}
}

func TestCreateStructured_MaxRetriesSetToOne(t *testing.T) {
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	ctx := context.Background()

	// Setup mock provider with invalid data
	mock := provider.NewMockProvider("test")
	invalidData := map[string]any{
		"name": "Eve",
		// Missing email
	}
	mock.AddStructuredResponse(provider.MockStructuredResponse(invalidData))

	// Create client with MaxRetries=1 (effectively disables retry)
	client := NewClient(mock, ClientConfig{
		DefaultModel: "test-model",
		MaxRetries:   1,
	})

	// Test immediate failure when MaxRetries=1
	_, err := CreateStructured[User](ctx, client, "Create a user", nil)
	if err == nil {
		t.Fatal("CreateStructured() error = nil, want validation error")
	}

	// Verify provider was called only once
	if len(mock.StructuredCalls) != 1 {
		t.Errorf("Provider calls = %d, want 1 (MaxRetries=1)", len(mock.StructuredCalls))
	}
}

func TestCreateList_ValidationRetry_Success(t *testing.T) {
	type Item struct {
		Name string `json:"name" validate:"required"`
		ID   int    `json:"id" validate:"gt=0"`
	}

	ctx := context.Background()

	// Setup mock provider - first response invalid, second response valid
	mock := provider.NewMockProvider("test")

	// First attempt: item with invalid ID
	invalidList := []map[string]any{
		{"name": "Item1", "id": 1},
		{"name": "Item2", "id": -5}, // Invalid
	}
	mock.AddStructuredResponse(provider.MockStructuredResponse(invalidList))

	// Second attempt: all valid
	validList := []map[string]any{
		{"name": "Item1", "id": 1},
		{"name": "Item2", "id": 2}, // Corrected
	}
	mock.AddStructuredResponse(provider.MockStructuredResponse(validList))

	client := NewClient(mock, ClientConfig{
		DefaultModel: "test-model",
		MaxRetries:   3,
	})

	// Test automatic retry on list validation failure
	items, err := CreateList[Item](ctx, client, "Create items", nil)
	if err != nil {
		t.Fatalf("CreateList() error = %v, want success after retry", err)
	}

	if len(items) != 2 {
		t.Errorf("Items length = %d, want 2", len(items))
	}

	if items[1].ID != 2 {
		t.Errorf("items[1].ID = %d, want 2 (should be corrected after retry)", items[1].ID)
	}

	// Verify provider was called twice
	if len(mock.StructuredCalls) != 2 {
		t.Errorf("Provider calls = %d, want 2 (initial + 1 retry)", len(mock.StructuredCalls))
	}
}

func TestCreateStructured_DefaultMaxRetries(t *testing.T) {
	type User struct {
		Name string `json:"name" validate:"required"`
	}

	ctx := context.Background()

	// Setup mock provider
	mock := provider.NewMockProvider("test")
	validData := map[string]any{
		"name": "Test",
	}
	mock.AddStructuredResponse(provider.MockStructuredResponse(validData))

	// Create client without specifying MaxRetries (should default to 3)
	client := NewClient(mock, ClientConfig{
		DefaultModel: "test-model",
		// MaxRetries not specified - should default to 3
	})

	if client.config.MaxRetries != 3 {
		t.Errorf("Client.config.MaxRetries = %d, want 3 (default Pydantic AI-style behavior)", client.config.MaxRetries)
	}

	_, err := CreateStructured[User](ctx, client, "Create a user", nil)
	if err != nil {
		t.Fatalf("CreateStructured() error = %v", err)
	}
}
