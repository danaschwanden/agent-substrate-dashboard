package control

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileBearerTokenCredentialsReloadsProjectedToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	credentials := fileBearerTokenCredentials(path)
	metadata, err := credentials.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata["authorization"]; got != "Bearer first" {
		t.Fatalf("authorization = %q, want %q", got, "Bearer first")
	}

	if err := os.WriteFile(path, []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}
	metadata, err = credentials.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata["authorization"]; got != "Bearer second" {
		t.Fatalf("authorization after rotation = %q, want %q", got, "Bearer second")
	}
	if !credentials.RequireTransportSecurity() {
		t.Fatal("bearer credentials must require transport security")
	}
}

func TestDialRejectsIncompleteAuthentication(t *testing.T) {
	tests := []DialConfig{
		{},
		{Endpoint: "api:443"},
		{Endpoint: "api:443", ServerName: "api.example"},
		{Endpoint: "api:443", ServerName: "api.example", TrustBundleFile: "bundle.pem"},
	}
	for _, config := range tests {
		if connection, err := Dial(context.Background(), config); err == nil {
			connection.Close()
			t.Fatalf("Dial(%+v) unexpectedly succeeded", config)
		}
	}
}
