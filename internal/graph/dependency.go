// Package graph provides dependency graph operations for agent startup ordering.
package graph

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

var (
	// ErrCycleDetected is returned when a dependency cycle is found in the graph.
	ErrCycleDetected = errors.New("dependency cycle detected")

	// ErrUnknownDependency is returned when an agent depends on an unknown agent.
	ErrUnknownDependency = errors.New("unknown dependency")
)

// Node represents an agent in the dependency graph.
type Node struct {
	Name         string
	Dependencies []string
}

// DependencyGraph manages agent startup dependencies using a directed acyclic graph.
// It supports topological sorting to determine the correct startup order.
type DependencyGraph struct {
	nodes map[string]*Node
	mu    sync.RWMutex
}

// NewDependencyGraph creates a new empty dependency graph.
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		nodes: make(map[string]*Node),
	}
}

// AddNode adds an agent to the dependency graph with its dependencies.
// If dependencies is nil or empty, the agent has no dependencies.
func (g *DependencyGraph) AddNode(name string, dependencies []string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Copy dependencies to avoid external mutation
	deps := make([]string, len(dependencies))
	copy(deps, dependencies)

	g.nodes[name] = &Node{
		Name:         name,
		Dependencies: deps,
	}
}

// Validate checks the graph for cycles and unknown dependencies.
// Returns an error if validation fails, nil otherwise.
func (g *DependencyGraph) Validate() error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Check for unknown dependencies
	for name, node := range g.nodes {
		for _, dep := range node.Dependencies {
			if _, exists := g.nodes[dep]; !exists {
				return fmt.Errorf("%w: agent %q depends on unknown agent %q",
					ErrUnknownDependency, name, dep)
			}
		}
	}

	// Check for cycles using DFS with coloring
	// Colors: 0=white (unvisited), 1=gray (visiting), 2=black (visited)
	colors := make(map[string]int)
	var stack []string

	var dfs func(name string) error
	dfs = func(name string) error {
		if colors[name] == 1 {
			// Found a cycle - build the cycle path for the error message
			cycleStart := -1
			for i, n := range stack {
				if n == name {
					cycleStart = i
					break
				}
			}
			cyclePath := append(stack[cycleStart:], name)
			return &CycleError{Path: cyclePath}
		}
		if colors[name] == 2 {
			return nil // Already fully explored
		}

		colors[name] = 1 // Mark as visiting
		stack = append(stack, name)

		for _, dep := range g.nodes[name].Dependencies {
			if err := dfs(dep); err != nil {
				return err
			}
		}

		colors[name] = 2 // Mark as visited
		stack = stack[:len(stack)-1]
		return nil
	}

	// Visit all nodes
	for name := range g.nodes {
		if colors[name] == 0 {
			if err := dfs(name); err != nil {
				return err
			}
		}
	}

	return nil
}

// TopologicalLevels returns agents grouped by dependency level using Kahn's algorithm.
// Level 0 contains agents with no dependencies.
// Level N contains agents whose dependencies are all in levels < N.
// Agents within the same level can be started in parallel.
//
// Returns an error if the graph contains cycles or unknown dependencies.
func (g *DependencyGraph) TopologicalLevels() ([][]string, error) {
	if err := g.Validate(); err != nil {
		return nil, err
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	if len(g.nodes) == 0 {
		return nil, nil
	}

	// Calculate in-degree for each node
	// In-degree = number of dependencies (edges pointing to this node)
	inDegree := make(map[string]int)
	for name := range g.nodes {
		inDegree[name] = len(g.nodes[name].Dependencies)
	}

	// Build reverse adjacency list (dep -> dependents)
	// This tells us which nodes to update when a dependency is satisfied
	dependents := make(map[string][]string)
	for name, node := range g.nodes {
		for _, dep := range node.Dependencies {
			dependents[dep] = append(dependents[dep], name)
		}
	}

	// Kahn's algorithm with level tracking
	var levels [][]string
	remaining := len(g.nodes)

	for remaining > 0 {
		// Find all nodes with in-degree 0 (no unsatisfied dependencies)
		var currentLevel []string
		for name, degree := range inDegree {
			if degree == 0 {
				currentLevel = append(currentLevel, name)
			}
		}

		if len(currentLevel) == 0 {
			// This should not happen if Validate() passed
			return nil, ErrCycleDetected
		}

		// Sort for deterministic ordering (helps with testing and debugging)
		sort.Strings(currentLevel)

		// Remove current level nodes and update in-degrees
		for _, name := range currentLevel {
			delete(inDegree, name)
			for _, dependent := range dependents[name] {
				inDegree[dependent]--
			}
		}

		levels = append(levels, currentLevel)
		remaining -= len(currentLevel)
	}

	return levels, nil
}

// NodeCount returns the number of nodes in the graph.
func (g *DependencyGraph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

// GetDependencies returns the dependencies for a given agent.
// Returns nil if the agent is not found.
func (g *DependencyGraph) GetDependencies(name string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, exists := g.nodes[name]
	if !exists {
		return nil
	}

	// Return a copy to prevent external mutation
	deps := make([]string, len(node.Dependencies))
	copy(deps, node.Dependencies)
	return deps
}
