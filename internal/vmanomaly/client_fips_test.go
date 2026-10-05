//go:build fips

package vmanomaly

import (
	"context"
	"crypto/fips140"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFIPSClientHTTPS(t *testing.T) {
	if !fips140.Enabled() {
		t.Fatal("FIPS test requires the module to be enabled")
	}
	for _, version := range []uint16{tls.VersionTLS12, tls.VersionTLS13, tls.VersionTLS11} {
		t.Run(tls.VersionName(version), func(t *testing.T) {
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("missing bearer token")
				}
				_, _ = w.Write([]byte(`{"status":"ok"}`))
			}))
			server.TLS = &tls.Config{
				MinVersion: version, MaxVersion: version,
				GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
					for _, offered := range hello.SupportedVersions {
						if offered < tls.VersionTLS12 {
							t.Errorf("client offered non-approved TLS version: %x", offered)
						}
					}
					return nil, nil
				},
			}
			server.StartTLS()
			defer server.Close()
			roots := x509.NewCertPool()
			roots.AddCert(server.Certificate())
			client := NewClient(server.URL, "test-token", nil)
			transport := http.DefaultTransport.(*http.Transport).Clone()
			// Allow old TLS in the caller configuration to verify the FIPS module
			// restricts it independently of net/http's default minimum version.
			transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS10}
			client.httpClient.Transport = transport
			defer transport.CloseIdleConnections()
			response, err := client.GetHealth(context.Background())
			if version == tls.VersionTLS11 {
				if err == nil {
					t.Fatal("TLS 1.1 accepted")
				}
			} else if err != nil || response["status"] != "ok" {
				t.Fatalf("HTTPS request failed: %v %v", response, err)
			}
			// Keep certificate verification mandatory as well as approved crypto.
			untrusted := transport.Clone()
			untrusted.TLSClientConfig.RootCAs = x509.NewCertPool()
			client.httpClient.Transport = untrusted
			defer untrusted.CloseIdleConnections()
			if _, err := client.GetHealth(context.Background()); err == nil {
				t.Fatal("untrusted certificate accepted")
			}
		})
	}
}
