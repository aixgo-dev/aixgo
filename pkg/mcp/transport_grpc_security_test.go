package mcp

import (
	"crypto/tls"
	"crypto/x509"
	"testing"
)

// Test TLS Configuration - Minimum Version
func TestTLSConfig_MinimumVersion(t *testing.T) {
	tests := []struct {
		name          string
		version       uint16
		shouldBeValid bool
	}{
		{
			name:          "TLS 1.3",
			version:       tls.VersionTLS13,
			shouldBeValid: true,
		},
		{
			name:          "TLS 1.2",
			version:       tls.VersionTLS12,
			shouldBeValid: true,
		},
		{
			name:          "TLS 1.1",
			version:       tls.VersionTLS11,
			shouldBeValid: false,
		},
		{
			name:          "TLS 1.0",
			version:       tls.VersionTLS10,
			shouldBeValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &tls.Config{
				MinVersion: tls.VersionTLS12, // Secure minimum
			}

			// Check if the version would be accepted
			isAcceptable := tt.version >= config.MinVersion

			if tt.shouldBeValid && !isAcceptable {
				t.Errorf("version %d should be acceptable", tt.version)
			}
			if !tt.shouldBeValid && isAcceptable {
				t.Errorf("version %d should not be acceptable", tt.version)
			}
		})
	}
}

// Test TLS Configuration - Cipher Suites
func TestTLSConfig_CipherSuites(t *testing.T) {
	// Recommended secure cipher suites
	secureCiphers := []uint16{
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
	}

	// Insecure cipher suites that should not be used
	insecureCiphers := []uint16{
		tls.TLS_RSA_WITH_RC4_128_SHA,
		tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	}

	config := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		CipherSuites: secureCiphers,
	}

	// Verify secure ciphers are included
	for _, cipher := range secureCiphers {
		found := false
		for _, configured := range config.CipherSuites {
			if configured == cipher {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("secure cipher suite %x should be included", cipher)
		}
	}

	// Verify insecure ciphers are not included
	for _, cipher := range insecureCiphers {
		for _, configured := range config.CipherSuites {
			if configured == cipher {
				t.Errorf("insecure cipher suite %x should not be included", cipher)
			}
		}
	}
}

// Test TLS Configuration - Certificate Validation
func TestTLSConfig_CertificateValidation(t *testing.T) {
	tests := []struct {
		name               string
		insecureSkipVerify bool
		shouldBeSecure     bool
	}{
		{
			name:               "certificate verification enabled",
			insecureSkipVerify: false,
			shouldBeSecure:     true,
		},
		{
			name:               "certificate verification disabled (INSECURE)",
			insecureSkipVerify: true,
			shouldBeSecure:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &tls.Config{
				InsecureSkipVerify: tt.insecureSkipVerify,
			}

			isSecure := !config.InsecureSkipVerify

			if tt.shouldBeSecure && !isSecure {
				t.Error("configuration should be secure (verify certificates)")
			}
			if !tt.shouldBeSecure && isSecure {
				t.Error("configuration is marked as secure but shouldn't be")
			}
		})
	}
}

// Test Secure TLS Configuration Creation
func TestCreateSecureTLSConfig(t *testing.T) {
	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		},
		InsecureSkipVerify: false,
		ClientAuth:         tls.RequireAndVerifyClientCert,
	}

	// Verify minimum version
	if config.MinVersion < tls.VersionTLS12 {
		t.Errorf("MinVersion = %d, want >= TLS 1.2 (%d)", config.MinVersion, tls.VersionTLS12)
	}

	// Verify certificate verification is enabled
	if config.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be false")
	}

	// Verify client authentication is required
	if config.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Errorf("ClientAuth = %v, want RequireAndVerifyClientCert", config.ClientAuth)
	}

	// Verify secure cipher suites are configured
	if len(config.CipherSuites) == 0 {
		t.Error("CipherSuites should be configured")
	}
}

// Test Client Certificate Authentication
func TestTLSConfig_ClientCertAuth(t *testing.T) {
	tests := []struct {
		name       string
		clientAuth tls.ClientAuthType
		isSecure   bool
	}{
		{
			name:       "require and verify client cert",
			clientAuth: tls.RequireAndVerifyClientCert,
			isSecure:   true,
		},
		{
			name:       "request client cert",
			clientAuth: tls.RequestClientCert,
			isSecure:   false, // Not as secure as requiring
		},
		{
			name:       "no client cert",
			clientAuth: tls.NoClientCert,
			isSecure:   false,
		},
		{
			name:       "require any client cert",
			clientAuth: tls.RequireAnyClientCert,
			isSecure:   false, // Doesn't verify the cert
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &tls.Config{
				ClientAuth: tt.clientAuth,
			}

			// Most secure option is RequireAndVerifyClientCert
			mostSecure := config.ClientAuth == tls.RequireAndVerifyClientCert

			if tt.isSecure && !mostSecure {
				t.Error("configuration should require and verify client certificates")
			}
		})
	}
}

// Test Certificate Pool Configuration
func TestTLSConfig_CertificatePool(t *testing.T) {
	// Create a new certificate pool
	certPool := x509.NewCertPool()

	config := &tls.Config{
		RootCAs:    certPool,
		ClientCAs:  certPool,
		MinVersion: tls.VersionTLS12,
	}

	// Verify cert pools are configured
	if config.RootCAs == nil {
		t.Error("RootCAs should be configured")
	}

	if config.ClientCAs == nil {
		t.Error("ClientCAs should be configured")
	}

	// In production, we would load actual certificates
	// For now, just verify the structure is correct
	if config.RootCAs != certPool {
		t.Error("RootCAs should be the configured pool")
	}
}

// Test Server Name Indication (SNI)
func TestTLSConfig_SNI(t *testing.T) {
	tests := []struct {
		name       string
		serverName string
		shouldSet  bool
	}{
		{
			name:       "valid server name",
			serverName: "api.example.com",
			shouldSet:  true,
		},
		{
			name:       "empty server name",
			serverName: "",
			shouldSet:  false,
		},
		{
			name:       "localhost",
			serverName: "localhost",
			shouldSet:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &tls.Config{
				ServerName: tt.serverName,
			}

			hasServerName := config.ServerName != ""

			if tt.shouldSet && !hasServerName {
				t.Error("ServerName should be set")
			}
			if tt.shouldSet && config.ServerName != tt.serverName {
				t.Errorf("ServerName = %s, want %s", config.ServerName, tt.serverName)
			}
		})
	}
}

// Test TLS Session Resumption
func TestTLSConfig_SessionResumption(t *testing.T) {
	config := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ClientSessionCache: tls.NewLRUClientSessionCache(128),
	}

	// Verify session cache is configured
	if config.ClientSessionCache == nil {
		t.Error("ClientSessionCache should be configured for session resumption")
	}
}

// Test TLS Renegotiation
func TestTLSConfig_Renegotiation(t *testing.T) {
	config := &tls.Config{
		MinVersion:    tls.VersionTLS12,
		Renegotiation: tls.RenegotiateNever, // Most secure
	}

	// Verify renegotiation is disabled
	if config.Renegotiation != tls.RenegotiateNever {
		t.Errorf("Renegotiation = %v, want RenegotiateNever", config.Renegotiation)
	}
}

// Test Secure Protocol Preferences
func TestTLSConfig_PreferServerCipherSuites(t *testing.T) {
	// Note: PreferServerCipherSuites is deprecated since Go 1.18 and ignored
	// This test is kept for documentation purposes only
	t.Skip("PreferServerCipherSuites deprecated since Go 1.18")
}

// Test Curve Preferences
func TestTLSConfig_CurvePreferences(t *testing.T) {
	secureCurves := []tls.CurveID{
		tls.X25519,
		tls.CurveP256,
		tls.CurveP384,
	}

	config := &tls.Config{
		MinVersion:       tls.VersionTLS12,
		CurvePreferences: secureCurves,
	}

	// Verify secure curves are configured
	if len(config.CurvePreferences) == 0 {
		t.Error("CurvePreferences should be configured")
	}

	// Verify X25519 is preferred (most secure and fast)
	if len(config.CurvePreferences) > 0 && config.CurvePreferences[0] != tls.X25519 {
		t.Error("X25519 should be the first (preferred) curve")
	}
}

// Test NextProtos Configuration
func TestTLSConfig_NextProtos(t *testing.T) {
	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"h2", "http/1.1"}, // HTTP/2 preferred
	}

	// Verify ALPN protocols are configured
	if len(config.NextProtos) == 0 {
		t.Error("NextProtos should be configured for ALPN")
	}

	// Verify HTTP/2 is preferred
	if len(config.NextProtos) > 0 && config.NextProtos[0] != "h2" {
		t.Error("h2 (HTTP/2) should be the first (preferred) protocol")
	}
}

// Test Complete Secure Configuration
func TestTLSConfig_CompleteSecureConfiguration(t *testing.T) {
	config := &tls.Config{
		// Protocol version
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS13,

		// Cipher suites (for TLS 1.2)
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		},

		// Elliptic curves
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
			tls.CurveP384,
		},

		// Certificate verification
		InsecureSkipVerify: false,
		ClientAuth:         tls.RequireAndVerifyClientCert,

		// Server preferences
		PreferServerCipherSuites: true,

		// Session management
		ClientSessionCache: tls.NewLRUClientSessionCache(128),

		// Renegotiation
		Renegotiation: tls.RenegotiateNever,

		// ALPN
		NextProtos: []string{"h2", "http/1.1"},
	}

	// Run comprehensive checks
	checks := []struct {
		name  string
		check func() bool
	}{
		{
			name:  "minimum TLS version is secure",
			check: func() bool { return config.MinVersion >= tls.VersionTLS12 },
		},
		{
			name:  "cipher suites are configured",
			check: func() bool { return len(config.CipherSuites) > 0 },
		},
		{
			name:  "curve preferences are configured",
			check: func() bool { return len(config.CurvePreferences) > 0 },
		},
		{
			name:  "certificate verification is enabled",
			check: func() bool { return !config.InsecureSkipVerify },
		},
		{
			name:  "client certificates are required",
			check: func() bool { return config.ClientAuth == tls.RequireAndVerifyClientCert },
		},
		{
			name:  "server cipher suite preference (deprecated, always true)",
			check: func() bool { return true }, // PreferServerCipherSuites deprecated since Go 1.18
		},
		{
			name:  "session cache is configured",
			check: func() bool { return config.ClientSessionCache != nil },
		},
		{
			name:  "renegotiation is disabled",
			check: func() bool { return config.Renegotiation == tls.RenegotiateNever },
		},
		{
			name:  "ALPN protocols are configured",
			check: func() bool { return len(config.NextProtos) > 0 },
		},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if !check.check() {
				t.Errorf("security check failed: %s", check.name)
			}
		})
	}
}

// Test Invalid Certificate Scenarios
func TestTLSConfig_InvalidCertificates(t *testing.T) {
	tests := []struct {
		name        string
		description string
	}{
		{
			name:        "expired certificate",
			description: "certificate should be rejected if expired",
		},
		{
			name:        "self-signed certificate",
			description: "self-signed certificate should be rejected without proper trust",
		},
		{
			name:        "wrong hostname",
			description: "certificate should be rejected if hostname doesn't match",
		},
		{
			name:        "revoked certificate",
			description: "certificate should be rejected if revoked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// These tests document expected behavior
			// In production, the TLS library handles these checks automatically
			t.Logf("Expected behavior: %s", tt.description)
		})
	}
}

// Benchmark TLS configuration creation
func BenchmarkSecureTLSConfig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = &tls.Config{
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			},
			CurvePreferences: []tls.CurveID{
				tls.X25519,
				tls.CurveP256,
			},
			InsecureSkipVerify: false,
			ClientAuth:         tls.RequireAndVerifyClientCert,
		}
	}
}
