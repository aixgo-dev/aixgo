package cost

import (
	"sync"
	"testing"
)

func TestGetPricing_NoConcurrentModification(t *testing.T) {
	calc := NewCalculator()

	// Add a test pricing
	testPricing := &ModelPricing{
		Model:       "test-model",
		InputPer1M:  10.0,
		OutputPer1M: 20.0,
	}
	calc.AddPricing(testPricing)

	var wg sync.WaitGroup
	numGoroutines := 100

	// Test concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pricing, ok := calc.GetPricing("test-model")
			if !ok {
				t.Errorf("expected to find pricing")
				return
			}
			// Modify the returned pricing (should not affect the calculator)
			pricing.InputPer1M = 999.0
		}()
	}

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			calc.AddPricing(&ModelPricing{
				Model:       "concurrent-model",
				InputPer1M:  float64(id),
				OutputPer1M: float64(id * 2),
			})
		}(i)
	}

	wg.Wait()

	// Verify original pricing is unchanged
	pricing, ok := calc.GetPricing("test-model")
	if !ok {
		t.Fatal("expected to find original pricing")
	}
	if pricing.InputPer1M != 10.0 {
		t.Errorf("expected InputPer1M=10.0, got %f", pricing.InputPer1M)
	}
}

func TestGetPricing_PrefixMatchDeterministic(t *testing.T) {
	calc := &Calculator{
		pricing: make(map[string]*ModelPricing),
	}

	// Add models with overlapping prefixes
	calc.AddPricing(&ModelPricing{
		Model:       "test-model",
		InputPer1M:  30.0,
		OutputPer1M: 60.0,
	})
	calc.AddPricing(&ModelPricing{
		Model:       "test-model-pro",
		InputPer1M:  2.5,
		OutputPer1M: 10.0,
	})

	// Test that longer prefix is matched first
	pricing, ok := calc.GetPricing("test-model-pro-v2")
	if !ok {
		t.Fatal("expected to find pricing")
	}
	// Should match "test-model-pro" (longer prefix) not "test-model"
	if pricing.InputPer1M != 2.5 {
		t.Errorf("expected test-model-pro pricing (2.5), got %f", pricing.InputPer1M)
	}
}

func TestGetPricing_ExactMatch(t *testing.T) {
	calc := NewCalculator()

	calc.AddPricing(&ModelPricing{
		Model:       "exact-model",
		InputPer1M:  5.0,
		OutputPer1M: 10.0,
	})

	pricing, ok := calc.GetPricing("exact-model")
	if !ok {
		t.Fatal("expected to find pricing")
	}
	if pricing.InputPer1M != 5.0 {
		t.Errorf("expected InputPer1M=5.0, got %f", pricing.InputPer1M)
	}
}

func TestGetPricing_NotFound(t *testing.T) {
	calc := NewCalculator()

	pricing, ok := calc.GetPricing("nonexistent-model")
	if ok {
		t.Error("expected not to find pricing")
	}
	if pricing != nil {
		t.Error("expected nil pricing")
	}
}
