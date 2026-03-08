package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewRenderer(t *testing.T) {
	t.Run("DefaultConfig", func(t *testing.T) {
		renderer := NewRenderer(Config{})
		if renderer == nil {
			t.Fatal("NewRenderer returned nil")
		}
	})

	t.Run("CustomWriter", func(t *testing.T) {
		var buf bytes.Buffer
		renderer := NewRenderer(Config{Writer: &buf})

		err := renderer.Render("test content")
		if err != nil {
			t.Fatalf("Render failed: %v", err)
		}

		if !strings.Contains(buf.String(), "test content") {
			t.Errorf("Output = %q, want to contain 'test content'", buf.String())
		}
	})
}

func TestRenderer_Render(t *testing.T) {
	var buf bytes.Buffer
	renderer := NewRenderer(Config{Writer: &buf})

	t.Run("SimpleText", func(t *testing.T) {
		buf.Reset()
		err := renderer.Render("Hello, World!")
		if err != nil {
			t.Fatalf("Render failed: %v", err)
		}
		if !strings.Contains(buf.String(), "Hello, World!") {
			t.Errorf("Output missing content")
		}
	})

	t.Run("Markdown", func(t *testing.T) {
		buf.Reset()
		err := renderer.Render("# Header\n\n- Item 1\n- Item 2")
		if err != nil {
			t.Fatalf("Render failed: %v", err)
		}
		if buf.Len() == 0 {
			t.Error("Expected non-empty output")
		}
	})
}

func TestRenderer_RenderCode(t *testing.T) {
	var buf bytes.Buffer
	renderer := NewRenderer(Config{Writer: &buf})

	err := renderer.RenderCode("func main() {}", "go")
	if err != nil {
		t.Fatalf("RenderCode failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "```go") {
		t.Error("Output should contain code fence with language")
	}
	if !strings.Contains(output, "func main() {}") {
		t.Error("Output should contain code")
	}
	if !strings.Contains(output, "```") {
		t.Error("Output should contain closing fence")
	}
}

func TestRenderer_RenderError(t *testing.T) {
	var buf bytes.Buffer
	renderer := NewRenderer(Config{Writer: &buf})

	err := renderer.RenderError(errTest{})
	if err != nil {
		t.Fatalf("RenderError failed: %v", err)
	}

	if !strings.Contains(buf.String(), "test error message") {
		t.Errorf("Output = %q, want to contain error message", buf.String())
	}
}

type errTest struct{}

func (e errTest) Error() string { return "test error message" }

func TestRenderer_RenderInfo(t *testing.T) {
	var buf bytes.Buffer
	renderer := NewRenderer(Config{Writer: &buf})

	err := renderer.RenderInfo("info message")
	if err != nil {
		t.Fatalf("RenderInfo failed: %v", err)
	}

	if !strings.Contains(buf.String(), "info message") {
		t.Errorf("Output = %q, want to contain message", buf.String())
	}
}

func TestRenderer_RenderSuccess(t *testing.T) {
	var buf bytes.Buffer
	renderer := NewRenderer(Config{Writer: &buf})

	err := renderer.RenderSuccess("success message")
	if err != nil {
		t.Fatalf("RenderSuccess failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "success message") {
		t.Errorf("Output = %q, want to contain message", output)
	}
	if !strings.Contains(output, "✓") {
		t.Error("Output should contain checkmark")
	}
}

func TestRenderer_RenderWarning(t *testing.T) {
	var buf bytes.Buffer
	renderer := NewRenderer(Config{Writer: &buf})

	err := renderer.RenderWarning("warning message")
	if err != nil {
		t.Fatalf("RenderWarning failed: %v", err)
	}

	if !strings.Contains(buf.String(), "warning message") {
		t.Errorf("Output = %q, want to contain message", buf.String())
	}
}

func TestRenderer_RenderList(t *testing.T) {
	var buf bytes.Buffer
	renderer := NewRenderer(Config{Writer: &buf})

	err := renderer.RenderList([]string{"item1", "item2", "item3"})
	if err != nil {
		t.Fatalf("RenderList failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "item1") {
		t.Error("Output should contain item1")
	}
	if !strings.Contains(output, "item2") {
		t.Error("Output should contain item2")
	}
	if !strings.Contains(output, "•") {
		t.Error("Output should contain bullet points")
	}
}

func TestRenderer_RenderTable(t *testing.T) {
	var buf bytes.Buffer
	renderer := NewRenderer(Config{Writer: &buf})

	t.Run("WithData", func(t *testing.T) {
		buf.Reset()
		headers := []string{"Name", "Age", "City"}
		rows := [][]string{
			{"Alice", "30", "NYC"},
			{"Bob", "25", "LA"},
		}

		err := renderer.RenderTable(headers, rows)
		if err != nil {
			t.Fatalf("RenderTable failed: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "Name") {
			t.Error("Output should contain header")
		}
		if !strings.Contains(output, "Alice") {
			t.Error("Output should contain data")
		}
		if !strings.Contains(output, "─") {
			t.Error("Output should contain separator")
		}
	})

	t.Run("EmptyHeaders", func(t *testing.T) {
		buf.Reset()
		err := renderer.RenderTable([]string{}, nil)
		if err != nil {
			t.Fatalf("RenderTable failed: %v", err)
		}
		// Should just return without rendering
	})

	t.Run("MismatchedColumns", func(t *testing.T) {
		buf.Reset()
		headers := []string{"A", "B", "C"}
		rows := [][]string{
			{"1"}, // Only one column
		}

		err := renderer.RenderTable(headers, rows)
		if err != nil {
			t.Fatalf("RenderTable failed: %v", err)
		}
		// Should handle gracefully
	})
}

func TestRenderer_RenderStreaming(t *testing.T) {
	var buf bytes.Buffer
	renderer := NewRenderer(Config{Writer: &buf})

	chunks := make(chan string, 3)
	chunks <- "Hello"
	chunks <- " "
	chunks <- "World"
	close(chunks)

	err := renderer.RenderStreaming(chunks)
	if err != nil {
		t.Fatalf("RenderStreaming failed: %v", err)
	}

	if !strings.Contains(buf.String(), "Hello World") {
		t.Errorf("Output = %q, want 'Hello World'", buf.String())
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		input    string
		width    int
		expected string
	}{
		{"hello", 10, "hello     "},
		{"hello", 5, "hello"},
		{"hello", 3, "hello"}, // Shorter than input
		{"", 5, "     "},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := padRight(tt.input, tt.width)
			if result != tt.expected {
				t.Errorf("padRight(%q, %d) = %q, want %q", tt.input, tt.width, result, tt.expected)
			}
		})
	}
}
