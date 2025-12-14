package security

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// SSRFConfig configures SSRF protection
type SSRFConfig struct {
	// AllowedHosts is the list of allowed hostnames
	AllowedHosts []string
	// AllowedSchemes is the list of allowed URL schemes (default: http, https)
	AllowedSchemes []string
	// AllowLocalhost allows localhost connections (default: true)
	AllowLocalhost bool
	// BlockPrivateIPs blocks RFC1918 private IP ranges (default: true)
	BlockPrivateIPs bool
	// BlockMetadata blocks cloud metadata endpoints (169.254.x.x) (default: true)
	BlockMetadata bool
	// BlockLinkLocal blocks link-local addresses (default: true)
	BlockLinkLocal bool
}

// DefaultSSRFConfig returns a secure default configuration
func DefaultSSRFConfig() SSRFConfig {
	return SSRFConfig{
		AllowedSchemes:  []string{"http", "https"},
		AllowLocalhost:  true,
		BlockPrivateIPs: true,
		BlockMetadata:   true,
		BlockLinkLocal:  true,
	}
}

// SSRFValidator validates URLs against SSRF attacks
type SSRFValidator struct {
	config          SSRFConfig
	allowedHostsMap map[string]bool
}

// NewSSRFValidator creates a new SSRF validator
func NewSSRFValidator(config SSRFConfig) *SSRFValidator {
	// Set defaults
	if len(config.AllowedSchemes) == 0 {
		config.AllowedSchemes = []string{"http", "https"}
	}

	// Build allowed hosts map for O(1) lookup
	allowedHostsMap := make(map[string]bool)
	for _, host := range config.AllowedHosts {
		allowedHostsMap[strings.ToLower(host)] = true
	}

	return &SSRFValidator{
		config:          config,
		allowedHostsMap: allowedHostsMap,
	}
}

// ValidateURL validates a URL against SSRF attacks
func (v *SSRFValidator) ValidateURL(rawURL string) error {
	// Parse URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Validate scheme
	schemeAllowed := false
	for _, scheme := range v.config.AllowedSchemes {
		if strings.EqualFold(parsedURL.Scheme, scheme) {
			schemeAllowed = true
			break
		}
	}
	if !schemeAllowed {
		return fmt.Errorf("invalid URL scheme: %s (only %v allowed)", parsedURL.Scheme, v.config.AllowedSchemes)
	}

	// Validate host
	host := parsedURL.Hostname()
	if err := v.ValidateHost(host); err != nil {
		return err
	}

	return nil
}

// ValidateHost validates a hostname against the allowlist
func (v *SSRFValidator) ValidateHost(host string) error {
	hostLower := strings.ToLower(host)

	// Check against allowlist
	if len(v.allowedHostsMap) > 0 && !v.allowedHostsMap[hostLower] {
		return fmt.Errorf("host not in allowlist: %s", host)
	}

	// Validate resolved IP addresses
	if err := v.validateResolvedIPs(host); err != nil {
		return fmt.Errorf("invalid IP address: %w", err)
	}

	return nil
}

// validateResolvedIPs validates that resolved IP addresses are safe
func (v *SSRFValidator) validateResolvedIPs(host string) error {
	// Skip IP validation for localhost only (always resolves to loopback)
	if v.config.AllowLocalhost && strings.EqualFold(host, "localhost") {
		return nil
	}

	// For certain Docker service names, allow DNS lookup failures
	// This allows testing in environments without Docker networking
	hostLower := strings.ToLower(host)
	isDockerhostname := hostLower == "ollama" ||
		hostLower == "ollama-service" ||
		strings.HasSuffix(hostLower, ".local")

	ips, err := net.LookupIP(host)
	if err != nil {
		// If lookup fails for Docker hostnames, allow to proceed
		if isDockerhostname {
			return nil
		}
		return err
	}

	for _, ip := range ips {
		if err := v.ValidateIP(ip); err != nil {
			return err
		}
	}

	return nil
}

// ValidateIP validates an IP address is not private/blocked
func (v *SSRFValidator) ValidateIP(ip net.IP) error {
	// Allow loopback if localhost is allowed
	if v.config.AllowLocalhost && ip.IsLoopback() {
		return nil
	}

	// Block private IP ranges if configured
	if v.config.BlockPrivateIPs && ip.IsPrivate() {
		return fmt.Errorf("private IP addresses not allowed: %s", ip)
	}

	// Block link-local if configured
	if v.config.BlockLinkLocal {
		if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("link-local addresses not allowed: %s", ip)
		}
	}

	// Block multicast
	if ip.IsMulticast() {
		return fmt.Errorf("multicast addresses not allowed: %s", ip)
	}

	// Special check for metadata service (169.254.169.254)
	if v.config.BlockMetadata {
		if ip.String() == "169.254.169.254" {
			return fmt.Errorf("metadata service address blocked: %s", ip)
		}
	}

	return nil
}

// CreateSecureTransport creates an http.Transport with SSRF protection in DialContext
func (v *SSRFValidator) CreateSecureTransport() *http.Transport {
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Validate the address we're about to connect to
			dialHost, _, err := net.SplitHostPort(addr)
			if err != nil {
				// If no port, the entire addr is the host
				dialHost = addr
			}

			// Re-validate the host (protects against DNS rebinding)
			if err := v.ValidateHost(dialHost); err != nil {
				return nil, fmt.Errorf("connection blocked: %w", err)
			}

			// Use default dialer
			dialer := &net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}
			return dialer.DialContext(ctx, network, addr)
		},
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// DefaultOllamaAllowlist returns the default allowlist for Ollama services
var DefaultOllamaAllowlist = []string{
	"localhost",
	"127.0.0.1",
	"::1",
	"ollama",
	"ollama-service",
}

// GetOllamaAllowedHosts returns the default allowlist plus any from OLLAMA_ALLOWED_HOSTS
func GetOllamaAllowedHosts() []string {
	hosts := make([]string, len(DefaultOllamaAllowlist))
	copy(hosts, DefaultOllamaAllowlist)

	// Check environment variable for additional hosts
	if envHosts := os.Getenv("OLLAMA_ALLOWED_HOSTS"); envHosts != "" {
		// Split by comma and trim whitespace
		additionalHosts := strings.Split(envHosts, ",")
		for _, host := range additionalHosts {
			host = strings.TrimSpace(host)
			if host != "" {
				hosts = append(hosts, host)
			}
		}
	}

	return hosts
}

// NewOllamaSSRFValidator creates a new SSRF validator configured for Ollama
func NewOllamaSSRFValidator() *SSRFValidator {
	config := DefaultSSRFConfig()
	config.AllowedHosts = GetOllamaAllowedHosts()
	return NewSSRFValidator(config)
}
