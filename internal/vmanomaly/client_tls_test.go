package vmanomaly

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"testing"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/testtls"
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
