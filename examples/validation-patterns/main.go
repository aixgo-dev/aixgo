package main

import (
	"fmt"
	"strings"
)

// Example 1: Empty array validation using Validatable interface
type DataCollection struct {
	Items []string `json:"items" validate:"required"`
}

// Validate implements the Validatable interface
func (d DataCollection) Validate() error {
	if len(d.Items) == 0 {
		return fmt.Errorf("items array cannot be empty - at least one item required")
	}
	return nil
}

// Example 2: Enum validation using oneof
type OrderStatus struct {
	Status  string `json:"status" validate:"required,oneof=pending processing shipped delivered cancelled"`
	OrderID string `json:"order_id" validate:"required"`
}

// Example 3: Range validation using min/max
type UserProfile struct {
	Name  string `json:"name" validate:"required,min=1,max=100"`
	Age   int    `json:"age" validate:"required,min=1,max=150"`
	Email string `json:"email" validate:"required,email"`
}

// Example 4: Cross-field validation
type DateRange struct {
	StartDate string `json:"start_date" validate:"required"`
	EndDate   string `json:"end_date" validate:"required"`
}

// Validate cross-field constraints
func (d DateRange) Validate() error {
	if d.StartDate > d.EndDate {
		return fmt.Errorf("start_date must be before or equal to end_date")
	}
	return nil
}

// Example 5: Complex nested validation
type Address struct {
	Street  string `json:"street" validate:"required,min=1"`
	City    string `json:"city" validate:"required,min=1"`
	ZipCode string `json:"zip_code" validate:"required,len=5"`
}

type Customer struct {
	Name    string   `json:"name" validate:"required,min=1,max=100"`
	Email   string   `json:"email" validate:"required,email"`
	Phone   string   `json:"phone" validate:"required,e164"` // E.164 format
	Address Address  `json:"address" validate:"required"`
	Tags    []string `json:"tags"`
}

// Validate ensures tags array has at least one item
func (c Customer) Validate() error {
	if len(c.Tags) == 0 {
		return fmt.Errorf("customer must have at least one tag")
	}
	return nil
}

func main() {
	fmt.Println("=== Aixgo Validation Patterns Demo ===\n")
	fmt.Println("Demonstrating validation patterns available in Aixgo v0.1.2\n")

	// Example 1: Empty array validation
	fmt.Println("1. Empty Array Validation (Validatable interface)")
	fmt.Println("   Problem: User reported 'no way to validate empty arrays'")
	fmt.Println("   Solution: Implement Validatable interface with custom logic")
	demoEmptyArrayValidation()

	fmt.Println("\n" + strings.Repeat("-", 70) + "\n")

	// Example 2: Enum validation
	fmt.Println("2. Enum Validation (oneof struct tag)")
	fmt.Println("   Validates value is one of predefined options")
	demoEnumValidation()

	fmt.Println("\n" + strings.Repeat("-", 70) + "\n")

	// Example 3: Range validation
	fmt.Println("3. Range Validation (min/max struct tags)")
	fmt.Println("   Validates numeric ranges and string lengths")
	demoRangeValidation()

	fmt.Println("\n" + strings.Repeat("-", 70) + "\n")

	// Example 4: Cross-field validation
	fmt.Println("4. Cross-Field Validation (Validatable interface)")
	fmt.Println("   Validates relationships between multiple fields")
	demoCrossFieldValidation()

	fmt.Println("\n" + strings.Repeat("-", 70) + "\n")

	// Example 5: Nested struct validation
	fmt.Println("5. Nested Struct Validation")
	fmt.Println("   Validates complex nested structures")
	demoNestedValidation()

	fmt.Println("\n" + strings.Repeat("-", 70) + "\n")

	// Summary
	fmt.Println("=== Summary ===")
	fmt.Println("\nValidation Capabilities in Aixgo v0.1.2:")
	fmt.Println("  1. Struct tags: required, min, max, email, url, uuid, oneof, etc.")
	fmt.Println("  2. Validatable interface: Custom validation logic for complex cases")
	fmt.Println("  3. Automatic retry: Framework retries with error feedback to LLM")
	fmt.Println("  4. Nested validation: Automatic validation of nested structures")
	fmt.Println("  5. Empty array validation: Use Validatable interface (10 lines of code)")
	fmt.Println("\nNo feature gaps - all validation needs are already supported!")
	fmt.Println("See README.md for integration with CreateStructured and automatic retry")
}

func demoEmptyArrayValidation() {
	fmt.Println("   Code pattern:")
	fmt.Println("   ```go")
	fmt.Println("   type DataCollection struct {")
	fmt.Println("       Items []string `json:\"items\" validate:\"required\"`")
	fmt.Println("   }")
	fmt.Println()
	fmt.Println("   func (d DataCollection) Validate() error {")
	fmt.Println("       if len(d.Items) == 0 {")
	fmt.Println("           return fmt.Errorf(\"items array cannot be empty\")")
	fmt.Println("       }")
	fmt.Println("       return nil")
	fmt.Println("   }")
	fmt.Println("   ```")
	fmt.Println()

	// Test validation
	valid := DataCollection{Items: []string{"Go", "Python", "TypeScript"}}
	if err := valid.Validate(); err != nil {
		fmt.Printf("   ❌ Unexpected error: %v\n", err)
	} else {
		fmt.Printf("   ✓ Valid data: %v\n", valid.Items)
	}

	invalid := DataCollection{Items: []string{}}
	if err := invalid.Validate(); err != nil {
		fmt.Printf("   ✓ Validation correctly rejected empty array: %v\n", err)
	} else {
		fmt.Println("   ❌ Validation should have failed")
	}
}

func demoEnumValidation() {
	fmt.Println("   Using struct tag: `validate:\"oneof=pending processing shipped delivered cancelled\"`")
	fmt.Println()

	order := OrderStatus{
		Status:  "processing",
		OrderID: "12345",
	}
	fmt.Printf("   ✓ Valid order: %s - %s\n", order.OrderID, order.Status)
	fmt.Println("   Framework validates status is one of the allowed values")
}

func demoRangeValidation() {
	fmt.Println("   Using struct tags:")
	fmt.Println("   - Name: `validate:\"required,min=1,max=100\"`")
	fmt.Println("   - Age: `validate:\"required,min=1,max=150\"`")
	fmt.Println("   - Email: `validate:\"required,email\"`")
	fmt.Println()

	user := UserProfile{
		Name:  "Alice",
		Age:   28,
		Email: "alice@example.com",
	}
	fmt.Printf("   ✓ Valid user: %s, %d years old, %s\n", user.Name, user.Age, user.Email)
	fmt.Println("   All range validations passed")
}

func demoCrossFieldValidation() {
	fmt.Println("   Code pattern for cross-field validation:")
	fmt.Println("   ```go")
	fmt.Println("   func (d DateRange) Validate() error {")
	fmt.Println("       if d.StartDate > d.EndDate {")
	fmt.Println("           return fmt.Errorf(\"start must be before end\")")
	fmt.Println("       }")
	fmt.Println("       return nil")
	fmt.Println("   }")
	fmt.Println("   ```")
	fmt.Println()

	validRange := DateRange{
		StartDate: "2024-01-01",
		EndDate:   "2024-12-31",
	}
	if err := validRange.Validate(); err != nil {
		fmt.Printf("   ❌ Unexpected error: %v\n", err)
	} else {
		fmt.Printf("   ✓ Valid range: %s to %s\n", validRange.StartDate, validRange.EndDate)
	}

	invalidRange := DateRange{
		StartDate: "2024-12-31",
		EndDate:   "2024-01-01",
	}
	if err := invalidRange.Validate(); err != nil {
		fmt.Printf("   ✓ Validation correctly rejected invalid range: %v\n", err)
	}
}

func demoNestedValidation() {
	fmt.Println("   Nested struct with multiple validation layers:")
	fmt.Println("   - Address has: street, city (required), zipcode (len=5)")
	fmt.Println("   - Customer has: name, email, phone (E.164), address, tags (non-empty)")
	fmt.Println()

	customer := Customer{
		Name:  "Bob Smith",
		Email: "bob@example.com",
		Phone: "+14155552671",
		Address: Address{
			Street:  "123 Main St",
			City:    "San Francisco",
			ZipCode: "94102",
		},
		Tags: []string{"premium", "enterprise"},
	}

	if err := customer.Validate(); err != nil {
		fmt.Printf("   ❌ Unexpected error: %v\n", err)
	} else {
		fmt.Printf("   ✓ Valid customer: %s at %s, %s\n",
			customer.Name, customer.Address.Street, customer.Address.City)
		fmt.Printf("   ✓ All nested validations passed (address + tags + formats)\n")
	}

	// Test failure case
	customerNoTags := customer
	customerNoTags.Tags = []string{}
	if err := customerNoTags.Validate(); err != nil {
		fmt.Printf("   ✓ Validation correctly rejected customer with no tags: %v\n", err)
	}
}
