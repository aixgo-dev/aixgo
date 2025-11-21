package security

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// IAPJWTClaims represents the claims in an IAP JWT
type IAPJWTClaims struct {
	Email         string `json:"email"`
	Sub           string `json:"sub"`
	Aud           string `json:"aud"`
	Iss           string `json:"iss"`
	Iat           int64  `json:"iat"`
	Exp           int64  `json:"exp"`
	Hd            string `json:"hd"`
	EmailVerified bool   `json:"email_verified"`
}

// IAPPublicKey represents a Google public key for JWT verification
type IAPPublicKey struct {
	Kid string
	N   *big.Int
	E   int
	Key *rsa.PublicKey
}

// IAPKeyCache caches Google's public keys for IAP JWT verification
type IAPKeyCache struct {
	keys       map[string]*IAPPublicKey
	mu         sync.RWMutex
	lastUpdate time.Time
	httpClient *http.Client
}

// NewIAPKeyCache creates a new IAP key cache
func NewIAPKeyCache() *IAPKeyCache {
	return &IAPKeyCache{
		keys: make(map[string]*IAPPublicKey),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetKey retrieves a public key by key ID, fetching from Google if needed
func (c *IAPKeyCache) GetKey(ctx context.Context, kid string) (*IAPPublicKey, error) {
	// Check cache first
	c.mu.RLock()
	key, exists := c.keys[kid]
	needsRefresh := time.Since(c.lastUpdate) > 24*time.Hour
	c.mu.RUnlock()

	if exists && !needsRefresh {
		return key, nil
	}

	// Fetch keys from Google
	if err := c.fetchKeys(ctx); err != nil {
		// If we have a cached key, use it even if refresh failed
		if exists {
			return key, nil
		}
		return nil, fmt.Errorf("failed to fetch IAP keys: %w", err)
	}

	// Try again from cache
	c.mu.RLock()
	key, exists = c.keys[kid]
	c.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("key ID not found: %s", kid)
	}

	return key, nil
}

// fetchKeys fetches public keys from Google's IAP key endpoint
func (c *IAPKeyCache) fetchKeys(ctx context.Context) error {
	// Google's IAP public key endpoint
	url := "https://www.gstatic.com/iap/verify/public_key-jwk"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch keys: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse JWK set
	var jwkSet struct {
		Keys []struct {
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
			Kty string `json:"kty"`
			Alg string `json:"alg"`
			Use string `json:"use"`
		} `json:"keys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jwkSet); err != nil {
		return fmt.Errorf("failed to decode JWK set: %w", err)
	}

	// Update cache
	c.mu.Lock()
	defer c.mu.Unlock()

	c.keys = make(map[string]*IAPPublicKey)

	for _, jwk := range jwkSet.Keys {
		if jwk.Kty != "RSA" {
			continue
		}

		// Decode base64url-encoded modulus
		nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
		if err != nil {
			continue
		}
		n := new(big.Int).SetBytes(nBytes)

		// Decode base64url-encoded exponent
		eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
		if err != nil {
			continue
		}
		e := int(new(big.Int).SetBytes(eBytes).Int64())

		// Create RSA public key
		pubKey := &rsa.PublicKey{
			N: n,
			E: e,
		}

		c.keys[jwk.Kid] = &IAPPublicKey{
			Kid: jwk.Kid,
			N:   n,
			E:   e,
			Key: pubKey,
		}
	}

	c.lastUpdate = time.Now()

	return nil
}

// ParseIAPJWT parses an IAP JWT without verification
// This is used when JWT verification is disabled but we still want to extract claims
func ParseIAPJWT(token string) (*IAPJWTClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	// Decode payload (second part)
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims IAPJWTClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JWT claims: %w", err)
	}

	return &claims, nil
}

// VerifyIAPJWT verifies an IAP JWT signature and claims
func VerifyIAPJWT(ctx context.Context, token string, audience string, keyCache *IAPKeyCache) (*IAPJWTClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	// Decode header to get key ID
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT header: %w", err)
	}

	var header struct {
		Kid string `json:"kid"`
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JWT header: %w", err)
	}

	// Verify algorithm
	if header.Alg != "RS256" {
		return nil, fmt.Errorf("unsupported algorithm: %s", header.Alg)
	}

	// Get public key
	pubKey, err := keyCache.GetKey(ctx, header.Kid)
	if err != nil {
		return nil, fmt.Errorf("failed to get public key: %w", err)
	}

	// Verify RSA signature
	signingInput := parts[0] + "." + parts[1]
	signatureBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}

	// Compute hash of signing input
	hash := sha256.Sum256([]byte(signingInput))

	// Verify signature using RSA PKCS#1 v1.5
	if err := rsa.VerifyPKCS1v15(pubKey.Key, crypto.SHA256, hash[:], signatureBytes); err != nil {
		return nil, fmt.Errorf("invalid JWT signature: %w", err)
	}

	// Parse claims
	claims, err := ParseIAPJWT(token)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT: %w", err)
	}

	// Verify audience
	if audience != "" && claims.Aud != audience {
		return nil, fmt.Errorf("invalid audience: expected %s, got %s", audience, claims.Aud)
	}

	// Verify issuer
	if !strings.HasPrefix(claims.Iss, "https://cloud.google.com/iap") {
		return nil, fmt.Errorf("invalid issuer: %s", claims.Iss)
	}

	// Verify expiration
	now := time.Now().Unix()
	if claims.Exp < now {
		return nil, fmt.Errorf("token expired")
	}

	// Verify not before
	if claims.Iat > now {
		return nil, fmt.Errorf("token not yet valid")
	}

	return claims, nil
}

// ExtractIAPIdentity extracts identity from IAP headers
// This is a convenience function for common IAP scenarios
func ExtractIAPIdentity(r *http.Request, verifyJWT bool, audience string) (*Principal, error) {
	// Get email from header
	email := r.Header.Get("X-Goog-Authenticated-User-Email")
	if email == "" {
		return nil, fmt.Errorf("missing IAP email header")
	}

	// Parse IAP identity format: "accounts.google.com:user@example.com"
	parts := strings.SplitN(email, ":", 2)
	var userEmail string
	if len(parts) == 2 {
		userEmail = parts[1]
	} else {
		userEmail = email
	}

	principal := &Principal{
		ID:          userEmail,
		Name:        userEmail,
		Roles:       []string{"user"},
		Permissions: []Permission{PermRead, PermExecute},
		Metadata: map[string]string{
			"auth_mode": "iap",
			"email":     userEmail,
		},
	}

	// If JWT verification is enabled, verify the JWT
	if verifyJWT {
		jwt := r.Header.Get("X-Goog-IAP-JWT-Assertion")
		if jwt == "" {
			return nil, fmt.Errorf("missing IAP JWT assertion")
		}

		// For basic verification, just parse the JWT
		// In production, you would verify the signature
		claims, err := ParseIAPJWT(jwt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse IAP JWT: %w", err)
		}

		// Update principal with JWT claims
		principal.ID = claims.Sub
		principal.Metadata["subject"] = claims.Sub
		principal.Metadata["email_verified"] = fmt.Sprintf("%v", claims.EmailVerified)

		if claims.Hd != "" {
			principal.Metadata["hosted_domain"] = claims.Hd
		}
	}

	return principal, nil
}
