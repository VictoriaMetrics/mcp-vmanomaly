package vmanomaly

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/testtls"
	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/tlsfiles"
)

func TestClientMutualTLS(t *testing.T) {
	server, pki := testtls.NewMTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	caPEM, err := os.ReadFile(pki.CAFile)
	if err != nil {
		t.Fatalf("failed to read CA: %v", err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(caPEM)

	// trusting the CA is not enough without a client certificate
	client := NewClient(server.URL, "", nil)
	if err := client.SetTLSConfig(&tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetHealth(context.Background()); err == nil {
		t.Fatal("expected handshake failure without a client certificate")
	}

	clientCert, err := tls.LoadX509KeyPair(pki.ClientCert, pki.ClientKey)
	if err != nil {
		t.Fatalf("failed to load client certificate: %v", err)
	}
	client = NewClient(server.URL, "", nil)
	if err := client.SetTLSConfig(&tls.Config{RootCAs: roots, Certificates: []tls.Certificate{clientCert}, MinVersion: tls.VersionTLS12}); err != nil {
		t.Fatal(err)
	}
	health, err := client.GetHealth(context.Background())
	if err != nil {
		t.Fatalf("GetHealth failed: %v", err)
	}
	assertEqual(t, health["status"], "ok")
}

func TestSetTLSConfigNilKeepsDefaults(t *testing.T) {
	client := NewClient("https://localhost:8490", "", nil)
	if err := client.SetTLSConfig(nil); err != nil {
		t.Fatal(err)
	}
	if client.httpClient.Transport != nil {
		t.Fatal("expected the default transport")
	}
}

func TestSetTLSConfigRejectsCustomTransport(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	http.DefaultTransport = customTLSTransport{}
	client := NewClient("https://localhost:8490", "", nil)
	if err := client.SetTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}); err == nil {
		t.Fatal("expected unsupported transport error")
	}
	if client.httpClient.Transport != nil {
		t.Fatal("client transport changed after error")
	}
}

type customTLSTransport struct{}

func (customTLSTransport) RoundTrip(*http.Request) (*http.Response, error) {
	panic("unexpected request")
}

func TestClientUsesRotatedTLSFiles(t *testing.T) {
	health := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	_, oldPKI := testtls.NewMTLSServer(t, health)
	newServer, newPKI := testtls.NewMTLSServer(t, health)

	dir := t.TempDir()
	files := tlsfiles.Files{
		CAFile: filepath.Join(dir, "ca.pem"), CertFile: filepath.Join(dir, "client.pem"), KeyFile: filepath.Join(dir, "client.key"),
		CASource: "VMANOMALY_TLS_CA_FILE", CertSource: "VMANOMALY_TLS_CERT_FILE",
	}
	copyTestFile(t, oldPKI.CAFile, files.CAFile)
	copyTestFile(t, oldPKI.ClientCert, files.CertFile)
	copyTestFile(t, oldPKI.ClientKey, files.KeyFile)

	client := NewClient(newServer.URL, "", nil)
	if err := client.SetTLSFiles(files); err == nil {
		t.Fatal("SetTLSFiles must require SetTLSConfig first")
	}
	if err := client.SetTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}); err != nil {
		t.Fatal(err)
	}
	if err := client.SetTLSFiles(files); err != nil {
		t.Fatalf("SetTLSFiles failed: %v", err)
	}
	if _, err := client.GetHealth(context.Background()); err == nil {
		t.Fatal("the server signed by the new CA must be rejected before rotation")
	}

	// Rotation is picked up on the next check, at most one second later.
	copyTestFile(t, newPKI.CAFile, files.CAFile)
	copyTestFile(t, newPKI.ClientCert, files.CertFile)
	copyTestFile(t, newPKI.ClientKey, files.KeyFile)
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, err := client.GetHealth(context.Background())
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("rotated TLS files were not used: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestSetTLSFilesWithoutFilesKeepsTransport(t *testing.T) {
	client := NewClient("https://localhost:8490", "", nil)
	if err := client.SetTLSFiles(tlsfiles.Files{}); err != nil {
		t.Fatal(err)
	}
	if client.httpClient.Transport != nil {
		t.Fatal("expected the default transport")
	}
}

func copyTestFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
