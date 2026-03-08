// Package output provides rendering utilities for chat output.
package output

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Config holds renderer configuration.
type Config struct {
	Streaming bool
	Width     int
	Theme     string
	Writer    io.Writer
}

// Renderer handles output rendering for the chat assistant.
type Renderer struct {
	config Config
	writer io.Writer
}

// NewRenderer creates a new renderer with the given configuration.
func NewRenderer(config Config) *Renderer {
	writer := config.Writer
	if writer == nil {
		writer = os.Stdout
	}

	width := config.Width
	if width == 0 {
		width = 80
	}

	return &Renderer{
		config: config,
		writer: writer,
	}
}

// Render renders markdown content to the output.
func (r *Renderer) Render(content string) error {
	// For now, output content directly
	// TODO: Add glamour markdown rendering when dependency is added
	_, err := fmt.Fprintln(r.writer, content)
	return err
}

// RenderStreaming renders streaming content chunk by chunk.
func (r *Renderer) RenderStreaming(chunks <-chan string) error {
	for chunk := range chunks {
		if _, err := fmt.Fprint(r.writer, chunk); err != nil {
			return err
		}
	}
	fmt.Fprintln(r.writer)
	return nil
}

// RenderCode renders a code block with syntax highlighting.
func (r *Renderer) RenderCode(code, language string) error {
	var sb strings.Builder
	sb.WriteString("```")
	sb.WriteString(language)
	sb.WriteString("\n")
	sb.WriteString(code)
	if !strings.HasSuffix(code, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("```\n")
	return r.Render(sb.String())
}

// RenderError renders an error message.
func (r *Renderer) RenderError(err error) error {
	return r.Render(fmt.Sprintf("Error: %v", err))
}

// RenderInfo renders an informational message.
func (r *Renderer) RenderInfo(msg string) error {
	return r.Render(fmt.Sprintf("ℹ️  %s", msg))
}

// RenderSuccess renders a success message.
func (r *Renderer) RenderSuccess(msg string) error {
	return r.Render(fmt.Sprintf("✓ %s", msg))
}

// RenderWarning renders a warning message.
func (r *Renderer) RenderWarning(msg string) error {
	return r.Render(fmt.Sprintf("⚠️  %s", msg))
}

// RenderList renders a list of items.
func (r *Renderer) RenderList(items []string) error {
	var sb strings.Builder
	for _, item := range items {
		sb.WriteString("• ")
		sb.WriteString(item)
		sb.WriteString("\n")
	}
	return r.Render(sb.String())
}

// RenderTable renders a simple table.
func (r *Renderer) RenderTable(headers []string, rows [][]string) error {
	if len(headers) == 0 {
		return nil
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	var sb strings.Builder

	// Header row
	for i, h := range headers {
		sb.WriteString(padRight(h, widths[i]))
		if i < len(headers)-1 {
			sb.WriteString("  ")
		}
	}
	sb.WriteString("\n")

	// Separator
	for i, w := range widths {
		sb.WriteString(strings.Repeat("─", w))
		if i < len(widths)-1 {
			sb.WriteString("  ")
		}
	}
	sb.WriteString("\n")

	// Data rows
	for _, row := range rows {
		for i := 0; i < len(headers); i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			sb.WriteString(padRight(cell, widths[i]))
			if i < len(headers)-1 {
				sb.WriteString("  ")
			}
		}
		sb.WriteString("\n")
	}

	return r.Render(sb.String())
}

// padRight pads a string to the right to reach the given width.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
