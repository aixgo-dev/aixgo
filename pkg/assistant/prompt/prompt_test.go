package prompt

import (
	"bytes"
	"strings"
	"testing"
)

func TestOption(t *testing.T) {
	opt := Option{
		Label:       "Test Label",
		Value:       "test_value",
		Description: "Test Description",
	}

	if opt.Label != "Test Label" {
		t.Errorf("Label = %v, want Test Label", opt.Label)
	}
	if opt.Value != "test_value" {
		t.Errorf("Value = %v, want test_value", opt.Value)
	}
	if opt.Description != "Test Description" {
		t.Errorf("Description = %v, want Test Description", opt.Description)
	}
}

func TestOptionDefaults(t *testing.T) {
	opt := Option{}
	if opt.Label != "" {
		t.Errorf("Label should be empty")
	}
	if opt.Value != "" {
		t.Errorf("Value should be empty")
	}
}

func TestNewPrompter(t *testing.T) {
	in := strings.NewReader("test\n")
	out := &bytes.Buffer{}

	p := NewPrompter(in, out)

	if p == nil {
		t.Fatal("NewPrompter returned nil")
	}
	if p.in != in {
		t.Error("in field not set correctly")
	}
	if p.out != out {
		t.Error("out field not set correctly")
	}
}

func TestDefaultPrompter(t *testing.T) {
	p := DefaultPrompter()
	if p == nil {
		t.Fatal("DefaultPrompter returned nil")
	}
}

func TestPrompter_Select(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		options  []Option
		want     string
		wantErr  bool
	}{
		{
			name:  "select first option",
			input: "1\n",
			options: []Option{
				{Label: "Option 1", Value: "opt1"},
				{Label: "Option 2", Value: "opt2"},
			},
			want:    "opt1",
			wantErr: false,
		},
		{
			name:  "select second option",
			input: "2\n",
			options: []Option{
				{Label: "Option 1", Value: "opt1"},
				{Label: "Option 2", Value: "opt2"},
			},
			want:    "opt2",
			wantErr: false,
		},
		{
			name:  "direct value input",
			input: "custom-model\n",
			options: []Option{
				{Label: "Option 1", Value: "opt1"},
			},
			want:    "custom-model",
			wantErr: false,
		},
		{
			name:  "empty input returns error",
			input: "\n",
			options: []Option{
				{Label: "Option 1", Value: "opt1"},
			},
			want:    "",
			wantErr: true,
		},
		{
			name:  "option with description",
			input: "1\n",
			options: []Option{
				{Label: "Option 1", Value: "opt1", Description: "First option"},
			},
			want:    "opt1",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := strings.NewReader(tt.input)
			out := &bytes.Buffer{}
			p := NewPrompter(in, out)

			got, err := p.Select("Test question", tt.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("Select() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Select() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrompter_SelectWithOther(t *testing.T) {
	// Test "Other" option that prompts for custom input
	in := strings.NewReader("2\ncustom-value\n")
	out := &bytes.Buffer{}
	p := NewPrompter(in, out)

	options := []Option{
		{Label: "Option 1", Value: "opt1"},
		{Label: "Other", Value: ""}, // Empty value triggers custom input
	}

	got, err := p.Select("Test question", options)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if got != "custom-value" {
		t.Errorf("Select() = %v, want custom-value", got)
	}
}

func TestPrompter_SelectModel(t *testing.T) {
	// Select the first (recommended) model
	in := strings.NewReader("1\n")
	out := &bytes.Buffer{}
	p := NewPrompter(in, out)

	got, err := p.SelectModel()
	if err != nil {
		t.Fatalf("SelectModel() error = %v", err)
	}
	if got != "claude-sonnet-4-6" {
		t.Errorf("SelectModel() = %v, want claude-sonnet-4-6", got)
	}
}

func TestPrompter_MultiSelect(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		options []Option
		want    []string
	}{
		{
			name:  "select single option",
			input: "1\n",
			options: []Option{
				{Label: "Option 1", Value: "opt1"},
				{Label: "Option 2", Value: "opt2"},
			},
			want: []string{"opt1"},
		},
		{
			name:  "select multiple options",
			input: "1,2\n",
			options: []Option{
				{Label: "Option 1", Value: "opt1"},
				{Label: "Option 2", Value: "opt2"},
			},
			want: []string{"opt1", "opt2"},
		},
		{
			name:  "select with spaces",
			input: "1, 3\n",
			options: []Option{
				{Label: "Option 1", Value: "opt1"},
				{Label: "Option 2", Value: "opt2"},
				{Label: "Option 3", Value: "opt3"},
			},
			want: []string{"opt1", "opt3"},
		},
		{
			name:  "invalid choices ignored",
			input: "1,99,abc,2\n",
			options: []Option{
				{Label: "Option 1", Value: "opt1"},
				{Label: "Option 2", Value: "opt2"},
			},
			want: []string{"opt1", "opt2"},
		},
		{
			name:  "option with description",
			input: "1\n",
			options: []Option{
				{Label: "Option 1", Value: "opt1", Description: "First option"},
			},
			want: []string{"opt1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := strings.NewReader(tt.input)
			out := &bytes.Buffer{}
			p := NewPrompter(in, out)

			got, err := p.MultiSelect("Test question", tt.options)
			if err != nil {
				t.Fatalf("MultiSelect() error = %v", err)
			}
			if len(got) != len(tt.want) {
				t.Errorf("MultiSelect() len = %d, want %d", len(got), len(tt.want))
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("MultiSelect()[%d] = %v, want %v", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestPrompter_Confirm(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"lowercase y", "y\n", true},
		{"uppercase Y", "Y\n", true},
		{"lowercase yes", "yes\n", true},
		{"uppercase YES", "YES\n", true},
		{"mixed case Yes", "Yes\n", true},
		{"lowercase n", "n\n", false},
		{"uppercase N", "N\n", false},
		{"lowercase no", "no\n", false},
		{"empty", "\n", false},
		{"other", "maybe\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := strings.NewReader(tt.input)
			out := &bytes.Buffer{}
			p := NewPrompter(in, out)

			got, err := p.Confirm("Test question")
			if err != nil {
				t.Fatalf("Confirm() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Confirm() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrompter_Input(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple input", "hello\n", "hello"},
		{"with spaces", "  hello world  \n", "hello world"},
		{"empty", "\n", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := strings.NewReader(tt.input)
			out := &bytes.Buffer{}
			p := NewPrompter(in, out)

			got, err := p.Input("Enter: ")
			if err != nil {
				t.Fatalf("Input() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Input() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrompter_InputWithDefault(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		defaultValue string
		want         string
	}{
		{"use input", "custom\n", "default", "custom"},
		{"use default", "\n", "default", "default"},
		{"empty default with input", "value\n", "", "value"},
		{"empty default empty input", "\n", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := strings.NewReader(tt.input)
			out := &bytes.Buffer{}
			p := NewPrompter(in, out)

			got, err := p.InputWithDefault("Enter: ", tt.defaultValue)
			if err != nil {
				t.Fatalf("InputWithDefault() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("InputWithDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrompter_Password(t *testing.T) {
	in := strings.NewReader("secret123\n")
	out := &bytes.Buffer{}
	p := NewPrompter(in, out)

	got, err := p.Password("Password: ")
	if err != nil {
		t.Fatalf("Password() error = %v", err)
	}
	if got != "secret123" {
		t.Errorf("Password() = %v, want secret123", got)
	}
}

func TestPrompter_ConfirmAction(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		action      string
		description string
		want        bool
	}{
		{"confirm with description", "y\n", "Delete file", "This will delete the file", true},
		{"confirm without description", "y\n", "Delete file", "", true},
		{"decline", "n\n", "Delete file", "This will delete the file", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := strings.NewReader(tt.input)
			out := &bytes.Buffer{}
			p := NewPrompter(in, out)

			got, err := p.ConfirmAction(tt.action, tt.description)
			if err != nil {
				t.Fatalf("ConfirmAction() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ConfirmAction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrompter_ConfirmDangerous(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"confirm with yes", "yes\n", true},
		{"confirm with YES", "YES\n", true},
		{"decline with no", "no\n", false},
		{"decline with empty", "\n", false},
		{"decline with y", "y\n", false}, // Must type full "yes"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := strings.NewReader(tt.input)
			out := &bytes.Buffer{}
			p := NewPrompter(in, out)

			got, err := p.ConfirmDangerous("Delete everything")
			if err != nil {
				t.Fatalf("ConfirmDangerous() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ConfirmDangerous() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectModelOptions(t *testing.T) {
	// Verify the expected models are well-formed
	expectedModels := []string{
		"claude-sonnet-4-6",
		"gpt-4o",
		"gemini-2.5-flash",
		"grok-4",
		"claude-opus-4-6",
	}

	for _, model := range expectedModels {
		if model == "" {
			t.Error("Model name should not be empty")
		}
		if strings.Contains(model, " ") {
			t.Errorf("Model name should not contain spaces: %s", model)
		}
	}
}

func TestOptionSlice(t *testing.T) {
	options := []Option{
		{Label: "Option 1", Value: "opt1", Description: "First option"},
		{Label: "Option 2", Value: "opt2", Description: "Second option"},
	}

	if len(options) != 2 {
		t.Errorf("Expected 2 options, got %d", len(options))
	}

	if options[0].Label != "Option 1" {
		t.Errorf("First option label = %v, want Option 1", options[0].Label)
	}
}

func TestInputParsing(t *testing.T) {
	inputs := []struct {
		raw      string
		expected string
	}{
		{"  hello  ", "hello"},
		{"hello\n", "hello"},
		{"\t\nworld\t\n", "world"},
		{"", ""},
	}

	for _, tc := range inputs {
		result := strings.TrimSpace(tc.raw)
		if result != tc.expected {
			t.Errorf("TrimSpace(%q) = %q, want %q", tc.raw, result, tc.expected)
		}
	}
}

func TestConfirmationParsing(t *testing.T) {
	yesResponses := []string{"y", "Y", "yes", "YES", "Yes"}
	noResponses := []string{"n", "N", "no", "NO", "No", "", "x", "maybe"}

	for _, resp := range yesResponses {
		normalized := strings.ToLower(strings.TrimSpace(resp))
		isYes := normalized == "y" || normalized == "yes"
		if !isYes {
			t.Errorf("Expected %q to be interpreted as yes", resp)
		}
	}

	for _, resp := range noResponses {
		normalized := strings.ToLower(strings.TrimSpace(resp))
		isYes := normalized == "y" || normalized == "yes"
		if isYes && resp != "" {
			t.Errorf("Expected %q to be interpreted as no", resp)
		}
	}
}

// Test output formatting
func TestPrompter_OutputFormat(t *testing.T) {
	in := strings.NewReader("1\n")
	out := &bytes.Buffer{}
	p := NewPrompter(in, out)

	options := []Option{
		{Label: "Test Option", Value: "test", Description: "A test"},
	}

	_, err := p.Select("Select one", options)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Select one") {
		t.Error("Output should contain question")
	}
	if !strings.Contains(output, "Test Option") {
		t.Error("Output should contain option label")
	}
	if !strings.Contains(output, "A test") {
		t.Error("Output should contain description")
	}
}

func TestPrompter_ConfirmOutputFormat(t *testing.T) {
	in := strings.NewReader("y\n")
	out := &bytes.Buffer{}
	p := NewPrompter(in, out)

	_, err := p.Confirm("Are you sure?")
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Are you sure?") {
		t.Error("Output should contain question")
	}
	if !strings.Contains(output, "(y/N)") {
		t.Error("Output should contain y/N hint")
	}
}
