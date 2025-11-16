package agent

import (
	"fmt"
)

// CreateAgent creates an agent using the default registry
func CreateAgent(def AgentDef, rt Runtime) (Agent, error) {
	return CreateAgentWithRegistry(def, rt, defaultRegistry)
}

// CreateAgentWithRegistry creates an agent using a custom registry (useful for testing)
func CreateAgentWithRegistry(def AgentDef, rt Runtime, registry Registry) (Agent, error) {
	if factory, ok := registry.GetFactory(def.Role); ok {
		return factory(def, rt)
	}

	return nil, fmt.Errorf("unknown role: %s", def.Role)
}
