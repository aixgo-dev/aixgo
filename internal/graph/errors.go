package graph

import (
	"fmt"
	"strings"
)

// CycleError provides detailed information about a dependency cycle.
type CycleError struct {
	Path []string
}

// Error returns a human-readable description of the cycle.
func (e *CycleError) Error() string {
	return fmt.Sprintf("dependency cycle detected: %s", strings.Join(e.Path, " -> "))
}

// Unwrap returns the base error for errors.Is compatibility.
func (e *CycleError) Unwrap() error {
	return ErrCycleDetected
}
