package tlsfiles

import (
	"bytes"
	"crypto/tls"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/testtls"
)

func setCheckInterval(t *testing.T, d time.Duration) {
	t.Helper()
	original := checkInterval
	checkInterval = d
	t.Cleanup(func() { checkInterval = original })
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var out bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&out, nil)))
	t.Cleanup(func() { slog.SetDefault(original) })
	return &out
}

// copyFile writes src over dst in place, as a mounted secret update does.
func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("failed to read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		t.Fatalf("failed to write %s: %v", dst, err)
	}
}

// mounted copies the PKI files a client uses into a separate directory.
func mounted(t *testing.T, pki testtls.PKI) testtls.PKI {
	t.Helper()
	dir := t.TempDir()
	m := testtls.PKI{
		CAFile:     filepath.Join(dir, "ca.pem"),
		ClientCert: filepath.Join(dir, "client.pem"),
		ClientKey:  filepath.Join(dir, "client-key.pem"),
	}
	copyFile(t, pki.CAFile, m.CAFile)
	copyFile(t, pki.ClientCert, m.ClientCert)
	copyFile(t, pki.ClientKey, m.ClientKey)
	return m
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}

func newClient(t *testing.T, files testtls.PKI) *http.Client {
	t.Helper()
	base := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	transport, err := NewTransport(base, Files{
		CAFile: files.CAFile, CertFile: files.ClientCert, KeyFile: files.ClientKey,
		CASource: "TEST_CA_FILE", CertSource: "TEST_CERT_FILE",
	})
	if err != nil {
		t.Fatalf("NewTransport failed: %v", err)
	}
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	t.Cleanup(client.CloseIdleConnections)
	return client
}

func get(client *http.Client, url string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

func TestRotatedCAAndClientCertificateAreUsedWithoutRestart(t *testing.T) {
	setCheckInterval(t, 0)
	oldServer, oldPKI := testtls.NewMTLSServer(t, okHandler())
	newServer, newPKI := testtls.NewMTLSServer(t, okHandler())
	files := mounted(t, oldPKI)
	client := newClient(t, files)

	if err := get(client, oldServer.URL); err != nil {
		t.Fatalf("request with the initial files failed: %v", err)
	}
	if err := get(client, newServer.URL); err == nil {
		t.Fatal("the new server must be rejected before rotation")
	}

	copyFile(t, newPKI.CAFile, files.CAFile)
	copyFile(t, newPKI.ClientCert, files.ClientCert)
	copyFile(t, newPKI.ClientKey, files.ClientKey)

	if err := get(client, newServer.URL); err != nil {
		t.Fatalf("request after rotation failed: %v", err)
	}
	if err := get(client, oldServer.URL); err == nil {
		t.Fatal("the old CA must no longer be trusted after rotation")
	}
}

func TestFilesAreReReadAtMostOncePerInterval(t *testing.T) {
	setCheckInterval(t, time.Hour)
	_, oldPKI := testtls.NewMTLSServer(t, okHandler())
	newServer, newPKI := testtls.NewMTLSServer(t, okHandler())
	files := mounted(t, oldPKI)
	client := newClient(t, files)

	copyFile(t, newPKI.CAFile, files.CAFile)
	copyFile(t, newPKI.ClientCert, files.ClientCert)
	copyFile(t, newPKI.ClientKey, files.ClientKey)

	if err := get(client, newServer.URL); err == nil {
		t.Fatal("files must not be re-read before the check interval elapses")
	}
}

func TestInvalidRotationKeepsPreviousMaterial(t *testing.T) {
	setCheckInterval(t, 0)
	logs := captureLogs(t)
	server, pki := testtls.NewMTLSServer(t, okHandler())
	_, otherPKI := testtls.NewMTLSServer(t, okHandler())
	files := mounted(t, pki)
	client := newClient(t, files)

	// A half-written rotation: a garbage CA and a certificate that does not match the key.
	if err := os.WriteFile(files.CAFile, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	copyFile(t, otherPKI.ClientCert, files.ClientCert)

	for range 3 {
		if err := get(client, server.URL); err != nil {
			t.Fatalf("request with the previous material failed: %v", err)
		}
	}
	output := logs.String()
	if got := strings.Count(output, "Failed to reload TLS files"); got != 1 {
		t.Errorf("failure logged %d times for one broken version, want once:\n%s", got, output)
	}

	// A different broken version is reported again.
	if err := os.WriteFile(files.CAFile, []byte("still not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := get(client, server.URL); err != nil {
		t.Fatalf("request with the previous material failed: %v", err)
	}
	if got := strings.Count(logs.String(), "Failed to reload TLS files"); got != 2 {
		t.Errorf("failure logged %d times for two broken versions, want twice:\n%s", got, logs.String())
	}

	// Completing the rotation recovers.
	copyFile(t, pki.CAFile, files.CAFile)
	copyFile(t, pki.ClientCert, files.ClientCert)
	if err := get(client, server.URL); err != nil {
		t.Fatalf("request after a completed rotation failed: %v", err)
	}
}

func TestClientCertificateRotationReplacesKeepAliveConnection(t *testing.T) {
	setCheckInterval(t, 0)
	// The server reports which client certificate the connection presented.
	server, pki := testtls.NewMTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.TLS.PeerCertificates[0].SerialNumber.String()))
	}))
	files := mounted(t, pki)
	client := newClient(t, files)

	serial := func() string {
		t.Helper()
		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return string(body)
	}
	before := serial()

	// Reissue a client certificate from the same CA, with a new serial, and rotate it in place.
	other := testtls.IssueClientCert(t, t.TempDir(), pki, 99)
	copyFile(t, other.ClientCert, files.ClientCert)
	copyFile(t, other.ClientKey, files.ClientKey)

	if after := serial(); after == before || after != "99" {
		t.Fatalf("client certificate serial after rotation = %s, want 99 (before %s)", after, before)
	}
}

func TestConstructorsRejectInvalidFiles(t *testing.T) {
	pki := testtls.WritePKI(t, t.TempDir())
	missing := filepath.Join(t.TempDir(), "missing.pem")
	base := &http.Transport{}
	for name, files := range map[string]Files{
		"missing certificate": {CertFile: missing, KeyFile: pki.ClientKey},
		"mismatched key":      {CertFile: pki.ServerCert, KeyFile: pki.ClientKey},
		"certificate only":    {CertFile: pki.ClientCert},
		"key only":            {KeyFile: pki.ClientKey},
	} {
		if _, err := NewTransport(base, files); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	// CA errors name the setting, e.g. "failed to read TEST_CA_FILE: open ...".
	for name, files := range map[string]Files{
		"missing CA":       {CAFile: missing, CASource: "TEST_CA_FILE"},
		"CA without certs": {CAFile: pki.ClientKey, CASource: "TEST_CA_FILE"},
	} {
		_, err := NewTransport(base, files)
		if err == nil || !strings.Contains(err.Error(), "TEST_CA_FILE") {
			t.Errorf("%s: expected an error naming TEST_CA_FILE, got %v", name, err)
		}
	}
}
