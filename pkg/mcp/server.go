package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/aixgo-dev/aixgo/pkg/security"
)

// Server represents an MCP server that hosts tools
type Server struct {
	name      string
	tools     map[string]Tool
	transport Transport
	mu        sync.RWMutex

	// Security components
	securityConfig  *security.SecurityConfig
	authExtractor   security.AuthExtractor
	authenticator   security.Authenticator
	authorizer      security.Authorizer
	auditLogger     security.AuditLogger
	rateLimiter     *security.RateLimiter
	toolRateLimiter *security.ToolRateLimiter
	timeoutManager  *security.TimeoutManager
	debugMode       bool
}

// ServerOption is a functional option for configuring the server
type ServerOption func(*Server)

// WithAuthenticator sets the authenticator
func WithAuthenticator(auth security.Authenticator) ServerOption {
	return func(s *Server) {
		s.authenticator = auth
	}
}

// WithAuthorizer sets the authorizer
func WithAuthorizer(authz security.Authorizer) ServerOption {
	return func(s *Server) {
		s.authorizer = authz
	}
}

// WithAuditLogger sets the audit logger
func WithAuditLogger(logger security.AuditLogger) ServerOption {
	return func(s *Server) {
		s.auditLogger = logger
	}
}

// WithIntegratedAuditLogger sets the integrated audit logger with middleware support
func WithIntegratedAuditLogger(logger *security.IntegratedAuditLogger) ServerOption {
	return func(s *Server) {
		s.auditLogger = logger
	}
}

// WithRateLimit sets rate limiting
func WithRateLimit(requestsPerSecond float64, burst int) ServerOption {
	return func(s *Server) {
		s.rateLimiter = security.NewRateLimiter(requestsPerSecond, burst)
	}
}

// WithDebugMode enables debug mode (exposes internal errors)
func WithDebugMode(debug bool) ServerOption {
	return func(s *Server) {
		s.debugMode = debug
	}
}

// WithSecurityConfig sets the security configuration
func WithSecurityConfig(config *security.SecurityConfig) ServerOption {
	return func(s *Server) {
		s.securityConfig = config
	}
}

// NewServer creates a new MCP server with options
func NewServer(name string, opts ...ServerOption) *Server {
	server := &Server{
		name:  name,
		tools: make(map[string]Tool),
		// Default security components (no-op/allow-all for backward compatibility)
		authenticator:   security.NewNoAuthAuthenticator(),
		authorizer:      security.NewAllowAllAuthorizer(),
		auditLogger:     security.NewNoOpAuditLogger(),
		toolRateLimiter: security.NewToolRateLimiter(),
		timeoutManager:  security.NewTimeoutManager(30 * time.Second),
		debugMode:       false,
	}

	// Apply options
	for _, opt := range opts {
		opt(server)
	}

	// If SecurityConfig is provided, initialize auth extractor
	if server.securityConfig != nil {
		// Validate config
		if err := server.securityConfig.Validate(); err != nil {
			panic(fmt.Sprintf("invalid security configuration: %v", err))
		}

		// Print security summary
		server.securityConfig.PrintSecuritySummary()

		// Create auth extractor from config
		extractor, err := security.NewAuthExtractorFromConfig(server.securityConfig)
		if err != nil {
			panic(fmt.Sprintf("failed to create auth extractor: %v", err))
		}
		server.authExtractor = extractor

		// Configure authorization if enabled
		if server.securityConfig.Authorization != nil && server.securityConfig.Authorization.Enabled {
			if server.authorizer == nil || fmt.Sprintf("%T", server.authorizer) == "*security.AllowAllAuthorizer" {
				// Replace default allow-all with RBAC
				server.authorizer = security.NewRBACAuthorizer()
			}
		}

		// Configure audit logging if enabled
		if server.securityConfig.Audit != nil && server.securityConfig.Audit.Enabled {
			if server.auditLogger == nil || fmt.Sprintf("%T", server.auditLogger) == "*security.NoOpAuditLogger" {
				// Use JSON audit logger by default
				server.auditLogger = security.NewJSONAuditLogger()
			}
		}
	}

	return server
}

// RegisterTool registers a tool with the server
func (s *Server) RegisterTool(tool Tool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	if tool.Handler == nil {
		return fmt.Errorf("tool handler cannot be nil")
	}

	if _, exists := s.tools[tool.Name]; exists {
		return fmt.Errorf("tool %s already registered", tool.Name)
	}

	s.tools[tool.Name] = tool
	return nil
}

// RegisterTypedTool registers a tool with type-safe handler
func RegisterTypedTool[TInput any, TOutput any](
	s *Server,
	name string,
	description string,
	handler func(context.Context, TInput) (TOutput, error),
) error {
	// Wrapper to convert typed handler to generic handler
	genericHandler := func(ctx context.Context, args Args) (any, error) {
		// TODO: Unmarshal args into TInput using reflection/JSON
		// For now, simple implementation
		var input TInput
		return handler(ctx, input)
	}

	return s.RegisterTool(Tool{
		Name:        name,
		Description: description,
		Handler:     genericHandler,
		Schema:      make(Schema), // TODO: Generate from TInput using reflection
	})
}

// ListTools returns all registered tools
func (s *Server) ListTools() []Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]Tool, 0, len(s.tools))
	for _, tool := range s.tools {
		tools = append(tools, tool)
	}
	return tools
}

// CallTool executes a tool by name with full security checks
func (s *Server) CallTool(ctx context.Context, params CallToolParams) (*CallToolResult, error) {
	// Validate tool name first
	if err := security.ValidateToolName(params.Name); err != nil {
		if s.auditLogger != nil {
			s.auditLogger.LogToolExecution(ctx, params.Name, params.Arguments, nil, err)
		}
		return s.errorResult(security.ErrCodeValidation,
			"invalid tool name", err)
	}

	// Get authentication context
	// Note: Auth extraction typically happens in HTTP middleware layer
	// Here we just check if auth context exists in the context
	authCtx, err := security.GetAuthContext(ctx)
	var principal *security.Principal
	if err == nil && authCtx != nil {
		principal = authCtx.Principal
	}

	// Get client ID for rate limiting
	clientID := "anonymous"
	if principal != nil {
		clientID = principal.ID
	}

	// Check global rate limit
	if s.rateLimiter != nil && !s.rateLimiter.Allow(clientID) {
		if s.auditLogger != nil {
			s.auditLogger.LogToolExecution(ctx, params.Name, params.Arguments, nil,
				fmt.Errorf("rate limit exceeded"))
		}
		return s.errorResult(security.ErrCodeRateLimit, "rate limit exceeded", nil)
	}

	// Get tool
	s.mu.RLock()
	tool, exists := s.tools[params.Name]
	s.mu.RUnlock()

	if !exists {
		if s.auditLogger != nil {
			s.auditLogger.LogToolExecution(ctx, params.Name, params.Arguments, nil,
				fmt.Errorf("tool not found"))
		}
		return s.errorResult(security.ErrCodeToolNotFound,
			fmt.Sprintf("tool not found: %s", params.Name), nil)
	}

	// Check per-tool rate limit
	if !s.toolRateLimiter.Allow(params.Name) {
		if s.auditLogger != nil {
			s.auditLogger.LogToolExecution(ctx, params.Name, params.Arguments, nil,
				fmt.Errorf("tool rate limit exceeded"))
		}
		return s.errorResult(security.ErrCodeRateLimit,
			fmt.Sprintf("rate limit exceeded for tool: %s", params.Name), nil)
	}

	// Check authorization
	if principal != nil {
		if err := s.authorizer.Authorize(ctx, principal, params.Name, tool.RequiredPermission); err != nil {
			if s.auditLogger != nil {
				s.auditLogger.LogToolExecution(ctx, params.Name, params.Arguments, nil, err)
			}
			return s.errorResult(security.ErrCodeForbidden, "access denied", err)
		}
	}

	// Validate argument types
	if err := security.ValidateJSONObject(params.Arguments); err != nil {
		if s.auditLogger != nil {
			s.auditLogger.LogToolExecution(ctx, params.Name, params.Arguments, nil, err)
		}
		return s.errorResult(security.ErrCodeValidation,
			"invalid arguments: must be a JSON object", err)
	}

	// Apply default string validation to all string arguments
	if err := validateToolArguments(params.Arguments); err != nil {
		if s.auditLogger != nil {
			s.auditLogger.LogToolExecution(ctx, params.Name, params.Arguments, nil, err)
		}
		return s.errorResult(security.ErrCodeValidation,
			"argument validation failed", err)
	}

	// Validate arguments against schema
	if len(tool.Schema) > 0 {
		if err := tool.Schema.ValidateArgs(Args(params.Arguments)); err != nil {
			if s.auditLogger != nil {
				s.auditLogger.LogToolExecution(ctx, params.Name, params.Arguments, nil, err)
			}
			return s.errorResult(security.ErrCodeValidation,
				"argument validation failed", err)
		}
	}

	// Set execution timeout
	toolCtx, cancel := s.timeoutManager.WithTimeout(ctx, params.Name)
	defer cancel()

	// Execute tool handler
	result, err := tool.Handler(toolCtx, Args(params.Arguments))

	// Log execution
	if s.auditLogger != nil {
		s.auditLogger.LogToolExecution(ctx, params.Name, params.Arguments, result, err)
	}

	if err != nil {
		return s.errorResult(security.ErrCodeToolExecution, "tool execution failed", err)
	}

	// Convert result to content
	content := formatResult(result)
	return &CallToolResult{
		Content: []Content{content},
		IsError: false,
	}, nil
}

// errorResult creates an error result with sanitized error messages
func (s *Server) errorResult(code security.ErrorCode, message string, err error) (*CallToolResult, error) {
	errorText := message

	if err != nil {
		sanitized := security.SanitizeErrorWithCode(err, code, message, s.debugMode)
		if sanitized != nil {
			errorText = sanitized.Message
		}

		if s.debugMode {
			errorText = fmt.Sprintf("%s: %v", message, err)
		}
	}

	return &CallToolResult{
		Content: []Content{{
			Type: "text",
			Text: errorText,
		}},
		IsError: true,
	}, nil
}

// Serve starts the MCP server with the given transport
func (s *Server) Serve(transport Transport) error {
	s.mu.Lock()
	s.transport = transport
	s.mu.Unlock()

	// For local transport, server is embedded (no-op)
	// For gRPC transport, start gRPC server
	return nil
}

// Close stops the server
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.transport != nil {
		return s.transport.Close()
	}
	return nil
}

// Name returns the server name
func (s *Server) Name() string {
	return s.name
}

// GetAuthExtractor returns the configured auth extractor
// This is useful for HTTP middleware that needs to extract auth from requests
func (s *Server) GetAuthExtractor() security.AuthExtractor {
	return s.authExtractor
}

// GetSecurityConfig returns the security configuration
func (s *Server) GetSecurityConfig() *security.SecurityConfig {
	return s.securityConfig
}

// formatResult converts a tool result to MCP Content
func formatResult(result any) Content {
	switch v := result.(type) {
	case string:
		return Content{Type: "text", Text: v}
	case map[string]any, []any:
		// Convert to JSON
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return Content{Type: "text", Text: fmt.Sprintf("%v", v)}
		}
		return Content{Type: "text", Text: string(jsonBytes)}
	default:
		return Content{Type: "text", Text: fmt.Sprintf("%v", v)}
	}
}

// validateToolArguments applies default security validation to tool arguments
func validateToolArguments(args map[string]any) error {
	// Create a default string validator with security checks
	stringValidator := &security.StringValidator{
		MaxLength:            10000, // 10KB max per string argument
		DisallowNullBytes:    true,
		DisallowControlChars: true,
		CheckSQLInjection:    true,
	}

	// Recursively validate all string values
	return validateMapRecursive(args, stringValidator, 0, 10) // Max depth of 10
}

// validateMapRecursive recursively validates all string values in a map
func validateMapRecursive(m map[string]any, validator *security.StringValidator, depth, maxDepth int) error {
	if depth > maxDepth {
		return fmt.Errorf("maximum nesting depth exceeded")
	}

	for key, value := range m {
		// Validate the key itself
		if err := validator.Validate(key); err != nil {
			return fmt.Errorf("invalid key %q: %w", key, err)
		}

		// Validate the value based on type
		switch v := value.(type) {
		case string:
			if err := validator.Validate(v); err != nil {
				return fmt.Errorf("invalid value for key %q: %w", key, err)
			}
		case map[string]any:
			if err := validateMapRecursive(v, validator, depth+1, maxDepth); err != nil {
				return fmt.Errorf("invalid nested object in key %q: %w", key, err)
			}
		case []any:
			if err := validateSliceRecursive(v, validator, depth+1, maxDepth); err != nil {
				return fmt.Errorf("invalid array in key %q: %w", key, err)
			}
			// Numbers, booleans, and null are safe - no validation needed
		}
	}

	return nil
}

// validateSliceRecursive recursively validates all string values in a slice
func validateSliceRecursive(s []any, validator *security.StringValidator, depth, maxDepth int) error {
	if depth > maxDepth {
		return fmt.Errorf("maximum nesting depth exceeded")
	}

	for i, value := range s {
		switch v := value.(type) {
		case string:
			if err := validator.Validate(v); err != nil {
				return fmt.Errorf("invalid value at index %d: %w", i, err)
			}
		case map[string]any:
			if err := validateMapRecursive(v, validator, depth+1, maxDepth); err != nil {
				return fmt.Errorf("invalid nested object at index %d: %w", i, err)
			}
		case []any:
			if err := validateSliceRecursive(v, validator, depth+1, maxDepth); err != nil {
				return fmt.Errorf("invalid nested array at index %d: %w", i, err)
			}
			// Numbers, booleans, and null are safe - no validation needed
		}
	}

	return nil
}
