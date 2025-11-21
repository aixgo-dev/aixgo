package mcp

import (
	"context"
	"fmt"
	"sync"
)

var (
	// Global registry of local servers
	localServers = make(map[string]*Server)
	localMu      sync.RWMutex
)

// RegisterLocalServer registers a server for local transport
func RegisterLocalServer(server *Server) {
	localMu.Lock()
	defer localMu.Unlock()
	localServers[server.name] = server
}

// ClearLocalServers clears all registered local servers (for testing)
func ClearLocalServers() {
	localMu.Lock()
	defer localMu.Unlock()
	localServers = make(map[string]*Server)
}

// LocalTransport implements in-process MCP communication
type LocalTransport struct {
	serverName string
	server     *Server
}

// NewLocalTransport creates a new local transport
func NewLocalTransport(serverName string) (*LocalTransport, error) {
	localMu.RLock()
	server, exists := localServers[serverName]
	localMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("local server not found: %s", serverName)
	}

	return &LocalTransport{
		serverName: serverName,
		server:     server,
	}, nil
}

// Send sends a request to the local server
func (t *LocalTransport) Send(ctx context.Context, method string, params any) (any, error) {
	switch method {
	case "tools/list":
		return t.server.ListTools(), nil

	case "tools/call":
		callParams, ok := params.(CallToolParams)
		if !ok {
			return nil, fmt.Errorf("invalid params type for tools/call")
		}
		return t.server.CallTool(ctx, callParams)

	default:
		return nil, fmt.Errorf("unsupported method: %s", method)
	}
}

// Close closes the transport
func (t *LocalTransport) Close() error {
	// Local transport doesn't need cleanup
	return nil
}
