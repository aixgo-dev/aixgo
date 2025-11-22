package mcp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	pb "github.com/aixgo-dev/aixgo/proto/mcp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCTransportConfig holds gRPC transport configuration
type GRPCTransportConfig struct {
	Address string
	TLS     *TLSConfig
}

// TLSConfig holds TLS configuration for gRPC
type TLSConfig struct {
	Enabled            bool
	CertFile           string
	KeyFile            string
	CAFile             string
	InsecureSkipVerify bool
	ServerName         string
	ClientAuth         tls.ClientAuthType
}

// GRPCTransport implements Transport for gRPC communication
type GRPCTransport struct {
	config     GRPCTransportConfig
	conn       *grpc.ClientConn
	client     pb.MCPServiceClient
	mu         sync.Mutex
	connected  bool
	recvChan   chan []byte
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// NewGRPCTransport creates a new gRPC client transport
func NewGRPCTransport(config ServerConfig) (Transport, error) {
	grpcConfig := GRPCTransportConfig{
		Address: config.Address,
	}

	if config.TLS {
		grpcConfig.TLS = &TLSConfig{
			Enabled: true,
		}
	}

	return NewGRPCTransportWithConfig(grpcConfig)
}

// NewGRPCTransportWithConfig creates a new gRPC transport with detailed config
func NewGRPCTransportWithConfig(config GRPCTransportConfig) (*GRPCTransport, error) {
	if config.Address == "" {
		return nil, errors.New("gRPC address is required")
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &GRPCTransport{
		config:     config,
		recvChan:   make(chan []byte, 100),
		ctx:        ctx,
		cancelFunc: cancel,
	}, nil
}

// Connect establishes the gRPC connection
func (t *GRPCTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return nil
	}

	opts := []grpc.DialOption{}

	if t.config.TLS != nil && t.config.TLS.Enabled {
		tlsConfig, err := t.buildTLSConfig()
		if err != nil {
			return fmt.Errorf("failed to build TLS config: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(t.config.Address, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	t.conn = conn
	t.client = pb.NewMCPServiceClient(conn)
	t.connected = true

	return nil
}

// buildTLSConfig creates a TLS configuration
func (t *GRPCTransport) buildTLSConfig() (*tls.Config, error) {
	tlsCfg := t.config.TLS
	if tlsCfg == nil {
		return nil, errors.New("TLS config is nil")
	}

	// SECURITY: Prevent InsecureSkipVerify in production environments
	// This enforces certificate verification when running in production
	if tlsCfg.InsecureSkipVerify {
		env := strings.ToLower(os.Getenv("ENVIRONMENT"))
		if env == "production" || env == "prod" {
			return nil, fmt.Errorf("SECURITY: InsecureSkipVerify cannot be enabled in production environment (ENVIRONMENT=%s)", env)
		}

		// Log warning for non-production environments
		log.Printf("WARNING: TLS certificate verification is disabled (InsecureSkipVerify=true). " +
			"This is a security risk and should NEVER be used in production. " +
			"Connections are vulnerable to man-in-the-middle attacks. " +
			"Current ENVIRONMENT=%s", env)
	}

	config := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: tlsCfg.InsecureSkipVerify,
		ServerName:         tlsCfg.ServerName,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		},
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
			tls.CurveP384,
		},
	}

	// Load client certificate if provided (for mTLS)
	if tlsCfg.CertFile != "" && tlsCfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(tlsCfg.CertFile, tlsCfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		config.Certificates = []tls.Certificate{cert}
	}

	// Load CA certificate if provided
	if tlsCfg.CAFile != "" {
		caCert, err := os.ReadFile(tlsCfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, errors.New("failed to append CA certificate")
		}
		config.RootCAs = caCertPool
	}

	return config, nil
}

// Send sends a request and returns the response
func (t *GRPCTransport) Send(ctx context.Context, method string, params any) (any, error) {
	t.mu.Lock()
	if !t.connected {
		t.mu.Unlock()
		if err := t.Connect(ctx); err != nil {
			return nil, err
		}
		t.mu.Lock()
	}
	client := t.client
	t.mu.Unlock()

	switch method {
	case "initialize":
		return t.handleInitialize(ctx, client, params)
	case "tools/list":
		return t.handleListTools(ctx, client, params)
	case "tools/call":
		return t.handleCallTool(ctx, client, params)
	case "ping":
		return t.handlePing(ctx, client, params)
	default:
		return nil, fmt.Errorf("unsupported method: %s", method)
	}
}

// handleInitialize handles the initialize RPC call
func (t *GRPCTransport) handleInitialize(ctx context.Context, client pb.MCPServiceClient, params any) (any, error) {
	req := &pb.InitializeRequest{
		ProtocolVersion: "2024-11-05",
		ClientInfo: &pb.ClientInfo{
			Name:    "aixgo-client",
			Version: "1.0.0",
		},
		Capabilities: &pb.Capabilities{
			SupportsStreaming:     true,
			SupportsCancellation:  true,
			SupportsProgress:      true,
			SupportedContentTypes: []string{"text/plain", "application/json"},
		},
	}

	// Override with provided params if available
	if p, ok := params.(map[string]any); ok {
		if v, ok := p["protocolVersion"].(string); ok {
			req.ProtocolVersion = v
		}
		if ci, ok := p["clientInfo"].(map[string]any); ok {
			if name, ok := ci["name"].(string); ok {
				req.ClientInfo.Name = name
			}
			if version, ok := ci["version"].(string); ok {
				req.ClientInfo.Version = version
			}
		}
	}

	resp, err := client.Initialize(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("initialize RPC failed: %w", err)
	}

	return map[string]any{
		"protocolVersion": resp.ProtocolVersion,
		"serverInfo": map[string]any{
			"name":    resp.ServerInfo.GetName(),
			"version": resp.ServerInfo.GetVersion(),
		},
		"capabilities": map[string]any{
			"supportsStreaming":     resp.Capabilities.GetSupportsStreaming(),
			"supportsCancellation":  resp.Capabilities.GetSupportsCancellation(),
			"supportsProgress":      resp.Capabilities.GetSupportsProgress(),
			"supportedContentTypes": resp.Capabilities.GetSupportedContentTypes(),
		},
	}, nil
}

// handleListTools handles the tools/list RPC call
func (t *GRPCTransport) handleListTools(ctx context.Context, client pb.MCPServiceClient, params any) (any, error) {
	req := &pb.ListToolsRequest{}

	// Handle cursor for pagination
	if p, ok := params.(map[string]any); ok {
		if cursor, ok := p["cursor"].(string); ok {
			req.Cursor = cursor
		}
	}

	resp, err := client.ListTools(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list tools RPC failed: %w", err)
	}

	tools := make([]map[string]any, 0, len(resp.Tools))
	for _, tool := range resp.Tools {
		toolMap := map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
		}

		if tool.InputSchema != nil {
			schema := map[string]any{
				"type": tool.InputSchema.Type,
			}
			if len(tool.InputSchema.Properties) > 0 {
				props := make(map[string]any)
				for k, v := range tool.InputSchema.Properties {
					props[k] = map[string]any{
						"type":        v.Type,
						"description": v.Description,
					}
				}
				schema["properties"] = props
			}
			if len(tool.InputSchema.Required) > 0 {
				schema["required"] = tool.InputSchema.Required
			}
			toolMap["inputSchema"] = schema
		}

		tools = append(tools, toolMap)
	}

	result := map[string]any{
		"tools": tools,
	}
	if resp.NextCursor != "" {
		result["nextCursor"] = resp.NextCursor
	}

	return result, nil
}

// handleCallTool handles the tools/call RPC call
func (t *GRPCTransport) handleCallTool(ctx context.Context, client pb.MCPServiceClient, params any) (any, error) {
	req := &pb.CallToolRequest{}

	p, ok := params.(map[string]any)
	if !ok {
		return nil, errors.New("invalid params for tools/call")
	}

	name, ok := p["name"].(string)
	if !ok {
		return nil, errors.New("missing tool name")
	}
	req.Name = name

	// Convert arguments to protobuf Values
	if args, ok := p["arguments"].(map[string]any); ok {
		req.Arguments = make(map[string]*pb.Value)
		for k, v := range args {
			req.Arguments[k] = toProtoValue(v)
		}
	}

	resp, err := client.CallTool(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("call tool RPC failed: %w", err)
	}

	content := make([]map[string]any, 0, len(resp.Content))
	for _, c := range resp.Content {
		contentMap := map[string]any{
			"type": c.Type,
		}
		if c.Text != "" {
			contentMap["text"] = c.Text
		}
		if len(c.Data) > 0 {
			contentMap["data"] = c.Data
		}
		if len(c.Metadata) > 0 {
			contentMap["metadata"] = c.Metadata
		}
		content = append(content, contentMap)
	}

	return map[string]any{
		"content": content,
		"isError": resp.IsError,
	}, nil
}

// handlePing handles the ping RPC call
func (t *GRPCTransport) handlePing(ctx context.Context, client pb.MCPServiceClient, params any) (any, error) {
	req := &pb.PingRequest{
		Timestamp: time.Now().UnixNano(),
	}

	resp, err := client.Ping(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ping RPC failed: %w", err)
	}

	return map[string]any{
		"timestamp": resp.Timestamp,
		"status":    resp.Status,
	}, nil
}

// toProtoValue converts a Go value to a protobuf Value
func toProtoValue(v any) *pb.Value {
	if v == nil {
		return &pb.Value{Kind: &pb.Value_NullValue{NullValue: pb.NullValue_NULL_VALUE}}
	}

	switch val := v.(type) {
	case string:
		return &pb.Value{Kind: &pb.Value_StringValue{StringValue: val}}
	case float64:
		return &pb.Value{Kind: &pb.Value_NumberValue{NumberValue: val}}
	case float32:
		return &pb.Value{Kind: &pb.Value_NumberValue{NumberValue: float64(val)}}
	case int:
		return &pb.Value{Kind: &pb.Value_NumberValue{NumberValue: float64(val)}}
	case int64:
		return &pb.Value{Kind: &pb.Value_NumberValue{NumberValue: float64(val)}}
	case int32:
		return &pb.Value{Kind: &pb.Value_NumberValue{NumberValue: float64(val)}}
	case bool:
		return &pb.Value{Kind: &pb.Value_BoolValue{BoolValue: val}}
	case []any:
		listVal := &pb.ListValue{Values: make([]*pb.Value, len(val))}
		for i, item := range val {
			listVal.Values[i] = toProtoValue(item)
		}
		return &pb.Value{Kind: &pb.Value_ListValue{ListValue: listVal}}
	case map[string]any:
		structVal := &pb.StructValue{Fields: make(map[string]*pb.Value)}
		for k, item := range val {
			structVal.Fields[k] = toProtoValue(item)
		}
		return &pb.Value{Kind: &pb.Value_StructValue{StructValue: structVal}}
	default:
		return &pb.Value{Kind: &pb.Value_StringValue{StringValue: fmt.Sprintf("%v", val)}}
	}
}

// fromProtoValue converts a protobuf Value to a Go value
func fromProtoValue(v *pb.Value) any {
	if v == nil {
		return nil
	}

	switch kind := v.Kind.(type) {
	case *pb.Value_NullValue:
		return nil
	case *pb.Value_StringValue:
		return kind.StringValue
	case *pb.Value_NumberValue:
		return kind.NumberValue
	case *pb.Value_BoolValue:
		return kind.BoolValue
	case *pb.Value_ListValue:
		if kind.ListValue == nil {
			return []any{}
		}
		result := make([]any, len(kind.ListValue.Values))
		for i, item := range kind.ListValue.Values {
			result[i] = fromProtoValue(item)
		}
		return result
	case *pb.Value_StructValue:
		if kind.StructValue == nil {
			return map[string]any{}
		}
		result := make(map[string]any)
		for k, item := range kind.StructValue.Fields {
			result[k] = fromProtoValue(item)
		}
		return result
	default:
		return nil
	}
}

// StreamCallTool executes a tool with streaming response
func (t *GRPCTransport) StreamCallTool(ctx context.Context, name string, args map[string]any) (<-chan *CallToolResult, <-chan error) {
	resultChan := make(chan *CallToolResult, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(resultChan)
		defer close(errChan)

		t.mu.Lock()
		if !t.connected {
			t.mu.Unlock()
			if err := t.Connect(ctx); err != nil {
				errChan <- err
				return
			}
			t.mu.Lock()
		}
		client := t.client
		t.mu.Unlock()

		req := &pb.CallToolRequest{
			Name:      name,
			Arguments: make(map[string]*pb.Value),
		}
		for k, v := range args {
			req.Arguments[k] = toProtoValue(v)
		}

		stream, err := client.StreamCallTool(ctx, req)
		if err != nil {
			errChan <- fmt.Errorf("stream call tool RPC failed: %w", err)
			return
		}

		for {
			resp, err := stream.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					return
				}
				errChan <- err
				return
			}

			content := make([]Content, 0, len(resp.Content))
			for _, c := range resp.Content {
				content = append(content, Content{
					Type: c.Type,
					Text: c.Text,
				})
			}

			resultChan <- &CallToolResult{
				Content: content,
				IsError: resp.IsError,
			}
		}
	}()

	return resultChan, errChan
}

// Receive reads a message from the transport
func (t *GRPCTransport) Receive() ([]byte, error) {
	select {
	case msg := <-t.recvChan:
		return msg, nil
	case <-t.ctx.Done():
		return nil, t.ctx.Err()
	}
}

// Close closes the gRPC connection
func (t *GRPCTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cancelFunc != nil {
		t.cancelFunc()
	}

	if t.conn != nil {
		err := t.conn.Close()
		t.conn = nil
		t.client = nil
		t.connected = false
		return err
	}

	return nil
}

// IsConnected returns whether the transport is connected
func (t *GRPCTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connected
}

// GRPCServer wraps an MCP server for gRPC serving
type GRPCServer struct {
	pb.UnimplementedMCPServiceServer
	server     *Server
	grpcServer *grpc.Server
	listener   net.Listener
	tlsConfig  *TLSConfig
	mu         sync.Mutex
}

// NewGRPCServer creates a new gRPC server wrapper
func NewGRPCServer(mcpServer *Server, tlsConfig *TLSConfig) (*GRPCServer, error) {
	return &GRPCServer{
		server:    mcpServer,
		tlsConfig: tlsConfig,
	}, nil
}

// Initialize implements the Initialize RPC
func (s *GRPCServer) Initialize(ctx context.Context, req *pb.InitializeRequest) (*pb.InitializeResponse, error) {
	return &pb.InitializeResponse{
		ProtocolVersion: "2024-11-05",
		ServerInfo: &pb.ServerInfo{
			Name:    s.server.Name(),
			Version: "1.0.0",
		},
		Capabilities: &pb.Capabilities{
			SupportsStreaming:     true,
			SupportsCancellation:  true,
			SupportsProgress:      true,
			SupportedContentTypes: []string{"text/plain", "application/json"},
		},
	}, nil
}

// ListTools implements the ListTools RPC
func (s *GRPCServer) ListTools(ctx context.Context, req *pb.ListToolsRequest) (*pb.ListToolsResponse, error) {
	tools := s.server.ListTools()

	pbTools := make([]*pb.Tool, 0, len(tools))
	for _, tool := range tools {
		pbTool := &pb.Tool{
			Name:        tool.Name,
			Description: tool.Description,
		}

		// Convert schema
		if len(tool.Schema) > 0 {
			pbTool.InputSchema = &pb.Schema{
				Type:       "object",
				Properties: make(map[string]*pb.SchemaField),
				Required:   []string{},
			}
			for fieldName, field := range tool.Schema {
				pbTool.InputSchema.Properties[fieldName] = &pb.SchemaField{
					Type:        field.Type,
					Description: field.Description,
				}
				if field.Required {
					pbTool.InputSchema.Required = append(pbTool.InputSchema.Required, fieldName)
				}
			}
		}

		pbTools = append(pbTools, pbTool)
	}

	return &pb.ListToolsResponse{
		Tools: pbTools,
	}, nil
}

// CallTool implements the CallTool RPC
func (s *GRPCServer) CallTool(ctx context.Context, req *pb.CallToolRequest) (*pb.CallToolResponse, error) {
	// Convert proto arguments to map[string]any
	args := make(map[string]any)
	for k, v := range req.Arguments {
		args[k] = fromProtoValue(v)
	}

	result, err := s.server.CallTool(ctx, CallToolParams{
		Name:      req.Name,
		Arguments: args,
	})
	if err != nil {
		return &pb.CallToolResponse{
			Content: []*pb.Content{{
				Type: "text",
				Text: err.Error(),
			}},
			IsError: true,
		}, nil
	}

	pbContent := make([]*pb.Content, 0, len(result.Content))
	for _, c := range result.Content {
		pbContent = append(pbContent, &pb.Content{
			Type: c.Type,
			Text: c.Text,
		})
	}

	return &pb.CallToolResponse{
		Content: pbContent,
		IsError: result.IsError,
	}, nil
}

// StreamCallTool implements the StreamCallTool RPC
func (s *GRPCServer) StreamCallTool(req *pb.CallToolRequest, stream grpc.ServerStreamingServer[pb.CallToolResponse]) error {
	// Convert proto arguments to map[string]any
	args := make(map[string]any)
	for k, v := range req.Arguments {
		args[k] = fromProtoValue(v)
	}

	ctx := stream.Context()

	result, err := s.server.CallTool(ctx, CallToolParams{
		Name:      req.Name,
		Arguments: args,
	})
	if err != nil {
		return stream.Send(&pb.CallToolResponse{
			Content: []*pb.Content{{
				Type: "text",
				Text: err.Error(),
			}},
			IsError: true,
		})
	}

	// Send result as a single streamed response
	// In a real streaming scenario, the tool handler would yield multiple results
	pbContent := make([]*pb.Content, 0, len(result.Content))
	for _, c := range result.Content {
		pbContent = append(pbContent, &pb.Content{
			Type: c.Type,
			Text: c.Text,
		})
	}

	return stream.Send(&pb.CallToolResponse{
		Content: pbContent,
		IsError: result.IsError,
	})
}

// Ping implements the Ping RPC
func (s *GRPCServer) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{
		Timestamp: time.Now().UnixNano(),
		Status:    "ok",
	}, nil
}

// Serve starts the gRPC server on the given address
func (s *GRPCServer) Serve(address string) error {
	s.mu.Lock()

	listener, err := net.Listen("tcp", address)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to listen: %w", err)
	}
	s.listener = listener

	opts := []grpc.ServerOption{}

	if s.tlsConfig != nil && s.tlsConfig.Enabled {
		tlsConfig, err := s.buildServerTLSConfig()
		if err != nil {
			s.mu.Unlock()
			return fmt.Errorf("failed to build TLS config: %w", err)
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsConfig)))
	}

	s.grpcServer = grpc.NewServer(opts...)
	pb.RegisterMCPServiceServer(s.grpcServer, s)
	s.mu.Unlock()

	return s.grpcServer.Serve(listener)
}

// buildServerTLSConfig creates server-side TLS configuration
func (s *GRPCServer) buildServerTLSConfig() (*tls.Config, error) {
	if s.tlsConfig == nil {
		return nil, errors.New("TLS config is nil")
	}

	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ClientAuth: s.tlsConfig.ClientAuth,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		},
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
			tls.CurveP384,
		},
	}

	// Load server certificate
	if s.tlsConfig.CertFile != "" && s.tlsConfig.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(s.tlsConfig.CertFile, s.tlsConfig.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load server certificate: %w", err)
		}
		config.Certificates = []tls.Certificate{cert}
	}

	// Load CA for client verification (mTLS)
	if s.tlsConfig.CAFile != "" {
		caCert, err := os.ReadFile(s.tlsConfig.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, errors.New("failed to append CA certificate")
		}
		config.ClientCAs = caCertPool
	}

	return config, nil
}

// Stop gracefully stops the gRPC server
func (s *GRPCServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
}

// ForceStop immediately stops the gRPC server
func (s *GRPCServer) ForceStop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.grpcServer != nil {
		s.grpcServer.Stop()
	}
}

// Address returns the server's listening address
func (s *GRPCServer) Address() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return ""
}

// CreateSecureTLSConfig creates a secure TLS configuration with best practices
func CreateSecureTLSConfig(certFile, keyFile, caFile string) (*TLSConfig, error) {
	return &TLSConfig{
		Enabled:            true,
		CertFile:           certFile,
		KeyFile:            keyFile,
		CAFile:             caFile,
		InsecureSkipVerify: false,
		ClientAuth:         tls.RequireAndVerifyClientCert,
	}, nil
}

// CreateInsecureTLSConfig creates a TLS config for development (skip verification).
//
// SECURITY WARNING: This function creates a TLS configuration that disables
// certificate verification. This makes the connection vulnerable to
// man-in-the-middle attacks. NEVER use this in production environments.
// This is intended ONLY for local development and testing purposes.
//
// This function will return an error if called in a production environment.
func CreateInsecureTLSConfig() (*TLSConfig, error) {
	// SECURITY: Prevent creating insecure TLS config in production
	env := strings.ToLower(os.Getenv("ENVIRONMENT"))
	if env == "production" || env == "prod" {
		return nil, fmt.Errorf("SECURITY: Cannot create insecure TLS config in production environment (ENVIRONMENT=%s)", env)
	}

	log.Printf("WARNING: Creating insecure TLS config with certificate verification disabled. " +
		"This should NEVER be used in production. Current ENVIRONMENT=%s", env)
	return &TLSConfig{
		Enabled:            true,
		InsecureSkipVerify: true,
	}, nil
}

// Ensure GRPCTransport implements Transport interface
var _ Transport = (*GRPCTransport)(nil)
