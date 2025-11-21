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

func TestMCPMessage_Serialization(t *testing.T) {
	msg := MCPMessage{
		Method: "tools/call",
		ID:     "123",
	}

	if msg.Method != "tools/call" {
		t.Errorf("Method = %q, want %q", msg.Method, "tools/call")
	}

	if msg.ID != "123" {
		t.Errorf("ID = %q, want %q", msg.ID, "123")
	}
}

func TestMCPError(t *testing.T) {
	err := MCPError{
		Code:    -32600,
		Message: "Invalid Request",
	}

	if err.Code != -32600 {
		t.Errorf("Code = %d, want %d", err.Code, -32600)
	}

	if err.Message != "Invalid Request" {
		t.Errorf("Message = %q, want %q", err.Message, "Invalid Request")
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
