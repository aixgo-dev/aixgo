package mcp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sync"

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
	mu         sync.Mutex
	connected  bool
	recvChan   chan []byte
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// MCPMessage is the wire format for MCP over gRPC
type MCPMessage struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *MCPError       `json:"error,omitempty"`
	ID     string          `json:"id,omitempty"`
}

// MCPError represents an error in MCP
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
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
	t.connected = true

	return nil
}

// buildTLSConfig creates a TLS configuration
func (t *GRPCTransport) buildTLSConfig() (*tls.Config, error) {
	tlsCfg := t.config.TLS
	if tlsCfg == nil {
		return nil, errors.New("TLS config is nil")
	}

	// SECURITY WARNING: InsecureSkipVerify disables certificate verification.
	// This should ONLY be used in development/testing environments.
	// Using this in production exposes the connection to man-in-the-middle attacks.
	if tlsCfg.InsecureSkipVerify {
		log.Printf("WARNING: TLS certificate verification is disabled (InsecureSkipVerify=true). " +
			"This is a security risk and should NEVER be used in production. " +
			"Connections are vulnerable to man-in-the-middle attacks.")
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
	t.mu.Unlock()

	// Serialize params
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	msg := &MCPMessage{
		Method: method,
		Params: paramsBytes,
	}

	// For now, we use unary-style communication over the connection
	// In a full implementation, this would use the gRPC streaming
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	// This is a simplified implementation
	// A full implementation would use proper gRPC service methods
	_ = msgBytes

	return nil, errors.New("gRPC transport requires generated proto files - use local transport for now")
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
	s.mu.Unlock()

	// Note: To fully implement, we would register the MCPService here
	// using the generated proto code. For now, we provide the structure.

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
func CreateInsecureTLSConfig() *TLSConfig {
	log.Printf("WARNING: Creating insecure TLS config with certificate verification disabled. " +
		"This should NEVER be used in production.")
	return &TLSConfig{
		Enabled:            true,
		InsecureSkipVerify: true,
	}
}

// Ensure GRPCTransport implements Transport interface
var _ Transport = (*GRPCTransport)(nil)
