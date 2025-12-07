package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aixgo-dev/aixgo/internal/llm"
	"github.com/aixgo-dev/aixgo/internal/llm/provider"
)

// User represents validated user data with strict validation rules
type User struct {
	Name  string `json:"name" validate:"required,min=1,max=100"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"required,min=1,max=150"`
	City  string `json:"city" validate:"required,min=1"`
}

// Product represents a product with validation constraints
type Product struct {
	Name        string  `json:"name" validate:"required,min=1"`
	Description string  `json:"description" validate:"required,min=10"`
	Price       float64 `json:"price" validate:"required,gt=0"`
	InStock     bool    `json:"in_stock"`
}

func main() {
	fmt.Println("=== Pydantic AI-Style Validation Retry Demo ===\n")

	// Example 1: Default behavior - automatic retry on validation failure
	fmt.Println("1. Default Behavior (Auto-retry enabled)")
	fmt.Println("   Prompt: 'Extract user: John is 25'")
	fmt.Println("   Expected: LLM might miss 'email' field initially, but auto-retry will ask for it")
	demoDefaultBehavior()

	fmt.Println("\n" + strings.Repeat("-", 60) + "\n")

	// Example 2: Opt-out - immediate failure without retry
	fmt.Println("2. Opt-Out Behavior (Retry disabled)")
	fmt.Println("   Same prompt, but with DisableValidationRetry=true")
	fmt.Println("   Expected: Immediate failure if validation fails")
	demoOptOutBehavior()

	fmt.Println("\n" + strings.Repeat("-", 60) + "\n")

	// Example 3: Complex validation - product extraction
	fmt.Println("3. Complex Product Validation")
	fmt.Println("   Demonstrating validation with multiple required fields")
	demoComplexValidation()
}

func demoDefaultBehavior() {
	ctx := context.Background()

	// Get provider from environment (default: mock for demo)
	providerName := os.Getenv("PROVIDER")
	if providerName == "" {
		providerName = "mock"
	}

	prov, err := provider.Get(providerName)
	if err != nil {
		log.Fatal(err)
	}

	// Create client with default settings - validation retry is ENABLED by default
	client := llm.NewClient(prov, llm.ClientConfig{
		DefaultModel:     getModel(),
		StrictValidation: true,
		// MaxRetries defaults to 3 - Pydantic AI-style behavior out-of-the-box!
	})

	// Prompt that may produce incomplete data
	prompt := "Extract user information: John is 25 years old"

	// The LLM might initially miss the 'email' field
	// With auto-retry: validation fails → retry with error feedback → LLM corrects → success
	result, err := llm.CreateStructured[User](
		ctx,
		client,
		prompt,
		&llm.CreateOptions{
			SystemPrompt: `You are a data extraction assistant.
Extract user information and return it as JSON with fields: name, email, age, city.
If information is missing from the input, make reasonable assumptions.`,
		},
	)

	if err != nil {
		fmt.Printf("   ❌ Error: %v\n", err)
		fmt.Println("   Note: In a real scenario with a proper LLM, auto-retry would succeed")
		return
	}

	fmt.Printf("   ✅ Success!\n")
	fmt.Printf("   User: %+v\n", result)
	fmt.Printf("   With auto-retry, validation errors are automatically corrected\n")
}

func demoOptOutBehavior() {
	ctx := context.Background()

	providerName := os.Getenv("PROVIDER")
	if providerName == "" {
		providerName = "mock"
	}

	prov, err := provider.Get(providerName)
	if err != nil {
		log.Fatal(err)
	}

	// Create client with retry DISABLED
	client := llm.NewClient(prov, llm.ClientConfig{
		DefaultModel:           getModel(),
		StrictValidation:       true,
		DisableValidationRetry: true, // Opt-out: fail immediately on validation error
	})

	prompt := "Extract user information: John is 25 years old"

	result, err := llm.CreateStructured[User](
		ctx,
		client,
		prompt,
		&llm.CreateOptions{
			SystemPrompt: `You are a data extraction assistant.
Extract user information and return it as JSON with fields: name, email, age, city.`,
		},
	)

	if err != nil {
		fmt.Printf("   ❌ Validation failed immediately (as expected):\n")
		fmt.Printf("   Error: %v\n", err)
		fmt.Println("   No retry attempted - immediate failure")
		return
	}

	fmt.Printf("   ✅ Success: %+v\n", result)
}

func demoComplexValidation() {
	ctx := context.Background()

	providerName := os.Getenv("PROVIDER")
	if providerName == "" {
		providerName = "mock"
	}

	prov, err := provider.Get(providerName)
	if err != nil {
		log.Fatal(err)
	}

	client := llm.NewClient(prov, llm.ClientConfig{
		DefaultModel:     getModel(),
		StrictValidation: true,
		MaxRetries:       3, // Allow up to 3 attempts
	})

	prompt := "Create a product listing for a premium laptop"

	result, err := llm.CreateStructured[Product](
		ctx,
		client,
		prompt,
		&llm.CreateOptions{
			SystemPrompt: `You are a product catalog assistant.
Create detailed product information with:
- name: product name (required)
- description: at least 10 characters (required)
- price: must be > 0 (required)
- in_stock: availability (boolean)`,
		},
	)

	if err != nil {
		fmt.Printf("   ❌ Error after retries: %v\n", err)
		return
	}

	fmt.Printf("   ✅ Product created successfully:\n")
	fmt.Printf("   Name: %s\n", result.Name)
	fmt.Printf("   Description: %s\n", result.Description)
	fmt.Printf("   Price: $%.2f\n", result.Price)
	fmt.Printf("   In Stock: %v\n", result.InStock)
}

func getModel() string {
	model := os.Getenv("MODEL")
	if model == "" {
		model = "gpt-4"
	}
	return model
}
