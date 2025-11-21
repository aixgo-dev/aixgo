package mcp

import (
	"context"
	"crypto/tls"
	"testing"
	"time"
)

func TestNewGRPCTransportWithConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    GRPCTransportConfig
		wantErr   bool
		errString string
	}{
		{
			name: "valid config",
			config: GRPCTransportConfig{
				Address: "localhost:50051",
			},
			wantErr: false,
		},
		{
			name:      "empty address",
			config:    GRPCTransportConfig{},
			wantErr:   true,
			errString: "gRPC address is required",
		},
		{
			name: "with TLS config",
			config: GRPCTransportConfig{
				Address: "localhost:50051",
				TLS: &TLSConfig{
					Enabled: true,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport, err := NewGRPCTransportWithConfig(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errString != "" && err.Error() != tt.errString {
					t.Errorf("expected error %q, got %q", tt.errString, err.Error())
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if transport == nil {
				t.Error("expected transport, got nil")
				return
			}
			defer func() {
				_ = transport.Close()
			}()

			if transport.config.Address != tt.config.Address {
				t.Errorf("address = %q, want %q", transport.config.Address, tt.config.Address)
			}
		})
	}
}

func TestNewGRPCTransport(t *testing.T) {
	tests := []struct {
		name    string
		config  ServerConfig
		wantErr bool
	}{
		{
			name: "valid config without TLS",
			config: ServerConfig{
				Address: "localhost:50051",
				TLS:     false,
			},
			wantErr: false,
		},
		{
			name: "valid config with TLS",
			config: ServerConfig{
				Address: "localhost:50051",
				TLS:     true,
			},
			wantErr: false,
		},
		{
			name: "empty address",
			config: ServerConfig{
				Address: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport, err := NewGRPCTransport(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if transport != nil {
				_ = transport.Close()
			}
		})
	}
}

func TestGRPCTransport_Close(t *testing.T) {
	transport, err := NewGRPCTransportWithConfig(GRPCTransportConfig{
		Address: "localhost:50051",
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	// Close should not error on unconnected transport
	if err := transport.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Double close should be safe
	if err := transport.Close(); err != nil {
		t.Errorf("double Close() error = %v", err)
	}
}

func TestGRPCTransport_IsConnected(t *testing.T) {
	transport, err := NewGRPCTransportWithConfig(GRPCTransportConfig{
		Address: "localhost:50051",
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer func() {
		_ = transport.Close()
	}()

	if transport.IsConnected() {
		t.Error("expected not connected initially")
	}
}

func TestGRPCTransport_Send_NotConnected(t *testing.T) {
	transport, err := NewGRPCTransportWithConfig(GRPCTransportConfig{
		Address: "localhost:99999", // Invalid port
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer func() { _ = transport.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Send should attempt to connect and fail
	_, err = transport.Send(ctx, "tools/list", nil)
	if err == nil {
		t.Error("expected error when sending on unconnected transport")
	}
}

func TestGRPCTransport_Receive_Cancelled(t *testing.T) {
	transport, err := NewGRPCTransportWithConfig(GRPCTransportConfig{
		Address: "localhost:50051",
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}

	// Close the transport to cancel context
	_ = transport.Close()

	// Receive should return context cancelled
	_, err = transport.Receive()
	if err == nil {
		t.Error("expected error on cancelled context")
	}
}

func TestGRPCServer_Creation(t *testing.T) {
	mcpServer := NewServer("test-server")

	server, err := NewGRPCServer(mcpServer, nil)
	if err != nil {
		t.Fatalf("failed to create gRPC server: %v", err)
	}

	if server.server != mcpServer {
		t.Error("server reference mismatch")
	}

	if server.Address() != "" {
		t.Error("expected empty address before serving")
	}
}

func TestGRPCServer_WithTLS(t *testing.T) {
	mcpServer := NewServer("test-server")
	tlsConfig := &TLSConfig{
		Enabled:            true,
		InsecureSkipVerify: true,
	}

	server, err := NewGRPCServer(mcpServer, tlsConfig)
	if err != nil {
		t.Fatalf("failed to create gRPC server: %v", err)
	}

	if server.tlsConfig != tlsConfig {
		t.Error("TLS config not set correctly")
	}
}

func TestTLSConfig_BuildClientConfig(t *testing.T) {
	transport, err := NewGRPCTransportWithConfig(GRPCTransportConfig{
		Address: "localhost:50051",
		TLS: &TLSConfig{
			Enabled:            true,
			InsecureSkipVerify: true,
			ServerName:         "test.example.com",
		},
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer func() { _ = transport.Close() }()

	tlsConfig, err := transport.buildTLSConfig()
	if err != nil {
		t.Fatalf("buildTLSConfig() error = %v", err)
	}

	if tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %d, want %d", tlsConfig.MinVersion, tls.VersionTLS12)
	}

	if !tlsConfig.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true")
	}

	if tlsConfig.ServerName != "test.example.com" {
		t.Errorf("ServerName = %q, want %q", tlsConfig.ServerName, "test.example.com")
	}

	if len(tlsConfig.CipherSuites) == 0 {
		t.Error("CipherSuites should be configured")
	}

	if len(tlsConfig.CurvePreferences) == 0 {
		t.Error("CurvePreferences should be configured")
	}
}

func TestTLSConfig_BuildClientConfig_NilConfig(t *testing.T) {
	transport, err := NewGRPCTransportWithConfig(GRPCTransportConfig{
		Address: "localhost:50051",
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer func() { _ = transport.Close() }()

	_, err = transport.buildTLSConfig()
	if err == nil {
		t.Error("expected error for nil TLS config")
	}
}

func TestTLSConfig_BuildClientConfig_InvalidCert(t *testing.T) {
	transport, err := NewGRPCTransportWithConfig(GRPCTransportConfig{
		Address: "localhost:50051",
		TLS: &TLSConfig{
			Enabled:  true,
			CertFile: "/nonexistent/cert.pem",
			KeyFile:  "/nonexistent/key.pem",
		},
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer func() { _ = transport.Close() }()

	_, err = transport.buildTLSConfig()
	if err == nil {
		t.Error("expected error for invalid certificate files")
	}
}

func TestTLSConfig_BuildClientConfig_InvalidCA(t *testing.T) {
	transport, err := NewGRPCTransportWithConfig(GRPCTransportConfig{
		Address: "localhost:50051",
		TLS: &TLSConfig{
			Enabled: true,
			CAFile:  "/nonexistent/ca.pem",
		},
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer func() { _ = transport.Close() }()

	_, err = transport.buildTLSConfig()
	if err == nil {
		t.Error("expected error for invalid CA file")
	}
}

func TestGRPCServer_BuildServerTLSConfig(t *testing.T) {
	mcpServer := NewServer("test-server")
	server, err := NewGRPCServer(mcpServer, &TLSConfig{
		Enabled:    true,
		ClientAuth: tls.RequireAndVerifyClientCert,
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	tlsConfig, err := server.buildServerTLSConfig()
	if err != nil {
		t.Fatalf("buildServerTLSConfig() error = %v", err)
	}

	if tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %d, want %d", tlsConfig.MinVersion, tls.VersionTLS12)
	}

	if tlsConfig.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Errorf("ClientAuth = %v, want %v", tlsConfig.ClientAuth, tls.RequireAndVerifyClientCert)
	}
}

func TestGRPCServer_BuildServerTLSConfig_NilConfig(t *testing.T) {
	mcpServer := NewServer("test-server")
	server, err := NewGRPCServer(mcpServer, nil)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	_, err = server.buildServerTLSConfig()
	if err == nil {
		t.Error("expected error for nil TLS config")
	}
}

func TestCreateSecureTLSConfig_GRPC(t *testing.T) {
	config, err := CreateSecureTLSConfig("cert.pem", "key.pem", "ca.pem")
	if err != nil {
		t.Fatalf("CreateSecureTLSConfig() error = %v", err)
	}

	if !config.Enabled {
		t.Error("Enabled should be true")
	}

	if config.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be false for secure config")
	}

	if config.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Errorf("ClientAuth = %v, want %v", config.ClientAuth, tls.RequireAndVerifyClientCert)
	}

	if config.CertFile != "cert.pem" {
		t.Errorf("CertFile = %q, want %q", config.CertFile, "cert.pem")
	}
}

func TestCreateInsecureTLSConfig(t *testing.T) {
	config := CreateInsecureTLSConfig()

	if !config.Enabled {
		t.Error("Enabled should be true")
	}

	if !config.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true for insecure config")
	}
}

func TestGRPCTransport_ImplementsInterface(t *testing.T) {
	var _ Transport = (*GRPCTransport)(nil)
}

func TestToProtoValue(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{"nil", nil},
		{"string", "hello"},
		{"float64", 3.14},
		{"int", 42},
		{"bool", true},
		{"array", []any{"a", "b", "c"}},
		{"map", map[string]any{"key": "value"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			protoVal := toProtoValue(tt.input)
			if protoVal == nil {
				t.Error("expected non-nil proto value")
			}
			// Convert back
			_ = fromProtoValue(protoVal)
		})
	}
}

func TestFromProtoValue_Nil(t *testing.T) {
	result := fromProtoValue(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}
}

func TestGRPCServer_StopWithoutServing(t *testing.T) {
	mcpServer := NewServer("test-server")
	server, err := NewGRPCServer(mcpServer, nil)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Stop should be safe even without serving
	server.Stop()
	server.ForceStop()
}

// startTestServer starts the gRPC server on a random port and returns when ready
func startTestServer(server *GRPCServer) (string, error) {
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Serve("localhost:0")
	}()

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	select {
	case err := <-errChan:
		return "", err
	default:
		return server.Address(), nil
	}
}

func TestGRPCServerClientCommunication(t *testing.T) {
	// Create MCP server with a test tool
	mcpServer := NewServer("test-server")
	err := mcpServer.RegisterTool(Tool{
		Name:        "echo",
		Description: "Echo the input",
		Handler: func(ctx context.Context, args Args) (any, error) {
			return args.String("message"), nil
		},
		Schema: Schema{
			"message": SchemaField{
				Type:        "string",
				Description: "Message to echo",
				Required:    true,
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}

	// Create gRPC server
	grpcServer, err := NewGRPCServer(mcpServer, nil)
	if err != nil {
		t.Fatalf("failed to create gRPC server: %v", err)
	}

	// Start server
	serverReady := make(chan struct{})
	go func() {
		_, err := startTestServer(grpcServer)
		if err != nil {
			return
		}
		close(serverReady)
	}()

	select {
	case <-serverReady:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not start in time")
	}
	time.Sleep(100 * time.Millisecond)

	serverAddr := grpcServer.Address()
	if serverAddr == "" {
		t.Fatal("server address is empty")
	}

	// Create client transport
	transport, err := NewGRPCTransportWithConfig(GRPCTransportConfig{
		Address: serverAddr,
	})
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer func() { _ = transport.Close() }()

	ctx := context.Background()

	// Test Initialize
	t.Run("Initialize", func(t *testing.T) {
		result, err := transport.Send(ctx, "initialize", nil)
		if err != nil {
			t.Fatalf("initialize failed: %v", err)
		}

		resp, ok := result.(map[string]any)
		if !ok {
			t.Fatal("unexpected response type")
		}

		if resp["protocolVersion"] != "2024-11-05" {
			t.Errorf("unexpected protocol version: %v", resp["protocolVersion"])
		}
	})

	// Test ListTools
	t.Run("ListTools", func(t *testing.T) {
		result, err := transport.Send(ctx, "tools/list", nil)
		if err != nil {
			t.Fatalf("list tools failed: %v", err)
		}

		resp, ok := result.(map[string]any)
		if !ok {
			t.Fatal("unexpected response type")
		}

		tools, ok := resp["tools"].([]map[string]any)
		if !ok {
			t.Fatal("missing tools")
		}

		if len(tools) != 1 {
			t.Fatalf("expected 1 tool, got %d", len(tools))
		}

		if tools[0]["name"] != "echo" {
			t.Errorf("unexpected tool name: %v", tools[0]["name"])
		}
	})

	// Test CallTool
	t.Run("CallTool", func(t *testing.T) {
		result, err := transport.Send(ctx, "tools/call", map[string]any{
			"name": "echo",
			"arguments": map[string]any{
				"message": "hello world",
			},
		})
		if err != nil {
			t.Fatalf("call tool failed: %v", err)
		}

		resp, ok := result.(map[string]any)
		if !ok {
			t.Fatal("unexpected response type")
		}

		if resp["isError"] == true {
			t.Error("unexpected error in response")
		}

		content, ok := resp["content"].([]map[string]any)
		if !ok || len(content) == 0 {
			t.Fatal("missing content")
		}

		if content[0]["text"] != "hello world" {
			t.Errorf("unexpected response text: %v", content[0]["text"])
		}
	})

	// Test Ping
	t.Run("Ping", func(t *testing.T) {
		result, err := transport.Send(ctx, "ping", nil)
		if err != nil {
			t.Fatalf("ping failed: %v", err)
		}

		resp, ok := result.(map[string]any)
		if !ok {
			t.Fatal("unexpected response type")
		}

		if resp["status"] != "ok" {
			t.Errorf("unexpected status: %v", resp["status"])
		}
	})

	// Test unsupported method
	t.Run("UnsupportedMethod", func(t *testing.T) {
		_, err := transport.Send(ctx, "unsupported/method", nil)
		if err == nil {
			t.Error("expected error for unsupported method")
		}
	})

	// Test invalid params
	t.Run("InvalidParams", func(t *testing.T) {
		_, err := transport.Send(ctx, "tools/call", "invalid")
		if err == nil {
			t.Error("expected error for invalid params")
		}
	})

	// Test tool not found
	t.Run("ToolNotFound", func(t *testing.T) {
		result, err := transport.Send(ctx, "tools/call", map[string]any{
			"name":      "nonexistent",
			"arguments": map[string]any{},
		})
		if err != nil {
			t.Fatalf("call tool should not return transport error: %v", err)
		}

		resp, ok := result.(map[string]any)
		if !ok {
			t.Fatal("unexpected response type")
		}

		if resp["isError"] != true {
			t.Error("expected isError to be true for nonexistent tool")
		}
	})

	grpcServer.Stop()
}

func TestGRPCTransport_StreamCallTool(t *testing.T) {
	mcpServer := NewServer("test-server")
	_ = mcpServer.RegisterTool(Tool{
		Name:        "stream-echo",
		Description: "Stream echo",
		Handler: func(ctx context.Context, args Args) (any, error) {
			return args.String("message"), nil
		},
		Schema: Schema{
			"message": SchemaField{Type: "string", Required: true},
		},
	})

	grpcServer, _ := NewGRPCServer(mcpServer, nil)

	serverReady := make(chan struct{})
	go func() {
		_, _ = startTestServer(grpcServer)
		close(serverReady)
	}()
	<-serverReady
	time.Sleep(100 * time.Millisecond)

	transport, _ := NewGRPCTransportWithConfig(GRPCTransportConfig{
		Address: grpcServer.Address(),
	})
	defer func() { _ = transport.Close() }()

	ctx := context.Background()
	resultChan, errChan := transport.StreamCallTool(ctx, "stream-echo", map[string]any{
		"message": "streaming hello",
	})

	var results []*CallToolResult
	done := make(chan struct{})
	go func() {
		for result := range resultChan {
			results = append(results, result)
		}
		close(done)
	}()

	// Drain error channel
	go func() {
		for range errChan {
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("streaming timeout")
	}

	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}

	if len(results[0].Content) == 0 || results[0].Content[0].Text != "streaming hello" {
		t.Error("unexpected streaming result")
	}

	grpcServer.Stop()
}
