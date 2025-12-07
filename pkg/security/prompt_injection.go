// Package security provides security utilities for the aixgo framework.
package security

import (
	"encoding/base64"
	"regexp"
	"strings"
	"unicode"
)

// Sensitivity levels for prompt injection detection
type Sensitivity int

const (
	// SensitivityLow catches obvious injection attempts
	SensitivityLow Sensitivity = iota
	// SensitivityMedium catches moderate injection attempts
	SensitivityMedium
	// SensitivityHigh catches subtle injection attempts (may have higher false positives)
	SensitivityHigh
)

// DetectionCategory represents the type of injection attack detected
type DetectionCategory string

const (
	CategorySystemOverride     DetectionCategory = "system_override"
	CategoryRoleHijacking      DetectionCategory = "role_hijacking"
	CategoryDelimiterInjection DetectionCategory = "delimiter_injection"
	CategoryEncodingAttack     DetectionCategory = "encoding_attack"
	CategoryJailbreak          DetectionCategory = "jailbreak"
)

// DetectionResult contains information about a detected injection attempt
type DetectionResult struct {
	Detected        bool              `json:"detected"`
	Confidence      float64           `json:"confidence"`
	Category        DetectionCategory `json:"category,omitempty"`
	MatchedPatterns []string          `json:"matched_patterns,omitempty"`
	Details         string            `json:"details,omitempty"`
}

// Pattern defines a detection pattern with its category and weight
type Pattern struct {
	Regex       *regexp.Regexp
	Category    DetectionCategory
	Weight      float64
	Description string
	MinLevel    Sensitivity
}

// PromptInjectionDetector detects prompt injection attacks
type PromptInjectionDetector struct {
	sensitivity Sensitivity
	patterns    []Pattern
}

// NewPromptInjectionDetector creates a new detector with the specified sensitivity
func NewPromptInjectionDetector(sensitivity Sensitivity) *PromptInjectionDetector {
	d := &PromptInjectionDetector{
		sensitivity: sensitivity,
	}
	d.initPatterns()
	return d
}

func (d *PromptInjectionDetector) initPatterns() {
	d.patterns = []Pattern{
		// System override patterns
		{
			Regex:       regexp.MustCompile(`(?i)ignore\s+(all\s+)?previous\s+instructions?`),
			Category:    CategorySystemOverride,
			Weight:      1.0,
			Description: "ignore previous instructions",
			MinLevel:    SensitivityLow,
		},
		{
			Regex:       regexp.MustCompile(`(?i)disregard\s+(your\s+|all\s+)?instructions?`),
			Category:    CategorySystemOverride,
			Weight:      1.0,
			Description: "disregard instructions",
			MinLevel:    SensitivityLow,
		},
		{
			Regex:       regexp.MustCompile(`(?i)forget\s+(everything|all|your\s+instructions?)`),
			Category:    CategorySystemOverride,
			Weight:      1.0,
			Description: "forget everything",
			MinLevel:    SensitivityLow,
		},
		{
			Regex:       regexp.MustCompile(`(?i)override\s+(your\s+)?(system|instructions?|programming)`),
			Category:    CategorySystemOverride,
			Weight:      0.9,
			Description: "override system",
			MinLevel:    SensitivityLow,
		},
		{
			Regex:       regexp.MustCompile(`(?i)new\s+instructions?:\s*`),
			Category:    CategorySystemOverride,
			Weight:      0.7,
			Description: "new instructions",
			MinLevel:    SensitivityMedium,
		},

		// Role hijacking patterns
		{
			Regex:       regexp.MustCompile(`(?i)you\s+are\s+now\s+a`),
			Category:    CategoryRoleHijacking,
			Weight:      1.0,
			Description: "you are now a",
			MinLevel:    SensitivityLow,
		},
		{
			Regex:       regexp.MustCompile(`(?i)pretend\s+(to\s+be|you\s+are)`),
			Category:    CategoryRoleHijacking,
			Weight:      0.9,
			Description: "pretend to be",
			MinLevel:    SensitivityLow,
		},
		{
			Regex:       regexp.MustCompile(`(?i)act\s+as\s+(if|though|a)`),
			Category:    CategoryRoleHijacking,
			Weight:      0.8,
			Description: "act as if",
			MinLevel:    SensitivityLow,
		},
		{
			Regex:       regexp.MustCompile(`(?i)roleplay\s+as`),
			Category:    CategoryRoleHijacking,
			Weight:      0.7,
			Description: "roleplay as",
			MinLevel:    SensitivityMedium,
		},
		{
			Regex:       regexp.MustCompile(`(?i)as\s+an\s+ai\s+(assistant|model|system)`),
			Category:    CategoryRoleHijacking,
			Weight:      0.6,
			Description: "as an AI",
			MinLevel:    SensitivityMedium,
		},
		{
			Regex:       regexp.MustCompile(`(?i)imagine\s+you\s+are`),
			Category:    CategoryRoleHijacking,
			Weight:      0.6,
			Description: "imagine you are",
			MinLevel:    SensitivityMedium,
		},

		// Delimiter injection patterns
		{
			Regex:       regexp.MustCompile(`(?i)^system:\s*`),
			Category:    CategoryDelimiterInjection,
			Weight:      1.0,
			Description: "system: prefix",
			MinLevel:    SensitivityLow,
		},
		{
			Regex:       regexp.MustCompile(`(?i)\[INST\]`),
			Category:    CategoryDelimiterInjection,
			Weight:      1.0,
			Description: "[INST] tag",
			MinLevel:    SensitivityLow,
		},
		{
			Regex:       regexp.MustCompile(`(?i)\[/INST\]`),
			Category:    CategoryDelimiterInjection,
			Weight:      1.0,
			Description: "[/INST] tag",
			MinLevel:    SensitivityLow,
		},
		{
			Regex:       regexp.MustCompile(`(?i)###\s*(System|Instruction|Human|Assistant)`),
			Category:    CategoryDelimiterInjection,
			Weight:      0.9,
			Description: "### delimiter",
			MinLevel:    SensitivityLow,
		},
		{
			Regex:       regexp.MustCompile(`<\|?(system|user|assistant|im_start|im_end)\|?>`),
			Category:    CategoryDelimiterInjection,
			Weight:      1.0,
			Description: "chat template tags",
			MinLevel:    SensitivityLow,
		},
		{
			Regex:       regexp.MustCompile(`(?i)</?system>`),
			Category:    CategoryDelimiterInjection,
			Weight:      0.9,
			Description: "<system> XML tag",
			MinLevel:    SensitivityLow,
		},
		{
			Regex:       regexp.MustCompile(`(?i)Human:\s*$`),
			Category:    CategoryDelimiterInjection,
			Weight:      0.7,
			Description: "Human: delimiter",
			MinLevel:    SensitivityMedium,
		},
		{
			Regex:       regexp.MustCompile(`(?i)Assistant:\s*$`),
			Category:    CategoryDelimiterInjection,
			Weight:      0.7,
			Description: "Assistant: delimiter",
			MinLevel:    SensitivityMedium,
		},

		// Jailbreak patterns
		{
			Regex:       regexp.MustCompile(`(?i)\bDAN\s+(mode|prompt)`),
			Category:    CategoryJailbreak,
			Weight:      0.9,
			Description: "DAN jailbreak",
			MinLevel:    SensitivityLow,
		},
		{
			Regex:       regexp.MustCompile(`(?i)jailbreak`),
			Category:    CategoryJailbreak,
			Weight:      0.8,
			Description: "jailbreak keyword",
			MinLevel:    SensitivityMedium,
		},
		{
			Regex:       regexp.MustCompile(`(?i)developer\s+mode`),
			Category:    CategoryJailbreak,
			Weight:      0.7,
			Description: "developer mode",
			MinLevel:    SensitivityMedium,
		},
		{
			Regex:       regexp.MustCompile(`(?i)bypass\s+(your\s+)?(filter|restriction|safety)`),
			Category:    CategoryJailbreak,
			Weight:      0.9,
			Description: "bypass filters",
			MinLevel:    SensitivityLow,
		},
	}
}

// MaxInputSize is the maximum input size (10KB) to prevent ReDoS attacks
const MaxInputSize = 10 * 1024

// Detect analyzes input text for prompt injection attempts
func (d *PromptInjectionDetector) Detect(input string) DetectionResult {
	result := DetectionResult{
		Detected:        false,
		Confidence:      0.0,
		MatchedPatterns: []string{},
	}

	if input == "" {
		return result
	}

	// Limit input size to prevent ReDoS attacks
	if len(input) > MaxInputSize {
		input = input[:MaxInputSize]
	}

	// Normalize input for detection
	normalized := d.normalizeInput(input)

	// Check for encoding attacks first
	if encodingResult := d.detectEncodingAttacks(input); encodingResult.Detected {
		return encodingResult
	}

	// Check for unicode homoglyphs
	if d.sensitivity >= SensitivityMedium {
		if homoglyphResult := d.detectHomoglyphs(input); homoglyphResult.Detected {
			return homoglyphResult
		}
	}

	// Check patterns
	var totalWeight float64
	var matchedWeight float64
	var highestCategory DetectionCategory

	for _, pattern := range d.patterns {
		if pattern.MinLevel > d.sensitivity {
			continue
		}

		totalWeight += pattern.Weight

		if pattern.Regex.MatchString(normalized) {
			matchedWeight += pattern.Weight
			result.MatchedPatterns = append(result.MatchedPatterns, pattern.Description)

			if pattern.Weight > result.Confidence {
				result.Confidence = pattern.Weight
				highestCategory = pattern.Category
			}
		}
	}

	if len(result.MatchedPatterns) > 0 {
		result.Detected = true
		result.Category = highestCategory

		// Boost confidence if multiple patterns matched
		if len(result.MatchedPatterns) > 1 {
			result.Confidence = min(1.0, result.Confidence+0.1*float64(len(result.MatchedPatterns)-1))
		}

		result.Details = "Detected potential prompt injection attack"
	}

	return result
}

// normalizeInput prepares input for pattern matching
func (d *PromptInjectionDetector) normalizeInput(input string) string {
	// Remove zero-width characters that might be used to evade detection first
	normalized := d.removeZeroWidthChars(input)

	// Normalize whitespace but preserve newlines for patterns that need them
	normalized = regexp.MustCompile(`[ \t]+`).ReplaceAllString(normalized, " ")

	return normalized
}

// removeZeroWidthChars removes invisible unicode characters
func (d *PromptInjectionDetector) removeZeroWidthChars(input string) string {
	var result strings.Builder
	for _, r := range input {
		// Skip zero-width and invisible characters
		if r == '\u200B' || r == '\u200C' || r == '\u200D' ||
			r == '\uFEFF' || r == '\u00AD' || r == '\u2060' {
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

// maxBase64Matches is the maximum number of base64 patterns to check to prevent DoS
const maxBase64Matches = 10

// detectEncodingAttacks checks for base64 or other encoded malicious content
func (d *PromptInjectionDetector) detectEncodingAttacks(input string) DetectionResult {
	result := DetectionResult{
		Detected:        false,
		MatchedPatterns: []string{},
	}

	// Look for base64-like patterns, limited to prevent DoS
	base64Pattern := regexp.MustCompile(`[A-Za-z0-9+/]{20,}={0,2}`)
	matches := base64Pattern.FindAllString(input, maxBase64Matches)

	for _, match := range matches {
		decoded, err := base64.StdEncoding.DecodeString(match)
		if err != nil {
			continue
		}

		decodedStr := string(decoded)

		// Check if decoded content contains injection patterns
		tempDetector := &PromptInjectionDetector{
			sensitivity: d.sensitivity,
		}
		tempDetector.initPatterns()

		// Use a simple check to avoid recursion issues
		for _, pattern := range tempDetector.patterns {
			if pattern.MinLevel > d.sensitivity {
				continue
			}
			if pattern.Regex.MatchString(strings.ToLower(decodedStr)) {
				result.Detected = true
				result.Confidence = 0.95
				result.Category = CategoryEncodingAttack
				result.MatchedPatterns = append(result.MatchedPatterns, "base64 encoded: "+pattern.Description)
				result.Details = "Detected base64-encoded prompt injection"
				return result
			}
		}
	}

	return result
}

// detectHomoglyphs checks for unicode homoglyph attacks
func (d *PromptInjectionDetector) detectHomoglyphs(input string) DetectionResult {
	result := DetectionResult{
		Detected:        false,
		MatchedPatterns: []string{},
	}

	// Common homoglyph mappings (attack characters -> ASCII equivalent)
	homoglyphs := map[rune]rune{
		'\u0430': 'a', // Cyrillic а
		'\u0435': 'e', // Cyrillic е
		'\u043E': 'o', // Cyrillic о
		'\u0440': 'p', // Cyrillic р
		'\u0441': 'c', // Cyrillic с
		'\u0445': 'x', // Cyrillic х
		'\u0443': 'y', // Cyrillic у
		'\u0456': 'i', // Cyrillic і
		'\u0391': 'A', // Greek Α
		'\u0392': 'B', // Greek Β
		'\u0395': 'E', // Greek Ε
		'\u0397': 'H', // Greek Η
		'\u0399': 'I', // Greek Ι
		'\u039A': 'K', // Greek Κ
		'\u039C': 'M', // Greek Μ
		'\u039D': 'N', // Greek Ν
		'\u039F': 'O', // Greek Ο
		'\u03A1': 'P', // Greek Ρ
		'\u03A4': 'T', // Greek Τ
		'\u03A7': 'X', // Greek Χ
		'\u03A5': 'Y', // Greek Υ
		'\u0417': 'Z', // Cyrillic З
	}

	var normalized strings.Builder
	homoglyphCount := 0

	for _, r := range input {
		if replacement, ok := homoglyphs[r]; ok {
			normalized.WriteRune(replacement)
			homoglyphCount++
		} else {
			normalized.WriteRune(r)
		}
	}

	// If we found homoglyphs, check the normalized text for injections
	if homoglyphCount > 0 {
		normalizedStr := normalized.String()

		for _, pattern := range d.patterns {
			if pattern.MinLevel > d.sensitivity {
				continue
			}
			if pattern.Regex.MatchString(strings.ToLower(normalizedStr)) {
				result.Detected = true
				result.Confidence = 0.9
				result.Category = CategoryEncodingAttack
				result.MatchedPatterns = append(result.MatchedPatterns, "homoglyph attack: "+pattern.Description)
				result.Details = "Detected unicode homoglyph obfuscation"
				return result
			}
		}

		// Also flag high homoglyph counts as suspicious at high sensitivity
		if d.sensitivity >= SensitivityHigh && homoglyphCount > 3 {
			asciiCount := 0
			for _, r := range input {
				if r < 128 && unicode.IsLetter(r) {
					asciiCount++
				}
			}
			// If homoglyphs make up significant portion of letters
			if asciiCount > 0 && float64(homoglyphCount)/float64(asciiCount+homoglyphCount) > 0.1 {
				result.Detected = true
				result.Confidence = 0.5
				result.Category = CategoryEncodingAttack
				result.MatchedPatterns = append(result.MatchedPatterns, "suspicious homoglyph usage")
				result.Details = "High ratio of unicode homoglyphs detected"
			}
		}
	}

	return result
}

// SetSensitivity changes the detection sensitivity level
func (d *PromptInjectionDetector) SetSensitivity(level Sensitivity) {
	d.sensitivity = level
}

// GetSensitivity returns the current sensitivity level
func (d *PromptInjectionDetector) GetSensitivity() Sensitivity {
	return d.sensitivity
}

// AddPattern adds a custom detection pattern
func (d *PromptInjectionDetector) AddPattern(pattern Pattern) {
	d.patterns = append(d.patterns, pattern)
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
