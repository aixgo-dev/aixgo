# gRPC Security Configuration

## TLS/SSL Requirements

### Production Deployments

**Cloud Run / GCP Serverless:**
- TLS is automatically handled by Google Cloud Platform
- No additional configuration needed
- Certificates managed by Google

**Self-Hosted / Kubernetes:**
- You MUST configure TLS for production use
- Default uses insecure credentials for development only
- Never use insecure credentials in production environments

## Enabling TLS

### Server-side Configuration

```go
package main

import (
    "crypto/tls"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
)

func startSecureServer() error {
    // Load server certificate and key
    cert, err := tls.LoadX509KeyPair("server.crt", "server.key")
    if err != nil {
        return err
    }

    // Create TLS credentials with minimum TLS 1.3
    creds := credentials.NewTLS(&tls.Config{
        Certificates: []tls.Certificate{cert},
        MinVersion:   tls.VersionTLS13,
    })

    // Create gRPC server with TLS
    server := grpc.NewServer(grpc.Creds(creds))

    // Register your services...

    return nil
}
```

### Client-side Configuration

```go
package main

import (
    "crypto/tls"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
)

func connectSecureClient(addr string) (*grpc.ClientConn, error) {
    // Create TLS credentials with minimum TLS 1.3
    creds := credentials.NewTLS(&tls.Config{
        MinVersion: tls.VersionTLS13,
    })

    // Connect with TLS
    conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
    if err != nil {
        return nil, err
    }

    return conn, nil
}
```

## Mutual TLS (mTLS)

For maximum security, use mTLS to authenticate both client and server:

### Server with mTLS

```go
package main

import (
    "crypto/tls"
    "crypto/x509"
    "os"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
)

func startMTLSServer() error {
    // Load server certificate
    cert, err := tls.LoadX509KeyPair("server.crt", "server.key")
    if err != nil {
        return err
    }

    // Load CA certificate for client verification
    certPool := x509.NewCertPool()
    ca, err := os.ReadFile("ca.crt")
    if err != nil {
        return err
    }
    if !certPool.AppendCertsFromPEM(ca) {
        return fmt.Errorf("failed to add CA certificate")
    }

    // Create mTLS credentials
    creds := credentials.NewTLS(&tls.Config{
        Certificates: []tls.Certificate{cert},
        ClientAuth:   tls.RequireAndVerifyClientCert,
        ClientCAs:    certPool,
        MinVersion:   tls.VersionTLS13,
    })

    server := grpc.NewServer(grpc.Creds(creds))

    return nil
}
```

### Client with mTLS

```go
package main

import (
    "crypto/tls"
    "crypto/x509"
    "os"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
)

func connectMTLSClient(addr string) (*grpc.ClientConn, error) {
    // Load client certificate
    cert, err := tls.LoadX509KeyPair("client.crt", "client.key")
    if err != nil {
        return nil, err
    }

    // Load CA certificate for server verification
    certPool := x509.NewCertPool()
    ca, err := os.ReadFile("ca.crt")
    if err != nil {
        return nil, err
    }
    if !certPool.AppendCertsFromPEM(ca) {
        return nil, fmt.Errorf("failed to add CA certificate")
    }

    // Create mTLS credentials
    creds := credentials.NewTLS(&tls.Config{
        Certificates: []tls.Certificate{cert},
        RootCAs:      certPool,
        MinVersion:   tls.VersionTLS13,
    })

    conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
    if err != nil {
        return nil, err
    }

    return conn, nil
}
```

## Environment Variables

Set these environment variables to enforce TLS in production:

- `GRPC_TLS_ENABLED=true` - Enforce TLS (recommended)
- `GRPC_CERT_FILE=/path/to/server.crt` - Server certificate path
- `GRPC_KEY_FILE=/path/to/server.key` - Server private key path
- `GRPC_CA_FILE=/path/to/ca.crt` - CA certificate for mTLS

## Certificate Generation

### For Development/Testing

Generate self-signed certificates for development:

```bash
# Generate CA
openssl genrsa -out ca.key 4096
openssl req -new -x509 -key ca.key -out ca.crt -days 365 \
    -subj "/C=US/ST=State/L=City/O=Organization/CN=CA"

# Generate server certificate
openssl genrsa -out server.key 4096
openssl req -new -key server.key -out server.csr \
    -subj "/C=US/ST=State/L=City/O=Organization/CN=localhost"
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key \
    -CAcreateserial -out server.crt -days 365

# Generate client certificate (for mTLS)
openssl genrsa -out client.key 4096
openssl req -new -key client.key -out client.csr \
    -subj "/C=US/ST=State/L=City/O=Organization/CN=client"
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key \
    -CAcreateserial -out client.crt -days 365
```

### For Production

Use a proper Certificate Authority:

- **Let's Encrypt** for public services
- **Internal PKI** for private services
- **Cloud Provider Certificates** (AWS ACM, GCP Certificate Manager)

## Security Best Practices

1. **Always use TLS 1.3** - Set `MinVersion: tls.VersionTLS13`
2. **Use strong cipher suites** - Go's default TLS config uses secure ciphers
3. **Rotate certificates regularly** - Automate certificate renewal
4. **Use mTLS for service-to-service** - Authenticate both client and server
5. **Never commit certificates to version control** - Use secrets management
6. **Monitor certificate expiration** - Set up alerts for expiring certificates
7. **Disable insecure options** - Never use `grpc.WithInsecure()` in production

## Troubleshooting

### Common Issues

**Certificate Verification Failed:**
```
rpc error: code = Unavailable desc = connection error: ... x509: certificate signed by unknown authority
```
**Solution:** Ensure CA certificate is in the client's trusted certificate pool.

**Handshake Timeout:**
```
rpc error: code = Unavailable desc = connection error: ... i/o timeout
```
**Solution:** Check firewall rules and ensure port is accessible.

**Protocol Version Mismatch:**
```
tls: protocol version not supported
```
**Solution:** Ensure both client and server support the same TLS version.

## Additional Resources

- [gRPC Authentication Guide](https://grpc.io/docs/guides/auth/)
- [Go TLS Documentation](https://pkg.go.dev/crypto/tls)
- [OWASP TLS Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Transport_Layer_Protection_Cheat_Sheet.html)
