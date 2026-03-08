// Package prompt provides interactive terminal UI components.
package prompt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Option represents a selectable option in a prompt.
type Option struct {
	Label       string
	Value       string
	Description string
}

// Prompter provides interactive prompts with configurable input/output.
type Prompter struct {
	in  io.Reader
	out io.Writer
}

// NewPrompter creates a new Prompter with the given input and output.
func NewPrompter(in io.Reader, out io.Writer) *Prompter {
	return &Prompter{in: in, out: out}
}

// DefaultPrompter returns a Prompter using stdin/stdout.
func DefaultPrompter() *Prompter {
	return &Prompter{in: os.Stdin, out: os.Stdout}
}

// SelectModel prompts the user to select a model.
func SelectModel() (string, error) {
	return DefaultPrompter().SelectModel()
}

// SelectModel prompts the user to select a model.
func (p *Prompter) SelectModel() (string, error) {
	options := []Option{
		{Label: "claude-3-5-sonnet (Recommended)", Value: "claude-3-5-sonnet", Description: "Best balance of speed and capability"},
		{Label: "gpt-4o", Value: "gpt-4o", Description: "OpenAI's latest model"},
		{Label: "gemini-1.5-pro", Value: "gemini-1.5-pro", Description: "Google's multimodal model"},
		{Label: "grok-2", Value: "grok-2", Description: "xAI's reasoning model"},
		{Label: "claude-opus-4", Value: "claude-opus-4", Description: "Most capable, best for complex tasks"},
		{Label: "Other (type model name)", Value: "", Description: "Enter a custom model name"},
	}

	return p.Select("Which model do you want to use?", options)
}

// Select prompts the user to select one option from a list.
func Select(question string, options []Option) (string, error) {
	return DefaultPrompter().Select(question, options)
}

// Select prompts the user to select one option from a list.
func (p *Prompter) Select(question string, options []Option) (string, error) {
	_, _ = fmt.Fprintln(p.out)
	_, _ = fmt.Fprintln(p.out, question)
	_, _ = fmt.Fprintln(p.out)

	for i, opt := range options {
		_, _ = fmt.Fprintf(p.out, "  %d. %s\n", i+1, opt.Label)
		if opt.Description != "" {
			_, _ = fmt.Fprintf(p.out, "     %s\n", opt.Description)
		}
	}

	_, _ = fmt.Fprintln(p.out)
	_, _ = fmt.Fprint(p.out, "Enter your choice (1-", len(options), "): ")

	reader := bufio.NewReader(p.in)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)

	// Parse as number
	choice, err := strconv.Atoi(input)
	if err != nil || choice < 1 || choice > len(options) {
		// Try to use input as direct value
		if input != "" {
			return input, nil
		}
		return "", fmt.Errorf("invalid choice: %s", input)
	}

	selected := options[choice-1]

	// If "Other" selected, prompt for custom input
	if selected.Value == "" {
		_, _ = fmt.Fprint(p.out, "Enter model name: ")
		input, err = reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read input: %w", err)
		}
		return strings.TrimSpace(input), nil
	}

	return selected.Value, nil
}

// MultiSelect prompts the user to select multiple options.
func MultiSelect(question string, options []Option) ([]string, error) {
	return DefaultPrompter().MultiSelect(question, options)
}

// MultiSelect prompts the user to select multiple options.
func (p *Prompter) MultiSelect(question string, options []Option) ([]string, error) {
	_, _ = fmt.Fprintln(p.out)
	_, _ = fmt.Fprintln(p.out, question)
	_, _ = fmt.Fprintln(p.out, "(Enter comma-separated numbers, e.g., 1,3,4)")
	_, _ = fmt.Fprintln(p.out)

	for i, opt := range options {
		_, _ = fmt.Fprintf(p.out, "  %d. %s\n", i+1, opt.Label)
		if opt.Description != "" {
			_, _ = fmt.Fprintf(p.out, "     %s\n", opt.Description)
		}
	}

	_, _ = fmt.Fprintln(p.out)
	_, _ = fmt.Fprint(p.out, "Your choices: ")

	reader := bufio.NewReader(p.in)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	parts := strings.Split(input, ",")

	var selected []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		choice, err := strconv.Atoi(part)
		if err != nil || choice < 1 || choice > len(options) {
			continue
		}
		selected = append(selected, options[choice-1].Value)
	}

	return selected, nil
}

// Confirm prompts the user for a yes/no confirmation.
func Confirm(question string) (bool, error) {
	return DefaultPrompter().Confirm(question)
}

// Confirm prompts the user for a yes/no confirmation.
func (p *Prompter) Confirm(question string) (bool, error) {
	_, _ = fmt.Fprintf(p.out, "%s (y/N): ", question)

	reader := bufio.NewReader(p.in)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.ToLower(strings.TrimSpace(input))
	return input == "y" || input == "yes", nil
}

// Input prompts the user for free text input.
func Input(prompt string) (string, error) {
	return DefaultPrompter().Input(prompt)
}

// Input prompts the user for free text input.
func (p *Prompter) Input(prompt string) (string, error) {
	_, _ = fmt.Fprint(p.out, prompt)

	reader := bufio.NewReader(p.in)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	return strings.TrimSpace(input), nil
}

// InputWithDefault prompts for input with a default value.
func InputWithDefault(prompt, defaultValue string) (string, error) {
	return DefaultPrompter().InputWithDefault(prompt, defaultValue)
}

// InputWithDefault prompts for input with a default value.
func (p *Prompter) InputWithDefault(prompt, defaultValue string) (string, error) {
	if defaultValue != "" {
		_, _ = fmt.Fprintf(p.out, "%s [%s]: ", prompt, defaultValue)
	} else {
		_, _ = fmt.Fprintf(p.out, "%s: ", prompt)
	}

	reader := bufio.NewReader(p.in)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue, nil
	}

	return input, nil
}

// Password prompts for password input (no echo).
// Note: This is a basic implementation. For proper password input,
// use golang.org/x/term.
func Password(prompt string) (string, error) {
	return DefaultPrompter().Password(prompt)
}

// Password prompts for password input (no echo).
func (p *Prompter) Password(prompt string) (string, error) {
	_, _ = fmt.Fprint(p.out, prompt)

	reader := bufio.NewReader(p.in)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	return strings.TrimSpace(input), nil
}

// ConfirmAction prompts for confirmation with a description.
func ConfirmAction(action, description string) (bool, error) {
	return DefaultPrompter().ConfirmAction(action, description)
}

// ConfirmAction prompts for confirmation with a description.
func (p *Prompter) ConfirmAction(action, description string) (bool, error) {
	_, _ = fmt.Fprintln(p.out)
	_, _ = fmt.Fprintf(p.out, "Action: %s\n", action)
	if description != "" {
		_, _ = fmt.Fprintf(p.out, "Description: %s\n", description)
	}
	return p.Confirm("Proceed?")
}

// ConfirmDangerous prompts for confirmation of a dangerous action.
func ConfirmDangerous(action string) (bool, error) {
	return DefaultPrompter().ConfirmDangerous(action)
}

// ConfirmDangerous prompts for confirmation of a dangerous action.
func (p *Prompter) ConfirmDangerous(action string) (bool, error) {
	_, _ = fmt.Fprintln(p.out)
	_, _ = fmt.Fprintln(p.out, "⚠️  DANGEROUS ACTION")
	_, _ = fmt.Fprintf(p.out, "This will: %s\n", action)
	_, _ = fmt.Fprintln(p.out)

	confirmation, err := p.Input("Type 'yes' to confirm: ")
	if err != nil {
		return false, err
	}

	return strings.ToLower(confirmation) == "yes", nil
}
