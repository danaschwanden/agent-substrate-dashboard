package control

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	ateapi "go.ate.dev/substrate/api/ateapi/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const roundRobinServiceConfig = `{"loadBalancingConfig":[{"round_robin":{}}]}`

// DialConfig describes an in-cluster, direct connection to the Control API.
// ServerName is the DNS identity on the serving certificate, which may differ
// from Endpoint when a local tunnel or unusual resolver is used.
type DialConfig struct {
	Endpoint        string
	ServerName      string
	TrustBundleFile string
	TokenFile       string
}

// Connection owns a typed Control client and its underlying gRPC connection.
type Connection struct {
	ateapi.ControlClient
	conn *grpc.ClientConn
}

// Dial creates a direct TLS connection authenticated with a rotating projected
// bearer token. It has no Kubernetes client or subprocess dependency.
func Dial(_ context.Context, cfg DialConfig, extraOptions ...grpc.DialOption) (*Connection, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("control endpoint is required")
	}
	if cfg.ServerName == "" {
		return nil, fmt.Errorf("control TLS server name is required")
	}
	if cfg.TrustBundleFile == "" {
		return nil, fmt.Errorf("control trust bundle file is required")
	}
	if cfg.TokenFile == "" {
		return nil, fmt.Errorf("control token file is required")
	}

	roots, err := loadTrustBundle(cfg.TrustBundleFile)
	if err != nil {
		return nil, err
	}
	creds := fileBearerTokenCredentials(cfg.TokenFile)
	if _, err := creds.GetRequestMetadata(context.Background()); err != nil {
		return nil, err
	}

	options := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS13,
			RootCAs:    roots,
			ServerName: cfg.ServerName,
		})),
		grpc.WithPerRPCCredentials(creds),
		grpc.WithDefaultServiceConfig(roundRobinServiceConfig),
	}
	options = append(options, extraOptions...)
	conn, err := grpc.NewClient(cfg.Endpoint, options...)
	if err != nil {
		return nil, fmt.Errorf("create Control API client: %w", err)
	}
	return &Connection{ControlClient: ateapi.NewControlClient(conn), conn: conn}, nil
}

func loadTrustBundle(path string) (*x509.CertPool, error) {
	bundle, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read Control API trust bundle %q: %w", path, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(bundle) {
		return nil, fmt.Errorf("Control API trust bundle %q contains no certificates", path)
	}
	return pool, nil
}

// Close releases the underlying gRPC connection.
func (c *Connection) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

type fileBearerTokenCredentials string

func (c fileBearerTokenCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	b, err := os.ReadFile(string(c))
	if err != nil {
		return nil, fmt.Errorf("read Control API bearer token file %q: %w", c, err)
	}
	token := strings.TrimSpace(string(b))
	if token == "" {
		return nil, fmt.Errorf("Control API bearer token file %q is empty", c)
	}
	return map[string]string{"authorization": "Bearer " + token}, nil
}

func (fileBearerTokenCredentials) RequireTransportSecurity() bool { return true }
