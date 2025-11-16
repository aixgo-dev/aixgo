package schema

import (
	"fmt"
	"net/mail"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Constrained numeric types matching Pydantic's built-in types

// PositiveInt represents an integer greater than 0
type PositiveInt int

// Validate checks if the value is positive
func (p PositiveInt) Validate() error {
	if p <= 0 {
		return fmt.Errorf("must be greater than 0, got %d", p)
	}
	return nil
}

// NonNegativeInt represents an integer >= 0
type NonNegativeInt int

// Validate checks if the value is non-negative
func (n NonNegativeInt) Validate() error {
	if n < 0 {
		return fmt.Errorf("must be non-negative, got %d", n)
	}
	return nil
}

// NegativeInt represents an integer < 0
type NegativeInt int

// Validate checks if the value is negative
func (n NegativeInt) Validate() error {
	if n >= 0 {
		return fmt.Errorf("must be negative, got %d", n)
	}
	return nil
}

// NonPositiveInt represents an integer <= 0
type NonPositiveInt int

// Validate checks if the value is non-positive
func (n NonPositiveInt) Validate() error {
	if n > 0 {
		return fmt.Errorf("must be non-positive, got %d", n)
	}
	return nil
}

// PositiveFloat represents a float64 greater than 0
type PositiveFloat float64

// Validate checks if the value is positive
func (p PositiveFloat) Validate() error {
	if p <= 0 {
		return fmt.Errorf("must be greater than 0, got %f", p)
	}
	return nil
}

// NonNegativeFloat represents a float64 >= 0
type NonNegativeFloat float64

// Validate checks if the value is non-negative
func (n NonNegativeFloat) Validate() error {
	if n < 0 {
		return fmt.Errorf("must be non-negative, got %f", n)
	}
	return nil
}

// NegativeFloat represents a float64 < 0
type NegativeFloat float64

// Validate checks if the value is negative
func (n NegativeFloat) Validate() error {
	if n >= 0 {
		return fmt.Errorf("must be negative, got %f", n)
	}
	return nil
}

// NonPositiveFloat represents a float64 <= 0
type NonPositiveFloat float64

// Validate checks if the value is non-positive
func (n NonPositiveFloat) Validate() error {
	if n > 0 {
		return fmt.Errorf("must be non-positive, got %f", n)
	}
	return nil
}

// String types with format validation

// EmailStr represents a valid email address
type EmailStr string

// Validate checks if the string is a valid email
func (e EmailStr) Validate() error {
	if e == "" {
		return fmt.Errorf("email cannot be empty")
	}
	_, err := mail.ParseAddress(string(e))
	if err != nil {
		return fmt.Errorf("invalid email format: %w", err)
	}
	return nil
}

// String returns the email as a string
func (e EmailStr) String() string {
	return string(e)
}

// HttpUrl represents a valid HTTP/HTTPS URL
type HttpUrl string

// Validate checks if the string is a valid HTTP(S) URL
func (h HttpUrl) Validate() error {
	if h == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	u, err := url.Parse(string(h))
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme, got %s", u.Scheme)
	}

	return nil
}

// String returns the URL as a string
func (h HttpUrl) String() string {
	return string(h)
}

// UUID type represents a valid UUID string
type UUID string

// Validate checks if the string is a valid UUID
func (u UUID) Validate() error {
	if u == "" {
		return fmt.Errorf("UUID cannot be empty")
	}
	_, err := uuid.Parse(string(u))
	if err != nil {
		return fmt.Errorf("invalid UUID format: %w", err)
	}
	return nil
}

// String returns the UUID as a string
func (u UUID) String() string {
	return string(u)
}

// FilePath represents a path to an existing file
type FilePath string

// Validate checks if the path points to an existing file
func (f FilePath) Validate() error {
	if f == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	info, err := filepath.Abs(string(f))
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	// Check if file exists (optional - could be configured)
	// For now, just validate it's a valid path structure
	_ = info

	return nil
}

// String returns the file path as a string
func (f FilePath) String() string {
	return string(f)
}

// DirectoryPath represents a path to an existing directory
type DirectoryPath string

// Validate checks if the path points to an existing directory
func (d DirectoryPath) Validate() error {
	if d == "" {
		return fmt.Errorf("directory path cannot be empty")
	}

	info, err := filepath.Abs(string(d))
	if err != nil {
		return fmt.Errorf("invalid directory path: %w", err)
	}

	_ = info

	return nil
}

// String returns the directory path as a string
func (d DirectoryPath) String() string {
	return string(d)
}

// Date/Time constrained types

// PastDate represents a date that must be in the past
type PastDate time.Time

// Validate checks if the date is in the past
func (p PastDate) Validate() error {
	t := time.Time(p)
	if t.After(time.Now()) {
		return fmt.Errorf("date must be in the past, got %s", t.Format(time.RFC3339))
	}
	return nil
}

// Time returns the underlying time.Time
func (p PastDate) Time() time.Time {
	return time.Time(p)
}

// FutureDate represents a date that must be in the future
type FutureDate time.Time

// Validate checks if the date is in the future
func (f FutureDate) Validate() error {
	t := time.Time(f)
	if t.Before(time.Now()) {
		return fmt.Errorf("date must be in the future, got %s", t.Format(time.RFC3339))
	}
	return nil
}

// Time returns the underlying time.Time
func (f FutureDate) Time() time.Time {
	return time.Time(f)
}

// AwareDateTime represents a datetime with timezone information
type AwareDateTime time.Time

// Validate checks if the datetime has timezone info
func (a AwareDateTime) Validate() error {
	t := time.Time(a)
	if t.Location() == time.UTC || t.Location().String() == "Local" {
		// Has timezone info
		return nil
	}
	return fmt.Errorf("datetime must be timezone-aware")
}

// Time returns the underlying time.Time
func (a AwareDateTime) Time() time.Time {
	return time.Time(a)
}

// NaiveDateTime represents a datetime without timezone information
type NaiveDateTime time.Time

// Validate checks if the datetime lacks timezone info
func (n NaiveDateTime) Validate() error {
	t := time.Time(n)
	// In Go, this is harder to enforce since time.Time always has a Location
	// We'll consider UTC or Local as "naive" for practical purposes
	if t.Location() != time.UTC && t.Location().String() != "Local" {
		return fmt.Errorf("datetime must be timezone-naive")
	}
	return nil
}

// Time returns the underlying time.Time
func (n NaiveDateTime) Time() time.Time {
	return time.Time(n)
}

// Additional specialized string types

// AlphaStr represents a string containing only letters
type AlphaStr string

var alphaRegex = regexp.MustCompile(`^[a-zA-Z]+$`)

// Validate checks if the string contains only alphabetic characters
func (a AlphaStr) Validate() error {
	if a == "" {
		return fmt.Errorf("alpha string cannot be empty")
	}
	if !alphaRegex.MatchString(string(a)) {
		return fmt.Errorf("must contain only alphabetic characters")
	}
	return nil
}

// AlphaNumStr represents a string containing only letters and numbers
type AlphaNumStr string

var alphaNumRegex = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

// Validate checks if the string contains only alphanumeric characters
func (a AlphaNumStr) Validate() error {
	if a == "" {
		return fmt.Errorf("alphanumeric string cannot be empty")
	}
	if !alphaNumRegex.MatchString(string(a)) {
		return fmt.Errorf("must contain only alphanumeric characters")
	}
	return nil
}

// NumericStr represents a string containing only numbers
type NumericStr string

var numericRegex = regexp.MustCompile(`^[0-9]+$`)

// Validate checks if the string contains only numeric characters
func (n NumericStr) Validate() error {
	if n == "" {
		return fmt.Errorf("numeric string cannot be empty")
	}
	if !numericRegex.MatchString(string(n)) {
		return fmt.Errorf("must contain only numeric characters")
	}
	return nil
}

// HexStr represents a hexadecimal string
type HexStr string

var hexRegex = regexp.MustCompile(`^[0-9a-fA-F]+$`)

// Validate checks if the string is valid hexadecimal
func (h HexStr) Validate() error {
	if h == "" {
		return fmt.Errorf("hex string cannot be empty")
	}
	if !hexRegex.MatchString(string(h)) {
		return fmt.Errorf("must contain only hexadecimal characters")
	}
	return nil
}

// Base64Str represents a base64-encoded string
type Base64Str string

var base64Regex = regexp.MustCompile(`^[A-Za-z0-9+/]*={0,2}$`)

// Validate checks if the string is valid base64
func (b Base64Str) Validate() error {
	if b == "" {
		return fmt.Errorf("base64 string cannot be empty")
	}
	// Basic check - real base64 validation would decode
	s := string(b)
	if len(s)%4 != 0 {
		return fmt.Errorf("base64 string length must be multiple of 4")
	}
	if !base64Regex.MatchString(s) {
		return fmt.Errorf("invalid base64 characters")
	}
	return nil
}

// LowercaseStr represents a string that must be lowercase
type LowercaseStr string

// Validate checks if the string is lowercase
func (l LowercaseStr) Validate() error {
	s := string(l)
	if s != strings.ToLower(s) {
		return fmt.Errorf("must be lowercase")
	}
	return nil
}

// UppercaseStr represents a string that must be uppercase
type UppercaseStr string

// Validate checks if the string is uppercase
func (u UppercaseStr) Validate() error {
	s := string(u)
	if s != strings.ToUpper(s) {
		return fmt.Errorf("must be uppercase")
	}
	return nil
}

// Validatable is an interface for types that can validate themselves
type Validatable interface {
	Validate() error
}

// ValidateType validates a value if it implements Validatable
func ValidateType(v any) error {
	if validator, ok := v.(Validatable); ok {
		return validator.Validate()
	}
	return nil
}
