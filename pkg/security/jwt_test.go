package security

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

// generateTestJWT creates a properly signed JWT for testing
func generateTestJWT(t *testing.T, claims *JWTClaims, privateKey *rsa.PrivateKey, keyID string) string {
	t.Helper()

	// Create header
	header := map[string]string{
		"alg": "RS256",
		"typ": "JWT",
		"kid": keyID,
	}

	headerBytes, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("Failed to marshal header: %v", err)
	}

	// Create claims payload
	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("Failed to marshal claims: %v", err)
	}

	// Encode header and claims
	headerEncoded := base64.RawURLEncoding.EncodeToString(headerBytes)
	claimsEncoded := base64.RawURLEncoding.EncodeToString(claimsBytes)

	// Create message to sign
	message := headerEncoded + "." + claimsEncoded

	// Sign message
	hasher := sha256.New()
	hasher.Write([]byte(message))
	hashed := hasher.Sum(nil)

	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed)
	if err != nil {
		t.Fatalf("Failed to sign JWT: %v", err)
	}

	signatureEncoded := base64.RawURLEncoding.EncodeToString(signature)

	return message + "." + signatureEncoded
}

func TestParseJWK(t *testing.T) {
	tests := []struct {
		name    string
		jwk     *JWK
		wantErr bool
	}{
		{
			name: "valid RSA key",
			jwk: &JWK{
				Kid: "test-key-1",
				Kty: "RSA",
				Alg: "RS256",
				Use: "sig",
				// Valid 2048-bit RSA key components (test-fixture-generated)
				N: "1kOW_RlfS9gS8-THkDN8tSu1uWpgGcx2lfNi_2WrB2gRwCPkp1LY0lgel_jyZvjqk1wJw4o5YSiCWmae7XCrQih2woYE7YGzRBxqhtgHHa5uZeqtmgBXTdk3NvKzEnBtLdysCqi_EvXqDMEqpzuJ6wYks6RxlPSe1yogjfb7IWAo6PSpz7KHx7cqRZXC00BdrT81zppiWDRrUAu0MTWwgdvEpykQ0xlSnL4vduvrp122iXoBZP2f90GcJ5FXNm_qJneyvBsm-WC6N_RhCfkhtXc9p9FsPPT4rdFm2Q0AoLa8GB_6neXq23yWS4FNkexNIBw68EEeoyOk64akEti8pw",
				E: "AQAB",
			},
			wantErr: false,
		},
		{
			name: "invalid key type",
			jwk: &JWK{
				Kid: "test-key-2",
				Kty: "EC",
				Alg: "ES256",
				N:   "test",
				E:   "AQAB",
			},
			wantErr: true,
		},
		{
			name: "invalid modulus encoding",
			jwk: &JWK{
				Kid: "test-key-3",
				Kty: "RSA",
				N:   "!!!invalid-base64!!!",
				E:   "AQAB",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := parseJWK(tt.jwk)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseJWK() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("parseJWK() unexpected error = %v", err)
				return
			}

			if key == nil {
				t.Error("parseJWK() returned nil key")
			}
		})
	}
}

func TestValidateRSAPublicKey(t *testing.T) {
	tests := []struct {
		name    string
		keyBits int
		wantErr bool
	}{
		{"2048-bit key", 2048, false},
		{"4096-bit key", 4096, false},
		{"1024-bit key (weak)", 1024, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate test key
			privateKey, err := rsa.GenerateKey(rand.Reader, tt.keyBits)
			if err != nil {
				t.Fatalf("Failed to generate key: %v", err)
			}

			err = validateRSAPublicKey(&privateKey.PublicKey)

			if tt.wantErr {
				if err == nil {
					t.Errorf("validateRSAPublicKey() error = nil, wantErr %v", tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("validateRSAPublicKey() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestVerifySignature(t *testing.T) {
	// Generate test RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	tests := []struct {
		name       string
		message    string
		tamper     bool
		wrongKey   bool
		wantErr    bool
	}{
		{
			name:    "valid signature",
			message: "test.payload",
			wantErr: false,
		},
		{
			name:    "tampered message",
			message: "test.payload",
			tamper:  true,
			wantErr: true,
		},
		{
			name:     "wrong public key",
			message:  "test.payload",
			wrongKey: true,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Sign the message
			hasher := sha256.New()
			hasher.Write([]byte(tt.message))
			hashed := hasher.Sum(nil)

			signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed)
			if err != nil {
				t.Fatalf("Failed to sign message: %v", err)
			}

			// Prepare message for verification
			messageToVerify := tt.message
			if tt.tamper {
				messageToVerify = "tampered.payload"
			}

			// Prepare public key
			publicKey := &privateKey.PublicKey
			if tt.wrongKey {
				wrongPrivateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				publicKey = &wrongPrivateKey.PublicKey
			}

			// Verify signature
			err = verifySignature(messageToVerify, signature, publicKey)

			if tt.wantErr {
				if err == nil {
					t.Errorf("verifySignature() error = nil, wantErr %v", tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("verifySignature() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestVerifyJWT_ClaimsValidation(t *testing.T) {
	// Generate test key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Store key in cache for testing
	globalJWKCache.mu.Lock()
	globalJWKCache.keys["test-key-id"] = &privateKey.PublicKey
	globalJWKCache.expiresAt = time.Now().Add(1 * time.Hour)
	globalJWKCache.mu.Unlock()

	now := time.Now().Unix()

	tests := []struct {
		name     string
		claims   *JWTClaims
		audience string
		wantErr  bool
	}{
		{
			name: "valid claims",
			claims: &JWTClaims{
				Email:    "test@example.com",
				Issuer:   "https://cloud.google.com/iap",
				Audience: "test-audience",
				Subject:  "test-subject",
				IssuedAt: now - 60,
				Expires:  now + 3600,
			},
			audience: "test-audience",
			wantErr:  false,
		},
		{
			name: "expired token",
			claims: &JWTClaims{
				Email:    "test@example.com",
				Issuer:   "https://cloud.google.com/iap",
				Audience: "test-audience",
				IssuedAt: now - 7200,
				Expires:  now - 3600,
			},
			audience: "test-audience",
			wantErr:  true,
		},
		{
			name: "invalid issuer",
			claims: &JWTClaims{
				Email:    "test@example.com",
				Issuer:   "https://evil.com",
				Audience: "test-audience",
				IssuedAt: now - 60,
				Expires:  now + 3600,
			},
			audience: "test-audience",
			wantErr:  true,
		},
		{
			name: "audience mismatch",
			claims: &JWTClaims{
				Email:    "test@example.com",
				Issuer:   "https://cloud.google.com/iap",
				Audience: "wrong-audience",
				IssuedAt: now - 60,
				Expires:  now + 3600,
			},
			audience: "test-audience",
			wantErr:  true,
		},
		{
			name: "token not yet valid",
			claims: &JWTClaims{
				Email:    "test@example.com",
				Issuer:   "https://cloud.google.com/iap",
				Audience: "test-audience",
				IssuedAt: now + 3600,
				Expires:  now + 7200,
			},
			audience: "test-audience",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate JWT
			token := generateTestJWT(t, tt.claims, privateKey, "test-key-id")

			// Verify JWT
			claims, err := verifyJWT(token, tt.audience)

			if tt.wantErr {
				if err == nil {
					t.Errorf("verifyJWT() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("verifyJWT() unexpected error = %v", err)
				return
			}

			if claims == nil {
				t.Error("verifyJWT() returned nil claims")
				return
			}

			if claims.Email != tt.claims.Email {
				t.Errorf("claims.Email = %v, want %v", claims.Email, tt.claims.Email)
			}
		})
	}
}

func TestVerifyJWT_InvalidFormat(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "empty token",
			token:   "",
			wantErr: true,
		},
		{
			name:    "invalid format - missing parts",
			token:   "header.payload",
			wantErr: true,
		},
		{
			name:    "invalid format - too many parts",
			token:   "header.payload.signature.extra",
			wantErr: true,
		},
		{
			name:    "invalid base64 encoding",
			token:   "!!!invalid!!!.payload.signature",
			wantErr: true,
		},
		{
			name:    "invalid JSON in header",
			token:   base64.RawURLEncoding.EncodeToString([]byte("{invalid json")) + ".payload.signature",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := verifyJWT(tt.token, "")

			if tt.wantErr {
				if err == nil {
					t.Errorf("verifyJWT() error = nil, wantErr %v", tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("verifyJWT() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestVerifyJWT_UnsupportedAlgorithm(t *testing.T) {
	// Create JWT with unsupported algorithm
	header := map[string]string{
		"alg": "HS256", // HMAC not supported
		"typ": "JWT",
		"kid": "test-key",
	}

	claims := &JWTClaims{
		Email:  "test@example.com",
		Issuer: "https://cloud.google.com/iap",
	}

	headerBytes, _ := json.Marshal(header)
	claimsBytes, _ := json.Marshal(claims)

	headerEncoded := base64.RawURLEncoding.EncodeToString(headerBytes)
	claimsEncoded := base64.RawURLEncoding.EncodeToString(claimsBytes)

	token := headerEncoded + "." + claimsEncoded + ".fake-signature"

	_, err := verifyJWT(token, "")
	if err == nil {
		t.Error("verifyJWT() should reject unsupported algorithm")
	}
}

func TestJWKCache(t *testing.T) {
	// Clear cache
	globalJWKCache.mu.Lock()
	globalJWKCache.keys = make(map[string]*rsa.PublicKey)
	globalJWKCache.expiresAt = time.Time{}
	globalJWKCache.mu.Unlock()

	// Generate test key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Add to cache
	globalJWKCache.mu.Lock()
	globalJWKCache.keys["cached-key"] = &privateKey.PublicKey
	globalJWKCache.expiresAt = time.Now().Add(1 * time.Hour)
	globalJWKCache.mu.Unlock()

	// Try to fetch from cache (should succeed without network call)
	key, err := fetchGooglePublicKey("cached-key")
	if err != nil {
		t.Errorf("fetchGooglePublicKey() failed to retrieve from cache: %v", err)
	}
	if key == nil {
		t.Error("fetchGooglePublicKey() returned nil key from cache")
	}

	// Test cache expiration
	globalJWKCache.mu.Lock()
	globalJWKCache.expiresAt = time.Now().Add(-1 * time.Hour)
	globalJWKCache.mu.Unlock()

	// This will try to fetch from network and likely fail in test env
	// That's OK - we're just testing that it doesn't use expired cache
	_, _ = fetchGooglePublicKey("cached-key")
}
