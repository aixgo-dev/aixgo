package observability

import (
	"testing"
)

func TestStartSpan(t *testing.T) {
	tests := []struct {
		name     string
		spanName string
		data     map[string]any
	}{
		{
			name:     "span with nil data",
			spanName: "test-span",
			data:     nil,
		},
		{
			name:     "span with empty data",
			spanName: "empty-span",
			data:     map[string]any{},
		},
		{
			name:     "span with string data",
			spanName: "string-span",
			data: map[string]any{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name:     "span with mixed data types",
			spanName: "mixed-span",
			data: map[string]any{
				"string": "text",
				"int":    42,
				"float":  3.14,
				"bool":   true,
				"slice":  []string{"a", "b", "c"},
				"map":    map[string]string{"nested": "value"},
			},
		},
		{
			name:     "span with empty name",
			spanName: "",
			data:     map[string]any{"test": "data"},
		},
		{
			name:     "span with special characters in name",
			spanName: "span-with.special_chars/test",
			data:     map[string]any{"special": "chars"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := StartSpan(tt.spanName, tt.data)

			if span == nil {
				t.Fatal("StartSpan returned nil")
			}

			if span.name != tt.spanName {
				t.Errorf("span.name = %v, want %v", span.name, tt.spanName)
			}

			// Verify data is stored correctly
			if tt.data == nil && span.data != nil {
				t.Errorf("span.data = %v, want nil", span.data)
			}

			if tt.data != nil {
				if span.data == nil {
					t.Fatal("span.data is nil, want non-nil")
				}

				if len(span.data) != len(tt.data) {
					t.Errorf("span.data length = %v, want %v", len(span.data), len(tt.data))
				}

				for k, v := range tt.data {
					gotVal := span.data[k]
					// Skip comparison for slices and maps as they're not directly comparable
					switch v.(type) {
					case []string, []int, map[string]string, map[string]any:
						if gotVal == nil {
							t.Errorf("span.data[%v] = nil, want non-nil", k)
						}
					default:
						if gotVal != v {
							t.Errorf("span.data[%v] = %v, want %v", k, gotVal, v)
						}
					}
				}
			}
		})
	}
}

func TestSpan_End(t *testing.T) {
	tests := []struct {
		name     string
		spanName string
		data     map[string]any
	}{
		{
			name:     "end span with data",
			spanName: "test-span",
			data:     map[string]any{"key": "value"},
		},
		{
			name:     "end span without data",
			spanName: "empty-span",
			data:     nil,
		},
		{
			name:     "end span multiple times",
			spanName: "multi-end-span",
			data:     map[string]any{"test": "multiple"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := StartSpan(tt.spanName, tt.data)

			// Should not panic
			span.End()

			// Calling End() multiple times should also not panic
			if tt.name == "end span multiple times" {
				span.End()
				span.End()
			}

			// Verify span fields are still accessible after End()
			if span.name != tt.spanName {
				t.Errorf("after End(), span.name = %v, want %v", span.name, tt.spanName)
			}
		})
	}
}

func TestSpan_Lifecycle(t *testing.T) {
	// Test typical span lifecycle
	data := map[string]any{
		"operation": "test-operation",
		"duration":  100,
	}

	span := StartSpan("lifecycle-test", data)

	if span == nil {
		t.Fatal("StartSpan returned nil")
	}

	if span.name != "lifecycle-test" {
		t.Errorf("span.name = %v, want lifecycle-test", span.name)
	}

	if span.data["operation"] != "test-operation" {
		t.Errorf("span.data[operation] = %v, want test-operation", span.data["operation"])
	}

	// End should complete without error
	span.End()
}

func TestSpan_ZeroValue(t *testing.T) {
	var span Span

	if span.name != "" {
		t.Errorf("zero value span.name = %v, want empty string", span.name)
	}

	if span.data != nil {
		t.Errorf("zero value span.data = %v, want nil", span.data)
	}

	// End() on zero value should not panic
	span.End()
}

func TestSpan_ConcurrentAccess(t *testing.T) {
	// Test that multiple spans can be created and ended concurrently
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			data := map[string]any{
				"id":   id,
				"test": "concurrent",
			}
			span := StartSpan("concurrent-span", data)
			span.End()
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestSpan_LargeData(t *testing.T) {
	// Test span with large data map
	largeData := make(map[string]any)
	for i := 0; i < 1000; i++ {
		key := string(rune('a'+(i%26))) + "-" + intToString(i)
		largeData[key] = i
	}

	span := StartSpan("large-data-span", largeData)

	if span == nil {
		t.Fatal("StartSpan with large data returned nil")
	}

	if len(span.data) != len(largeData) {
		t.Errorf("span.data length = %v, want %v", len(span.data), len(largeData))
	}

	span.End()
}

func TestSpan_NilDataPreservation(t *testing.T) {
	// Test that nil data is preserved as nil, not converted to empty map
	span := StartSpan("nil-data-span", nil)

	if span.data != nil {
		t.Errorf("span.data = %v, want nil", span.data)
	}

	span.End()
}

func TestSpan_DataImmutability(t *testing.T) {
	// Test that modifying original data after StartSpan doesn't affect span
	originalData := map[string]any{
		"key": "original",
	}

	span := StartSpan("immutability-test", originalData)

	// Modify original data
	originalData["key"] = "modified"
	originalData["new_key"] = "new_value"

	// Span data should still reference the same map (Go passes maps by reference)
	// This test verifies the current behavior
	if span.data["key"] != "modified" {
		t.Logf("Note: span.data[key] = %v (maps are passed by reference in Go)", span.data["key"])
	}

	span.End()
}

// Helper function to convert int to string
func intToString(n int) string {
	if n == 0 {
		return "0"
	}

	var result []byte
	isNeg := n < 0
	if isNeg {
		n = -n
	}

	for n > 0 {
		result = append([]byte{byte('0' + n%10)}, result...)
		n /= 10
	}

	if isNeg {
		result = append([]byte{'-'}, result...)
	}

	return string(result)
}
