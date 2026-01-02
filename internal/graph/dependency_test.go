package graph

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDependencyGraph(t *testing.T) {
	g := NewDependencyGraph()
	assert.NotNil(t, g)
	assert.Equal(t, 0, g.NodeCount())
}

func TestAddNode(t *testing.T) {
	g := NewDependencyGraph()

	g.AddNode("a", nil)
	g.AddNode("b", []string{"a"})
	g.AddNode("c", []string{"a", "b"})

	assert.Equal(t, 3, g.NodeCount())
	assert.Empty(t, g.GetDependencies("a"))
	assert.Equal(t, []string{"a"}, g.GetDependencies("b"))
	assert.Equal(t, []string{"a", "b"}, g.GetDependencies("c"))
}

func TestAddNode_MutationProtection(t *testing.T) {
	g := NewDependencyGraph()

	deps := []string{"a", "b"}
	g.AddNode("c", deps)

	// Modify original slice
	deps[0] = "x"

	// Graph should have original values
	assert.Equal(t, []string{"a", "b"}, g.GetDependencies("c"))
}

func TestGetDependencies_NotFound(t *testing.T) {
	g := NewDependencyGraph()
	assert.Nil(t, g.GetDependencies("unknown"))
}

func TestValidate_NoDependencies(t *testing.T) {
	g := NewDependencyGraph()
	g.AddNode("a", nil)
	g.AddNode("b", nil)
	g.AddNode("c", nil)

	err := g.Validate()
	assert.NoError(t, err)
}

func TestValidate_ValidDependencies(t *testing.T) {
	g := NewDependencyGraph()
	g.AddNode("a", nil)
	g.AddNode("b", []string{"a"})
	g.AddNode("c", []string{"a", "b"})

	err := g.Validate()
	assert.NoError(t, err)
}

func TestValidate_UnknownDependency(t *testing.T) {
	g := NewDependencyGraph()
	g.AddNode("a", []string{"unknown"})

	err := g.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnknownDependency))
	assert.Contains(t, err.Error(), "unknown")
}

func TestValidate_SelfDependency(t *testing.T) {
	g := NewDependencyGraph()
	g.AddNode("a", []string{"a"})

	err := g.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCycleDetected))
}

func TestValidate_SimpleCycle(t *testing.T) {
	g := NewDependencyGraph()
	g.AddNode("a", []string{"b"})
	g.AddNode("b", []string{"a"})

	err := g.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCycleDetected))

	var cycleErr *CycleError
	require.True(t, errors.As(err, &cycleErr))
	assert.Len(t, cycleErr.Path, 3) // a -> b -> a
}

func TestValidate_LongCycle(t *testing.T) {
	g := NewDependencyGraph()
	g.AddNode("a", []string{"c"})
	g.AddNode("b", []string{"a"})
	g.AddNode("c", []string{"b"})

	err := g.Validate()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCycleDetected))
}

func TestTopologicalLevels_Empty(t *testing.T) {
	g := NewDependencyGraph()

	levels, err := g.TopologicalLevels()
	require.NoError(t, err)
	assert.Nil(t, levels)
}

func TestTopologicalLevels_NoDependencies(t *testing.T) {
	g := NewDependencyGraph()
	g.AddNode("a", nil)
	g.AddNode("b", nil)
	g.AddNode("c", nil)

	levels, err := g.TopologicalLevels()
	require.NoError(t, err)
	require.Len(t, levels, 1)
	assert.ElementsMatch(t, []string{"a", "b", "c"}, levels[0])
}

func TestTopologicalLevels_LinearChain(t *testing.T) {
	g := NewDependencyGraph()
	g.AddNode("a", nil)
	g.AddNode("b", []string{"a"})
	g.AddNode("c", []string{"b"})

	levels, err := g.TopologicalLevels()
	require.NoError(t, err)
	require.Len(t, levels, 3)

	assert.Equal(t, []string{"a"}, levels[0])
	assert.Equal(t, []string{"b"}, levels[1])
	assert.Equal(t, []string{"c"}, levels[2])
}

func TestTopologicalLevels_Diamond(t *testing.T) {
	// Diamond pattern:
	//     a
	//    / \
	//   b   c
	//    \ /
	//     d
	g := NewDependencyGraph()
	g.AddNode("a", nil)
	g.AddNode("b", []string{"a"})
	g.AddNode("c", []string{"a"})
	g.AddNode("d", []string{"b", "c"})

	levels, err := g.TopologicalLevels()
	require.NoError(t, err)
	require.Len(t, levels, 3)

	assert.Equal(t, []string{"a"}, levels[0])
	assert.ElementsMatch(t, []string{"b", "c"}, levels[1])
	assert.Equal(t, []string{"d"}, levels[2])
}

func TestTopologicalLevels_MultipleRoots(t *testing.T) {
	// Two independent chains:
	// a -> b
	// x -> y
	g := NewDependencyGraph()
	g.AddNode("a", nil)
	g.AddNode("b", []string{"a"})
	g.AddNode("x", nil)
	g.AddNode("y", []string{"x"})

	levels, err := g.TopologicalLevels()
	require.NoError(t, err)
	require.Len(t, levels, 2)

	assert.ElementsMatch(t, []string{"a", "x"}, levels[0])
	assert.ElementsMatch(t, []string{"b", "y"}, levels[1])
}

func TestTopologicalLevels_ComplexGraph(t *testing.T) {
	// Complex dependency pattern:
	// Level 0: doc-analyzer, risk-assessor, validation-agent
	// Level 1: orchestrator (depends on all level 0)
	// Level 2: reporter (depends on orchestrator)
	g := NewDependencyGraph()
	g.AddNode("doc-analyzer", nil)
	g.AddNode("risk-assessor", nil)
	g.AddNode("validation-agent", nil)
	g.AddNode("orchestrator", []string{"doc-analyzer", "risk-assessor", "validation-agent"})
	g.AddNode("reporter", []string{"orchestrator"})

	levels, err := g.TopologicalLevels()
	require.NoError(t, err)
	require.Len(t, levels, 3)

	assert.ElementsMatch(t, []string{"doc-analyzer", "risk-assessor", "validation-agent"}, levels[0])
	assert.Equal(t, []string{"orchestrator"}, levels[1])
	assert.Equal(t, []string{"reporter"}, levels[2])
}

func TestTopologicalLevels_Deterministic(t *testing.T) {
	// Run multiple times to verify deterministic ordering
	for i := 0; i < 10; i++ {
		g := NewDependencyGraph()
		g.AddNode("c", nil)
		g.AddNode("a", nil)
		g.AddNode("b", nil)

		levels, err := g.TopologicalLevels()
		require.NoError(t, err)
		require.Len(t, levels, 1)

		// Should be alphabetically sorted
		assert.Equal(t, []string{"a", "b", "c"}, levels[0])
	}
}

func TestTopologicalLevels_CycleError(t *testing.T) {
	g := NewDependencyGraph()
	g.AddNode("a", []string{"b"})
	g.AddNode("b", []string{"a"})

	levels, err := g.TopologicalLevels()
	assert.Nil(t, levels)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCycleDetected))
}

func TestTopologicalLevels_UnknownDependencyError(t *testing.T) {
	g := NewDependencyGraph()
	g.AddNode("a", []string{"unknown"})

	levels, err := g.TopologicalLevels()
	assert.Nil(t, levels)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnknownDependency))
}

func TestCycleError_Error(t *testing.T) {
	err := &CycleError{Path: []string{"a", "b", "c", "a"}}
	assert.Equal(t, "dependency cycle detected: a -> b -> c -> a", err.Error())
}

func TestCycleError_Unwrap(t *testing.T) {
	err := &CycleError{Path: []string{"a", "b", "a"}}
	assert.True(t, errors.Is(err, ErrCycleDetected))
}

func BenchmarkTopologicalLevels_10Agents(b *testing.B) {
	g := NewDependencyGraph()
	for i := 0; i < 10; i++ {
		var deps []string
		if i > 0 {
			deps = []string{string(rune('a' + i - 1))}
		}
		g.AddNode(string(rune('a'+i)), deps)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = g.TopologicalLevels()
	}
}

func BenchmarkTopologicalLevels_50Agents(b *testing.B) {
	g := NewDependencyGraph()

	// Create 50 agents with complex dependencies
	// Level 0: 10 base agents
	// Levels 1-4: Each level has 10 agents depending on previous level
	for i := 0; i < 10; i++ {
		g.AddNode(agentName(0, i), nil)
	}
	for level := 1; level < 5; level++ {
		for i := 0; i < 10; i++ {
			deps := make([]string, 0, 5)
			for j := 0; j < 5; j++ {
				deps = append(deps, agentName(level-1, (i+j)%10))
			}
			g.AddNode(agentName(level, i), deps)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = g.TopologicalLevels()
	}
}

func agentName(level, index int) string {
	return string(rune('a'+level)) + string(rune('0'+index))
}
